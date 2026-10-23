// 管理（后台）域的 API 端点。
//
// 每个端点都标注了对应的 PHP 源码位置，便于逐条比对契约：
// application/controllers/api/admin.php。
//
// 与其它域的两点差异：
//
//  1. 鉴权级别是 httpapi.AuthAdminWebUser —— 必须是已登录的管理员，
//     非管理员与未登录一律返回 0x0101（PHP 侧 APIAuth::auth 的行为）。
//  2. admin/config 的写路径修复了 PHP 侧的 P0 缺陷：
//     PHP 侧 data 字段解码为 null 时 array_merge($config, null) 只发 Warning
//     并返回 null，随后 yaml_emit_file() 把 config.yaml 写成空结构，
//     此后 www/index.php:21 的 make_constants('YAML_', yaml_parse_file(config.yaml))
//     抛 TypeError —— 登录与全部 API 永久 500。
//     新实现在解析阶段就拒绝非法载荷，写文件走"备份 → 临时文件 → fsync → rename"
//     的原子路径，并在启动时自检 config.yaml。
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"

	"codefever-next/backend/internal/auth"
	"codefever-next/backend/internal/config"
	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
	"codefever-next/backend/internal/httpapi"
	"codefever-next/backend/internal/repo"
	"codefever-next/backend/internal/storage"
)

// siteBackupSuffix 是 config.yaml 备份文件的后缀。
const siteBackupSuffix = ".bak"

// adminRegisterIP 是新建用户时写入 u_register_ip 的值。
//
// PHP 侧写的是请求来源 IP（$this->input->ip_address()）。
// httpapi.Request 没有暴露远端地址，且该字段不参与任何契约输出，
// 因此这里写固定值；需要真实 IP 时应扩展 httpapi.Request。
const adminRegisterIP = "0.0.0.0"

// AdminHandler 实现 /api/admin/* 端点。
type AdminHandler struct {
	// Cfg 提供站点配置（host / ssh）与 config.yaml 的位置。
	Cfg *config.Config
	// Admin 承担管理域专有的查询与写操作。
	Admin *repo.AdminRepo
	// Users 用于按主键/邮箱读取用户。
	Users *repo.UserRepo
	// Groups 用于仓库组与组成员的读写。
	Groups *repo.GroupRepo
	// Repos 用于仓库、仓库成员与"用户可见仓库"的读取。
	Repos *repo.RepositoryRepo
	// Disk 用于存放新用户的头像文件。
	Disk *storage.Disk

	// site 是当前生效的 config.yaml 文档。
	//
	// PHP 侧每次请求都重新读文件；新实现在启动时加载一次，写入成功后热更新。
	// 用原子指针保存，保证读请求看到的始终是一个完整快照。
	site atomic.Pointer[map[string]any]
}

// NewAdminHandler 创建管理域处理器。
//
// 构造过程会做**启动自检**：读取 config.yaml 并要求它可解析且是一个键值映射。
// 不满足时返回错误 —— 调用方（main）必须因此拒绝启动，
// 而不是让后续每个请求都 500（这正是 PHP 侧 P0 缺陷的后果）。
func NewAdminHandler(
	cfg *config.Config,
	admin *repo.AdminRepo,
	users *repo.UserRepo,
	groups *repo.GroupRepo,
	repos *repo.RepositoryRepo,
	disk *storage.Disk,
) (*AdminHandler, error) {
	h := &AdminHandler{
		Cfg:    cfg,
		Admin:  admin,
		Users:  users,
		Groups: groups,
		Repos:  repos,
		Disk:   disk,
	}
	doc, err := h.loadSiteDocument()
	if err != nil {
		return nil, fmt.Errorf("config.yaml 启动自检失败（拒绝启动）: %w", err)
	}
	h.storeSiteDocument(doc)
	return h, nil
}

// Register 把管理域端点注册到服务器。
//
// 逐字对应 application/controllers/api/admin.php 的 19 个公开方法。
// 动作名与 PHP 的 <action>_<method> 前缀保持一致。
//
// 注意：必须通过本方法注册（而不是在 main.go 里自己拼 Controller）——
// 因为这里还挂了一层 multipart 表单预解析的包装（见 adminBodyPaths 的说明）。
func (h *AdminHandler) Register(s *httpapi.Server) {
	actions := map[string]map[string]httpapi.ActionHandler{
		"userList":               {"POST": h.userList},
		"updateUserStatus":       {"POST": h.updateUserStatus},
		"closeUserMFA":           {"POST": h.closeUserMFA},
		"resetPassword":          {"POST": h.resetPassword},
		"checkPassword":          {"POST": h.checkPassword},
		"setAdministrator":       {"POST": h.setAdministrator},
		"addUser":                {"POST": h.addUser},
		"groupList":              {"POST": h.groupList},
		"groupMembers":           {"POST": h.groupMembers},
		"groupAddMember":         {"POST": h.groupAddMember},
		"groupRemoveMember":      {"POST": h.groupRemoveMember},
		"setGroupOwner":          {"POST": h.setGroupOwner},
		"repositoryList":         {"POST": h.repositoryList},
		"repositoryMembers":      {"POST": h.repositoryMembers},
		"repositoryAddMember":    {"POST": h.repositoryAddMember},
		"repositoryRemoveMember": {"POST": h.repositoryRemoveMember},
		"systemStatus":           {"GET": h.systemStatus},
		"config":                 {"GET": h.configGet, "POST": h.configPost},
	}

	s.Register(&httpapi.Controller{
		Name:    "admin",
		Auth:    httpapi.AuthAdminWebUser,
		Actions: actions,
	})

	// multipart 表单预解析。
	//
	// 背景：internal/contract 的 Parse() 先用 io.ReadAll 把整个请求体读完，
	// 之后才调用 r.ParseMultipartForm —— 此时请求流已空，
	// multipart 的表单字段与文件全部丢失，p.Body 恒为空
	// （见 internal/contract/parse.go 的 Parse()）。
	// admin 域的分页 / 排序 / 过滤参数全部走 body，因此必须先把这层补上。
	//
	// 做法：在进入 API 分发器之前先调用一次 r.ParseMultipartForm。
	// ParseMultipartForm 填充 r.MultipartForm 之后再调用会直接返回，
	// 于是 contract.Parse 仍能正确填充 p.Body 与 p.Files，
	// 鉴权、动作查找与响应信封仍由统一的分发器负责（行为与其它域完全一致）。
	//
	// 待 contract 层修复（Parse 解析 multipart 前复位请求流）后，本段即可删除。
	apiMux := s.Handler()
	for _, path := range adminBodyPaths {
		s.RegisterPage(path, adminMultipartPreParse(apiMux))
	}
}

// adminBodyPaths 是需要读取 multipart 表单字段的管理域端点路径。
//
// GET 端点（systemStatus / config）不需要表单字段，因此不在列表里。
var adminBodyPaths = []string{
	"/api/admin/userList",
	"/api/admin/updateUserStatus",
	"/api/admin/closeUserMFA",
	"/api/admin/resetPassword",
	"/api/admin/checkPassword",
	"/api/admin/setAdministrator",
	"/api/admin/addUser",
	"/api/admin/groupList",
	"/api/admin/groupMembers",
	"/api/admin/groupAddMember",
	"/api/admin/groupRemoveMember",
	"/api/admin/setGroupOwner",
	"/api/admin/repositoryList",
	"/api/admin/repositoryMembers",
	"/api/admin/repositoryAddMember",
	"/api/admin/repositoryRemoveMember",
	"/api/admin/config",
}

// adminMaxMemory 与 contract.Parse 使用同一个 multipart 内存上限。
const adminMaxMemory = 32 << 20

// adminMultipartPreParse 返回一个"先解析 multipart 表单、再把请求交回
// API 分发器"的包装器。
func adminMultipartPreParse(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
		if strings.HasPrefix(contentType, "multipart/form-data") {
			// 解析失败时忽略：随后的 contract.Parse 会给出同样的失败结论。
			_ = r.ParseMultipartForm(adminMaxMemory)
		}
		next.ServeHTTP(w, r)
	}
}

// ---------------------------------------------------------------------------
// 用户
// ---------------------------------------------------------------------------

// userList 实现 POST /api/admin/userList。
//
// 逐字对应 application/controllers/api/admin.php:24-45：
//  1. category / keyword / sort / page / pagesize 全部来自 body；
//  2. sort 命中白名单才排序（name / email / created）；
//  3. 先取总数，再取当页列表；
//  4. normalizeUserList() 逐条补上"该用户可见的仓库数"（projects）。
//
// 注意 admin 域的分页参数走 body（multipart），因此 (int) 转换有效 ——
// 真实分页生效，与 repository 域"query 分页被强制归一化成 1/20"不同。
func (h *AdminHandler) userList(ctx context.Context, req *httpapi.Request) (any, error) {
	p := req.Parsed

	category := p.BodyInt("category")
	keyword := strings.TrimSpace(p.BodyString("keyword"))
	sort := strings.TrimSpace(p.BodyString("sort"))
	page, pagesize := domain.AdminPaging(p.BodyInt("page"), p.BodyInt("pagesize"))

	count, err := h.Admin.CountUsers(ctx, category, keyword)
	if err != nil {
		return nil, err
	}
	users, err := h.Admin.ListUsers(ctx, category, keyword, sort, page, pagesize)
	if err != nil {
		return nil, err
	}

	list := make([]domain.AdminUserListItem, 0, len(users))
	for i := range users {
		u := &users[i]
		projects, err := h.Repos.ListJoined(ctx, u.Key)
		if err != nil {
			return nil, err
		}
		list = append(list, domain.AdminUserListItem{
			ID:         u.Key,
			Email:      u.Email,
			Name:       u.Name,
			Avatar:     u.Avatar,
			MFAEnabled: u.TwoFA != nil && *u.TwoFA != "",
			Created:    u.Created,
			Status:     u.Status == domain.CommonStatusNormal,
			Admin:      u.Admin,
			Projects:   len(projects),
		})
	}

	return map[string]any{"count": count, "list": list}, nil
}

// updateUserStatus 实现 POST /api/admin/updateUserStatus。
//
// 逐字对应 application/controllers/api/admin.php:47-63：
//  1. user / status 缺一 → 0x0201；
//  2. status 等于 COMMON_STATUS_NORMAL 时置为正常，否则置为 COMMON_STATUS_DELETE；
//  3. 写库失败 → 0x0405。
func (h *AdminHandler) updateUserStatus(ctx context.Context, req *httpapi.Request) (any, error) {
	p := req.Parsed

	uKey := strings.TrimSpace(p.BodyString("user"))
	status := p.BodyInt("status")

	if !domain.PHPTruthy(uKey) || status == 0 {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	newStatus := domain.CommonStatusNormal
	if status != domain.CommonStatusNormal {
		newStatus = domain.AdminUserCategoryStatusDeleteValue
	}

	if err := h.Admin.WithTx(ctx, func(tx *repo.AdminRepo) error {
		return tx.UpdateUserFields(ctx, uKey, map[string]any{"u_status": newStatus})
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	return []any{}, nil
}

// closeUserMFA 实现 POST /api/admin/closeUserMFA。
//
// 逐字对应 application/controllers/api/admin.php:65-80：
//  1. user 缺失 → 0x0201；
//  2. 把 u_2fa 置空（关闭双因素认证）。
func (h *AdminHandler) closeUserMFA(ctx context.Context, req *httpapi.Request) (any, error) {
	p := req.Parsed

	uKey := strings.TrimSpace(p.BodyString("user"))
	if !domain.PHPTruthy(uKey) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	if err := h.Admin.WithTx(ctx, func(tx *repo.AdminRepo) error {
		return tx.UpdateUserFields(ctx, uKey, map[string]any{"u_2fa": nil})
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	return []any{}, nil
}

// resetPassword 实现 POST /api/admin/resetPassword。
//
// 逐字对应 application/controllers/api/admin.php:82-98：
//  1. user / password 缺一 → 0x0201；
//  2. 重置口令。
//
// 与 PHP 的差别（存储格式，不影响对外行为）：
// PHP 写 md5(md5($password))，新实现默认也写 md5(md5($password))
// （由 auth.hash_format / CODEFEVER_PASSWORD_HASH 决定，默认 legacy）。
// 保持 legacy 的原因是：PHP 后端只会用 md5(md5($password)) 校验口令，
// 共存期一旦写成 argon2id，该账号就无法在 PHP 侧登录，回滚目标随之失效。
// 口令校验统一走 auth.VerifyPassword —— 它同时支持旧版双层 md5 与 argon2id，
// 因此既有用户与已升级用户都能正常登录。
func (h *AdminHandler) resetPassword(ctx context.Context, req *httpapi.Request) (any, error) {
	p := req.Parsed

	uKey := strings.TrimSpace(p.BodyString("user"))
	password := strings.TrimSpace(p.BodyString("password"))

	if !domain.PHPTruthy(uKey) || !domain.PHPTruthy(password) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	hashed, err := auth.HashPasswordForFormat(password, h.Cfg.PasswordHashFormat())
	if err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	if err := h.Admin.WithTx(ctx, func(tx *repo.AdminRepo) error {
		return tx.UpdateUserFields(ctx, uKey, map[string]any{"u_password": hashed})
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	return []any{}, nil
}

// checkPassword 实现 POST /api/admin/checkPassword。
//
// 逐字对应 application/controllers/api/admin.php:100-111：
// 比对**当前登录用户**的口令，不匹配 → 0x0405。
//
// 注意 PHP 侧没有"参数缺失"分支：口令为空时同样走比对失败（0x0405）。
func (h *AdminHandler) checkPassword(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}

	password := strings.TrimSpace(req.Parsed.BodyString("password"))
	if ok, _ := auth.VerifyPassword(u.Password, password); !ok {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	return []any{}, nil
}

// setAdministrator 实现 POST /api/admin/setAdministrator。
//
// 逐字对应 application/controllers/api/admin.php:113-129：
//  1. user 缺失 → 0x0201；
//  2. admin 为 PHP 真值时置为 1，否则置为 0。
func (h *AdminHandler) setAdministrator(ctx context.Context, req *httpapi.Request) (any, error) {
	p := req.Parsed

	uKey := strings.TrimSpace(p.BodyString("user"))
	admin := strings.TrimSpace(p.BodyString("admin"))

	if !domain.PHPTruthy(uKey) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	value := 0
	if domain.PHPTruthy(admin) {
		value = 1
	}

	if err := h.Admin.WithTx(ctx, func(tx *repo.AdminRepo) error {
		return tx.UpdateUserFields(ctx, uKey, map[string]any{"u_admin": value})
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	return []any{}, nil
}

// addUser 实现 POST /api/admin/addUser。
//
// 逐字对应 application/controllers/api/admin.php:131-160：
//  1. name 为空或 email 不符合格式 → 0x0201；
//  2. email 已存在 → 0x0406；
//  3. 生成随机初始口令与 Material Design 头像；
//  4. 写库失败 → 0x0405；
//  5. **明文返回初始口令**（前端直接展示给管理员），因此调用方必须妥善处理。
//
// 头像生成参考 application/libraries/Avatars/MaterialDesign.Avatars.class.php：
// 同样的 6 色 Material Design 调色板 + 居中白色首字母，
// 但底色改为按邮箱哈希确定（PHP 侧是 mt_rand 随机）。
func (h *AdminHandler) addUser(ctx context.Context, req *httpapi.Request) (any, error) {
	p := req.Parsed

	name := strings.TrimSpace(p.BodyString("name"))
	email := strings.TrimSpace(p.BodyString("email"))

	if !domain.PHPTruthy(name) || !domain.AdminEmailValid(email) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	existing, err := h.Users.GetByEmail(ctx, email)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return nil, err
	}
	if err == nil && existing != nil {
		return nil, contract.NewError(contract.CodeTargetExists)
	}

	password := domain.AdminInitialPassword()
	hashed, err := auth.HashPasswordForFormat(password, h.Cfg.PasswordHashFormat())
	if err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	// 头像文件（先落盘，插入失败时清理，避免留下孤儿文件）。
	avatarKey, err := h.createAvatar(email)
	if err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	user := &domain.AdminUser{
		Key:    contract.NewKey(),
		Email:  email,
		Name:   name,
		Avatar: &avatarKey,
	}
	if err := h.Admin.WithTx(ctx, func(tx *repo.AdminRepo) error {
		return tx.AddUser(ctx, user, hashed, adminRegisterIP)
	}); err != nil {
		h.removeAvatar(avatarKey)
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	return map[string]any{"password": password}, nil
}

// createAvatar 生成并保存头像文件，返回文件主键。
func (h *AdminHandler) createAvatar(seed string) (string, error) {
	key := contract.NewKey()
	payload, err := domain.AdminAvatarPNG(seed)
	if err != nil {
		return "", err
	}
	if h.Disk == nil {
		return key, nil
	}
	if err := h.Disk.Save("avatar", key, payload); err != nil {
		return "", err
	}
	return key, nil
}

// removeAvatar 删除头像文件（新建用户失败时清理）。
func (h *AdminHandler) removeAvatar(key string) {
	if h.Disk == nil {
		return
	}
	_ = h.Disk.Remove("avatar", key)
}

// ---------------------------------------------------------------------------
// 仓库组
// ---------------------------------------------------------------------------

// groupList 实现 POST /api/admin/groupList。
//
// 逐字对应 application/controllers/api/admin.php:162-182：
//  1. keyword 命中 g_display_name；
//  2. 只列 g_status 正常的组；
//  3. sort 命中 name / created 白名单才排序；
//  4. 先取总数再取当页列表。
func (h *AdminHandler) groupList(ctx context.Context, req *httpapi.Request) (any, error) {
	p := req.Parsed

	keyword := strings.TrimSpace(p.BodyString("keyword"))
	sort := strings.TrimSpace(p.BodyString("sort"))
	page, pagesize := domain.AdminPaging(p.BodyInt("page"), p.BodyInt("pagesize"))

	count, err := h.Admin.CountGroups(ctx, keyword)
	if err != nil {
		return nil, err
	}
	groups, err := h.Admin.ListGroups(ctx, keyword, sort, page, pagesize)
	if err != nil {
		return nil, err
	}

	list := make([]domain.NormalizedGroup, 0, len(groups))
	for i := range groups {
		list = append(list, domain.NormalizeGroup(&groups[i]))
	}

	return map[string]any{"count": count, "list": list}, nil
}

// groupMembers 实现 POST /api/admin/groupMembers。
//
// 逐字对应 application/controllers/api/admin.php:184-191：
// 输出组成员（含组创建者），并按 id 去重。
//
// PHP 的 getMembers() 在组不存在时会走进 normalizeCreator() 的
// `userModel->get(null)` —— PHP 8 抛 TypeError（致命错误 → HTTP 500）。
// 这里原样复刻该行为（而不是悄悄返回一个空数组），
// 避免把"查错了组"伪装成"组里没人"。
func (h *AdminHandler) groupMembers(ctx context.Context, req *httpapi.Request) (any, error) {
	p := req.Parsed

	gKey := strings.TrimSpace(p.BodyString("group"))

	group, err := h.Groups.Get(ctx, gKey)
	if err != nil {
		return nil, phpFatalError()
	}

	members, err := h.Groups.ListMembers(ctx, gKey)
	if err != nil {
		return nil, err
	}

	cache := newMemberCache(h.Users)
	list := make([]domain.NormalizedGroupMember, 0, len(members)+1)
	for i := range members {
		m := &members[i]
		u, err := cache.user(ctx, m.UKey)
		if err != nil {
			// 成员行指向的用户已不存在：PHP 侧会输出一个全 null 的条目，
			// 新实现直接跳过（该条目对前端没有意义）。
			continue
		}
		emails, err := cache.commitEmails(ctx, m.UKey)
		if err != nil {
			return nil, err
		}
		list = append(list, domain.NormalizeGroupMember(m, u, emails, h.host(), h.ssh()))
	}

	owner, err := cache.user(ctx, group.OwnerKey)
	if err != nil {
		return nil, phpFatalError()
	}
	ownerEmails, err := cache.commitEmails(ctx, group.OwnerKey)
	if err != nil {
		return nil, err
	}
	list = append(list, domain.NormalizeGroupCreator(group, owner, ownerEmails, h.host(), h.ssh()))

	return mergeAdminGroupMembers(list), nil
}

// groupAddMember 实现 POST /api/admin/groupAddMember。
//
// 逐字对应 application/controllers/api/admin.php:193-219：
//  1. group / email 缺一 → 0x0201；
//  2. 邮箱查无此人 → 0x0407；
//  3. 不能把组创建者加为成员 → 0x0408；
//  4. 写入失败 → 0x0405。
func (h *AdminHandler) groupAddMember(ctx context.Context, req *httpapi.Request) (any, error) {
	p := req.Parsed

	groupKey := strings.TrimSpace(p.BodyString("group"))
	email := strings.TrimSpace(p.BodyString("email"))

	if !domain.PHPTruthy(groupKey) || !domain.PHPTruthy(email) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	member, err := h.Users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, contract.NewError(contract.CodeUserNotFound)
		}
		return nil, err
	}

	group, err := h.Groups.Get(ctx, groupKey)
	if err == nil && group.OwnerKey == member.Key {
		return nil, contract.NewError(contract.CodeCannotAddCreator)
	}
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return nil, err
	}

	if err := h.Groups.WithTx(ctx, func(tx *repo.GroupRepo) error {
		return tx.AddMember(ctx, member.Key, groupKey)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	return []any{}, nil
}

// groupRemoveMember 实现 POST /api/admin/groupRemoveMember。
//
// 逐字对应 application/controllers/api/admin.php:221-247：
//  1. group / user 缺一 → 0x0201；
//  2. 用户不存在 → 0x0407；
//  3. 不能把组创建者移出 → 0x0106；
//  4. 写入失败 → 0x0405。
func (h *AdminHandler) groupRemoveMember(ctx context.Context, req *httpapi.Request) (any, error) {
	p := req.Parsed

	groupKey := strings.TrimSpace(p.BodyString("group"))
	userKey := strings.TrimSpace(p.BodyString("user"))

	if !domain.PHPTruthy(groupKey) || !domain.PHPTruthy(userKey) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	member, err := h.Users.Get(ctx, userKey)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, contract.NewError(contract.CodeUserNotFound)
		}
		return nil, err
	}

	group, err := h.Groups.Get(ctx, groupKey)
	if err == nil && group.OwnerKey == member.Key {
		return nil, contract.NewError(contract.CodeActionNotAllowed)
	}
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return nil, err
	}

	if err := h.Groups.WithTx(ctx, func(tx *repo.GroupRepo) error {
		return tx.RemoveMember(ctx, userKey, groupKey)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	return []any{}, nil
}

// setGroupOwner 实现 POST /api/admin/setGroupOwner。
//
// 逐字对应 application/controllers/api/admin.php:249-271：
//  1. **没有任何参数校验**（PHP 原样）—— 缺 user 时 userModel->get(”)
//     查不到 → 0x0407，在 setMemberRole 之前就返回，不写库；
//  2. 不能把组创建者设为 owner → 0x0106；
//  3. 写入失败 → 0x0405。
func (h *AdminHandler) setGroupOwner(ctx context.Context, req *httpapi.Request) (any, error) {
	p := req.Parsed

	groupKey := strings.TrimSpace(p.BodyString("group"))
	userKey := strings.TrimSpace(p.BodyString("user"))

	member, err := h.Users.Get(ctx, userKey)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, contract.NewError(contract.CodeUserNotFound)
		}
		return nil, err
	}

	group, err := h.Groups.Get(ctx, groupKey)
	if err == nil && group.OwnerKey == member.Key {
		return nil, contract.NewError(contract.CodeActionNotAllowed)
	}
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return nil, err
	}

	if err := h.Groups.WithTx(ctx, func(tx *repo.GroupRepo) error {
		return tx.SetMemberRole(ctx, userKey, groupKey, int(domain.RoleOwner))
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	return []any{}, nil
}

// ---------------------------------------------------------------------------
// 仓库
// ---------------------------------------------------------------------------

// repositoryList 实现 POST /api/admin/repositoryList。
//
// 逐字对应 application/controllers/api/admin.php:273-293：
//  1. keyword 命中 r_display_name；
//  2. 只列 r_status 正常的仓库；
//  3. sort 命中 name / created 白名单才排序；
//  4. 逐条归一化：附带所属仓库组、fork 数与合并请求计数。
func (h *AdminHandler) repositoryList(ctx context.Context, req *httpapi.Request) (any, error) {
	p := req.Parsed

	keyword := strings.TrimSpace(p.BodyString("keyword"))
	sort := strings.TrimSpace(p.BodyString("sort"))
	page, pagesize := domain.AdminPaging(p.BodyInt("page"), p.BodyInt("pagesize"))

	count, err := h.Admin.CountRepositories(ctx, keyword)
	if err != nil {
		return nil, err
	}
	repositories, err := h.Admin.ListRepositories(ctx, keyword, sort, page, pagesize)
	if err != nil {
		return nil, err
	}

	rKeys := make([]string, 0, len(repositories))
	gKeys := make([]string, 0, len(repositories))
	for _, r := range repositories {
		rKeys = append(rKeys, r.Key)
		gKeys = append(gKeys, r.GroupKey)
	}
	groups, err := h.Repos.GroupsByKeys(ctx, gKeys)
	if err != nil {
		return nil, err
	}
	forks, err := h.Repos.ForkCounts(ctx, rKeys)
	if err != nil {
		return nil, err
	}
	mrs, err := h.Repos.MergeRequestCounts(ctx, rKeys)
	if err != nil {
		return nil, err
	}

	list := make([]domain.NormalizedRepository, 0, len(repositories))
	for i := range repositories {
		r := &repositories[i]
		list = append(list, domain.NormalizeRepository(r, groups[r.GroupKey], forks[r.Key], mrs[r.Key]))
	}

	return map[string]any{"count": count, "list": list}, nil
}

// repositoryMembers 实现 POST /api/admin/repositoryMembers。
//
// 逐字对应 application/controllers/api/admin.php:295-302 →
// application/models/repository_model.php:2100-2122 的 getMembers()：
// 组成员（含组创建者）+ 仓库成员 + 仓库创建者，再按 id 去重。
//
// 仓库不存在（或组不存在）时 PHP 侧会走进 normalizeCreator() 的
// `userModel->get(null)` —— TypeError（致命错误 → HTTP 500），这里原样复刻。
func (h *AdminHandler) repositoryMembers(ctx context.Context, req *httpapi.Request) (any, error) {
	p := req.Parsed

	rKey := strings.TrimSpace(p.BodyString("repository"))

	repository, err := h.Repos.Get(ctx, rKey)
	if err != nil {
		return nil, phpFatalError()
	}

	group, err := h.Groups.Get(ctx, repository.GroupKey)
	if err != nil {
		return nil, phpFatalError()
	}

	groupMembers, err := h.Groups.ListMembers(ctx, repository.GroupKey)
	if err != nil {
		return nil, err
	}
	repoMembers, err := h.Repos.MembersOf(ctx, []string{repository.Key})
	if err != nil {
		return nil, err
	}

	cache := newMemberCache(h.Users)
	list := make([]domain.NormalizedRepositoryMember, 0,
		len(groupMembers)+len(repoMembers[repository.Key])+2)

	// 组成员（含组创建者）。
	for i := range groupMembers {
		m := &groupMembers[i]
		u, err := cache.user(ctx, m.UKey)
		if err != nil {
			continue
		}
		emails, err := cache.commitEmails(ctx, m.UKey)
		if err != nil {
			return nil, err
		}
		list = append(list, domain.NormalizeRepositoryGroupMember(
			&domain.RepositoryGroupMember{
				Key: m.Key, GKey: m.GKey, UKey: m.UKey,
				Role: m.Role, Status: m.Status,
			}, u, emails, h.host(), h.ssh()))
	}
	groupOwner, err := cache.user(ctx, group.OwnerKey)
	if err != nil {
		return nil, phpFatalError()
	}
	groupOwnerEmails, err := cache.commitEmails(ctx, group.OwnerKey)
	if err != nil {
		return nil, err
	}
	list = append(list, domain.NormalizeRepositoryGroupCreator(
		&domain.RepositoryGroup{
			Key: group.Key, OwnerKey: group.OwnerKey, Type: group.Type,
			DisplayName: group.DisplayName, Name: group.Name,
			Description: group.Description, Avatar: group.Avatar,
			Created: group.Created.UTC().Format("2006-01-02 15:04:05"),
			Updated: group.Updated.UTC().Format("2006-01-02 15:04:05"),
			Status:  group.Status,
		}, groupOwner, groupOwnerEmails, h.host(), h.ssh()))

	// 仓库成员 + 仓库创建者。
	for i := range repoMembers[repository.Key] {
		m := &repoMembers[repository.Key][i]
		u, err := cache.user(ctx, m.UKey)
		if err != nil {
			continue
		}
		emails, err := cache.commitEmails(ctx, m.UKey)
		if err != nil {
			return nil, err
		}
		list = append(list, domain.NormalizeRepositoryMember(
			&domain.RepositoryMember{
				Key: m.Key, RKey: m.RKey, UKey: m.UKey,
				Role: m.Role, Created: m.Created, Status: m.Status,
			}, u, emails, h.host(), h.ssh()))
	}
	repoOwner, err := cache.user(ctx, repository.OwnerKey)
	if err != nil {
		return nil, phpFatalError()
	}
	repoOwnerEmails, err := cache.commitEmails(ctx, repository.OwnerKey)
	if err != nil {
		return nil, err
	}
	list = append(list, domain.NormalizeRepositoryCreator(repository, repoOwner, repoOwnerEmails, h.host(), h.ssh()))

	return mergeAdminMembers(list), nil
}

// repositoryAddMember 实现 POST /api/admin/repositoryAddMember。
//
// 逐字对应 application/controllers/api/admin.php:304-330：
//  1. repository / email 缺一 → 0x0201；
//  2. 邮箱查无此人 → 0x0407；
//  3. 不能把仓库创建者加为成员 → 0x0408；
//  4. 写入失败 → 0x0405。
func (h *AdminHandler) repositoryAddMember(ctx context.Context, req *httpapi.Request) (any, error) {
	p := req.Parsed

	repositoryKey := strings.TrimSpace(p.BodyString("repository"))
	email := strings.TrimSpace(p.BodyString("email"))

	if !domain.PHPTruthy(repositoryKey) || !domain.PHPTruthy(email) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	member, err := h.Users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, contract.NewError(contract.CodeUserNotFound)
		}
		return nil, err
	}

	repository, err := h.Repos.Get(ctx, repositoryKey)
	if err == nil && repository.OwnerKey == member.Key {
		return nil, contract.NewError(contract.CodeCannotAddCreator)
	}
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return nil, err
	}

	if err := h.Admin.WithTx(ctx, func(tx *repo.AdminRepo) error {
		return tx.AddRepositoryMember(ctx, member.Key, repositoryKey)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	return []any{}, nil
}

// repositoryRemoveMember 实现 POST /api/admin/repositoryRemoveMember。
//
// 逐字对应 application/controllers/api/admin.php:332-358：
//  1. repository / user 缺一 → 0x0201；
//  2. 用户不存在 → 0x0407；
//  3. 不能把仓库创建者移出 → 0x0106；
//  4. 写入失败 → 0x0405。
func (h *AdminHandler) repositoryRemoveMember(ctx context.Context, req *httpapi.Request) (any, error) {
	p := req.Parsed

	repositoryKey := strings.TrimSpace(p.BodyString("repository"))
	userKey := strings.TrimSpace(p.BodyString("user"))

	if !domain.PHPTruthy(repositoryKey) || !domain.PHPTruthy(userKey) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	member, err := h.Users.Get(ctx, userKey)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, contract.NewError(contract.CodeUserNotFound)
		}
		return nil, err
	}

	repository, err := h.Repos.Get(ctx, repositoryKey)
	if err == nil && repository.OwnerKey == member.Key {
		return nil, contract.NewError(contract.CodeActionNotAllowed)
	}
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return nil, err
	}

	if err := h.Admin.WithTx(ctx, func(tx *repo.AdminRepo) error {
		return tx.RemoveRepositoryMember(ctx, userKey, repositoryKey)
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	return []any{}, nil
}

// ---------------------------------------------------------------------------
// 系统状态
// ---------------------------------------------------------------------------

// systemStatus 实现 GET /api/admin/systemStatus。
//
// 逐字对应 application/controllers/api/admin.php:360-468：
//  1. usage：正常状态的用户 / 仓库 / 仓库组数量；
//  2. vm：CPU、内存、磁盘占用（来自 vmstat / free -m / df）；
//  3. service：nginx / php / codefever / crond / sendmail 的存活状态。
//
// PHP 侧靠 exec 跑 `ps uax | grep ... | wc -l` 判断进程存活。
// 新实现复刻**语义**（哪些进程算 running），但：
//   - 不经过 shell 管道，直接读 /proc（不可用时回退到 ps），避免 grep 误配；
//   - 命令缺失时只把对应指标留成零值，**绝不让端点 500**。
func (h *AdminHandler) systemStatus(ctx context.Context, req *httpapi.Request) (any, error) {
	usage, err := h.Admin.UsageCounts(ctx)
	if err != nil {
		return nil, err
	}

	cmdlines := processCommandLines()
	rules := domain.AdminServiceRules()
	// php 与 crond 已随 PHP 运行时下线而移除（见 domain.AdminServiceRules）。
	service := domain.AdminServiceStatus{
		Nginx:     domain.AdminServiceRunning(rules["nginx"], cmdlines),
		CodeFever: domain.AdminServiceRunning(rules["codefever"], cmdlines),
		Sendmail:  domain.AdminServiceRunning(rules["sendmail"], cmdlines),
	}

	vm := domain.AdminVMStatus{Disk: []domain.AdminDiskUsage{}}
	if out, err := runAdminCommand("vmstat"); err == nil {
		if cpu, ok := domain.AdminParseCPU(out); ok {
			vm.CPU = cpu
		}
	}
	if out, err := runAdminCommand("free", "-m"); err == nil {
		if memory, ok := domain.AdminParseMemory(out); ok {
			vm.Memory = memory
		}
	}
	if out, err := runAdminCommand("df"); err == nil {
		if disk, ok := domain.AdminParseDisk(out); ok {
			vm.Disk = []domain.AdminDiskUsage{disk}
		}
	}

	return map[string]any{
		"usage":   usage,
		"vm":      vm,
		"service": service,
	}, nil
}

// ---------------------------------------------------------------------------
// 站点配置
// ---------------------------------------------------------------------------

// configGet 实现 GET /api/admin/config。
//
// 逐字对应 application/controllers/api/admin.php:470-474：
// 输出 config.yaml 的完整内容。
//
// PHP 侧每次请求都重新读文件；新实现在启动时加载一次、写入成功后热更新，
// 因此这里返回内存中的当前快照（对前端而言等价）。
func (h *AdminHandler) configGet(ctx context.Context, req *httpapi.Request) (any, error) {
	doc := h.siteDocument()
	if doc == nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return *doc, nil
}

// configPost 实现 POST /api/admin/config。
//
// 逐字对应 application/controllers/api/admin.php:476-484 的语义，
// 但**修复了 PHP 侧的 P0 缺陷**：
//
//	PHP：$final = array_merge($config, $input);
//	     data 字段解码为 null 时 array_merge 只发 Warning 并返回 null，
//	     随后 yaml_emit_file(config.yaml, null) 把文件写成空结构，
//	     此后 www/index.php:21 的 make_constants('YAML_', yaml_parse_file(...))
//	     抛 TypeError —— 登录与全部 API 永久 500。
//
// 新实现：
//  1. 校验载荷：必须是 JSON 对象（映射），否则 0x0201 且**不写文件**；
//  2. 按 PHP 的 array_merge 语义做浅合并（载荷里的键覆盖现有键，缺的键沿用现值）；
//  3. 校验合并结果仍满足模板要求的结构，不满足则 0x0201 且不写文件；
//  4. 原子写入：备份 → 同目录临时文件 → fsync → rename 覆盖；
//  5. 写入成功后热更新内存配置。
func (h *AdminHandler) configPost(ctx context.Context, req *httpapi.Request) (any, error) {
	p := req.Parsed

	raw := p.BodyString("data")
	var payload any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	patch, ok := payload.(map[string]any)
	if !ok {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	current := h.siteDocument()
	base := map[string]any{}
	if current != nil {
		base = *current
	}

	merged := domain.MergeSiteDocument(base, patch)
	if err := domain.ValidateSiteDocument(merged); err != nil {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	if err := writeSiteDocumentAtomic(h.sitePath(), merged); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	h.storeSiteDocument(merged)
	h.Cfg.Site = siteConfigFromDocument(merged)

	return merged, nil
}

// sitePath 返回 config.yaml 的绝对路径。
func (h *AdminHandler) sitePath() string {
	return filepath.Join(h.Cfg.BaseDir, "config.yaml")
}

// loadSiteDocument 读取并解析 config.yaml。
//
// 解析结果必须是映射，否则报错 —— 这是 P0 缺陷的第一道防线。
func (h *AdminHandler) loadSiteDocument() (map[string]any, error) {
	data, err := os.ReadFile(h.sitePath())
	if err != nil {
		return nil, err
	}
	return domain.ParseSiteDocument(data)
}

// siteDocument 返回当前生效的配置文档快照。
func (h *AdminHandler) siteDocument() *map[string]any {
	return h.site.Load()
}

// storeSiteDocument 保存配置文档快照。
func (h *AdminHandler) storeSiteDocument(doc map[string]any) {
	h.site.Store(&doc)
}

// host 与 ssh 返回站点配置里的 host / ssh。
func (h *AdminHandler) host() string { return h.Cfg.Site.Host }
func (h *AdminHandler) ssh() string  { return h.Cfg.Site.SSH }

// siteConfigFromDocument 由配置文档构造 config.Site。
func siteConfigFromDocument(doc map[string]any) config.Site {
	site := config.Site{}
	if v, ok := doc["host"].(string); ok {
		site.Host = v
	}
	if v, ok := doc["ssh"].(string); ok {
		site.SSH = v
	}
	if v, ok := doc["allowRegister"].(bool); ok {
		site.AllowRegister = v
	}
	if email, ok := doc["email"].(map[string]any); ok {
		if v, ok := email["name"].(string); ok {
			site.Email.Name = v
		}
		if v, ok := email["from"].(string); ok {
			site.Email.From = v
		}
	}
	return site
}

// ---------------------------------------------------------------------------
// 文件原子写入
// ---------------------------------------------------------------------------

// writeSiteDocumentAtomic 原子写入 config.yaml。
//
// 步骤（任何一步失败都不会破坏现有 config.yaml）：
//  1. 把原文件复制为 config.yaml.bak（备份失败即中止）；
//  2. 在**同目录**写临时文件并 fsync；
//  3. rename 覆盖目标（同文件系统下的原子替换）；
//  4. 尽力 fsync 目录。
func writeSiteDocumentAtomic(path string, doc map[string]any) error {
	data, err := domain.MarshalSiteDocument(doc)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)

	// 1) 备份原文件。
	mode := os.FileMode(0o644)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
		if err := copySiteFile(path, path+siteBackupSuffix, mode); err != nil {
			return fmt.Errorf("备份 %s 失败: %w", path, err)
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}

	// 2) 同目录临时文件 + fsync。
	tmp, err := os.CreateTemp(dir, ".config.yaml.*.tmp")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("写入临时文件失败: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("同步临时文件失败: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("关闭临时文件失败: %w", err)
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return fmt.Errorf("设置临时文件权限失败: %w", err)
	}

	// 3) rename 覆盖（原子）。
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("替换 %s 失败: %w", path, err)
	}
	cleanup = false

	// 4) 尽力 fsync 目录（Windows 上不支持，忽略）。
	if handle, err := os.Open(dir); err == nil {
		_ = handle.Sync()
		_ = handle.Close()
	}
	return nil
}

// copySiteFile 把 src 复制到 dst。
func copySiteFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// ---------------------------------------------------------------------------
// 进程探测
// ---------------------------------------------------------------------------

// processCommandLines 返回当前全部进程的命令行。
//
// 优先读 /proc（Linux，不依赖外部命令）；不可用时回退到 `ps -eo args=`。
// 两者都不可用时返回空列表 —— 此时所有服务状态都是 false，绝不 500。
func processCommandLines() []string {
	if lines, ok := procCommandLines(); ok {
		return lines
	}
	return psCommandLines()
}

// procCommandLines 读取 /proc 下的进程命令行。
func procCommandLines() ([]string, bool) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, false
	}
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err != nil {
			continue
		}
		parts := strings.Split(strings.TrimRight(string(raw), "\x00"), "\x00")
		fields := make([]string, 0, len(parts))
		for _, p := range parts {
			if p != "" {
				fields = append(fields, p)
			}
		}
		if len(fields) == 0 {
			continue
		}
		out = append(out, strings.Join(fields, " "))
	}
	return out, true
}

// psCommandLines 通过 ps 命令读取进程命令行。
func psCommandLines() []string {
	out, err := exec.Command("ps", "-eo", "args=").Output()
	if err != nil {
		return nil
	}
	lines := strings.Split(string(out), "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

// runAdminCommand 执行一条只读命令并返回标准输出。
func runAdminCommand(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// ---------------------------------------------------------------------------
// 成员去重
// ---------------------------------------------------------------------------

// mergeAdminGroupMembers 按 id 去重仓库组成员。
//
// 逐字对应 application/libraries/service/Utility/Helper.php:59-71 的
// getUniqueMemberList()：已存在且**旧条目是组成员**、而**新条目已删除**时
// 保留旧值，否则覆盖（位置保持首次出现）。
func mergeAdminGroupMembers(items []domain.NormalizedGroupMember) []domain.NormalizedGroupMember {
	index := map[string]int{}
	out := make([]domain.NormalizedGroupMember, 0, len(items))
	for _, item := range items {
		if i, ok := index[item.ID]; ok {
			if out[i].GroupMember && item.DeleteFlag {
				continue
			}
			out[i] = item
			continue
		}
		index[item.ID] = len(out)
		out = append(out, item)
	}
	return out
}

// mergeAdminMembers 按 id 去重仓库成员。
//
// 逐字对应 application/libraries/service/Utility/Helper.php:59-71 的
// getUniqueMemberList()，语义与 mergeAdminGroupMembers 一致。
func mergeAdminMembers(items []domain.NormalizedRepositoryMember) []domain.NormalizedRepositoryMember {
	index := map[string]int{}
	out := make([]domain.NormalizedRepositoryMember, 0, len(items))
	for _, item := range items {
		if i, ok := index[item.ID]; ok {
			if out[i].GroupMember && item.DeleteFlag {
				continue
			}
			out[i] = item
			continue
		}
		index[item.ID] = len(out)
		out = append(out, item)
	}
	return out
}
