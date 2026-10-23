package handlers

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"codefever-next/backend/internal/auth"
	"codefever-next/backend/internal/authkeys"
	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
	"codefever-next/backend/internal/httpapi"
	"codefever-next/backend/internal/repo"
)

// 注册 user 域的"资料 / SSH 公钥 / MFA"端点。
//
// 每个 handler 的实现都标注了对应的 PHP 位置，便于逐条比对契约。
func (h *UserHandler) registerProfileActions(actions map[string]map[string]httpapi.ActionHandler) {
	actions["addSSHKey"] = map[string]httpapi.ActionHandler{"POST": h.addSSHKey}
	actions["removeSSHKey"] = map[string]httpapi.ActionHandler{"POST": h.removeSSHKey}
	actions["updateBasicInfo"] = map[string]httpapi.ActionHandler{"POST": h.updateBasicInfo}
	actions["update"] = map[string]httpapi.ActionHandler{"POST": h.updateProfileField}
	actions["updatePassword"] = map[string]httpapi.ActionHandler{"POST": h.updatePassword}
	actions["getMFAData"] = map[string]httpapi.ActionHandler{"GET": h.getMFAData}
	actions["updateMFAData"] = map[string]httpapi.ActionHandler{"POST": h.updateMFAData}
	actions["revokeMFAData"] = map[string]httpapi.ActionHandler{"POST": h.revokeMFAData}
}

// addSSHKey 实现 POST /api/user/addSSHKey。
//
// 逐字对应 application/controllers/api/user.php:49-73：
//  1. name 与 key 缺一 → 0x0201
//  2. 计算 keyType 与 keyHash
//  3. 指纹重复 → 0x040D
//  4. 写入失败 → 0x0405
//  5. 成功后把公钥同步进 authorized_keys
func (h *UserHandler) addSSHKey(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	name := req.Parsed.BodyString("name")
	key := req.Parsed.BodyString("key")
	if name == "" || key == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	keyType := auth.SSHKeyType(key)
	keyHash := auth.SSHKeyHash(key)

	if _, err := h.Users.SSHKeyByHash(ctx, keyHash); err == nil {
		return nil, contract.NewError(contract.CodeDuplicateSSHKey)
	} else if !errors.Is(err, repo.ErrNotFound) {
		return nil, err
	}

	record := &domain.SSHKey{
		Key:     contract.NewKey(),
		UKey:    u.Key,
		Name:    name,
		KeyType: keyType,
		KeyHash: keyHash,
		KeyData: key,
		Status:  domain.CommonStatusNormal,
		Created: time.Now(),
		Updated: time.Now(),
	}
	if err := h.Users.AddSSHKey(ctx, record); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	// 同步 authorized_keys。
	//
	// **有意不阻断业务**：写文件失败只记日志，接口照常返回成功。
	// PHP 侧是同样的取舍 —— user_sshkey_model.php:112-119 里写 FIFO 的失败
	// 不影响 add() 返回 TRUE，公钥已经落库；此时 SSH 方式暂时不可用，
	// 但用户可以在页面上看到公钥、也可以重新添加 —— 比整个接口 500 更好。
	//
	// Windows 开发机没有 /home/<git 用户>/.ssh 也没有 sshd，直接跳过。
	if authkeys.Supported() {
		if err := h.AuthKeys.Add(u.Key, keyType, key, name); err != nil {
			slog.Error("写入 authorized_keys 失败", "user", u.Key, "error", err)
		}
	}

	return []any{}, nil
}

// removeSSHKey 实现 POST /api/user/removeSSHKey。
//
// 逐字对应 application/controllers/api/user.php:75-96：
//  1. id 缺失 → 0x0201
//  2. 公钥不存在或不属于当前用户 → 0x0106
//  3. 删除失败 → 0x0405
func (h *UserHandler) removeSSHKey(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	id := req.Parsed.BodyString("id")
	if id == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	record, err := h.Users.SSHKeyByID(ctx, id)
	if err != nil && !errors.Is(err, repo.ErrNotFound) {
		return nil, err
	}
	if errors.Is(err, repo.ErrNotFound) || record.UKey != u.Key {
		return nil, contract.NewError(contract.CodeActionNotAllowed)
	}

	if err := h.Users.DeleteSSHKey(ctx, id); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	// 按**公钥正文子串**从 authorized_keys 里删掉对应的那一行。
	//
	// 删除语义与 PHP 守护进程一致（misc/modify_authorized_keys.php:125-140）：
	// 拆行过滤子串命中者再拼回，而不是整行精确匹配 —— 历史文件里同一把钥匙
	// 可能被追加过多次。
	//
	// 与 addSSHKey 一样：**有意不阻断业务**，写文件失败只记日志。
	// PHP 侧同样忽略这一步的失败（user_sshkey_model.php:57）。
	if authkeys.Supported() {
		if err := h.AuthKeys.Remove(record.KeyData); err != nil {
			slog.Error("从 authorized_keys 删除公钥失败", "user", u.Key, "error", err)
		}
	}

	return []any{}, nil
}

// updateBasicInfo 实现 POST /api/user/updateBasicInfo。
//
// 逐字对应 application/controllers/api/user.php:108-131。
func (h *UserHandler) updateBasicInfo(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	name := req.Parsed.BodyString("name")
	email := req.Parsed.BodyString("email")
	if name == "" || email == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	fields := map[string]any{
		"u_name":    name,
		"u_email":   email,
		"u_updated": time.Now(),
	}
	if team, ok := req.Parsed.Body["team"]; ok {
		fields["u_team"] = nullableString(team)
	}
	if role, ok := req.Parsed.Body["role"]; ok {
		fields["u_role"] = nullableString(role)
	}

	if err := h.Users.UpdateFields(ctx, u.Key, fields); err != nil {
		return nil, err
	}
	return []any{}, nil
}

// updateProfileField 实现 POST /api/user/update（单字段更新）。
//
// 逐字对应 application/controllers/api/user.php:133-180：
//  1. field / data 缺一 → 0x0201
//  2. field=name 且长度 > 15 → 0x0201
//  3. 值未发生变化 → 0x0405
//
// 与原实现的一处**有意差异**：PHP 的白名单包含 job 与 company，
// 但 cc_users 表里根本没有 u_job / u_company 两列 —— 写入必然失败并抛 SQL 错误。
// 新后端只接受实际存在的列（name / tel / location），其余字段返回 0x0201。
func (h *UserHandler) updateProfileField(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	field := req.Parsed.BodyString("field")
	value := req.Parsed.BodyString("data")
	if field == "" || value == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	var column string
	switch field {
	case "name":
		if utf8.RuneCountInString(value) > 15 {
			return nil, contract.NewError(contract.CodeMissingParameter)
		}
		column = "u_name"
	case "tel":
		column = "u_tel"
	case "location":
		column = "u_location"
	default:
		// 含 PHP 白名单里存在但表中没有的 job / company。
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	// 未发生变化 → 0x0405（PHP 用 == 比较，NULL 与空串在 PHP 8 下不等价）。
	current := h.currentFieldValue(u, field)
	if current != nil && *current == value {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	if err := h.Users.UpdateFields(ctx, u.Key, map[string]any{
		column:      value,
		"u_updated": time.Now(),
	}); err != nil {
		return nil, err
	}
	return []any{}, nil
}

// currentFieldValue 读取用户当前的字段值（可能为 NULL）。
func (h *UserHandler) currentFieldValue(u *domain.User, field string) *string {
	switch field {
	case "name":
		v := u.Name
		return &v
	case "tel":
		return u.Tel
	case "location":
		return u.Location
	default:
		return nil
	}
}

// updatePassword 实现 POST /api/user/updatePassword。
//
// 逐字对应 application/controllers/api/user.php 的 updatePassword_post：
//  1. current / new 缺一 → 0x0201
//  2. 旧口令错误 → 0x0410
//
// 与原实现的一处**有意改进**：PHP 侧写入 md5(md5(x))，
// 新后端默认也写入 md5(md5(x))（由 auth.hash_format /
// CODEFEVER_PASSWORD_HASH 决定，默认 legacy）。
// 保持 legacy 是共存期的硬要求：PHP 侧只会用 md5(md5($password))
// 校验口令，Go 若写成 argon2id，改过口令的账号就无法再在 PHP 侧登录。
// 登录时同样支持校验旧哈希，是否自动升级取决于同一个配置。
func (h *UserHandler) updatePassword(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	current := req.Parsed.BodyString("current")
	next := req.Parsed.BodyString("new")
	if current == "" || next == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	ok, _ := auth.VerifyPassword(u.Password, current)
	if !ok {
		return nil, contract.NewError(contract.CodePasswordNotCorrect)
	}

	hashed, err := auth.HashPasswordForFormat(next, h.Cfg.PasswordHashFormat())
	if err != nil {
		return nil, err
	}
	if err := h.Users.UpdateFields(ctx, u.Key, map[string]any{
		"u_password": hashed,
		"u_updated":  time.Now(),
	}); err != nil {
		return nil, err
	}
	return []any{}, nil
}

// getMFAData 实现 GET /api/user/getMFAData。
//
// 逐字对应 application/controllers/api/user.php:388-411：
// 生成 TOTP secret 与 otpauth URL，渲染 QR 码为 SVG data URI。
//
// 与原实现的一处**非语义差异**：PHP 用 Endroid QrCode（尺寸 300、边距 10、
// 纠错等级 L），Go 用 skip2/go-qrcode（尺寸 300、纠错等级 L），
// 静默区（quiet zone）宽度不同，因此二维码图像的像素不完全一致，
// 但编码内容（otpauth URL）完全相同，认证器 App 扫码结果一致。
func (h *UserHandler) getMFAData(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}

	secret, err := auth.NewTOTPSecret()
	if err != nil {
		return nil, err
	}
	authURL := "otpauth://totp/Codefever-Community:" + u.Email + "?secret=" + secret

	svg, err := renderQRCode(authURL)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"image":  "data:image/svg+xml;charset=utf-8;base64," + base64Std(svg),
		"secret": secret,
	}, nil
}

// updateMFAData 实现 POST /api/user/updateMFAData。
//
// 逐字对应 application/controllers/api/user.php:413-435：
// secret / code1 / code2 缺一 → 0x0201；
// 第一个验证码错 → 0x0411；第二个验证码错 → 0x0412。
//
// 校验窗口为 ±5 个 30 秒时间片，与 GoogleAuthenticator::verifyCode($s,$c,5) 一致。
//
// 与原实现的一处**安全改进**：secret 以 AES-GCM 加密后入库（u_2fa），
// 而非明文。读取时兼容历史明文值。
func (h *UserHandler) updateMFAData(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	secret := req.Parsed.BodyString("secret")
	code1 := req.Parsed.BodyString("code1")
	code2 := req.Parsed.BodyString("code2")
	if secret == "" || code1 == "" || code2 == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	totp := auth.TOTPFromSecret(secret, 5)
	now := time.Now()
	if !totp.Verify(code1, now) {
		return nil, contract.NewError(contract.CodeMFAInvalid)
	}
	if !totp.Verify(code2, now) {
		return nil, contract.NewError(contract.CodeMFAInvalidSecond)
	}

	encrypted, err := auth.EncryptSecret(secret, h.Cfg.Env.TOTP.Salt)
	if err != nil {
		return nil, err
	}
	if err := h.Users.UpdateFields(ctx, u.Key, map[string]any{
		"u_2fa":     encrypted,
		"u_updated": time.Now(),
	}); err != nil {
		return nil, err
	}
	return []any{}, nil
}

// revokeMFAData 实现 POST /api/user/revokeMFAData。
//
// 逐字对应 application/controllers/api/user.php:437-442：
// 把 u_2fa 置为空。
//
// 注意：PHP 侧写入的是空字符串，而 admin 的"已禁用 MFA"筛选条件是
// `u_2fa IS NULL`（user_model.php:315）—— 两者不一致导致该筛选恒为空。
// 新后端写入 NULL，与筛选条件保持一致。
func (h *UserHandler) revokeMFAData(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	if err := h.Users.UpdateFields(ctx, u.Key, map[string]any{
		"u_2fa":     nil,
		"u_updated": time.Now(),
	}); err != nil {
		return nil, err
	}
	return []any{}, nil
}

// nullableString 把表单值转为可空字符串。
//
// PHP 的 $_POST 里缺失的字段是 NULL，显式空串是 ""，
// 两者写库后的效果不同，因此这里保留区分。
func nullableString(v any) any {
	if v == nil {
		return nil
	}
	if s, ok := v.(string); ok {
		if s == "" {
			return nil
		}
		return s
	}
	return v
}

// trimSpace 去除首尾空白。
func trimSpace(s string) string { return strings.TrimSpace(s) }
