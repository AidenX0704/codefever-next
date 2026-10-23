package contract

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// NewRequestID 生成响应的 request-id。
//
// 格式与 PHP 侧 UUID::getUUID() 完全一致：RIPEMD-128 哈希取 32 位十六进制，
// 按 8-4-4-4-12 排列并大写（实测形如 AA0F618B-0560-5B08-0E3B-99946AD3C1C9）。
//
// 与原实现的唯一差异是随机源：PHP 用 md5(microtime . rand()×5)，
// Go 侧改用 crypto/rand。这是有意的改进——PHP 的 rand() 可预测且 md5 拼接
// 并非密码学强度。request-id 只是一个不透明的相关性标识符，前端从不解析其内容
// （www/view/src/helpers/NetworkHelper.js 只读取 code/message/data），
// 因此格式一致即可满足契约，而唯一性更强。
func NewRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand 读取失败属于系统级异常；退化为时间戳派生值以避免 panic。
		b = []byte(fmt.Sprintf("%016x", time.Now().UnixNano()))
	}
	h := strings.ToUpper(hex.EncodeToString(b))
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

// NewKey 生成 32 位小写十六进制的业务主键。
//
// 与 PHP 侧 UUID::getKey()（md5(time() . rand() . rand() . rand() . uniqid())）
// 输出一致：32 位小写 hex，无分隔符。该值用作 u_key / g_key / r_key 等主键，
// 也是 git-storage 下裸仓库目录名的组成部分，因此格式必须保持不变。
func NewKey() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%032x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
