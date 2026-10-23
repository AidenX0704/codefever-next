package domain

import (
	"bytes"
	"image/png"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// config.yaml 文档
// ---------------------------------------------------------------------------

// siteTemplate 是 config.template.yaml 的内容（逐字搬运）。
const siteTemplate = `---
host: http://localhost
ssh: 'git@localhost:'
allowRegister: true
email:
  name: CodeFever Next
  from: git@mail.sender
...
`

// TestParseSiteDocumentMatchesTemplate 校验 config.yaml 能被解析成映射。
func TestParseSiteDocumentMatchesTemplate(t *testing.T) {
	doc, err := ParseSiteDocument([]byte(siteTemplate))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if got := doc["host"]; got != "http://localhost" {
		t.Errorf("host = %v, want http://localhost", got)
	}
	if got := doc["ssh"]; got != "git@localhost:" {
		t.Errorf("ssh = %v, want git@localhost:", got)
	}
	if got := doc["allowRegister"]; got != true {
		t.Errorf("allowRegister = %v, want true", got)
	}
	email, ok := doc["email"].(map[string]any)
	if !ok {
		t.Fatalf("email 不是映射: %T", doc["email"])
	}
	if email["name"] != "CodeFever Next" || email["from"] != "git@mail.sender" {
		t.Errorf("email = %v, want {name: CodeFever Next, from: git@mail.sender}", email)
	}
}

// TestParseSiteDocumentRejectsNonMap 校验非映射文档被拒绝 ——
// 这正是 PHP 侧 P0 缺陷的入口（config.yaml 被写成 "--- ~" 后
// yaml_parse_file 返回 null）。
func TestParseSiteDocumentRejectsNonMap(t *testing.T) {
	cases := map[string]string{
		"空文档":     "",
		"null 文档": "---\n~\n...\n",
		"标量文档":    "---\nhello\n...\n",
		"列表文档":    "---\n- a\n- b\n...\n",
		"只有注释":    "# nothing\n",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseSiteDocument([]byte(content)); err == nil {
				t.Fatalf("期望被拒绝，实际解析成功")
			}
		})
	}
}

// TestParseSiteDocumentRejectsBrokenYAML 校验非法 YAML 被拒绝。
func TestParseSiteDocumentRejectsBrokenYAML(t *testing.T) {
	if _, err := ParseSiteDocument([]byte("host: [unclosed\n")); err == nil {
		t.Fatalf("期望解析失败，实际成功")
	}
}

// TestMergeSiteDocumentFollowsPHPSemantics 校验合并语义与 PHP 的
// array_merge($config, $input) 一致：载荷里的键覆盖现有键，
// 载荷里缺的键沿用现值（浅合并）。
func TestMergeSiteDocumentFollowsPHPSemantics(t *testing.T) {
	base, err := ParseSiteDocument([]byte(siteTemplate))
	if err != nil {
		t.Fatalf("解析基础文档失败: %v", err)
	}
	patch := map[string]any{"host": "https://git.example.com"}

	merged := MergeSiteDocument(base, patch)
	if merged["host"] != "https://git.example.com" {
		t.Errorf("host = %v, want https://git.example.com", merged["host"])
	}
	if merged["ssh"] != "git@localhost:" {
		t.Errorf("ssh = %v, want git@localhost:", merged["ssh"])
	}
	if merged["allowRegister"] != true {
		t.Errorf("allowRegister = %v, want true", merged["allowRegister"])
	}
	// 浅合并：email 被整体替换。
	patch = map[string]any{"email": map[string]any{"name": "X", "from": "y@z"}}
	merged = MergeSiteDocument(base, patch)
	email := merged["email"].(map[string]any)
	if email["name"] != "X" || email["from"] != "y@z" {
		t.Errorf("email = %v, want {name: X, from: y@z}", email)
	}
	// 基础文档不受影响。
	if base["host"] != "http://localhost" {
		t.Errorf("基础文档被污染: host = %v", base["host"])
	}
}

// TestMarshalSiteDocumentKeepsTemplateShape 校验写回文件的形状与
// config.template.yaml 一致（键顺序固定）。
func TestMarshalSiteDocumentKeepsTemplateShape(t *testing.T) {
	doc, err := ParseSiteDocument([]byte(siteTemplate))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	doc["host"] = "https://git.example.com"

	out, err := MarshalSiteDocument(doc)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	text := string(out)
	for _, key := range []string{"host:", "ssh:", "allowRegister:", "email:", "name:", "from:"} {
		if !strings.Contains(text, key) {
			t.Errorf("输出缺少键 %q:\n%s", key, text)
		}
	}
	// 键顺序必须与模板一致。
	wantOrder := []string{"host:", "ssh:", "allowRegister:", "email:"}
	last := -1
	for _, key := range wantOrder {
		idx := strings.Index(text, key)
		if idx <= last {
			t.Errorf("键顺序错误: %q 出现在位置 %d，上一个键在 %d\n%s", key, idx, last, text)
		}
		last = idx
	}

	// 往返：序列化结果必须能再次解析成等价的映射。
	again, err := ParseSiteDocument(out)
	if err != nil {
		t.Fatalf("往返解析失败: %v\n%s", err, text)
	}
	if again["host"] != "https://git.example.com" {
		t.Errorf("往返后 host = %v", again["host"])
	}
}

// TestValidateSiteDocument 校验结构校验会拒绝缺字段 / 类型错误的文档。
func TestValidateSiteDocument(t *testing.T) {
	valid := map[string]any{
		"host":          "http://localhost",
		"ssh":           "git@localhost:",
		"allowRegister": true,
		"email":         map[string]any{"name": "CodeFever Next", "from": "git@mail.sender"},
	}
	if err := ValidateSiteDocument(valid); err != nil {
		t.Fatalf("合法文档被拒绝: %v", err)
	}

	cases := map[string]map[string]any{
		"缺 host": {
			"ssh": "git@localhost:", "allowRegister": true,
			"email": map[string]any{"name": "n", "from": "f"},
		},
		"host 为空": {
			"host": "  ", "ssh": "git@localhost:", "allowRegister": true,
			"email": map[string]any{"name": "n", "from": "f"},
		},
		"ssh 类型错误": {
			"host": "h", "ssh": 1, "allowRegister": true,
			"email": map[string]any{"name": "n", "from": "f"},
		},
		"allowRegister 类型错误": {
			"host": "h", "ssh": "s", "allowRegister": "yes",
			"email": map[string]any{"name": "n", "from": "f"},
		},
		"email 不是映射": {
			"host": "h", "ssh": "s", "allowRegister": true, "email": "x@y",
		},
		"email 缺 from": {
			"host": "h", "ssh": "s", "allowRegister": true,
			"email": map[string]any{"name": "n"},
		},
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			if err := ValidateSiteDocument(doc); err == nil {
				t.Fatalf("期望校验失败，实际通过")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 分页 / 排序 / 过滤
// ---------------------------------------------------------------------------

// TestAdminPaging 校验分页归一化与 PHP 的 `$page ? $page : 1` 一致。
func TestAdminPaging(t *testing.T) {
	page, size := AdminPaging(0, 0)
	if page != 1 || size != 20 {
		t.Errorf("AdminPaging(0,0) = (%d,%d), want (1,20)", page, size)
	}
	page, size = AdminPaging(3, 50)
	if page != 3 || size != 50 {
		t.Errorf("AdminPaging(3,50) = (%d,%d), want (3,50)", page, size)
	}
}

// TestAdminPagingOffset 校验偏移量计算。
func TestAdminPagingOffset(t *testing.T) {
	if got := AdminPagingOffset(1, 20); got != 0 {
		t.Errorf("偏移量 = %d, want 0", got)
	}
	if got := AdminPagingOffset(3, 20); got != 40 {
		t.Errorf("偏移量 = %d, want 40", got)
	}
	// 负偏移量收敛到 0（PHP 会因此报错）。
	if got := AdminPagingOffset(-5, 20); got != 0 {
		t.Errorf("负偏移量 = %d, want 0", got)
	}
}

// TestAdminSortColumns 校验排序白名单。
func TestAdminSortColumns(t *testing.T) {
	if got := AdminUserSortColumn("name"); got != "u_name" {
		t.Errorf("用户排序 = %q, want u_name", got)
	}
	if got := AdminUserSortColumn("email"); got != "u_email" {
		t.Errorf("用户排序 = %q, want u_email", got)
	}
	if got := AdminUserSortColumn("created"); got != "u_created" {
		t.Errorf("用户排序 = %q, want u_created", got)
	}
	if got := AdminUserSortColumn("u_name"); got != "" {
		t.Errorf("非法排序 = %q, want 空串", got)
	}
	if got := AdminGroupSortColumn("name"); got != "g_name" {
		t.Errorf("组排序 = %q, want g_name", got)
	}
	if got := AdminGroupSortColumn("email"); got != "" {
		t.Errorf("组排序 = %q, want 空串", got)
	}
	if got := AdminRepositorySortColumn("created"); got != "r_created" {
		t.Errorf("仓库排序 = %q, want r_created", got)
	}
	if got := AdminRepositorySortColumn("bogus"); got != "" {
		t.Errorf("仓库排序 = %q, want 空串", got)
	}
}

// TestAdminUserCategoryWhere 校验用户分类过滤条件。
func TestAdminUserCategoryWhere(t *testing.T) {
	where, args := AdminUserCategoryWhere(AdminUserCategoryAll)
	if where != "" || args != nil {
		t.Errorf("全部用户不应加条件: %q %v", where, args)
	}
	where, _ = AdminUserCategoryWhere(AdminUserCategoryMFAEnabled)
	if where != "u_2fa != ?" {
		t.Errorf("MFAEnabled 条件 = %q", where)
	}
	where, _ = AdminUserCategoryWhere(AdminUserCategoryMFADisabled)
	if where != "u_2fa IS NULL" {
		t.Errorf("MFADisabled 条件 = %q", where)
	}
	where, args = AdminUserCategoryWhere(AdminUserCategoryStatusDelete)
	if where != "u_status = ?" || len(args) != 1 || args[0] != 4 {
		t.Errorf("StatusDelete 条件 = %q %v", where, args)
	}
}

// TestAdminUserKeywordWhere 校验关键词过滤。
func TestAdminUserKeywordWhere(t *testing.T) {
	where, args := AdminUserKeywordWhere("")
	if where != "" {
		t.Errorf("空关键词不应加条件: %q", where)
	}
	where, args = AdminUserKeywordWhere("test")
	if where != "(u_name LIKE ? OR u_email LIKE ?)" || len(args) != 2 {
		t.Errorf("关键词条件 = %q %v", where, args)
	}
	if args[0] != "%test%" {
		t.Errorf("关键词参数 = %v, want %%test%%", args[0])
	}
}

// ---------------------------------------------------------------------------
// 邮箱与口令
// ---------------------------------------------------------------------------

// TestAdminEmailValid 校验邮箱格式判定与 PHP 正则一致。
func TestAdminEmailValid(t *testing.T) {
	valid := []string{
		"test@pgyer.com",
		"test2@pgyer.com",
		"a.b-c_d@example.co.uk",
		"root@codefever.cn",
	}
	for _, email := range valid {
		if !AdminEmailValid(email) {
			t.Errorf("%q 应当合法", email)
		}
	}
	invalid := []string{
		"",
		"test",
		"test@",
		"@pgyer.com",
		"TEST@pgyer.com", // PHP 正则区分大小写
		"test@pgyer.com ",
	}
	for _, email := range invalid {
		if AdminEmailValid(email) {
			t.Errorf("%q 应当非法", email)
		}
	}
}

// TestAdminInitialPassword 校验初始口令是 32 位小写十六进制。
func TestAdminInitialPassword(t *testing.T) {
	password := AdminInitialPassword()
	if len(password) != 32 {
		t.Fatalf("口令长度 = %d, want 32", len(password))
	}
	for _, r := range password {
		if !strings.ContainsRune("0123456789abcdef", r) {
			t.Fatalf("口令含非法字符 %q", r)
		}
	}
	if password == AdminInitialPassword() {
		t.Errorf("两次生成的口令相同")
	}
}

// ---------------------------------------------------------------------------
// 头像
// ---------------------------------------------------------------------------

// TestAdminAvatarColorIsDeterministic 校验底色由哈希确定且可复现。
func TestAdminAvatarColorIsDeterministic(t *testing.T) {
	r1, g1, b1 := AdminAvatarColor("test@pgyer.com")
	r2, g2, b2 := AdminAvatarColor("test@pgyer.com")
	if r1 != r2 || g1 != g2 || b1 != b2 {
		t.Errorf("同一种子的底色不一致: (%d,%d,%d) vs (%d,%d,%d)", r1, g1, b1, r2, g2, b2)
	}
	// 必须落在 Material Design 调色板内。
	found := false
	for _, c := range adminMaterialColors {
		if c[0] == r1 && c[1] == g1 && c[2] == b1 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("底色 (%d,%d,%d) 不在调色板内", r1, g1, b1)
	}
}

// TestAdminAvatarLetter 校验首字母取法。
func TestAdminAvatarLetter(t *testing.T) {
	if got := AdminAvatarLetter("test@pgyer.com"); got != "T" {
		t.Errorf("首字母 = %q, want T", got)
	}
	if got := AdminAvatarLetter("9lives@example.com"); got != "9" {
		t.Errorf("首字母 = %q, want 9", got)
	}
	if got := AdminAvatarLetter("中文@example.com"); got != "#" {
		t.Errorf("首字母 = %q, want #", got)
	}
}

// TestAdminAvatarPNG 校验生成的是合法 PNG 且尺寸为 256×256。
func TestAdminAvatarPNG(t *testing.T) {
	payload, err := AdminAvatarPNG("test@pgyer.com")
	if err != nil {
		t.Fatalf("生成头像失败: %v", err)
	}
	if !bytes.HasPrefix(payload, []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatalf("不是 PNG 数据")
	}
	img, err := png.Decode(bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("解析 PNG 失败: %v", err)
	}
	if img.Bounds().Dx() != 256 || img.Bounds().Dy() != 256 {
		t.Errorf("头像尺寸 = %dx%d, want 256x256", img.Bounds().Dx(), img.Bounds().Dy())
	}
}

// ---------------------------------------------------------------------------
// 服务存活判定
// ---------------------------------------------------------------------------

// TestAdminServiceRunning 校验服务存活判定与 PHP 的 ps|grep|wc -l 一致。
func TestAdminServiceRunning(t *testing.T) {
	rules := AdminServiceRules()

	// php 与 crond 已随 PHP 运行时下线，规则表里不应再有这两个键。
	for _, gone := range []string{"php", "crond"} {
		if _, ok := rules[gone]; ok {
			t.Errorf("%s 已下线，不应再有探测规则", gone)
		}
	}

	phpLike := []string{
		"root 66 0.0.0 8860 5696 ? Ssl Oct22 0:00 nginx: master process /usr/sbin/nginx -g daemon off;",
		"root 69 0.0.0 510704 48252 ? Ssl Oct22 0:01 php-fpm: master process (/usr/local/etc/php-fpm.conf)",
		"root 65 0.0.0 215776 43032 ? S Oct22 0:00 php /data/www/codefever-community/misc/modify_authorized_keys.php modify_authorized_keys_shell_running",
		"git 110 0.0.0 215776 13648 ? S Oct22 0:00 php /data/www/codefever-community/misc/modify_authorized_keys.php modify_authorized_keys_shell_running",
	}
	if !AdminServiceRunning(rules["nginx"], phpLike) {
		t.Errorf("nginx 应为运行中")
	}
	// codefever 由后端进程代表（Go 版是 codefever-api）。
	if AdminServiceRunning(rules["codefever"], phpLike) {
		t.Errorf("只有 PHP 守护进程时，codefever 应为未运行")
	}
	if AdminServiceRunning(rules["sendmail"], phpLike) {
		t.Errorf("sendmail 应为未运行")
	}

	// Go 版后端进程代表 codefever 服务在运行。
	goLike := []string{"root 1 0.0.0 1369452 13928 ? Ssl 03:24 0:01 /data/www/codefever-community/codefever-api"}
	if !AdminServiceRunning(rules["codefever"], goLike) {
		t.Errorf("Go 后端进程应让 codefever 为运行中")
	}
	if AdminServiceRunning(rules["nginx"], goLike) {
		t.Errorf("nginx 应为未运行")
	}

	// 空列表（命令不可用）时全部为未运行，且不能 panic。
	for name, rule := range rules {
		if AdminServiceRunning(rule, nil) {
			t.Errorf("%s 在无进程信息时不应为运行中", name)
		}
	}
}

// ---------------------------------------------------------------------------
// vm 段解析
// ---------------------------------------------------------------------------

// TestAdminParseCPU 校验 vmstat 解析。
func TestAdminParseCPU(t *testing.T) {
	output := "procs -----------memory---------- ---swap-- -----io---- -system-- ------cpu-----\n" +
		" 2  0  65516 102640  3980  21756  0  14  0   0  0   1  0  0  0  0  0\n"
	cpu, ok := AdminParseCPU(output)
	if !ok {
		t.Fatalf("解析失败")
	}
	if cpu != 100 {
		t.Errorf("cpu = %d, want 100", cpu)
	}
	if _, ok := AdminParseCPU("procs -----------memory----------\n"); ok {
		t.Errorf("无数字行时不应解析成功")
	}
}

// TestAdminParseMemory 校验 free -m 解析。
func TestAdminParseMemory(t *testing.T) {
	output := "              total        used        free      shared  buff/cache   available\n" +
		"Mem:           3941        1062        1444          10        1435        2598\n" +
		"Swap:           2047           0        2047\n"
	memory, ok := AdminParseMemory(output)
	if !ok {
		t.Fatalf("解析失败")
	}
	// 100 - ceil(2598 / 3941 * 100) = 100 - 66 = 34。
	if memory != 34 {
		t.Errorf("memory = %d, want 34", memory)
	}
	if _, ok := AdminParseMemory("no mem line here\n"); ok {
		t.Errorf("缺少 Mem 行时不应解析成功")
	}
}

// TestAdminParseDisk 校验 df 解析。
func TestAdminParseDisk(t *testing.T) {
	output := "Filesystem     1K-blocks    Used Available Use% Mounted on\n" +
		"/dev/sde       41153028  1231012  37838160   4% /\n" +
		"/dev/sdf       41153028  1231012  37838160   9% /data\n"
	disk, ok := AdminParseDisk(output)
	if !ok {
		t.Fatalf("解析失败")
	}
	if disk.Name != "/dev/sde" {
		t.Errorf("磁盘名 = %q, want /dev/sde", disk.Name)
	}
	if disk.Usage != 4 {
		t.Errorf("占用率 = %d, want 4", disk.Usage)
	}
	if _, ok := AdminParseDisk("Filesystem 1K-blocks Used Available Use% Mounted on\n"); ok {
		t.Errorf("没有 /dev 行时不应解析成功")
	}
}
