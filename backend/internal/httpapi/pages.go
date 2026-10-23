// Package httpapi 实现 HTTP 层：服务端渲染页面与表单 AJAX 响应。
//
// 本包把 PHP 的服务端渲染页面逐字移植为 Go html/template。
// application/views 下的 13 个模板与 templates/ 下的 14 个 .html 一一对应
// （多出来的 absent_include.html 是给 4 个 include 目标文件缺失的页面占位，见该文件注释）。
//
// 模板目录放在本包内（backend/internal/httpapi/templates/），
// 这样 //go:embed templates/*.html 就能直接引用它：
// go:embed 只能嵌入"包目录之内及子目录"的文件，模板若放在 backend/templates/，
// 就必须另起一个位于 backend/ 根目录的包来承载 embed 变量，
// 反而多出一个只服务于渲染的空壳包。放在一起也让"模板 ↔ PHP 源文件"的对应关系更好维护。
package httpapi

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"codefever-next/backend/internal/domain"
)

// templatesFS 承载本包内的模板文件。
//
//go:embed templates/*.html
var templatesFS embed.FS

var (
	tmplOnce sync.Once
	tmplSet  *template.Template
	tmplErr  error
)

// pageTemplates 解析并返回模板集合。
//
// 只解析一次；解析失败返回 error 而不是 panic —— 模板是编译期常量，
// 解析失败意味着模板文件被改坏了，此时应让请求失败并暴露问题，而不是让整个进程崩溃。
func pageTemplates() (*template.Template, error) {
	tmplOnce.Do(func() {
		tmplSet, tmplErr = template.New("codefever").Funcs(template.FuncMap{
			// mbSubstr 等价 PHP 的 mb_substr()，按字符（而非字节）截取。
			// countries_dropdown_menu.php 用它取国家名首字母分组。
			"mbSubstr": mbSubstr,
			// safeHTML 让一段 HTML 原样输出。
			// Go 的 html/template 会把模板里的 HTML 注释从输出中删掉，
			// 而 PHP 会原样输出，两者必须对齐才能保持页面逐字节一致。
			"safeHTML": safeHTML,
			// js 让一段 JavaScript 原样输出。
			// Go 的 html/template 按 JS 规范**丢弃注释**
			// （"Comments behave like white space and are discarded"），
			// 而 PHP 侧原样输出 —— 用 template.JS 包一层即可保留注释。
			"js": js,
		}).ParseFS(templatesFS, "templates/*.html")
	})
	return tmplSet, tmplErr
}

// Translations 是语言文案表。
//
// 逐字搬运自 application/language/{zh-cn,english}/{base,user}_lang.php。
type Translations map[string]string

// Get 返回语言条目；缺失时返回空串。
//
// PHP 侧 lang() 找不到条目时返回 FALSE，echo FALSE 输出空串，因此这里返回 ""。
func (t Translations) Get(key string) string { return t[key] }

// langZhCN 是简体中文语言包。
//
// 逐字搬运自 application/language/zh-cn/base_lang.php 与 user_lang.php。
var langZhCN = Translations{
	"base_cn":                                   "中文",
	"base_confirm":                              "确定",
	"base_directory":                            "目录",
	"base_en":                                   "English",
	"base_error_message":                        "错误提示",
	"base_error_message_tips":                   "抱歉，请重新尝试访问网站，或者刷新重试",
	"base_go_home":                              "返回主页",
	"base_go_prepage":                           "返回上一页",
	"base_message":                              "消息",
	"user_bind_registering":                     "注册中...",
	"user_form_code":                            "请输入验证码",
	"user_form_code_sent":                       "已发送",
	"user_form_email_code":                      "请输入邮箱验证码",
	"user_form_email_error":                     "您输入的邮箱有误，请重新输入",
	"user_form_email_tel":                       "请输入邮箱",
	"user_form_forget_password":                 "忘记密码？",
	"user_form_get_code":                        "获取验证码",
	"user_form_get_code_re":                     "重新发送",
	"user_form_login_now":                       "去登录",
	"user_form_page_title":                      "登录",
	"user_form_page_welcome":                    "欢迎登录",
	"user_form_password":                        "请输入登录密码",
	"user_form_password_min_error":              "登录密码不得少于6位，请重新输入",
	"user_form_password_re_error":               "两次密码不一致",
	"user_form_password_space_error":            "密码不能有空格",
	"user_form_register_email":                  "请输入邮箱",
	"user_form_register_page_title":             "注册",
	"user_form_register_page_welcome":           "欢迎注册",
	"user_form_sending":                         "正在发送",
	"user_form_verfiy":                          "验证",
	"user_js_2fa_retrieval_link_expired":        "验证链接已失效，请重新登录",
	"user_js_2fa_verify_fail":                   "验证码错误",
	"user_js_account_not_exist":                 "该账号尚未注册，请注册账户",
	"user_js_email_had_exist":                   "该账号已注册，请直接登录",
	"user_js_login_fail":                        "您输入的账号或密码有误，请重新输入",
	"user_js_login_success":                     "登录成功",
	"user_js_register_failed":                   "注册失败",
	"user_js_register_success":                  "注册成功",
	"user_login_btn_login":                      "登 录",
	"user_login_no_account":                     "还没有账号？",
	"user_login_register_new":                   "去注册",
	"user_password_retrieval":                   "重置密码",
	"user_password_retrieval_by_email":          "通过邮箱找回",
	"user_password_retrieval_by_tel":            "通过手机号找回",
	"user_password_retrieval_enter_password":    "请输入新密码",
	"user_password_retrieval_login_now":         "返回登录",
	"user_password_retrieval_re_enter_password": "请再次输入新密码",
	"user_password_retrieval_reset_password":    "确认修改",
	"user_password_retrieval_title":             "找回密码",
	"user_register_free":                        "注 册",
	"user_register_have_account":                "已有账号？",
	"user_register_password":                    "请输入6-18位登录密码",
	"user_register_password_regex":              "密码长度不能小于 6 个字符",
	"user_register_re_password":                 "请再次输入密码",
}

// langEnglish 是英文语言包。
//
// 逐字搬运自 application/language/english/base_lang.php 与 user_lang.php。
var langEnglish = Translations{
	"base_cn":                                   "中文",
	"base_confirm":                              "Confirm",
	"base_directory":                            "Directory",
	"base_en":                                   "English",
	"base_error_message":                        "Error message",
	"base_error_message_tips":                   "Sorry, please try again to visit the site, or refresh retry",
	"base_go_home":                              "Home",
	"base_go_prepage":                           "Return to previous page",
	"base_message":                              "Message",
	"user_bind_registering":                     "Registering...",
	"user_form_code":                            "Please enter verification code",
	"user_form_code_sent":                       "Has been sent",
	"user_form_email_code":                      "Please enter the email verification code",
	"user_form_email_error":                     "The email address you entered is incorrect, please re-enter",
	"user_form_email_tel":                       "Please enter email/phone number",
	"user_form_forget_password":                 "forget password?",
	"user_form_get_code":                        "get verification code",
	"user_form_get_code_re":                     "Resend",
	"user_form_login_now":                       "Go to login",
	"user_form_page_title":                      "Login",
	"user_form_page_welcome":                    "Welcome to login",
	"user_form_password":                        "Please enter your password",
	"user_form_password_min_error":              "Login password must not be less than 6 digits, please re-enter",
	"user_form_password_re_error":               "The two passwords are inconsistent",
	"user_form_password_space_error":            "Password cannot have spaces",
	"user_form_register_email":                  "Please enter email or phone number",
	"user_form_register_page_title":             "Reigster",
	"user_form_register_page_welcome":           "Welcome to register",
	"user_form_sending":                         "sending",
	"user_form_verfiy":                          "Verfiy",
	"user_js_2fa_retrieval_link_expired":        "The verification link has expired, please log in again",
	"user_js_2fa_verify_fail":                   "Verification code error",
	"user_js_account_not_exist":                 "The account has not been registered, please register an account",
	"user_js_email_had_exist":                   "The mailbox has been registered, please log in directly",
	"user_js_login_fail":                        "The account or password you entered is incorrect, please re-enter",
	"user_js_login_success":                     "login successful",
	"user_js_register_failed":                   "registration failed",
	"user_js_register_success":                  "registration success",
	"user_login_btn_login":                      "Login",
	"user_login_no_account":                     "No account yet?",
	"user_login_register_new":                   "Go to register",
	"user_password_retrieval":                   "Reset Password",
	"user_password_retrieval_by_email":          "Retrieve via email",
	"user_password_retrieval_by_tel":            "Retrieve by phone number",
	"user_password_retrieval_enter_password":    "Please enter a new password",
	"user_password_retrieval_login_now":         "Back to login",
	"user_password_retrieval_re_enter_password": "Please enter the new password again",
	"user_password_retrieval_reset_password":    "Confirm the changes",
	"user_password_retrieval_title":             "Retrieve password",
	"user_register_free":                        "Sign up",
	"user_register_have_account":                "Already have an account?",
	"user_register_password":                    "Please enter 6-18 digits login password",
	"user_register_password_regex":              "The password must be 6-18 digits",
	"user_register_re_password":                 "Please enter the password again",
}

// NewTranslations 按语言标识返回语言包。
//
// PHP 侧 Base::_detectLang() 只加载 'zh-cn' 与 'english' 两个语言包
// （其余语言一律回落到 english），这里保持同样的行为。
func NewTranslations(lang string) Translations {
	switch strings.ToLower(lang) {
	case "english", "en-us", "en":
		return langEnglish
	default:
		return langZhCN
	}
}

// DocMenuItem 是文档页左侧菜单的一项。
//
// 逐字对应 application/controllers/doc.php::_extractMenu() 解析出来的
// array('name' => ..., 'path' => ...) 结构。
type DocMenuItem struct {
	// Name 是菜单显示名。
	Name string
	// Path 是菜单对应的目录路径。
	Path string
	// Submenu 是二级菜单；为空表示该项没有子菜单。
	Submenu []DocMenuItem
}

// PageData 承载页面模板需要的数据。
//
// 字段与 PHP 侧各控制器传给 load->view() 的数组一一对应；
// 缺省值（零值）与 PHP 侧变量未定义时的表现一致。
type PageData struct {
	// Lang 是语言包，对应 PHP 的 lang() 函数。
	Lang Translations

	// Host 对应 PHP 常量 YAML_HOST（config.yaml 的 host）。
	Host string
	// ReleaseNumber 对应 PHP 常量 RELEASENUMBER（env.yaml 的 releaseNumber）。
	ReleaseNumber string
	// AppName 对应 PHP 常量 APP_NAME。
	AppName string
	// AllowRegister 对应 PHP 常量 YAML_ALLOWREGISTER（config.yaml 的 allowRegister）。
	AllowRegister bool

	// Email 是登录/注册页回填的邮箱（PHP $email）。
	Email string
	// Bind 是登录页“去注册”链接上的 bind 查询参数（PHP $bind）。
	//
	// PHP 侧 User::login() 并未给 $data['bind'] 赋值，因此它恒为空串。
	Bind string
	// Key 是 2FA 与重置密码的会话 key（PHP $key）。
	//
	// PHP 侧控制层用 /^[0-9a-f]{32}$/i 校验，不合法时置空。
	Key string
	// Mode 是重置密码页的模式（PHP $mode）：空串=按邮箱找回，'reset'=设置新密码。
	Mode string
	// Message 是页面上的提示文案（PHP $message）。
	Message string

	// RedirectURL 是成功提示页的跳转地址（PHP $redirectURL）。
	RedirectURL string
	// RedirectButtonTitle 是成功提示页的跳转按钮文案（PHP $redirectButtonTitle）。
	RedirectButtonTitle string

	// DocMenu 是文档页左侧菜单（PHP $menu）。
	DocMenu []DocMenuItem
	// DocSegments 是文档页的 URL 路径分段（PHP $segments）。
	DocSegments []string
	// Doc 是文档页的 Markdown 正文（PHP $doc）。
	Doc string

	// Countries 是国家下拉菜单的数据（PHP $countries）。
	Countries []domain.Country
}

// L 返回语言文案，等价 PHP 的 lang($key)。
//
// 缺失的键返回空串 —— PHP 侧 lang() 返回 FALSE，echo FALSE 同样是空串。
func (p PageData) L(key string) string { return p.Lang.Get(key) }

// RedirectTitle 返回成功提示页的跳转按钮文案。
//
// PHP 侧 base/success_message.php 里 $redirectButtonTitle 为空时会回落到 lang('base_confirm')，
// 这里保持同样的回落逻辑。
func (p PageData) RedirectTitle() string {
	if p.RedirectButtonTitle != "" {
		return p.RedirectButtonTitle
	}
	return p.L("base_confirm")
}

// DocLang 返回文档页 URL 的语言段（PHP $segments[1]，通常是 "cn" 或 "en"）。
func (p PageData) DocLang() string { return p.docSegment(1) }

// DocRest 返回文档页去掉语言段之后的路径（PHP implode('/', array_slice($segments, 2))）。
func (p PageData) DocRest() string {
	if len(p.DocSegments) <= 2 {
		return ""
	}
	return strings.Join(p.DocSegments[2:], "/")
}

// DocItemHref 返回文档页二级菜单项的链接（PHP $segments[1] . '/' . $item2['path']）。
func (p PageData) DocItemHref(path string) string { return p.DocLang() + "/" + path }

// docSegment 安全地取路径分段，越界时返回空串。
func (p PageData) docSegment(i int) string {
	if i < 0 || i >= len(p.DocSegments) {
		return ""
	}
	return p.DocSegments[i]
}

// safeHTML 让一段 HTML 原样输出。
//
// PHP 的 echo 不转义，模板里可以直接输出 HTML 注释；
// 而 Go 的 html/template 默认会把模板中的 HTML 注释从输出里删除，
// 因此这里用 template.HTML 把注释原样写回，保证渲染结果与 PHP 一致。
//
// 只允许用于服务端写死的静态片段（如 <!-- CSS ... -->），
// 绝不能用来承载用户输入。
func safeHTML(s string) template.HTML { return template.HTML(s) }

// js 让一段 JavaScript 原样输出。
//
// Go 的 html/template 在 JS 上下文里按 JS 规范**丢弃注释**
// （"Comments behave like white space and are discarded"），
// 而 PHP 侧原样输出。两边不一致会导致脚本内容不同 ——
// 因此用 template.JS 把注释原样写回，保证渲染结果与 PHP 一致。
//
// 只允许用于服务端写死的静态片段，绝不能用来承载用户输入。
func js(s string) template.JS { return template.JS(s) }

// mbSubstr 等价 PHP 的 mb_substr()，按字符截取字符串。
func mbSubstr(s string, start, length int) string {
	runes := []rune(s)
	if start < 0 {
		start = 0
	}
	if start >= len(runes) {
		return ""
	}
	end := len(runes)
	if length >= 0 && start+length < end {
		end = start + length
	}
	return string(runes[start:end])
}

// RenderLogin 渲染登录页（PHP newpage/user/login.php）。
func RenderLogin(w http.ResponseWriter, data PageData) error {
	return renderPage(w, "login.html", data)
}

// RenderRegister 渲染注册页（PHP newpage/user/register.php）。
func RenderRegister(w http.ResponseWriter, data PageData) error {
	return renderPage(w, "register.html", data)
}

// RenderResetPassword 渲染重置密码页（PHP newpage/user/reset_password.php）。
func RenderResetPassword(w http.ResponseWriter, data PageData) error {
	return renderPage(w, "reset_password.html", data)
}

// RenderVerify 渲染 2FA 验证码页（PHP newpage/user/verify.php）。
func RenderVerify(w http.ResponseWriter, data PageData) error {
	return renderPage(w, "verify.html", data)
}

// RenderDocDetail 渲染文档页（PHP doc/detail.php）。
func RenderDocDetail(w http.ResponseWriter, data PageData) error {
	return renderPage(w, "doc_detail.html", data)
}

// RenderErrorMessage 渲染错误提示页（PHP base/error_message.php）。
func RenderErrorMessage(w http.ResponseWriter, data PageData) error {
	return renderPage(w, "error_message.html", data)
}

// RenderSuccessMessage 渲染成功提示页（PHP base/success_message.php）。
func RenderSuccessMessage(w http.ResponseWriter, data PageData) error {
	return renderPage(w, "success_message.html", data)
}

// RenderDisableMessage 渲染禁用提示页（PHP base/disable_message.php）。
func RenderDisableMessage(w http.ResponseWriter, data PageData) error {
	return renderPage(w, "disable_message.html", data)
}

// RenderNotFound 渲染 404 页（PHP nopage/index.php）。
//
// HTTP 状态码为 404 —— 与 PHP 侧一致：
// 实测 GET /api/nope/nope → 404 + Content-Type: text/html。
func RenderNotFound(w http.ResponseWriter, data PageData) error {
	return renderPageStatus(w, http.StatusNotFound, "index.html", data)
}

// RenderError404 渲染 CI 的 404 错误页（PHP application/errors/error_404.php）。
//
// 与 RenderNotFound 是**两个不同的页面**，不能混用：
//   - 控制器不存在 → CI 的 404_override（routes.php: nopage）→ nopage/index.php；
//   - 控制器存在、内部判定为 404 → Exceptions::show_404() → errors/error_404.php。
//
// 实测：GET /doc/cn/（容器里没有 doc/ 目录）→ CI 的 error_404.php 页面；
//
// GET /USER/register（控制器不存在）→ nopage/index.php 页面。
//
// HTTP 状态码为 404 —— 与 PHP 侧 show_404() 一致
// （system/core/Exceptions.php:137 的 set_status_header(404)）。
func RenderError404(w http.ResponseWriter, data PageData) error {
	return renderPageStatus(w, http.StatusNotFound, "error_404.html", data)
}

// RenderCountriesDropdownMenu 渲染国家下拉菜单片段
// （PHP newpage/user/countries_dropdown_menu.php）。
//
// 注意：该片段在 PHP 侧没有被任何模板 include，属于历史遗留代码；
// 这里仍然提供渲染入口，便于后续把它接回页面。
func RenderCountriesDropdownMenu(w http.ResponseWriter, data PageData) error {
	return renderPage(w, "countries_dropdown_menu.html", data)
}

// renderPage 渲染指定模板并写出响应。
//
// 响应头与 PHP 侧一致：Content-Type: text/html; charset=UTF-8
// （PHP 未显式设置时就是默认值）。
func renderPage(w http.ResponseWriter, name string, data PageData) error {
	return renderPageStatus(w, http.StatusOK, name, data)
}

// renderPageStatus 渲染指定模板并以给定的 HTTP 状态码写出响应。
func renderPageStatus(w http.ResponseWriter, status int, name string, data PageData) error {
	tmpl, err := pageTemplates()
	if err != nil {
		return fmt.Errorf("解析页面模板失败: %w", err)
	}
	if tmpl.Lookup(name) == nil {
		return fmt.Errorf("页面模板不存在: %s", name)
	}

	// 先渲染到内存再写响应头：渲染失败时不应该留下半个响应体。
	var buf strings.Builder
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return fmt.Errorf("渲染页面模板 %s 失败: %w", name, err)
	}

	h := w.Header()
	h.Set("Content-Type", "text/html; charset=UTF-8")
	h.Set("Content-Length", strconv.Itoa(buf.Len()))
	w.WriteHeader(status)
	_, _ = w.Write([]byte(buf.String()))
	return nil
}

// AjaxEnvelope 是登录/注册/重置密码等表单 AJAX 的响应信封。
//
// 逐字对应 PHP 侧 Base::_ajax_message()：
//
//	{"code":0,"message":"登录成功","extra":{"redirect":"/repositories"}}
//
// 与 API 通用信封 contract.Envelope 不同：这里**没有** request-id，也**没有** data。
type AjaxEnvelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Extra   any    `json:"extra"`
}

// NewAjaxEnvelope 构造 AJAX 信封。
//
// extra 为 nil 时用空数组填充，以复刻 PHP 中 json_encode(array()) 的输出 ——
// 它是 [] 而不是 {}，前端 www/view/src/helpers/NetworkHelper.js 依赖这一点。
func NewAjaxEnvelope(code int, message string, extra any) *AjaxEnvelope {
	if extra == nil {
		extra = []any{}
	}
	return &AjaxEnvelope{
		Code:    code,
		Message: message,
		Extra:   extra,
	}
}

// WriteAjaxJSON 写出 AJAX 信封响应。
//
// 响应头与 PHP 侧保持一致：PHP 的 _ajax_message() 只是 echo，
// 没有设置任何响应头，因此 Content-Type 是 PHP 的默认值
// text/html; charset=UTF-8（**不是** application/json）。
//
// 响应体用 PHP 风格的 JSON 编码（见 json.go）：非 ASCII 转成 \uXXXX、
// 斜杠转成 "\/"，与 PHP 的 json_encode 默认行为逐字节一致。
func WriteAjaxJSON(w http.ResponseWriter, env *AjaxEnvelope) error {
	payload, err := env.Encode()
	if err != nil {
		return fmt.Errorf("序列化 AJAX 信封失败: %w", err)
	}
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=UTF-8")
	h.Set("Content-Length", strconv.Itoa(len(payload)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(payload))
	return nil
}

// Encode 按 PHP 的 json_encode 规则序列化信封。
//
// 字段顺序固定为 code → message → extra，与 PHP 的
// json_encode(['code' => .., 'message' => .., 'extra' => ..]) 一致。
func (e *AjaxEnvelope) Encode() (string, error) {
	extra, err := phpJSONEncode(e.Extra)
	if err != nil {
		return "", err
	}
	return `{"code":` + strconv.Itoa(e.Code) +
		`,"message":` + phpJSONString(e.Message) +
		`,"extra":` + extra + `}`, nil
}
