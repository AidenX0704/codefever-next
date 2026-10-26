package publicapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestBearerToken 锁定访问令牌的提取规则。
//
// 支持两种头写法（并额外兼容 GitHub 客户端），
// 以及 query 参数（部分 CI 场景无法设置请求头）。
func TestBearerToken(t *testing.T) {
	cases := []struct {
		name  string
		build func() *http.Request
		want  string
	}{
		{
			name: "Token 头",
			build: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/api/v1/user", nil)
				r.Header.Set("Authorization", "token abc123")
				return r
			},
			want: "abc123",
		},
		{
			name: "Bearer 头",
			build: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/api/v1/user", nil)
				r.Header.Set("Authorization", "Bearer abc123")
				return r
			},
			want: "abc123",
		},
		{
			name: "大小写不敏感的前缀",
			build: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/api/v1/user", nil)
				r.Header.Set("Authorization", "TOKEN abc123")
				return r
			},
			want: "abc123",
		},
		{
			name: "query 参数",
			build: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/api/v1/user?access_token=abc123", nil)
			},
			want: "abc123",
		},
		{
			name: "无凭据",
			build: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/api/v1/user", nil)
			},
			want: "",
		},
		{
			name: "不支持的前缀",
			build: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/api/v1/user", nil)
				r.Header.Set("Authorization", "Basic abc123")
				return r
			},
			want: "",
		},
		{
			name: "缺少令牌部分",
			build: func() *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/api/v1/user", nil)
				r.Header.Set("Authorization", "token")
				return r
			},
			want: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := BearerToken(c.build()); got != c.want {
				t.Errorf("BearerToken() = %q, want %q", got, c.want)
			}
		})
	}
}

// TestWriteErrorEnvelope 锁定公开 API 的错误体形态。
//
// 公开 API **不复用**内部 /api/* 的 {request-id,code,message,data} 信封，
// 而是 {message,url} 形态 —— 这条断言就是防止有人误改回去。
func TestWriteErrorEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, http.StatusUnauthorized, "缺少访问令牌")

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("状态码 = %d, want 401", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want application/json; charset=utf-8", ct)
	}
	body := rec.Body.String()
	want := `{"message":"缺少访问令牌","url":"https://localhost/doc/cn/api/overview.md"}`
	if body != want {
		t.Errorf("响应体 = %s, want %s", body, want)
	}
}

// TestSetDocURL 锁定错误体文档地址的拼装规则。
//
// 文档地址必须落在**文档中心的 API 文档页**上：早先拼成站点根的 /api/v1，
// 而那个前缀只服务 JSON 端点、浏览器访问是 404，等于错误提示里给了一条死链。
func TestSetDocURL(t *testing.T) {
	original := docsURL
	t.Cleanup(func() { docsURL = original })

	SetDocURL("https://git.example.com")
	if docsURL != "https://git.example.com"+DefaultDocPath {
		t.Errorf("docsURL = %q, want %q", docsURL, "https://git.example.com"+DefaultDocPath)
	}
	// 站点地址带尾斜杠时不能拼出双斜杠。
	SetDocURL("https://git.example.com/")
	if docsURL != "https://git.example.com"+DefaultDocPath {
		t.Errorf("尾斜杠未收敛：docsURL = %q", docsURL)
	}
	// 空串表示"保持默认"，不能把地址拼成只有路径。
	SetDocURL("")
	if docsURL != "https://git.example.com"+DefaultDocPath {
		t.Errorf("空串不应改写地址：docsURL = %q", docsURL)
	}
}

// TestIsPublicAPIPath 锁定公开 API 路径前缀判定。
func TestIsPublicAPIPath(t *testing.T) {
	cases := map[string]bool{
		"/api/v1":                  true,
		"/api/v1/":                 true,
		"/api/v1/user":             true,
		"/api/v1/repos/owner/repo": true,
		"/api/user/info":           false,
		"/api/v1x/user":            false,
		"/apiv1/internal/action":   false,
		"/api":                     false,
		"/user/login":              false,
	}
	for path, want := range cases {
		if got := IsPublicAPIPath(path); got != want {
			t.Errorf("IsPublicAPIPath(%q) = %v, want %v", path, got, want)
		}
	}
}

// TestUserNumericIDStable 锁定主键 → 数字 ID 的映射稳定性与取值范围。
//
// 公开 API 用数字 ID 标识用户与仓库（与常见的自增 ID 结构对齐），
// 而内部主键是 32 位 hex。映射必须对同一主键恒定，
// 否则同一仓库在不同请求里会得到不同的 ID。
func TestUserNumericIDStable(t *testing.T) {
	key := "de7347a84f6236f06b0356ce6668c27d"
	first := userNumericID(key)
	if first != userNumericID(key) {
		t.Error("userNumericID 对同一主键返回了不同的值")
	}
	if first == 0 {
		t.Error("userNumericID 返回 0，映射失败")
	}
	// 不同主键应得到不同 ID（含"仅末位不同"这一最易碰撞的情形）。
	if userNumericID("de7347a84f6236f06b0356ce6668c27e") == first {
		t.Error("不同主键映射到了相同的数字 ID")
	}
	// 空主键映射为 0。
	if got := userNumericID(""); got != 0 {
		t.Errorf("userNumericID(\"\") = %d, want 0", got)
	}
}

// TestUserNumericIDRange 锁定 ID 的取值范围：恒非负，且不超过 JS 安全整数上限。
//
// 两条都踩过：
//   - 直接 int64(uint64) 会把最高位当符号位，约一半对象拿到负 ID；
//   - 64 位值超过 2^53-1 后，JavaScript 的 Number 无法精确表示 ——
//     JSON.parse 会把 7341916824063314155 读成 7341916824063314000，
//     调用方按 ID 比较或建索引就会错。
func TestUserNumericIDRange(t *testing.T) {
	const maxSafe = int64(1)<<53 - 1

	// 用一批构造值覆盖各种字节形态（含全 1 的高位，足以触发符号位与精度问题）。
	keys := []string{
		"de7347a84f6236f06b0356ce6668c27d",
		"0f9c1f4f7a2b4c3d8e5f60718293a4b5",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"ffffffffffffffffffffffffffffffff",
		"00000000000000000000000000000001",
	}
	for _, key := range keys {
		got := userNumericID(key)
		if got < 0 {
			t.Errorf("userNumericID(%q) = %d，出现了负值", key, got)
		}
		if got > maxSafe {
			t.Errorf("userNumericID(%q) = %d 超过 JS 安全整数上限 %d", key, got, maxSafe)
		}
	}

	// 固定一组已知映射，防止有人无意改动公式（改了会让所有对外 ID 变化）。
	if got := userNumericID("de7347a84f6236f06b0356ce6668c27d"); got != 8779932756338603 {
		t.Errorf("映射公式已变化：userNumericID = %d, want %d", got, int64(8779932756338603))
	}
}
