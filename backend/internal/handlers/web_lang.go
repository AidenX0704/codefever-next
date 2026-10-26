package handlers

import (
	"net/http"
	"strings"

	"codefever-next/backend/internal/config"
	"codefever-next/backend/internal/httpapi"
)

// LangHandler 实现 lang 控制器。
//
// 逐字对应 application/controllers/lang.php:13-28。
type LangHandler struct {
	Cfg *config.Config
}

// NewLangHandler 创建 lang 控制器。
func NewLangHandler(cfg *config.Config) *LangHandler { return &LangHandler{Cfg: cfg} }

// RegisterWeb 把 lang 控制器注册到服务器。
func (h *LangHandler) RegisterWeb(s *httpapi.Server) {
	s.RegisterWebController(&httpapi.WebController{
		Name: "lang",
		Methods: map[string]http.HandlerFunc{
			// 键是 PHP 侧的方法名（select 是 Go 的关键字，方法本身改名为 selectLang）。
			"select": h.selectLang,
		},
	})
}

// langCookieName 是语言偏好 Cookie 的名字。
//
// PHP 侧把语言偏好存在 CI 会话里（application/controllers/lang.php:21-25 的
// $this->session->set_userdata('lang', ...)）。新后端的会话表以登录用户为主，
// 而匿名用户同样需要记住语言，因此改用独立 Cookie —— 对外可见效果一致。
const langCookieName = "codefever_lang"

// 语言偏好的存储值，逐字对应 PHP 的常量
// application/config/constants.php:62-63（LANG_CN = 1、LANG_EN = 2）。
//
// 注意这里存的是**整数常量值**而不是 'zh-cn'/'english' —— PHP 侧就是这么存的
// （lang.php:21-24 的 set_userdata('lang', LANG_CN)），
// 而 Base::_detectLang() 判定时用的是
// in_array(strtolower($userLang), $cnLangArr)，
// cn_langs 里只有 'zh-cn'/'zh-tw'/... 这些**字符串**，
// 因此整数 1 匹配不上 —— 选了"中文"反而会落到英文。
// 这是 PHP 侧的一处缺陷，这里逐字复刻（见 currentLang 的说明）。
const (
	langValueCN = "1"
	langValueEN = "2"
)

// selectLang 实现 lang/select。
//
// 逐字对应 application/controllers/lang.php:13-28：
//
//	没有 l 参数 → 渲染 base/error_message.php 页面，提示"参数错误"
//	有 l 参数   → 记住语言，然后 302 跳回 /
//
// PHP 用的是 !$lang 判定，"0" 也算没有参数（PHP 的 falsy 规则）。
func (h *LangHandler) selectLang(w http.ResponseWriter, r *http.Request) {
	lang := r.URL.Query().Get("l")
	if lang == "" || lang == "0" {
		// 前端切换后不再有"参数错误"静态页：直接回首页，由 SPA 决定去向
		// （未登录 → /user/login，已登录 → /repositories）。
		redirect(w, "/")
		return
	}
	if lang == "cn" {
		h.writeLangCookie(w, langValueCN)
	} else {
		h.writeLangCookie(w, langValueEN)
	}
	redirect(w, "/")
}

// writeLangCookie 记住语言偏好。
func (h *LangHandler) writeLangCookie(w http.ResponseWriter, lang string) {
	http.SetCookie(w, &http.Cookie{
		Name:     langCookieName,
		Value:    lang,
		Path:     "/",
		MaxAge:   int((30 * 24 * 60 * 60)),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// currentLang 返回当前请求使用的语言。
//
// 逐字对应 Base::_detectLang()（application/controllers/base.php:23-34）：
//
//	$userLang = $this->session->userdata('lang');
//	$userLang = $userLang ? $userLang : 'zh-cn';
//	$actualLang = in_array(strtolower($userLang), $cnLangArr) ? 'zh-cn' : 'english';
//
// cn_langs 只有 'zh-cn'/'zh-tw'/'zh-hk'/'zh-mo'/'zh-sg'（application/config/lang.php:18-24），
// 而 Lang 控制器存进去的是整数常量 1/2 —— 因此"选择中文"会落到英文。
// 这是 PHP 侧的一处缺陷，这里逐字复刻，保证页面字节输出与 PHP 一致。
func currentLang(r *http.Request) string {
	if c, err := r.Cookie(langCookieName); err == nil && c.Value != "" {
		switch strings.ToLower(c.Value) {
		case "zh-cn", "zh-tw", "zh-hk", "zh-mo", "zh-sg":
			return "zh-cn"
		default:
			return "english"
		}
	}
	return "zh-cn"
}
