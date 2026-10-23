// Command page-compare 逐路径比对 Go 后端与 PHP 后端的页面路由行为。
//
// 用法：
//
//	go run ./cmd/page-compare -php http://localhost:9080 -go http://127.0.0.1:9003
//
// 可选参数：
//
//	-email    登录用的账号（默认 root@codefever.cn）
//	-password 登录用的口令（默认 123456）
//
// 比对维度与验收标准一致：
//   - HTTP 状态码
//   - Location 头
//   - Content-Type
//   - 响应体逐字节
//
// 例外：/captcha/view 的图片字节本身是随机的（PHP 用随机文本 + 随机角度渲染），
// 因此只校验"是合法 PNG 且尺寸为 150x50"。
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"
)

// testCase 是一条待比对的路径。
type testCase struct {
	// Path 是请求路径（含查询串）。
	Path string
	// Method 是 HTTP 方法，默认 GET。
	Method string
	// Form 是 POST 的表单内容；非空时用 application/x-www-form-urlencoded 发送。
	Form string
	// Note 是用例说明。
	Note string
	// Image 为 true 时只校验图片格式与尺寸。
	Image bool
	// LoggedIn 为 true 时带上登录态（先调用登录接口拿到会话 Cookie）。
	LoggedIn bool
}

// cases 是全部比对用例。
//
// 路径与行为逐条对应任务说明里的规格表；POST 用例只取**确定性**的分支
// （校验失败、账号已存在），避免两个实例写同一张表导致结果不可复现。
var cases = []testCase{
	{Path: "/", Note: "默认控制器 home → 302"},
	{Path: "/user", Note: "控制器存在、方法不存在 → 501"},
	{Path: "/user/login", Note: "未登录 → 渲染登录页"},
	{Path: "/user/login", Method: "POST", Note: "POST 空表单 → code 1"},
	{Path: "/user/login", LoggedIn: true, Note: "已登录 → 302 /repositories"},
	{Path: "/user/logout", Note: "登出 → 302 /user/login"},
	{Path: "/user/register", Note: "注册页"},
	{Path: "/user/register", Method: "POST", Note: "POST 空表单 → code 1"},
	{Path: "/user/register", Method: "POST", Form: "email=bad", Note: "POST 邮箱格式错误 → code 2"},
	{Path: "/user/register", Method: "POST", Form: "email=root@codefever.cn", Note: "POST 账号已存在 → code 3"},
	{Path: "/user/verify", Note: "2FA 验证码页"},
	{Path: "/user/verify", Method: "POST", Note: "POST 空表单 → code 100"},
	{Path: "/user/resetPassword", Note: "重置口令页"},
	{Path: "/user/resetPassword", Method: "POST", Note: "POST 空表单 → code 1"},
	{Path: "/user/resetPassword", Method: "POST", Form: "mode=reset", Note: "POST mode=reset → code 100"},
	{Path: "/user/getResetPasswordCode", Note: "方法存在但不是 GET 语义 → code 3"},
	{Path: "/user/getResetPasswordCode", Method: "POST", Note: "POST 空表单 → code 3"},
	{Path: "/user/getResetPasswordCode", Method: "POST", Form: "email=nosuch@example.com", Note: "POST 邮箱不存在 → code 4"},
	{Path: "/user/nosuchmethod", Note: "未知方法 → 501"},
	{Path: "/user/resetpassword", Note: "方法名小写 → 命中 resetPassword"},
	{Path: "/file", Note: "控制器存在、方法不存在 → 501"},
	{Path: "/file/fetch", Note: "无存储名 → No storage"},
	{Path: "/file/fetch/avatar/nosuchfile", Note: "文件不存在 → Not Found"},
	{Path: "/file/fetch/other/key", Note: "存储名不在白名单 → Not Allowed"},
	{Path: "/doc", Note: "→ 302 /doc/cn/"},
	{Path: "/doc/index", Note: "→ 302 /doc/index/"},
	{Path: "/doc/cn", Note: "→ 302 /doc/cn/"},
	{Path: "/doc/en", Note: "→ 302 /doc/en/"},
	{Path: "/doc/xx", Note: "→ 302 /doc/xx/"},
	{Path: "/doc/cn/", Note: "容器内没有 doc/ 目录 → 404"},
	{Path: "/doc/en/", Note: "容器内没有 doc/ 目录 → 404"},
	{Path: "/doc/zh-cn/", Note: "方法不存在 → 501"},
	{Path: "/lang", Note: "控制器存在、方法不存在 → 501"},
	{Path: "/lang/select", Note: "缺少 l 参数 → 错误提示页"},
	{Path: "/lang/select?l=cn", Note: "→ 302 /"},
	{Path: "/lang/select?l=en", Note: "→ 302 /"},
	{Path: "/captcha", Note: "控制器存在、方法不存在 → 501"},
	{Path: "/captcha/view", Image: true, Note: "150x50 PNG"},
	{Path: "/feature", Note: "控制器不存在 → 404 HTML"},
	{Path: "/community", Note: "控制器不存在 → 404 HTML"},
	{Path: "/boss", Note: "控制器不存在 → 404 HTML"},
	{Path: "/service", Note: "控制器不存在 → 404 HTML"},
	{Path: "/pricing", Note: "控制器不存在 → 404 HTML"},
}

// instance 是一个待比对的后端实例。
//
// 每个实例有两个客户端：一个不带 Cookie（匿名），一个带登录态。
// 二者都不跟随重定向 —— 比对的是**原始**响应（状态码 + Location + 响应体），
// 跟随重定向会把 302 变成落地页，无法比对 Location 头。
type instance struct {
	name      string
	base      string
	anon      *http.Client
	authed    *http.Client
	authedJar *cookiejar.Jar
}

// response 是一次请求的响应摘要。
type response struct {
	status      int
	location    string
	contentType string
	body        []byte
}

// summary 返回用于表格展示的简短描述。
func (r *response) summary() string {
	s := fmt.Sprintf("%d", r.status)
	if r.location != "" {
		s += " → " + r.location
	}
	if r.contentType != "" {
		s += " | " + r.contentType
	}
	return s
}

// client 返回该用例应当使用的客户端。
func (in *instance) client(tc testCase) *http.Client {
	if tc.LoggedIn {
		return in.authed
	}
	return in.anon
}

// do 发起一次请求。
func (in *instance) do(tc testCase) (*response, error) {
	method := tc.Method
	if method == "" {
		method = http.MethodGet
	}
	u := in.base + tc.Path

	var body io.Reader
	if tc.Form != "" {
		body = strings.NewReader(tc.Form)
	}
	req, err := http.NewRequest(method, u, body)
	if err != nil {
		return nil, err
	}
	if tc.Form != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := in.client(tc).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &response{
		status:      resp.StatusCode,
		location:    resp.Header.Get("Location"),
		contentType: resp.Header.Get("Content-Type"),
		body:        content,
	}, nil
}

// login 调用登录接口，把会话 Cookie 存进 CookieJar。
func (in *instance) login(email, password string) error {
	form := url.Values{"email": {email}, "password": {password}}
	req, err := http.NewRequest(http.MethodPost, in.base+"/user/login", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := in.authed.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var env struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(content, &env); err != nil {
		return fmt.Errorf("解析登录响应失败: %w", err)
	}
	if env.Code != 0 {
		return fmt.Errorf("登录失败: code=%d message=%s", env.Code, env.Message)
	}
	return nil
}

// checkImage 校验响应体是合法 PNG 且尺寸符合预期。
func checkImage(body []byte, wantW, wantH int) error {
	img, err := png.Decode(bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("不是合法 PNG: %w", err)
	}
	b := img.Bounds()
	if b.Dx() != wantW || b.Dy() != wantH {
		return fmt.Errorf("尺寸 = %dx%d, 期望 %dx%d", b.Dx(), b.Dy(), wantW, wantH)
	}
	return nil
}

// firstDiff 返回两段字节第一个不同的位置及上下文。
func firstDiff(a, b []byte) string {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	start := i - 40
	if start < 0 {
		start = 0
	}
	endA := i + 60
	if endA > len(a) {
		endA = len(a)
	}
	endB := i + 60
	if endB > len(b) {
		endB = len(b)
	}
	return fmt.Sprintf("首个差异在第 %d 字节\n      php: %q\n      go : %q", i, a[start:endA], b[start:endB])
}

func main() {
	phpBase := flag.String("php", "http://localhost:9080", "PHP 实例地址")
	goBase := flag.String("go", "http://127.0.0.1:9003", "Go 实例地址")
	email := flag.String("email", "root@codefever.cn", "登录用的账号")
	password := flag.String("password", "123456", "登录用的口令")
	flag.Parse()

	newInstance := func(name, base string) *instance {
		// 不跟随重定向：要比对的是原始响应（状态码 + Location + 响应体）。
		noRedirect := func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
		anonJar, _ := cookiejar.New(nil)
		authedJar, _ := cookiejar.New(nil)
		return &instance{
			name:      name,
			base:      strings.TrimRight(base, "/"),
			anon:      &http.Client{Jar: anonJar, Timeout: 30 * time.Second, CheckRedirect: noRedirect},
			authed:    &http.Client{Jar: authedJar, Timeout: 30 * time.Second, CheckRedirect: noRedirect},
			authedJar: authedJar,
		}
	}
	php := newInstance("php", *phpBase)
	goInst := newInstance("go", *goBase)

	// 两个实例分别登录（会话互不相通）。
	phpLogged := php.login(*email, *password) == nil
	goLogged := goInst.login(*email, *password) == nil
	if !phpLogged || !goLogged {
		fmt.Printf("提示：登录态不可用（php=%v go=%v），带登录态的用例将被跳过\n", phpLogged, goLogged)
	}

	fmt.Printf("PHP: %s\nGo : %s\n\n", php.base, goInst.base)
	fmt.Printf("%-4s %-40s %-34s %-34s %s\n", "", "路径", "PHP", "Go", "一致")
	fmt.Println(strings.Repeat("-", 150))

	failed := 0
	for _, tc := range cases {
		if tc.LoggedIn && (!phpLogged || !goLogged) {
			fmt.Printf("%-4d %-40s %-34s %-34s %s\n", 0, tc.Path, "-", "-", "跳过（无登录态）")
			continue
		}

		phpResp, err := php.do(tc)
		if err != nil {
			fmt.Printf("%-4d %-40s %s\n", 0, tc.Path, "PHP 请求失败: "+err.Error())
			failed++
			continue
		}
		goResp, err := goInst.do(tc)
		if err != nil {
			fmt.Printf("%-4d %-40s %s\n", 0, tc.Path, "Go 请求失败: "+err.Error())
			failed++
			continue
		}

		problems := []string{}
		if phpResp.status != goResp.status {
			problems = append(problems, fmt.Sprintf("状态码 %d ≠ %d", phpResp.status, goResp.status))
		}
		if phpResp.location != goResp.location {
			problems = append(problems, fmt.Sprintf("Location %q ≠ %q", phpResp.location, goResp.location))
		}
		if phpResp.contentType != goResp.contentType {
			problems = append(problems, fmt.Sprintf("Content-Type %q ≠ %q", phpResp.contentType, goResp.contentType))
		}
		if tc.Image {
			// 图片字节随机，只校验格式与尺寸。
			if err := checkImage(phpResp.body, 150, 50); err != nil {
				problems = append(problems, "PHP 图片: "+err.Error())
			}
			if err := checkImage(goResp.body, 150, 50); err != nil {
				problems = append(problems, "Go 图片: "+err.Error())
			}
		} else if !bytes.Equal(phpResp.body, goResp.body) {
			problems = append(problems, "响应体不一致: "+firstDiff(phpResp.body, goResp.body))
		}

		mark := "是"
		if len(problems) > 0 {
			mark = "否"
			failed++
		}
		fmt.Printf("%-4d %-40s %-34s %-34s %s\n", 0, tc.Path, phpResp.summary(), goResp.summary(), mark)
		for _, p := range problems {
			fmt.Printf("     ↳ %s\n", p)
		}
	}

	fmt.Println()
	if failed == 0 {
		fmt.Printf("全部 %d 条路径一致。\n", len(cases))
		return
	}
	fmt.Printf("有 %d 条路径不一致（共 %d 条）。\n", failed, len(cases))
	os.Exit(1)
}
