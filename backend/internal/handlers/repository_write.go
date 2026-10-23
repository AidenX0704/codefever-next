// 仓库域的**写端点**。
//
// 本文件与 handlers/repository.go（只读端点）属于同一个控制器
// （PHP 侧 application/controllers/api/repository.php），
// 但分成两个文件的原因：
//
//  1. 只读端点已经上线并通过契约比对，改动它风险高；
//  2. 写端点需要事务、事件与 git 写操作，代码量是只读侧的数倍。
//
// 注册方式：见 Register() 与 WriteActions() —— 写端点被**合并**进
// 已存在的 "repository" 控制器，而不是新注册一个同名控制器
// （httpapi.Server.Register 会覆盖同名控制器）。
//
// 与 PHP 侧的关键差异（有意为之，逐条在代码注释里标注）：
//
//   - 所有多步写入都在**事务**里完成，事件用**同一个 tx** 入队；
//   - git 失败按**退出码**精确区分，不再像 PHP 那样丢弃退出码；
//   - 建仓 / 派生失败会**回滚事务并清理磁盘目录**，不留孤儿数据。
package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codefever-next/backend/internal/config"
	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
	"codefever-next/backend/internal/gitsvc"
	"codefever-next/backend/internal/httpapi"
	"codefever-next/backend/internal/repo"
)

// RepositoryGit 是写端点依赖的 git 操作集合。
//
// 抽成接口是为了让单元测试可以替换成假的实现 —— 建仓失败、派生失败这类
// 分支必须能在没有真实 git 的环境里被覆盖。
//
// *gitsvc.Service 天然满足该接口。
type RepositoryGit interface {
	// RepoPath 由 r_path 推导出裸仓库的绝对路径。
	RepoPath(rPath string) (string, error)
	// InitBare 初始化裸仓库并装好钩子软链接。
	InitBare(ctx context.Context, repoPath string) error
	// CloneBare 从源裸仓库克隆一个裸仓库。
	CloneBare(ctx context.Context, repoPath, sourcePath string) error
	// InstallHooks 装钩子软链接。
	InstallHooks(ctx context.Context, repoPath string) error
	// RemoveRepo 删除磁盘上的裸仓库目录。
	RemoveRepo(ctx context.Context, repoPath string) error
	// RefExists 判断修订号是否存在。
	RefExists(ctx context.Context, repoPath, ref string) (bool, error)
	// Branches / Tags 列出分支与标签。
	Branches(ctx context.Context, repoPath string) ([]string, error)
	Tags(ctx context.Context, repoPath string) ([]string, error)
	// LastCommit 读取某个引用的最后一次提交。
	LastCommit(ctx context.Context, repoPath, ref, path string, lastSHA ...string) (*domain.Commit, error)
	// CommitListFromLocal 列出两个提交之间的提交（同仓库）。
	CommitListFromLocal(ctx context.Context, repoPath, source, target string) ([]domain.Commit, error)
	// CreateBranch / DeleteBranch 创建与删除分支。
	CreateBranch(ctx context.Context, repoPath, name, origin string) error
	DeleteBranch(ctx context.Context, repoPath, name string) error
	// CreateTag / DeleteTag 创建与删除标签。
	CreateTag(ctx context.Context, repoPath, name, description, commitSHA, authorName, authorEmail string) error
	DeleteTag(ctx context.Context, repoPath, name string) error
	// CanFastForward 判断源提交能否快进到目标提交。
	CanFastForward(ctx context.Context, sourceRepoPath, sourceSHA, targetRepoPath, targetSHA string) (bool, error)
	// MergeBranch 把源分支合并进目标分支并推送。
	MergeBranch(ctx context.Context, sourceRepoPath, sourceBranch, targetRepoPath, targetBranch, message, authorName, authorEmail string, squash bool) (*gitsvc.MergeResult, error)
	// CommitsSinceRemote 列出源仓库中相对目标仓库多出来的提交。
	CommitsSinceRemote(ctx context.Context, sourceRepoPath, sourceSHA, targetRepoPath, targetSHA string) ([]domain.Commit, error)
}

// RepositoryStore 是写端点依赖的仓库读写操作集合。
//
// 抽成接口是为了让单元测试可以注入假的实现 —— 建仓失败、派生失败这类
// 分支必须能在没有真实数据库与 git 的环境里被覆盖。
//
// *repo.RepositoryWriteRepo 天然满足该接口。
type RepositoryStore interface {
	// WithTx 在事务里执行 fn；fn 收到的是绑定到该事务的仓储。
	WithTx(ctx context.Context, fn func(tx repo.RepositoryTx, raw *sql.Tx) error) error

	// 只读查询
	Get(ctx context.Context, rKey string) (*domain.Repository, error)
	RepositoryPath(ctx context.Context, rKey string) (string, error)
	SearchByName(ctx context.Context, name, gKey string) (*domain.Repository, error)
	SearchByDisplayName(ctx context.Context, displayName, gKey string) (*domain.Repository, error)
	GroupsByKeys(ctx context.Context, gKeys []string) (map[string]*domain.RepositoryGroup, error)
	ForkCounts(ctx context.Context, rKeys []string) (map[string]int, error)
	MergeRequestCounts(ctx context.Context, rKeys []string) (map[string]domain.RepositoryMergeRequestCount, error)
	ListProtectedBranchRules(ctx context.Context, rKey string) ([]domain.ProtectedBranchRule, error)
	GetProtectedBranchRule(ctx context.Context, rKey, rule string) (*domain.ProtectedBranchRule, error)
	MergeRequestExists(ctx context.Context, sourceRKey, sourceBranch, targetRKey, targetBranch string) (bool, error)
	NextMergeRequestID(ctx context.Context, targetRKey string) (int, error)
	GetMergeRequestByKey(ctx context.Context, mrKey string) (*domain.RepositoryMergeRequest, error)
	ReviewPending(ctx context.Context, mrKey string) (bool, error)
	GetWebhook(ctx context.Context, rwKey string) (*domain.RepositoryWebhook, *string, error)
	ListWebhooks(ctx context.Context, rKey string) ([]domain.RepositoryWebhook, map[string]*string, error)
	ListWebhookLogs(ctx context.Context, rwKey string) ([]domain.RepositoryWebhookLog, error)
	GetWebhookLogData(ctx context.Context, id string) (*domain.RepositoryWebhookLog, error)
}

// UserStore 是写端点依赖的用户查询集合。
//
// *repo.UserRepo 天然满足该接口。
type UserStore interface {
	Get(ctx context.Context, uKey string) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
}

// RepositoryGuard 是资源级权限校验的最小抽象。
//
// *Guard 天然满足该接口。
type RepositoryGuard interface {
	RequireRepository(ctx context.Context, rKey, uKey string, perm domain.Permission) error
	RequireGroup(ctx context.Context, gKey, uKey string, perm domain.Permission) error
	RepositoryRole(ctx context.Context, rKey, uKey string) (domain.Role, error)
}

// EventBus 是事件总线的最小抽象。
//
// *events.Bus 天然满足该接口。
type EventBus interface {
	Emit(ctx context.Context, tx *sql.Tx, eventType domain.EventType, desc domain.EventDataDesc, data map[string]any, generatedBy string) error
}

// RepositoryWriteHandler 实现 /api/repository/* 的写端点。
type RepositoryWriteHandler struct {
	// Repos 负责仓库域的读写。
	Repos RepositoryStore
	// Users 用于按邮箱 / 主键查用户。
	Users UserStore
	// Guard 是资源级权限守卫。
	Guard RepositoryGuard
	// Git 是 git 写操作。
	Git RepositoryGit
	// Bus 是事件总线；事件与业务写入在同一个事务里入队。
	Bus EventBus
	// Cfg 提供站点配置（头像存储根目录）。
	Cfg *config.Config
}

// RepositoryWriteOption 是 RepositoryWriteHandler 的可选依赖。
type RepositoryWriteOption func(*RepositoryWriteHandler)

// WithRepositoryWriteSiteConfig 注入站点配置。
func WithRepositoryWriteSiteConfig(cfg *config.Config) RepositoryWriteOption {
	return func(h *RepositoryWriteHandler) { h.Cfg = cfg }
}

// NewRepositoryWriteHandler 创建仓库写端点处理器。
func NewRepositoryWriteHandler(
	repos RepositoryStore,
	users UserStore,
	guard RepositoryGuard,
	git RepositoryGit,
	bus EventBus,
	opts ...RepositoryWriteOption,
) *RepositoryWriteHandler {
	h := &RepositoryWriteHandler{
		Repos: repos, Users: users, Guard: guard, Git: git, Bus: bus,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// WriteActions 返回写端点的动作表。
//
// 供外部（main.go）合并进已注册的 "repository" 控制器。
func (h *RepositoryWriteHandler) WriteActions() map[string]map[string]httpapi.ActionHandler {
	return map[string]map[string]httpapi.ActionHandler{
		// 仓库本体
		"create":           {"POST": h.create},
		"fork":             {"POST": h.fork},
		"update":           {"POST": h.update},
		"updateName":       {"POST": h.updateName},
		"deleteRepository": {"POST": h.deleteRepository},
		"uploadAvatar":     {"POST": h.uploadAvatar},

		// 成员
		"addMember":        {"POST": h.addMember},
		"changeMemberRole": {"POST": h.changeMemberRole},
		"removeMember":     {"POST": h.removeMember},
		"changeOwner":      {"POST": h.changeOwner},

		// 分支与标签
		"createBranch":  {"POST": h.createBranch},
		"deleteBranch":  {"POST": h.deleteBranch},
		"createTag":     {"POST": h.createTag},
		"deleteTag":     {"POST": h.deleteTag},
		"defaultBranch": {"POST": h.defaultBranch},

		// 保护分支
		"protectedBranchRules":      {"GET": h.protectedBranchRules},
		"createProtectedBranchRule": {"POST": h.createProtectedBranchRule},
		"updateProtectedBranchRule": {"POST": h.updateProtectedBranchRule},
		"deleteProtectedBranchRule": {"POST": h.deleteProtectedBranchRule},

		// Webhook
		"getWebhook":                  {"POST": h.getWebhook},
		"webhooks":                    {"POST": h.webhooks},
		"editWebhook":                 {"POST": h.editWebhook},
		"deleteWebhook":               {"POST": h.deleteWebhook},
		"getRepositoryWebhookLogs":    {"POST": h.getRepositoryWebhookLogs},
		"getRepositoryWebhookLogData": {"POST": h.getRepositoryWebhookLogData},

		// 合并请求
		"createMergeRequest": {"POST": h.createMergeRequest},
		"mergeRequestReview": {"POST": h.mergeRequestReview},
		"assignReviewer":     {"POST": h.assignReviewer},
		"deleteReviewer":     {"POST": h.deleteReviewer},
		"mergeRequestClose":  {"POST": h.mergeRequestClose},
		"checkMergeType":     {"POST": h.checkMergeType},
		"mergeBranch":        {"POST": h.mergeBranch},
	}
}

// Register 把写端点合并进已注册的 "repository" 控制器。
//
// httpapi.Server.Register 会**覆盖**同名控制器，因此这里不能直接
// s.Register(&httpapi.Controller{Name: "repository", ...}) —— 那会把
// 只读端点全部丢掉。正确做法是取出已注册的控制器，把写端点并进它的
// Actions 表。
//
// 若 "repository" 控制器尚未注册，则新建一个（便于独立部署 / 测试）。
func (h *RepositoryWriteHandler) Register(s *httpapi.Server) {
	ctrl, ok := s.Controller("repository")
	if !ok {
		ctrl = &httpapi.Controller{
			Name:    "repository",
			Auth:    httpapi.AuthWebUser,
			Actions: map[string]map[string]httpapi.ActionHandler{},
		}
		s.Register(ctrl)
	}
	if ctrl.Actions == nil {
		ctrl.Actions = map[string]map[string]httpapi.ActionHandler{}
	}
	for action, methods := range h.WriteActions() {
		ctrl.Actions[action] = methods
	}
}

// MergeRepositoryWriteActions 把写端点并入服务器上已注册的控制器。
//
// 这是给 main.go 用的便捷函数，等价于 h.Register(s)。
func MergeRepositoryWriteActions(s *httpapi.Server, h *RepositoryWriteHandler) {
	if h == nil {
		return
	}
	h.Register(s)
}

// ---------------------------------------------------------------------------
// 公共辅助
// ---------------------------------------------------------------------------

// emit 把领域事件发到事件总线。
//
// tx 必须是产生该事件的业务写入所在的**同一个事务**，
// 从而保证"业务提交则事件在、业务回滚则事件消失"。
//
// Bus 为 nil 时静默跳过（仅用于单元测试）。
func (h *RepositoryWriteHandler) emit(ctx context.Context, tx *sql.Tx, eventType domain.EventType, desc domain.EventDataDesc, data map[string]any, generatedBy string) error {
	if h.Bus == nil || tx == nil {
		return nil
	}
	return h.Bus.Emit(ctx, tx, eventType, desc, data, generatedBy)
}

// repoPath 解析仓库对应的裸仓库绝对路径。
func (h *RepositoryWriteHandler) repoPath(ctx context.Context, rKey string) (string, error) {
	rPath, err := h.Repos.RepositoryPath(ctx, rKey)
	if err != nil {
		return "", err
	}
	return h.Git.RepoPath(rPath)
}

// repository 读取仓库行。
func (h *RepositoryWriteHandler) repository(ctx context.Context, rKey string) (*domain.Repository, error) {
	return h.Repos.Get(ctx, rKey)
}

// groupDisplayName 读取仓库组的显示名（事件里的 "组名/仓库名" 需要它）。
func (h *RepositoryWriteHandler) groupDisplayName(ctx context.Context, gKey string) string {
	groups, err := h.Repos.GroupsByKeys(ctx, []string{gKey})
	if err != nil {
		return ""
	}
	if g, ok := groups[gKey]; ok && g != nil {
		return g.DisplayName
	}
	return ""
}

// groupName 读取仓库组的名称（URL 形态）。
func (h *RepositoryWriteHandler) groupName(ctx context.Context, gKey string) string {
	groups, err := h.Repos.GroupsByKeys(ctx, []string{gKey})
	if err != nil {
		return ""
	}
	if g, ok := groups[gKey]; ok && g != nil {
		return g.Name
	}
	return ""
}

// normalizedRepository 归一化一个仓库（与 repository/list 里的形态一致）。
func (h *RepositoryWriteHandler) normalizedRepository(ctx context.Context, r *domain.Repository) (domain.NormalizedRepository, error) {
	groups, err := h.Repos.GroupsByKeys(ctx, []string{r.GroupKey})
	if err != nil {
		return domain.NormalizedRepository{}, err
	}
	forkCounts, err := h.Repos.ForkCounts(ctx, []string{r.Key})
	if err != nil {
		return domain.NormalizedRepository{}, err
	}
	mrCounts, err := h.Repos.MergeRequestCounts(ctx, []string{r.Key})
	if err != nil {
		return domain.NormalizedRepository{}, err
	}
	return domain.NormalizeRepository(r, groups[r.GroupKey], forkCounts[r.Key], mrCounts[r.Key]), nil
}

// containsString 判断字符串切片里是否含有目标值。
func containsString(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}

// committer 返回提交者的姓名与邮箱（git 提交需要）。
//
// 逐字对应 PHP 侧 Repository_model::execCommand() 的
// `Command::wrapArgument(explode('@', $userInfo['u_email'])[0])` 与
// `Command::wrapArgument($userInfo['u_email'])`。
func (h *RepositoryWriteHandler) committer(ctx context.Context, uKey string) (string, string) {
	if h.Users == nil {
		return uKey, ""
	}
	u, err := h.Users.Get(ctx, uKey)
	if err != nil || u == nil {
		return uKey, ""
	}
	name := u.Email
	if i := strings.IndexByte(name, '@'); i > 0 {
		name = name[:i]
	}
	return name, u.Email
}

// ---------------------------------------------------------------------------
// repository/create
// ---------------------------------------------------------------------------

// create 实现 POST /api/repository/create。
//
// 逐字对应 application/controllers/api/repository.php:22-76：
//
//  1. 缺 displayName / group / name → 0x0201
//  2. 无仓库组建仓权限 → 0x0106
//  3. 仓库名重复 → 0x0401；显示名重复 → 0x0402
//  4. 建仓失败 → 0x0405
//  5. 返回新建仓库的归一化形态
//
// 【与 PHP 的差异 —— 缺陷修复】PHP 侧是"插库 → git init → 失败删行"的
// 手工补偿（application/models/repository_model.php:929-935）：git init 失败时
// 删掉那一行，但**不会清理已经建出来的目录**，而且删行与插行不在一个事务里。
// 这里改成：事务内插库 + 建裸仓库 + 装钩子 + 发事件，任何一步失败都回滚
// 事务（行不会落库）并显式清理磁盘目录。
func (h *RepositoryWriteHandler) create(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	displayName, _ := phpBodyArg(p, "displayName")
	name, _ := phpBodyArg(p, "name")
	groupKey, _ := phpBodyArg(p, "group")

	if !domain.PHPTruthy(displayName) || !domain.PHPTruthy(groupKey) || !domain.PHPTruthy(name) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	if err := h.Guard.RequireGroup(ctx, groupKey, uKey, domain.UACGroupCreateRepo); err != nil {
		return nil, err
	}

	dup, err := h.Repos.SearchByName(ctx, name, groupKey)
	if err != nil {
		return nil, err
	}
	if dup != nil {
		return nil, contract.NewError(contract.CodeDuplicateRepoSlug)
	}
	dup, err = h.Repos.SearchByDisplayName(ctx, displayName, groupKey)
	if err != nil {
		return nil, err
	}
	if dup != nil {
		return nil, contract.NewError(contract.CodeDuplicateRepoDisplayName)
	}

	// PHP 侧 Repository_model::create() 的 $description 形参是 string，
	// 客户端没传时是 null → PHP 8 抛 TypeError（致命错误 → HTTP 500 空响应体）。
	description, hasDescription := phpBodyArg(p, "description")
	if !hasDescription {
		return nil, phpFatalError()
	}

	repoKey := contract.NewKey()
	repoPath := "/" + contract.NewKey()
	absPath, err := h.Git.RepoPath(repoPath)
	if err != nil {
		return nil, err
	}
	groupName := h.groupDisplayName(ctx, groupKey)

	newRepo := &domain.Repository{
		Key:         repoKey,
		OwnerKey:    uKey,
		GroupKey:    groupKey,
		DisplayName: displayName,
		Name:        name,
		Path:        repoPath,
		Description: description,
	}

	if err := h.Repos.WithTx(ctx, func(tx repo.RepositoryTx, raw *sql.Tx) error {
		if err := tx.Create(ctx, newRepo); err != nil {
			return err
		}
		if err := h.Git.InitBare(ctx, absPath); err != nil {
			return err
		}
		return h.emit(ctx, raw, domain.EventTypeREPO_CREATE, domain.EventDataDescREPO_CREATE_D, map[string]any{
			"rKey": repoKey,
			"gKey": groupKey,
			"name": groupName + "/" + displayName,
		}, uKey)
	}); err != nil {
		// 建仓失败：事务已回滚（不会留下数据库行），再清理磁盘目录。
		_ = h.Git.RemoveRepo(ctx, absPath)
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	created, err := h.repository(ctx, repoKey)
	if err != nil {
		return nil, err
	}
	return h.normalizedRepository(ctx, created)
}

// ---------------------------------------------------------------------------
// repository/fork
// ---------------------------------------------------------------------------

// fork 实现 POST /api/repository/fork。
//
// 逐字对应 application/controllers/api/repository.php:78-147：
//
//  1. 缺 displayName / group / name / forkID → 0x0201
//  2. 无源仓库读权限 → 0x0106
//  3. 无仓库组建仓权限 → 0x0106
//  4. 仓库名重复 → 0x0401；显示名重复 → 0x0402
//  5. 派生失败 → 0x0405
//
// 【与 PHP 的差异 —— 缺陷修复】PHP 侧 Repository_model::fork()
// （application/models/repository_model.php:974-976）**丢弃了 execCommand 的
// 返回值**：克隆失败也照样返回 $insertData，于是留下"有数据库行、无磁盘仓库"
// 的孤儿。这里检查克隆结果，失败时回滚事务并清理目录。
func (h *RepositoryWriteHandler) fork(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	displayName, _ := phpBodyArg(p, "displayName")
	name, _ := phpBodyArg(p, "name")
	groupKey, _ := phpBodyArg(p, "group")
	sourceKey, _ := phpBodyArg(p, "forkID")

	if !domain.PHPTruthy(displayName) || !domain.PHPTruthy(groupKey) ||
		!domain.PHPTruthy(name) || !domain.PHPTruthy(sourceKey) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	if err := h.Guard.RequireRepository(ctx, sourceKey, uKey, domain.UACRepoRead); err != nil {
		return nil, err
	}
	if err := h.Guard.RequireGroup(ctx, groupKey, uKey, domain.UACGroupCreateRepo); err != nil {
		return nil, err
	}

	dup, err := h.Repos.SearchByName(ctx, name, groupKey)
	if err != nil {
		return nil, err
	}
	if dup != nil {
		return nil, contract.NewError(contract.CodeDuplicateRepoSlug)
	}
	dup, err = h.Repos.SearchByDisplayName(ctx, displayName, groupKey)
	if err != nil {
		return nil, err
	}
	if dup != nil {
		return nil, contract.NewError(contract.CodeDuplicateRepoDisplayName)
	}

	source, err := h.repository(ctx, sourceKey)
	if err != nil {
		return nil, err
	}
	if source == nil {
		return nil, contract.NewError(contract.CodeActionNotAllowed)
	}

	// PHP 侧 Repository_model::fork() 的 $description 形参是 string，
	// 缺参数 → TypeError → HTTP 500。
	description, hasDescription := phpBodyArg(p, "description")
	if !hasDescription {
		return nil, phpFatalError()
	}

	sourcePath, err := h.repoPath(ctx, sourceKey)
	if err != nil {
		return nil, err
	}

	repoKey := contract.NewKey()
	repoPath := "/" + contract.NewKey()
	absPath, err := h.Git.RepoPath(repoPath)
	if err != nil {
		return nil, err
	}

	targetGroupName := h.groupDisplayName(ctx, groupKey)
	sourceGroupName := h.groupDisplayName(ctx, source.GroupKey)

	newRepo := &domain.Repository{
		Key:         repoKey,
		OwnerKey:    uKey,
		GroupKey:    groupKey,
		ForkFromKey: &sourceKey,
		DisplayName: displayName,
		Name:        name,
		Path:        repoPath,
		Description: description,
		// PHP 侧派生时沿用源仓库头像。
		Avatar: source.Avatar,
	}

	if err := h.Repos.WithTx(ctx, func(tx repo.RepositoryTx, raw *sql.Tx) error {
		if err := tx.Create(ctx, newRepo); err != nil {
			return err
		}
		if err := h.Git.CloneBare(ctx, absPath, sourcePath); err != nil {
			return err
		}
		return h.emit(ctx, raw, domain.EventTypeREPO_FORK, domain.EventDataDescREPO_FORK_D, map[string]any{
			"rKey":       repoKey,
			"gKey":       groupKey,
			"forkFrom":   sourceKey,
			"name":       targetGroupName + "/" + displayName,
			"sourceName": sourceGroupName + "/" + source.DisplayName,
		}, uKey)
	}); err != nil {
		// 派生失败：回滚事务并清理克隆出来的目录。
		_ = h.Git.RemoveRepo(ctx, absPath)
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	created, err := h.repository(ctx, repoKey)
	if err != nil {
		return nil, err
	}
	return h.normalizedRepository(ctx, created)
}

// ---------------------------------------------------------------------------
// repository/update & updateName
// ---------------------------------------------------------------------------

// update 实现 POST /api/repository/update。
//
// 逐字对应 application/controllers/api/repository.php:282-332：
//
//  1. 缺 repository / displayName → 0x0201
//  2. 无仓库信息修改权限 → 0x0106
//  3. 显示名重复 → 0x0402
//  4. 更新显示名与描述；改名 / 改描述分别发事件
func (h *RepositoryWriteHandler) update(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	rKey, _ := phpBodyArg(p, "repository")
	displayName, _ := phpBodyArg(p, "displayName")

	if !domain.PHPTruthy(rKey) || !domain.PHPTruthy(displayName) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoChangeInfo); err != nil {
		return nil, err
	}

	current, err := h.repository(ctx, rKey)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, contract.NewError(contract.CodeActionNotAllowed)
	}

	if current.DisplayName != displayName {
		dup, err := h.Repos.SearchByDisplayName(ctx, displayName, current.GroupKey)
		if err != nil {
			return nil, err
		}
		if dup != nil {
			return nil, contract.NewError(contract.CodeDuplicateRepoDisplayName)
		}
	}

	// PHP 侧 r_description 是 NOT NULL 列，缺参数时会把 NULL 写进去 →
	// MySQL 报错 → CI 的 db_debug 展示错误页（HTTP 500）。
	description, hasDescription := phpBodyArg(p, "description")
	if !hasDescription {
		return nil, phpFatalError()
	}

	if err := h.Repos.WithTx(ctx, func(tx repo.RepositoryTx, raw *sql.Tx) error {
		if err := tx.Update(ctx, rKey, map[string]any{
			"r_display_name": displayName,
			"r_description":  description,
		}); err != nil {
			return err
		}
		if current.DisplayName != displayName {
			if err := h.emit(ctx, raw, domain.EventTypeREPO_UPDATE_NAME, domain.EventDataDescREPO_UPDATE_NAME_D, map[string]any{
				"rKey": rKey,
				"gKey": current.GroupKey,
				"from": current.DisplayName,
				"to":   displayName,
			}, uKey); err != nil {
				return err
			}
		}
		if current.Description != description {
			if err := h.emit(ctx, raw, domain.EventTypeREPO_UPDATE_DESCRIPTION, domain.EventDataDescREPO_UPDATE_DESCRIPTION_D, map[string]any{
				"rKey": rKey,
				"gKey": current.GroupKey,
			}, uKey); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return []any{}, nil
}

// updateName 实现 POST /api/repository/updateName。
//
// 逐字对应 application/controllers/api/repository.php:334-374：
//
//  1. 缺 repository / name → 0x0201
//  2. 无仓库改名权限（UAC_REPO_CHANGE_URL）→ 0x0106
//  3. 仓库名重复 → 0x0401
//  4. 更新 r_name 并发 REPO_CHANGE_URL 事件
func (h *RepositoryWriteHandler) updateName(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	rKey, _ := phpBodyArg(p, "repository")
	name, _ := phpBodyArg(p, "name")

	if !domain.PHPTruthy(rKey) || !domain.PHPTruthy(name) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoChangeURL); err != nil {
		return nil, err
	}

	current, err := h.repository(ctx, rKey)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, contract.NewError(contract.CodeActionNotAllowed)
	}

	dup, err := h.Repos.SearchByName(ctx, name, current.GroupKey)
	if err != nil {
		return nil, err
	}
	if dup != nil {
		return nil, contract.NewError(contract.CodeDuplicateRepoSlug)
	}

	groupName := h.groupName(ctx, current.GroupKey)
	if err := h.Repos.WithTx(ctx, func(tx repo.RepositoryTx, raw *sql.Tx) error {
		if err := tx.Update(ctx, rKey, map[string]any{"r_name": name}); err != nil {
			return err
		}
		return h.emit(ctx, raw, domain.EventTypeREPO_CHANGE_URL, domain.EventDataDescREPO_CHANGE_URL_D, map[string]any{
			"rKey": rKey,
			"gKey": current.GroupKey,
			"from": groupName + "/" + current.Name,
			"to":   groupName + "/" + name,
		}, uKey)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return []any{}, nil
}

// ---------------------------------------------------------------------------
// repository/deleteRepository & uploadAvatar
// ---------------------------------------------------------------------------

// deleteRepository 实现 POST /api/repository/deleteRepository。
//
// 逐字对应 application/controllers/api/repository.php:585-619：
//
//  1. 缺 repository → 0x0201
//  2. 无仓库删除权限 → 0x0106
//  3. 软删除仓库（r_status = 4）并发 REPO_REMOVE 事件
//
// 注意：与 PHP 一致，**不删除磁盘上的裸仓库** —— 仓库是软删除，
// 目录仍可能被恢复使用。
func (h *RepositoryWriteHandler) deleteRepository(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	rKey, _ := phpBodyArg(p, "repository")
	if !domain.PHPTruthy(rKey) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoRemove); err != nil {
		return nil, err
	}

	current, err := h.repository(ctx, rKey)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, contract.NewError(contract.CodeActionNotAllowed)
	}
	groupName := h.groupDisplayName(ctx, current.GroupKey)

	if err := h.Repos.WithTx(ctx, func(tx repo.RepositoryTx, raw *sql.Tx) error {
		if err := tx.RemoveRepository(ctx, rKey); err != nil {
			return err
		}
		return h.emit(ctx, raw, domain.EventTypeREPO_REMOVE, domain.EventDataDescREPO_REMOVE_D, map[string]any{
			"rKey": rKey,
			"gKey": current.GroupKey,
			"name": groupName + "/" + current.DisplayName,
		}, uKey)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return []any{}, nil
}

// uploadAvatar 实现 POST /api/repository/uploadAvatar。
//
// 逐字对应 application/controllers/api/repository.php:244-280：
//
//  1. 缺 repository 或文件 → 0x0201
//  2. 无仓库信息修改权限 → 0x0106
//  3. 保存文件、更新 r_avatar、发 REPO_UPDATE_AVATAR 事件
//
// 注意：PHP 侧是在 update **之后**才读取仓库行，因此事件里的 from 与 to
// 相同（application/controllers/api/repository.php:269-277）。此处逐字保留
// 该行为，避免与既有事件消费者产生差异。
func (h *RepositoryWriteHandler) uploadAvatar(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	rKey, _ := phpBodyArg(p, "repository")
	file := p.File("avatar")

	if !domain.PHPTruthy(rKey) || file == nil {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoChangeInfo); err != nil {
		return nil, err
	}

	key := contract.NewKey()
	if err := h.saveAvatar(key, file); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	if err := h.Repos.WithTx(ctx, func(tx repo.RepositoryTx, raw *sql.Tx) error {
		if err := tx.Update(ctx, rKey, map[string]any{"r_avatar": key}); err != nil {
			return err
		}
		current, err := h.repository(ctx, rKey)
		if err != nil {
			return err
		}
		if current == nil {
			return contract.NewError(contract.CodeActionNotAllowed)
		}
		// PHP 侧在 update 之后才读仓库行 → from 与 to 相同（逐字保留）。
		from := ""
		if current.Avatar != nil {
			from = *current.Avatar
		}
		return h.emit(ctx, raw, domain.EventTypeREPO_UPDATE_AVATAR, domain.EventDataDescREPO_UPDATE_AVATAR_D, map[string]any{
			"rKey": rKey,
			"gKey": current.GroupKey,
			"from": from,
			"to":   key,
		}, uKey)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return []any{}, nil
}

// saveAvatar 保存头像文件。
//
// 与 GroupHandler.saveAvatar 同源：PHP 的存储根目录是 <base>/file-storage/avatar
// （application/libraries/storage/disk.php:12），文件名按前 5 个字符分目录。
func (h *RepositoryWriteHandler) saveAvatar(key string, file *contract.UploadedFile) error {
	if h.Cfg == nil {
		return errors.New("缺少文件存储配置")
	}
	target := filepath.Join(h.Cfg.FileStoragePath(), "avatar",
		filepath.FromSlash(avatarRelativePath(key)))
	if err := os.MkdirAll(filepath.Dir(target), 0o777); err != nil {
		return err
	}
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(target)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = dst.ReadFrom(src)
	return err
}

// ---------------------------------------------------------------------------
// 成员
// ---------------------------------------------------------------------------

// addMember 实现 POST /api/repository/addMember。
//
// 逐字对应 application/controllers/api/repository.php:376-423：
//
//  1. 缺 repository / email → 0x0201
//  2. 无仓库成员管理权限 → 0x0106
//  3. 邮箱查无此人 → 0x0407
//  4. 不能把创建者加为成员 → 0x0408
//  5. 加入失败 → 0x0405
func (h *RepositoryWriteHandler) addMember(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	rKey, _ := phpBodyArg(p, "repository")
	email, _ := phpBodyArg(p, "email")

	if !domain.PHPTruthy(rKey) || !domain.PHPTruthy(email) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoChangeMember); err != nil {
		return nil, err
	}

	member, err := h.Users.GetByEmail(ctx, email)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return nil, err
	}
	if errors.Is(err, repo.ErrNotFound) || member == nil {
		return nil, contract.NewError(contract.CodeUserNotFound)
	}

	current, err := h.repository(ctx, rKey)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, contract.NewError(contract.CodeActionNotAllowed)
	}
	if current.OwnerKey == member.Key {
		return nil, contract.NewError(contract.CodeCannotAddCreator)
	}

	if err := h.Repos.WithTx(ctx, func(tx repo.RepositoryTx, raw *sql.Tx) error {
		if err := tx.AddMember(ctx, member.Key, rKey); err != nil {
			return err
		}
		return h.emit(ctx, raw, domain.EventTypeREPO_ADD_MEMBER, domain.EventDataDescREPO_ADD_MEMBER_D, map[string]any{
			"rKey":  rKey,
			"gKey":  current.GroupKey,
			"uid":   member.Key,
			"name":  member.Name,
			"email": member.Email,
		}, uKey)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return []any{}, nil
}

// changeMemberRole 实现 POST /api/repository/changeMemberRole。
//
// 逐字对应 application/controllers/api/repository.php:425-477：
//
//  1. 缺 repository / userID / 非法 roleID → 0x0201
//  2. 无仓库成员管理权限 → 0x0106
//  3. 用户不存在 → 0x0407
//  4. 不能改创建者的角色 → 0x0106
//  5. 改角色失败 → 0x0405
func (h *RepositoryWriteHandler) changeMemberRole(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	rKey, _ := phpBodyArg(p, "repository")
	userID, _ := phpBodyArg(p, "userID")
	roleRaw, rolePresent := phpBodyArg(p, "roleID")

	if !domain.PHPTruthy(rKey) || !domain.PHPTruthy(userID) ||
		!domain.GroupMemberRoleAllowed(roleRaw, rolePresent) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoChangeMember); err != nil {
		return nil, err
	}

	member, err := h.Users.Get(ctx, userID)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return nil, err
	}
	if errors.Is(err, repo.ErrNotFound) || member == nil {
		return nil, contract.NewError(contract.CodeUserNotFound)
	}

	current, err := h.repository(ctx, rKey)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, contract.NewError(contract.CodeActionNotAllowed)
	}
	if current.OwnerKey == member.Key {
		return nil, contract.NewError(contract.CodeActionNotAllowed)
	}

	// roleID 已经被 in_array 校验过（只能是 1~5），这里的转换不会失败。
	role := atoiOrZero(roleRaw)
	if err := h.Repos.WithTx(ctx, func(tx repo.RepositoryTx, raw *sql.Tx) error {
		if err := tx.SetMemberRole(ctx, member.Key, rKey, role); err != nil {
			return err
		}
		return h.emit(ctx, raw, domain.EventTypeREPO_CHANGE_MEMBER_ROLE, domain.EventDataDescREPO_CHANGE_MEMBER_ROLE_D, map[string]any{
			"rKey":  rKey,
			"gKey":  current.GroupKey,
			"uid":   member.Key,
			"name":  member.Name,
			"email": member.Email,
			"to":    role,
		}, uKey)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return []any{}, nil
}

// removeMember 实现 POST /api/repository/removeMember。
//
// 逐字对应 application/controllers/api/repository.php:538-583：
//
//  1. 缺 repository / userID → 0x0201
//  2. 无仓库成员管理权限 → 0x0106
//  3. 用户不存在 → 0x0407
//  4. 不能移除创建者 → 0x0106
//  5. 移除失败 → 0x0405
func (h *RepositoryWriteHandler) removeMember(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	rKey, _ := phpBodyArg(p, "repository")
	userID, _ := phpBodyArg(p, "userID")

	if !domain.PHPTruthy(rKey) || !domain.PHPTruthy(userID) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoChangeMember); err != nil {
		return nil, err
	}

	member, err := h.Users.Get(ctx, userID)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return nil, err
	}
	if errors.Is(err, repo.ErrNotFound) || member == nil {
		return nil, contract.NewError(contract.CodeUserNotFound)
	}

	current, err := h.repository(ctx, rKey)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, contract.NewError(contract.CodeActionNotAllowed)
	}
	if current.OwnerKey == member.Key {
		return nil, contract.NewError(contract.CodeActionNotAllowed)
	}

	if err := h.Repos.WithTx(ctx, func(tx repo.RepositoryTx, raw *sql.Tx) error {
		if err := tx.RemoveMember(ctx, member.Key, rKey); err != nil {
			return err
		}
		return h.emit(ctx, raw, domain.EventTypeREPO_REMOVE_MEMBER, domain.EventDataDescREPO_REMOVE_MEMBER_D, map[string]any{
			"rKey":  rKey,
			"gKey":  current.GroupKey,
			"uid":   member.Key,
			"name":  member.Name,
			"email": member.Email,
		}, uKey)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return []any{}, nil
}

// changeOwner 实现 POST /api/repository/changeOwner。
//
// 逐字对应 application/controllers/api/repository.php:479-536：
//
//  1. 缺 repository / userID → 0x0201
//  2. 无仓库转让权限 → 0x0106
//  3. 用户不存在 → 0x0407
//  4. 不能把创建者转让给自己 → 0x0106
//  5. 三步写入（旧属主降为成员 / 新属主入主 / 移除新属主的成员身份）
//
// 【与 PHP 的差异 —— 缺陷修复】PHP 侧这三步写入**没有事务**
// （application/controllers/api/repository.php:511-525），中途失败会留下
// "属主已换但成员表没换"的半成品。这里放进同一个事务。
func (h *RepositoryWriteHandler) changeOwner(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	rKey, _ := phpBodyArg(p, "repository")
	userID, _ := phpBodyArg(p, "userID")

	if !domain.PHPTruthy(rKey) || !domain.PHPTruthy(userID) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoChangeOwner); err != nil {
		return nil, err
	}

	member, err := h.Users.Get(ctx, userID)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return nil, err
	}
	if errors.Is(err, repo.ErrNotFound) || member == nil {
		return nil, contract.NewError(contract.CodeUserNotFound)
	}

	current, err := h.repository(ctx, rKey)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, contract.NewError(contract.CodeActionNotAllowed)
	}
	if current.OwnerKey == member.Key {
		return nil, contract.NewError(contract.CodeActionNotAllowed)
	}

	if err := h.Repos.WithTx(ctx, func(tx repo.RepositoryTx, raw *sql.Tx) error {
		// 旧属主降为"仓库成员（OWNER 角色）"。
		if err := tx.SetMemberRole(ctx, current.OwnerKey, rKey, int(domain.RoleOwner)); err != nil {
			return err
		}
		// 新属主入主。
		if err := tx.Update(ctx, rKey, map[string]any{"u_key": member.Key}); err != nil {
			return err
		}
		// 移除新属主的仓库成员身份。
		if err := tx.RemoveMember(ctx, member.Key, rKey); err != nil {
			return err
		}
		return h.emit(ctx, raw, domain.EventTypeREPO_CHANGE_OWNER, domain.EventDataDescREPO_CHANGE_OWNER_D, map[string]any{
			"rKey":  rKey,
			"gKey":  current.GroupKey,
			"uid":   member.Key,
			"name":  member.Name,
			"email": member.Email,
		}, uKey)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return []any{}, nil
}

// ---------------------------------------------------------------------------
// 分支与标签
// ---------------------------------------------------------------------------

// createBranch 实现 POST /api/repository/createBranch。
//
// 逐字对应 application/controllers/api/repository.php:1407-1455：
//
//  1. 缺 repository / name / origin → 0x0201
//  2. 分支名非法 → 0x0202
//  3. 无建分支权限 → 0x0106
//  4. 分支名命中保留 URI → 0x0403
//  5. 分支名与已有分支 / 标签重名 → 0x0406
//  6. 建分支失败 → 0x0405
//
// 【已修掉的 PHP 缺陷 ①】application/controllers/api/repository.php:1442-1445 写成
//
//	$result = $this->repositoryModel->createBranch($name, $rKey, $origin, $uKey);
//	if ($result) { Response::reject(0x040C); }
//
// 而 Repository_model::createBranch()（application/models/repository_model.php:1136-1155）
// **成功时返回 true** —— 于是"建分支成功"返回 0x040C（Old Target Not Exists），
// 失败反而走到下面输出空对象（成功）。这里修正为正确语义：
// 成功 → 发事件 + 输出空对象；失败 → 0x0405。
//
// 【已修掉的 PHP 缺陷 ②】PHP 把"源 ref 不存在"也报成 0x040C，与
// "新增目标已存在"（0x0406 New Target Exists）混为一谈。这里按语义区分：
// 源 ref 不存在 → 0x040C；新增目标已存在 → 0x0406。
func (h *RepositoryWriteHandler) createBranch(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	rKey, _ := phpBodyArg(p, "repository")
	name := domain.PHPTrim(bodyOrEmpty(p, "name"))
	origin := domain.PHPTrim(bodyOrEmpty(p, "origin"))

	if !domain.PHPTruthy(rKey) || !domain.PHPTruthy(name) || !domain.PHPTruthy(origin) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if !domain.BranchNameValid(name) {
		return nil, contract.NewError(contract.CodeAmountOutOfRange)
	}
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoBranchCreate); err != nil {
		return nil, err
	}
	if domain.GroupNameIsPreserved(name) {
		return nil, contract.NewError(contract.CodeDuplicateGroupSlug)
	}

	repoPath, err := h.repoPath(ctx, rKey)
	if err != nil {
		return nil, err
	}

	// 新增目标已存在 → 0x0406（PHP 侧的 in_array 检查）。
	branches, err := h.Git.Branches(ctx, repoPath)
	if err != nil {
		return nil, err
	}
	tags, err := h.Git.Tags(ctx, repoPath)
	if err != nil {
		return nil, err
	}
	if containsString(branches, name) || containsString(tags, name) {
		return nil, contract.NewError(contract.CodeTargetExists)
	}

	// 源 ref 不存在 → 0x040C（Old Target Not Exists）。
	exists, err := h.Git.RefExists(ctx, repoPath, origin)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, contract.NewError(contract.CodeOldTargetNotExists)
	}

	if err := h.Git.CreateBranch(ctx, repoPath, name, origin); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	current, err := h.repository(ctx, rKey)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, contract.NewError(contract.CodeActionNotAllowed)
	}
	if err := h.Repos.WithTx(ctx, func(tx repo.RepositoryTx, raw *sql.Tx) error {
		return h.emit(ctx, raw, domain.EventTypeBRANCH_CREATE, domain.EventDataDescBRANCH_CREATE_D, map[string]any{
			"gKey": current.GroupKey,
			"rKey": rKey,
			"name": name,
		}, uKey)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return []any{}, nil
}

// deleteBranch 实现 POST /api/repository/deleteBranch。
//
// 逐字对应 application/controllers/api/repository.php:1866-1900：
//
//  1. 缺 repository / branch → 0x0201
//  2. 无删分支权限 → 0x0106
//  3. 不能删默认分支 → 0x0405
//  4. 删除失败 → 0x0405
func (h *RepositoryWriteHandler) deleteBranch(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	rKey, _ := phpBodyArg(p, "repository")
	branch := domain.PHPTrim(bodyOrEmpty(p, "branch"))

	if !domain.PHPTruthy(rKey) || !domain.PHPTruthy(branch) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoBranchRemove); err != nil {
		return nil, err
	}

	current, err := h.repository(ctx, rKey)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, contract.NewError(contract.CodeActionNotAllowed)
	}
	if current.DefaultBranch != nil && *current.DefaultBranch == branch {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	repoPath, err := h.repoPath(ctx, rKey)
	if err != nil {
		return nil, err
	}
	if err := h.Git.DeleteBranch(ctx, repoPath, branch); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	if err := h.Repos.WithTx(ctx, func(tx repo.RepositoryTx, raw *sql.Tx) error {
		return h.emit(ctx, raw, domain.EventTypeBRANCH_REMOVE, domain.EventDataDescBRANCH_REMOVE_D, map[string]any{
			"gKey": current.GroupKey,
			"rKey": rKey,
			"name": branch,
		}, uKey)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return []any{}, nil
}

// createTag 实现 POST /api/repository/createTag。
//
// 逐字对应 application/controllers/api/repository.php:1920-1965：
//
//  1. 缺 repository / name → 0x0201
//  2. 标签名非法 → 0x0202
//  3. 无建标签权限 → 0x0106
//  4. 标签名命中保留 URI → 0x0403
//  5. 标签名与已有分支 / 标签重名 → 0x0406
//  6. 建标签失败 → 0x0405
//
// 【已修掉的 PHP 缺陷 ①】application/controllers/api/repository.php:1952-1955
// 与建分支是同一处错误：`if ($result) { Response::reject(0x040C); }` ——
// Repository_model::createTag() 成功时返回 true，因此"建标签成功"反而报
// 0x040C。这里修正为正确语义。
//
// 【已修掉的 PHP 缺陷 ②】源提交不存在时按语义报 0x040C（Old Target Not Exists）。
func (h *RepositoryWriteHandler) createTag(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	rKey, _ := phpBodyArg(p, "repository")
	name := domain.PHPTrim(bodyOrEmpty(p, "name"))
	origin := domain.PHPTrim(bodyOrEmpty(p, "origin"))
	description := domain.PHPTrim(bodyOrEmpty(p, "description"))

	if !domain.PHPTruthy(rKey) || !domain.PHPTruthy(name) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if !domain.TagNameValid(name) {
		return nil, contract.NewError(contract.CodeAmountOutOfRange)
	}
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoTagCreate); err != nil {
		return nil, err
	}
	if domain.GroupNameIsPreserved(name) {
		return nil, contract.NewError(contract.CodeDuplicateGroupSlug)
	}

	repoPath, err := h.repoPath(ctx, rKey)
	if err != nil {
		return nil, err
	}

	branches, err := h.Git.Branches(ctx, repoPath)
	if err != nil {
		return nil, err
	}
	tags, err := h.Git.Tags(ctx, repoPath)
	if err != nil {
		return nil, err
	}
	if containsString(branches, name) || containsString(tags, name) {
		return nil, contract.NewError(contract.CodeTargetExists)
	}

	// 源提交（可选）。给了但不存在 → 0x040C。
	if origin != "" {
		exists, err := h.Git.RefExists(ctx, repoPath, origin)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, contract.NewError(contract.CodeOldTargetNotExists)
		}
	}

	// 附注标签需要提交者身份（PHP 侧由 GIT_COMMAND_QUERY 分支导出）。
	committerName, committerEmail := h.committer(ctx, uKey)

	if err := h.Git.CreateTag(ctx, repoPath, name, description, origin, committerName, committerEmail); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	current, err := h.repository(ctx, rKey)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, contract.NewError(contract.CodeActionNotAllowed)
	}
	if err := h.Repos.WithTx(ctx, func(tx repo.RepositoryTx, raw *sql.Tx) error {
		return h.emit(ctx, raw, domain.EventTypeTAG_CREATE, domain.EventDataDescTAG_CREATE_D, map[string]any{
			"gKey": current.GroupKey,
			"rKey": rKey,
			"name": name,
		}, uKey)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return []any{}, nil
}

// deleteTag 实现 POST /api/repository/deleteTag。
//
// 逐字对应 application/controllers/api/repository.php:1967-1996：
//
//  1. 缺 repository / tag → 0x0201
//  2. 无删标签权限 → 0x0106
//  3. 删除失败 → 0x0405
func (h *RepositoryWriteHandler) deleteTag(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	rKey, _ := phpBodyArg(p, "repository")
	tag := domain.PHPTrim(bodyOrEmpty(p, "tag"))

	if !domain.PHPTruthy(rKey) || !domain.PHPTruthy(tag) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoTagRemove); err != nil {
		return nil, err
	}

	repoPath, err := h.repoPath(ctx, rKey)
	if err != nil {
		return nil, err
	}
	if err := h.Git.DeleteTag(ctx, repoPath, tag); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	current, err := h.repository(ctx, rKey)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, contract.NewError(contract.CodeActionNotAllowed)
	}
	if err := h.Repos.WithTx(ctx, func(tx repo.RepositoryTx, raw *sql.Tx) error {
		return h.emit(ctx, raw, domain.EventTypeTAG_REMOVE, domain.EventDataDescTAG_REMOVE_D, map[string]any{
			"gKey": current.GroupKey,
			"rKey": rKey,
			"name": tag,
		}, uKey)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return []any{}, nil
}

// defaultBranch 实现 POST /api/repository/defaultBranch。
//
// 逐字对应 application/controllers/api/repository.php:621-654：
//
//  1. 缺 branch 或 old === branch → 0x0201
//  2. 无改默认分支权限 → 0x0106
//  3. 更新失败 → 0x0405
func (h *RepositoryWriteHandler) defaultBranch(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	rKey, _ := phpBodyArg(p, "repository")
	old := bodyOrEmpty(p, "old")
	branch := bodyOrEmpty(p, "branch")

	if !domain.PHPTruthy(branch) || old == branch {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoDefaultBranchChange); err != nil {
		return nil, err
	}

	if err := h.Repos.WithTx(ctx, func(tx repo.RepositoryTx, raw *sql.Tx) error {
		if err := tx.Update(ctx, rKey, map[string]any{"r_default_branch_name": branch}); err != nil {
			return err
		}
		current, err := h.repository(ctx, rKey)
		if err != nil {
			return err
		}
		if current == nil {
			return contract.NewError(contract.CodeActionNotAllowed)
		}
		return h.emit(ctx, raw, domain.EventTypeDEFAULT_BRANCH_CHANGE, domain.EventDataDescDEFAULT_BRANCH_CHANGE_D, map[string]any{
			"gKey": current.GroupKey,
			"rKey": rKey,
			"from": old,
			"to":   branch,
		}, uKey)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return []any{}, nil
}

// ---------------------------------------------------------------------------
// 保护分支
// ---------------------------------------------------------------------------

// protectedBranchRules 实现 GET /api/repository/protectedBranchRules。
//
// 逐字对应 application/controllers/api/repository.php:656-681：
//
//  1. 缺 repository → 0x0201
//  2. 无仓库读权限 → 0x0106
//  3. 返回保护分支规则列表
func (h *RepositoryWriteHandler) protectedBranchRules(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	rKey := req.Parsed.QueryString("repository")

	if !domain.PHPTruthy(rKey) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoRead); err != nil {
		return nil, err
	}

	rules, err := h.Repos.ListProtectedBranchRules(ctx, rKey)
	if err != nil {
		return nil, err
	}
	return domain.NormalizeProtectedBranchRules(rules), nil
}

// createProtectedBranchRule 实现 POST /api/repository/createProtectedBranchRule。
//
// 逐字对应 application/controllers/api/repository.php:683-733：
//
//  1. 缺 repository / rule / rolePush / roleMerge → 0x0201
//  2. 角色取值非法 → 0x0201
//  3. 规则已存在 → 0x0201
//  4. 无建规则权限 → 0x0106
//  5. 写入失败 → 0x0405
//
// 注意 PHP 的**校验顺序**：重名检查在权限检查**之前**，这里逐字保留。
func (h *RepositoryWriteHandler) createProtectedBranchRule(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	rKey, _ := phpBodyArg(p, "repository")
	rule, _ := phpBodyArg(p, "rule")
	rolePushRaw, rolePushPresent := phpBodyArg(p, "rolePush")
	roleMergeRaw, roleMergePresent := phpBodyArg(p, "roleMerge")

	if !domain.PHPTruthy(rKey) || !domain.PHPTruthy(rule) ||
		!domain.PHPTruthy(rolePushRaw) || !domain.PHPTruthy(roleMergeRaw) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if !domain.ProtectedBranchPushRoleAllowed(rolePushRaw, rolePushPresent) ||
		!domain.ProtectedBranchMergeRoleAllowed(roleMergeRaw, roleMergePresent) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	existing, err := h.Repos.GetProtectedBranchRule(ctx, rKey, rule)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoProtectedBranchRuleCreate); err != nil {
		return nil, err
	}

	rolePush := atoiOrZero(rolePushRaw)
	roleMerge := atoiOrZero(roleMergeRaw)

	if err := h.Repos.WithTx(ctx, func(tx repo.RepositoryTx, raw *sql.Tx) error {
		if err := tx.CreateProtectedBranchRule(ctx, rKey, rule, rolePush, roleMerge); err != nil {
			return err
		}
		current, err := h.repository(ctx, rKey)
		if err != nil {
			return err
		}
		if current == nil {
			return contract.NewError(contract.CodeActionNotAllowed)
		}
		return h.emit(ctx, raw, domain.EventTypePROTECTED_BRANCH_RULE_CREATE, domain.EventDataDescPROTECTED_BRANCH_RULE_CREATE_D, map[string]any{
			"gKey": current.GroupKey,
			"rKey": rKey,
			"name": rule,
		}, uKey)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return []any{}, nil
}

// updateProtectedBranchRule 实现 POST /api/repository/updateProtectedBranchRule。
//
// 逐字对应 application/controllers/api/repository.php:735-793：
//
//  1. 缺 repository / id / rule / rolePush / roleMerge → 0x0201
//  2. 角色取值非法 → 0x0201
//  3. 同名规则已属于另一条规则 → 0x0201
//  4. 无权限 → 0x0106
//  5. 更新失败 → 0x0405
//
// 注意：PHP 侧这里用的是 UAC_REPO_PROTECTED_BRANCH_RULE_CREATE
// （application/controllers/api/repository.php:770），而常量表里另有
// UAC_REPO_PROTECTED_BRANCH_RULE_UPDATE（0x1E）。二者在权限矩阵里
// 完全等价（只有 Maintainer 与 Owner 同时拥有两者），因此这里按 PHP
// 原样使用 CREATE，行为零差异。
func (h *RepositoryWriteHandler) updateProtectedBranchRule(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	rKey, _ := phpBodyArg(p, "repository")
	pbrKey, _ := phpBodyArg(p, "id")
	rule, _ := phpBodyArg(p, "rule")
	rolePushRaw, rolePushPresent := phpBodyArg(p, "rolePush")
	roleMergeRaw, roleMergePresent := phpBodyArg(p, "roleMerge")

	if !domain.PHPTruthy(rKey) || !domain.PHPTruthy(pbrKey) || !domain.PHPTruthy(rule) ||
		!domain.PHPTruthy(rolePushRaw) || !domain.PHPTruthy(roleMergeRaw) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if !domain.ProtectedBranchPushRoleAllowed(rolePushRaw, rolePushPresent) ||
		!domain.ProtectedBranchMergeRoleAllowed(roleMergeRaw, roleMergePresent) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	sameRule, err := h.Repos.GetProtectedBranchRule(ctx, rKey, rule)
	if err != nil {
		return nil, err
	}
	if sameRule != nil && sameRule.Key != pbrKey {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoProtectedBranchRuleCreate); err != nil {
		return nil, err
	}

	rolePush := atoiOrZero(rolePushRaw)
	roleMerge := atoiOrZero(roleMergeRaw)

	if err := h.Repos.WithTx(ctx, func(tx repo.RepositoryTx, raw *sql.Tx) error {
		if err := tx.UpdateProtectedBranchRule(ctx, pbrKey, map[string]any{
			"pbr_rule":              rule,
			"pbr_push_min_role_id":  rolePush,
			"pbr_merge_min_role_id": roleMerge,
		}); err != nil {
			return err
		}
		current, err := h.repository(ctx, rKey)
		if err != nil {
			return err
		}
		if current == nil {
			return contract.NewError(contract.CodeActionNotAllowed)
		}
		return h.emit(ctx, raw, domain.EventTypePROTECTED_BRANCH_RULE_CHANGE, domain.EventDataDescPROTECTED_BRANCH_RULE_CHANGE_D, map[string]any{
			"gKey": current.GroupKey,
			"rKey": rKey,
			"name": rule,
		}, uKey)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return []any{}, nil
}

// deleteProtectedBranchRule 实现 POST /api/repository/deleteProtectedBranchRule。
//
// 逐字对应 application/controllers/api/repository.php:795-832：
//
//  1. 缺 repository / id / rule → 0x0201
//  2. 无删规则权限 → 0x0106
//  3. 软删除（pbr_status = 4）失败 → 0x0405
func (h *RepositoryWriteHandler) deleteProtectedBranchRule(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	rKey, _ := phpBodyArg(p, "repository")
	pbrKey, _ := phpBodyArg(p, "id")
	rule, _ := phpBodyArg(p, "rule")

	if !domain.PHPTruthy(rKey) || !domain.PHPTruthy(rule) || !domain.PHPTruthy(pbrKey) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoProtectedBranchRuleRemove); err != nil {
		return nil, err
	}

	if err := h.Repos.WithTx(ctx, func(tx repo.RepositoryTx, raw *sql.Tx) error {
		if err := tx.UpdateProtectedBranchRule(ctx, pbrKey, map[string]any{
			"pbr_status":  domain.CommonStatusDelete,
			"pbr_deleted": nowString(),
		}); err != nil {
			return err
		}
		current, err := h.repository(ctx, rKey)
		if err != nil {
			return err
		}
		if current == nil {
			return contract.NewError(contract.CodeActionNotAllowed)
		}
		return h.emit(ctx, raw, domain.EventTypePROTECTED_BRANCH_RULE_REMOVE, domain.EventDataDescPROTECTED_BRANCH_RULE_REMOVE_D, map[string]any{
			"gKey": current.GroupKey,
			"rKey": rKey,
			"name": rule,
		}, uKey)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return []any{}, nil
}

// ---------------------------------------------------------------------------
// Webhook
// ---------------------------------------------------------------------------

// getWebhook 实现 POST /api/repository/getWebhook。
//
// 逐字对应 application/controllers/api/repository.php:2034-2052：
//
//  1. 缺 repository / rwKey → 0x0201
//  2. 无仓库读权限 → 0x0106
//  3. 返回 webhook；不存在时返回空数组
func (h *RepositoryWriteHandler) getWebhook(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	p := req.Parsed

	rKey, _ := phpBodyArg(p, "repository")
	rwKey, _ := phpBodyArg(p, "rwKey")

	if !domain.PHPTruthy(rKey) || !domain.PHPTruthy(rwKey) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoRead); err != nil {
		return nil, err
	}

	webhook, _, err := h.Repos.GetWebhook(ctx, rwKey)
	if err != nil {
		return nil, err
	}
	if webhook == nil {
		return []any{}, nil
	}
	return domain.NormalizeRepositoryWebhook(webhook, nil), nil
}

// webhooks 实现 POST /api/repository/webhooks。
//
// 逐字对应 application/controllers/api/repository.php:2054-2071。
func (h *RepositoryWriteHandler) webhooks(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	p := req.Parsed

	rKey, _ := phpBodyArg(p, "repository")
	if !domain.PHPTruthy(rKey) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoRead); err != nil {
		return nil, err
	}

	list, users, err := h.Repos.ListWebhooks(ctx, rKey)
	if err != nil {
		return nil, err
	}
	out := make([]domain.RepositoryWebhookView, 0, len(list))
	for i := range list {
		out = append(out, domain.NormalizeRepositoryWebhook(&list[i], users[list[i].Key]))
	}
	return out, nil
}

// editWebhook 实现 POST /api/repository/editWebhook。
//
// 逐字对应 application/controllers/api/repository.php:2073-2121：
//
//  1. 缺 url / events / active 非法 → 0x0201
//  2. 无编辑权限 → 0x0106
//  3. 带 rwKey 则更新（WEBHOOK_UPDATE 事件），否则新建（WEBHOOK_CREATE 事件）
//  4. 写入失败 → 0x0405
//
// 【有意差异 —— 安全修复】PHP 侧不校验 rwKey 是否属于该仓库，
// 于是"编辑 A 仓库的 webhook"只要传 B 仓库的 rwKey 就能改到别人的配置。
// 这里限定 rwKey 必须属于 rKey，否则 0x0405。
//
// rw_secret 保持**明文**存储（PHP 侧的既有行为，前端要回显配置），
// 但不要新增其它明文密钥。
func (h *RepositoryWriteHandler) editWebhook(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	rKey, _ := phpBodyArg(p, "repository")
	rwKey, _ := phpBodyArg(p, "rwKey")
	url, _ := phpBodyArg(p, "url")
	events, _ := phpBodyArg(p, "events")
	activeRaw, activePresent := phpBodyArg(p, "active")

	active := 0
	if activePresent {
		active = atoiOrZero(activeRaw)
	}
	if !domain.PHPTruthy(url) || !domain.PHPTruthy(events) ||
		(active != 1 && active != 2) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoWebhookEdit); err != nil {
		return nil, err
	}

	// PHP 侧 rw_secret 是 NOT NULL 列，缺参数时写入 NULL → MySQL 报错 → HTTP 500。
	secret, hasSecret := phpBodyArg(p, "secret")
	if !hasSecret {
		return nil, phpFatalError()
	}

	fields := map[string]any{
		"rw_url":    url,
		"rw_secret": secret,
		"rw_events": events,
		"rw_active": active,
	}

	eventType := domain.EventTypeWEBHOOK_CREATE
	eventDesc := domain.EventDataDescWEBHOOK_CREATE_D

	if domain.PHPTruthy(rwKey) {
		existing, _, err := h.Repos.GetWebhook(ctx, rwKey)
		if err != nil {
			return nil, err
		}
		if existing != nil && existing.RKey != rKey {
			// rwKey 属于别的仓库 → 拒绝（PHP 侧无此校验，是越权缺口）。
			return nil, contract.NewError(contract.CodeOperateFail)
		}
		eventType = domain.EventTypeWEBHOOK_UPDATE
		eventDesc = domain.EventDataDescWEBHOOK_UPDATE_D
	}

	resultKey := rwKey
	if err := h.Repos.WithTx(ctx, func(tx repo.RepositoryTx, raw *sql.Tx) error {
		if domain.PHPTruthy(rwKey) {
			if _, err := tx.UpdateWebhook(ctx, rwKey, fields); err != nil {
				return err
			}
		} else {
			created, err := tx.CreateWebhook(ctx, &domain.RepositoryWebhook{
				RKey:   rKey,
				UKey:   uKey,
				URL:    url,
				Secret: secret,
				Events: events,
				Active: active,
			})
			if err != nil {
				return err
			}
			resultKey = created
		}
		current, err := h.repository(ctx, rKey)
		if err != nil {
			return err
		}
		if current == nil {
			return contract.NewError(contract.CodeActionNotAllowed)
		}
		return h.emit(ctx, raw, eventType, eventDesc, map[string]any{
			"gKey":  current.GroupKey,
			"rKey":  rKey,
			"rwKey": resultKey,
		}, uKey)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return []any{}, nil
}

// deleteWebhook 实现 POST /api/repository/deleteWebhook。
//
// 逐字对应 application/controllers/api/repository.php:2123-2154：
//
//  1. 缺 repository / rwKey → 0x0201
//  2. 无删除权限 → 0x0106
//  3. 删除失败 → 0x0405
//  4. 一并删除该 webhook 的待投递事件与投递日志
//
// 【有意差异 —— 安全修复】与 editWebhook 相同：rwKey 必须属于 rKey。
func (h *RepositoryWriteHandler) deleteWebhook(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	rKey, _ := phpBodyArg(p, "repository")
	rwKey, _ := phpBodyArg(p, "rwKey")

	if !domain.PHPTruthy(rKey) || !domain.PHPTruthy(rwKey) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoWebhookRemove); err != nil {
		return nil, err
	}

	existing, _, err := h.Repos.GetWebhook(ctx, rwKey)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.RKey != rKey {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	if err := h.Repos.WithTx(ctx, func(tx repo.RepositoryTx, raw *sql.Tx) error {
		if _, err := tx.DeleteWebhook(ctx, rwKey); err != nil {
			return err
		}
		if err := tx.DeleteWebhookEventsByRwKey(ctx, rwKey); err != nil {
			return err
		}
		if err := tx.DeleteWebhookLogsByRwKey(ctx, rwKey); err != nil {
			return err
		}
		current, err := h.repository(ctx, rKey)
		if err != nil {
			return err
		}
		if current == nil {
			return contract.NewError(contract.CodeActionNotAllowed)
		}
		return h.emit(ctx, raw, domain.EventTypeWEBHOOK_DELETE, domain.EventDataDescWEBHOOK_DELETE_D, map[string]any{
			"gKey":  current.GroupKey,
			"rKey":  rKey,
			"rwKey": rwKey,
		}, uKey)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return []any{}, nil
}

// getRepositoryWebhookLogs 实现 POST /api/repository/getRepositoryWebhookLogs。
//
// 逐字对应 application/controllers/api/repository.php:2156-2169：
// 缺 webhook → 0x0201；返回该 webhook 的投递日志列表。
//
// 【有意差异 —— 安全修复】PHP 侧**没有任何权限校验**，知道 rwKey 就能读
// 别人的投递日志。这里补上仓库读权限校验（先按 rwKey 反查仓库）。
func (h *RepositoryWriteHandler) getRepositoryWebhookLogs(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	p := req.Parsed

	rwKey, _ := phpBodyArg(p, "webhook")
	if !domain.PHPTruthy(rwKey) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	webhook, _, err := h.Repos.GetWebhook(ctx, rwKey)
	if err != nil {
		return nil, err
	}
	if webhook != nil {
		if err := h.Guard.RequireRepository(ctx, webhook.RKey, uKey, domain.UACRepoRead); err != nil {
			return nil, err
		}
	}

	logs, err := h.Repos.ListWebhookLogs(ctx, rwKey)
	if err != nil {
		return nil, err
	}
	return domain.NormalizeRepositoryWebhookLogs(logs), nil
}

// getRepositoryWebhookLogData 实现 POST /api/repository/getRepositoryWebhookLogData。
//
// 逐字对应 application/controllers/api/repository.php:2171-2184：
// 缺 id → 0x0201；返回 {request, response}。
//
// 【有意差异 —— 安全修复】与 getRepositoryWebhookLogs 相同：补上仓库读权限校验。
func (h *RepositoryWriteHandler) getRepositoryWebhookLogData(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	p := req.Parsed

	id, _ := phpBodyArg(p, "id")
	if !domain.PHPTruthy(id) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	log, err := h.Repos.GetWebhookLogData(ctx, id)
	if err != nil {
		return nil, err
	}
	if log != nil {
		webhook, _, err := h.Repos.GetWebhook(ctx, log.Webhook)
		if err != nil {
			return nil, err
		}
		if webhook != nil {
			if err := h.Guard.RequireRepository(ctx, webhook.RKey, uKey, domain.UACRepoRead); err != nil {
				return nil, err
			}
		}
	}

	data := domain.RepositoryWebhookLogData{Request: nil, Response: nil}
	if log != nil {
		data.Request = decodeJSONOrNil(log.Request)
		data.Response = decodeJSONOrNil(log.Response)
	}
	return data, nil
}

// decodeJSONOrNil 解析 JSON 文本；非法或空串返回 nil。
//
// 逐字对应 PHP 侧 json_decode() 的返回值语义（非对象 → null）。
func decodeJSONOrNil(raw string) any {
	if raw == "" {
		return nil
	}
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return nil
	}
	return v
}

// ---------------------------------------------------------------------------
// 合并请求
// ---------------------------------------------------------------------------

// createMergeRequest 实现 POST /api/repository/createMergeRequest。
//
// 逐字对应 application/controllers/api/repository.php:1489-1587：
//
//  1. 缺源/目标仓库或分支 → 0x0201
//  2. 无目标仓库的 MR 推送权限 → 0x0106
//  3. 同一对分支已有待合并的 MR → 0x0406
//  4. 目标分支受保护 → 0x040F
//  5. 源相对目标没有新提交 → 0x040C
//  6. 创建失败 → 0x040C
//  7. 可选：逐个指派评审人并发事件
//  8. 返回 {"id": mrId}
func (h *RepositoryWriteHandler) createMergeRequest(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	sourceKey := domain.PHPTrim(bodyOrEmpty(p, "sourceRepository"))
	targetKey := domain.PHPTrim(bodyOrEmpty(p, "targetRepository"))
	sourceBranch := domain.PHPTrim(bodyOrEmpty(p, "sourceBranch"))
	targetBranch := domain.PHPTrim(bodyOrEmpty(p, "targetBranch"))
	title := domain.PHPTrim(bodyOrEmpty(p, "title"))
	description := domain.PHPTrim(bodyOrEmpty(p, "description"))
	reviewers := domain.PHPTrim(bodyOrEmpty(p, "reviewers"))

	if !domain.PHPTruthy(sourceKey) || !domain.PHPTruthy(targetKey) ||
		!domain.PHPTruthy(sourceBranch) || !domain.PHPTruthy(targetBranch) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireRepository(ctx, targetKey, uKey, domain.UACMRPush); err != nil {
		return nil, err
	}

	exists, err := h.Repos.MergeRequestExists(ctx, sourceKey, sourceBranch, targetKey, targetBranch)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, contract.NewError(contract.CodeTargetExists)
	}

	if err := h.branchProtected(ctx, targetKey, targetBranch, uKey); err != nil {
		return nil, err
	}

	sourcePath, err := h.repoPath(ctx, sourceKey)
	if err != nil {
		return nil, err
	}
	targetPath, err := h.repoPath(ctx, targetKey)
	if err != nil {
		return nil, err
	}

	sourceSHA, targetSHA, err := h.mergeRequestHeads(ctx, sourceKey, sourceBranch, targetKey, targetBranch)
	if err != nil {
		return nil, err
	}

	// 源相对目标必须有新提交，否则 0x040C。
	commits, err := h.commitsSince(ctx, sourceKey, sourcePath, sourceSHA, targetKey, targetPath, targetSHA)
	if err != nil {
		return nil, err
	}
	if len(commits) == 0 {
		return nil, contract.NewError(contract.CodeOldTargetNotExists)
	}

	target, err := h.repository(ctx, targetKey)
	if err != nil {
		return nil, err
	}
	source, err := h.repository(ctx, sourceKey)
	if err != nil {
		return nil, err
	}
	if target == nil || source == nil {
		return nil, contract.NewError(contract.CodeActionNotAllowed)
	}

	mrKey := contract.NewKey()
	mrID, err := h.Repos.NextMergeRequestID(ctx, targetKey)
	if err != nil {
		return nil, err
	}

	newMR := &domain.RepositoryMergeRequest{
		Key:           mrKey,
		UKey:          uKey,
		SourceRKey:    sourceKey,
		SourceBranch:  sourceBranch,
		TargetRKey:    targetKey,
		TargetBranch:  targetBranch,
		StartHash:     sourceSHA,
		ID:            mrID,
		Title:         title,
		Description:   &description,
		ProceedStatus: domain.MergeRequestStatusOpen,
	}

	if err := h.Repos.WithTx(ctx, func(tx repo.RepositoryTx, raw *sql.Tx) error {
		if err := tx.CreateMergeRequest(ctx, newMR); err != nil {
			return err
		}
		if err := h.emit(ctx, raw, domain.EventTypeMERGE_REQUEST_CREATE, domain.EventDataDescMERGE_REQUEST_CREATE_D, map[string]any{
			"gKey":         target.GroupKey,
			"rKey":         targetKey,
			"id":           mrID,
			"mrKey":        mrKey,
			"sourceGKey":   source.GroupKey,
			"sourceRKey":   sourceKey,
			"sourceBranch": sourceBranch,
			"targetBranch": targetBranch,
		}, uKey); err != nil {
			return err
		}
		if reviewers == "" {
			return nil
		}
		for _, reviewer := range strings.Split(reviewers, ",") {
			reviewer = strings.TrimSpace(reviewer)
			if reviewer == "" {
				continue
			}
			mrrKey, err := tx.AssignReviewer(ctx, mrKey, reviewer)
			if err != nil {
				return err
			}
			if mrrKey == "" {
				continue
			}
			if err := h.emit(ctx, raw, domain.EventTypeMERGE_REQUEST_REVIEWER_CREATE, domain.EventDataDescMERGE_REQUEST_REVIEWER_CREATE_D, map[string]any{
				"gKey":     target.GroupKey,
				"rKey":     targetKey,
				"id":       mrID,
				"mrKey":    mrKey,
				"reviewer": reviewer,
				"mrrKey":   mrrKey,
			}, uKey); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, contract.NewError(contract.CodeOldTargetNotExists)
	}

	return map[string]any{"id": mrID}, nil
}

// mergeRequestHeads 取源分支与目标分支的最后一个提交哈希。
//
// 逐字对应 PHP 侧 getMergeRequestLastCommitSha() 的 else 分支
// （application/models/repository_model.php:1275-1283）。
func (h *RepositoryWriteHandler) mergeRequestHeads(ctx context.Context, sourceKey, sourceBranch, targetKey, targetBranch string) (string, string, error) {
	sourcePath, err := h.repoPath(ctx, sourceKey)
	if err != nil {
		return "", "", err
	}
	targetPath, err := h.repoPath(ctx, targetKey)
	if err != nil {
		return "", "", err
	}
	sourceCommit, err := h.Git.LastCommit(ctx, sourcePath, sourceBranch, "")
	if err != nil {
		return "", "", err
	}
	targetCommit, err := h.Git.LastCommit(ctx, targetPath, targetBranch, "")
	if err != nil {
		return "", "", err
	}
	if sourceCommit == nil || targetCommit == nil {
		return "", "", contract.NewError(contract.CodeOldTargetNotExists)
	}
	return sourceCommit.SHA, targetCommit.SHA, nil
}

// commitsSince 列出源仓库相对目标仓库多出来的提交。
//
// 同仓库时用 `log --cherry-pick --right-only <target>...<source>`
// （逐字对应 PHP 侧 getCommitWithLocal()：它把两个哈希**反序**传给
// getCommitListFromLocal()，application/models/repository_model.php:1895）；
// 跨仓库时用临时工作区 fetch 目标仓库后
// `log --cherry-pick --left-only <source>...<target>`
// （逐字对应 PHP 侧 getCommitWithRemote()）。
func (h *RepositoryWriteHandler) commitsSince(ctx context.Context, sourceKey, sourcePath, sourceSHA, targetKey, targetPath, targetSHA string) ([]domain.Commit, error) {
	if sourceKey == targetKey {
		return h.Git.CommitListFromLocal(ctx, sourcePath, targetSHA, sourceSHA)
	}
	return h.Git.CommitsSinceRemote(ctx, sourcePath, sourceSHA, targetPath, targetSHA)
}

// branchProtected 判断目标分支是否受保护（PHP 的 checkBranchProteced）。
//
// 逐字对应 application/models/repository_model.php:2039-2079，
// 复用 domain.CheckBranchProtected（该函数已按同一语义实现）。
func (h *RepositoryWriteHandler) branchProtected(ctx context.Context, rKey, ref, uKey string) error {
	role, err := h.Guard.RepositoryRole(ctx, rKey, uKey)
	if err != nil {
		return err
	}
	rules, err := h.Repos.ListProtectedBranchRules(ctx, rKey)
	if err != nil {
		return err
	}
	if domain.CheckBranchProtected(role, rules, ref, true) {
		return contract.NewError(contract.CodeBranchProtected)
	}
	return nil
}

// mergeRequestReview 实现 POST /api/repository/mergeRequestReview。
//
// 逐字对应 application/controllers/api/repository.php:1589-1622：
//
//  1. 缺 id → 0x0201
//  2. 无 MR 推送权限 → 0x0106
//  3. 标记失败 → 0x0405
func (h *RepositoryWriteHandler) mergeRequestReview(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	mrKey := domain.PHPTrim(bodyOrEmpty(p, "id"))
	if !domain.PHPTruthy(mrKey) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	mr, err := h.Repos.GetMergeRequestByKey(ctx, mrKey)
	if err != nil {
		return nil, err
	}
	targetKey := ""
	mrID := 0
	if mr != nil {
		targetKey = mr.TargetRKey
		mrID = mr.ID
	}
	if err := h.Guard.RequireRepository(ctx, targetKey, uKey, domain.UACMRPush); err != nil {
		return nil, err
	}

	if err := h.Repos.WithTx(ctx, func(tx repo.RepositoryTx, raw *sql.Tx) error {
		if err := tx.MergeRequestReview(ctx, mrKey, uKey); err != nil {
			return err
		}
		return h.emit(ctx, raw, domain.EventTypeMERGE_REQUEST_REVIEWER_REVIEW, domain.EventDataDescMERGE_REQUEST_REVIEWER_REVIEW_D, map[string]any{
			"gKey":     h.groupKeyOf(ctx, targetKey),
			"rKey":     targetKey,
			"id":       mrID,
			"mrKey":    mrKey,
			"reviewer": uKey,
		}, uKey)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return []any{}, nil
}

// assignReviewer 实现 POST /api/repository/assignReviewer。
//
// 逐字对应 application/controllers/api/repository.php:1624-1660：
//
//  1. 缺 id / reviewer → 0x0201
//  2. 无 MR 合并权限 → 0x0106
//  3. 指派失败 → 0x0405
func (h *RepositoryWriteHandler) assignReviewer(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	mrKey := domain.PHPTrim(bodyOrEmpty(p, "id"))
	reviewer := domain.PHPTrim(bodyOrEmpty(p, "reviewer"))

	if !domain.PHPTruthy(mrKey) || !domain.PHPTruthy(reviewer) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	mr, err := h.Repos.GetMergeRequestByKey(ctx, mrKey)
	if err != nil {
		return nil, err
	}
	targetKey := ""
	mrID := 0
	if mr != nil {
		targetKey = mr.TargetRKey
		mrID = mr.ID
	}
	if err := h.Guard.RequireRepository(ctx, targetKey, uKey, domain.UACMRMerge); err != nil {
		return nil, err
	}

	var mrrKey string
	if err := h.Repos.WithTx(ctx, func(tx repo.RepositoryTx, raw *sql.Tx) error {
		created, err := tx.AssignReviewer(ctx, mrKey, reviewer)
		if err != nil {
			return err
		}
		if created == "" {
			// 已评审过 → PHP 侧 assignReviewer() 返回 FALSE → 0x0405。
			return contract.NewError(contract.CodeOperateFail)
		}
		mrrKey = created
		return h.emit(ctx, raw, domain.EventTypeMERGE_REQUEST_REVIEWER_CREATE, domain.EventDataDescMERGE_REQUEST_REVIEWER_CREATE_D, map[string]any{
			"gKey":     h.groupKeyOf(ctx, targetKey),
			"rKey":     targetKey,
			"id":       mrID,
			"mrKey":    mrKey,
			"reviewer": reviewer,
			"mrrKey":   mrrKey,
		}, uKey)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return []any{}, nil
}

// deleteReviewer 实现 POST /api/repository/deleteReviewer。
//
// 逐字对应 application/controllers/api/repository.php:1662-1696：
//
//  1. 缺 id → 0x0201
//  2. 无 MR 合并权限 → 0x0106
//  3. 删除失败 → 0x0405
func (h *RepositoryWriteHandler) deleteReviewer(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	mrKey := domain.PHPTrim(bodyOrEmpty(p, "id"))
	reviewer := domain.PHPTrim(bodyOrEmpty(p, "reviewer"))

	if !domain.PHPTruthy(mrKey) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	mr, err := h.Repos.GetMergeRequestByKey(ctx, mrKey)
	if err != nil {
		return nil, err
	}
	targetKey := ""
	mrID := 0
	if mr != nil {
		targetKey = mr.TargetRKey
		mrID = mr.ID
	}
	if err := h.Guard.RequireRepository(ctx, targetKey, uKey, domain.UACMRMerge); err != nil {
		return nil, err
	}

	if err := h.Repos.WithTx(ctx, func(tx repo.RepositoryTx, raw *sql.Tx) error {
		if err := tx.DeleteReviewer(ctx, mrKey, reviewer); err != nil {
			return err
		}
		return h.emit(ctx, raw, domain.EventTypeMERGE_REQUEST_REVIEWER_DELETE, domain.EventDataDescMERGE_REQUEST_REVIEWER_DELETE_D, map[string]any{
			"gKey":     h.groupKeyOf(ctx, targetKey),
			"rKey":     targetKey,
			"id":       mrID,
			"mrKey":    mrKey,
			"reviewer": reviewer,
		}, uKey)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return []any{}, nil
}

// mergeRequestClose 实现 POST /api/repository/mergeRequestClose。
//
// 逐字对应 application/controllers/api/repository.php:1331-1372：
//
//  1. 缺 mergeRequest → 0x0201
//  2. MR 不存在或不是待合并状态 → 0x0201
//  3. 无 MR 关闭权限 → 0x0106
//  4. 关闭失败 → 0x0201
func (h *RepositoryWriteHandler) mergeRequestClose(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	mrKey := domain.PHPTrim(bodyOrEmpty(p, "mergeRequest"))
	if !domain.PHPTruthy(mrKey) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	mr, err := h.Repos.GetMergeRequestByKey(ctx, mrKey)
	if err != nil {
		return nil, err
	}
	if mr == nil || !domain.MergeRequestCanTransition(mr.ProceedStatus, domain.MergeRequestStatusClosed) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireRepository(ctx, mr.TargetRKey, uKey, domain.UACMRClose); err != nil {
		return nil, err
	}

	sourceSHA, targetSHA, err := h.mergeRequestHeads(ctx, mr.SourceRKey, mr.SourceBranch, mr.TargetRKey, mr.TargetBranch)
	if err != nil {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	if err := h.Repos.WithTx(ctx, func(tx repo.RepositoryTx, raw *sql.Tx) error {
		if err := tx.MergeRequestClose(ctx, uKey, mrKey, sourceSHA, targetSHA); err != nil {
			return err
		}
		sourceGroup := h.groupKeyOf(ctx, mr.SourceRKey)
		return h.emit(ctx, raw, domain.EventTypeMERGE_REQUEST_CLOSE, domain.EventDataDescMERGE_REQUEST_CLOSE_D, map[string]any{
			"gKey":         h.groupKeyOf(ctx, mr.TargetRKey),
			"rKey":         mr.TargetRKey,
			"id":           mr.ID,
			"mrKey":        mrKey,
			"sourceGKey":   sourceGroup,
			"sourceRKey":   mr.SourceRKey,
			"sourceBranch": mr.SourceBranch,
			"targetBranch": mr.TargetBranch,
		}, uKey)
	}); err != nil {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	return []any{}, nil
}

// checkMergeType 实现 POST /api/repository/checkMergeType。
//
// 逐字对应 application/controllers/api/repository.php:1698-1732：
//
//  1. 缺 mergeRequest → 0x0201
//  2. MR 不存在或不是待合并状态 → 0x0201
//  3. 无 MR 合并权限 → 0x0106
//  4. 返回 {"canBeFastForward": bool}
func (h *RepositoryWriteHandler) checkMergeType(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	mrKey := domain.PHPTrim(bodyOrEmpty(p, "mergeRequest"))
	if !domain.PHPTruthy(mrKey) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	mr, err := h.Repos.GetMergeRequestByKey(ctx, mrKey)
	if err != nil {
		return nil, err
	}
	if mr == nil || !domain.MergeRequestCanTransition(mr.ProceedStatus, domain.MergeRequestStatusMerged) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireRepository(ctx, mr.TargetRKey, uKey, domain.UACMRMerge); err != nil {
		return nil, err
	}

	sourcePath, err := h.repoPath(ctx, mr.SourceRKey)
	if err != nil {
		return nil, err
	}
	targetPath, err := h.repoPath(ctx, mr.TargetRKey)
	if err != nil {
		return nil, err
	}
	sourceSHA, targetSHA, err := h.mergeRequestHeads(ctx, mr.SourceRKey, mr.SourceBranch, mr.TargetRKey, mr.TargetBranch)
	if err != nil {
		return nil, err
	}

	fastForward, err := h.Git.CanFastForward(ctx, sourcePath, sourceSHA, targetPath, targetSHA)
	if err != nil {
		return nil, err
	}
	return map[string]any{"canBeFastForward": fastForward}, nil
}

// mergeBranch 实现 POST /api/repository/mergeBranch。
//
// 逐字对应 application/controllers/api/repository.php:1734-1808：
//
//  1. 缺 mergeRequest → 0x0201
//  2. MR 不存在或不是待合并状态 → 0x0201
//  3. 无 MR 合并权限 → 0x0106
//  4. 目标分支受保护 → 0x040F
//  5. 还有评审人未评审 → 0x0405
//  6. 合并冲突 → {"conflict": true}；成功 → 标记已合并并发事件；其它失败 → 0x0405
func (h *RepositoryWriteHandler) mergeBranch(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	mrKey := domain.PHPTrim(bodyOrEmpty(p, "mergeRequest"))
	message := domain.PHPTrim(bodyOrEmpty(p, "message"))
	// PHP 侧：trim($data['usingSquash']) === 'false' 才用非 squash 模式，
	// 其余（含缺省）一律走 squash。
	usingSquash := domain.PHPTrim(bodyOrEmpty(p, "usingSquash")) != "false"

	if !domain.PHPTruthy(mrKey) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	mr, err := h.Repos.GetMergeRequestByKey(ctx, mrKey)
	if err != nil {
		return nil, err
	}
	if mr == nil || !domain.MergeRequestCanTransition(mr.ProceedStatus, domain.MergeRequestStatusMerged) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireRepository(ctx, mr.TargetRKey, uKey, domain.UACMRMerge); err != nil {
		return nil, err
	}
	if err := h.branchProtected(ctx, mr.TargetRKey, mr.TargetBranch, uKey); err != nil {
		return nil, err
	}

	pending, err := h.Repos.ReviewPending(ctx, mrKey)
	if err != nil {
		return nil, err
	}
	if pending {
		// 还有评审人没评审 → 0x0405。
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	sourcePath, err := h.repoPath(ctx, mr.SourceRKey)
	if err != nil {
		return nil, err
	}
	targetPath, err := h.repoPath(ctx, mr.TargetRKey)
	if err != nil {
		return nil, err
	}
	sourceSHA, targetSHA, err := h.mergeRequestHeads(ctx, mr.SourceRKey, mr.SourceBranch, mr.TargetRKey, mr.TargetBranch)
	if err != nil {
		return nil, err
	}

	name, email := h.committer(ctx, uKey)
	result, err := h.Git.MergeBranch(ctx, sourcePath, mr.SourceBranch, targetPath, mr.TargetBranch,
		message, name, email, usingSquash)
	if err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	if result != nil && result.Conflict {
		return map[string]any{"conflict": true}, nil
	}

	if err := h.Repos.WithTx(ctx, func(tx repo.RepositoryTx, raw *sql.Tx) error {
		if err := tx.MergeRequestMerge(ctx, uKey, mrKey, sourceSHA, targetSHA); err != nil {
			return err
		}
		return h.emit(ctx, raw, domain.EventTypeMERGE_REQUEST_MERGE, domain.EventDataDescMERGE_REQUEST_MERGE_D, map[string]any{
			"gKey":         h.groupKeyOf(ctx, mr.TargetRKey),
			"rKey":         mr.TargetRKey,
			"id":           mr.ID,
			"mrKey":        mrKey,
			"sourceGKey":   h.groupKeyOf(ctx, mr.SourceRKey),
			"sourceRKey":   mr.SourceRKey,
			"sourceBranch": mr.SourceBranch,
			"targetBranch": mr.TargetBranch,
		}, uKey)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return []any{}, nil
}

// groupKeyOf 读取仓库所属的仓库组主键（事件数据需要）。
func (h *RepositoryWriteHandler) groupKeyOf(ctx context.Context, rKey string) string {
	if rKey == "" {
		return ""
	}
	r, err := h.repository(ctx, rKey)
	if err != nil || r == nil {
		return ""
	}
	return r.GroupKey
}

// ---------------------------------------------------------------------------
// 小工具
// ---------------------------------------------------------------------------

// bodyOrEmpty 读取 body / JSON 参数，缺失时返回空串。
func bodyOrEmpty(p *contract.Parsed, key string) string {
	v, _ := phpBodyArg(p, key)
	return v
}

// atoiOrZero 把字符串转成整数，失败时返回 0（PHP 的 (int) 转换）。
func atoiOrZero(raw string) int {
	n := 0
	neg := false
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if i == 0 && (c == '-' || c == '+') {
			neg = c == '-'
			continue
		}
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	if neg {
		return -n
	}
	return n
}

// nowString 返回当前时间的 MySQL 字符串形态。
//
// 逐字对应 PHP 侧 date('Y-m-d H:i:s')。
func nowString() string {
	return time.Now().Format("2006-01-02 15:04:05")
}
