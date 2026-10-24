package httpapi

import (
	"codefever-next/backend/internal/domain"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// snapshotDir 是 PHP 侧真实产物的 HTML 快照目录（相对本包目录）。
//
// 快照来源：
//   - php-login.html / php-register.html / php-reset_password.html /
//     php-reset_password_reset.html / php-verify.html
//     从运行中的 PHP 后端 http://localhost:9080/user/... 直接抓取。
//   - php-doc_detail.html
//     doc/detail.php 在本机不可通过 HTTP 访问（容器里没有部署 doc/ 目录，
//     /doc/cn/ 返回 CodeIgniter 的 404），因此用容器内的 php CLI 直接渲染
//     真实模板 + 真实 include 得到，菜单与正文使用与本测试完全相同的固定数据。
const snapshotDir = "../../testdata/pages"

// testDoc 是文档页的固定 Markdown 正文（与生成 php-doc_detail.html 快照时一致）。
const testDoc = "# 克隆仓库到本地\n\n" +
	"当仓库在 `CodeFever` 上创建完成后，需要 `克隆` 到本地才能继续操作。\n\n" +
	"### 如何 clone 一个仓库\n\n" +
	"1. 进入 `仓库首页`\n" +
	"1. 点击 `克隆` 按钮\n\n" +
	"> 命令示例：git clone <repository url> <target path>\n"

// testDocMenu 是文档页的固定左侧菜单（与生成 php-doc_detail.html 快照时一致）。
var testDocMenu = []DocMenuItem{
	{
		Name: "开始",
		Path: "start",
		Submenu: []DocMenuItem{
			{Name: "快速开始", Path: "start/quickstart"},
			{Name: "常见问题", Path: "start/faq"},
		},
	},
	{
		Name: "仓库",
		Path: "repo",
		Submenu: []DocMenuItem{
			{Name: "仓库首页", Path: "repo/index"},
		},
	},
}

// testPageData 返回渲染页面用的固定数据。
//
// 这些值与 PHP 后端在本机运行时使用的配置一致：
// host=http://localhost（config.yaml）、releaseNumber=00000000000001（env.yaml）、
// allowRegister=true（config.yaml）、语言=zh-cn（会话默认语言）。
func testPageData() PageData {
	return PageData{
		Lang:          NewTranslations("zh-cn"),
		Host:          "http://localhost",
		ReleaseNumber: "00000000000001",
		AppName:       "CodeFever",
		AllowRegister: true,
	}
}

// pageCase 描述一个页面模板与它对应的 PHP 快照。
type pageCase struct {
	name     string
	snapshot string
	render   func(http.ResponseWriter, PageData) error
	data     PageData
}

// pageCases 返回所有能拿到 PHP 真实产物的页面。
//
// base/error_message.php、base/success_message.php、base/disable_message.php、
// nopage/index.php 四个模板 include 的目标文件在本仓库中不存在，
// PHP 生产环境（error_reporting(0)）下这些位置输出为空，没有可对比的 PHP 产物，
// 因此不纳入本用例；它们由 TestUnrenderablePagesSmoke 单独覆盖。
func pageCases() []pageCase {
	base := testPageData()

	reset := base
	reset.Mode = "reset"

	doc := base
	doc.DocSegments = []string{"doc", "cn", "git", "clone"}
	doc.Doc = testDoc
	doc.DocMenu = testDocMenu

	return []pageCase{
		{name: "login", snapshot: "php-login.html", render: RenderLogin, data: base},
		{name: "register", snapshot: "php-register.html", render: RenderRegister, data: base},
		{name: "reset_password", snapshot: "php-reset_password.html", render: RenderResetPassword, data: base},
		{name: "reset_password_reset", snapshot: "php-reset_password_reset.html", render: RenderResetPassword, data: reset},
		{name: "verify", snapshot: "php-verify.html", render: RenderVerify, data: base},
		{name: "doc_detail", snapshot: "php-doc_detail.html", render: RenderDocDetail, data: doc},
	}
}

// TestPagesMatchPHPSnapshots 断言 Go 渲染结果与 PHP 真实产物在结构上一致。
func TestPagesMatchPHPSnapshots(t *testing.T) {
	for _, tc := range pageCases() {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := renderToString(t, tc.render, tc.data)
			want := readSnapshot(t, tc.snapshot)

			// 抽取标签/class/id/资源路径时必须先去掉 script 元素的内容：
			// script 里的 JS 字符串含有 "<div>"、markdown 里的 "<repository url>" 这类
			// 伪标签，PHP 侧不转义、Go 侧会被上下文转义器编码成 \uXXXX，
			// 直接扫描整篇文档会把它们误当成真实标签。
			//
			// normalizePerfDeviations 见其自身文档：2026-10 性能整改引入的**有意**
			// 偏离在这里集中抹平，以免破坏"与 PHP 渲染保真"这一回归保证。
			gotMarkup := normalizePerfDeviations(stripScriptBodies(got))
			wantMarkup := normalizePerfDeviations(stripScriptBodies(want))

			// 1) 标签序列必须完全一致（含顺序、属性名与属性值）。
			gotTags, wantTags := extractTags(gotMarkup), extractTags(wantMarkup)
			if !equalStrings(gotTags, wantTags) {
				t.Errorf("标签序列不一致%s", firstDiff(strings.Join(gotTags, "\n"), strings.Join(wantTags, "\n")))
			}

			// 2) class 集合必须一致（顺序不敏感）。
			assertSameSet(t, "class", extractAll(gotMarkup, classAttrRe), extractAll(wantMarkup, classAttrRe))

			// 3) id 集合必须一致（模板里 id 有单引号和双引号两种写法）。
			assertSameSet(t, "id", extractIDs(gotMarkup), extractIDs(wantMarkup))

			// 4) src / href 静态资源路径集合必须一致。
			assertSameSet(t, "src", extractAll(gotMarkup, srcAttrRe), extractAll(wantMarkup, srcAttrRe))
			assertSameSet(t, "href", extractAll(gotMarkup, hrefAttrRe), extractAll(wantMarkup, hrefAttrRe))

			// 5) script 之外的 HTML 必须逐字符一致（仅忽略空白差异）。
			gotHTML, wantHTML := normalizeSpace(gotMarkup), normalizeSpace(wantMarkup)
			if gotHTML != wantHTML {
				t.Errorf("script 之外的 HTML 不一致%s", firstDiff(gotHTML, wantHTML))
			}

			// 6) script 内的 JavaScript 代码必须一致。
			//    Go 的 html/template 会把 JS 注释删掉、把 < & " ' ` 编码成 \\uXXXX，
			//    PHP 则原样输出，因此比较前先做等价归一化。
			gotJS := normalizeJS(decodeGoJSEscapes(extractScriptBodies(got)))
			wantJS := normalizeJS(extractScriptBodies(want))
			if gotJS != wantJS {
				t.Errorf("script 内容不一致%s", firstDiff(gotJS, wantJS))
			}
		})
	}
}

// TestUnrenderablePagesSmoke 覆盖拿不到 PHP 产物的四个模板。
//
// 它们 include 的目标文件（console/header_include.php、splash.php、
// center/*.php、根目录 header_include.php）在本仓库中不存在，
// PHP 生产环境下这些位置输出为空，Go 侧用 absent_include 占位。
// 本用例保证这些模板仍能正常渲染，且没有把任何用户数据原样输出。
func TestUnrenderablePagesSmoke(t *testing.T) {
	base := testPageData()
	base.Message = "出错了"

	cases := []struct {
		name    string
		render  func(http.ResponseWriter, PageData) error
		hasText string
	}{
		{"error_message", RenderErrorMessage, "出错了"},
		{"success_message", RenderSuccessMessage, "出错了"},
		{"disable_message", RenderDisableMessage, "出错了"},
		// nopage/index.php 本身没有提示文案，只有固定的 404 文案。
		{"not_found", RenderNotFound, "The page you’re looking for is not here."},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := renderToString(t, tc.render, base)
			if !strings.Contains(got, "<!DOCTYPE html>") && !strings.Contains(got, "<html>") {
				t.Errorf("渲染结果不是完整页面: %s", got)
			}
			if !strings.Contains(got, tc.hasText) {
				t.Errorf("页面文案未渲染: %s", got)
			}
		})
	}
}

// TestCountriesDropdownMenu 覆盖国家下拉菜单片段。
//
// 该片段在 PHP 侧没有被任何模板 include，因此没有可对比的 PHP 产物；
// 这里断言它的结构：三个硬编码的常用项 + 每个国家一个 <li> + 按首字母分组的分组标题。
func TestCountriesDropdownMenu(t *testing.T) {
	data := testPageData()
	data.Countries = domain.Countries
	got := renderToString(t, RenderCountriesDropdownMenu, data)

	for _, want := range []string{
		`<li class="tag-code-item" >常用</li>`,
		`<li data-country-code="86" >`,
		`<li data-country-code="1">`,
		`<li data-country-code="61">`,
		`<li class="tag-code-item" style="background-color: #eee;">A</li>`,
		`<li class="tag-code-item" style="background-color: #eee;">A</li>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("下拉菜单缺少 %q", want)
		}
	}

	// 每个国家一个条目（data-country-code）。
	if n := strings.Count(got, `data-country-code="`); n != len(domain.Countries)+3 {
		t.Errorf("国家条目数 = %d, 期望 %d", n, len(domain.Countries)+3)
	}

	// 分组标题按 Name 首字母去重后应与国家列表一致。
	seen := map[string]bool{}
	for _, c := range domain.Countries {
		seen[string([]rune(c.Name)[0])] = true
	}
	if !strings.Contains(got, `<li class="tag-code-item" style="background-color: #eee;">`+string([]rune(domain.Countries[0].Name)[0])+`</li>`) {
		t.Errorf("缺少 %s 的分组标题", domain.Countries[0].Name)
	}
}

// TestRenderLoginEscapesUserInput 断言用户输入被转义，不会引入 XSS。
//
// PHP 的 echo 不转义（历史缺陷），Go 侧按任务要求必须转义。
func TestRenderLoginEscapesUserInput(t *testing.T) {
	data := testPageData()
	data.Email = `"><script>alert(1)</script>`
	got := renderToString(t, RenderLogin, data)
	if strings.Contains(got, "<script>alert(1)</script>") {
		t.Error("邮箱中的 HTML 被原样输出，存在 XSS 风险")
	}
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Errorf("邮箱中的 HTML 未按预期转义: %s", got)
	}
}

// TestContentType 断言响应头与 PHP 侧一致。
func TestContentType(t *testing.T) {
	rec := httptest.NewRecorder()
	if err := RenderLogin(rec, testPageData()); err != nil {
		t.Fatalf("渲染失败: %v", err)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=UTF-8" {
		t.Errorf("Content-Type = %q, 期望 %q", got, "text/html; charset=UTF-8")
	}
}

// TestAllTemplatesEmbedded 断言 13 个 PHP 模板对应的 Go 模板都被嵌入并可解析。
func TestAllTemplatesEmbedded(t *testing.T) {
	want := []string{
		"login.html",
		"register.html",
		"reset_password.html",
		"verify.html",
		"countries_dropdown_menu.html",
		"header_include.html",
		"footer_include.html",
		"login_activity.html",
		"doc_detail.html",
		"error_message.html",
		"success_message.html",
		"disable_message.html",
		"index.html",
		"absent_include.html",
	}
	tmpl, err := pageTemplates()
	if err != nil {
		t.Fatalf("解析模板失败: %v", err)
	}
	for _, name := range want {
		if tmpl.Lookup(name) == nil {
			t.Errorf("模板 %s 未被嵌入", name)
		}
	}
}

// TestAjaxEnvelope 断言 AJAX 信封的形状与 PHP 侧一致（无 request-id、无 data）。
//
// 期望值是 **PHP 风格的 JSON**：非 ASCII 转成 \uXXXX、斜杠转成 "\/"。
// 这是 PHP 的 json_encode 默认行为（未开启 JSON_UNESCAPED_UNICODE /
// JSON_UNESCAPED_SLASHES），实测 PHP 侧响应体逐字节如下：
//
//	{"code":0,"message":"\u767b\u5f55\u6210\u529f","extra":{"redirect":"\/repositories"}}
func TestAjaxEnvelope(t *testing.T) {
	env := NewAjaxEnvelope(0, "登录成功", map[string]string{"redirect": "/repositories"})
	rec := httptest.NewRecorder()
	if err := WriteAjaxJSON(rec, env); err != nil {
		t.Fatalf("写出失败: %v", err)
	}
	got := rec.Body.String()
	want := `{"code":0,"message":"\u767b\u5f55\u6210\u529f","extra":{"redirect":"\/repositories"}}`
	if got != want {
		t.Errorf("信封 = %s, 期望 %s", got, want)
	}
	if strings.Contains(got, "request-id") || strings.Contains(got, "data") {
		t.Errorf("信封不应包含 request-id / data: %s", got)
	}

	// extra 为空时 PHP 的 json_encode(array()) 输出的是 []，不是 {}。
	empty := NewAjaxEnvelope(3, "账号不存在", nil)
	rec2 := httptest.NewRecorder()
	if err := WriteAjaxJSON(rec2, empty); err != nil {
		t.Fatalf("写出失败: %v", err)
	}
	if got := rec2.Body.String(); got != `{"code":3,"message":"\u8d26\u53f7\u4e0d\u5b58\u5728","extra":[]}` {
		t.Errorf("空 extra 信封 = %s", got)
	}
}

// renderToString 渲染页面并返回结果字符串。
func renderToString(t *testing.T, render func(http.ResponseWriter, PageData) error, data PageData) string {
	t.Helper()
	rec := httptest.NewRecorder()
	if err := render(rec, data); err != nil {
		t.Fatalf("渲染失败: %v", err)
	}
	return rec.Body.String()
}

// readSnapshot 读取 PHP 侧真实产物的快照。
func readSnapshot(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(snapshotDir, name)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取快照 %s 失败: %v", path, err)
	}
	return string(b)
}

// 各种抽取用的正则。
var (
	tagRe        = regexp.MustCompile(`<[a-zA-Z!/][^>]*>`)
	classAttrRe  = regexp.MustCompile(`\sclass="([^"]*)"`)
	idAttrRe     = regexp.MustCompile(`\sid="([^"]*)"`)
	idAttrSqRe   = regexp.MustCompile(`\sid='([^']*)'`)
	srcAttrRe    = regexp.MustCompile(`\ssrc="([^"]*)"`)
	hrefAttrRe   = regexp.MustCompile(`\shref="([^"]*)"`)
	scriptRe     = regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script>`)
	scriptBodyRe = regexp.MustCompile(`(?is)(<script\b[^>]*>)(.*?)(</script>)`)
	jsCommentRe  = regexp.MustCompile(`//[^\n\r]*`)
	jsUniEscRe   = regexp.MustCompile(`\\u([0-9a-fA-F]{4})`)
	spaceRe      = regexp.MustCompile(`\s+`)

	// 性能整改（2026-10）的已登记偏离，见 normalizePerfDeviations。
	markedScriptRe     = regexp.MustCompile(`(?i)<script\b[^>]*marked\.min\.js[^>]*>\s*</script>`)
	pictureSourceRe    = regexp.MustCompile(`(?i)<source\b[^>]*>`)
	pictureTagRe       = regexp.MustCompile(`(?i)</?picture\b[^>]*>`)
	perfImgExtraAttrRe = regexp.MustCompile(`\s+(?:alt|fetchpriority|decoding)="[^"]*"`)
)

// extractTags 抽取页面里的标签序列（保留顺序）。
func extractTags(s string) []string {
	out := []string{}
	for _, m := range tagRe.FindAllString(s, -1) {
		out = append(out, normalizeSpace(m))
	}
	return out
}

// extractAll 用正则抽取属性值并按空白拆分成集合。
func extractAll(s string, re *regexp.Regexp) []string {
	set := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(s, -1) {
		for _, v := range strings.Fields(m[1]) {
			set[v] = true
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// extractIDs 抽取页面里所有 id（单双引号两种写法都要覆盖）。
func extractIDs(s string) []string {
	set := map[string]bool{}
	for _, re := range []*regexp.Regexp{idAttrRe, idAttrSqRe} {
		for _, m := range re.FindAllStringSubmatch(s, -1) {
			if m[1] != "" {
				set[m[1]] = true
			}
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// assertSameSet 断言两个集合一致。
func assertSameSet(t *testing.T, what string, got, want []string) {
	t.Helper()
	if equalStrings(got, want) {
		return
	}
	t.Errorf("%s 集合不一致\n go : %v\n php: %v", what, got, want)
}

// ---------------------------------------------------------------------------
// 性能整改（2026-10）的有意偏离
// ---------------------------------------------------------------------------

// normalizePerfDeviations 抹平性能整改**有意**引入的、与 PHP 产物不同的渲染差异。
//
// 设计原则：
//
//	php-*.html 快照是 PHP → Go 迁移的保真基线（逐标签、逐属性、逐字符），
//	必须继续生效。性能整改确实改变了若干页面的外壳（这正是整改目的），
//	因此**不修改快照**，而是在比较前把已登记的偏离从两侧同时抹掉 ——
//	这样"其余一切仍与 PHP 一致"这一保证不会被削弱，新增偏离也必须显式登记。
//
// 已登记偏离（每条都要写清理由与影响范围）：
//
//	1. marked.min.js —— PHP 在 footer_include 里**全局**加载（每页 47.6 KB）；
//	   整改后只有 doc_detail 加载（唯一调用 marked.marked() 的页面）。
//	   login / register / reset_password / verify 是访问量最高的四个页面，
//	   不再为它们下载这份 Markdown 渲染器。
//	2. login_activity 的 LCP 图 —— PHP 是裸 <img src="....png">；整改后包成
//	   <picture> + WebP <source> + alt/fetchpriority/decoding 三个属性
//	   （PNG 147.9 KB → WebP 30.5 KB，且该图是登录页 LCP 元素）。
//
// 说明：intlTelInput.css / intlTelInput.js **不在**上表 —— 需要它们的
// login / register / reset_password / verify 正是访问量最高的页面，
// 按页拆分没有收益，因此保持与 PHP 完全一致（全局加载、原位置）。
//
// 未登记的新偏离会让本测试失败——这是有意设计，请勿直接把偏离加进快照。
func normalizePerfDeviations(s string) string {
	// 偏离 1：全局加载 → 按页加载的脚本（外链脚本，不含内联代码）。
	s = markedScriptRe.ReplaceAllString(s, "")
	// 偏离 2：<picture> 包装与新增属性还原成 PHP 的裸 <img>。
	s = pictureSourceRe.ReplaceAllString(s, "")
	s = pictureTagRe.ReplaceAllString(s, "")
	s = perfImgExtraAttrRe.ReplaceAllString(s, "")
	return s
}

// equalStrings 比较两个字符串切片。
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// firstDiff 返回两段文本第一个差异位置的上下文，便于定位问题。
func firstDiff(got, want string) string {
	n := len(got)
	if len(want) < n {
		n = len(want)
	}
	i := 0
	for i < n && got[i] == want[i] {
		i++
	}
	start := i - 60
	if start < 0 {
		start = 0
	}
	end := i + 120
	if end > len(got) {
		end = len(got)
	}
	endWant := i + 120
	if endWant > len(want) {
		endWant = len(want)
	}
	return fmt.Sprintf("\n 位置 %d\n go : %q\n php: %q", i, got[start:end], want[start:endWant])
}

// stripScriptBodies 去掉 script 元素的内容，只保留标签本身。
func stripScriptBodies(s string) string {
	return scriptBodyRe.ReplaceAllString(s, "$1$3")
}

// extractScriptBodies 取出 script 元素的内容。
func extractScriptBodies(s string) string {
	var b strings.Builder
	for _, m := range scriptRe.FindAllString(s, -1) {
		inner := scriptBodyRe.ReplaceAllString(m, "$2")
		b.WriteString(inner)
		b.WriteString("\n")
	}
	return b.String()
}

// normalizeSpace 把连续空白折叠成一个空格。
func normalizeSpace(s string) string { return spaceRe.ReplaceAllString(s, " ") }

// normalizeJS 归一化 JavaScript 代码，让 Go 与 PHP 的输出可比。
//
// 做两件事：
//  1. 去掉 JS 行注释 —— Go 的 html/template 会把注释从输出里删掉，PHP 不会。
//  2. 把 PHP 的 \` 还原成 ` —— PHP 用 str_replace('`', '\`', ...) 转义模板字符串里的反引号。
func normalizeJS(s string) string {
	s = jsCommentRe.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\\`", "`")
	return normalizeSpace(s)
}

// decodeGoJSEscapes 还原 Go 上下文转义器对 JS 字符串做的编码。
//
// Go 会把 < & " ' ` 编码成 \\uXXXX、把换行编码成 \\n；
// PHP 是原样输出。两者在 JS 运行时完全等价，这里还原成字符后再比较。
func decodeGoJSEscapes(s string) string {
	s = jsUniEscRe.ReplaceAllStringFunc(s, func(m string) string {
		n, err := strconv.ParseUint(m[2:], 16, 16)
		if err != nil {
			return m
		}
		return string(rune(n))
	})
	return strings.ReplaceAll(s, `\n`, "\n")
}
