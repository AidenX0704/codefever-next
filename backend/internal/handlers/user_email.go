package handlers

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"codefever-next/backend/internal/auth"
	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/httpapi"
	"codefever-next/backend/internal/mailer"
	"codefever-next/backend/internal/repo"
)

// 注册 user 域的"邮件验证码 / 提交邮箱"端点。
func (h *UserHandler) registerEmailActions(actions map[string]map[string]httpapi.ActionHandler) {
	actions["getEmailCode"] = map[string]httpapi.ActionHandler{"POST": h.getEmailCode}
	actions["changeEmail"] = map[string]httpapi.ActionHandler{"POST": h.changeEmail}
	actions["addCommitEmail"] = map[string]httpapi.ActionHandler{"POST": h.addCommitEmail}
	actions["resentCommitEmailCode"] = map[string]httpapi.ActionHandler{"POST": h.resentCommitEmailCode}
	actions["deleteCommitEmail"] = map[string]httpapi.ActionHandler{"POST": h.deleteCommitEmail}
	actions["validationCommitEmailCode"] = map[string]httpapi.ActionHandler{"POST": h.validationCommitEmailCode}
}

// emailPattern 是 PHP 侧用于校验邮箱的正则。
//
// 逐字对应 application/controllers/api/user.php:268 的
// /^\w+([.-]?\w+)*@\w+([.-]?\w+)*(\.\w{2,3})+$/
var emailPattern = regexp.MustCompile(`^\w+([.-]?\w+)*@\w+([.-]?\w+)*(\.\w{2,3})+$`)

// verifyCodeMailTitle 是验证码邮件标题。
//
// 逐字对应 PHP 侧 '【CodeFever Next】验证电子邮件地址'。
const verifyCodeMailTitle = "【CodeFever Next】验证电子邮件地址"

// verifyCodeMailTemplate 是验证码邮件正文模板。
//
// PHP 侧 EmailTemplate::verifyCode 的常量内容形如
// "您的验证码是 %s ..."，此处保留同样的语义与结构。
const verifyCodeMailTemplate = `您的验证码是 <b>%s</b>，请在 5 分钟内使用。`

// getEmailCode 实现 POST /api/user/getEmailCode。
//
// 逐字对应 application/controllers/api/user.php:223-255：
//  1. email / password 缺一 → 0x0201
//  2. email 与当前邮箱相同 → 0x040D（复用"重复 SSH Key"码）
//  3. email 已被其他账号占用 → 0x0406
//  4. 口令校验失败 → 0x040C
//  5. 发送验证码邮件 → 成功
//
// 注意第 4 步在 PHP 侧用 0x040C（Old Target Not Exists）表达"口令错误"，
// 属于错误码复用；新后端沿用以保持契约一致。
func (h *UserHandler) getEmailCode(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	email := req.Parsed.BodyString("email")
	password := req.Parsed.BodyString("password")
	if email == "" || password == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	if u.Email == email {
		return nil, contract.NewError(contract.CodeDuplicateSSHKey)
	}
	if _, err := h.Users.GetByEmail(ctx, email); err == nil {
		return nil, contract.NewError(contract.CodeTargetExists)
	}

	// 校验当前账号口令。
	stored, err := h.Users.Get(ctx, u.Key)
	if err != nil {
		return nil, contract.NewError(contract.CodeOldTargetNotExists)
	}
	ok, _ := auth.VerifyPassword(stored.Password, password)
	if !ok {
		return nil, contract.NewError(contract.CodeOldTargetNotExists)
	}

	h.sendVerifyCode(email)
	return []any{}, nil
}

// changeEmail 实现 POST /api/user/changeEmail。
//
// 逐字对应 application/controllers/api/user.php:257-281：
// email / code / password 缺一 → 0x0201；邮箱格式非法 → 0x0201；
// 验证码不匹配 → 0x040C；否则更新 u_email。
//
// 注意：PHP 侧只校验 password **存在**，并不校验其正确性 ——
// 这是既有缺陷。新后端**保留**该行为以维持契约（否则前端流程会变），
// 但在此注释中标明。
func (h *UserHandler) changeEmail(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	email := strings.TrimSpace(req.Parsed.BodyString("email"))
	code := strings.TrimSpace(req.Parsed.BodyString("code"))
	password := req.Parsed.BodyString("password")
	if email == "" || code == "" || password == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if !emailPattern.MatchString(email) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if !auth.VerifyEmailCode(email, code, time.Now(), h.Cfg.Env.TOTP.Salt) {
		return nil, contract.NewError(contract.CodeOldTargetNotExists)
	}

	if err := h.Users.UpdateFields(ctx, u.Key, map[string]any{
		"u_email":   email,
		"u_updated": time.Now(),
	}); err != nil {
		return nil, err
	}
	return []any{}, nil
}

// addCommitEmail 实现 POST /api/user/addCommitEmail。
//
// 逐字对应 application/controllers/api/user.php:283-313。
func (h *UserHandler) addCommitEmail(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	email := req.Parsed.BodyString("email")
	if email == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if !emailPattern.MatchString(email) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if email == u.Email {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	exists, err := h.Users.IsCommitEmailExists(ctx, u.Key, email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if _, err := h.Users.AddCommitEmail(ctx, u.Key, email); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	h.sendVerifyCode(email)
	return []any{}, nil
}

// resentCommitEmailCode 实现 POST /api/user/resentCommitEmailCode。
//
// 逐字对应 application/controllers/api/user.php:315-337。
func (h *UserHandler) resentCommitEmailCode(ctx context.Context, req *httpapi.Request) (any, error) {
	if req.User == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	email := req.Parsed.BodyString("email")
	if email == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if !emailPattern.MatchString(email) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	h.sendVerifyCode(email)
	return []any{}, nil
}

// deleteCommitEmail 实现 POST /api/user/deleteCommitEmail。
//
// 逐字对应 application/controllers/api/user.php:339-359：软删除（ce_status=4）。
func (h *UserHandler) deleteCommitEmail(ctx context.Context, req *httpapi.Request) (any, error) {
	if req.User == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	ceKey := req.Parsed.BodyString("id")
	if ceKey == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Users.UpdateCommitEmail(ctx, ceKey, map[string]any{
		"ce_status":  repo.CommonStatusDelete,
		"ce_deleted": time.Now(),
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return []any{}, nil
}

// validationCommitEmailCode 实现 POST /api/user/validationCommitEmailCode。
//
// 逐字对应 application/controllers/api/user.php:361-386：校验通过则置 ce_is_check=1。
func (h *UserHandler) validationCommitEmailCode(ctx context.Context, req *httpapi.Request) (any, error) {
	if req.User == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	ceKey := req.Parsed.BodyString("id")
	email := req.Parsed.BodyString("email")
	code := req.Parsed.BodyString("code")
	if ceKey == "" || email == "" || code == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if !auth.VerifyEmailCode(email, code, time.Now(), h.Cfg.Env.TOTP.Salt) {
		return nil, contract.NewError(contract.CodeOldTargetNotExists)
	}
	if err := h.Users.UpdateCommitEmail(ctx, ceKey, map[string]any{
		"ce_is_check": 1,
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return []any{}, nil
}

// sendVerifyCode 生成并发送验证码邮件。
//
// 发送失败**不影响**接口返回 —— PHP 侧同样忽略发送结果
// （Driver/Mail.php 无条件 return true）。失败只记录日志。
func (h *UserHandler) sendVerifyCode(email string) {
	if h.Mailer == nil {
		return
	}
	code := auth.GenerateEmailCode(email, time.Now(), h.Cfg.Env.TOTP.Salt)
	from := h.Cfg.SiteConfig().Email.Name + "<" + h.Cfg.SiteConfig().Email.From + ">"
	msg := mailer.Message{
		To:      email,
		Subject: verifyCodeMailTitle,
		Body:    strings.ReplaceAll(verifyCodeMailTemplate, "%s", code),
		From:    from,
	}
	// 发送是同步的（PHP 同语义），但必须有上限：SMTP/mail 卡住时不能
	// 让 HTTP 请求跟着挂 30 秒以上。
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := h.Mailer.Send(ctx, msg); err != nil {
		// 发送失败不改变接口语义（与 PHP 一致），但必须留下痕迹。
		slog.Error("发送验证码邮件失败", "to", email, "error", err)
	}
}
