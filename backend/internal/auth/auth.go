// Package auth 实现口令哈希、双因素验证码与网关令牌校验。
package auth

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

// 口令哈希参数（OWASP 2024 推荐基线）。
const (
	argon2Memory      = 64 * 1024 // 64 MiB
	argon2Iterations  = 3
	argon2Parallelism = 2
	argon2KeyLength   = 32
	argon2SaltLength  = 16
)

// ErrInvalidHash 表示存储的哈希格式无法识别。
var ErrInvalidHash = errors.New("口令哈希格式非法")

// HashFormat 决定**新写入**的口令哈希格式。
//
// 共存期的关键约束：PHP 后端只会用 md5(md5($password)) 校验口令
// （application/controllers/user.php:79、api/user.php:214、api/admin.php:106、
// apiv1/internal.php:91）。因此只要 Go 侧写入了别的格式，PHP 侧就再也
// 无法验证该账号 —— 账号一旦被 Go 改写过一次，"秒级回滚"就对该账号失效。
//
// 所以默认必须是 legacy：双后端并存期间库里的哈希始终是 PHP 可读的，
// 回滚随时可用。等 PHP 下线之后，再把配置切成 argon2id 即可。
type HashFormat string

const (
	// HashFormatLegacy 是 PHP 侧同款的 md5(md5(x))，PHP 可验证。
	HashFormatLegacy HashFormat = "legacy"
	// HashFormatArgon2id 是 argon2id，PHP **不可**验证。
	HashFormatArgon2id HashFormat = "argon2id"
)

// ParseHashFormat 解析配置中的哈希格式。
//
// 空值或无法识别的取值一律回落到 legacy —— 这是更安全的一侧：
// 写错配置最多让口令哈希保持与 PHP 一致，而不会让 PHP 回滚目标失效。
func ParseHashFormat(s string) HashFormat {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case string(HashFormatArgon2id):
		return HashFormatArgon2id
	default:
		return HashFormatLegacy
	}
}

// HashPasswordForFormat 按指定格式生成口令哈希。
//
// legacy 格式直接返回 md5(md5(x))，与 PHP 侧逐字一致；
// argon2id 格式走带随机盐的内存硬函数。
func HashPasswordForFormat(password string, format HashFormat) (string, error) {
	if format == HashFormatArgon2id {
		return HashPassword(password)
	}
	return LegacyHash(password), nil
}

// HashPassword 用 argon2id 生成口令哈希。
//
// 格式：$argon2id$v=19$m=65536,t=3,p=2$<base64 salt>$<base64 key>
func HashPassword(password string) (string, error) {
	salt := make([]byte, argon2SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("生成盐值失败: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argon2Iterations, argon2Memory, argon2Parallelism, argon2KeyLength)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argon2Memory, argon2Iterations, argon2Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

// VerifyPassword 校验口令。
//
// 返回 (ok, needsUpgrade)：
//   - ok 为 true 表示口令正确；
//   - needsUpgrade 为 true 表示存储的是旧版 md5(md5(x)) 哈希，调用方**可以**
//     在校验成功后用 HashPassword 重新哈希并写回。
//
// 两种格式都必须能校验：现有库中所有用户的口令都是 md5(md5(x))，
// 直接切换算法会导致全量用户无法登录；反过来，argon2id 哈希也只能由
// 本实现校验（PHP 侧不认）。
//
// 注意：needsUpgrade 只是一个**提示**，写不写回由调用方根据部署形态决定。
// 双后端共存期间必须保持 legacy，否则 PHP 回滚目标会失去该账号 ——
// 详见 HashFormat 的说明。
func VerifyPassword(stored, password string) (ok bool, needsUpgrade bool) {
	if stored == "" {
		return false, false
	}
	if strings.HasPrefix(stored, "$argon2id$") {
		return verifyArgon2(stored, password), false
	}
	// 旧版双层 md5。
	return LegacyHash(password) == stored, true
}

// verifyArgon2 校验 argon2id 哈希。
func verifyArgon2(stored, password string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false
	}
	var memory, iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// LegacyHash 计算旧版双层 md5 哈希。
//
// 逐字对应 PHP 侧 md5(md5($password))。
//
// 关键细节：PHP 的 md5() 返回的是 **32 字符的十六进制字符串**，
// 因此第二次 md5 是对**该字符串**求哈希，而不是对 16 字节原始摘要。
// 对原始摘要再求一次 md5 会得到完全不同的值 —— 这是实现时最容易踩的坑。
//
// 实测：LegacyHash("123456") 必须等于 14e1b600b1fd579f47433b88e8d85291
// （与 misc/db_data.sql 中种子管理员的口令哈希一致）。
func LegacyHash(password string) string {
	first := md5.Sum([]byte(password))
	firstHex := hex.EncodeToString(first[:])
	second := md5.Sum([]byte(firstHex))
	return hex.EncodeToString(second[:])
}

// IsLegacyHash 判断某个存储值是否为旧版 md5 哈希。
func IsLegacyHash(stored string) bool {
	if len(stored) != 32 {
		return false
	}
	_, err := hex.DecodeString(stored)
	return err == nil
}

// CompareToken 恒定时间比较两个令牌。
//
// PHP 侧 APIAuth.php:18 用 `===` 直接比较字符串，存在计时侧信道；
// 网关令牌是内部服务的唯一凭据，必须用常量时间比较。
func CompareToken(got, want string) bool {
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// TOTP 是基于 RFC 6238 的一次性口令。
type TOTP struct {
	Secret string // base32 编码，不含填充
	Period time.Duration
	Digits int
	Window int
}

// NewTOTP 创建默认配置的 TOTP（30 秒周期、6 位数字、允许 ±1 个时间片）。
func NewTOTP(secret string) *TOTP {
	return &TOTP{Secret: secret, Period: 30 * time.Second, Digits: 6, Window: 1}
}

// Code 计算指定时刻的验证码。
func (t *TOTP) Code(at time.Time) (string, error) {
	counter := uint64(at.Unix()) / uint64(t.Period.Seconds())
	return t.codeForCounter(counter)
}

// codeForCounter 按 RFC 6238 的 HOTP 流程生成验证码。
func (t *TOTP) codeForCounter(counter uint64) (string, error) {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(t.Secret))
	if err != nil {
		return "", fmt.Errorf("解码 TOTP 密钥失败: %w", err)
	}
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)

	mac := hmac.New(sha1.New, key)
	mac.Write(buf[:])
	sum := mac.Sum(nil)

	// RFC 4226 动态截断。
	offset := sum[len(sum)-1] & 0x0f
	value := (uint32(sum[offset])&0x7f)<<24 |
		uint32(sum[offset+1])<<16 |
		uint32(sum[offset+2])<<8 |
		uint32(sum[offset+3])

	mod := uint32(1)
	for i := 0; i < t.Digits; i++ {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", t.Digits, value%mod), nil
}

// Verify 校验验证码，允许 ±Window 个时间片。
func (t *TOTP) Verify(code string, at time.Time) bool {
	code = strings.TrimSpace(code)
	if code == "" {
		return false
	}
	for delta := -t.Window; delta <= t.Window; delta++ {
		counter := uint64(at.Add(time.Duration(delta)*t.Period).Unix()) / uint64(t.Period.Seconds())
		want, err := t.codeForCounter(counter)
		if err != nil {
			return false
		}
		if subtle.ConstantTimeCompare([]byte(want), []byte(code)) == 1 {
			return true
		}
	}
	return false
}

// GenerateEmailCode 生成邮件验证码。
//
// 现有的 Utility/TOTP.php 是项目自定义的非标算法（sha256(md5(i).md5(salt)) 加
// 非标准截断，校验窗口 ±300 秒）。该算法是纯服务端闭环 —— 前端只负责展示与回传，
// 不参与计算 —— 因此可以替换为标准实现。
//
// 代价：替换瞬间已发出的在途验证码（10 分钟内）会失效，属可接受的降级。
// 新的实现使用 6 位数字、5 分钟有效期，显著降低爆破空间。
//
// input 是与验证码绑定的输入（PHP 侧用邮箱），因此同一个验证码只能用于该邮箱。
func GenerateEmailCode(input string, at time.Time, salt string) string {
	counter := uint64(at.Unix()) / 300
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)

	sum := sha1.Sum(append([]byte(salt+":"+input+":"), buf...))
	value := (uint32(sum[0])&0x7f)<<24 | uint32(sum[1])<<16 | uint32(sum[2])<<8 | uint32(sum[3])
	return fmt.Sprintf("%06d", value%1000000)
}

// VerifyEmailCode 校验邮件验证码。
func VerifyEmailCode(input, code string, at time.Time, salt string) bool {
	code = strings.TrimSpace(code)
	if len(code) != 6 {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(GenerateEmailCode(input, at, salt)), []byte(code)) == 1
}

// FormatCodeError 把验证码错误细分为前端可区分的两类。
//
// PHP 侧用 0x0411（MFA Code Invalid）与 0x0412（MFA Code Invalid (2)）区分
// "第一段验证码错" 与 "第二段验证码错"，前端按数值区分提示文案。
func FormatCodeError(second bool) string {
	if second {
		return "MFA Code Invalid (2)"
	}
	return "MFA Code Invalid"
}

// ParseIntDefault 解析整数，失败时返回默认值。
func ParseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return def
	}
	return n
}

// sshKeyTypePattern 逐字对应 PHP 侧 Helper::getSSHKeyType 的正则。
var sshKeyTypePattern = regexp.MustCompile(`((?:ssh|ecdsa)-[\w\d]+)`)

// SSHKeyType 识别 SSH 公钥类型。
//
// 逐字对应 application/libraries/service/Utility/Helper.php:31-42：
// base64 解码后在二进制内容中匹配 `(?:ssh|ecdsa)-[\w\d]+`，
// 匹配不到时返回 'ssh-rsa'。
func SSHKeyType(publicKey string) string {
	data, err := base64DecodeLenient(publicKey)
	if err != nil {
		return "ssh-rsa"
	}
	if m := sshKeyTypePattern.FindSubmatch(data); m != nil && len(m) > 1 {
		return string(m[1])
	}
	return "ssh-rsa"
}

// SSHKeyHash 计算 SSH 公钥指纹。
//
// 逐字对应 Helper::getSSHKeyHash：hash('md5', base64_decode($sshKey))。
//
// 注意指纹是对**解码后的二进制**求 md5，而不是对 base64 字符串求 md5。
func SSHKeyHash(publicKey string) string {
	data, err := base64DecodeLenient(publicKey)
	if err != nil {
		data = []byte(publicKey)
	}
	sum := md5.Sum(data)
	return hex.EncodeToString(sum[:])
}

// base64DecodeLenient 以 PHP 的宽松方式解码 base64。
//
// PHP 的 base64_decode($str) 默认忽略非法字符；Go 的标准解码是严格的。
// 为了与既有数据（前端传入的公钥可能带换行）保持兼容，这里先剔除
// base64 字母表之外的字符再解码，并补齐 padding。
func base64DecodeLenient(s string) ([]byte, error) {
	cleaned := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, s)
	cleaned = strings.Map(func(r rune) rune {
		if strings.ContainsRune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=", r) {
			return r
		}
		return -1
	}, cleaned)

	if pad := len(cleaned) % 4; pad != 0 {
		cleaned += strings.Repeat("=", 4-pad)
	}
	return base64.StdEncoding.DecodeString(cleaned)
}
