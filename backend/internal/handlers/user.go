// Package handlers 实现各业务域的 API 端点。
//
// 每个 handler 只负责业务逻辑，协议解析、鉴权与响应封装由 httpapi 包统一处理。
package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"codefever-next/backend/internal/auth"
	"codefever-next/backend/internal/authkeys"
	"codefever-next/backend/internal/config"
	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
	"codefever-next/backend/internal/httpapi"
	"codefever-next/backend/internal/mailer"
	"codefever-next/backend/internal/repo"
	"codefever-next/backend/internal/session"
	"codefever-next/backend/internal/storage"
)

// UserHandler 实现 /api/user/* 端点。
type UserHandler struct {
	Users         *repo.UserRepo
	Notifications *repo.NotificationRepo
	Disk          *storage.Disk
	Mailer        mailer.Sender
	Sessions      *session.Store
	Cfg           *config.Config

	// Tokens 是公开 API 访问令牌仓储（`cc_access_tokens`）。
	//
	// 由 main 在装配时注入；为 nil 时令牌管理动作**不注册**（见 user_token.go），
	// 测试与旧调用点因此不必凭空构造一个数据库。
	Tokens *repo.AccessTokenRepo

	// AuthKeys 负责把 SSH 公钥同步进 /home/<git 用户>/.ssh/authorized_keys。
	//
	// PHP 时代这一步由 misc/modify_authorized_keys.php 守护进程完成
	// （FIFO + SIGUSR1），PHP 运行时下线后改由 Go 后端直接写文件。
	AuthKeys *authkeys.Writer
}

// NewUserHandler 创建用户域处理器。
func NewUserHandler(cfg *config.Config, users *repo.UserRepo, notifications *repo.NotificationRepo, disk *storage.Disk, mail mailer.Sender, sessions *session.Store) *UserHandler {
	return &UserHandler{
		Users:         users,
		Notifications: notifications,
		Disk:          disk,
		Mailer:        mail,
		Sessions:      sessions,
		Cfg:           cfg,
		// env.yaml 的 users.git / cli.git 是公钥同步路径与 PATH= 前缀的来源。
		// users.git 在 Go 后端里第一次被消费 —— 此前只有 PHP 用它。
		AuthKeys: authkeys.NewWriter(cfg.BaseDir, cfg.Env.Users.Git, cfg.Env.CLI.Git),
	}
}

// Register 把用户域端点注册到服务器。
func (h *UserHandler) Register(s *httpapi.Server) {
	actions := map[string]map[string]httpapi.ActionHandler{
		"info":    {"GET": h.info},
		"listKey": {"GET": h.listKey},
	}
	h.registerProfileActions(actions)
	h.registerNotificationActions(actions)
	h.registerEmailActions(actions)
	actions["uploadAvatar"] = map[string]httpapi.ActionHandler{"POST": h.uploadAvatar}
	h.registerTokenActions(actions)

	s.Register(&httpapi.Controller{
		Name:    "user",
		Auth:    httpapi.AuthWebUser,
		Actions: actions,
	})
}

// info 实现 GET /api/user/info。
//
// 逐字对应 application/controllers/api/user.php:32-38：
// 取当前登录用户 → normalize([$userInfo], TRUE) → 输出。
//
// normalize 的 $extra=TRUE 意味着：
//   - 提交邮箱以对象形态 {id, email, isCheck} 返回，且不过滤未验证邮箱；
//   - 额外带上 unReadNotification 计数。
func (h *UserHandler) info(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}

	emails, err := h.Users.CommitEmails(ctx, u.Key, false)
	if err != nil {
		return nil, err
	}
	unRead, err := h.Users.UnReadNotificationCount(ctx, u.Key)
	if err != nil {
		return nil, err
	}

	return domain.NormalizeUser(u, emails, true, unRead, h.Cfg.SiteConfig().Host, h.Cfg.SiteConfig().SSH), nil
}

// listKey 实现 GET /api/user/listKey。
//
// 逐字对应 application/controllers/api/user.php:40-47。
func (h *UserHandler) listKey(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}

	keys, err := h.Users.SSHKeys(ctx, u.Key)
	if err != nil {
		return nil, err
	}
	out := make([]any, 0, len(keys))
	for i := range keys {
		out = append(out, domain.NormalizeSSHKey(&keys[i]))
	}
	return out, nil
}

// LoginResult 是登录接口的响应体。
//
// 对应 PHP 侧 Base::_ajax_message()：
//
//	{"code":0,"message":"登录成功","extra":{"redirect":"/repositories"}}
//
// 注意这个信封与 /api/* 的信封**不同**：没有 data、没有 request-id，
// 且 extra 在无内容时是空数组 [] 而不是空对象。
type LoginResult struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Extra   any    `json:"extra"`
}

// 登录流程的 message 常量，逐字取自 application/language/zh-cn/user_lang.php。
const (
	msgEmailRequired    = "请输入邮箱"
	msgPasswordRequired = "请输入登录密码"
	msgAccountNotExist  = "该账号尚未注册，请注册账户"
	msgLoginFailed      = "您输入的账号或密码有误，请重新输入"
	msgLoginSuccess     = "登录成功"
)

// LoginHandler 实现 /user/login 的 POST 分支。
type LoginHandler struct {
	Users    *repo.UserRepo
	Sessions *session.Store
	Cfg      *config.Config
}

// NewLoginHandler 创建登录处理器。
func NewLoginHandler(cfg *config.Config, users *repo.UserRepo, sessions *session.Store) *LoginHandler {
	return &LoginHandler{Users: users, Sessions: sessions, Cfg: cfg}
}

// Login 处理登录提交。
//
// 逐字对应 application/controllers/user.php:21-63 的 POST 分支：
//  1. 邮箱为空 → code 1
//  2. 密码为空 → code 2
//  3. 账号不存在 → code 3
//  4. 口令错误 → code 4
//  5. 账号启用了 2FA → code 100，extra.key 为二次验证用的临时键
//  6. 成功 → code 0，extra.redirect = /repositories，并下发会话 Cookie
func (h *LoginHandler) Login(ctx context.Context, w http.ResponseWriter, r *http.Request) (LoginResult, error) {
	email := formValue(r, "email")
	password := formValue(r, "password")

	if email == "" {
		return LoginResult{Code: 1, Message: msgEmailRequired, Extra: []any{}}, nil
	}
	if password == "" {
		return LoginResult{Code: 2, Message: msgPasswordRequired, Extra: []any{}}, nil
	}

	user, err := h.Users.GetByEmail(ctx, email)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return LoginResult{}, err
	}
	if errors.Is(err, repo.ErrNotFound) {
		return LoginResult{Code: 3, Message: msgAccountNotExist, Extra: []any{}}, nil
	}

	ok, needsUpgrade := auth.VerifyPassword(user.Password, password)
	if !ok {
		return LoginResult{Code: 4, Message: msgLoginFailed, Extra: []any{}}, nil
	}

	// 口令哈希升级：旧版 md5(md5(x)) 在登录成功时透明升级为 argon2id。
	//
	// 但只有在**明确配置为 argon2id** 时才做这次写回。
	// 默认（legacy）下绝不改写：PHP 侧只会用 md5(md5($password)) 校验口令，
	// Go 一旦把库里的哈希换成 argon2id，该账号就无法再在 PHP 侧登录，
	// "秒级回滚"承诺随之失效（实测已发生，见 docs/IMPLEMENTATION_STATUS.md）。
	if needsUpgrade && h.Cfg.PasswordHashFormat() == auth.HashFormatArgon2id {
		if hashed, herr := auth.HashPassword(password); herr == nil {
			_ = h.Users.UpdateFields(ctx, user.Key, map[string]any{"u_password": hashed})
		}
	}

	// 2FA：PHP 侧把 u_key 暂存到会话并要求二次验证码（code 100）。
	// 这里用同一个会话 id 作为 extra.key，验证阶段凭它取回用户。
	if user.HasTwoFA() {
		sess, serr := h.Sessions.Create(ctx, user.Key, r.RemoteAddr, r.UserAgent())
		if serr != nil {
			return LoginResult{}, serr
		}
		return LoginResult{Code: 100, Message: msgLoginFailed, Extra: map[string]any{"key": sess.Key}}, nil
	}

	sess, err := h.Sessions.Create(ctx, user.Key, r.RemoteAddr, r.UserAgent())
	if err != nil {
		return LoginResult{}, err
	}
	h.writeSessionCookie(w, r, sess.Key)

	return LoginResult{Code: 0, Message: msgLoginSuccess, Extra: map[string]any{"redirect": "/repositories"}}, nil
}

// Logout 处理登出：删除服务端会话、清除 Cookie 并跳转登录页。
//
// 逐字对应 application/controllers/user.php:314-318：
//
//	$this->session->unset_userdata('userInfo');
//	header('Location:/user/login');
//
// PHP 的 header('Location: ...') 会把状态码置为 302，响应体为空。
//
// 与 PHP 的差异：PHP 的 CI2 会话在登出时会下发**两个** Set-Cookie
// （一个清掉旧的加密 Cookie，一个下发新的会话 id）；
// 新后端是服务端会话，登出只需要清掉那一个 Cookie 即可。
func (h *LoginHandler) Logout(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	if key := session.ReadCookie(r, h.Cfg.Env.Session.Name); key != "" {
		_ = h.Sessions.Delete(ctx, key)
	}
	session.ClearCookie(w, session.CookieOptions{
		Name:   h.Cfg.Env.Session.Name,
		Domain: h.Cfg.Env.Session.Domain,
		Secure: isSecureRequest(r),
		Path:   "/",
	})
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.Header().Set("Location", "/user/login")
	w.Header().Set("Content-Length", "0")
	w.WriteHeader(http.StatusFound)
}

// LoggedIn 判断当前请求是否带着有效的登录会话。
//
// 逐字对应 PHP 侧 Base::getLoggedUserInfo()
// （application/controllers/base.php:36-51）：会话里有 userInfo.u_key，
// 且该用户仍存在 —— 两者缺一即视为未登录。
func (h *LoginHandler) LoggedIn(ctx context.Context, r *http.Request) bool {
	_, ok := sessionUser(ctx, r, h.Sessions, h.Users, h.Cfg.Env.Session.Name)
	return ok
}

// writeSessionCookie 下发会话 Cookie。
func (h *LoginHandler) writeSessionCookie(w http.ResponseWriter, r *http.Request, key string) {
	session.WriteCookie(w, session.CookieOptions{
		Name:   h.Cfg.Env.Session.Name,
		Domain: h.Cfg.Env.Session.Domain,
		Secure: isSecureRequest(r),
		Path:   "/",
		MaxAge: h.Sessions.TTL(),
	}, key)
}

// formValue 读取表单字段，**同时支持** urlencoded 与 multipart。
//
// PHP 侧的 $this->input->post('x') 走 $_POST，而 $_POST 对
// application/x-www-form-urlencoded 与 multipart/form-data 都会填充。
// Go 的 r.ParseForm() 只处理 urlencoded（multipart 会被忽略），
// 因此这里额外处理 multipart，保持与 PHP 相同的兼容面。
//
// 实际前端（jQuery serialize）发的是 urlencoded，但保持 multipart 可用
// 能避免"换一种客户端就登录不了"的坑。
func formValue(r *http.Request, key string) string {
	if r == nil {
		return ""
	}
	_ = r.ParseForm()
	if v := r.PostFormValue(key); v != "" {
		return v
	}
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(8 << 20); err == nil && r.MultipartForm != nil {
			if vals, ok := r.MultipartForm.Value[key]; ok && len(vals) > 0 {
				return vals[0]
			}
		}
	}
	return ""
}

// isSecureRequest 判断当前请求是否经由 HTTPS 抵达。
//
// Cookie 的 Secure 标志**必须由请求协议决定**，不能用配置里的环境名决定：
// 本项目的默认部署是明文 HTTP（容器内 nginx 监听 80），
// 若按 env=production 打上 Secure，浏览器与 HTTP 客户端都不会回传该 Cookie，
// 结果是"登录接口返回成功、但后续每个请求都 401/403"。
//
// 判定依据：
//   - 直连 TLS（r.TLS != nil）；
//   - 前置 TLS 终止代理时透传的 X-Forwarded-Proto: https。
func isSecureRequest(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
}
