package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
)

// 加密存储 TOTP secret 的版本前缀。
//
// 现状：PHP 侧把 u_2fa 明文存库（user_model.php:88-100），一旦数据库泄露，
// 攻击者可直接生成验证码。新后端改为 AES-GCM 加密存储，同时**保留对明文
// 历史值的读取能力**，因此可以平滑升级，无需强制用户重新绑定。
const secretPrefix = "enc:v1:"

// ErrSecretFormat 表示密文格式无法识别。
var ErrSecretFormat = errors.New("密文格式非法")

// DeriveKey 从盐值派生对称密钥。
func DeriveKey(salt string) []byte {
	sum := sha256.Sum256([]byte(salt))
	return sum[:]
}

// EncryptSecret 用 AES-GCM 加密敏感值。
//
// 输出格式：enc:v1:<base64(nonce||ciphertext)>
func EncryptSecret(plaintext, salt string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	block, err := aes.NewCipher(DeriveKey(salt))
	if err != nil {
		return "", fmt.Errorf("创建加密器失败: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建 GCM 失败: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("生成 nonce 失败: %w", err)
	}
	sealed := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	return secretPrefix + base64.RawStdEncoding.EncodeToString(append(nonce, sealed...)), nil
}

// DecryptSecret 解密敏感值。
//
// 兼容两种输入：
//   - 以 enc:v1: 开头 → AES-GCM 解密；
//   - 其他 → 视为历史明文，原样返回。
func DecryptSecret(stored, salt string) (string, error) {
	if stored == "" {
		return "", nil
	}
	if !strings.HasPrefix(stored, secretPrefix) {
		// 历史明文值。
		return stored, nil
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(stored, secretPrefix))
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrSecretFormat, err)
	}
	block, err := aes.NewCipher(DeriveKey(salt))
	if err != nil {
		return "", fmt.Errorf("创建加密器失败: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("创建 GCM 失败: %w", err)
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("%w: 密文长度不足", ErrSecretFormat)
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("%w: 解密失败", ErrSecretFormat)
	}
	return string(plain), nil
}

// NewTOTPSecret 生成新的 TOTP 密钥。
//
// 逐字对应 GoogleAuthenticator::createSecret()：16 字节随机数 → base32（无填充），
// 即 32 个字符。
func NewTOTPSecret() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("生成 TOTP 密钥失败: %w", err)
	}
	return strings.TrimRight(base32.StdEncoding.EncodeToString(b), "="), nil
}

// TOTPFromSecret 按 RFC 6238 的默认参数构造 TOTP 校验器。
//
// 对应 GoogleAuthenticator::verifyCode($secret, $code, 5)：
// 时间窗口为 ±5 个 30 秒时间片（±150 秒）。
func TOTPFromSecret(secret string, window int) *TOTP {
	return &TOTP{Secret: secret, Period: 30 * time.Second, Digits: 6, Window: window}
}
