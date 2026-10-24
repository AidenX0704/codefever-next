// 管理（后台）域的业务规则。
//
// 每个端点都标注了对应的 PHP 源码位置，便于逐条比对契约：
// application/controllers/api/admin.php。
//
// 本文件只放**纯逻辑**（排序白名单、分页归一化、分类过滤、邮箱格式、
// 头像生成、config.yaml 文档的解析/合并/序列化/校验）。
// 任何 I/O（SQL、文件、进程探测）都在 repo 与 handlers 包里。
package domain

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// 常量
// ---------------------------------------------------------------------------

// AdminUserCategory* 对应 PHP 常量 ADMIN_USER_CATEGORY_*
// （application/config/constants.php:167-169）。
//
// 前端管理页的四个页签按 0/1/2/3 依次是"全部 / 已启用 MFA / 未启用 MFA / 已禁用"
// （www/view/src/components/view/admin/Users.js:388-393）。
const (
	// AdminUserCategoryAll 是"全部用户"，不加任何过滤条件。
	AdminUserCategoryAll = 0
	// AdminUserCategoryMFAEnabled 对应 ADMIN_USER_CATEGORY_MFAENABLED。
	AdminUserCategoryMFAEnabled = 1
	// AdminUserCategoryMFADisabled 对应 ADMIN_USER_CATEGORY_MFADISABLED。
	AdminUserCategoryMFADisabled = 2
	// AdminUserCategoryStatusDelete 对应 ADMIN_USER_CATEGORY_STATUSDELETE。
	AdminUserCategoryStatusDelete = 3
)

// AdminUserCategoryStatusDeleteValue 是 COMMON_STATUS_DELETE 的取值。
const AdminUserCategoryStatusDeleteValue = 4

// AdminPageSizeDefault 与 AdminPageSizeMax 对应 PHP 侧的默认分页与上限。
//
// PHP 侧 admin 域的分页参数来自 body（multipart），因此 (int) 转换有效，
// **不像 repository 域那样被强制归一化成 1/20**。
const (
	AdminPageSizeDefault = 20
	AdminPageSizeMax     = 100
)

// ---------------------------------------------------------------------------
// 表行与归一化输出
// ---------------------------------------------------------------------------

// AdminUser 是 cc_users 表的一行（管理域视角）。
//
// 与 domain.User 的差别有两点：
//   - Created 保留 MySQL 的**原始字符串**（PHP 的 row_array() 也是原样返回），
//     管理页要展示的是 "2022-01-01 00:00:00" 而不是时间戳；
//   - 只取管理域需要的列。
type AdminUser struct {
	Key     string
	Email   string
	Name    string
	Avatar  *string
	TwoFA   *string
	Created string
	Status  int
	Admin   bool
}

// AdminUserListItem 是 admin/userList 的单条记录。
//
// 逐字对应 application/models/user_model.php:286-309 的 normalizeUserList()：
// 字段集合、顺序与类型必须与 PHP 一致（avatar 为 null 还是空串也必须一致）。
type AdminUserListItem struct {
	ID         string  `json:"id"`
	Email      string  `json:"email"`
	Name       string  `json:"name"`
	Avatar     *string `json:"avatar"`
	MFAEnabled bool    `json:"mfaEnabled"`
	Created    string  `json:"created"`
	Status     bool    `json:"status"`
	Admin      bool    `json:"admin"`
	Projects   int     `json:"projects"`
}

// AdminUsage 是 admin/systemStatus 的 usage 段。
//
// 逐字对应 application/controllers/api/admin.php:362-374。
type AdminUsage struct {
	User       int `json:"user"`
	Repository int `json:"repository"`
	Group      int `json:"group"`
}

// AdminDiskUsage 是 vm 段里的单个磁盘条目。
type AdminDiskUsage struct {
	Name  string `json:"name"`
	Usage int    `json:"usage"`
}

// AdminVMStatus 是 admin/systemStatus 的 vm 段。
//
// PHP 侧只有在对应命令执行成功时才写入某个键（vm['cpu'] 等），
// 因此命令缺失时 JSON 里会缺字段。新实现**始终输出三个键**，
// 保证前端拿到的结构稳定（命令缺失时取零值）。
type AdminVMStatus struct {
	CPU    int              `json:"cpu"`
	Memory int              `json:"memory"`
	Disk   []AdminDiskUsage `json:"disk"`
}

// AdminServiceStatus 是 admin/systemStatus 的 service 段。
//
// 逐字对应 application/controllers/api/admin.php:420-426 的五个键。
//
// **php 与 crond 两个键已随 PHP 运行时下线而移除**（详见 AdminServiceRules）：
// 前端管理端仪表盘同步去掉了 PHP-FPM / Crond 两张卡片。
type AdminServiceStatus struct {
	Nginx     bool `json:"nginx"`
	CodeFever bool `json:"codefever"`
	Sendmail  bool `json:"sendmail"`
}

// ---------------------------------------------------------------------------
// 排序白名单
// ---------------------------------------------------------------------------

// AdminUserSortColumn 把前端传入的排序名映射到数据库列。
//
// 逐字对应 application/controllers/api/admin.php:35：
//
//	$sort = in_array($sort, ['name', 'email', 'created']) ? 'u_' . $sort : '';
//
// 返回空串表示"不排序"（PHP 侧同样不追加 order_by）。
func AdminUserSortColumn(sort string) string {
	switch sort {
	case "name":
		return "u_name"
	case "email":
		return "u_email"
	case "created":
		return "u_created"
	}
	return ""
}

// AdminGroupSortColumn 把前端传入的排序名映射到数据库列。
//
// 逐字对应 application/controllers/api/admin.php:172：
//
//	$sort = in_array($sort, ['name', 'created']) ? 'g_' . $sort : '';
func AdminGroupSortColumn(sort string) string {
	switch sort {
	case "name":
		return "g_name"
	case "created":
		return "g_created"
	}
	return ""
}

// AdminRepositorySortColumn 把前端传入的排序名映射到数据库列。
//
// 逐字对应 application/controllers/api/admin.php:283：
//
//	$sort = in_array($sort, ['name', 'created']) ? 'r_' . $sort : '';
func AdminRepositorySortColumn(sort string) string {
	switch sort {
	case "name":
		return "r_name"
	case "created":
		return "r_created"
	}
	return ""
}

// ---------------------------------------------------------------------------
// 分页与过滤
// ---------------------------------------------------------------------------

// AdminPaging 归一化分页参数。
//
// 逐字对应 application/controllers/api/admin.php:33-34：
//
//	$page = $page ? $page : 1;
//	$pagesize = $pagesize ? $pagesize : 20;
//
// 注意 admin 域的分页参数走 body（multipart），
// 因此 PHP 的 (int) 转换有效 —— 与 repository 域"query 分页被强制归一化"不同。
//
// 性能整改 C1（2026-10）：在 PHP 语义之上**补了上限**。
// 原先这里不封顶，而 AdminPageSizeMax 声明后全仓库无人引用，
// 导致 `pagesize=1000000` 会变成 `LIMIT 1000000`。
// 上限口径统一收敛到 domain.ClampPaging（见 paging.go）。
func AdminPaging(page, pagesize int) (int, int) {
	return ClampPaging(page, pagesize)
}

// AdminPagingOffset 计算 SQL LIMIT 的偏移量。
//
// 逐字对应 PHP 侧的 ($page - 1) * $pagesize。
//
// 偏移量为负时收敛到 0 —— PHP 会把负偏移量原样交给 MySQL 从而报错，
// 新实现不让一个畸形参数变成 500。
func AdminPagingOffset(page, pagesize int) int {
	offset := (page - 1) * pagesize
	if offset < 0 {
		return 0
	}
	return offset
}

// AdminUserCategoryWhere 返回用户分类对应的 SQL 条件。
//
// 逐字对应 application/models/user_model.php:313-316：
//
//	$category == ADMIN_USER_CATEGORY_MFAENABLED   && where('u_2fa !=', '')
//	$category == ADMIN_USER_CATEGORY_MFADISABLED  && where('u_2fa IS NULL')
//	$category == ADMIN_USER_CATEGORY_STATUSDELETE && where('u_status', COMMON_STATUS_DELETE)
//
// 分类为 0（全部）时不加条件。
func AdminUserCategoryWhere(category int) (string, []any) {
	switch category {
	case AdminUserCategoryMFAEnabled:
		return "u_2fa != ?", []any{""}
	case AdminUserCategoryMFADisabled:
		return "u_2fa IS NULL", nil
	case AdminUserCategoryStatusDelete:
		return "u_status = ?", []any{AdminUserCategoryStatusDeleteValue}
	}
	return "", nil
}

// AdminUserKeywordWhere 返回关键词过滤条件。
//
// 逐字对应 application/models/user_model.php:318-321：
//
//	$this->db->where("(u_name like '%{$keyword}%' OR u_email like '%{$keyword}%')");
func AdminUserKeywordWhere(keyword string) (string, []any) {
	if keyword == "" {
		return "", nil
	}
	return "(u_name LIKE ? OR u_email LIKE ?)", []any{
		"%" + escapeLikePattern(keyword) + "%",
		"%" + escapeLikePattern(keyword) + "%",
	}
}

// AdminDisplayNameKeywordWhere 返回按显示名模糊匹配的关键词条件。
//
// 逐字对应 application/models/group_model.php:348 与
// application/models/repository_model.php:2085 的 `$this->db->like(...)`。
func AdminDisplayNameKeywordWhere(column, keyword string) (string, []any) {
	if keyword == "" {
		return "", nil
	}
	return column + " LIKE ?", []any{"%" + escapeLikePattern(keyword) + "%"}
}

// escapeLikePattern 转义 LIKE 通配符。
//
// PHP 侧用 CI 的 escape_like_str（转义符为 !），生成的 SQL 却不带 ESCAPE 子句，
// 这里按 MySQL 默认转义符 \ 处理 % 与 _，语义等价且更安全。
func escapeLikePattern(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

// ---------------------------------------------------------------------------
// 邮箱与初始口令
// ---------------------------------------------------------------------------

// adminEmailPattern 逐字搬运 application/controllers/api/admin.php:137 的正则。
//
// 注意该正则**区分大小写**（PHP 侧没有 i 修饰符），
// 因此大写字母的邮箱会被判定为非法 —— 这是原实现的行为，这里原样复刻。
var adminEmailPattern = regexp.MustCompile(`^[a-z0-9]+([._\-]*[a-z0-9])*@([a-z0-9]+[-a-z0-9]*[a-z0-9]+.){1,63}[a-z0-9]+$`)

// AdminEmailValid 判断邮箱是否满足 PHP 侧的格式要求。
func AdminEmailValid(email string) bool {
	return adminEmailPattern.MatchString(email)
}

// AdminInitialPassword 生成 32 位小写十六进制的初始口令。
//
// 逐字对应 PHP 侧 UUID::getKey()
// （application/libraries/service/Utility/UUID.php:9）：
//
//	md5(time() . rand() . rand() . rand() . uniqid())
//
// 同样是 32 位小写十六进制；随机源换成 crypto/rand（PHP 的 rand() 可预测）。
func AdminInitialPassword() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%032x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// ---------------------------------------------------------------------------
// 头像
// ---------------------------------------------------------------------------

// adminMaterialColors 是 PHP 侧 MaterialDesign Avatars 的调色板。
//
// 逐字搬运 application/libraries/Avatars/MaterialDesign.Avatars.class.php:100-107。
//
// PHP 侧用 mt_rand() 随机挑一个颜色；新实现改成**按邮箱哈希确定**，
// 好处是同一个邮箱每次生成的头像颜色一致（可测试、可复现）。
var adminMaterialColors = [][3]uint8{
	{111, 212, 230},
	{248, 216, 104},
	{157, 215, 203},
	{254, 165, 141},
	{162, 209, 71},
	{86, 156, 235},
}

// AdminAvatarColor 按 seed 的哈希确定头像底色。
func AdminAvatarColor(seed string) (uint8, uint8, uint8) {
	sum := md5.Sum([]byte(seed))
	c := adminMaterialColors[int(sum[0])%len(adminMaterialColors)]
	return c[0], c[1], c[2]
}

// AdminAvatarLetter 取头像上显示的字符。
//
// 逐字对应 MaterialDesign.Avatars.class.php:29 的
// `strtoupper(mb_substr($Char, 0, 1, "UTF-8"))`：
// 只取**首字符**并转大写。ASCII 之外的字符（中文等）PHP 侧会走中文字体，
// 新实现没有内嵌中文字体，退化为 "#"。
func AdminAvatarLetter(seed string) string {
	for _, r := range seed {
		if r >= 'a' && r <= 'z' {
			return strings.ToUpper(string(r))
		}
		if r >= 'A' && r <= 'Z' {
			return string(r)
		}
		if r >= '0' && r <= '9' {
			return string(r)
		}
		break
	}
	return "#"
}

// adminFont5x7 是 5×7 点阵字模（列优先，最低位为最上一行）。
//
// 只包含头像需要的 A-Z、0-9 与 '#'，其余字符统一回退到 '#'。
// 字模取自经典的 5×7 ASCII 点阵字库。
var adminFont5x7 = map[rune][5]byte{
	'0': {0x3E, 0x51, 0x49, 0x45, 0x3E},
	'1': {0x00, 0x42, 0x7F, 0x40, 0x00},
	'2': {0x42, 0x61, 0x51, 0x49, 0x46},
	'3': {0x21, 0x41, 0x45, 0x4B, 0x31},
	'4': {0x18, 0x14, 0x12, 0x7F, 0x10},
	'5': {0x27, 0x45, 0x45, 0x45, 0x39},
	'6': {0x3C, 0x4A, 0x49, 0x49, 0x30},
	'7': {0x01, 0x71, 0x09, 0x05, 0x03},
	'8': {0x36, 0x49, 0x49, 0x49, 0x36},
	'9': {0x06, 0x49, 0x49, 0x29, 0x1E},
	'A': {0x7E, 0x11, 0x11, 0x11, 0x7E},
	'B': {0x7F, 0x49, 0x49, 0x49, 0x36},
	'C': {0x3E, 0x41, 0x41, 0x41, 0x22},
	'D': {0x7F, 0x41, 0x41, 0x22, 0x1C},
	'E': {0x7F, 0x49, 0x49, 0x49, 0x41},
	'F': {0x7F, 0x09, 0x09, 0x01, 0x01},
	'G': {0x3E, 0x41, 0x41, 0x51, 0x32},
	'H': {0x7F, 0x08, 0x08, 0x08, 0x7F},
	'I': {0x00, 0x41, 0x7F, 0x41, 0x00},
	'J': {0x20, 0x40, 0x41, 0x3F, 0x01},
	'K': {0x7F, 0x08, 0x14, 0x22, 0x41},
	'L': {0x7F, 0x40, 0x40, 0x40, 0x40},
	'M': {0x7F, 0x02, 0x04, 0x02, 0x7F},
	'N': {0x7F, 0x04, 0x08, 0x10, 0x7F},
	'O': {0x3E, 0x41, 0x41, 0x41, 0x3E},
	'P': {0x7F, 0x09, 0x09, 0x09, 0x06},
	'Q': {0x3E, 0x41, 0x51, 0x21, 0x5E},
	'R': {0x7F, 0x09, 0x19, 0x29, 0x46},
	'S': {0x46, 0x49, 0x49, 0x49, 0x31},
	'T': {0x01, 0x01, 0x7F, 0x01, 0x01},
	'U': {0x3F, 0x40, 0x40, 0x40, 0x3F},
	'V': {0x1F, 0x20, 0x40, 0x20, 0x1F},
	'W': {0x7F, 0x20, 0x18, 0x20, 0x7F},
	'X': {0x63, 0x14, 0x08, 0x14, 0x63},
	'Y': {0x03, 0x04, 0x78, 0x04, 0x03},
	'Z': {0x61, 0x51, 0x49, 0x45, 0x43},
	'#': {0x14, 0x7F, 0x14, 0x7F, 0x14},
}

// 头像画布尺寸。PHP 侧先画 512×512 再缩到 256×256
// （MaterialDesign.Avatars.class.php:247-248），这里直接画 256×256。
const (
	adminAvatarSize    = 256
	adminAvatarScale   = 14
	adminAvatarCapHigh = 7
	adminAvatarCapWide = 5
)

// AdminAvatarPNG 生成头像 PNG 字节流。
//
// 逐字对应 PHP 侧 User_model::createAvatar()：
// 512×512 画布 → 填充 Material Design 底色 → 居中画一个白色字符 → 存成 PNG。
// 底色由 AdminAvatarColor 确定，字符由 AdminAvatarLetter 确定。
func AdminAvatarPNG(seed string) ([]byte, error) {
	r, g, b := AdminAvatarColor(seed)
	letter := AdminAvatarLetter(seed)

	img := image.NewRGBA(image.Rect(0, 0, adminAvatarSize, adminAvatarSize))
	background := color.RGBA{R: r, G: g, B: b, A: 255}
	for y := 0; y < adminAvatarSize; y++ {
		for x := 0; x < adminAvatarSize; x++ {
			img.Set(x, y, background)
		}
	}

	glyph, ok := adminFont5x7[rune(letter[0])]
	if !ok {
		glyph = adminFont5x7['#']
	}

	// 居中：字模 5×7，按 adminAvatarScale 放大。
	glyphW := adminAvatarCapWide * adminAvatarScale
	glyphH := adminAvatarCapHigh * adminAvatarScale
	originX := (adminAvatarSize - glyphW) / 2
	originY := (adminAvatarSize - glyphH) / 2

	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	for col := 0; col < adminAvatarCapWide; col++ {
		bits := glyph[col]
		for row := 0; row < adminAvatarCapHigh; row++ {
			if bits&(1<<uint(row)) == 0 {
				continue
			}
			for dy := 0; dy < adminAvatarScale; dy++ {
				for dx := 0; dx < adminAvatarScale; dx++ {
					img.Set(originX+col*adminAvatarScale+dx, originY+row*adminAvatarScale+dy, white)
				}
			}
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ---------------------------------------------------------------------------
// 系统状态探测规则
// ---------------------------------------------------------------------------

// AdminProcessMatcher 是一条进程存活判定规则。
type AdminProcessMatcher struct {
	// Pattern 是进程命令行中要匹配的子串。
	Pattern string
	// MinCount 是判定为"运行中"所需的最少匹配进程数。
	MinCount int
}

// AdminServiceRule 描述一个服务的存活判定规则。
type AdminServiceRule struct {
	// Matchers 是候选规则；**任意一条**满足即认为该服务在运行。
	Matchers []AdminProcessMatcher
}

// AdminServiceRules 给出 5 个服务对应的进程判定规则。
//
// 逐字对应 application/controllers/api/admin.php:420-461 的
// `ps uax | grep "<pattern>" | grep -v grep | wc -l` 判定：
//
//	nginx     → "nginx: master process"                  计数 > 0
//	codefever → "codefever-api"                           计数 > 0
//	sendmail  → "sendmail"                               计数 > 0
//
// **php 与 crond 已随 PHP 运行时一起下线**，因此不再出现在探测规则里：
//
//   - php-fpm：PHP 后端已由 Go 后端取代，进程不再存在
//   - crond：原本只为 PHP 的定时任务调度器（codefever_schedule）而存在，
//     webhook 投递现在由 Go worker 内部承担，不再需要系统 crond
//
// Go 部署里"CodeFever 服务在运行"由后端进程本身代表（codefever-api）。
func AdminServiceRules() map[string]AdminServiceRule {
	return map[string]AdminServiceRule{
		"nginx": {Matchers: []AdminProcessMatcher{
			{Pattern: "nginx: master process", MinCount: 1},
		}},
		"codefever": {Matchers: []AdminProcessMatcher{
			{Pattern: "codefever-api", MinCount: 1},
		}},
		"sendmail": {Matchers: []AdminProcessMatcher{
			{Pattern: "sendmail", MinCount: 1},
		}},
	}
}

// AdminServiceRunning 判断某个服务是否处于运行中。
//
// cmdlines 是全部进程的命令行列表。命令探测不可用（列表为空）时返回 false，
// 绝不能因此让端点 500。
func AdminServiceRunning(rule AdminServiceRule, cmdlines []string) bool {
	for _, matcher := range rule.Matchers {
		count := 0
		for _, line := range cmdlines {
			if strings.Contains(line, matcher.Pattern) {
				count++
			}
		}
		if count >= matcher.MinCount {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// vm 段解析
// ---------------------------------------------------------------------------

// numberPattern 匹配一段数字，逐字对应 PHP 的 /(\d+)/。
var numberPattern = regexp.MustCompile(`\d+`)

// spacedNumberPattern 匹配"空白 + 数字"，逐字对应 PHP 的 /\s+(\d+)/。
var spacedNumberPattern = regexp.MustCompile(`\s+(\d+)`)

// AdminParseCPU 从 vmstat 输出解析 CPU 使用率。
//
// 逐字对应 application/controllers/api/admin.php:379-388：
// 取第一行含数字的行，第 15 个数字是 vmstat 的 idle 百分比，
// 返回 100 - idle（vmstat 的 cpu 段依次为 us/sy/id/wa/st，第 15 个即 id）。
func AdminParseCPU(output string) (int, bool) {
	for _, line := range strings.Split(output, "\n") {
		numbers := numberPattern.FindAllString(line, -1)
		if len(numbers) <= 14 {
			continue
		}
		idle, err := strconv.Atoi(numbers[14])
		if err != nil {
			continue
		}
		return 100 - idle, true
	}
	return 0, false
}

// AdminParseMemory 从 free -m 输出解析内存占用率。
//
// 逐字对应 application/controllers/api/admin.php:391-400：
// 取以 "Mem:" 开头的行，$matches[1][0] 是 total、$matches[1][5] 是 available，
// 返回 100 - ceil(available / total * 100)。
func AdminParseMemory(output string) (int, bool) {
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, "Mem:") {
			continue
		}
		numbers := spacedNumberPattern.FindAllStringSubmatch(line, -1)
		if len(numbers) <= 5 {
			continue
		}
		total, err := strconv.Atoi(numbers[0][1])
		if err != nil || total == 0 {
			continue
		}
		available, err := strconv.Atoi(numbers[5][1])
		if err != nil {
			continue
		}
		used := int(float64(available) / float64(total) * 100)
		if float64(used) < float64(available)/float64(total)*100 {
			used++
		}
		return 100 - used, true
	}
	return 0, false
}

// AdminParseDisk 从 df 输出解析磁盘占用。
//
// 逐字对应 application/controllers/api/admin.php:403-417：
// 取第一行以 "/dev" 开头的行，名字是行首第一个空格分隔的字段，
// 占用率是第 4 个数字（Use%）。
func AdminParseDisk(output string) (AdminDiskUsage, bool) {
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, "/dev") {
			continue
		}
		numbers := spacedNumberPattern.FindAllStringSubmatch(line, -1)
		if len(numbers) <= 3 {
			continue
		}
		usage, err := strconv.Atoi(numbers[3][1])
		if err != nil {
			continue
		}
		name := strings.Fields(line)[0]
		return AdminDiskUsage{Name: name, Usage: usage}, true
	}
	return AdminDiskUsage{}, false
}

// ---------------------------------------------------------------------------
// config.yaml 文档
//
// PHP 侧 admin/config 直接读写 config.yaml（application/controllers/api/admin.php:470-484）。
// 键结构必须与 config.template.yaml 一致：
//
//	host / ssh / allowRegister / email.{name,from}
// ---------------------------------------------------------------------------

// SiteDocumentKeyOrder 是 config.yaml 的规范键顺序。
//
// 逐字对应 config.template.yaml 的键顺序；写回文件时保持该顺序，
// 使 diff 可读、也与 PHP 侧 yaml_emit_file 的产物形状一致。
var SiteDocumentKeyOrder = []string{"host", "ssh", "allowRegister", "email"}

// SiteEmailKeyOrder 是 config.yaml 中 email 子映射的规范键顺序。
var SiteEmailKeyOrder = []string{"name", "from"}

// ErrSiteDocumentNotMap 表示配置文档不是一个键值映射。
var ErrSiteDocumentNotMap = errors.New("配置文档不是键值映射")

// ParseSiteDocument 解析 config.yaml 的内容，返回键值映射。
//
// 逐字对应 PHP 侧 yaml_parse_file()，但**额外要求**结果必须是映射：
// PHP 侧拿到 null 后仍然继续写文件，最终把 config.yaml 写成空结构，
// 使 www/index.php:21 的 make_constants('YAML_', ...) 抛 TypeError，
// 登录与全部 API 永久 500（P0 缺陷）。
// 新实现在解析阶段就拒绝非映射的文档。
func ParseSiteDocument(data []byte) (map[string]any, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, ErrSiteDocumentNotMap
	}
	var decoded any
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		return nil, err
	}
	doc, ok := decoded.(map[string]any)
	if !ok || doc == nil {
		return nil, ErrSiteDocumentNotMap
	}
	return doc, nil
}

// MergeSiteDocument 按 PHP 的 array_merge 语义合并配置文档。
//
// 逐字对应 application/controllers/api/admin.php:481：
//
//	$final = array_merge($config, $input);
//
// array_merge 对字符串键是**浅合并**且后者覆盖前者，
// 因此 patch 里出现的键覆盖 base 里的同键，patch 里缺的键沿用 base 的值。
// （PHP 侧 array_merge($config, null) 只发 Warning 并返回 null —— 正是 P0 缺陷的根因，
// 因此这里要求 patch 必须是映射，由调用方先行校验。）
func MergeSiteDocument(base, patch map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(patch))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range patch {
		out[k] = v
	}
	return out
}

// MarshalSiteDocument 把配置文档序列化成 YAML 字节流。
//
// 键顺序按 SiteDocumentKeyOrder 排列，其余键按字典序追加在后面，
// 保证写回的文件与 config.template.yaml 的形状一致。
func MarshalSiteDocument(doc map[string]any) ([]byte, error) {
	root, err := siteMappingNode(doc, SiteDocumentKeyOrder)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// siteMappingNode 构造一个保序的 YAML 映射节点。
func siteMappingNode(values map[string]any, order []string) (*yaml.Node, error) {
	node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	used := map[string]bool{}

	appendKey := func(key string) error {
		value, ok := values[key]
		if !ok {
			return nil
		}
		used[key] = true
		valueNode, err := siteValueNode(value, key)
		if err != nil {
			return err
		}
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
			valueNode,
		)
		return nil
	}

	for _, key := range order {
		if err := appendKey(key); err != nil {
			return nil, err
		}
	}

	rest := make([]string, 0, len(values))
	for key := range values {
		if !used[key] {
			rest = append(rest, key)
		}
	}
	sort.Strings(rest)
	for _, key := range rest {
		if err := appendKey(key); err != nil {
			return nil, err
		}
	}
	return node, nil
}

// siteValueNode 把任意值转换为 YAML 节点；嵌套映射按 SiteEmailKeyOrder 保序。
func siteValueNode(value any, key string) (*yaml.Node, error) {
	if m, ok := value.(map[string]any); ok {
		order := []string(nil)
		if key == "email" {
			order = SiteEmailKeyOrder
		}
		return siteMappingNode(m, order)
	}
	node := &yaml.Node{}
	if err := node.Encode(value); err != nil {
		return nil, err
	}
	return node, nil
}

// ValidateSiteDocument 校验合并后的配置文档是否保持模板要求的结构。
//
// 校验通过意味着写回后的 config.yaml 一定能被 www/index.php 的
// make_constants('YAML_', yaml_parse_file(config.yaml)) 正常展开 ——
// 即 P0 缺陷不会再复发。
func ValidateSiteDocument(doc map[string]any) error {
	if doc == nil {
		return ErrSiteDocumentNotMap
	}
	host, ok := doc["host"].(string)
	if !ok || strings.TrimSpace(host) == "" {
		return errors.New("host 缺失或不是非空字符串")
	}
	if _, ok := doc["ssh"].(string); !ok {
		return errors.New("ssh 缺失或不是字符串")
	}
	if _, ok := doc["allowRegister"].(bool); !ok {
		return errors.New("allowRegister 缺失或不是布尔值")
	}
	email, ok := doc["email"].(map[string]any)
	if !ok {
		return errors.New("email 缺失或不是键值映射")
	}
	if _, ok := email["name"].(string); !ok {
		return errors.New("email.name 缺失或不是字符串")
	}
	if _, ok := email["from"].(string); !ok {
		return errors.New("email.from 缺失或不是字符串")
	}
	return nil
}
