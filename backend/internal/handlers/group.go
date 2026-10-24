// 仓库组（namespace）域的 API 端点。
//
// 每个端点都标注了对应的 PHP 源码位置，便于逐条比对契约：
// application/controllers/api/group.php。
package handlers

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"codefever-next/backend/internal/config"
	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
	"codefever-next/backend/internal/httpapi"
	"codefever-next/backend/internal/repo"
)

// GroupHandler 实现 /api/group/* 端点。
type GroupHandler struct {
	// Groups 是仓库组仓储：只读查询 + 事务内写入。
	//
	// 抽象成接口是为了让单元测试能替换成假的实现 —— 事件必须与业务写入
	// 在**同一个事务**里入队，因此事务要能同时交出"事务内仓储"与"事务对象"。
	Groups GroupStore
	// Guard 是仓库组权限守卫。
	Guard GroupGuard
	// Users 用于读取成员与创建者的用户信息。
	Users GroupUsers
	// Cfg 提供站点 host / ssh（用户归一化形态需要）与文件存储根目录。
	Cfg *config.Config
	// Bus 是事件总线；事件与业务写入在同一个事务里入队。
	Bus EventBus
	// Git 用于合并请求列表里的 last commit（可选）。
	Git GroupGit
	// RepoPaths 由仓库主键查 r_path（可选）。
	RepoPaths GroupRepoPaths
}

// GroupGit 是仓库组域需要的 git 只读操作。
type GroupGit interface {
	RepoPath(rPath string) (string, error)
	LastCommit(ctx context.Context, repoPath, ref, path string, lastSHA ...string) (*domain.Commit, error)
}

// GroupRepoPaths 由仓库主键查裸仓库相对路径。
type GroupRepoPaths interface {
	RepositoryPath(ctx context.Context, rKey string) (string, error)
}

// GroupOption 是 GroupHandler 的可选依赖。
type GroupOption func(*GroupHandler)

// WithGroupGit 注入 git 服务与仓库路径查询。
//
// 合并请求列表的 commit 字段需要查源仓库最后一次提交；
// 不注入时该字段保持空值（退化为旧行为）。
func WithGroupGit(git GroupGit, paths GroupRepoPaths) GroupOption {
	return func(h *GroupHandler) {
		h.Git = git
		h.RepoPaths = paths
	}
}

// WithGroupUserRepo 注入用户仓储。
func WithGroupUserRepo(users *repo.UserRepo) GroupOption {
	return func(h *GroupHandler) { h.Users = users }
}

// WithGroupSiteConfig 注入站点配置。
func WithGroupSiteConfig(cfg *config.Config) GroupOption {
	return func(h *GroupHandler) { h.Cfg = cfg }
}

// WithGroupEventBus 注入事件总线。
//
// 事件与业务写入在**同一个事务**里入队（cc_events 表），由后台 worker
// 异步消费，从而保证"业务提交则事件在、业务回滚则事件消失"。
//
// 参数是 EventBus 接口而不是具体的 *events.Bus，便于单元测试注入假实现；
// *events.Bus 天然满足该接口。
func WithGroupEventBus(bus EventBus) GroupOption {
	return func(h *GroupHandler) { h.Bus = bus }
}

// NewGroupHandler 创建仓库组处理器。
//
// 用户仓储与站点配置通过 Option 注入：成员列表需要用户归一化形态，
// 其中 host / ssh 两个字段来自站点配置（PHP 侧的 YAML_HOST / YAML_SSH）。
//
// 事件总线同样通过 Option 注入（WithGroupEventBus）：没注入时写端点照常
// 工作，只是不产生事件。
func NewGroupHandler(groups *repo.GroupRepo, guard *Guard, opts ...GroupOption) *GroupHandler {
	h := &GroupHandler{Groups: newRepoGroupStore(groups), Guard: guard}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// Register 把仓库组端点注册到服务器。
//
// 逐字对应 application/controllers/api/group.php 的 13 个公开方法，
// 动作名与 PHP 的 <action>_<method> 前缀保持一致。
func (h *GroupHandler) Register(s *httpapi.Server) {
	actions := map[string]map[string]httpapi.ActionHandler{
		"list":             {"GET": h.list},
		"create":           {"POST": h.create},
		"config":           {"GET": h.config},
		"mergeRequests":    {"GET": h.mergeRequests},
		"activities":       {"GET": h.activities},
		"uploadAvatar":     {"POST": h.uploadAvatar},
		"update":           {"POST": h.update},
		"addMember":        {"POST": h.addMember},
		"changeMemberRole": {"POST": h.changeMemberRole},
		"removeMember":     {"POST": h.removeMember},
		"changeOwner":      {"POST": h.changeOwner},
		"updateName":       {"POST": h.updateName},
		"deleteGroup":      {"POST": h.deleteGroup},
	}

	s.Register(&httpapi.Controller{
		Name:    "group",
		Auth:    httpapi.AuthWebUser,
		Actions: actions,
	})
}

// groupListItem 是 group/list 的单条记录：仓库组归一化形态 +
// 当前用户在该组的角色 + 成员列表。
type groupListItem struct {
	domain.NormalizedGroup
	Role    int                            `json:"role"`
	Members []domain.NormalizedGroupMember `json:"members"`
}

// groupConfig 是 group/config 的响应体。
type groupConfig struct {
	Group   domain.NormalizedGroup         `json:"group"`
	Members []domain.NormalizedGroupMember `json:"members"`
	Count   groupCount                     `json:"count"`
}

// groupCount 是 group/config 的计数字段。
type groupCount struct {
	Member int `json:"member"`
}

// list 实现 GET /api/group/list。
//
// 逐字对应 application/controllers/api/group.php:22-40：
//  1. 取"我创建的组"与"我加入的组"，按 g_key 去重
//  2. 逐个归一化，补上我在该组的角色（role）与成员列表（members）
func (h *GroupHandler) list(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}

	owned, err := h.Groups.ListOwned(ctx, u.Key)
	if err != nil {
		return nil, err
	}
	joined, err := h.Groups.ListJoined(ctx, u.Key)
	if err != nil {
		return nil, err
	}
	groups := uniqueGroups(append(owned, joined...))

	cache := newMemberCache(h.Users)
	out := make([]any, 0, len(groups))
	for i := range groups {
		g := &groups[i]

		role, err := h.Guard.GroupRole(ctx, g.Key, u.Key)
		if err != nil {
			return nil, err
		}
		members, err := h.groupMembers(ctx, g, cache)
		if err != nil {
			return nil, err
		}
		out = append(out, groupListItem{
			NormalizedGroup: domain.NormalizeGroup(g),
			Role:            int(role),
			Members:         members,
		})
	}
	return out, nil
}

// create 实现 POST /api/group/create。
//
// 逐字对应 application/controllers/api/group.php:42-90：
//  1. displayName / name / type 缺一 → 0x0201
//  2. 名称重复 → 0x0403；显示名重复 → 0x0404
//  3. 名称命中保留 URI → 0x0403
//  4. 建组 → 失败 0x0405
//  5. 返回新建仓库组的归一化形态
func (h *GroupHandler) create(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	displayName, _ := phpBodyArg(p, "displayName")
	name, _ := phpBodyArg(p, "name")
	typeRaw, typePresent := phpBodyArg(p, "type")

	if !domain.PHPTruthy(displayName) || !domain.PHPTruthy(name) ||
		!domain.GroupTypeAllowed(typeRaw, typePresent) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	if _, err := h.Groups.SearchByName(ctx, name); err == nil {
		return nil, contract.NewError(contract.CodeDuplicateGroupSlug)
	} else if !errors.Is(err, repo.ErrNotFound) {
		return nil, err
	}
	if _, err := h.Groups.SearchByDisplayName(ctx, displayName); err == nil {
		return nil, contract.NewError(contract.CodeDuplicateGroupDisplay)
	} else if !errors.Is(err, repo.ErrNotFound) {
		return nil, err
	}
	if domain.GroupNameIsPreserved(name) {
		return nil, contract.NewError(contract.CodeDuplicateGroupSlug)
	}

	// PHP 的 Group_model::create(string $displayName, string $name,
	// string $description, int $type, string $uKey) 收不到 description 时
	// 会拿到 null → TypeError（致命错误 → HTTP 500 空响应体）。
	description, hasDescription := phpBodyArg(p, "description")
	if !hasDescription {
		return nil, phpFatalError()
	}

	groupType, _ := domain.PHPIntArg(typeRaw, typePresent)
	g := &domain.Group{
		Key:         contract.NewKey(),
		OwnerKey:    u.Key,
		Type:        groupType,
		DisplayName: displayName,
		Name:        name,
		Description: description,
	}
	if err := h.Groups.WithTx(ctx, func(tx GroupTx, raw *sql.Tx) error {
		if err := tx.Create(ctx, g); err != nil {
			return err
		}
		// PHP 侧 application/controllers/api/group.php:79-82：
		//	newEvent('GROUP_CREATE', ['gKey' => $gKey, 'name' => $displayName], $uKey)
		//
		// 事件与建组在同一个事务里入队：事件入队失败 → 建组一起回滚。
		return h.emit(ctx, raw, groupCreateEvent(g.Key, g.DisplayName), u.Key)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	created, err := h.Groups.Get(ctx, g.Key)
	if err != nil {
		return nil, err
	}
	return domain.NormalizeGroup(created), nil
}

// config 实现 GET /api/group/config。
//
// 逐字对应 application/controllers/api/group.php:92-127：
//  1. gKey 缺失 → 0x0201
//  2. 无仓库读权限 → 0x0106
//  3. 返回仓库组、成员列表与成员数
func (h *GroupHandler) config(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	gKey := req.Parsed.QueryString("gKey")

	if u.Key == "" || gKey == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireGroup(ctx, gKey, u.Key, domain.UACRepoRead); err != nil {
		return nil, err
	}

	g, err := h.groupOrEmpty(ctx, gKey)
	if err != nil {
		return nil, err
	}
	members, err := h.groupMembers(ctx, g, newMemberCache(h.Users))
	if err != nil {
		return nil, err
	}

	return groupConfig{
		Group:   domain.NormalizeGroup(g),
		Members: members,
		Count:   groupCount{Member: len(members)},
	}, nil
}

// mergeRequests 实现 GET /api/group/mergeRequests。
//
// 逐字对应 application/controllers/api/group.php:129-161：
//  1. group 缺失 / sort 不是 asc|desc / status 非法 → 0x0201
//  2. 无合并请求读权限 → 0x0106
//  3. 返回该组下的合并请求列表
func (h *GroupHandler) mergeRequests(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	gKey := p.QueryString("group")
	sort := p.QueryString("sort")
	status := p.QueryInt("status")

	// status 的合法取值对应 PHP 的 service\Constant\MergeRequestStatus。
	if !domain.PHPTruthy(gKey) || (sort != "desc" && sort != "asc") ||
		(status != domain.MergeRequestStatusNone && status != domain.MergeRequestStatusOpen &&
			status != domain.MergeRequestStatusMerged && status != domain.MergeRequestStatusClosed) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireGroup(ctx, gKey, u.Key, domain.UACMRRead); err != nil {
		return nil, err
	}

	// PHP 的 Group_model::getMergeRequests(string $gKey, int $status,
	// string $keyword, string $sort, int $page, int $pagesize)：
	// 客户端没传的参数在 PHP 里是 null，传给带类型的形参会抛 TypeError
	// （致命错误 → HTTP 500 空响应体）。
	keyword, ok := domain.PHPStringArg(phpQueryArg(p, "keyword"))
	if !ok {
		return nil, phpFatalError()
	}
	if _, ok := domain.PHPIntArg(phpQueryArg(p, "page")); !ok {
		return nil, phpFatalError()
	}
	if _, ok := domain.PHPIntArg(phpQueryArg(p, "pagesize")); !ok {
		return nil, phpFatalError()
	}

	mrs, err := h.Groups.MergeRequests(ctx, gKey, status, keyword, sort,
		p.QueryInt("page"), p.QueryInt("pagesize"))
	if err != nil {
		return nil, err
	}

	out := make([]any, 0, len(mrs))
	for i := range mrs {
		list, err := h.Groups.MergeRequestReviewers(ctx, mrs[i].Key)
		if err != nil {
			return nil, err
		}
		reviewers := make([]domain.NormalizedReviewer, 0, len(list))
		for _, rv := range list {
			reviewers = append(reviewers, domain.NormalizedReviewer{
				ID: rv.Key, User: rv.UKey, IsReview: rv.IsReview,
			})
		}
		view := domain.NormalizeMergeRequest(&mrs[i], reviewers)
		// commit 字段需要查源仓库的最后一次提交；仓库已被删除或 git
		// 读取失败时保持空值，与 repository.mergeRequests 的降级行为一致。
		view.Commit = h.mrCommit(ctx, mrs[i].SourceRKey, mrs[i].SourceBranch)
		out = append(out, view)
	}
	return out, nil
}

// mrCommit 读取合并请求源分支的最后一次提交。
//
// PHP 侧 normalizeMergeRequests() 对每条 MR 调 getLastCommit()；Go 侧
// 同样按需查询。依赖未注入时返回零值。
func (h *GroupHandler) mrCommit(ctx context.Context, sourceRKey, sourceBranch string) domain.Commit {
	if h.Git == nil || h.RepoPaths == nil || sourceRKey == "" || sourceBranch == "" {
		return domain.Commit{}
	}
	rPath, err := h.RepoPaths.RepositoryPath(ctx, sourceRKey)
	if err != nil {
		return domain.Commit{}
	}
	repoPath, err := h.Git.RepoPath(rPath)
	if err != nil {
		return domain.Commit{}
	}
	commit, err := h.Git.LastCommit(ctx, repoPath, sourceBranch, "")
	if err != nil || commit == nil {
		return domain.Commit{}
	}
	return *commit
}

// activities 实现 GET /api/group/activities。
//
// 逐字对应 application/controllers/api/group.php:163-182：
//  1. group 缺失 / category 非法 → 0x0201
//  2. 返回该组的动态列表
//
// 注意：PHP 侧**不做任何权限校验**（docs/ARCHITECTURE_ANALYSIS.md
// 把它列为越权读取缺口之一），因此不存在的组也会返回 200 + 空数组。
func (h *GroupHandler) activities(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	gKey := p.QueryString("group")
	categoryRaw, categoryPresent := phpQueryArg(p, "category")

	if !domain.PHPTruthy(gKey) || !domain.GroupActivityCategoryAllowed(categoryRaw, categoryPresent) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	// PHP 的 Group_model::listActivities(string $gKey, int $category,
	// string $keyword, int $page, int $pagesize)：客户端没传的参数是 null，
	// 传给带类型的形参会抛 TypeError（致命错误 → HTTP 500 空响应体）。
	category, ok := domain.PHPIntArg(categoryRaw, categoryPresent)
	if !ok {
		return nil, phpFatalError()
	}
	keyword, ok := domain.PHPStringArg(phpQueryArg(p, "keyword"))
	if !ok {
		return nil, phpFatalError()
	}
	if _, ok := domain.PHPIntArg(phpQueryArg(p, "page")); !ok {
		return nil, phpFatalError()
	}
	if _, ok := domain.PHPIntArg(phpQueryArg(p, "pagesize")); !ok {
		return nil, phpFatalError()
	}

	acts, err := h.Groups.Activities(ctx, gKey, category, keyword,
		p.QueryInt("page"), p.QueryInt("pagesize"))
	if err != nil {
		return nil, err
	}

	out := make([]any, 0, len(acts))
	for i := range acts {
		out = append(out, domain.NormalizeActivity(&acts[i]))
	}
	return out, nil
}

// uploadAvatar 实现 POST /api/group/uploadAvatar。
//
// 逐字对应 application/controllers/api/group.php:184-219：
//  1. group 或头像文件缺失 → 0x0201
//  2. 无仓库组信息修改权限 → 0x0106
//  3. 保存文件并更新 g_avatar
func (h *GroupHandler) uploadAvatar(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	groupKey, _ := phpBodyArg(p, "group")
	file := p.File("avatar")

	if !domain.PHPTruthy(groupKey) || file == nil {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireGroup(ctx, groupKey, u.Key, domain.UACGroupChangeInfo); err != nil {
		return nil, err
	}

	key := contract.NewKey()
	if err := h.saveAvatar(key, file); err != nil {
		return nil, err
	}

	// PHP 侧 application/controllers/api/group.php:209-216：
	// 先读组信息拿到旧头像，再更新 g_avatar，最后发事件。
	g, err := h.groupOrEmpty(ctx, groupKey)
	if err != nil {
		return nil, err
	}
	oldAvatar := ""
	if g.Avatar != nil {
		oldAvatar = *g.Avatar
	}

	if err := h.Groups.WithTx(ctx, func(tx GroupTx, raw *sql.Tx) error {
		if err := tx.Update(ctx, groupKey, map[string]any{"g_avatar": key}); err != nil {
			return err
		}
		//	newEvent('GROUP_UPDATE_AVATAR',
		//	    ['gKey' => $gKey, 'from' => $oldAvatar, 'to' => $key], $uKey)
		return h.emit(ctx, raw, groupUpdateAvatarEvent(groupKey, oldAvatar, key), u.Key)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	return []any{}, nil
}

// update 实现 POST /api/group/update。
//
// 逐字对应 application/controllers/api/group.php:221-269：
//  1. group / displayName 缺失 → 0x0201
//  2. 无仓库组信息修改权限 → 0x0106
//  3. 显示名重复 → 0x0404
//  4. 更新显示名与描述
func (h *GroupHandler) update(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	groupKey, _ := phpBodyArg(p, "group")
	displayName, _ := phpBodyArg(p, "displayName")

	if !domain.PHPTruthy(groupKey) || !domain.PHPTruthy(displayName) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireGroup(ctx, groupKey, u.Key, domain.UACGroupChangeInfo); err != nil {
		return nil, err
	}

	g, err := h.groupOrEmpty(ctx, groupKey)
	if err != nil {
		return nil, err
	}

	// PHP 侧 g_description 是 NOT NULL 列，缺参数时会把 NULL 写进去 →
	// MySQL 报错 → CI 的 db_debug 展示错误页（HTTP 500）。
	description, hasDescription := phpBodyArg(p, "description")
	if !hasDescription {
		return nil, phpFatalError()
	}

	if g.DisplayName != displayName {
		if _, err := h.Groups.SearchByDisplayName(ctx, displayName); err == nil {
			return nil, contract.NewError(contract.CodeDuplicateGroupDisplay)
		} else if !errors.Is(err, repo.ErrNotFound) {
			return nil, err
		}
	}

	if err := h.Groups.WithTx(ctx, func(tx GroupTx, raw *sql.Tx) error {
		if err := tx.Update(ctx, groupKey, map[string]any{
			"g_display_name": displayName,
			"g_description":  description,
		}); err != nil {
			return err
		}
		// PHP 侧 application/controllers/api/group.php:254-266：
		// 显示名变了才发 GROUP_UPDATE_NAME，描述变了才发 GROUP_UPDATE_DESCRIPTION。
		if g.DisplayName != displayName {
			//	newEvent('GROUP_UPDATE_NAME',
			//	    ['gKey' => $gKey, 'from' => $oldName, 'to' => $displayName], $uKey)
			if err := h.emit(ctx, raw,
				groupUpdateNameEvent(groupKey, g.DisplayName, displayName), u.Key); err != nil {
				return err
			}
		}
		if g.Description != description {
			//	newEvent('GROUP_UPDATE_DESCRIPTION', ['gKey' => $gKey], $uKey)
			if err := h.emit(ctx, raw, groupUpdateDescriptionEvent(groupKey), u.Key); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	return []any{}, nil
}

// addMember 实现 POST /api/group/addMember。
//
// 逐字对应 application/controllers/api/group.php:271-316：
//  1. group / email 缺失 → 0x0201
//  2. 无仓库组成员管理权限 → 0x0106
//  3. 邮箱查无此人 → 0x0407
//  4. 不能把创建者加为成员 → 0x0408
//  5. 加入失败 → 0x0405
func (h *GroupHandler) addMember(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	groupKey, _ := phpBodyArg(p, "group")
	email, _ := phpBodyArg(p, "email")

	if !domain.PHPTruthy(groupKey) || !domain.PHPTruthy(email) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireGroup(ctx, groupKey, u.Key, domain.UACGroupChangeMember); err != nil {
		return nil, err
	}

	member, err := h.Users.GetByEmail(ctx, email)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return nil, err
	}
	if errors.Is(err, repo.ErrNotFound) || member == nil {
		return nil, contract.NewError(contract.CodeUserNotFound)
	}

	g, err := h.groupOrEmpty(ctx, groupKey)
	if err != nil {
		return nil, err
	}
	if g.OwnerKey == member.Key {
		return nil, contract.NewError(contract.CodeCannotAddCreator)
	}

	if err := h.Groups.WithTx(ctx, func(tx GroupTx, raw *sql.Tx) error {
		if err := tx.AddMember(ctx, member.Key, groupKey); err != nil {
			return err
		}
		// PHP 侧 application/controllers/api/group.php:308-313：
		//	newEvent('GROUP_ADD_MEMBER',
		//	    ['gKey' => $gKey, 'uid' => $uKey, 'name' => $name, 'email' => $email], $uKey)
		return h.emit(ctx, raw,
			groupAddMemberEvent(groupKey, member.Key, member.Name, member.Email), u.Key)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	return []any{}, nil
}

// changeMemberRole 实现 POST /api/group/changeMemberRole。
//
// 逐字对应 application/controllers/api/group.php:318-369：
//  1. group / userID / roleID 缺失 → 0x0201
//  2. 无仓库组成员管理权限 → 0x0106
//  3. 用户不存在 → 0x0407
//  4. 不能改创建者的角色 → 0x0106
//  5. 改角色失败 → 0x0405
func (h *GroupHandler) changeMemberRole(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	groupKey, _ := phpBodyArg(p, "group")
	userID, _ := phpBodyArg(p, "userID")
	roleRaw, rolePresent := phpBodyArg(p, "roleID")

	if !domain.PHPTruthy(groupKey) || !domain.PHPTruthy(userID) ||
		!domain.GroupMemberRoleAllowed(roleRaw, rolePresent) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireGroup(ctx, groupKey, u.Key, domain.UACGroupChangeMember); err != nil {
		return nil, err
	}

	member, err := h.Users.Get(ctx, userID)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return nil, err
	}
	if errors.Is(err, repo.ErrNotFound) || member == nil {
		return nil, contract.NewError(contract.CodeUserNotFound)
	}

	g, err := h.groupOrEmpty(ctx, groupKey)
	if err != nil {
		return nil, err
	}
	if g.OwnerKey == member.Key {
		return nil, contract.NewError(contract.CodeActionNotAllowed)
	}

	role, _ := domain.PHPIntArg(roleRaw, rolePresent)
	if err := h.Groups.WithTx(ctx, func(tx GroupTx, raw *sql.Tx) error {
		if err := tx.SetMemberRole(ctx, userID, groupKey, role); err != nil {
			return err
		}
		// PHP 侧 application/controllers/api/group.php:360-366：
		//	newEvent('GROUP_CHANGE_MEMBER_ROLE',
		//	    ['gKey' => $gKey, 'uid' => $uKey, 'name' => $name, 'email' => $email,
		//	     'to' => (int) $roleID], $uKey)
		return h.emit(ctx, raw,
			groupChangeMemberRoleEvent(groupKey, member.Key, member.Name, member.Email, role), u.Key)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	return []any{}, nil
}

// removeMember 实现 POST /api/group/removeMember。
//
// 逐字对应 application/controllers/api/group.php:371-415：
//  1. group / userID 缺失 → 0x0201
//  2. 无仓库组成员管理权限 → 0x0106
//  3. 用户不存在 → 0x0407
//  4. 不能移除创建者 → 0x0106
//  5. 移除失败 → 0x0405
func (h *GroupHandler) removeMember(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	groupKey, _ := phpBodyArg(p, "group")
	userID, _ := phpBodyArg(p, "userID")

	if !domain.PHPTruthy(groupKey) || !domain.PHPTruthy(userID) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireGroup(ctx, groupKey, u.Key, domain.UACGroupChangeMember); err != nil {
		return nil, err
	}

	member, err := h.Users.Get(ctx, userID)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return nil, err
	}
	if errors.Is(err, repo.ErrNotFound) || member == nil {
		return nil, contract.NewError(contract.CodeUserNotFound)
	}

	g, err := h.groupOrEmpty(ctx, groupKey)
	if err != nil {
		return nil, err
	}
	if g.OwnerKey == member.Key {
		return nil, contract.NewError(contract.CodeActionNotAllowed)
	}

	if err := h.Groups.WithTx(ctx, func(tx GroupTx, raw *sql.Tx) error {
		if err := tx.RemoveMember(ctx, userID, groupKey); err != nil {
			return err
		}
		// PHP 侧 application/controllers/api/group.php:407-412：
		//	newEvent('GROUP_REMOVE_MEMBER',
		//	    ['gKey' => $gKey, 'uid' => $uKey, 'name' => $name, 'email' => $email], $uKey)
		return h.emit(ctx, raw,
			groupRemoveMemberEvent(groupKey, member.Key, member.Name, member.Email), u.Key)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	return []any{}, nil
}

// changeOwner 实现 POST /api/group/changeOwner。
//
// 逐字对应 application/controllers/api/group.php:417-478：
//  1. group / userID 缺失 → 0x0201
//  2. 无仓库组转让权限 → 0x0106
//  3. 用户不存在 → 0x0407
//  4. 不能把创建者设为创建者 → 0x0106
//  5. 个人组不能转让 → 0x040B
//  6. 旧创建者降为成员、新创建者接管、移除新创建者的成员身份
//
// 注意第 6 步在 PHP 侧没有事务，中途失败会留下半成品状态；这里用事务包住。
func (h *GroupHandler) changeOwner(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	groupKey, _ := phpBodyArg(p, "group")
	userID, _ := phpBodyArg(p, "userID")

	if !domain.PHPTruthy(groupKey) || !domain.PHPTruthy(userID) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireGroup(ctx, groupKey, u.Key, domain.UACGroupChangeOwner); err != nil {
		return nil, err
	}

	member, err := h.Users.Get(ctx, userID)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return nil, err
	}
	if errors.Is(err, repo.ErrNotFound) || member == nil {
		return nil, contract.NewError(contract.CodeUserNotFound)
	}

	g, err := h.groupOrEmpty(ctx, groupKey)
	if err != nil {
		return nil, err
	}
	if g.OwnerKey == member.Key {
		return nil, contract.NewError(contract.CodeActionNotAllowed)
	}
	if g.Type == domain.GroupTypeUser {
		return nil, contract.NewError(contract.CodeCannotChangeOwnerOfGroup)
	}

	if err := h.Groups.WithTx(ctx, func(tx GroupTx, raw *sql.Tx) error {
		// 旧创建者降为 ROLE_OWNER 身份的普通成员。
		if err := tx.SetMemberRole(ctx, g.OwnerKey, groupKey, int(domain.RoleOwner)); err != nil {
			return err
		}
		// 新创建者接管仓库组。
		if err := tx.Update(ctx, groupKey, map[string]any{"u_key": userID}); err != nil {
			return err
		}
		// 移除新创建者的成员身份（他已经是创建者了）。
		if err := tx.RemoveMember(ctx, userID, groupKey); err != nil {
			return err
		}
		// PHP 侧 application/controllers/api/group.php:470-475：
		//	newEvent('GROUP_CHANGE_OWNER',
		//	    ['gKey' => $gKey, 'uid' => $uKey, 'name' => $name, 'email' => $email], $uKey)
		//
		// uid / name / email 是**新创建者**的信息。
		return h.emit(ctx, raw,
			groupChangeOwnerEvent(groupKey, member.Key, member.Name, member.Email), u.Key)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	return []any{}, nil
}

// updateName 实现 POST /api/group/updateName。
//
// 逐字对应 application/controllers/api/group.php:480-523：
//  1. group / name 缺失 → 0x0201
//  2. 无仓库组改名权限 → 0x0106
//  3. 名称重复或命中保留 URI → 0x0403
//  4. 更新 g_name
func (h *GroupHandler) updateName(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	groupKey, _ := phpBodyArg(p, "group")
	name, _ := phpBodyArg(p, "name")

	if !domain.PHPTruthy(groupKey) || !domain.PHPTruthy(name) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireGroup(ctx, groupKey, u.Key, domain.UACGroupChangeURL); err != nil {
		return nil, err
	}

	if _, err := h.Groups.SearchByName(ctx, name); err == nil {
		return nil, contract.NewError(contract.CodeDuplicateGroupSlug)
	} else if !errors.Is(err, repo.ErrNotFound) {
		return nil, err
	}
	if domain.GroupNameIsPreserved(name) {
		return nil, contract.NewError(contract.CodeDuplicateGroupSlug)
	}

	// PHP 侧 application/controllers/api/group.php:500 在更新前读组信息，
	// 事件里的 from 取的是旧名称（g_name）。
	g, err := h.groupOrEmpty(ctx, groupKey)
	if err != nil {
		return nil, err
	}

	if err := h.Groups.WithTx(ctx, func(tx GroupTx, raw *sql.Tx) error {
		if err := tx.Update(ctx, groupKey, map[string]any{"g_name": name}); err != nil {
			return err
		}
		//	newEvent('GROUP_CHANGE_URL',
		//	    ['gKey' => $gKey, 'from' => $oldName, 'to' => $name], $uKey)
		return h.emit(ctx, raw, groupChangeURLEvent(groupKey, g.Name, name), u.Key)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	return []any{}, nil
}

// deleteGroup 实现 POST /api/group/deleteGroup。
//
// 逐字对应 application/controllers/api/group.php:525-566：
//  1. group 缺失 → 0x0201
//  2. 无仓库组删除权限 → 0x0106
//  3. 个人组不能删 → 0x0409
//  4. 组内还有仓库 → 0x040A
//  5. 软删除仓库组
func (h *GroupHandler) deleteGroup(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	p := req.Parsed

	groupKey, _ := phpBodyArg(p, "group")
	if !domain.PHPTruthy(groupKey) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireGroup(ctx, groupKey, u.Key, domain.UACGroupRemove); err != nil {
		return nil, err
	}

	g, err := h.groupOrEmpty(ctx, groupKey)
	if err != nil {
		return nil, err
	}
	if g.Type == domain.GroupTypeUser {
		return nil, contract.NewError(contract.CodeCannotDeleteUserGroup)
	}

	count, err := h.Groups.RepositoriesInGroup(ctx, groupKey)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, contract.NewError(contract.CodeCannotDeleteNonEmpty)
	}

	if err := h.Groups.WithTx(ctx, func(tx GroupTx, raw *sql.Tx) error {
		if err := tx.RemoveGroup(ctx, groupKey); err != nil {
			return err
		}
		// PHP 侧 application/controllers/api/group.php:560-563：
		//	newEvent('GROUP_REMOVE', ['gKey' => $gKey, 'name' => $displayName], $uKey)
		return h.emit(ctx, raw, groupRemoveEvent(groupKey, g.DisplayName), u.Key)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	return []any{}, nil
}

// ---------------------------------------------------------------------------
// 辅助
// ---------------------------------------------------------------------------

// groupOrEmpty 读取仓库组；记录不存在时返回空结构。
//
// PHP 侧 Group_model::get() 找不到记录时返回 FALSE，后续访问数组键得到 null，
// 控制器并不会因此报错。这里用空结构复刻同样的"继续往下走"的行为。
func (h *GroupHandler) groupOrEmpty(ctx context.Context, gKey string) (*domain.Group, error) {
	g, err := h.Groups.Get(ctx, gKey)
	if errors.Is(err, repo.ErrNotFound) {
		return &domain.Group{}, nil
	}
	if err != nil {
		return nil, err
	}
	return g, nil
}

// groupMembers 组装仓库组成员列表。
//
// 逐字对应 application/controllers/api/group.php:33-37（list）
// 与 :115-120（config）：成员行归一化 + 组创建者归一化，合并后按 id 去重。
func (h *GroupHandler) groupMembers(ctx context.Context, g *domain.Group, cache *memberCache) ([]domain.NormalizedGroupMember, error) {
	members, err := h.Groups.ListMembers(ctx, g.Key)
	if err != nil {
		return nil, err
	}

	out := make([]domain.NormalizedGroupMember, 0, len(members)+1)
	for i := range members {
		m := &members[i]
		u, err := cache.user(ctx, m.UKey)
		if err != nil {
			return nil, err
		}
		emails, err := cache.commitEmails(ctx, m.UKey)
		if err != nil {
			return nil, err
		}
		out = append(out, domain.NormalizeGroupMember(m, u, emails, h.host(), h.ssh()))
	}

	owner, err := cache.user(ctx, g.OwnerKey)
	if err != nil {
		return nil, err
	}
	ownerEmails, err := cache.commitEmails(ctx, g.OwnerKey)
	if err != nil {
		return nil, err
	}
	out = append(out, domain.NormalizeGroupCreator(g, owner, ownerEmails, h.host(), h.ssh()))

	return uniqueMembersByID(out), nil
}

// host 返回站点 host（PHP 侧的 YAML_HOST）。
func (h *GroupHandler) host() string {
	if h.Cfg == nil {
		return ""
	}
	return h.Cfg.SiteConfig().Host
}

// ssh 返回站点 ssh 地址（PHP 侧的 YAML_SSH）。
func (h *GroupHandler) ssh() string {
	if h.Cfg == nil {
		return ""
	}
	return h.Cfg.SiteConfig().SSH
}

// saveAvatar 保存头像文件。
//
// 逐字对应 application/controllers/api/group.php:204-207：
// Storage::factory('avatar') → getPathByFileName($key) → moveUploadFile()。
// PHP 的存储根目录是 <base>/file-storage/avatar
// （application/libraries/storage/disk.php:12），文件名按前 5 个字符分目录。
func (h *GroupHandler) saveAvatar(key string, file *contract.UploadedFile) error {
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
	_, err = io.Copy(dst, src)
	return err
}

// avatarRelativePath 复刻 Storage_Disk::getPathByFileName()：
// 用文件名的前 5 个字符分目录。
func avatarRelativePath(key string) string {
	if len(key) < 5 {
		return key
	}
	return key[0:1] + "/" + key[1:2] + "/" + key[2:3] + "/" + key[3:4] + "/" + key[4:5] + "/" + key
}

// uniqueGroups 按 g_key 去重，保留首次出现的位置、最后一次出现的值。
//
// 逐字对应 application/libraries/service/Utility/Helper.php:49-57 的
// getUniqueItemList()：PHP 用 $output[$key] = $item 覆盖，
// 最后 array_values() 保留首次插入的顺序。
func uniqueGroups(groups []domain.Group) []domain.Group {
	index := make(map[string]int, len(groups))
	out := make([]domain.Group, 0, len(groups))
	for _, g := range groups {
		if pos, ok := index[g.Key]; ok {
			out[pos] = g
			continue
		}
		index[g.Key] = len(out)
		out = append(out, g)
	}
	return out
}

// uniqueMembersByID 按 id 去重，保留首次出现的位置、最后一次出现的值。
//
// 逐字对应 application/libraries/service/Utility/Helper.php:49-57 的
// getUniqueItemList()（控制器用的是它，不是 getUniqueMemberList）。
func uniqueMembersByID(items []domain.NormalizedGroupMember) []domain.NormalizedGroupMember {
	index := make(map[string]int, len(items))
	out := make([]domain.NormalizedGroupMember, 0, len(items))
	for _, it := range items {
		if pos, ok := index[it.ID]; ok {
			out[pos] = it
			continue
		}
		index[it.ID] = len(out)
		out = append(out, it)
	}
	return out
}

// memberCache 在一次请求内缓存用户与其提交邮箱，避免重复查询。
type memberCache struct {
	repo   GroupUsers
	users  map[string]*domain.User
	emails map[string][]domain.CommitEmail
}

// newMemberCache 创建请求内的成员缓存。
func newMemberCache(users GroupUsers) *memberCache {
	return &memberCache{
		repo:   users,
		users:  map[string]*domain.User{},
		emails: map[string][]domain.CommitEmail{},
	}
}

// user 读取用户（带缓存）。
func (c *memberCache) user(ctx context.Context, uKey string) (*domain.User, error) {
	if u, ok := c.users[uKey]; ok {
		return u, nil
	}
	u, err := c.repo.Get(ctx, uKey)
	if err != nil {
		return nil, err
	}
	c.users[uKey] = u
	return u, nil
}

// commitEmails 读取用户已验证的提交邮箱（带缓存）。
func (c *memberCache) commitEmails(ctx context.Context, uKey string) ([]domain.CommitEmail, error) {
	if e, ok := c.emails[uKey]; ok {
		return e, nil
	}
	e, err := c.repo.CommitEmails(ctx, uKey, true)
	if err != nil {
		return nil, err
	}
	c.emails[uKey] = e
	return e, nil
}

// phpQueryArg 读取"客户端真正传进来"的 query 参数。
//
// PHP 侧 Request::parse() 的分页归一化有一处写时复制（COW）缺陷：
// 它把 page / pagesize 写进局部 $finalData，而 self::$request 里保存的
// 是归一化之前的 $_GET 副本。因此在 Request::parse()->query 里，
// "客户端没传 page" 与 "客户端传了 page" 同样表现为键不存在。
//
// 后果是：处理器把 null 传给带类型的模型方法（listActivities /
// getMergeRequests）时，PHP 8 会抛 TypeError（致命错误），
// 页面返回 HTTP 500 且响应体为空。contract 包已经把 page/pagesize
// 归一化成 1/20，无法据此判断客户端是否真的传了它们，
// 因此这里用 RawQuery 还原原始 query。
func phpQueryArg(p *contract.Parsed, key string) (string, bool) {
	if p == nil {
		return "", false
	}
	values, ok := p.RawQuery[key]
	if !ok || len(values) == 0 {
		return "", false
	}
	return values[len(values)-1], true
}

// phpBodyArg 读取"客户端真正传进来"的 body 参数。
//
// PHP 侧 Request::parse()->parsed 就是 $_POST（multipart / urlencoded）
// 或 json_decode(body)（application/json）。键不存在时取值为 null，
// 传给带类型的形参会触发 TypeError，因此必须区分"缺失"与"空串"。
func phpBodyArg(p *contract.Parsed, key string) (string, bool) {
	if p == nil {
		return "", false
	}
	if v, ok := p.Body[key]; ok {
		return phpScalarString(v), true
	}
	if v, ok := p.JSON[key]; ok {
		return phpScalarString(v), true
	}
	return "", false
}

// phpScalarString 把 body 参数转成字符串（PHP 的 (string) 转换）。
func phpScalarString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		if t {
			return "1"
		}
		return ""
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	default:
		return ""
	}
}

// phpFatalCode 是 PHP 致命错误在契约错误码表里的占位。
//
// PHP 8 把 null 传给带类型的形参会抛 TypeError（致命错误），
// 页面直接返回 HTTP 500 且响应体为空。
// contract 包的错误码表没有这一项，而 contract.APIError.HTTPStatus()
// 对未登记的错误码返回 500 —— 这里用它复刻 HTTP 状态码。
const phpFatalCode = contract.ErrorCode(0x0500)

// phpFatalError 返回一个等价于 PHP 致命错误的错误。
func phpFatalError() error {
	return contract.NewError(phpFatalCode)
}
