package publicapi

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"net/http"
	"sort"
	"strconv"
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
// 失败返回 error，由 requireUser 统一写成 401 + `{message,url}` 错误体
// （错误体带文档地址，便于调用方自助排查；不带响应体的写法与公开 API 其余
// 错误响应不一致，因此这里不采用）。
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
	DisplayName   string `json:"display_name"`
	FullName      string `json:"full_name"`
	Description   string `json:"description"`
	Private       bool   `json:"private"`
	Fork          bool   `json:"fork"`
	Empty         bool   `json:"empty"`
	DefaultBranch string `json:"default_branch"`
	ForkCount     int    `json:"fork_count"`
	OpenMRCount   int    `json:"open_mr_count"`
	UpdatedAt     int64  `json:"updated_at"`
	HTMLURL       string `json:"html_url"`
	CloneURL      string `json:"clone_url"`
}

// GroupBrief 是公开 API 的仓库组简要信息。
type GroupBrief struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	DisplayName     string `json:"display_name"`
	Description     string `json:"description"`
	Type            int    `json:"type"`
	RepositoryCount int    `json:"repository_count"`
	MemberCount     int    `json:"member_count"`
	HTMLURL         string `json:"html_url"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
}

// ---------------------------------------------------------------------------
// 端点
// ---------------------------------------------------------------------------

// notFound 处理未命中任何端点的公开 API 路径（由路由表兜底注册）。
//
// 为什么要显式兜底：Go 的 ServeMux 默认回 `text/plain` 的 "404 page not found"，
// 调用方按 `{message,url}` 解析会直接失败 —— 而"路径拼错"恰恰是最常见的错误。
func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	WriteError(w, http.StatusNotFound, "未知的 API 端点，请查阅文档确认路径")
}

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
//
// 支持可选查询参数：q（关键字过滤）、sort（updated|created|name）、page / per_page（分页）。
// 批量查询仓库组、fork 数与开放 MR 数，消除逐仓库 N+1。
func (s *Server) GetMyRepos(w http.ResponseWriter, r *http.Request, u *domain.User) {
	list, err := s.Repos.ListJoined(r.Context(), u.Key)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "读取仓库列表失败")
		return
	}
	out := s.repoBriefsBatch(r.Context(), list)
	out = filterAndPaginateRepoBriefs(out, r)
	WriteJSON(w, http.StatusOK, out)
}

// GetMyGroups 处理 GET /api/v1/groups。
//
// 返回当前令牌可见的全部仓库组（自有 ∪ 加入），支持可选 q / page / per_page。
func (s *Server) GetMyGroups(w http.ResponseWriter, r *http.Request, u *domain.User) {
	if s.Groups == nil {
		WriteJSON(w, http.StatusOK, []GroupBrief{})
		return
	}
	owned, err := s.Groups.ListOwned(r.Context(), u.Key)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "读取仓库组列表失败")
		return
	}
	joined, err := s.Groups.ListJoined(r.Context(), u.Key)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "读取仓库组列表失败")
		return
	}
	seen := make(map[string]bool, len(owned)+len(joined))
	merged := make([]domain.Group, 0, len(owned)+len(joined))
	for _, g := range append(owned, joined...) {
		if seen[g.Key] {
			continue
		}
		seen[g.Key] = true
		merged = append(merged, g)
	}
	out := s.groupBriefsBatch(r.Context(), merged)
	out = filterAndPaginateGroupBriefs(out, r)
	WriteJSON(w, http.StatusOK, out)
}

// GetGroup 处理 GET /api/v1/groups/{group}。
//
// 私有仓库组对无权限者一律返回 404（不泄露存在性）。
func (s *Server) GetGroup(w http.ResponseWriter, r *http.Request) {
	groupName := strings.TrimSpace(r.PathValue("group"))
	if groupName == "" {
		WriteError(w, http.StatusUnprocessableEntity, "仓库组名不能为空")
		return
	}
	g, ok := s.resolveGroup(w, r, groupName)
	if !ok {
		return
	}
	briefs := s.groupBriefsBatch(r.Context(), []domain.Group{*g})
	if len(briefs) == 0 {
		WriteError(w, http.StatusNotFound, "仓库组不存在")
		return
	}
	WriteJSON(w, http.StatusOK, briefs[0])
}

// GetGroupRepos 处理 GET /api/v1/groups/{group}/repos。
//
// 返回指定仓库组内当前调用者可读的仓库列表；无权限或组不存在均返回 404。
func (s *Server) GetGroupRepos(w http.ResponseWriter, r *http.Request) {
	groupName := strings.TrimSpace(r.PathValue("group"))
	if groupName == "" {
		WriteError(w, http.StatusUnprocessableEntity, "仓库组名不能为空")
		return
	}
	g, ok := s.resolveGroup(w, r, groupName)
	if !ok {
		return
	}
	if s.Repos == nil {
		WriteJSON(w, http.StatusOK, []RepositoryBrief{})
		return
	}
	list, err := s.Repos.ListInGroup(r.Context(), g.Key)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "读取组内仓库失败")
		return
	}
	out := s.repoBriefsBatch(r.Context(), list)
	out = filterAndPaginateRepoBriefs(out, r)
	WriteJSON(w, http.StatusOK, out)
}

func (s *Server) resolveGroup(w http.ResponseWriter, r *http.Request, groupName string) (*domain.Group, bool) {
	if s.Groups == nil || s.Access == nil {
		WriteError(w, http.StatusNotFound, "仓库组不存在")
		return nil, false
	}
	user := s.optionalUser(r)
	if user == nil {
		WriteError(w, http.StatusNotFound, "仓库组不存在")
		return nil, false
	}
	g, err := s.Groups.SearchByName(r.Context(), groupName)
	if err != nil || g == nil {
		WriteError(w, http.StatusNotFound, "仓库组不存在")
		return nil, false
	}
	role, err := s.Access.GroupRole(r.Context(), g.Key, user.Key)
	if err != nil || !domain.CheckPermission(domain.UACRepoRead, role) {
		WriteError(w, http.StatusNotFound, "仓库组不存在")
		return nil, false
	}
	return g, true
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

// repoBrief 组装单个仓库简要信息。
func (s *Server) repoBrief(ctx context.Context, repository *domain.Repository) RepositoryBrief {
	list := s.repoBriefsBatch(ctx, []domain.Repository{*repository})
	if len(list) > 0 {
		return list[0]
	}
	return RepositoryBrief{}
}

// repoBriefsBatch 批量组装仓库简要信息（避免逐条查组 / fork / MR 的 N+1）。
func (s *Server) repoBriefsBatch(ctx context.Context, list []domain.Repository) []RepositoryBrief {
	if len(list) == 0 {
		return []RepositoryBrief{}
	}
	rKeys := make([]string, 0, len(list))
	gKeys := make([]string, 0, len(list))
	for i := range list {
		rKeys = append(rKeys, list[i].Key)
		gKeys = append(gKeys, list[i].GroupKey)
	}
	var groups map[string]*domain.RepositoryGroup
	var forkCounts map[string]int
	var mrCounts map[string]domain.RepositoryMergeRequestCount
	if s.Repos != nil {
		groups, _ = s.Repos.GroupsByKeys(ctx, gKeys)
		forkCounts, _ = s.Repos.ForkCounts(ctx, rKeys)
		mrCounts, _ = s.Repos.MergeRequestCounts(ctx, rKeys)
	}

	baseURL := strings.TrimSuffix(s.SiteURL, "/")
	out := make([]RepositoryBrief, 0, len(list))
	for i := range list {
		r := &list[i]
		owner := ""
		if g, ok := groups[r.GroupKey]; ok && g != nil {
			owner = g.Name
		}
		if owner == "" && s.Groups != nil {
			if g, err := s.Groups.Get(ctx, r.GroupKey); err == nil && g != nil {
				owner = g.Name
			}
		}
		if owner == "" {
			owner = r.Name
		}
		defaultBranch := ""
		if r.DefaultBranch != nil {
			defaultBranch = *r.DefaultBranch
		}
		fullName := owner + "/" + r.Name
		out = append(out, RepositoryBrief{
			ID:            userNumericID(r.Key),
			Owner:         owner,
			Name:          r.Name,
			DisplayName:   r.DisplayName,
			FullName:      fullName,
			Description:   r.Description,
			Private:       true,
			Fork:          r.ForkFromKey != nil,
			Empty:         defaultBranch == "",
			DefaultBranch: defaultBranch,
			ForkCount:     forkCounts[r.Key],
			OpenMRCount:   mrCounts[r.Key].Open,
			UpdatedAt:     domain.PhpTime(r.Updated),
			HTMLURL:       baseURL + "/" + fullName,
			CloneURL:      baseURL + "/" + fullName + ".git",
		})
	}
	return out
}

// groupBriefsBatch 批量组装仓库组简要信息。
func (s *Server) groupBriefsBatch(ctx context.Context, list []domain.Group) []GroupBrief {
	if len(list) == 0 {
		return []GroupBrief{}
	}
	gKeys := make([]string, 0, len(list))
	for i := range list {
		gKeys = append(gKeys, list[i].Key)
	}
	repoCounts := map[string]int{}
	membersByGroup := map[string][]domain.GroupMember{}
	if s.Groups != nil {
		if rc, err := s.Groups.RepoCountsByGroups(ctx, gKeys); err == nil {
			repoCounts = rc
		}
		if mb, err := s.Groups.ListMembersByGroups(ctx, gKeys); err == nil {
			membersByGroup = mb
		}
	}

	baseURL := strings.TrimSuffix(s.SiteURL, "/")
	out := make([]GroupBrief, 0, len(list))
	for i := range list {
		g := &list[i]
		// 成员数 = 创建者 + 未删除的去重成员
		memberSet := map[string]bool{g.OwnerKey: true}
		for _, m := range membersByGroup[g.Key] {
			if m.Status == domain.CommonStatusNormal {
				memberSet[m.UKey] = true
			}
		}
		out = append(out, GroupBrief{
			ID:              userNumericID(g.Key),
			Name:            g.Name,
			DisplayName:     g.DisplayName,
			Description:     g.Description,
			Type:            g.Type,
			RepositoryCount: repoCounts[g.Key],
			MemberCount:     len(memberSet),
			HTMLURL:         baseURL + "/groups/" + g.Name,
			CreatedAt:       g.Created.Unix(),
			UpdatedAt:       g.Updated.Unix(),
		})
	}
	return out
}

func filterAndPaginateRepoBriefs(list []RepositoryBrief, r *http.Request) []RepositoryBrief {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if q != "" {
		filtered := make([]RepositoryBrief, 0, len(list))
		for _, item := range list {
			if strings.Contains(strings.ToLower(item.FullName), q) ||
				strings.Contains(strings.ToLower(item.DisplayName), q) ||
				strings.Contains(strings.ToLower(item.Description), q) {
				filtered = append(filtered, item)
			}
		}
		list = filtered
	}
	switch strings.TrimSpace(r.URL.Query().Get("sort")) {
	case "name":
		sort.SliceStable(list, func(i, j int) bool {
			return strings.ToLower(list[i].FullName) < strings.ToLower(list[j].FullName)
		})
	case "updated":
		sort.SliceStable(list, func(i, j int) bool {
			return list[i].UpdatedAt > list[j].UpdatedAt
		})
	}
	return applyOptionalPagination(list, r)
}

func filterAndPaginateGroupBriefs(list []GroupBrief, r *http.Request) []GroupBrief {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if q != "" {
		filtered := make([]GroupBrief, 0, len(list))
		for _, item := range list {
			if strings.Contains(strings.ToLower(item.Name), q) ||
				strings.Contains(strings.ToLower(item.DisplayName), q) ||
				strings.Contains(strings.ToLower(item.Description), q) {
				filtered = append(filtered, item)
			}
		}
		list = filtered
	}
	return applyOptionalPagination(list, r)
}

func applyOptionalPagination[T any](list []T, r *http.Request) []T {
	rawPage := strings.TrimSpace(r.URL.Query().Get("page"))
	rawPerPage := strings.TrimSpace(r.URL.Query().Get("per_page"))
	if rawPage == "" && rawPerPage == "" {
		return list
	}
	p, _ := strconv.Atoi(rawPage)
	pp, _ := strconv.Atoi(rawPerPage)
	page, perPage := domain.ClampPaging(p, pp)
	start := (page - 1) * perPage
	if start >= len(list) {
		return []T{}
	}
	end := start + perPage
	if end > len(list) {
		end = len(list)
	}
	return list[start:end]
}

// userNumericID 把 32 位 hex 主键映射为一个稳定的数字 ID。
//
// 对外的公开 API 用数字 ID 标识用户与仓库；CodeFever 内部全程用 32 位 hex 主键。
// 这里对**整个主键**取 SHA-256，取前 8 字节后**右移 11 位**，保证：
//   - 同一主键恒得同一数字（可跨请求稳定引用）；
//   - 熵来自完整主键而非前缀 —— 只取前缀会让"仅末位不同"的主键碰撞；
//   - **恒为非负**：直接 int64(uint64) 会把最高位当符号位，约一半对象拿到负 ID；
//   - **落在 JS 安全整数内**：右移 11 位后最大 2^53-1 = 9007199254740991，
//     JavaScript 的 Number 能精确表示（超过就会在 JSON.parse 里被四舍五入 ——
//     实测 7341916824063314155 → 7341916824063314000，调用方拿 ID 做比较或建索引
//     会直接错）。
//
// 53 位熵（≈9.0e15）对自托管实例绰绰有余：按生日问题估算，100 万对象的碰撞概率
// 约 5.6e-5，1000 万对象约 0.56%；而 CodeFever 的仓库/用户数量级远低于此。
// 若将来确实需要更大空间，正确的做法是把 ID 改成字符串而不是放宽到这个上限之外。
func userNumericID(key string) int64 {
	if key == "" {
		return 0
	}
	sum := sha256.Sum256([]byte(key))
	return int64(binary.BigEndian.Uint64(sum[:8]) >> 11)
}
