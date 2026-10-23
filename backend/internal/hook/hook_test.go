package hook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// capturedRequest 记录假回调端点收到的请求。
type capturedRequest struct {
	method string
	header http.Header
	body   []byte
}

// newStubServer 起一个假回调端点，把收到的请求记到 captured 里，
// 并用 status/body 作为响应。
func newStubServer(t *testing.T, captured *capturedRequest, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("读取请求体失败: %v", err)
		}
		captured.method = r.Method
		captured.header = r.Header.Clone()
		captured.body = raw
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// writeEnvYAML 在 dir 下写入一份 env.yaml（gateway 段指向 endpoint）。
func writeEnvYAML(t *testing.T, dir, endpoint, token string) {
	t.Helper()
	content := fmt.Sprintf(`env: 'production'

gateway:
  token: %s
  hooks: %s
  authHTTP: %s
  authSSH: %s
`, token, endpoint, endpoint, endpoint)
	if err := os.WriteFile(filepath.Join(dir, "env.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("写入 env.yaml 失败: %v", err)
	}
}

// runHook 跑一次钩子，返回 stdout 与退出码。
func runHook(t *testing.T, baseDir string, args []string, stdin string) (string, int) {
	t.Helper()
	var out bytes.Buffer
	code := Execute(context.Background(), baseDir, args, strings.NewReader(stdin), &out)
	return out.String(), code
}

// ---------------------------------------------------------------------------
// 请求形状
// ---------------------------------------------------------------------------

// TestRequestShape 断言请求方法、请求头与请求体结构。
//
// 对应 PHP misc/hooks/main:15-33。
func TestRequestShape(t *testing.T) {
	var captured capturedRequest
	srv := newStubServer(t, &captured, http.StatusOK, `{"request-id":"rid","code":0,"message":"Success","data":{"remote":""}}`)

	base := t.TempDir()
	writeEnvYAML(t, base, srv.URL, "secret-token")

	args := []string{"/data/www/codefever-community/misc/hooks/pre-receive"}
	stdin := "abc123 def456 refs/heads/master\n"

	out, code := runHook(t, base, args, stdin)
	if code != ExitAllow {
		t.Fatalf("退出码 = %d, 期望 %d (stdout=%q)", code, ExitAllow, out)
	}

	if captured.method != http.MethodPost {
		t.Errorf("方法 = %q, 期望 POST", captured.method)
	}
	if got := captured.header.Get("Token"); got != "secret-token" {
		t.Errorf("Token 头 = %q, 期望 %q", got, "secret-token")
	}
	if got := captured.header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type 头 = %q, 期望 application/json", got)
	}
	if got := captured.header.Get("Accept"); got != "application/json" {
		t.Errorf("Accept 头 = %q, 期望 application/json", got)
	}

	// 请求体必须是 {"uid":…,"args":[…],"stdin":…}。
	var body map[string]any
	if err := json.Unmarshal(captured.body, &body); err != nil {
		t.Fatalf("请求体不是合法 JSON: %v (%s)", err, captured.body)
	}
	if _, ok := body["uid"]; !ok {
		t.Errorf("请求体缺少 uid 字段: %s", captured.body)
	}
	argv, ok := body["args"].([]any)
	if !ok {
		t.Fatalf("args 不是数组: %#v", body["args"])
	}
	if len(argv) != 1 || argv[0] != args[0] {
		t.Errorf("args = %#v, 期望完整透传 %#v", argv, args)
	}
	if got := body["stdin"]; got != stdin {
		t.Errorf("stdin = %q, 期望 %q", got, stdin)
	}
}

// TestArgsFullPassthrough 断言 argv 全部透传（含 argv[0] 自身路径与 update 钩子的 3 个参数）。
//
// 对应 PHP misc/hooks/main:29 —— 'args' => $argv。
func TestArgsFullPassthrough(t *testing.T) {
	var captured capturedRequest
	srv := newStubServer(t, &captured, http.StatusOK, `{"request-id":"rid","code":0,"data":{"remote":""}}`)

	base := t.TempDir()
	writeEnvYAML(t, base, srv.URL, "tok")

	args := []string{"/base/git-storage/repo/hooks/update", "refs/heads/master", "old0000", "new1111"}
	if _, code := runHook(t, base, args, ""); code != ExitAllow {
		t.Fatalf("退出码 = %d, 期望 0", code)
	}

	var body struct {
		Args []string `json:"args"`
	}
	if err := json.Unmarshal(captured.body, &body); err != nil {
		t.Fatalf("请求体解析失败: %v", err)
	}
	if len(body.Args) != len(args) {
		t.Fatalf("args 长度 = %d, 期望 %d (%#v)", len(body.Args), len(args), body.Args)
	}
	for i := range args {
		if body.Args[i] != args[i] {
			t.Errorf("args[%d] = %q, 期望 %q", i, body.Args[i], args[i])
		}
	}
}

// TestUIDUnsetIsFalse 断言 PGYER_UID 未设置时 uid 编码为 false。
//
// 对应 PHP misc/hooks/main:28 —— getenv() 未命中时返回 false。
func TestUIDUnsetIsFalse(t *testing.T) {
	var captured capturedRequest
	srv := newStubServer(t, &captured, http.StatusOK, `{"request-id":"rid","code":0,"data":{"remote":""}}`)

	base := t.TempDir()
	writeEnvYAML(t, base, srv.URL, "tok")

	t.Setenv("PGYER_UID", "")
	if err := os.Unsetenv("PGYER_UID"); err != nil {
		t.Fatalf("清除 PGYER_UID 失败: %v", err)
	}

	if _, code := runHook(t, base, []string{"/base/hooks/pre-receive"}, ""); code != ExitAllow {
		t.Fatalf("退出码 = %d, 期望 0", code)
	}
	if !bytes.Contains(captured.body, []byte(`"uid":false`)) {
		t.Errorf("请求体 = %s, 期望含 \"uid\":false", captured.body)
	}
}

// TestUIDSet 断言 PGYER_UID 已设置时 uid 编码为字符串。
func TestUIDSet(t *testing.T) {
	var captured capturedRequest
	srv := newStubServer(t, &captured, http.StatusOK, `{"request-id":"rid","code":0,"data":{"remote":""}}`)

	base := t.TempDir()
	writeEnvYAML(t, base, srv.URL, "tok")
	t.Setenv("PGYER_UID", "u_key|r_key")

	if _, code := runHook(t, base, []string{"/base/hooks/pre-receive"}, ""); code != ExitAllow {
		t.Fatalf("退出码 = %d, 期望 0", code)
	}
	if !bytes.Contains(captured.body, []byte(`"uid":"u_key|r_key"`)) {
		t.Errorf("请求体 = %s, 期望含 \"uid\":\"u_key|r_key\"", captured.body)
	}
}

// TestLargeStdin 断言 ≥1MB 的 stdin 不被截断。
//
// 这是对 PHP 缺陷的修复：misc/hooks/main:26 用 stream_set_blocking($input, 0)
// 非阻塞读，大 payload 会被截断（docs/ARCHITECTURE_ANALYSIS.md:214）。
func TestLargeStdin(t *testing.T) {
	var captured capturedRequest
	srv := newStubServer(t, &captured, http.StatusOK, `{"request-id":"rid","code":0,"data":{"remote":""}}`)

	base := t.TempDir()
	writeEnvYAML(t, base, srv.URL, "tok")

	big := strings.Repeat("0123456789abcdef", 64*1024) // 1 MiB
	if len(big) < 1024*1024 {
		t.Fatalf("测试数据只有 %d 字节, 期望 ≥ 1 MiB", len(big))
	}

	if _, code := runHook(t, base, []string{"/base/hooks/post-receive"}, big); code != ExitAllow {
		t.Fatalf("退出码 = %d, 期望 0", code)
	}

	var body struct {
		Stdin string `json:"stdin"`
	}
	if err := json.Unmarshal(captured.body, &body); err != nil {
		t.Fatalf("请求体解析失败: %v", err)
	}
	if len(body.Stdin) != len(big) {
		t.Fatalf("stdin 长度 = %d, 期望 %d (被截断了 %d 字节)", len(body.Stdin), len(big), len(big)-len(body.Stdin))
	}
	if body.Stdin != big {
		t.Errorf("stdin 内容与原始输入不一致")
	}
}

// ---------------------------------------------------------------------------
// 三条响应分支：stdout 逐字节 + 退出码
// ---------------------------------------------------------------------------

// TestResponseSuccess 断言成功分支：输出 data.remote，退出码 0。
//
// 对应 PHP misc/hooks/main:42-46。
func TestResponseSuccess(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "remote 为空",
			body: `{"request-id":"rid-1","code":0,"message":"Success","data":{"remote":""}}`,
			want: "",
		},
		{
			name: "remote 非空（不加换行）",
			body: `{"request-id":"rid-2","code":0,"message":"Success","data":{"remote":"hello world"}}`,
			want: "hello world",
		},
		{
			name: "remote 自带换行（原样透传）",
			body: `{"request-id":"rid-3","code":0,"message":"Success","data":{"remote":"line1\nline2\n"}}`,
			want: "line1\nline2\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var captured capturedRequest
			srv := newStubServer(t, &captured, http.StatusOK, tc.body)

			base := t.TempDir()
			writeEnvYAML(t, base, srv.URL, "tok")

			out, code := runHook(t, base, []string{"/base/hooks/post-receive"}, "")
			if code != ExitAllow {
				t.Errorf("退出码 = %d, 期望 %d", code, ExitAllow)
			}
			if out != tc.want {
				t.Errorf("stdout = %q, 期望 %q", out, tc.want)
			}
		})
	}
}

// TestResponseRejected 断言业务失败分支：输出 Operation Stopped / Request-ID，退出码 255。
//
// 对应 PHP misc/hooks/main:47-53。
func TestResponseRejected(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "保护分支拒绝（remote 自带前导换行）",
			body: `{"request-id":"rid-9","code":1039,"message":"Branch Proteced",` +
				`"data":{"remote":"\n[CodeFever Next]: You are not allowed to push code to protected branch <master> !"}}`,
			want: "Operation Stopped: \n[CodeFever Next]: You are not allowed to push code to protected branch <master> !\n" +
				"Request-ID: rid-9\n",
		},
		{
			name: "remote 为空",
			body: `{"request-id":"rid-10","code":513,"message":"Missing Parameter","data":{"remote":""}}`,
			want: "Operation Stopped: Request-ID: rid-10\n",
		},
		{
			name: "缺少 request-id",
			body: `{"code":513,"message":"Missing Parameter","data":{"remote":""}}`,
			want: "Operation Stopped: Request-ID: \n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var captured capturedRequest
			// 保护分支拒绝时后端返回 403 —— PHP 不看状态码，只解析响应体。
			srv := newStubServer(t, &captured, http.StatusForbidden, tc.body)

			base := t.TempDir()
			writeEnvYAML(t, base, srv.URL, "tok")

			out, code := runHook(t, base, []string{"/base/hooks/pre-receive"}, "")
			if code != ExitReject {
				t.Errorf("退出码 = %d, 期望 %d", code, ExitReject)
			}
			if out != tc.want {
				t.Errorf("stdout = %q, 期望 %q", out, tc.want)
			}
		})
	}
}

// TestResponseParseFailure 断言解析失败分支：输出提示，退出码 255。
//
// 对应 PHP misc/hooks/main:54-56。
func TestResponseParseFailure(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "非 JSON", body: "not a json"},
		{name: "HTML 错误页", body: "<html><body>502 Bad Gateway</body></html>"},
		{name: "空响应体", body: ""},
		{name: "JSON 但顶层为假值", body: "0"},
		{name: "JSON 尾部有多余内容", body: `{"code":0} garbage`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var captured capturedRequest
			srv := newStubServer(t, &captured, http.StatusInternalServerError, tc.body)

			base := t.TempDir()
			writeEnvYAML(t, base, srv.URL, "tok")

			out, code := runHook(t, base, []string{"/base/hooks/pre-receive"}, "")
			if code != ExitReject {
				t.Errorf("退出码 = %d, 期望 %d", code, ExitReject)
			}
			if out != ContactAdminMessage {
				t.Errorf("stdout = %q, 期望 %q", out, ContactAdminMessage)
			}
		})
	}
}

// TestNetworkFailure 断言网络失败被当作失败（PHP 的 curl_exec 返回 false）。
func TestNetworkFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	endpoint := srv.URL
	srv.Close() // 端口随即不可达

	base := t.TempDir()
	writeEnvYAML(t, base, endpoint, "tok")

	out, code := runHook(t, base, []string{"/base/hooks/pre-receive"}, "")
	if code != ExitReject {
		t.Errorf("退出码 = %d, 期望 %d", code, ExitReject)
	}
	if out != ContactAdminMessage {
		t.Errorf("stdout = %q, 期望 %q", out, ContactAdminMessage)
	}
}

// ---------------------------------------------------------------------------
// 配置缺失
// ---------------------------------------------------------------------------

// TestBadConfig 断言配置缺失时的输出与退出码。
//
// 对应 PHP misc/hooks/main:5-13。
func TestBadConfig(t *testing.T) {
	cases := []struct {
		name    string
		content string // env.yaml 内容；空字符串表示不写 env.yaml
	}{
		{name: "env.yaml 不存在", content: ""},
		{name: "gateway 段缺失", content: "env: 'production'\n"},
		{name: "gateway.token 为空", content: "gateway:\n  token: ''\n  hooks: http://127.0.0.1/hooks\n"},
		{name: "gateway.hooks 为空", content: "gateway:\n  token: tok\n  hooks: ''\n"},
		{name: "env.yaml 不是合法 YAML", content: "gateway: [\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base := t.TempDir()
			if tc.content != "" {
				if err := os.WriteFile(filepath.Join(base, "env.yaml"), []byte(tc.content), 0o644); err != nil {
					t.Fatalf("写入 env.yaml 失败: %v", err)
				}
			}

			out, code := runHook(t, base, []string{"/base/hooks/pre-receive"}, "")
			if code != ExitReject {
				t.Errorf("退出码 = %d, 期望 %d", code, ExitReject)
			}
			if out != BadConfigMessage {
				t.Errorf("stdout = %q, 期望 %q", out, BadConfigMessage)
			}
		})
	}
}

// TestBadConfigDoesNotCallBackend 断言配置缺失时**不会**发出请求。
func TestBadConfigDoesNotCallBackend(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()

	base := t.TempDir()
	writeEnvYAML(t, base, srv.URL, "")

	if _, code := runHook(t, base, []string{"/base/hooks/pre-receive"}, ""); code != ExitReject {
		t.Fatalf("退出码 = %d, 期望 %d", code, ExitReject)
	}
	if called {
		t.Errorf("配置缺失时不应发出请求")
	}
}

// ---------------------------------------------------------------------------
// env.yaml 定位
// ---------------------------------------------------------------------------

// TestResolveBaseDirUpwards 断言从可执行文件所在目录逐级向上找 env.yaml。
func TestResolveBaseDirUpwards(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "misc", "hooks")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "env.yaml"), []byte("gateway:\n  token: t\n  hooks: h\n"), 0o644); err != nil {
		t.Fatalf("写 env.yaml 失败: %v", err)
	}

	got, ok := findEnvYAMLUpwards(base)
	if !ok {
		t.Fatalf("从 %s 向上没找到 env.yaml", base)
	}
	if got != root {
		t.Errorf("找到的目录 = %q, 期望 %q", got, root)
	}
}

// TestResolveBaseDirEnvOverride 断言 CODEFEVER_BASE_DIR 优先。
func TestResolveBaseDirEnvOverride(t *testing.T) {
	t.Setenv("CODEFEVER_BASE_DIR", "/some/where")
	if got := ResolveBaseDir(); got != "/some/where" {
		t.Errorf("ResolveBaseDir = %q, 期望 %q", got, "/some/where")
	}
}

// ---------------------------------------------------------------------------
// PHP 风格 JSON 编码
// ---------------------------------------------------------------------------

// TestEncodePayloadPHPCompatible 锁定请求体的字节级编码。
//
// 期望值由 PHP 的 json_encode 生成（容器内实测）：
//
//	php -r 'echo json_encode(["uid"=>"u_key|r_key","args"=>["/base/hooks/pre-receive"],"stdin"=>"a b refs/heads/master\n"]);'
func TestEncodePayloadPHPCompatible(t *testing.T) {
	got := string(EncodePayload(Payload{
		UID:   "u_key|r_key",
		Args:  []string{"/base/hooks/pre-receive"},
		Stdin: "a b refs/heads/master\n",
	}))
	want := `{"uid":"u_key|r_key","args":["\/base\/hooks\/pre-receive"],"stdin":"a b refs\/heads\/master\n"}`
	if got != want {
		t.Errorf("编码结果 =\n%s\n期望\n%s", got, want)
	}
}

// TestEncodePayloadNonASCII 断言非 ASCII 字符转成 \uXXXX（PHP 默认行为）。
func TestEncodePayloadNonASCII(t *testing.T) {
	got := string(EncodePayload(Payload{
		UID:   "u",
		Args:  []string{},
		Stdin: "中文",
	}))
	want := `{"uid":"u","args":[],"stdin":"\u4e2d\u6587"}`
	if got != want {
		t.Errorf("编码结果 = %s, 期望 %s", got, want)
	}
}

// TestEncodePayloadUIDFalse 断言未设置 PGYER_UID 时 uid 编码为 false。
func TestEncodePayloadUIDFalse(t *testing.T) {
	got := string(EncodePayload(Payload{UID: false, Args: []string{"x"}}))
	want := `{"uid":false,"args":["x"],"stdin":""}`
	if got != want {
		t.Errorf("编码结果 = %s, 期望 %s", got, want)
	}
}
