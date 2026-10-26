package publicapi

import (
	"net/http"
	"strings"
)

// Router 是公开 API（/api/v1）的路由表。
//
// 路径风格对齐 GitHub 等常见约定：
//
//	GET    /api/v1/user
//	GET    /api/v1/users/{username}
//	GET    /api/v1/user/repos
//	GET    /api/v1/repos/{owner}/{repo}
//
// 路由由 httpapi.Server 以 **子树** 形式挂载（mux.HandleFunc("/api/v1/", ...)），
// 因此这里用同一个前缀再注册一遍完整路径是安全的：
// Go 的 ServeMux 会先按最长匹配选中子 mux，再由子 mux 自己分发。
//
// 注意：本包**不复用** httpapi 的 /api/{ctrl}/{action} 分发器 ——
// 那套分发器是 PHP 兼容契约的一部分（动词式路径 + multipart 编码 +
// {request-id,code,message,data} 信封），与公开 API 的语义完全不同。
type Router struct {
	server *Server
	mux    *http.ServeMux
}

// NewRouter 创建公开 API 路由。
func NewRouter(s *Server) *Router {
	r := &Router{server: s, mux: http.NewServeMux()}
	r.register()
	return r
}

// ServeHTTP 实现 http.Handler。
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

// register 注册全部公开端点。
func (r *Router) register() {
	r.mux.HandleFunc("/api/v1/user", r.server.requireUser(r.server.GetUser))
	r.mux.HandleFunc("/api/v1/user/repos", r.server.requireUser(r.server.GetMyRepos))
	r.mux.HandleFunc("/api/v1/users/{username}", r.server.GetUserByName)
	r.mux.HandleFunc("/api/v1/repos/{owner}/{repo}", r.server.GetRepo)
}

// Wrap 把公开 API 分流到自己的 mux，其余请求交给 next。
//
// 【为什么不直接 mux.Handle("/api/v1/", ...)】
//
// Go 的 ServeMux 会拒绝有歧义的模式：主服务器已经注册了
//
//	/api/{ctrl}
//	/api/{ctrl}/{action}
//
// 再注册 /api/v1/ 会 panic —— 因为 /api/v1/ 与 /api/{ctrl}/{action}
// 都能匹配 "/api/v1/action"，且**谁都不比谁更具体**
// （前者能匹配 "/api/v1/"，后者匹配不了；后者能匹配 "/api/ctrl/action"，
// 前者也匹配不了）。
//
// 因此公开 API 改为**在进入主 mux 之前**分流：
// 见 httpapi.Server.publicAPIDispatch —— 它按路径前缀直接交给公开 API 的
// 自己的 mux，主 mux 完全看不到 /api/v1 请求。
//
// 这也顺带彻底避开了 normalizeAPIPath 的段数截断（见 IsPublicAPIPath）。
func (r *Router) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if IsPublicAPIPath(req.URL.Path) {
			r.mux.ServeHTTP(w, req)
			return
		}
		next.ServeHTTP(w, req)
	})
}

// IsPublicAPIPath 判断路径是否属于公开 API 命名空间。
//
// 用途：normalizeAPIPath 必须对公开 API **豁免**段数截断 ——
// 它会把 /api/ 开头的路径截到 3 段，于是
// /api/v1/repos/{owner}/{repo} 会被砍成 /api/v1/repos 而静默 404。
func IsPublicAPIPath(path string) bool {
	return path == "/api/v1" || strings.HasPrefix(path, "/api/v1/")
}
