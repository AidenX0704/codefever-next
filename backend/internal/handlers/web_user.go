package handlers

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"codefever-next/backend/internal/auth"
	"codefever-next/backend/internal/config"
	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
	"codefever-next/backend/internal/httpapi"
	"codefever-next/backend/internal/mailer"
	"codefever-next/backend/internal/repo"
	"codefever-next/backend/internal/session"
	"codefever-next/backend/internal/storage"
)

// WebUserHandler 实现 user 控制器的 web 方法。
//
// 对应 PHP 侧 application/controllers/user.php 的 register / verify /
// resetPassword / getResetPasswordCode 四个方法。
//
// 与 /api/* 的 UserHandler 是两套东西：这里的响应是
// Base::_ajax_message() 的 {code, message, extra} 信封，或者是服务端渲染的页面。
type WebUserHandler struct {
	Users    *repo.UserRepo
	Sessions *session.Store
	Mailer   mailer.Sender
	Disk     *storage.Disk
	Cfg      *config.Config
}

// NewWebUserHandler 创建 user 控制器。
func NewWebUserHandler(cfg *config.Config, users *repo.UserRepo, sessions *session.Store, disk *storage.Disk, mail mailer.Sender) *WebUserHandler {
	return &WebUserHandler{Users: users, Sessions: sessions, Mailer: mail, Disk: disk, Cfg: cfg}
}

// RegisterWeb 把 user 控制器注册到服务器。
//
// 方法名按 PHP 的原样给出（resetPassword / getResetPasswordCode），
// 注册时会统一小写化，因此 /user/resetpassword 与 /user/RESETPASSWORD
// 都能命中（PHP 的 method_exists 大小写不敏感）。
//
// login / logout 不在这里：它们由 main.go 以精确页面路由
// （RegisterPage）注册，分发器会在控制器方法表里再查一次页面路由表，
// 因此 /user/LOGIN 也能命中同一个处理器。
func (h *WebUserHandler) RegisterWeb(s *httpapi.Server) {
	s.RegisterWebController(&httpapi.WebController{
		Name: "user",
		Methods: map[string]http.HandlerFunc{
			"register":             h.register,
			"verify":               h.verify,
			"resetPassword":        h.resetPassword,
			"getResetPasswordCode": h.getResetPasswordCode,
		},
	})
}

// register 实现 user/register。
//
// 逐字对应 application/controllers/user.php:132-201：
//
//	GET  → 渲染注册页（已登录则 302 /repositories）
//	POST → 注册账号并返回 {code, message, extra}
//
// 注意 PHP 的注册开关判断在 GET/POST 分支**之前**：
// 关闭注册时，POST 也会 302 跳到 /user/login（不是返回 JSON）。
func (h *WebUserHandler) register(w http.ResponseWriter, r *http.Request) {
	// 逐字对应 user.php:134-137。
	if !h.Cfg.SiteConfig().AllowRegister {
		redirect(w, "/user/login")
		return
	}
	if r.Method == http.MethodPost {
		h.registerPost(w, r)
		return
	}

	// GET 分支：user.php:184-200。
	email := trimGet(r, "email")
	if _, ok := sessionUser(r.Context(), r, h.Sessions, h.Users, h.Cfg.Env.Session.Name); ok {
		redirect(w, "/repositories")
		return
	}
	data := pageData(h.Cfg, currentLang(r))
	data.Email = email
	_ = httpapi.RenderRegister(w, data)
}

// registerPost 实现 POST /user/register。
//
// 逐字对应 application/controllers/user.php:139-183。
func (h *WebUserHandler) registerPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	email := trimPost(r, "email")
	if email == "" {
		_ = writeAjax(w, 1, "请输入邮箱", nil)
		return
	}

	newUser := repo.NewUser{}
	var (
		existing *domain.User
		err      error
	)
	switch {
	case webEmailPattern.MatchString(email):
		// 逐字对应 user.php:147-149。
		newUser.Email = email
		existing, err = h.Users.GetByEmail(ctx, email)
	case telPattern.MatchString(email):
		// 逐字对应 user.php:150-152。
		newUser.Tel = email
		existing, err = h.Users.GetByTel(ctx, email)
	default:
		_ = writeAjax(w, 2, "您输入的邮箱有误，请重新输入", nil)
		return
	}
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		// 数据库出错：PHP 侧 addUser 失败会走 code 5，这里同样处理。
		_ = writeAjax(w, 5, "注册失败", nil)
		return
	}
	if existing != nil {
		// 逐字对应 user.php:157-159。
		_ = writeAjax(w, 3, "该账号已注册，请直接登录", nil)
		return
	}

	password := trimPost(r, "password")
	if password == "" {
		// 逐字对应 user.php:161-165。
		_ = writeAjax(w, 4, "请输入登录密码", nil)
		return
	}

	// 逐字对应 user.php:167-171。
	newUser.Key = contract.NewKey()
	hashed, herr := auth.HashPasswordForFormat(password, h.Cfg.PasswordHashFormat())
	if herr != nil {
		_ = writeAjax(w, 5, "注册失败", nil)
		return
	}
	newUser.Password = hashed
	newUser.Name = "user_" + newUser.Key[len(newUser.Key)-6:]
	newUser.Avatar = h.createAvatar(email)
	newUser.RegisterIP = clientIP(r)

	if err := h.Users.AddUser(ctx, newUser); err != nil {
		_ = writeAjax(w, 5, "注册失败", nil)
		return
	}

	// 逐字对应 user.php:180-183：注册成功后直接建立登录会话。
	sess, serr := h.Sessions.Create(ctx, newUser.Key, r.RemoteAddr, r.UserAgent())
	if serr != nil {
		_ = writeAjax(w, 5, "注册失败", nil)
		return
	}
	h.writeSessionCookie(w, r, sess.Key)

	_ = writeAjax(w, 0, "注册成功", httpapi.AjaxObject{{Key: "redirect", Value: "/repositories"}})
}

// verify 实现 user/verify（2FA 二次验证）。
//
// 逐字对应 application/controllers/user.php:85-130：
//
//	GET  → 渲染验证码页（已登录则 302 /repositories）
//	POST → 校验 2FA 验证码
func (h *WebUserHandler) verify(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		h.verifyPost(w, r)
		return
	}

	// GET 分支：user.php:113-129。
	key := trimGet(r, "key")
	if !keyPattern.MatchString(key) {
		key = ""
	}
	if _, ok := sessionUser(r.Context(), r, h.Sessions, h.Users, h.Cfg.Env.Session.Name); ok {
		redirect(w, "/repositories")
		return
	}
	data := pageData(h.Cfg, currentLang(r))
	data.Key = key
	_ = httpapi.RenderVerify(w, data)
}

// verifyPost 实现 POST /user/verify。
//
// 逐字对应 application/controllers/user.php:86-112。
//
// PHP 把 2FA 的临时 key 放在 $_SESSION 里；新后端用服务端会话表承载
// （登录接口在 2FA 分支创建会话并把会话 id 作为 extra.key 回传），
// 因此这里用会话 id 反查用户。
func (h *WebUserHandler) verifyPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	key := trimPost(r, "key")
	code := trimPost(r, "code")

	sess, err := h.Sessions.Get(ctx, key)
	if err != nil {
		_ = writeAjax(w, 100, "验证链接已失效，请重新登录",
			httpapi.AjaxObject{{Key: "redirect", Value: "/user/login"}})
		return
	}
	user, err := h.Users.Get(ctx, sess.UKey)
	if err != nil {
		_ = writeAjax(w, 100, "验证链接已失效，请重新登录",
			httpapi.AjaxObject{{Key: "redirect", Value: "/user/login"}})
		return
	}
	if !user.HasTwoFA() {
		_ = writeAjax(w, 100, "验证链接已失效，请重新登录",
			httpapi.AjaxObject{{Key: "redirect", Value: "/user/login"}})
		return
	}
	if !verifyTwoFACode(*user.TwoFA, code, time.Now()) {
		_ = writeAjax(w, 1, "验证码错误", nil)
		return
	}

	// 校验通过：把会话变成正式登录态（下发 Cookie）。
	// PHP 侧是 session_delete($key) + set_userdata('userInfo')，
	// 新后端的会话本身就是登录态，因此这里只需下发 Cookie。
	h.writeSessionCookie(w, r, sess.Key)

	_ = writeAjax(w, 0, "登录成功", httpapi.AjaxObject{{Key: "redirect", Value: "/repositories"}})
}

// resetPassword 实现 user/resetPassword。
//
// 逐字对应 application/controllers/user.php:203-289：
//
//	GET  → 渲染重置口令页
//	POST → mode=reset 时设置新口令；否则校验邮箱验证码并发临时 key
func (h *WebUserHandler) resetPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		h.resetPasswordPost(w, r)
		return
	}

	// GET 分支：user.php:270-288。
	mode := trimGet(r, "mode")
	key := trimGet(r, "key")
	if !keyPattern.MatchString(key) {
		key = ""
	}
	data := pageData(h.Cfg, currentLang(r))
	data.Key = key
	data.Mode = mode
	_ = httpapi.RenderResetPassword(w, data)
}

// resetPasswordPost 实现 POST /user/resetPassword。
//
// 逐字对应 application/controllers/user.php:205-269。
func (h *WebUserHandler) resetPasswordPost(w http.ResponseWriter, r *http.Request) {
	if trimPost(r, "mode") == "reset" {
		h.resetPasswordApply(w, r)
		return
	}
	h.resetPasswordRequest(w, r)
}

// resetPasswordApply 实现"设置新口令"分支。
//
// 逐字对应 application/controllers/user.php:208-238。
//
// 注意 code 100 / 0 / 8 的文案在 PHP 侧是 lang('user_js_password_retrieval_*')，
// 而这三个键在 application/language/{zh-cn,english}/user_lang.php 里**不存在**，
// lang() 返回 FALSE，echo 后是空串 —— 新后端逐字复刻这个空文案。
func (h *WebUserHandler) resetPasswordApply(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	key := trimPost(r, "key")

	sess, err := h.Sessions.Get(ctx, key)
	if err != nil {
		_ = writeAjax(w, 100, "",
			httpapi.AjaxObject{{Key: "redirect", Value: "/user/resetPassword"}})
		return
	}
	user, err := h.Users.Get(ctx, sess.UKey)
	if err != nil {
		_ = writeAjax(w, 100, "",
			httpapi.AjaxObject{{Key: "redirect", Value: "/user/resetPassword"}})
		return
	}

	password := formValue(r, "password")
	if len(password) < 6 {
		_ = writeAjax(w, 7, "密码长度不能小于6个字符", nil)
		return
	}

	hashed, herr := auth.HashPasswordForFormat(password, h.Cfg.PasswordHashFormat())
	if herr != nil {
		_ = writeAjax(w, 8, "", nil)
		return
	}
	if err := h.Users.UpdateFields(ctx, user.Key, map[string]any{"u_password": hashed}); err != nil {
		_ = writeAjax(w, 8, "", nil)
		return
	}

	// 临时 key 用完即焚（逐字对应 user.php:234 的 session_delete）。
	_ = h.Sessions.Delete(ctx, key)

	_ = writeAjax(w, 0, "",
		httpapi.AjaxObject{{Key: "redirect", Value: "/user/login?email=" + user.Email}})
}

// resetPasswordRequest 实现"用邮箱验证码换取临时 key"分支。
//
// 逐字对应 application/controllers/user.php:239-269。
//
// 临时 key 用服务端会话承载（PHP 侧是 $_SESSION[$key]，
// 有效期 600 秒），这里用会话的有效期表达同样的过期语义。
func (h *WebUserHandler) resetPasswordRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	email := trimPost(r, "email")
	if email == "" {
		// user.php:244 用的是 lang('user_form_email')，
		// 该键在语言包里不存在（只有 user_form_email_tel），因此文案是空串。
		_ = writeAjax(w, 1, "", nil)
		return
	}
	emailCode := trimPost(r, "emailCode")
	if emailCode == "" {
		_ = writeAjax(w, 1, "请输入邮箱验证码", nil)
		return
	}

	user, err := h.Users.GetByEmail(ctx, email)
	if err != nil {
		_ = writeAjax(w, 4, "该账号尚未注册，请注册账户", nil)
		return
	}
	if !auth.VerifyEmailCode(email, emailCode, time.Now(), h.Cfg.Env.TOTP.Salt) {
		// user.php:262 用的是 lang('user_js_code_verfiy_error')，该键不存在 → 空串。
		_ = writeAjax(w, 8, "", nil)
		return
	}

	// 逐字对应 user.php:265-268：生成临时 key 并写进会话。
	sess, serr := h.Sessions.Create(ctx, user.Key, r.RemoteAddr, r.UserAgent())
	if serr != nil {
		_ = writeAjax(w, 8, "", nil)
		return
	}

	_ = writeAjax(w, 0, "登录成功", httpapi.AjaxObject{
		{Key: "redirect", Value: "/user/resetPassword?mode=reset&key=" + sess.Key},
	})
}

// getResetPasswordCode 实现 user/getResetPasswordCode。
//
// 逐字对应 application/controllers/user.php:291-312。
//
// code 3 / 4 的文案在 PHP 侧是 lang('user_js_email_error') 与
// lang('user_js_email_not_exist')，两个键在语言包里都不存在 → 空串。
func (h *WebUserHandler) getResetPasswordCode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	email := trimPost(r, "email")
	if !webEmailPattern.MatchString(email) {
		_ = writeAjax(w, 3, "", nil)
		return
	}
	_, err := h.Users.GetByEmail(ctx, email)
	if err != nil {
		_ = writeAjax(w, 4, "", nil)
		return
	}

	// 逐字对应 user.php:306-310：发送验证码邮件。
	code := auth.GenerateEmailCode(email, time.Now(), h.Cfg.Env.TOTP.Salt)
	if h.Mailer != nil {
		_ = h.Mailer.Send(ctx, mailer.Message{
			To:      email,
			From:    h.Cfg.SiteConfig().Email.From,
			Subject: "【CodeFever Next】重置密码",
			Body:    "您的验证码是：" + code,
		})
	}

	_ = writeAjax(w, 0, "", nil)
}

// createAvatar 生成用户头像并返回文件 key。
//
// 逐字对应 application/models/user_model.php:233-252（createAvatar）：
// 生成一个 key、把图片写进 avatar 存储、把 key 作为 u_avatar 存库。
//
// 差异：PHP 用 MDAvtars 扩展渲染，Go 侧用内置点阵生成一枚对称识别图 ——
// 图片内容不同，但"key → 图片文件"的契约一致（/file/fetch/avatar/<key> 能取到图）。
func (h *WebUserHandler) createAvatar(email string) string {
	if email == "" {
		return ""
	}
	key := contract.NewKey()
	img := identicon(email, 256)
	content, err := encodePNG(img)
	if err != nil {
		return ""
	}
	if h.Disk == nil {
		return key
	}
	if err := h.Disk.Save("avatar", key, content); err != nil {
		return ""
	}
	return key
}

// writeSessionCookie 下发会话 Cookie。
func (h *WebUserHandler) writeSessionCookie(w http.ResponseWriter, r *http.Request, key string) {
	session.WriteCookie(w, session.CookieOptions{
		Name:   h.Cfg.Env.Session.Name,
		Domain: h.Cfg.Env.Session.Domain,
		Secure: isSecureRequest(r),
		Path:   "/",
		MaxAge: h.Sessions.TTL(),
	}, key)
}

// verifyTwoFACode 校验 2FA（TOTP）验证码。
//
// 逐字对应 application/libraries/service/Utility/GoogleAuthenticator.php:128-146：
// 时间片 30 秒，允许前后 5 个时间片的漂移。
func verifyTwoFACode(secret, code string, at time.Time) bool {
	if len(code) != 6 {
		return false
	}
	totp := auth.NewTOTP(secret)
	totp.Window = 5
	return totp.Verify(code, at)
}

// clientIP 返回请求来源 IP。
//
// 对应 PHP 的 $this->input->ip_address()。
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		if r.RemoteAddr != "" {
			return r.RemoteAddr
		}
		return "0.0.0.0"
	}
	return strings.TrimSpace(host)
}
