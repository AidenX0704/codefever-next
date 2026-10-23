// Command contract-collect 采集 CodeFever Next 的 API 契约基线。
//
// 用法（工作目录为 backend/）：
//
//	go run ./cmd/contract-collect
//	go run ./cmd/contract-collect -only user.info -verbose
//	go run ./cmd/contract-collect -base http://localhost:9080 -out testdata/contract
//
// 它读取 contract/manifest.yaml，逐条按清单发请求，把真实响应写到
// testdata/contract/<id>.json。
//
// # 安全约束（硬性，违反即中止，不发请求）
//
//  1. manifest 的 safety.forbidden_write_paths 里的路径，永远不允许收到
//     POST/PUT/DELETE。POST /api/admin/config 属于"单请求砖化全站"的 P0 缺陷：
//     data 字段解码为 null 时 array_merge($config, null) 只发 Warning，随后
//     yaml_emit_file() 把 config.yaml 写成空结构；此后每一次请求都会在
//     www/index.php:21 的 make_constants('YAML_', yaml_parse_file(config.yaml))
//     上抛 TypeError，整个应用（登录 + 全部 API）永久 500。
//
//  2. 写方法（POST/PUT/DELETE）且未标记 readonly: true 的端点，只允许发送
//     "所有参数均为空值/缺省"的请求，即只能走到"缺参 / 资源不存在"这类
//     不会产生任何写操作的失败分支。
//
//  3. 采集期间不写任何业务数据；唯一的会话痕迹是客户端 Cookie
//     （CodeIgniter 的 sess_use_database = FALSE，会话不落库）。
package main

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// 归一化占位符与规则
//
// R1 request-id → <REQUEST_ID>
//
//	JSON 中 key == "request-id" 的字符串值；响应头 Request-ID；
//	raw_body 中的同一段 UUID 文本。
//
// R2 host → <HOST>
//
//	JSON 中 key == "host" 的字符串值（值形如 http://localhost）。
//
// R3 绝对路径 → <BASE>
//
//	任意字符串中的 /data/www/codefever-community 前缀（含其后紧跟的 "/"）。
//
// R4 Unix 时间戳 → <TS>
//
//	JSON 中取值落在 [1.7e9, 2.2e9] 的整数（含端点）。
//
// R5 MFA 随机量 → <MFA_SECRET> / <MFA_QR_IMAGE>
//
//	user.getMFAData 每次都会生成新的 secret 与二维码，属随机量，
//	否则基线每次采集都不可 diff。image 保留 data: 前缀，只替换 base64 载荷。
//
// R6 实时资源指标 → <VM_METRICS>
//
//	admin.systemStatus 的 vm 子树（cpu/内存/磁盘占用）每次都不同。
//
// R7 仓库磁盘占用 → <REPO_SIZE>
//
//	repository.config 的 count.file 是裸仓库的磁盘占用（字节）。
//	它随**任何一次 push**变化：推入新对象后占用变大，
//	把引用删掉并 gc 之后又会变小。演示数据的一次 push 就能让基线无法 diff，
//	而它与后端实现毫无关系 —— 因此按与 vm 同级的"环境易变量"归一化。
//
// R1~R4 是契约要求的基础规则，R5/R6/R7 是为了让基线可 diff 而追加的扩展规则。
// 全部规则都是确定性的：同样的响应永远得到同样的占位符。
// ---------------------------------------------------------------------------
const (
	placeholderRequestID = "<REQUEST_ID>"
	placeholderHost      = "<HOST>"
	placeholderBase      = "<BASE>"
	placeholderTS        = "<TS>"
	placeholderMFASecret = "<MFA_SECRET>"
	placeholderMFAImage  = "<MFA_QR_IMAGE>"
	placeholderVM        = "<VM_METRICS>"
	placeholderRepoSize  = "<REPO_SIZE>"

	// 部署内的绝对路径前缀（env.yaml 所在目录）。
	basePathPrefix = "/data/www/codefever-community"

	// Unix 时间戳判定区间。
	tsMin = 1700000000
	tsMax = 2200000000

	// raw_body 落盘上限 8KB。
	maxRawBody = 8 * 1024
)

// ---------------------------------------------------------------------------
// manifest
// ---------------------------------------------------------------------------

type param struct {
	Name  string `yaml:"name"`
	From  string `yaml:"from"`
	Value any    `yaml:"value"`
}

type endpoint struct {
	ID                string  `yaml:"id"`
	Controller        string  `yaml:"controller"`
	Action            string  `yaml:"action"`
	Method            string  `yaml:"method"`
	Auth              string  `yaml:"auth"`
	Path              string  `yaml:"path"`
	Params            []param `yaml:"params"`
	ExpectError       *int    `yaml:"expect_error"`
	ExpectStatus      *int    `yaml:"expect_status"`
	ExpectContentType string  `yaml:"expect_content_type"`
	ReadOnly          bool    `yaml:"readonly"`
	OmitAccept        bool    `yaml:"omit_accept"`
	BadToken          bool    `yaml:"bad_token"`
	Notes             string  `yaml:"notes"`
}

type loginSpec struct {
	Path string            `yaml:"path"`
	Form map[string]string `yaml:"form"`
}

type sessionSpec struct {
	CookieName      string `yaml:"cookie_name"`
	Key             string `yaml:"key"`
	NonAdminUserKey string `yaml:"non_admin_user_key"`
}

type gatewaySpec struct {
	Token string `yaml:"token"`
}

type safetySpec struct {
	ForbiddenWritePaths              []string `yaml:"forbidden_write_paths"`
	RequireEmptyParamsForWriteEndpts bool     `yaml:"require_empty_params_for_write_endpoints"`
}

type manifest struct {
	BaseURL   string      `yaml:"base_url"`
	Login     loginSpec   `yaml:"login"`
	Session   sessionSpec `yaml:"session"`
	Gateway   gatewaySpec `yaml:"gateway"`
	Safety    safetySpec  `yaml:"safety"`
	Endpoints []endpoint  `yaml:"endpoints"`
}

// ---------------------------------------------------------------------------
// 落盘记录
// ---------------------------------------------------------------------------

type record struct {
	ID           string `json:"id"`
	Method       string `json:"method"`
	Path         string `json:"path"`
	Auth         string `json:"auth"`
	Status       int    `json:"status"`
	ContentType  string `json:"content_type"`
	RequestID    string `json:"request_id"`
	RequestIDFmt string `json:"request_id_format"`
	ExpectError  *int   `json:"expect_error"`
	ExpectStatus *int   `json:"expect_status"`
	ActualCode   *int   `json:"actual_code"`
	Match        bool   `json:"match"`
	Body         any    `json:"body"`
	RawBody      string `json:"raw_body"`
}

func main() {
	var (
		baseFlag     = flag.String("base", "", "覆盖 manifest 里的 base_url")
		manifestFlag = flag.String("manifest", "contract/manifest.yaml", "manifest 路径")
		outFlag      = flag.String("out", "testdata/contract", "输出目录")
		onlyFlag     = flag.String("only", "", "只跑指定 id 的用例")
		verbose      = flag.Bool("verbose", false, "打印每条用例的细节")
	)
	flag.Parse()

	if err := run(*manifestFlag, *baseFlag, *outFlag, *onlyFlag, *verbose); err != nil {
		fmt.Fprintln(os.Stderr, "contract-collect:", err)
		os.Exit(1)
	}
}

func run(manifestPath, baseOverride, outDir, only string, verbose bool) error {
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("读取 manifest 失败: %w", err)
	}
	var m manifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("解析 manifest 失败: %w", err)
	}

	base := strings.TrimRight(m.BaseURL, "/")
	if baseOverride != "" {
		base = strings.TrimRight(baseOverride, "/")
	}
	if base == "" {
		return errors.New("manifest 缺少 base_url")
	}

	manifestDir, err := filepath.Abs(filepath.Dir(manifestPath))
	if err != nil {
		return err
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	// 带 Cookie 的客户端（登录后的 root 会话）与不带 Cookie 的客户端。
	jar, err := cookiejar.New(nil)
	if err != nil {
		return err
	}
	jarClient := &http.Client{Jar: jar, Timeout: 60 * time.Second}
	plainClient := &http.Client{Jar: nil, Timeout: 60 * time.Second}

	if err := login(jarClient, base, m.Login); err != nil {
		return fmt.Errorf("登录失败: %w", err)
	}
	fmt.Println("登录成功：", m.Login.Form["email"])

	forbidden := map[string]bool{}
	for _, p := range m.Safety.ForbiddenWritePaths {
		forbidden[strings.TrimRight(p, "/")] = true
	}

	var (
		okCount     int
		errCount    int
		skipCount   int
		codeCount   = map[int]int{}
		statusCount = map[int]int{}
		mismatches  []string
	)

	for _, ep := range m.Endpoints {
		if only != "" && ep.ID != only {
			continue
		}

		if err := checkSafety(ep, forbidden, m.Safety.RequireEmptyParamsForWriteEndpts); err != nil {
			fmt.Fprintf(os.Stderr, "跳过 %s：%v\n", ep.ID, err)
			skipCount++
			continue
		}

		rec, err := collect(jarClient, plainClient, base, m, ep, manifestDir, verbose)
		if err != nil {
			fmt.Fprintf(os.Stderr, "跳过 %s：%v\n", ep.ID, err)
			skipCount++
			continue
		}

		if err := writeRecord(outDir, rec); err != nil {
			return err
		}

		statusCount[rec.Status]++
		if rec.ActualCode != nil {
			codeCount[*rec.ActualCode]++
			if *rec.ActualCode == 0 && rec.Status == http.StatusOK {
				okCount++
			} else {
				errCount++
			}
		} else {
			errCount++
		}
		if !rec.Match {
			mismatches = append(mismatches, fmt.Sprintf("%s（期望 error=%v status=%v，实际 error=%v status=%d）",
				rec.ID, derefInt(rec.ExpectError), derefInt(rec.ExpectStatus), derefInt(rec.ActualCode), rec.Status))
		}
		if verbose {
			fmt.Printf("%-38s %-4s %-3d code=%-6v %s\n", rec.ID, rec.Method, rec.Status,
				derefInt(rec.ActualCode), rec.ContentType)
		}
	}

	fmt.Println()
	fmt.Println("========== 采集统计 ==========")
	fmt.Printf("用例总数     : %d\n", okCount+errCount+skipCount)
	fmt.Printf("成功(code=0) : %d\n", okCount)
	fmt.Printf("失败/非 JSON : %d\n", errCount)
	fmt.Printf("跳过         : %d\n", skipCount)
	fmt.Println("错误码分布   :")
	for _, c := range sortedKeys(codeCount) {
		fmt.Printf("  %5d (0x%04X) : %d\n", c, c, codeCount[c])
	}
	fmt.Println("HTTP 状态分布:")
	for _, s := range sortedKeys(statusCount) {
		fmt.Printf("  %3d : %d\n", s, statusCount[s])
	}
	if len(mismatches) > 0 {
		fmt.Println("与 manifest 期望不符的用例:")
		for _, mm := range mismatches {
			fmt.Println("  -", mm)
		}
	} else {
		fmt.Println("所有用例均与 manifest 期望一致")
	}
	return nil
}

func derefInt(p *int) any {
	if p == nil {
		return "nil"
	}
	return *p
}

func sortedKeys(m map[int]int) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}

// checkSafety 实现采集器的硬安全约束。
func checkSafety(ep endpoint, forbidden map[string]bool, requireEmpty bool) error {
	isWrite := ep.Method != http.MethodGet

	if isWrite && forbidden[strings.TrimRight(ep.Path, "/")] {
		return fmt.Errorf("safety.forbidden_write_paths 禁止对该路径发送写请求")
	}
	if isWrite && requireEmpty && !ep.ReadOnly {
		for _, p := range ep.Params {
			if !isEmptyValue(p.Value) {
				return fmt.Errorf("写端点未标记 readonly: true，只允许发送缺参/空值请求（违规参数 %q）", p.Name)
			}
		}
	}
	return nil
}

// isEmptyValue 判断参数是否为空值（缺参形态）。
func isEmptyValue(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return t == ""
	case []any:
		return len(t) == 0
	case map[string]any:
		return len(t) == 0
	case bool:
		return !t
	case int:
		return t == 0
	case int64:
		return t == 0
	case float64:
		return t == 0
	default:
		return false
	}
}

// login 走 PHP 侧的表单登录，拿到会话 Cookie。
func login(client *http.Client, base string, spec loginSpec) error {
	form := url.Values{}
	for k, v := range spec.Form {
		form.Set(k, v)
	}
	path := spec.Path
	if path == "" {
		path = "/user/login"
	}
	req, err := http.NewRequest(http.MethodPost, base+path, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	// PHP 侧 /user/login 只认 application/x-www-form-urlencoded。
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var loginResp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &loginResp); err != nil {
		return fmt.Errorf("登录响应不是 JSON (HTTP %d): %s", resp.StatusCode, truncate(string(body), 200))
	}
	if loginResp.Code != 0 {
		return fmt.Errorf("登录被拒绝: code=%d message=%s", loginResp.Code, loginResp.Message)
	}
	return nil
}

// collect 发一条请求并生成落盘记录。
func collect(jarClient, plainClient *http.Client, base string, m manifest, ep endpoint, manifestDir string, verbose bool) (*record, error) {
	var (
		query      url.Values
		bodyFields []param
		jsonFields []param
		fileFields []param
	)
	query = url.Values{}
	for _, p := range ep.Params {
		switch p.From {
		case "query":
			query.Set(p.Name, paramToString(p.Value))
		case "body":
			bodyFields = append(bodyFields, p)
		case "json":
			jsonFields = append(jsonFields, p)
		case "file":
			fileFields = append(fileFields, p)
		default:
			return nil, fmt.Errorf("未知的参数来源 %q", p.From)
		}
	}

	target := base + ep.Path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	var (
		body        io.Reader
		contentType string
	)
	switch {
	case len(jsonFields) > 0:
		payload := map[string]any{}
		for _, p := range jsonFields {
			payload[p.Name] = p.Value
		}
		buf, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(buf)
		contentType = "application/json"
	case len(bodyFields) > 0 || len(fileFields) > 0 || ep.Method == http.MethodPost:
		// PHP 侧 POST 用 multipart/form-data（$_POST / $_FILES）。
		buf := &bytes.Buffer{}
		w := multipart.NewWriter(buf)
		for _, p := range bodyFields {
			if err := w.WriteField(p.Name, paramToString(p.Value)); err != nil {
				return nil, err
			}
		}
		for _, p := range fileFields {
			path := p.Value.(string)
			if !filepath.IsAbs(path) {
				path = filepath.Join(manifestDir, path)
			}
			f, err := os.Open(path)
			if err != nil {
				return nil, fmt.Errorf("打开上传文件失败: %w", err)
			}
			fw, err := w.CreateFormFile(p.Name, filepath.Base(path))
			if err != nil {
				f.Close()
				return nil, err
			}
			if _, err := io.Copy(fw, f); err != nil {
				f.Close()
				return nil, err
			}
			f.Close()
		}
		if err := w.Close(); err != nil {
			return nil, err
		}
		body = bytes.NewReader(buf.Bytes())
		contentType = w.FormDataContentType()
	}

	req, err := http.NewRequest(ep.Method, target, body)
	if err != nil {
		return nil, err
	}
	if !ep.OmitAccept {
		// PHP 侧只接受精确的 "application/json"（不做 explode(';')）。
		req.Header.Set("Accept", "application/json")
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	switch ep.Auth {
	case "web_user", "admin_web_user", "internal_gateway":
		// 走带 Cookie 的客户端；internal_gateway 不依赖会话。
	case "anonymous":
		// 走不带 Cookie 的客户端。
	case "non_admin_user":
		cookie, err := forgeNonAdminCookie(m)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Cookie", m.Session.CookieName+"="+url.QueryEscape(cookie))
	default:
		return nil, fmt.Errorf("未知的 auth 类型 %q", ep.Auth)
	}

	client := jarClient
	if ep.Auth == "anonymous" || ep.Auth == "non_admin_user" {
		client = plainClient
	}

	if ep.Auth == "internal_gateway" {
		token := m.Gateway.Token
		if ep.BadToken {
			token = "invalid-token-for-contract-baseline"
		}
		req.Header.Set("Token", token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	requestID := resp.Header.Get("Request-ID")
	rec := &record{
		ID:           ep.ID,
		Method:       ep.Method,
		Path:         ep.Path,
		Auth:         ep.Auth,
		Status:       resp.StatusCode,
		ContentType:  resp.Header.Get("Content-Type"),
		RequestID:    placeholderRequestID,
		RequestIDFmt: requestIDFormat(requestID),
		ExpectError:  ep.ExpectError,
		ExpectStatus: ep.ExpectStatus,
		RawBody:      normalizeRaw(string(raw), requestID),
	}

	decoded, code, ok := parseJSONBody(raw)
	if ok {
		rec.Body = normalizeTree(decoded, "")
	}
	rec.ActualCode = code
	rec.Match = matchesExpectation(ep, rec)
	return rec, nil
}

func matchesExpectation(ep endpoint, rec *record) bool {
	if ep.ExpectStatus != nil && *ep.ExpectStatus != rec.Status {
		return false
	}
	if ep.ExpectContentType != "" && !strings.Contains(rec.ContentType, ep.ExpectContentType) {
		return false
	}
	if ep.ExpectError == nil {
		if ep.ExpectStatus != nil && *ep.ExpectStatus != http.StatusOK {
			// 期望的是"非信封"响应（HTML 404 页 / PHP 致命错误 500）：
			// 只校验 HTTP 状态与 Content-Type，body 本来就不是 JSON。
			return true
		}
		// 期望成功：HTTP 200 且 code=0。
		return rec.Status == http.StatusOK && rec.ActualCode != nil && *rec.ActualCode == 0
	}
	return rec.ActualCode != nil && *rec.ActualCode == *ep.ExpectError
}

// parseJSONBody 尝试把响应体解析成 JSON，并取出信封里的 code。
func parseJSONBody(raw []byte) (any, *int, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, nil, false
	}
	if trimmed[0] != '{' && trimmed[0] != '[' {
		return nil, nil, false
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, nil, false
	}
	var code *int
	if obj, ok := v.(map[string]any); ok {
		if n, ok := obj["code"].(json.Number); ok {
			if i, err := strconv.Atoi(n.String()); err == nil {
				code = &i
			}
		}
	}
	return v, code, true
}

// normalizeTree 按 R1~R6 归一化 JSON 树。
func normalizeTree(v any, key string) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			if k == "vm" {
				// R6：admin.systemStatus 的实时指标每次都不同。
				out[k] = placeholderVM
				continue
			}
			if k == "file" && key == "count" {
				// R7：裸仓库的磁盘占用随 push 变化，与实现无关。
				out[k] = placeholderRepoSize
				continue
			}
			out[k] = normalizeTree(val, k)
		}
		return out
	case []any:
		out := make([]any, 0, len(t))
		for _, item := range t {
			out = append(out, normalizeTree(item, ""))
		}
		return out
	case string:
		switch key {
		case "request-id":
			return placeholderRequestID
		case "host":
			return placeholderHost
		case "secret":
			return placeholderMFASecret
		case "image":
			return normalizeImage(t)
		}
		return normalizeBasePath(t)
	case json.Number:
		if i, err := strconv.ParseInt(t.String(), 10, 64); err == nil {
			if i >= tsMin && i <= tsMax {
				return placeholderTS
			}
		}
		return t
	default:
		return v
	}
}

// normalizeImage 保留 data: 前缀，只替换 base64 载荷（R5）。
func normalizeImage(s string) string {
	idx := strings.LastIndex(s, ",")
	if idx < 0 {
		return placeholderMFAImage
	}
	return s[:idx+1] + placeholderMFAImage
}

// normalizeBasePath 把部署内绝对路径前缀换成 <BASE>（R3）。
//
// PHP 的 json_encode 会把 "/" 转义成 "\/"，所以原始响应文本里出现的是
// "\/data\/www\/codefever-community"；两种形态都要替换。
func normalizeBasePath(s string) string {
	s = strings.ReplaceAll(s, basePathPrefix, placeholderBase)
	s = strings.ReplaceAll(s, strings.ReplaceAll(basePathPrefix, "/", `\/`), placeholderBase)
	return s
}

// normalizeRaw 归一化原始响应文本：request-id 与绝对路径（R1/R3），
// 以及 MFA secret / 二维码载荷与 vm 实时指标（R5/R6），最后截断到 8KB。
//
// raw_body 里也套用同样的规则，是为了让基线文件可以逐字节 diff：
// 否则 admin.systemStatus（cpu/内存/磁盘）与 user.getMFAData（随机 secret
// 与二维码）每次采集都会产生差异。
func normalizeRaw(s string, requestID string) string {
	if requestID != "" {
		s = strings.ReplaceAll(s, requestID, placeholderRequestID)
	}
	s = normalizeBasePath(s)
	s = replaceRawJSONValue(s, "secret", `"`+placeholderMFASecret+`"`)
	if start, end, raw, ok := findRawJSONValue(s, "image"); ok {
		unquoted := strings.Trim(raw, `"`)
		if idx := strings.LastIndex(unquoted, ","); idx >= 0 {
			s = s[:start] + `"` + unquoted[:idx+1] + placeholderMFAImage + `"` + s[end:]
		}
	}
	s = replaceRawJSONValue(s, "vm", `"`+placeholderVM+`"`)
	// R7：裸仓库的磁盘占用（count.file）随 push 变化。
	s = replaceRawJSONValue(s, "file", `"`+placeholderRepoSize+`"`)
	if len(s) > maxRawBody {
		s = s[:maxRawBody]
	}
	return s
}

// findRawJSONValue 在原始 JSON 文本里定位 "key" 对应的值区间。
// 返回值的原文（含引号，若为字符串）、起止下标。
func findRawJSONValue(s, key string) (int, int, string, bool) {
	needle := `"` + key + `":`
	idx := strings.Index(s, needle)
	if idx < 0 {
		return 0, 0, "", false
	}
	start := idx + len(needle)
	for start < len(s) && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	if start >= len(s) {
		return 0, 0, "", false
	}
	end := scanJSONValueEnd(s, start)
	if end < 0 {
		return 0, 0, "", false
	}
	return start, end, s[start:end], true
}

// scanJSONValueEnd 返回从 start 开始的 JSON 值结束位置（不含）。
func scanJSONValueEnd(s string, start int) int {
	if start >= len(s) {
		return -1
	}
	switch s[start] {
	case '"':
		i := start + 1
		for i < len(s) {
			if s[i] == '\\' {
				i += 2
				continue
			}
			if s[i] == '"' {
				return i + 1
			}
			i++
		}
		return -1
	case '{', '[':
		depth := 0
		i := start
		for i < len(s) {
			switch s[i] {
			case '"':
				j := scanJSONValueEnd(s, i)
				if j < 0 {
					return -1
				}
				i = j
				continue
			case '{', '[':
				depth++
			case '}', ']':
				depth--
				if depth == 0 {
					return i + 1
				}
			}
			i++
		}
		return -1
	default:
		i := start
		for i < len(s) && s[i] != ',' && s[i] != '}' && s[i] != ']' {
			i++
		}
		return i
	}
}

// replaceRawJSONValue 把原始 JSON 文本里 "key" 的值整体替换成 newLiteral。
func replaceRawJSONValue(s, key, newLiteral string) string {
	start, end, _, ok := findRawJSONValue(s, key)
	if !ok {
		return s
	}
	return s[:start] + newLiteral + s[end:]
}

func paramToString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(b)
	}
}

func writeRecord(outDir string, rec *record) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	// 关掉 HTML 转义，让 <REQUEST_ID> 这类占位符保持可读。
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rec); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, rec.ID+".json"), buf.Bytes(), 0o644)
}

// requestIDFormat 记录 Request-ID 的形状（而不是它的具体值），
// 这样既保留了"大写带连字符 UUID"这一格式契约，又不会让基线文件每次都变。
func requestIDFormat(v string) string {
	if v == "" {
		return "missing"
	}
	parts := strings.Split(v, "-")
	ok := len(parts) == 5
	for i, p := range parts {
		want := map[int]int{0: 8, 1: 4, 2: 4, 3: 4, 4: 12}[i]
		if len(p) != want {
			ok = false
			break
		}
		if p != strings.ToUpper(p) {
			ok = false
			break
		}
		for _, c := range p {
			if !strings.ContainsRune("0123456789ABCDEF", c) {
				ok = false
				break
			}
		}
	}
	if ok {
		return "UUID-8-4-4-4-12-UPPERCASE-HEX"
	}
	return "other:" + v
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// ---------------------------------------------------------------------------
// CodeIgniter 会话 Cookie 伪造
//
// CodeIgniter 3 的 Session 库在本项目里把会话整体放进客户端 Cookie
// （sess_use_database = FALSE），并用 CI_Encrypt 的 XOR 方案加密。
// 因此只要知道 $config['encryption_key']（= env.yaml 的 session.key），
// 就能在客户端构造出任意 u_key 的会话 Cookie —— 全程不写数据库。
//
// 这条能力只用于覆盖"已登录但非管理员访问 /api/admin/*"这一个分支：
// 演示库里唯一的非管理员账号 u_status=4（已软删除），无法走登录流程。
// ---------------------------------------------------------------------------

// forgeNonAdminCookie 生成一个属于非管理员账号的会话 Cookie 值。
func forgeNonAdminCookie(m manifest) (string, error) {
	if m.Session.Key == "" || m.Session.NonAdminUserKey == "" {
		return "", errors.New("manifest 缺少 session.key / session.non_admin_user_key")
	}
	sid, err := randomHex(16)
	if err != nil {
		return "", err
	}
	payload := phpSerializeSession(sid, "", "", time.Now().Unix(), m.Session.NonAdminUserKey)
	return ciEncrypt(payload, m.Session.Key), nil
}

// phpSerializeSession 构造 CI 会话数组的 PHP serialize() 表示。
// 顶层 6 个键：session_id / ip_address / user_agent / last_activity /
// user_data / userInfo。
func phpSerializeSession(sessionID, ip, ua string, lastActivity int64, uKey string) string {
	var b strings.Builder
	b.WriteString("a:6:{")
	b.WriteString(fmt.Sprintf("s:10:\"session_id\";s:%d:\"%s\";", len(sessionID), sessionID))
	b.WriteString(fmt.Sprintf("s:10:\"ip_address\";s:%d:\"%s\";", len(ip), ip))
	b.WriteString(fmt.Sprintf("s:10:\"user_agent\";s:%d:\"%s\";", len(ua), ua))
	b.WriteString(fmt.Sprintf("s:13:\"last_activity\";i:%d;", lastActivity))
	b.WriteString("s:9:\"user_data\";s:0:\"\";")
	b.WriteString(fmt.Sprintf("s:8:\"userInfo\";a:1:{s:5:\"u_key\";s:%d:\"%s\";}}", len(uKey), uKey))
	return b.String()
}

// ciEncrypt 复刻 CI_Encrypt::encode（无 mcrypt 时的 XOR 路径）。
func ciEncrypt(plaintext, encryptionKey string) string {
	key := ciKey(encryptionKey)
	randHex := sha1Hex(randomMaterial())
	enc := make([]byte, 0, len(plaintext)*2)
	for i := 0; i < len(plaintext); i++ {
		c := randHex[i%len(randHex)]
		enc = append(enc, c, c^plaintext[i])
	}
	return base64.StdEncoding.EncodeToString([]byte(ciXorMerge(string(enc), key)))
}

// ciDecrypt 复刻 CI_Encrypt::decode（用于自检）。
func ciDecrypt(cipherB64, encryptionKey string) string {
	key := ciKey(encryptionKey)
	raw, err := base64.StdEncoding.DecodeString(cipherB64)
	if err != nil {
		return ""
	}
	merged := ciXorMerge(string(raw), key)
	out := make([]byte, 0, len(merged)/2)
	for i := 0; i+1 < len(merged); i += 2 {
		out = append(out, merged[i]^merged[i+1])
	}
	return string(out)
}

// ciKey 复刻 CI_Encrypt::get_key —— md5 之后是 32 个十六进制字符。
func ciKey(encryptionKey string) string {
	sum := md5.Sum([]byte(encryptionKey))
	return hex.EncodeToString(sum[:])
}

// ciXorMerge 复刻 CI_Encrypt::_xor_merge。
func ciXorMerge(s, key string) string {
	hash := sha1Hex(key)
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		out[i] = s[i] ^ hash[i%len(hash)]
	}
	return string(out)
}

// sha1Hex 复刻 CI_Encrypt::hash（默认 sha1）。
func sha1Hex(s string) string {
	sum := sha1.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func randomMaterial() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "codefever-contract-baseline"
	}
	return hex.EncodeToString(buf)
}
