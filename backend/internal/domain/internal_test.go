package domain

import "testing"

// TestParseRepositoryRef 覆盖 repo 字段的多种形态。
//
// 逐字对应 PHP 侧 internal.php:43 / :85 的
// preg_match("/([\w\d_]+)\/([\w\d_]+)\.git/", $data['repo'], $parsedRepo)。
func TestParseRepositoryRef(t *testing.T) {
	cases := []struct {
		name  string
		raw   string
		group string
		repo  string
		ok    bool
	}{
		{
			name:  "http-gateway 形态：带前导斜杠",
			raw:   "/rootteam/demorepo.git",
			group: "rootteam",
			repo:  "demorepo",
			ok:    true,
		},
		{
			name:  "ssh-gateway 形态：带单引号",
			raw:   "'rootteam/demorepo.git'",
			group: "rootteam",
			repo:  "demorepo",
			ok:    true,
		},
		{
			name:  "契约采集器形态：无前缀",
			raw:   "rootteam/demorepo.git",
			group: "rootteam",
			repo:  "demorepo",
			ok:    true,
		},
		{
			name:  "最左匹配：多级路径只取最后两段",
			raw:   "a/b/c.git",
			group: "b",
			repo:  "c",
			ok:    true,
		},
		{
			name: "空字符串",
			raw:  "",
			ok:   false,
		},
		{
			name: "缺少 .git 后缀",
			raw:  "/rootteam/demorepo",
			ok:   false,
		},
		{
			name: "缺少组名",
			raw:  "/demorepo.git",
			ok:   false,
		},
		{
			name:  "下划线与数字属于 \\w",
			raw:   "/group_1/repo_2.git",
			group: "group_1",
			repo:  "repo_2",
			ok:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ref, ok := ParseRepositoryRef(tc.raw)
			if ok != tc.ok {
				t.Fatalf("ok = %v, 期望 %v", ok, tc.ok)
			}
			if !ok {
				return
			}
			if ref.Group != tc.group || ref.Name != tc.repo {
				t.Fatalf("解析结果 = (%q, %q), 期望 (%q, %q)", ref.Group, ref.Name, tc.group, tc.repo)
			}
		})
	}
}

// TestParseUID 覆盖跨进程硬契约 "<u_key>|<r_key>" 的解析。
func TestParseUID(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		uKey string
		rKey string
		ok   bool
	}{
		{name: "标准两段", raw: "uk|rk", uKey: "uk", rKey: "rk", ok: true},
		{name: "多出第三段时取前两段", raw: "uk|rk|extra", uKey: "uk", rKey: "rk", ok: true},
		{name: "缺少第二段", raw: "uk", ok: false},
		{name: "用户主键为空", raw: "|rk", ok: false},
		{name: "仓库主键为空", raw: "uk|", ok: false},
		{name: "空字符串", raw: "", ok: false},
		{name: "只有分隔符", raw: "|", ok: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uKey, rKey, ok := ParseUID(tc.raw)
			if ok != tc.ok {
				t.Fatalf("ok = %v, 期望 %v", ok, tc.ok)
			}
			if !ok {
				return
			}
			if uKey != tc.uKey || rKey != tc.rKey {
				t.Fatalf("解析结果 = (%q, %q), 期望 (%q, %q)", uKey, rKey, tc.uKey, tc.rKey)
			}
		})
	}
}

// TestParseHookPushLine 覆盖 git 钩子 stdin 的三段解析。
//
// 逐字对应 internal.php:153-154 的 explode(' ', $push) 与 count(...) === 3。
func TestParseHookPushLine(t *testing.T) {
	cases := []struct {
		name string
		line string
		from string
		to   string
		ref  string
		ok   bool
	}{
		{
			name: "标准三段",
			line: "c943e85a f67cfee7 refs/heads/master",
			from: "c943e85a",
			to:   "f67cfee7",
			ref:  "refs/heads/master",
			ok:   true,
		},
		{
			name: "两段不合法",
			line: "c943e85a f67cfee7",
			ok:   false,
		},
		{
			name: "四段不合法",
			line: "a b c d",
			ok:   false,
		},
		{
			name: "双空格会产生四段",
			line: "a  b c",
			ok:   false,
		},
		{
			name: "空行不合法",
			line: "",
			ok:   false,
		},
		{
			name: "首段为空仍算三段",
			line: " b c",
			from: "",
			to:   "b",
			ref:  "c",
			ok:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			push, ok := ParseHookPushLine(tc.line)
			if ok != tc.ok {
				t.Fatalf("ok = %v, 期望 %v", ok, tc.ok)
			}
			if !ok {
				return
			}
			if push.From != tc.from || push.To != tc.to || push.Ref != tc.ref {
				t.Fatalf("解析结果 = %+v, 期望 (%q, %q, %q)", push, tc.from, tc.to, tc.ref)
			}
		})
	}
}

// TestParseHookPushLines 覆盖整段 stdin 的解析。
func TestParseHookPushLines(t *testing.T) {
	stdin := "aaa bbb refs/heads/master\nccc ddd refs/heads/dev\n"
	pushes := ParseHookPushLines(stdin)
	if len(pushes) != 2 {
		t.Fatalf("解析出 %d 条 push 记录, 期望 2", len(pushes))
	}
	if pushes[0].From != "aaa" || pushes[0].To != "bbb" || pushes[0].Ref != "refs/heads/master" {
		t.Fatalf("第 1 条 = %+v", pushes[0])
	}
	if pushes[1].From != "ccc" || pushes[1].To != "ddd" || pushes[1].Ref != "refs/heads/dev" {
		t.Fatalf("第 2 条 = %+v", pushes[1])
	}

	// 末尾多一个换行不应产生额外记录。
	pushes = ParseHookPushLines("aaa bbb refs/heads/master\n")
	if len(pushes) != 1 {
		t.Fatalf("解析出 %d 条 push 记录, 期望 1", len(pushes))
	}
}

// TestHookName 覆盖钩子名的提取。
func TestHookName(t *testing.T) {
	cases := map[string]string{
		"/data/www/git-storage/de7347/hooks/post-receive": "post-receive",
		"/data/www/git-storage/de7347/hooks/pre-receive":  "pre-receive",
		"post-receive": "post-receive",
		"/path/":       "",
		"":             "",
	}
	for in, want := range cases {
		if got := HookName(in); got != want {
			t.Fatalf("HookName(%q) = %q, 期望 %q", in, got, want)
		}
	}
}

// TestBranchFromRef 覆盖 refs/heads/ 前缀的剥离。
func TestBranchFromRef(t *testing.T) {
	cases := map[string]string{
		"refs/heads/master":       "master",
		"master":                  "master",
		"refs/heads/feature/x":    "feature/x",
		"refs/heads/refs/heads/x": "x",
		"":                        "",
	}
	for in, want := range cases {
		if got := BranchFromRef(in); got != want {
			t.Fatalf("BranchFromRef(%q) = %q, 期望 %q", in, got, want)
		}
	}
}

// TestPHPTruthy 覆盖 PHP 的字符串真值规则。
func TestPHPTruthy(t *testing.T) {
	cases := map[string]bool{
		"":          false,
		"0":         false,
		"a":         true,
		"123":       true,
		"root@x.cn": true,
	}
	for in, want := range cases {
		if got := PHPTruthy(in); got != want {
			t.Fatalf("PHPTruthy(%q) = %v, 期望 %v", in, got, want)
		}
	}
}

// TestProtectedBranchRuleMatch 覆盖保护分支规则的匹配。
//
// 逐字对应 repository_model.php:2052-2062：
// 以 "/…/" 包裹的规则按正则处理，其余按精确字符串匹配。
func TestProtectedBranchRuleMatch(t *testing.T) {
	cases := []struct {
		name   string
		rule   string
		branch string
		match  bool
	}{
		{name: "精确匹配命中", rule: "master", branch: "master", match: true},
		{name: "精确匹配未命中", rule: "master", branch: "develop", match: false},
		{name: "正则命中", rule: "/^master$/", branch: "master", match: true},
		{name: "正则未命中", rule: "/^master$/", branch: "develop", match: false},
		{name: "正则带 i 修饰符", rule: "/^MASTER$/i", branch: "master", match: true},
		{name: "正则带 g 修饰符按正则处理", rule: "/^mas.*$/g", branch: "master", match: true},
		{name: "正则含路径分隔", rule: "/^release/.*/", branch: "release/1.0", match: true},
		{name: "非法正则不命中", rule: "/^(unclosed/", branch: "unclosed", match: false},
		{name: "空规则不命中", rule: "", branch: "master", match: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rule := ProtectedBranchRule{Rule: tc.rule}
			if got := MatchProtectedBranchRule(rule, tc.branch); got != tc.match {
				t.Fatalf("MatchProtectedBranchRule(%q) = %v, 期望 %v", tc.rule, got, tc.match)
			}
		})
	}
}

// TestCheckBranchProtected 覆盖保护分支的整体判定。
//
// 逐字对应 repository_model.php:2039-2079 的 checkBranchProteced。
func TestCheckBranchProtected(t *testing.T) {
	rules := []ProtectedBranchRule{
		{Key: "1", Rule: "master", PushRole: int(RoleMaintainer), MergeRole: int(RoleOwner)},
		{Key: "2", Rule: "/^release\\/.*$/", PushRole: int(RoleDeveloper), MergeRole: int(RoleOwner)},
	}

	cases := []struct {
		name         string
		role         Role
		branch       string
		protectMerge bool
		protected    bool
	}{
		{
			name:      "角色低于 DEVELOPER 一律受保护",
			role:      RoleReporter,
			branch:    "develop",
			protected: true,
		},
		{
			name:      "无权限角色受保护",
			role:      RoleNoPermission,
			branch:    "develop",
			protected: true,
		},
		{
			name:      "DEVELOPER 低于 master 的 push 最低角色",
			role:      RoleDeveloper,
			branch:    "master",
			protected: true,
		},
		{
			name:      "MAINTAINER 达到 master 的 push 最低角色",
			role:      RoleMaintainer,
			branch:    "master",
			protected: false,
		},
		{
			name:      "OWNER 达到 master 的 push 最低角色",
			role:      RoleOwner,
			branch:    "master",
			protected: false,
		},
		{
			name:      "未命中任何规则",
			role:      RoleDeveloper,
			branch:    "develop",
			protected: false,
		},
		{
			name:      "正则规则命中且最低角色为 DEVELOPER",
			role:      RoleDeveloper,
			branch:    "release/1.0",
			protected: false,
		},
		{
			name:         "合并场景用 merge 最低角色",
			role:         RoleMaintainer,
			branch:       "master",
			protectMerge: true,
			protected:    true,
		},
		{
			name:         "合并场景 OWNER 放行",
			role:         RoleOwner,
			branch:       "master",
			protectMerge: true,
			protected:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CheckBranchProtected(tc.role, rules, tc.branch, tc.protectMerge)
			if got != tc.protected {
				t.Fatalf("CheckBranchProtected = %v, 期望 %v", got, tc.protected)
			}
		})
	}
}
