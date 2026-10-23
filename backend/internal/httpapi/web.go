package httpapi

import (
	"net/http"
	"strconv"
	"strings"
)

// 本文件实现 PHP 侧 **web 控制器命名空间** 的路由判定。
//
// 与 /api/* 命名空间的关键差异：
//
//	1. 响应信封不同 —— web 控制器用 Base::_ajax_message()，
//	   输出 {"code":..,"message":..,"extra":..}，没有 request-id、没有 data。
//	2. 路由判定不同 —— CI 先看控制器文件在不在，再看方法在不在：
//	     - 控制器文件不存在 → 走 routes.php 的 404_override（nopage）→ HTML 404 页；
//	     - 控制器存在、方法不存在 → API_Controller::_remap → 501 "Unknown method"。
//	3. 大小写规则不同 —— 控制器名**区分**大小写，方法名**不区分**大小写。
//
// 判定依据：
//   - 控制器名：CI_Router::_validate_request() 用
//     file_exists(APPPATH.'controllers/'.$segments[0].'.php') 探测控制器文件，
//     Linux 文件系统区分大小写。
//     实测 GET /USER/register → 404 HTML（不是注册页），GET /user/Register → 200 注册页。
//   - 方法名：API_Controller::_remap() 用 method_exists()，PHP 的方法名大小写不敏感。
//     实测 GET /user/resetpassword 与 /user/RESETPASSWORD 都命中 resetPassword。
//
// PHP 参考：
//   - application/libraries/api_controller.php:55-63（_remap → 501）
//   - application/libraries/api_controller.php:96-108（errorResponse 信封）
//   - application/config/routes.php:42（404_override = nopage）

// defaultWebController 是 CI 的默认控制器。
//
// 逐字对应 application/config/routes.php:41：$route['default_controller'] = "home"。
// 路径为 "/"（没有控制器段）时，CI 的 _set_default_controller() 会走这里。
const defaultWebController = "home"

// WebController 对应 PHP 侧 application/controllers/<name>.php 的顶层控制器。
type WebController struct {
	// Name 是控制器名，对应 URL 的 <controller> 段。
	//
	// **区分大小写**：CI 用 file_exists() 探测控制器文件，Linux 文件系统区分大小写。
	Name string
	// Methods 是方法名 → 处理器。
	//
	// 键会被统一小写化后存储：PHP 的 method_exists() 大小写不敏感，
	// 查表前小写化即可让 /user/resetpassword 与 /user/RESETPASSWORD
	// 都命中同一个 resetPassword 处理器。
	Methods map[string]http.HandlerFunc
	// Before 是控制器**构造函数**的等价物。
	//
	// PHP 侧控制器构造函数在方法分发之前执行，并且可以自行结束请求：
	// Doc 控制器就是靠它把 "/doc/xxx" 重定向到 "/doc/xxx/" 的
	// （application/controllers/doc.php:9-22，命中后 exit()，
	// 因此连"方法不存在"的 501 都不会走到）。
	//
	// 返回 true 表示请求已被处理，不再分发方法。
	Before func(w http.ResponseWriter, r *http.Request) bool
}

// RegisterWebController 注册一个 web 控制器。
//
// 控制器名区分大小写（与 PHP 一致），方法名大小写不敏感。
func (s *Server) RegisterWebController(c *WebController) {
	if c == nil || c.Name == "" {
		return
	}
	methods := make(map[string]http.HandlerFunc, len(c.Methods))
	for name, h := range c.Methods {
		if h == nil {
			continue
		}
		methods[strings.ToLower(name)] = h
	}
	s.webControllers[c.Name] = &WebController{Name: c.Name, Methods: methods, Before: c.Before}
}

// WebController 返回已注册的 web 控制器。
func (s *Server) WebController(name string) (*WebController, bool) {
	c, ok := s.webControllers[name]
	return c, ok
}

// handleWebRoute 分发一个落在 web 控制器命名空间的请求。
//
// 路径为 "/" 时没有控制器段，CI 会走 routes.php 的 default_controller（home）与其
// index 方法（CI_Router::_set_default_controller()）。
func (s *Server) handleWebRoute(w http.ResponseWriter, r *http.Request) {
	ctrl, method, ok := splitWebPath(r.URL.Path)
	if !ok {
		ctrl, method = defaultWebController, "index"
	}
	s.dispatchWebController(w, r, ctrl, method)
}

// dispatchWebController 按 CI 的三段判定分发请求。
func (s *Server) dispatchWebController(w http.ResponseWriter, r *http.Request, ctrl, method string) {
	c, ok := s.webControllers[ctrl]
	if !ok {
		// 控制器文件不存在 → CI 走 404_override（nopage）→ HTML 404 页。
		s.handleNotFound(w, r)
		return
	}

	// 控制器构造函数先于方法分发执行（PHP 侧 new Controller() 在 _remap 之前）。
	if c.Before != nil && c.Before(w, r) {
		return
	}

	h, ok := c.Methods[strings.ToLower(method)]
	if !ok {
		// RegisterPage 注册的精确页面路由同样是控制器方法
		// （PHP 侧 /user/login 就是 User::login），
		// 因此这里再查一次页面路由表，让 /user/LOGIN 也能命中。
		h, ok = s.pageRouteHandler(ctrl, method)
	}
	if !ok {
		// 控制器存在、方法不存在 → 501 "Unknown method"。
		s.writeUnknownMethod(w)
		return
	}
	h(w, r)
}

// pageRouteHandler 按控制器名 + 方法名查找已注册的页面路由。
//
// 页面路由表以完整路径为键（"/user/login"），这里用小写拼出同样的键再查，
// 从而让方法名的大小写变体（/user/LOGIN）也命中同一个处理器。
func (s *Server) pageRouteHandler(ctrl, method string) (http.HandlerFunc, bool) {
	h, ok := s.pageRoutesCI["/"+strings.ToLower(ctrl)+"/"+strings.ToLower(method)]
	return h, ok
}

// splitWebPath 把请求路径拆成 CI 形状的 <controller>/<method>。
//
// 对应 CI 的 URI::_explode_segments()：路径先 trim('/')，再按 '/' 拆分并
// **丢弃空段**。因此：
//
//	"/"          → ok=false        —— 没有控制器段，走 default_controller
//	"/user"      → ("user", "index")
//	"/user/"     → ("user", "index")  —— 尾部斜杠产生的空段被丢弃
//	"/user/x"    → ("user", "x")
//	"/doc/cn/"   → ("doc", "cn")
//	"/doc/cn/a"  → ("doc", "cn")      —— 第三段及以后是方法参数
//
// 缺第二段时方法名为 "index"：CI_Router::_set_request() 在 segments[1] 不存在时
// 会补上 'index'。
func splitWebPath(path string) (ctrl, method string, ok bool) {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	kept := make([]string, 0, len(segments))
	for _, seg := range segments {
		if seg != "" {
			kept = append(kept, seg)
		}
	}
	if len(kept) == 0 {
		return "", "", false
	}
	ctrl = kept[0]
	method = "index"
	if len(kept) > 1 {
		method = kept[1]
	}
	return ctrl, method, true
}

// unknownMethodBody 是 501 响应的响应体。
//
// 逐字对应 API_Controller::errorResponse(501, 'Unknown method') 的 json_encode 输出。
const unknownMethodBody = `{"code":501,"message":"Unknown method","data":[]}`

// writeUnknownMethod 输出 CI 的 501 响应。
//
// 逐字对应 application/libraries/api_controller.php:96-108：
//
//	errorResponse(501, 'Unknown method')
//	  → response(['code' => 501, 'message' => 'Unknown method', 'data' => []])
//	     → header('Content-Type: application/json')
//	     → HTTP 状态码是 **200**（response() 里 $http_code 为 null 时被置为 200），
//	       501 只出现在 body 的 code 字段里
//	     → header('Content-Length: ' . strlen($output))
//
// 注意这个响应与 /api/* 的信封不同：没有 request-id，data 是空数组 []。
func (s *Server) writeUnknownMethod(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("Content-Length", strconv.Itoa(len(unknownMethodBody)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(unknownMethodBody))
}

// Redirect 输出一次 302 跳转。
//
// 对应 PHP 的 header('Location: ...')：PHP 在发送 Location 头时会**自动**把
// 状态码置为 302，响应体为空。Content-Type 保持 PHP 的默认值
// text/html; charset=UTF-8（PHP 未显式设置时的默认值）。
func Redirect(w http.ResponseWriter, location string) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=UTF-8")
	h.Set("Location", location)
	h.Set("Content-Length", "0")
	w.WriteHeader(http.StatusFound)
}
