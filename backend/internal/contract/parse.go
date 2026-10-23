package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// AcceptMode 表示 Accept 头解析结果。
type AcceptMode string

const (
	// AcceptJSON 对应 PHP 侧的 'json'，输出 JSON 信封。
	AcceptJSON AcceptMode = "json"
	// AcceptBinary 对应 PHP 侧的 'binary'，直接输出原始内容（无信封）。
	AcceptBinary AcceptMode = "binary"
)

// ContentMode 表示 Content-Type 头解析结果，对应 PHP 侧 Request::parse() 的 contentType。
type ContentMode string

const (
	// ContentJSON 表示 application/json，参数来自 JSON body。
	ContentJSON ContentMode = "json"
	// ContentForm 表示 multipart/form-data，参数来自 $_POST，文件来自 $_FILES。
	ContentForm ContentMode = "form"
	// ContentFormText 表示 application/x-www-form-urlencoded，参数来自 $_POST。
	ContentFormText ContentMode = "form-text"
	// ContentGet 表示没有 Content-Type，参数只能来自 query。
	ContentGet ContentMode = "get"
)

// UploadedFile 对应 PHP $_FILES 中的单个文件项。
type UploadedFile struct {
	Field    string
	Filename string
	MIMEType string
	Size     int64

	header *multipart.FileHeader
}

// Open 打开上传文件内容。
//
// multipart.FileHeader.Open() 对小文件返回内存 reader、对大文件返回磁盘临时文件，
// 两种情况都可重复调用。临时文件由 multipart.Form.RemoveAll() 统一清理，
// 因此调用方必须在请求结束后触发它（见 httpapi 中间件）。
func (f *UploadedFile) Open() (io.ReadCloser, error) {
	if f == nil || f.header == nil {
		return nil, os.ErrNotExist
	}
	return f.header.Open()
}

// ReadAll 读取上传文件的全部内容。
func (f *UploadedFile) ReadAll() ([]byte, error) {
	fh, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	return io.ReadAll(fh)
}

// Parsed 是一次请求的解析结果，语义与 PHP 侧 Request::parse() 对齐。
type Parsed struct {
	ID          string
	Token       string
	URI         string
	Method      string
	Accept      AcceptMode
	ContentMode ContentMode

	// Query 是 $_GET 语义的参数（已应用 PHP 的分页归一化）。
	Query map[string]any
	// RawQuery 是未经归一化的原始 query 参数。
	RawQuery url.Values

	// Body 是 $_POST 语义的参数（multipart 与 urlencoded 均落在此处）。
	Body map[string]any
	// JSON 是 application/json body 解析结果。
	JSON map[string]any

	// Files 是 $_FILES 语义的��传文件。
	Files map[string][]*UploadedFile
}

// Parse 解析并校验请求。
//
// 校验顺序与 PHP 侧 Request::parse() 完全一致（这一点影响前端可见行为）：
//
//  1. HTTP 方法必须是 GET/POST/PUT/DELETE，否则 0x0102
//  2. Accept 必须是 application/json 或 */*，且 Content-Type 必须可识别，否则 0x0103
//  3. Content-Length 必须与 body 长度一致（multipart 豁免），否则 0x0104
//
// 注意第 2 步先于鉴权：实测缺 Accept 时未登录请求返回 0x0103 而非 0x0101。
//
// 返回的 *Parsed 在出错时仍然非 nil —— 它的 ID 字段已经生成，
// 错误响应需要带上同一个 request-id（与 PHP 侧行为一致）。
func Parse(r *http.Request) (*Parsed, error) {
	p := &Parsed{
		ID:       NewRequestID(),
		Token:    r.Header.Get("Token"),
		URI:      r.RequestURI,
		Method:   r.Method,
		Files:    map[string][]*UploadedFile{},
		RawQuery: r.URL.Query(),
		Query:    map[string]any{},
		Body:     map[string]any{},
		JSON:     map[string]any{},
	}

	// 1) 方法白名单。
	switch r.Method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete:
	default:
		return p, NewError(CodeMethodNotAllowed)
	}

	// 2) Accept —— 注意 PHP 侧不做 explode(';')，因此 "application/json; charset=utf-8" 会被判为非法。
	switch r.Header.Get("Accept") {
	case "application/json":
		p.Accept = AcceptJSON
	case "*/*":
		p.Accept = AcceptBinary
	default:
		return p, NewError(CodeHeaderRequired)
	}

	// 3) Content-Type —— PHP 侧按 ';' 截断后再匹配。
	rawCT := r.Header.Get("Content-Type")
	ct := ""
	if rawCT != "" {
		ct = strings.TrimSpace(strings.SplitN(rawCT, ";", 2)[0])
	}
	switch ct {
	case "":
		p.ContentMode = ContentGet
	case "application/json":
		p.ContentMode = ContentJSON
	case "multipart/form-data":
		p.ContentMode = ContentForm
	case "application/x-www-form-urlencoded":
		p.ContentMode = ContentFormText
	default:
		return p, NewError(CodeHeaderRequired)
	}

	// 4) 读取 body 并按 Content-Type 分派参数来源。
	//
	// 关键：先把 body 读进缓冲区，再**把请求流复位**。
	// 否则 multipart 分支调用 r.ParseMultipartForm 时流已空，
	// 所有表单字段都会被静默丢弃（表现为"参数永远缺失"）。
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return p, NewError(CodeContentLength)
	}
	defer r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(body))

	switch p.ContentMode {
	case ContentJSON:
		p.JSON = decodeJSONObject(body)
		if len(body) > 0 && p.JSON == nil {
			// PHP 侧 json_decode 失败返回 null，后续 $data['xxx'] 取值会触发 TypeError。
			// 这里保持等价：视为空对象，由各 handler 自行判定必填参数。
			p.JSON = map[string]any{}
		}
	case ContentForm:
		if err := r.ParseMultipartForm(32 << 20); err == nil {
			p.Body = phpArrayFromValues(r.MultipartForm.Value)
			p.Files = collectFiles(r)
		} else {
			p.Body = map[string]any{}
		}
	case ContentFormText:
		values, perr := url.ParseQuery(string(body))
		if perr != nil {
			values = url.Values{}
		}
		p.Body = phpArrayFromValues(values)
	case ContentGet:
		// 无 body。
	}

	// 5) Content-Length 与实际长度的一致性校验（multipart 豁免，与 PHP 一致）。
	if p.ContentMode != ContentForm {
		declared := r.Header.Get("Content-Length")
		if declared == "" {
			// PHP: NULL != strlen($content) 在 strlen>0 时成立 → 拒绝。
			if len(body) > 0 {
				return p, NewError(CodeContentLength)
			}
		} else if n, cerr := strconv.Atoi(declared); cerr != nil || n != len(body) {
			return p, NewError(CodeContentLength)
		}
	}

	// 6) 分页归一化 —— 复刻 PHP 的 is_int($_GET[...]) 判定。
	p.Query = phpArrayFromValues(r.URL.Query())
	normalizePaging(p.Query)

	return p, nil
}

// normalizePaging 复刻 PHP 侧 Request::parse() 的分页归一化。
//
// PHP 用 is_int($_GET['page']) 判定，而 query 参数值恒为字符串，
// 因此走 query 的分页端点实际永远被改写为 page=1 / pagesize=20。
// 这是现有实现的缺陷，但属于"当前可观察行为"，重写阶段先原样复刻，
// 修正与否由契约快照的比对结果决定（见 docs/BACKEND_REWRITE_PLAN.md §5.4）。
func normalizePaging(query map[string]any) {
	if v, ok := query["page"]; !ok || !isPHPInt(v) || toInt(v) < 0 {
		query["page"] = 1
	}
	if v, ok := query["pagesize"]; !ok || !isPHPInt(v) || toInt(v) > DefaultPageSizeMax || toInt(v) < 0 {
		query["pagesize"] = DefaultPageSize
	}
}

// DefaultPageSize 与 DefaultPageSizeMax 来自 application/config/constants.php。
const (
	DefaultPageSize    = 20
	DefaultPageSizeMax = 100
)

// QueryString 从 query 参数读取字符串。
func (p *Parsed) QueryString(key string) string {
	return anyToString(p.Query[key])
}

// QueryInt 从 query 参数读取整数。
func (p *Parsed) QueryInt(key string) int { return toInt(p.Query[key]) }

// BodyString 从 body 参数读取字符串。
func (p *Parsed) BodyString(key string) string {
	return anyToString(p.Body[key])
}

// BodyInt 从 body 参数读取整数。
func (p *Parsed) BodyInt(key string) int { return toInt(p.Body[key]) }

// JSONString 从 JSON body 读取字符串。
func (p *Parsed) JSONString(key string) string {
	return anyToString(p.JSON[key])
}

// JSONInt 从 JSON body 读取整数。
func (p *Parsed) JSONInt(key string) int { return toInt(p.JSON[key]) }

// ParamString 按 PHP 的习惯依次从 body、JSON、query 中查找参数。
//
// 前端所有写操作使用 multipart，读操作使用 query，
// 而内部网关回调使用 JSON body，因此 handler 需要一个统一的取值入口。
func (p *Parsed) ParamString(key string) string {
	if v, ok := p.Body[key]; ok {
		return anyToString(v)
	}
	if v, ok := p.JSON[key]; ok {
		return anyToString(v)
	}
	return p.QueryString(key)
}

// ParamInt 按 ParamString 的顺序读取整数。
func (p *Parsed) ParamInt(key string) int { return toInt(p.ParamString(key)) }

// HasParam 判断参数是否存在（区分"缺失"与"空字符串"）。
func (p *Parsed) HasParam(key string) bool {
	if _, ok := p.Body[key]; ok {
		return true
	}
	if _, ok := p.JSON[key]; ok {
		return true
	}
	_, ok := p.Query[key]
	return ok
}

// File 返回指定字段的首个上传文件。
func (p *Parsed) File(field string) *UploadedFile {
	list := p.Files[field]
	if len(list) == 0 {
		return nil
	}
	return list[0]
}

// decodeJSONObject 解析 JSON body 为对象。
//
// 非对象（数组、标量、非法 JSON）一律返回 nil，与 PHP json_decode($x, true) 的行为一致。
func decodeJSONObject(body []byte) map[string]any {
	if len(body) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return nil
	}
	return m
}

// collectFiles 从 multipart 表单中收集文件项。
func collectFiles(r *http.Request) map[string][]*UploadedFile {
	out := map[string][]*UploadedFile{}
	if r.MultipartForm == nil {
		return out
	}
	for field, headers := range r.MultipartForm.File {
		for _, h := range headers {
			if h == nil {
				continue
			}
			mimeType := h.Header.Get("Content-Type")
			if mimeType == "" {
				mimeType = mime.TypeByExtension(filepath.Ext(h.Filename))
			}
			out[field] = append(out[field], &UploadedFile{
				Field:    field,
				Filename: h.Filename,
				MIMEType: mimeType,
				Size:     h.Size,
				header:   h,
			})
		}
	}
	return out
}

// phpArrayFromValues 把扁平的表单/查询参数还原成 PHP 的数组结构。
//
// 支持：
//
//	a          → "v"            （多值时保留最后一个，与 PHP 一致）
//	a[]        → []any{"v1","v2"}
//	a[b]       → map{"b":"v"}
//	a[b][c]    → map{"b":map{"c":"v"}}
func phpArrayFromValues(values url.Values) map[string]any {
	root := map[string]*phpNode{}
	for key, vals := range values {
		if len(vals) == 0 {
			continue
		}
		segs := parseBracketKey(key)
		insertPHPValue(root, segs, vals)
	}
	out := map[string]any{}
	for k, n := range root {
		out[k] = n.toAny()
	}
	return out
}

// parseBracketKey 把 "a[b][]" 解析为 ["a","b",""]。
func parseBracketKey(key string) []string {
	if !strings.Contains(key, "[") {
		return []string{key}
	}
	i := strings.Index(key, "[")
	segs := []string{key[:i]}
	rest := key[i:]
	for len(rest) > 0 && rest[0] == '[' {
		j := strings.Index(rest, "]")
		if j < 0 {
			break
		}
		segs = append(segs, rest[1:j])
		rest = rest[j+1:]
	}
	return segs
}

// phpNode 是构造 PHP 数组结构的中间节点。
type phpNode struct {
	kind int // 0=标量 1=列表 2=映射
	val  string
	list []*phpNode
	m    map[string]*phpNode
}

const (
	phpLeaf = iota
	phpList
	phpMap
)

func newMapNode() *phpNode { return &phpNode{kind: phpMap, m: map[string]*phpNode{}} }

func (n *phpNode) toAny() any {
	switch n.kind {
	case phpList:
		out := make([]any, 0, len(n.list))
		for _, item := range n.list {
			out = append(out, item.toAny())
		}
		return out
	case phpMap:
		out := map[string]any{}
		for k, v := range n.m {
			out[k] = v.toAny()
		}
		return out
	default:
		return n.val
	}
}

// insertPHPValue 把一组值插入到树中。
func insertPHPValue(root map[string]*phpNode, segs []string, vals []string) {
	if len(segs) == 0 {
		return
	}
	head := segs[0]
	cur, ok := root[head]
	if !ok {
		cur = newMapNode()
		root[head] = cur
	}
	if len(segs) == 1 {
		// 标量：PHP 对重复的普通键保留最后一个值。
		cur.kind = phpLeaf
		cur.val = vals[len(vals)-1]
		return
	}
	if segs[1] == "" {
		// 列表追加。
		if cur.kind != phpList {
			cur = &phpNode{kind: phpList}
			root[head] = cur
		}
		for _, v := range vals {
			cur.list = append(cur.list, &phpNode{kind: phpLeaf, val: v})
		}
		return
	}
	if cur.kind != phpMap {
		cur = newMapNode()
		root[head] = cur
	}
	child, ok := cur.m[segs[1]]
	if !ok {
		child = newMapNode()
		cur.m[segs[1]] = child
	}
	// 递归处理剩余层级，复用同一套插入逻辑。
	sub := map[string]*phpNode{segs[1]: child}
	insertPHPValue(sub, segs[1:], vals)
}

// anyToString 把任意标量转成字符串（PHP 的 (string) 转换）。
func anyToString(v any) string {
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
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case []any:
		if len(t) == 0 {
			return ""
		}
		return anyToString(t[0])
	case map[string]any:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

// toInt 把任意标量转成整数（PHP 的 (int) 转换）。
func toInt(v any) int {
	switch t := v.(type) {
	case nil:
		return 0
	case int:
		return t
	case int64:
		return int(t)
	case float64:
		return int(t)
	case bool:
		if t {
			return 1
		}
		return 0
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(t))
		if err != nil {
			f, ferr := strconv.ParseFloat(strings.TrimSpace(t), 64)
			if ferr != nil {
				return 0
			}
			return int(f)
		}
		return n
	default:
		return 0
	}
}

// isPHPInt 判断一个值是否是"整数类型"（PHP 的 is_int）。
//
// 只有真实的整数类型才算 true —— 这正是 PHP 分页归一化总是失效的原因：
// $_GET 的值永远是字符串。
func isPHPInt(v any) bool {
	switch v.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	default:
		return false
	}
}
