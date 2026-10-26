package publicapi

import (
	"net/http"
	"strings"
)

// Router 是公开 API（/api/v1）的路由表。
//
// 路径风格对齐 GitHub 等常见约定；**全部只读**，非 GET/HEAD 一律 405：
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
//
// 【为什么方法约束写在包装器里，而不是用 Go 1.22 的 "GET /api/v1/..." 模式】
//
// 方法模式会让 Go 的 ServeMux 自己回 405，但响应体是 **text/plain** 的
// "Method Not Allowed\n"（实测），直接打破公开 API "错误永远是 {message,url} JSON"
// 的约定；要保住 JSON 就得在子 mux 外再包一层、并自己维护"路径是否存在"来区分
// 404/405，反而更绕。requireGet 只有几行，错误体形态与 Allow 头完全可控，
// 与既有的 requireUser 包装器同一风格。
func (r *Router) register() {
	r.mux.HandleFunc("/api/v1/user", requireGet(r.server.requireUser(r.server.GetUser)))
	r.mux.HandleFunc("/api/v1/user/repos", requireGet(r.server.requireUser(r.server.GetMyRepos)))
	r.mux.HandleFunc("/api/v1/users/{username}", requireGet(r.server.GetUserByName))
	r.mux.HandleFunc("/api/v1/repos/{owner}/{repo}", requireGet(r.server.GetRepo))

	// 兜底：未命中的公开 API 路径返回 JSON 404。
	//
	// Go 的 ServeMux 默认回 text/plain 的 "404 page not found" —— 调用方按
	// {message,url} 解析会直接失败。两个模式都要注册："/api/v1/" 只管子树，
	// 裸的 "/api/v1" 不在其内（不注册则 ServeMux 会 301 到 "/api/v1/"，对 API 无意义）。
	r.mux.HandleFunc("/api/v1", r.server.notFound)
	r.mux.HandleFunc("/api/v1/", r.server.notFound)
}

// requireGet 只放行 GET 与 HEAD，其余方法回 405（JSON 错误体 + Allow 头）。
//
// 方法判定放在鉴权**之前**：方法不匹配时不该走令牌校验（也不该因为"没带令牌"
// 而回 401 —— 那样调用方会误以为是凭据问题）。
//
// HEAD 一并放行：Go 的 http.Server 对 HEAD 自动丢弃响应体，探活脚本常用它；
// 若只放 GET，监控会看到 405。
func requireGet(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			WriteError(w, http.StatusMethodNotAllowed, "该端点只支持 GET 请求")
			return
		}
		h(w, r)
	}
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
