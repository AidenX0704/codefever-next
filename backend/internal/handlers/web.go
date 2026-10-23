// 本文件是 web 控制器（PHP 侧 application/controllers/ 下的顶层控制器）
// 共用的辅助函数。
//
// web 控制器的响应有三种形态，与 /api/* 命名空间完全不同：
//
//  1. 表单 AJAX：Base::_ajax_message() → {"code":..,"message":..,"extra":..}；
//  2. 302 跳转：PHP 的 header('Location: ...')（PHP 会自动把状态码置为 302）；
//  3. 服务端渲染页面：load->view()。
package handlers

import (
	"context"
	"net/http"
	"regexp"
	"strings"

	"codefever-next/backend/internal/config"
	"codefever-next/backend/internal/domain"
	"codefever-next/backend/internal/httpapi"
	"codefever-next/backend/internal/repo"
	"codefever-next/backend/internal/session"
)

// AppName 对应 PHP 常量 APP_NAME。
//
// 逐字对应 application/config/constants.php:58。
const AppName = "CodeFever"

// webEmailPattern 是 web 控制器侧的邮箱格式判定。
//
// 逐字对应 application/controllers/user.php:35、147、295 的
// preg_match('/^\S+@\S+\.\S+$/', $email)。
//
// 注意与 /api/* 侧的 emailPattern（更严格）是两套规则：
// web 控制器用 PHP 原样搬过来的宽松规则，/api/* 用的是 api/user.php 的严格规则。
var webEmailPattern = regexp.MustCompile(`^\S+@\S+\.\S+$`)

// telPattern 是 PHP 侧的手机号格式判定。
//
// 逐字对应 application/controllers/user.php:150 的
// preg_match('/^[0-9]{5,11}$/', $email)。
var telPattern = regexp.MustCompile(`^[0-9]{5,11}$`)

// keyPattern 是 2FA / 重置口令页面的 key 校验。
//
// 逐字对应 application/controllers/user.php:117、276 的
// preg_match('/^[0-9a-f]{32}$/i', $key)。
var keyPattern = regexp.MustCompile(`(?i)^[0-9a-f]{32}$`)

// writeAjax 输出表单 AJAX 信封。
//
// 逐字对应 Base::_ajax_message()（application/controllers/base.php:101-108）：
//
//	{"code":0,"message":"登录成功","extra":{"redirect":"/repositories"}}
//
// extra 为 nil 时是空数组 []（PHP 的 json_encode(array())），
// 前端 www/view/src/helpers/NetworkHelper.js 依赖这一点。
func writeAjax(w http.ResponseWriter, code int, message string, extra any) error {
	return httpapi.WriteAjaxJSON(w, httpapi.NewAjaxEnvelope(code, message, extra))
}

// redirect 输出一条 302 跳转。
//
// 对应 PHP 的 header('Location: ...')：PHP 在发送 Location 头时会**自动**把
// 状态码置为 302，响应体为空。
func redirect(w http.ResponseWriter, location string) {
	httpapi.Redirect(w, location)
}

// pageData 构造页面模板数据。
//
// 逐字对应 Base::__construct() 的 $this->load->vars() 与各控制器的 $data 组装：
// Host/ReleaseNumber/AppName/AllowRegister 来自配置，语言来自会话。
func pageData(cfg *config.Config, lang string) httpapi.PageData {
	return httpapi.PageData{
		Lang:          httpapi.NewTranslations(lang),
		Host:          cfg.Site.Host,
		ReleaseNumber: cfg.Env.ReleaseNumber,
		AppName:       AppName,
		AllowRegister: cfg.Site.AllowRegister,
	}
}

// sessionUser 解析当前请求的登录用户。
//
// 对应 PHP 侧 Base::getLoggedUserInfo()（application/controllers/base.php:36-51）：
// 读会话里的 userInfo.u_key，再查一次用户表 —— 查不到（账号被删）同样视为未登录。
func sessionUser(ctx context.Context, r *http.Request, sessions *session.Store, users *repo.UserRepo, cookieName string) (*domain.User, bool) {
	key := session.ReadCookie(r, cookieName)
	if key == "" {
		return nil, false
	}
	sess, err := sessions.Get(ctx, key)
	if err != nil {
		return nil, false
	}
	u, err := users.Get(ctx, sess.UKey)
	if err != nil {
		return nil, false
	}
	return u, true
}

// trimPost 读取并去掉首尾空白的表单字段。
//
// 对应 PHP 的 trim($this->input->post('key'))。
func trimPost(r *http.Request, key string) string {
	return strings.TrimSpace(formValue(r, key))
}

// trimGet 读取并去掉首尾空白的查询参数。
//
// 对应 PHP 的 trim($this->input->get('key'))。
func trimGet(r *http.Request, key string) string {
	return strings.TrimSpace(r.URL.Query().Get(key))
}

// HomeHandler 实现 home 控制器。
//
// 逐字对应 application/controllers/home.php:12-19：
// 已登录 → 302 /repositories；未登录 → 302 /user/login。
//
// home 同时是 CI 的 default_controller（application/config/routes.php:41），
// 因此路径为 "/" 时也会走到这里。
type HomeHandler struct {
	Sessions *session.Store
	Users    *repo.UserRepo
	Cfg      *config.Config
}

// NewHomeHandler 创建 home 控制器。
func NewHomeHandler(cfg *config.Config, users *repo.UserRepo, sessions *session.Store) *HomeHandler {
	return &HomeHandler{Sessions: sessions, Users: users, Cfg: cfg}
}

// RegisterWeb 把 home 控制器注册到服务器。
func (h *HomeHandler) RegisterWeb(s *httpapi.Server) {
	s.RegisterWebController(&httpapi.WebController{
		Name: "home",
		Methods: map[string]http.HandlerFunc{
			"index": h.index,
		},
	})
}

// index 实现 home/index。
func (h *HomeHandler) index(w http.ResponseWriter, r *http.Request) {
	if _, ok := sessionUser(r.Context(), r, h.Sessions, h.Users, h.Cfg.Env.Session.Name); ok {
		redirect(w, "/repositories")
		return
	}
	redirect(w, "/user/login")
}

// NopageHandler 实现 nopage 控制器。
//
// 逐字对应 application/controllers/nopage.php:12-16：
// 设置 404 状态码并渲染 nopage/index.php。
//
// 它同时是 CI 的 404_override（application/config/routes.php:42）——
// 控制器不存在时，CI 会回落到这里，因此"控制器不存在"的 404 页也是这个模板。
type NopageHandler struct {
	Cfg *config.Config
}

// NewNopageHandler 创建 nopage 控制器。
func NewNopageHandler(cfg *config.Config) *NopageHandler { return &NopageHandler{Cfg: cfg} }

// RegisterWeb 把 nopage 控制器注册到服务器。
func (h *NopageHandler) RegisterWeb(s *httpapi.Server) {
	s.RegisterWebController(&httpapi.WebController{
		Name: "nopage",
		Methods: map[string]http.HandlerFunc{
			"index": h.index,
		},
	})
}

// index 实现 nopage/index。
func (h *NopageHandler) index(w http.ResponseWriter, r *http.Request) {
	_ = httpapi.RenderNotFound(w, pageData(h.Cfg, currentLang(r)))
}

// RegisterBaseController 注册 base 控制器。
//
// base 是 PHP 侧所有 web 控制器的抽象基类（application/controllers/base.php），
// 它**没有**自己的 public 方法，因此任何方法名都会落到 501 "Unknown method"。
//
// 这里显式注册它，是为了让 /base 与 /base/xxx 的行为与 PHP 一致
// （PHP 侧同样因为找不到 index 方法而返回 501）。
func RegisterBaseController(s *httpapi.Server) {
	s.RegisterWebController(&httpapi.WebController{
		Name:    "base",
		Methods: map[string]http.HandlerFunc{},
	})
}
