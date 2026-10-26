package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"codefever-next/backend/internal/config"
	"codefever-next/backend/internal/httpapi"
)

// DocHandler 实现 doc 控制器。
//
// 逐字对应 application/controllers/doc.php。
//
// 文档正文来自应用根目录下的 doc/<语言> 目录（PHP 侧是
// realpath(dirname(APPPATH).'/doc/'.$docPath)）。容器里**没有** doc/ 目录时，
// PHP 会 show_404()，新后端逐字复刻（CI 的 error_404.php 页面）。
type DocHandler struct {
	Cfg *config.Config
}

// NewDocHandler 创建 doc 控制器。
func NewDocHandler(cfg *config.Config) *DocHandler { return &DocHandler{Cfg: cfg} }

// RegisterWeb 把 doc 控制器注册到服务器。
//
// Before 对应 PHP 侧 Doc::__construct() —— 它在方法分发之前执行，
// 因此 "/doc/xxx"（xxx 不是方法名）也会先被重定向到 "/doc/xxx/"，
// 而不会落到 501 "Unknown method"。
func (h *DocHandler) RegisterWeb(s *httpapi.Server) {
	s.RegisterWebController(&httpapi.WebController{
		Name: "doc",
		Methods: map[string]http.HandlerFunc{
			"index": h.index,
			"cn":    h.cn,
			"en":    h.en,
		},
		Before: h.docRedirect,
	})
}

// docPathHasExt 判定路径是否带文件扩展名。
//
// 逐字对应 application/controllers/doc.php:12 的
// preg_match('/\.[a-z0-9]+$/i', $path)。
var docPathHasExt = regexp.MustCompile(`(?i)\.[a-z0-9]+$`)

// docAssetName 判定文件名是否是文档配图。
//
// 逐字对应 application/controllers/doc.php:57、165 的
// preg_match('/^\w{32}\.png$/', $fileName)。
var docAssetName = regexp.MustCompile(`^\w{32}\.png$`)

// docMenuRe 解析文档目录里的 Markdown 链接行。
//
// 逐字对应 application/controllers/doc.php:154 的
// preg_match('/\[([^\]]+)\]\(([^\)]+)\)/', $line, $matches)。
var docMenuRe = regexp.MustCompile(`\[([^\]]+)\]\(([^\)]+)\)`)

// index 实现 doc/index。
//
// 逐字对应 application/controllers/doc.php:24-27 —— 什么都不做。
//
// 构造函数里的重定向逻辑（doc.php:9-22）由 Before 钩子负责。
func (h *DocHandler) index(w http.ResponseWriter, r *http.Request) {
	// index() 本身没有输出，但 PHP 仍然会带上默认的 Content-Type，
	// 这里显式补上（Go 在空响应体上不会自动推断 Content-Type）。
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.Header().Set("Content-Length", "0")
	w.WriteHeader(http.StatusOK)
}

// cn 实现 doc/cn（简体中文文档）。
func (h *DocHandler) cn(w http.ResponseWriter, r *http.Request) {
	h.detail(w, r, "zh-cn")
}

// en 实现 doc/en（英文文档）。
func (h *DocHandler) en(w http.ResponseWriter, r *http.Request) {
	h.detail(w, r, "en-us")
}

// docRedirect 复刻 PHP 侧 Doc::__construct() 的重定向逻辑。
//
// 逐字对应 application/controllers/doc.php:9-22：
// PATH_INFO 不带文件扩展名时：
//   - '/doc' 与 '/doc/' → 302 /doc/cn/（PHP **不** exit，方法继续执行；
//     由于 index() 没有输出，最终响应与这里直接 302 等价）；
//   - 其它不以 '/' 结尾的路径 → 302 "<path>/"（PHP 随后 exit）。
//
// 返回 true 表示响应已输出，调用方应直接返回。
func (h *DocHandler) docRedirect(w http.ResponseWriter, r *http.Request) bool {
	path := r.URL.Path
	if docPathHasExt.MatchString(path) {
		return false
	}
	if path == "/doc" || path == "/doc/" {
		redirect(w, "/doc/cn/")
		return true
	}
	if !strings.HasSuffix(path, "/") {
		redirect(w, path+"/")
		return true
	}
	return false
}

// detail 实现文档详情页。
//
// 逐字对应 application/controllers/doc.php:39-119（_detail）。
func (h *DocHandler) detail(w http.ResponseWriter, r *http.Request, docLang string) {
	// PHP 用 $this->uri->uri_string（去掉首斜杠的原始路径）分段。
	uriString := strings.TrimPrefix(r.URL.Path, "/")
	segments := strings.Split(uriString, "/")
	fileName := segments[len(segments)-1]

	docRoot := h.docRoot(docLang)
	if docRoot == "" {
		// 逐字对应 doc.php:45-48：文档根目录不存在 → show_404()。
		h.show404(w, r)
		return
	}
	rootDir := docRoot + string(os.PathSeparator)

	// 逐字对应 doc.php:50-51：$docDir = array_slice($segments, 2)。
	docDir := ""
	if len(segments) > 2 {
		docDir = strings.Join(segments[2:], "/")
	}
	docFile, ok := resolveDocPath(docRoot, rootDir+docDir, true)
	if !ok {
		// 逐字对应 doc.php:52-55。
		h.show404(w, r)
		return
	}

	// 逐字对应 doc.php:57-60：文档配图直接输出二进制。
	if docAssetName.MatchString(fileName) {
		h.serveAsset(w, r, docRoot, docFile)
		return
	}

	// 逐字对应 doc.php:62-65：非 .md 的文件一律 404。
	if info, err := os.Stat(docFile); err == nil && !info.IsDir() {
		if strings.ToLower(filepath.Ext(docFile)) != ".md" {
			h.show404(w, r)
			return
		}
	}

	menu := h.extractMenu(docRoot, rootDir+"index.md")
	menu = h.withSubmenu(docRoot, rootDir, menu)

	// 逐字对应 doc.php:85-112：目录按 index.md 里的第一条菜单逐级下钻。
	if info, err := os.Stat(docFile); err == nil && info.IsDir() {
		menuOptions := h.extractMenu(docRoot, docFile+"/index.md")
		pathInfo := r.URL.Path
		for len(menu) > 0 && len(menuOptions) > 0 {
			first := menuOptions[0].Path
			if strings.Index(first, ".") > 0 {
				pathInfo = strings.TrimRight(pathInfo, "/") + "/" + first
				target, ok := resolveDocPath(docRoot, docFile+"/"+first, false)
				if !ok || strings.ToLower(filepath.Ext(target)) != ".md" {
					h.show404(w, r)
					return
				}
				redirect(w, pathInfo)
				return
			}
			pathInfo = strings.TrimRight(pathInfo, "/") + "/" + first
			next, ok := resolveDocPath(docRoot, docFile+"/"+first, true)
			if !ok {
				h.show404(w, r)
				return
			}
			docFile = next
			menuOptions = h.extractMenu(docRoot, docFile+"/index.md")
		}
		h.show404(w, r)
		return
	}

	// 逐字对应 doc.php:114-118。
	content, err := os.ReadFile(docFile)
	if err != nil {
		h.show404(w, r)
		return
	}
	// 前端切换后：文档正文由 SPA 通过 /api/doc/content 拉取，服务端只下发壳。
	_ = content
	_ = segments
	_ = menu
	if httpapi.ServeSPA(w, r) {
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusInternalServerError)
}

// docRoot 返回文档根目录的绝对路径；目录不存在时返回空串。
//
// 逐字对应 application/controllers/doc.php:44：
// realpath(dirname(APPPATH).'/doc/'.$docPath)。
func (h *DocHandler) docRoot(docLang string) string {
	base := filepath.Join(h.Cfg.BaseDir, "doc", docLang)
	resolved, err := filepath.EvalSymlinks(base)
	if err != nil {
		return ""
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return ""
	}
	return resolved
}

// extractMenu 解析目录里的 index.md，抽出菜单项。
//
// 逐字对应 application/controllers/doc.php:143-160（_extractMenu）。
func (h *DocHandler) extractMenu(docRoot, menuFilePath string) []httpapi.DocMenuItem {
	resolved, ok := resolveDocPath(docRoot, menuFilePath, false)
	if !ok || strings.ToLower(filepath.Ext(resolved)) != ".md" {
		return nil
	}
	content, err := os.ReadFile(resolved)
	if err != nil {
		return nil
	}
	out := []httpapi.DocMenuItem{}
	for _, line := range strings.Split(string(content), "\n") {
		m := docMenuRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		out = append(out, httpapi.DocMenuItem{Name: m[1], Path: strings.TrimSpace(m[2])})
	}
	return out
}

// withSubmenu 给一级菜单补上二级菜单。
//
// 逐字对应 application/controllers/doc.php:67-83。
func (h *DocHandler) withSubmenu(docRoot, rootDir string, menu []httpapi.DocMenuItem) []httpapi.DocMenuItem {
	for i := range menu {
		submenuFile := rootDir + menu[i].Path + "/index.md"
		if _, err := os.Stat(submenuFile); err != nil {
			continue
		}
		submenu := h.extractMenu(docRoot, submenuFile)
		for j := range submenu {
			submenu[j].Path = menu[i].Path + "/" + submenu[j].Path
		}
		menu[i].Submenu = submenu
	}
	return menu
}

// serveAsset 输出文档配图。
//
// 逐字对应 application/controllers/doc.php:162-179（_assets）。
func (h *DocHandler) serveAsset(w http.ResponseWriter, r *http.Request, docRoot, path string) {
	resolved, ok := resolveDocPath(docRoot, path, false)
	if !ok || !docAssetName.MatchString(filepath.Base(resolved)) {
		h.show404(w, r)
		return
	}
	content, err := os.ReadFile(resolved)
	if err != nil {
		h.show404(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(len(content)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

// show404 输出 CI 的 404 页面。
//
// 逐字对应 CodeIgniter 的 Exceptions::show_404()：渲染
// application/errors/error_404.php，HTTP 状态码 404。
//
// 注意这与"控制器不存在"时的 404 页（nopage/index.php）是**两个不同的页面**。
func (h *DocHandler) show404(w http.ResponseWriter, r *http.Request) {
	if httpapi.ServeSPAStatus(w, r, http.StatusNotFound) {
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusNotFound)
}

// resolveDocPath 解析文档路径并防止目录穿越。
//
// 逐字对应 application/controllers/doc.php:122-141（_resolveDocPath）：
// 解析符号链接后必须仍在 docRoot 之内，且必须可读；
// allowDirectory 为 true 时目录也算命中。
func resolveDocPath(docRoot, path string, allowDirectory bool) (string, bool) {
	if docRoot == "" || strings.ContainsRune(path, 0) {
		return "", false
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		resolved, err = filepath.Abs(path)
		if err != nil {
			return "", false
		}
	}
	prefix := docRoot + string(os.PathSeparator)
	if resolved != docRoot && !strings.HasPrefix(resolved, prefix) {
		return "", false
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", false
	}
	if info.IsDir() {
		if allowDirectory {
			return resolved, true
		}
		return "", false
	}
	return resolved, true
}

// ---------------------------------------------------------------------------
// 文档中心的 JSON 端点（供新前端 React 文档页使用）
// ---------------------------------------------------------------------------
//
// 旧前端（SSR）把 Markdown 原文塞进模板的 `<script>var doc = ...` 里，由浏览器用
// marked 渲染。React 版需要结构化数据，因此新增两个只读端点：
//
//	GET /api/doc/tree?lang=cn|en              → 菜单树（含二级）
//	GET /api/doc/content?lang=cn|en&path=…    → 单篇 Markdown 原文
//
// 两者都**复用**既有的 docRoot / extractMenu / withSubmenu / resolveDocPath，
// 因此目录解析与路径穿越校验与 SSR 页面完全同源（安全边界不打折）。
// 它们通过 `RegisterPage` 注册为字面量路由（与 /api/web-vitals 同一手法），
// 从而绕过 /api/{ctrl}/{action} 分发器的鉴权与 Content-Type 校验 —— 文档是公开内容。

// docAPIItem 是菜单项的 JSON 形态（字段名小写，前端直接消费）。
type docAPIItem struct {
	Name    string       `json:"name"`
	Path    string       `json:"path"`
	Submenu []docAPIItem `json:"submenu,omitempty"`
}

// docAPILang 把前端语言标识（cn/en）映射为文档目录名（zh-cn/en-us）。
//
// 与旧 URL 语义一致：`/doc/cn/…` 对应 zh-cn，`/doc/en/…` 对应 en-us。
func docAPILang(raw string) string {
	if strings.EqualFold(raw, "en") || strings.EqualFold(raw, "en-us") {
		return "en-us"
	}
	return "zh-cn"
}

// toDocAPIItems 把内部菜单（含 Submenu）转成 JSON 形态。
func toDocAPIItems(menu []httpapi.DocMenuItem) []docAPIItem {
	out := make([]docAPIItem, 0, len(menu))
	for i := range menu {
		item := docAPIItem{Name: menu[i].Name, Path: menu[i].Path}
		if len(menu[i].Submenu) > 0 {
			item.Submenu = toDocAPIItems(menu[i].Submenu)
		}
		out = append(out, item)
	}
	return out
}

// writeDocJSON 输出与 /api/* 同形的信封（`request-id` 留空：文档接口不进访问链路统计）。
func writeDocJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(status)
	if data == nil {
		data = []any{}
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"request-id": "",
		"code":       map[bool]int{true: 0, false: 1}[status == http.StatusOK],
		"message":    map[bool]string{true: "Success", false: "Not Found"}[status == http.StatusOK],
		"data":       data,
	})
}

// RegisterAPI 注册文档中心的 JSON 端点。
func (h *DocHandler) RegisterAPI(s *httpapi.Server) {
	s.RegisterPage("/api/doc/tree", h.apiTree)
	s.RegisterPage("/api/doc/content", h.apiContent)
}

// apiTree 返回文档菜单树。
func (h *DocHandler) apiTree(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeDocJSON(w, http.StatusNotFound, nil)
		return
	}
	docRoot := h.docRoot(docAPILang(r.URL.Query().Get("lang")))
	if docRoot == "" {
		writeDocJSON(w, http.StatusNotFound, nil)
		return
	}
	rootDir := docRoot + string(os.PathSeparator)
	menu := h.withSubmenu(docRoot, rootDir, h.extractMenu(docRoot, rootDir+"index.md"))
	writeDocJSON(w, http.StatusOK, map[string]any{"menu": toDocAPIItems(menu)})
}

// apiContent 返回单篇文档的 Markdown 原文。
//
// path 是相对文档根目录的路径（例如 `quickstart/install.md`），
// 由 resolveDocPath 做符号链接与穿越校验，非 .md 一律 404。
func (h *DocHandler) apiContent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeDocJSON(w, http.StatusNotFound, nil)
		return
	}
	docRoot := h.docRoot(docAPILang(r.URL.Query().Get("lang")))
	if docRoot == "" {
		writeDocJSON(w, http.StatusNotFound, nil)
		return
	}
	rel := strings.TrimPrefix(strings.TrimSpace(r.URL.Query().Get("path")), "/")
	if rel == "" {
		writeDocJSON(w, http.StatusNotFound, nil)
		return
	}
	resolved, ok := resolveDocPath(docRoot, docRoot+string(os.PathSeparator)+rel, false)
	if !ok || strings.ToLower(filepath.Ext(resolved)) != ".md" {
		writeDocJSON(w, http.StatusNotFound, nil)
		return
	}
	content, err := os.ReadFile(resolved)
	if err != nil {
		writeDocJSON(w, http.StatusNotFound, nil)
		return
	}
	writeDocJSON(w, http.StatusOK, map[string]any{"path": rel, "content": string(content)})
}
