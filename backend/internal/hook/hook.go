// Package hook 实现 git 钩子可执行体（misc/hooks/main 的 Go 重写版）。
//
// 它必须逐字复刻 PHP 版 misc/hooks/main（58 行）的行为契约 ——
// 后端已经换成 Go，钩子是"彻底移除 PHP 运行时"的第一块拼图：
// PHP 一删，misc/hooks/main 就无法执行，**所有 git push 都会被拒**。
//
// PHP 侧行为与本文件的逐条对应关系：
//
//	PHP misc/hooks/main:3    yaml_parse_file(dirname(__FILE__, 3) . '/env.yaml')
//	    → ResolveBaseDir + LoadConfig（本实现改为从可执行文件所在目录逐级向上找 env.yaml）
//	PHP misc/hooks/main:5    gateway.token / gateway.hooks 缺一即失败
//	    → LoadConfig
//	PHP misc/hooks/main:11   Bad configuration 文案 + exit(255)
//	    → BadConfigMessage
//	PHP misc/hooks/main:18-22  请求头 Token / Content-Type / Accept
//	    → postHook
//	PHP misc/hooks/main:28   'uid' => getenv('PGYER_UID')（未设置时为 false）
//	    → UIDFromEnv
//	PHP misc/hooks/main:29   'args' => $argv（含 argv[0] 自身路径）
//	    → Payload.Args
//	PHP misc/hooks/main:26-30  非阻塞读 stdin（本实现改为读到 EOF，见 Execute 注释）
//	    → Execute
//	PHP misc/hooks/main:33   json_encode($data)
//	    → EncodePayload
//	PHP misc/hooks/main:39   json_decode 失败 → 视为失败
//	    → handleResponse
//	PHP misc/hooks/main:42-46 code == 0 → 输出 data.remote，exit(0)
//	PHP misc/hooks/main:47-53 code != 0 → 输出 Operation Stopped / Request-ID，exit(255)
//	PHP misc/hooks/main:54-56 解析失败 → 输出 Contact site administrator...，exit(255)
package hook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"codefever-next/backend/internal/config"
)

// 退出码语义：0 = 放行 push；255 = 拒绝 push。
const (
	// ExitAllow 表示放行 push（PHP 的 exit(0)）。
	ExitAllow = 0
	// ExitReject 表示拒绝 push（PHP 的 exit(255)）。
	ExitReject = 255
)

// 三条分支的输出文案，**逐字**取自 PHP 侧。
const (
	// BadConfigMessage 对应 misc/hooks/main:11（注意末尾是 "!."，感叹号加句点）。
	BadConfigMessage = "[CodeFever Next]: Bad configuration, Contact site administrator if operation rejected!."

	// ContactAdminMessage 对应 misc/hooks/main:55。
	ContactAdminMessage = "Contact site administrator if operation rejected!"

	// operationStoppedPrefix 对应 misc/hooks/main:48。
	operationStoppedPrefix = "Operation Stopped: "

	// requestIDPrefix 对应 misc/hooks/main:52。
	requestIDPrefix = "Request-ID: "
)

// defaultBaseDir 是容器内的应用根目录，与 codefever-api 保持一致。
//
// 用途：向上查找 env.yaml 失败时的兜底位置。
const defaultBaseDir = "/data/www/codefever-community"

// requestTimeout 是钩子请求后端的超时。
//
// PHP 侧的 curl 没有设置超时（可能永久挂起）；这里给 30 秒上限，
// 避免一次 push 被后端卡死时钩子永远不返回。
const requestTimeout = 30 * time.Second

// ErrBadConfig 表示 env.yaml 缺失或 gateway 段不完整。
//
// PHP 侧对应 misc/hooks/main:5-13 的 else 分支。
var ErrBadConfig = errors.New("配置缺失或 gateway 段不完整")

// Config 是钩子需要的最小配置（对应 env.yaml 的 gateway 段）。
type Config struct {
	// Endpoint 是后端回调地址（gateway.hooks）。
	Endpoint string
	// Token 是内部网关令牌（gateway.token）。
	Token string
}

// Payload 是 POST 给 /apiv1/internal/hooks 的请求体。
//
// 逐字对应 misc/hooks/main:27-31 的 $data：
//
//	['uid' => getenv('PGYER_UID'), 'args' => $argv, 'stdin' => stream_get_contents($input)]
//
// 注意 PHP 的 json_encode 保持**插入顺序**，因此这里的字段顺序也是 uid/args/stdin。
type Payload struct {
	// UID 是 PGYER_UID 的值；未设置时为 false（PHP 的 getenv() 返回 false）。
	UID any
	// Args 是全部 argv（含 argv[0] 自身路径）。
	Args []string
	// Stdin 是 stdin 原文。
	Stdin string
}

// UIDFromEnv 读取 PGYER_UID。
//
// PHP: 'uid' => getenv('PGYER_UID')
//
// getenv() 在变量**未设置**时返回 false，json_encode(false) 编码为 false；
// 变量已设置但为空串时返回 ""，编码为 ""。这里保持同样语义，
// 不能改成 null 或空字符串 —— 后端按真值判定 uid 是否存在。
func UIDFromEnv() any {
	if v, ok := os.LookupEnv("PGYER_UID"); ok {
		return v
	}
	return false
}

// LoadConfig 读取 baseDir 下的 env.yaml，取出 gateway 段。
//
// PHP: $config = yaml_parse_file(dirname(__FILE__, 3) . '/env.yaml');
// PHP: if ($config['gateway'] && $config['gateway']['token'] && $config['gateway']['hooks'])
//
// env.yaml 读不到、YAML 解析失败、或 gateway.token / gateway.hooks 为空，
// 一律返回 ErrBadConfig —— 对应 PHP 侧进入 else 分支（Bad configuration）。
func LoadConfig(baseDir string) (*Config, error) {
	env, err := config.LoadEnv(baseDir)
	if err != nil {
		return nil, ErrBadConfig
	}
	// PHP 用的是真值判定：空串与 "0" 都是假值。
	if !phpTruthyString(env.Gateway.Token) || !phpTruthyString(env.Gateway.Hooks) {
		return nil, ErrBadConfig
	}
	return &Config{Endpoint: env.Gateway.Hooks, Token: env.Gateway.Token}, nil
}

// ResolveBaseDir 定位 env.yaml 所在目录（<BASE>）。
//
// PHP 侧固定用 dirname(__FILE__, 3)。实测（容器内 php -r）：
//
//	dirname("/data/www/codefever-community/misc/hooks/main", 3)
//	    → /data/www/codefever-community
//	dirname("/data/www/codefever-community/git-storage/hooks/main", 3)
//	    → /data/www/codefever-community
//
// 即两个安装位在**当前目录结构**下都能定位到同一份 env.yaml；
// 但该算法绑死了"可执行文件正好在 <BASE> 下第 3 层"这一前提 ——
// 目录层级一变（多套一层、或把钩子放到别处）就会偏掉。
//
// 本实现改为：
//
//  1. 环境变量 CODEFEVER_BASE_DIR 优先（与 codefever-api 的同名变量一致）；
//  2. 否则从**可执行文件所在目录**逐级向上找 env.yaml；
//  3. 找不到再回退到 <BASE>/env.yaml（容器默认位置 defaultBaseDir）。
func ResolveBaseDir() string {
	if v := strings.TrimSpace(os.Getenv("CODEFEVER_BASE_DIR")); v != "" {
		return v
	}

	exeDir := ""
	if exe, err := os.Executable(); err == nil {
		// os.Executable 在 Linux 上已返回真实路径；EvalSymlinks 兜住其它平台。
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		exeDir = filepath.Dir(exe)
		if dir, ok := findEnvYAMLUpwards(exeDir); ok {
			return dir
		}
	}

	if fileExists(filepath.Join(defaultBaseDir, "env.yaml")) {
		return defaultBaseDir
	}
	if exeDir != "" {
		// 兜底：可执行文件所在目录的上一级。
		return filepath.Dir(exeDir)
	}
	return defaultBaseDir
}

// findEnvYAMLUpwards 从 dir 开始逐级向上找 env.yaml。
func findEnvYAMLUpwards(dir string) (string, bool) {
	for {
		if fileExists(filepath.Join(dir, "env.yaml")) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// fileExists 判断路径是否存在且是普通文件。
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// Execute 是钩子的完整执行流程，返回进程退出码。
//
// 参数与 PHP 侧的对应关系：
//
//	args  —— $argv（含 argv[0] 自身路径，**必须完整透传**）；
//	stdin —— php://stdin。
//
// 与 PHP 版的两处差异（均为有意为之）：
//
//  1. stdin 读到 EOF（PHP 用 stream_set_blocking($input, 0) 非阻塞读，
//     大 payload 有截断风险，见 docs/ARCHITECTURE_ANALYSIS.md:214）；
//  2. env.yaml 的定位方式（见 ResolveBaseDir）。
func Execute(ctx context.Context, baseDir string, args []string, stdin io.Reader, stdout io.Writer) int {
	cfg, err := LoadConfig(baseDir)
	if err != nil {
		// PHP misc/hooks/main:11-12
		fmt.Fprint(stdout, BadConfigMessage)
		return ExitReject
	}

	// 读到 EOF —— PHP 的非阻塞读会截断大 payload，这里是明确的修复。
	payload := Payload{
		UID:   UIDFromEnv(),
		Args:  args,
		Stdin: readAll(stdin),
	}

	body, err := postHook(ctx, cfg, EncodePayload(payload))
	if err != nil {
		// PHP: curl_exec 失败返回 false → json_decode(false) 得到 null → 走 else 分支。
		fmt.Fprint(stdout, ContactAdminMessage)
		return ExitReject
	}
	return handleResponse(body, stdout)
}

// readAll 读完 stdin。
//
// 读出错时返回已读到的部分（PHP 的 stream_get_contents 同样可能只拿到一部分），
// 并把错误写到 stderr —— stdout 必须保持与 PHP 逐字节一致。
func readAll(stdin io.Reader) string {
	if stdin == nil {
		return ""
	}
	data, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[CodeFever Next]: 读取 stdin 失败: %v\n", err)
	}
	return string(data)
}

// postHook 把载荷 POST 给后端回调地址，返回响应体。
//
// PHP misc/hooks/main:15-37 的 curl 配置逐项对应：
//
//	CURLOPT_URL         → cfg.Endpoint
//	CURLOPT_HTTPHEADER  → Token / Content-Type / Accept
//	CURLOPT_POST        → POST
//	CURLOPT_POSTFIELDS  → 请求体
//
// 注意 PHP **不检查 HTTP 状态码** —— 保护分支拒绝时后端返回 403 +
// JSON 信封，PHP 照样解析响应体。因此这里也不按状态码判定失败。
func postHook(ctx context.Context, cfg *Config, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	// 逐字对应 PHP 的三个 CURLOPT_HTTPHEADER。
	req.Header.Set("Token", cfg.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{
		Timeout: requestTimeout,
		Transport: &http.Transport{
			// curl 默认**不**发送 Accept-Encoding（除非设置了 CURLOPT_ENCODING），
			// Go 的 http.Transport 默认会发 "Accept-Encoding: gzip" 并自动解压 ——
			// 关掉压缩，行为与 curl 一致（后端也就不会走压缩分支）。
			DisableCompression: true,
		},
		// curl 默认**不跟随重定向**（CURLOPT_FOLLOWLOCATION 默认关闭），
		// Go 的 http.Client 默认跟随 —— 这里关掉，保持一致。
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// handleResponse 解析响应体并输出，返回退出码。
//
// 逐字对应 misc/hooks/main:39-58：
//
//	$result = json_decode($response, TRUE);
//	if ($result) {
//	    if (!$result['code']) {            // 成功：code == 0
//	        if ($result['data'] && $result['data']['remote']) { echo $result['data']['remote']; }
//	        exit(0);
//	    } else {                            // 业务失败：code != 0
//	        echo 'Operation Stopped: ';
//	        if ($result['data'] && $result['data']['remote']) { echo $result['data']['remote'] . "\n"; }
//	        echo 'Request-ID: ' . $result['request-id'] . "\n";
//	    }
//	} else {
//	    echo 'Contact site administrator if operation rejected!';
//	}
//	exit(255);
func handleResponse(body []byte, stdout io.Writer) int {
	decoded, ok := decodeJSON(body)
	if !ok || !phpTruthy(decoded) {
		// PHP misc/hooks/main:54-56（json_decode 失败 / 顶层为假值）
		fmt.Fprint(stdout, ContactAdminMessage)
		return ExitReject
	}

	code, requestID, remote := extractEnvelope(decoded)

	if phpTruthy(code) {
		// 业务失败分支：PHP misc/hooks/main:47-53
		fmt.Fprint(stdout, operationStoppedPrefix)
		if phpTruthyString(remote) {
			// remote 自身可能自带换行，原样透传，不额外 trim。
			fmt.Fprint(stdout, remote+"\n")
		}
		fmt.Fprint(stdout, requestIDPrefix+requestID+"\n")
		return ExitReject
	}

	// 成功分支：PHP misc/hooks/main:42-46（remote 原样输出，不额外加换行）
	if phpTruthyString(remote) {
		fmt.Fprint(stdout, remote)
	}
	return ExitAllow
}

// decodeJSON 解析响应体。
//
// PHP: $result = json_decode($response, TRUE);
//
// json_decode 失败返回 null；解析成功但顶层是 false / null / 0 / "" / []
// 时 $result 同样是**假值** —— PHP 侧这两种情况走同一个分支，这里保持一致。
func decodeJSON(body []byte) (any, bool) {
	dec := json.NewDecoder(bytes.NewReader(body))
	// 保留数字字面量，避免 0.0 之类被折成科学计数法。
	dec.UseNumber()

	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, false
	}
	// PHP 的 json_decode 要求整串都是合法 JSON：
	// 尾部有多余内容时返回 null。
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, false
	}
	return v, true
}

// extractEnvelope 从已解析的响应里取出 code / request-id / data.remote。
//
// 顶层是对象时逐字对应 PHP 的 $result['code'] / $result['request-id'] /
// $result['data']['remote']；非对象时按 PHP 的取值语义退化
// （数组与数字取不到下标 → null；字符串按 PHP 8 的语义取下标 0）。
func extractEnvelope(v any) (code any, requestID string, remote string) {
	switch t := v.(type) {
	case map[string]any:
		code = t["code"]
		requestID = phpString(t["request-id"])
		if data, ok := t["data"].(map[string]any); ok {
			remote = phpString(data["remote"])
		}
	case []any:
		// $result['code'] 未定义 → null（真值判定为假 → 放行）
	case string:
		// PHP 8：字符串取非整型下标 → 取下标 0 的字符。
		if len(t) > 0 {
			code = t[:1]
		}
	default:
		// 数字 / 布尔：$result['code'] 未定义 → null
	}
	return code, requestID, remote
}

// phpString 按 PHP 的字符串转换规则取值。
func phpString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		if t {
			return "1"
		}
		return ""
	case json.Number:
		return t.String()
	case float64:
		return strconv.FormatFloat(t, 'G', -1, 64)
	default:
		return ""
	}
}

// phpTruthy 按 PHP 的真值规则判断。
//
// PHP 的规则：null / false / 0 / 0.0 / "" / "0" / 空数组 / 空对象 为假，其余为真。
func phpTruthy(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case string:
		return phpTruthyString(t)
	case json.Number:
		f, err := t.Float64()
		return err == nil && f != 0
	case float64:
		return t != 0
	case []any:
		return len(t) > 0
	case map[string]any:
		return len(t) > 0
	default:
		return true
	}
}

// phpTruthyString 是字符串版本的 PHP 真值判定（空串与 "0" 为假）。
func phpTruthyString(s string) bool { return s != "" && s != "0" }
