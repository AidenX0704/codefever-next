package httpapi

import (
	"net/http"
	"strings"
)

// handleFallback 处理未匹配到具体路由的请求。
//
// 与 PHP 侧的 CodeIgniter 默认行为一致：未匹配路径按 /<controller>/<method>
// 形状交给 web 控制器分发器（见 web.go）：
//   - 控制器存在、方法存在 → 执行方法；
//   - 控制器存在、方法不存在 → JSON 501 "Unknown method"；
//   - 控制器不存在 → HTML 404 页（CI 的 404_override = nopage）。
//
// 实测证据：GET /api/nope/nope → HTTP 404 + Content-Type: text/html。
func (s *Server) handleFallback(w http.ResponseWriter, r *http.Request) {
	s.handleWebRoute(w, r)
}

// handleNotFound 输出 HTML 404 页。
//
// 优先使用页面模板层移植的 nopage/index.php 等价模板；
// 未注入 PageDataProvider 时退回到自包含实现，保证行为始终正确。
func (s *Server) handleNotFound(w http.ResponseWriter, r *http.Request) {
	if s.PageDataProvider != nil {
		if err := RenderNotFound(w, s.PageDataProvider()); err == nil {
			return
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.Header().Set("Cache-Control", "no-cache, must-revalidate")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(notFoundHTML))
}

// notFoundHTML 是 nopage/index.php 的等价页面。
//
// 结构与 PHP 版一致（class 名与文案逐字保持），静态资源路径同样保持 ——
// 页面引用的是 /static/assets/images/404_pic.png，由 nginx 托管。
const notFoundHTML = `<!DOCTYPE html>
<html>
<head>
    <title>CodeFever</title>
</head>
<body style="background-color:#fff">
 <div style="height:100px;margin-bottom100px;">&nbsp;</div>
 <div ng-controller="nopageCtrl">
     <div class="container error-v4" style="margin-top:100px;">
         <div class="row">
             <div class="col-lg-12 col-xs-12 col-md-12 col-sm-12 text-center">
                 <p class="img-responsive center-block animated fadeInDown"><img src="/static/assets/images/404_pic.png"></p>
                 <h1 class="sorry">The page you&rsquo;re looking for is not here.</h1>
                 <p class="regret">抱歉，页面不见了~</p>
                 <a href="/" role="button" class="btn btn-primary btn-border">
                     <i class="fa fa-home fa-fw"></i>
                     返回首页
                 </a>
             </div>
         </div>
     </div>
 </div>
</body>
</html>
`

// isAPIPath 判断路径是否属于 API 命名空间。
//
// 用途：未知 API 路径必须返回 HTML 404（复刻 CI 的回落行为），
// 而普通页面路径也应如此 —— 因此该函数主要用于日志与诊断。
func isAPIPath(path string) bool {
	return strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/apiv1/")
}
