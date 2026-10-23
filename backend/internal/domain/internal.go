// 本文件承载内部网关回调端点（/apiv1/internal/*）所需的纯逻辑：
// 仓库引用解析、uid 解析、钩子 stdin 解析、保护分支规则匹配。
//
// 这些规则全部逐字搬运自 PHP 侧：
//
//	application/controllers/apiv1/internal.php        （端点主流程）
//	application/models/repository_model.php:2039-2079 （保护分支匹配）
//
// 这些函数不碰数据库，因此可以被单元测试完整覆盖 —— 而端点本身的
// 失败分支（缺参、仓库不存在、权限不足）也全部由这里的判定决定。
package domain

import (
	"regexp"
	"strings"
)

// PHPTruthy 判断字符串在 PHP 语境下是否为"真"。
//
// PHP 的字符串真值规则是：空字符串为假，"0" 为假，其余为真。
// 端点里大量使用 `if ($data['user'] && $data['pass'])` 这类判定，
// 因此必须复刻，否则口令 "0" 之类的边界值会走错分支。
func PHPTruthy(s string) bool { return s != "" && s != "0" }

// repoRefPattern 是 PHP 侧解析 repo 字段的正则。
//
// 逐字对应 application/controllers/apiv1/internal.php:43 与 :85 的
// preg_match("/([\w\d_]+)\/([\w\d_]+)\.git/", $data['repo'], $parsedRepo)。
//
// 关键点：PHP 的 preg_match 是**最左匹配**且未锚定，因此
//
//	"/group/repo.git"      → group=group,  name=repo
//	"'group/repo.git'"     → group=group,  name=repo
//	"a/b/c.git"            → group=b,      name=c
//
// 前两种形态分别来自 http-gateway（URL 路径里的前导斜杠）与
// ssh-gateway（git 客户端加的单引号），两者都必须能解析。
var repoRefPattern = regexp.MustCompile(`([\w\d_]+)\/([\w\d_]+)\.git`)

// RepositoryRef 是从 repo 字段解析出的仓库组名与仓库名。
type RepositoryRef struct {
	// Group 是仓库组名（对应 PHP 的 $parsedRepo[1]）。
	Group string
	// Name 是仓库名（对应 PHP 的 $parsedRepo[2]）。
	Name string
}

// ParseRepositoryRef 解析 repo 字段，返回组名与仓库名。
//
// 正则未命中时返回 ok=false —— 对应 PHP 侧 $parsedRepo 为空数组的分支。
func ParseRepositoryRef(raw string) (RepositoryRef, bool) {
	m := repoRefPattern.FindStringSubmatch(raw)
	if m == nil || m[1] == "" || m[2] == "" {
		return RepositoryRef{}, false
	}
	return RepositoryRef{Group: m[1], Name: m[2]}, true
}

// InternalRepository 是内部网关回调端点所需的仓库字段。
//
// 只取 cc_repositories 的三列：主键、仓库名与 git-storage 下的相对路径。
type InternalRepository struct {
	// Key 是仓库主键（r_key）。
	Key string
	// Name 是仓库名（r_name）。
	Name string
	// Path 是仓库在 git-storage 下的相对路径（r_path），形如 "/de7347a8..."。
	Path string
}

// ParseUID 解析 "<u_key>|<r_key>" 形式的 uid。
//
// 逐字对应 internal.php:145-146：
//
//	$uKey = explode('|', $data['uid'])[0];
//	$rKey = explode('|', $data['uid'])[1];
//
// uid 是跨进程硬契约：http-gateway / ssh-gateway 把它写进 PGYER_UID 环境变量，
// git 钩子再原样回传给 internal.hooks。
// 两段中任一段为空都视为非法（PHP 侧靠真值判定落到 0x0201）。
func ParseUID(raw string) (uKey, rKey string, ok bool) {
	parts := strings.Split(raw, "|")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// HookPush 是一行 push 记录。
type HookPush struct {
	// From 是推送前的 commit（PHP 的 $pushInfo[0]）。
	From string
	// To 是推送后的 commit（PHP 的 $pushInfo[1]）。
	To string
	// Ref 是被更新的引用（PHP 的 $pushInfo[2]）。
	Ref string
}

// ParseHookPushLine 解析 git 钩子 stdin 的一行。
//
// 逐字对应 internal.php:153-154：
//
//	$pushInfo = explode(' ', $push);
//	if (count($pushInfo) === 3) { ... }
//
// 注意 explode(' ') 只按**单个空格**切分，且段数必须恰好为 3：
// "a b c" 合法；"a  b c"（双空格）、"a b c d"、"" 都会被跳过。
func ParseHookPushLine(line string) (HookPush, bool) {
	parts := strings.Split(line, " ")
	if len(parts) != 3 {
		return HookPush{}, false
	}
	return HookPush{From: parts[0], To: parts[1], Ref: parts[2]}, true
}

// ParseHookPushLines 解析 git 钩子的整段 stdin。
//
// 逐字对应 internal.php:151-154 的 foreach + 段数判定。
// stdin 末尾的换行会多出一个空行，空行段数为 1，自动被跳过。
func ParseHookPushLines(stdin string) []HookPush {
	out := []HookPush{}
	for _, line := range strings.Split(stdin, "\n") {
		if push, ok := ParseHookPushLine(line); ok {
			out = append(out, push)
		}
	}
	return out
}

// HookName 返回钩子脚本名。
//
// 逐字对应 internal.php:147 的 array_pop(explode('/', $data['args'][0]))。
func HookName(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx < 0 {
		return path
	}
	return path[idx+1:]
}

// BranchFromRef 去掉引用中的 refs/heads/ 前缀，得到分支名。
//
// 逐字对应 internal.php:168 的 str_replace（去掉 refs/heads/ 前缀）：
// 注意 str_replace 替换的是**所有**出现位置，因此这里用 ReplaceAll。
func BranchFromRef(ref string) string {
	return strings.ReplaceAll(ref, "refs/heads/", "")
}

// protectedBranchRegexPattern 判断一条规则是否是正则表达式。
//
// 逐字对应 repository_model.php:2054 的
// preg_match('/^\/(.*)\/[igm]*$/', $rule['pbr_rule'])：
// 以 "/" 开头、含第二个 "/"、后跟可选的 [igm] 修饰符的规则按正则处理，
// 其余规则按**精确字符串**匹配。
var protectedBranchRegexPattern = regexp.MustCompile(`^/(.*)/[igm]*$`)

// MatchProtectedBranchRule 判断一条保护分支规则是否命中给定的分支名。
//
// 逐字对应 repository_model.php:2052-2062。
//
// 这里用包级函数而不是方法：ProtectedBranchRule 是 cc_proteced_branch_rules
// 的行模型（定义在 domain/repository.go），把匹配逻辑挂在方法上会占用它的
// 方法集，其他域若也要判断保护分支就会撞名。
func MatchProtectedBranchRule(rule ProtectedBranchRule, branch string) bool {
	if !protectedBranchRegexPattern.MatchString(rule.Rule) {
		return rule.Rule == branch
	}
	pattern, ok := phpRegexToGo(rule.Rule)
	if !ok {
		return false
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		// PHP 侧非法正则同样导致 preg_match 失败（不命中）。
		return false
	}
	return re.MatchString(branch)
}

// phpRegexToGo 把 PHP 的 "/pattern/flags" 写法转换成 Go 的正则表达式。
//
// PHP 把分隔符与修饰符都写进模式串里，Go 的 regexp.Compile 不接受这种写法，
// 因此这里剥离分隔符并把 [imsx] 修饰符转换成 Go 的内联标志。
func phpRegexToGo(rule string) (string, bool) {
	if len(rule) < 2 || rule[0] != '/' {
		return "", false
	}
	end := strings.LastIndex(rule, "/")
	if end <= 0 {
		return "", false
	}
	body := rule[1:end]
	flags := rule[end+1:]

	var inline strings.Builder
	for _, f := range flags {
		switch f {
		case 'i':
			inline.WriteString("i")
		case 'm':
			inline.WriteString("m")
		case 's':
			inline.WriteString("s")
		case 'x':
			inline.WriteString("x")
		}
	}
	if inline.Len() == 0 {
		return body, true
	}
	return "(?" + inline.String() + ")" + body, true
}

// CheckBranchProtected 判断该角色能否向目标分支推送。
//
// 逐字对应 application/models/repository_model.php:2039-2079 的
// checkBranchProteced($rKey, $uKey, $ref, $isProtectMerge)：
//
//  1. 角色低于 ROLE_DEVELOPER 一律受保护（无论规则如何）；
//  2. 逐条匹配规则，命中后按 push/merge 的最低角色要求判定；
//  3. 角色低于最低要求 → 受保护。
//
// protectMerge 为 true 时用合并的最低角色，否则用 push 的最低角色。
func CheckBranchProtected(role Role, rules []ProtectedBranchRule, branch string, protectMerge bool) bool {
	if role < RoleDeveloper {
		return true
	}
	for _, rule := range rules {
		if !MatchProtectedBranchRule(rule, branch) {
			continue
		}
		min := Role(rule.PushRole)
		if protectMerge {
			min = Role(rule.MergeRole)
		}
		if role < min {
			return true
		}
	}
	return false
}
