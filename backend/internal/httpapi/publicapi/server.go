package publicapi

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"net/http"
	"strings"

	"codefever-next/backend/internal/domain"
	"codefever-next/backend/internal/repo"
)

// Server 是公开 API 的处理器集合。
//
// 鉴权模型：**令牌 ≡ 用户**。
// 令牌不带 scope，权限完全继承所属用户 —— 因此公开 API 不新建权限体系，
// 而是「令牌 → 用户 → 复用现有 repo 层查询」，
// 判定语义与内部 /api/* 完全一致（复用 domain.CheckPermission 与 repo.AccessRepo）。
type Server struct {
	Users   *repo.UserRepo
	Repos   *repo.RepositoryRepo
	Tokens  *repo.AccessTokenRepo
	Access  *repo.AccessRepo
	Groups  *repo.GroupRepo
	SiteURL string
}

// NewServer 创建公开 API 服务。
func NewServer(users *repo.UserRepo, repos *repo.RepositoryRepo, tokens *repo.AccessTokenRepo,
	access *repo.AccessRepo, groups *repo.GroupRepo, siteURL string) *Server {
	return &Server{
		Users:   users,
		Repos:   repos,
		Tokens:  tokens,
		Access:  access,
		Groups:  groups,
		SiteURL: siteURL,
	}
}

// authenticate 解析访问令牌并返回令牌所属用户。
//
// 失败一律返回 401 —— 鉴权失败直接写 http.StatusUnauthorized，不带响应体。
//
// 这里额外带上 {message,url} 错误体，便于调用方定位问题。
func (s *Server) authenticate(r *http.Request) (*domain.User, error) {
	if s.Tokens == nil || s.Users == nil {
		return nil, errors.New("访问令牌服务未启用")
	}
	plaintext := BearerToken(r)
	if plaintext == "" {
		return nil, errors.New("缺少访问令牌")
	}
	token, err := s.Tokens.BySHA256(r.Context(), repo.HashTokenPlaintext(plaintext))
	if err != nil {
		return nil, errors.New("访问令牌无效")
	}
	user, err := s.Users.Get(r.Context(), token.UKey)
	if err != nil {
		return nil, errors.New("访问令牌无效")
	}
	// 已软删除/禁用的账号，其令牌立即失效。
	if user.Status == domain.CommonStatusDelete {
		return nil, errors.New("访问令牌无效")
	}
	// 记录最后使用时间；失败不影响本次请求。
	_ = s.Tokens.Touch(r.Context(), token.Key)
	return user, nil
}

// requireUser 是需要令牌才能访问的端点的包装器。
func (s *Server) requireUser(h func(http.ResponseWriter, *http.Request, *domain.User)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := s.authenticate(r)
		if err != nil {
			WriteError(w, http.StatusUnauthorized, err.Error())
			return
		}
		h(w, r, user)
	}
}

// ---------------------------------------------------------------------------
// 响应体
// ---------------------------------------------------------------------------

// UserBrief 是公开 API 的用户简要信息。
//
// 字段命名采用通用的 login / id / full_name / email 组合，
// 同时保留 CodeFever 自身的 name / avatar 便于前端复用。
type UserBrief struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	FullName  string `json:"full_name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

// RepositoryBrief 是公开 API 的仓库简要信息。
type RepositoryBrief struct {
	ID            int64  `json:"id"`
	Owner         string `json:"owner"`
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Description   string `json:"description"`
	Private       bool   `json:"private"`
	Fork          bool   `json:"fork"`
	Empty         bool   `json:"empty"`
	DefaultBranch string `json:"default_branch"`
	HTMLURL       string `json:"html_url"`
	CloneURL      string `json:"clone_url"`
}

// ---------------------------------------------------------------------------
// 端点
// ---------------------------------------------------------------------------

// GetUser 处理 GET /api/v1/user。
//
// 返回当前令牌所属用户的信息。
func (s *Server) GetUser(w http.ResponseWriter, r *http.Request, u *domain.User) {
	WriteJSON(w, http.StatusOK, s.userBrief(u))
}

// GetUserByName 处理 GET /api/v1/users/{username}。
//
// 公开端点：任何人都可以读取用户的基本资料。
func (s *Server) GetUserByName(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("username"))
	if name == "" {
		WriteError(w, http.StatusUnprocessableEntity, "用户名不能为空")
		return
	}
	u, err := s.Users.GetByName(r.Context(), name)
	if err != nil {
		// 用户不存在或不可见都返回 404。
		WriteError(w, http.StatusNotFound, "用户不存在")
		return
	}
	WriteJSON(w, http.StatusOK, s.userBrief(u))
}

// GetMyRepos 处理 GET /api/v1/user/repos。
func (s *Server) GetMyRepos(w http.ResponseWriter, r *http.Request, u *domain.User) {
	list, err := s.Repos.ListJoined(r.Context(), u.Key)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "读取仓库列表失败")
		return
	}
	out := make([]RepositoryBrief, 0, len(list))
	for i := range list {
		out = append(out, s.repoBrief(r.Context(), &list[i]))
	}
	WriteJSON(w, http.StatusOK, out)
}

// GetRepo 处理 GET /api/v1/repos/{owner}/{repo}。
//
// 公开端点，但**私有仓库对无权限者不可见** —— 返回 404 而不是 403，
// 避免泄露"该仓库存在"这一事实。
//
// 令牌是**可选**的：带了令牌就按令牌所属用户判定权限，
// 没带就按匿名处理（当前所有仓库都是私有可见性，匿名一律不可读）。
func (s *Server) GetRepo(w http.ResponseWriter, r *http.Request) {
	owner := strings.TrimSpace(r.PathValue("owner"))
	name := strings.TrimSpace(r.PathValue("repo"))
	if owner == "" || name == "" {
		WriteError(w, http.StatusUnprocessableEntity, "仓库名不能为空")
		return
	}
	repository, ok := s.resolveRepo(w, r, owner, name)
	if !ok {
		return
	}
	WriteJSON(w, http.StatusOK, s.repoBrief(r.Context(), repository))
}

// resolveRepo 按 owner 名 + 仓库名解析仓库，并校验当前调用者的读权限。
//
// owner 段既可以是用户名也可以是仓库组名 —— CodeFever 的仓库挂在仓库组下，
// 对外用「组名/仓库名」表达，与常见的「owner/仓库名」结构对齐。
//
// 解析顺序：先按组名找仓库，再按用户名找仓库。
func (s *Server) resolveRepo(w http.ResponseWriter, r *http.Request, owner, name string) (*domain.Repository, bool) {
	if s.Repos == nil {
		WriteError(w, http.StatusNotFound, "仓库不存在")
		return nil, false
	}
	repository, err := s.Repos.GetByGroupAndName(r.Context(), owner, name)
	if err != nil || repository == nil {
		repository, err = s.Repos.GetByOwnerAndName(r.Context(), owner, name)
	}
	if err != nil || repository == nil {
		WriteError(w, http.StatusNotFound, "仓库不存在")
		return nil, false
	}
	if !s.canReadRepo(r, repository) {
		WriteError(w, http.StatusNotFound, "仓库不存在")
		return nil, false
	}
	return repository, true
}

// canReadRepo 判断当前调用者是否有该仓库的读权限。
//
// 令牌是可选的：带了令牌就按令牌所属用户判定，没带就按匿名处理。
// 本项目当前所有仓库都是私有可见性（cc_repositories 无 private 列），
// 因此匿名访问一律不可读 —— 判定仍然走统一的角色解析，
// 避免将来增加公开仓库时再改一遍。
func (s *Server) canReadRepo(r *http.Request, repository *domain.Repository) bool {
	if s.Access == nil {
		return false
	}
	user := s.optionalUser(r)
	if user == nil {
		return false
	}
	role, err := s.Access.RepositoryRole(r.Context(), repository.Key, user.Key)
	if err != nil {
		return false
	}
	return domain.CheckPermission(domain.UACRepoRead, role)
}

// optionalUser 解析令牌（如果带了），返回令牌所属用户。
//
// 与 authenticate 的区别：令牌缺失或无效时**不报错**，只是返回 nil
// （匿名访问）。用于「公开但私有资源不可见」的端点。
func (s *Server) optionalUser(r *http.Request) *domain.User {
	if s.Tokens == nil || s.Users == nil {
		return nil
	}
	plaintext := BearerToken(r)
	if plaintext == "" {
		return nil
	}
	token, err := s.Tokens.BySHA256(r.Context(), repo.HashTokenPlaintext(plaintext))
	if err != nil {
		return nil
	}
	user, err := s.Users.Get(r.Context(), token.UKey)
	if err != nil || user.Status == domain.CommonStatusDelete {
		return nil
	}
	return user
}

// userBrief 组装用户简要信息。
func (s *Server) userBrief(u *domain.User) UserBrief {
	avatar := ""
	if u.Avatar != nil {
		avatar = *u.Avatar
	}
	return UserBrief{
		ID:        userNumericID(u.Key),
		Login:     u.Name,
		FullName:  u.Name,
		Email:     u.Email,
		AvatarURL: avatar,
	}
}

// repoBrief 组装仓库简要信息。
func (s *Server) repoBrief(ctx context.Context, repository *domain.Repository) RepositoryBrief {
	owner := ""
	if g, err := s.Groups.Get(ctx, repository.GroupKey); err == nil && g != nil {
		owner = g.Name
	}
	if owner == "" {
		owner = repository.Name
	}
	defaultBranch := ""
	if repository.DefaultBranch != nil {
		defaultBranch = *repository.DefaultBranch
	}
	fullName := owner + "/" + repository.Name
	return RepositoryBrief{
		ID:            userNumericID(repository.Key),
		Owner:         owner,
		Name:          repository.Name,
		FullName:      fullName,
		Description:   repository.Description,
		Private:       true,
		Fork:          repository.ForkFromKey != nil,
		Empty:         false,
		DefaultBranch: defaultBranch,
		HTMLURL:       strings.TrimSuffix(s.SiteURL, "/") + "/" + fullName,
		CloneURL:      strings.TrimSuffix(s.SiteURL, "/") + "/" + fullName + ".git",
	}
}

// userNumericID 把 32 位 hex 主键映射为一个稳定的数字 ID。
//
// 对外的公开 API 用自增数字 ID 标识用户与仓库；CodeFever 内部全程用 32 位 hex 主键。
// 这里对**整个主键**取 SHA-256，再取前 8 字节解释成 int64，保证：
//   - 同一主键恒得同一数字（可跨请求稳定引用）；
//   - 熵来自完整主键而非前缀 —— 只取前缀会让"仅末位不同"的主键碰撞；
//   - 数值不溢出 int64（直接解释 8 字节，不做算术）。
//
// 64 位数值对公开 API 而言绰绰有余：即使有 10 亿个对象，
// 碰撞概率也在 10⁻⁸ 量级以下。
func userNumericID(key string) int64 {
	if key == "" {
		return 0
	}
	sum := sha256.Sum256([]byte(key))
	return int64(binary.BigEndian.Uint64(sum[:8]))
}
