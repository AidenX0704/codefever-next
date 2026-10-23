package contract

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// TestErrorCodeTable 锁定全部错误码的数值、HTTP 状态与消息。
//
// 这些值来自 application/libraries/service/ErrorCode/ErrorCode.php，
// 前端 www/view/src/helpers/ErrorCodeParser.js 按数值分支处理，
// 任何一个数字或文案偏移都会导致前端行为错误，因此逐条断言。
func TestErrorCodeTable(t *testing.T) {
	cases := []struct {
		code    ErrorCode
		status  int
		message string
	}{
		{CodeSuccess, 200, "Success"},
		{CodeRequestError, 403, "Request Error"},
		{CodeInvalidCredential, 403, "Invalid Login Credential"},
		{CodeMethodNotAllowed, 400, "Method Not Allowed"},
		{CodeHeaderRequired, 400, "Accept and Content-Type Is Required In Header"},
		{CodeContentLength, 400, "Content-Length Is Not Match The Content Size"},
		{CodeEndpointNotFound, 404, "Endpoint Not Found Or Not Allowed Current Request Method"},
		{CodeActionNotAllowed, 403, "Action Not Allowed"},
		{CodeEmpty, 400, "empty"},
		{CodeMissingParameter, 400, "Missing Parameters"},
		{CodeAmountOutOfRange, 400, "Missing Amount Or Amount Out Of Range"},
		{CodeGroupNotFound, 404, "Group Not Found"},
		{CodeRepositoryNotFound, 404, "Repository Not Found"},
		{CodeDuplicateRepoSlug, 400, "Duplicate Repository URL Slug"},
		{CodeDuplicateRepoDisplayName, 400, "Duplicate Repository Display Name"},
		{CodeDuplicateGroupSlug, 400, "Duplicate Group URL Slug"},
		{CodeDuplicateGroupDisplay, 400, "Duplicate Group Display Name"},
		{CodeOperateFail, 400, "Operate Fail"},
		{CodeTargetExists, 400, "New Target Exists"},
		{CodeUserNotFound, 404, "User Not Found"},
		{CodeCannotAddCreator, 404, "Can Not Add Creator As Member"},
		{CodeCannotDeleteUserGroup, 400, "Can Not Delete User Group"},
		{CodeCannotDeleteNonEmpty, 400, "Can Not Non-empty Group"},
		{CodeCannotChangeOwnerOfGroup, 400, "Can Not Change Owner Of User Group"},
		{CodeOldTargetNotExists, 400, "Old Target Not Exists"},
		{CodeDuplicateSSHKey, 400, "Duplicate SSH Key"},
		{CodeFileNotFound, 404, "File Not Found"},
		{CodeBranchProtected, 403, "Branch Proteced"},
		{CodePasswordNotCorrect, 403, "Password Not Correct"},
		{CodeMFAInvalid, 403, "MFA Code Invalid"},
		{CodeMFAInvalidSecond, 403, "MFA Code Invalid (2)"},
	}

	if len(cases) != len(codeTable) {
		t.Fatalf("错误码表数量不符：用例 %d 条，表 %d 条", len(cases), len(codeTable))
	}
	for _, c := range cases {
		if got := c.code.HTTPStatus(); got != c.status {
			t.Errorf("0x%04X HTTPStatus = %d, want %d", int(c.code), got, c.status)
		}
		if got := c.code.Message(); got != c.message {
			t.Errorf("0x%04X Message = %q, want %q", int(c.code), got, c.message)
		}
	}
}

// TestErrorCodeThreshold 锁定前端依赖的 0x0400 阈值语义。
//
// NetworkHelper.js：code < 0x0400 → 业务错误弹 toast；code > 0x0400 → 系统错误。
func TestErrorCodeThreshold(t *testing.T) {
	business := []ErrorCode{
		CodeRequestError, CodeInvalidCredential, CodeMethodNotAllowed, CodeHeaderRequired,
		CodeContentLength, CodeEndpointNotFound, CodeActionNotAllowed,
		CodeEmpty, CodeMissingParameter, CodeAmountOutOfRange,
		CodeGroupNotFound, CodeRepositoryNotFound,
	}
	for _, c := range business {
		if int(c) >= 0x0400 {
			t.Errorf("0x%04X 应属于业务错误区（< 0x0400）", int(c))
		}
	}
}

// TestEnvelopeDataEmptyIsArray 锁定失败响应的 data 必须是空数组而非 null。
func TestEnvelopeDataEmptyIsArray(t *testing.T) {
	e := NewErrorEnvelope("REQ", NewError(CodeInvalidCredential))
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	if !strings.Contains(string(raw), `"data":[]`) {
		t.Fatalf("失败信封的 data 应为 []，实际 %s", raw)
	}
	if !strings.Contains(string(raw), `"request-id":"REQ"`) {
		t.Fatalf("request-id 应出现在信封中，实际 %s", raw)
	}
	// 字段顺序：request-id, code, message, data。
	idx := func(sub string) int { return strings.Index(string(raw), sub) }
	if !(idx(`"request-id"`) < idx(`"code"`) && idx(`"code"`) < idx(`"message"`) && idx(`"message"`) < idx(`"data"`)) {
		t.Fatalf("信封字段顺序不符: %s", raw)
	}
}

// TestErrorEnvelopeWithExtraMessage 锁定 extraMessage 的拼接格式。
func TestErrorEnvelopeWithExtraMessage(t *testing.T) {
	e := NewErrorWithExtra(CodeMissingParameter, "字段 name 缺失")
	want := "Missing Parameters >>> Extra: 字段 name 缺失"
	if got := e.Message(); got != want {
		t.Fatalf("Message = %q, want %q", got, want)
	}
}

// TestNewRequestIDFormat 锁定 request-id 的格式：大写带连字符的 UUID 形状。
func TestNewRequestIDFormat(t *testing.T) {
	id := NewRequestID()
	parts := strings.Split(id, "-")
	if len(parts) != 5 {
		t.Fatalf("request-id 应为 5 段，实际 %q", id)
	}
	wantLens := []int{8, 4, 4, 4, 12}
	for i, p := range parts {
		if len(p) != wantLens[i] {
			t.Fatalf("第 %d 段长度应为 %d，实际 %d (%q)", i+1, wantLens[i], len(p), id)
		}
		if strings.ToUpper(p) != p {
			t.Fatalf("request-id 应全大写，实际 %q", id)
		}
	}
}

// TestNewKeyFormat 锁定业务主键格式：32 位小写 hex。
func TestNewKeyFormat(t *testing.T) {
	k := NewKey()
	if len(k) != 32 {
		t.Fatalf("主键长度应为 32，实际 %d", len(k))
	}
	if strings.ToLower(k) != k {
		t.Fatalf("主键应全小写，实际 %q", k)
	}
	for _, c := range k {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Fatalf("主键应为 hex，实际 %q", k)
		}
	}
}

// TestParseAcceptStrict 锁定 Accept 头的严格匹配。
//
// PHP 侧不做 explode(';')，因此带 charset 参数的 Accept 会被判为非法 → 0x0103。
func TestParseAcceptStrict(t *testing.T) {
	cases := []struct {
		accept string
		want   AcceptMode
		fail   bool
	}{
		{"application/json", AcceptJSON, false},
		{"*/*", AcceptBinary, false},
		{"application/json; charset=utf-8", "", true},
		{"text/html", "", true},
		{"", "", true},
	}
	for _, c := range cases {
		r := httptest.NewRequest(http.MethodGet, "/api/user/info", nil)
		if c.accept != "" {
			r.Header.Set("Accept", c.accept)
		}
		_, err := Parse(r)
		if c.fail {
			if err == nil {
				t.Errorf("Accept=%q 应当被拒绝", c.accept)
			} else if apiErr, ok := err.(*APIError); !ok || apiErr.Code != CodeHeaderRequired {
				t.Errorf("Accept=%q 错误码应为 0x0103，实际 %v", c.accept, err)
			}
		} else if err != nil {
			t.Errorf("Accept=%q 不应被拒绝: %v", c.accept, err)
		}
	}
}

// TestParseContentType 锁定 Content-Type 的分派规则。
func TestParseContentType(t *testing.T) {
	cases := []struct {
		ct   string
		want ContentMode
	}{
		{"", ContentGet},
		{"application/json", ContentJSON},
		{"multipart/form-data; boundary=xxx", ContentForm},
		{"application/x-www-form-urlencoded", ContentFormText},
	}
	for _, c := range cases {
		r := httptest.NewRequest(http.MethodGet, "/api/user/info", nil)
		if c.ct != "" {
			r.Header.Set("Content-Type", c.ct)
		}
		r.Header.Set("Accept", "application/json")
		p, err := Parse(r)
		if err != nil {
			t.Fatalf("CT=%q 不应被拒绝: %v", c.ct, err)
		}
		if p.ContentMode != c.want {
			t.Errorf("CT=%q → %q, want %q", c.ct, p.ContentMode, c.want)
		}
	}
}

// TestParseMethodWhitelist 锁定方法白名单。
func TestParseMethodWhitelist(t *testing.T) {
	for _, m := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete} {
		r := httptest.NewRequest(m, "/api/user/info", nil)
		r.Header.Set("Accept", "application/json")
		if _, err := Parse(r); err != nil {
			t.Errorf("方法 %s 不应被拒绝: %v", m, err)
		}
	}
	for _, m := range []string{http.MethodPatch, http.MethodOptions, http.MethodHead} {
		r := httptest.NewRequest(m, "/api/user/info", nil)
		r.Header.Set("Accept", "application/json")
		_, err := Parse(r)
		if err == nil {
			t.Errorf("方法 %s 应被拒绝", m)
			continue
		}
		if apiErr, ok := err.(*APIError); !ok || apiErr.Code != CodeMethodNotAllowed {
			t.Errorf("方法 %s 错误码应为 0x0102，实际 %v", m, err)
		}
	}
}

// TestParseContentLengthMismatch 锁定 Content-Length 校验。
func TestParseContentLengthMismatch(t *testing.T) {
	body := strings.NewReader(`{"a":1}`)
	r := httptest.NewRequest(http.MethodPost, "/api/user/info", body)
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Content-Length", "999")
	_, err := Parse(r)
	if err == nil {
		t.Fatal("Content-Length 不匹配应被拒绝")
	}
	if apiErr, ok := err.(*APIError); !ok || apiErr.Code != CodeContentLength {
		t.Fatalf("错误码应为 0x0104，实际 %v", err)
	}
}

// TestParsePHPArraySemantics 锁定 PHP 数组语义的还原。
func TestParsePHPArraySemantics(t *testing.T) {
	body := "a=1&a=2&list[]=x&list[]=y&nested[sub]=v&deep[k][j]=w"
	r := httptest.NewRequest(http.MethodPost, "/api/user/info", strings.NewReader(body))
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Content-Length", strconv.Itoa(len(body)))
	p, err := Parse(r)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	// 普通键重复时 PHP 保留最后一个值。
	if got := p.BodyString("a"); got != "2" {
		t.Errorf("a = %q, want \"2\"（PHP 保留最后一个）", got)
	}

	list, ok := p.Body["list"].([]any)
	if !ok || len(list) != 2 || list[0] != "x" || list[1] != "y" {
		t.Errorf("list = %#v, want []any{\"x\",\"y\"}", p.Body["list"])
	}

	nested, ok := p.Body["nested"].(map[string]any)
	if !ok || nested["sub"] != "v" {
		t.Errorf("nested = %#v, want map{sub:\"v\"}", p.Body["nested"])
	}

	deep, ok := p.Body["deep"].(map[string]any)
	if !ok {
		t.Fatalf("deep = %#v, want map", p.Body["deep"])
	}
	inner, ok := deep["k"].(map[string]any)
	if !ok || inner["j"] != "w" {
		t.Errorf("deep.k = %#v, want map{j:\"w\"}", deep["k"])
	}
}

// TestPagingNormalization 锁定 PHP 的分页归一化行为。
//
// PHP 用 is_int($_GET['page']) 判定，而 query 值恒为字符串，
// 因此走 query 的分页端点永远被改写为 page=1 / pagesize=20。
func TestPagingNormalization(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/repository/commitList?repository=x&page=5&pagesize=50", nil)
	r.Header.Set("Accept", "application/json")
	p, err := Parse(r)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if got := p.QueryInt("page"); got != 1 {
		t.Errorf("query page = %d, want 1（复刻 PHP 的 is_int 判定失效）", got)
	}
	if got := p.QueryInt("pagesize"); got != DefaultPageSize {
		t.Errorf("query pagesize = %d, want %d", got, DefaultPageSize)
	}
	// 原始值仍可取到，便于将来修正该缺陷。
	if got := p.RawQuery.Get("page"); got != "5" {
		t.Errorf("RawQuery page = %q, want \"5\"", got)
	}
}

// TestParseJSONBody 锁定 JSON body 的解析。
func TestParseJSONBody(t *testing.T) {
	raw := `{"name":"x","n":3}`
	r := httptest.NewRequest(http.MethodPost, "/api/user/info", strings.NewReader(raw))
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Content-Length", strconv.Itoa(len(raw)))
	p, err := Parse(r)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if got := p.JSONString("name"); got != "x" {
		t.Errorf("name = %q, want \"x\"", got)
	}
	if got := p.JSONInt("n"); got != 3 {
		t.Errorf("n = %d, want 3", got)
	}
}

// TestServerExposesContentLengthHeader 锁定一个容易被忽略的前提：
// Go 的 net/http server 会把 Content-Length 保留在 r.Header 中，
// 而 httptest.NewRequest 不会。契约解析的长度校验依赖这一点，
// 因此用真实 server 固化该行为，避免将来误改解析器导致线上全量 0x0104。
func TestServerExposesContentLengthHeader(t *testing.T) {
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Content-Length")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	raw := `{"a":1}`
	req, err := http.NewRequest(http.MethodPost, srv.URL, strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	if seen != strconv.Itoa(len(raw)) {
		t.Fatalf("真实 server 的 Content-Length 头 = %q, want %q", seen, strconv.Itoa(len(raw)))
	}
}

// TestParseKeepsRequestIDOnError 锁定出错时 request-id 仍然可用。
func TestParseKeepsRequestIDOnError(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/user/info", nil)
	p, err := Parse(r)
	if err == nil {
		t.Fatal("缺少 Accept 应当报错")
	}
	if p == nil || p.ID == "" {
		t.Fatal("出错时仍应返回带 request-id 的解析结果")
	}
}

// TestParseMultipartKeepsFormFields 锁定 multipart 表单字段不丢失。
//
// 这是曾经踩过的坑：Parse() 先 io.ReadAll 读完请求流，再调 r.ParseMultipartForm，
// 此时流已空，导致**所有** multipart 表单参数被静默丢弃 ——
// 表现为"任何 multipart POST 都报参数缺失"，且缺参用例照样通过（错误的原因相同）。
func TestParseMultipartKeepsFormFields(t *testing.T) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.WriteField("name", "codefever"); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteField("email", "root@codefever.cn"); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteField("count", "7"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodPost, "/api/user/updateBasicInfo", &buf)
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Content-Type", w.FormDataContentType())

	p, err := Parse(r)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if got := p.BodyString("name"); got != "codefever" {
		t.Errorf("name = %q, want \"codefever\"", got)
	}
	if got := p.BodyString("email"); got != "root@codefever.cn" {
		t.Errorf("email = %q, want \"root@codefever.cn\"", got)
	}
	if got := p.BodyInt("count"); got != 7 {
		t.Errorf("count = %d, want 7", got)
	}
}

// TestParseMultipartKeepsFile 锁定 multipart 文件上传不丢失。
func TestParseMultipartKeepsFile(t *testing.T) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("avatar", "a.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("\x89PNG\r\n\x1a\nfake")); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteField("other", "x"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest(http.MethodPost, "/api/user/uploadAvatar", &buf)
	r.Header.Set("Accept", "application/json")
	r.Header.Set("Content-Type", w.FormDataContentType())

	p, err := Parse(r)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	f := p.File("avatar")
	if f == nil {
		t.Fatal("上传文件丢失")
	}
	if f.Filename != "a.png" {
		t.Errorf("文件名 = %q, want \"a.png\"", f.Filename)
	}
	content, err := f.ReadAll()
	if err != nil {
		t.Fatalf("读取上传内容失败: %v", err)
	}
	if len(content) != 12 {
		t.Errorf("上传内容长度 = %d, want 12", len(content))
	}
	// 普通字段也不能因为有文件而丢失。
	if got := p.BodyString("other"); got != "x" {
		t.Errorf("other = %q, want \"x\"", got)
	}
}
