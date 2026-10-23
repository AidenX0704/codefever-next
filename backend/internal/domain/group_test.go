package domain

import (
	"testing"
	"time"
)

// TestNormalizeGroupMatchesPHP 校验仓库组归一化与 PHP 逐字一致。
//
// 基线取自 backend/testdata/contract/group.list.json 的 raw_body：
// updated 是 MySQL 零值日期 '0000-00-00 00:00:00' 在 PHP 下的 strtotime 结果
// -62169984000（不是 Go 的 time.Time 零值 -62135596800）。
func TestNormalizeGroupMatchesPHP(t *testing.T) {
	avatar := "avatar-key"
	g := &Group{
		Key:         "29efb28ef9dd3018353eb5a02c018b28",
		OwnerKey:    "00000000000000000000000000000000",
		Type:        2,
		DisplayName: "Root Team",
		Name:        "rootteam",
		Description: "local verification group",
		Avatar:      &avatar,
		Created:     time.Unix(1792682255, 0).UTC(),
		Updated:     time.Unix(PHPZeroDateTimestamp, 0).UTC(),
		Status:      1,
	}

	got := NormalizeGroup(g)
	if got.ID != g.Key {
		t.Errorf("id = %q, want %q", got.ID, g.Key)
	}
	if got.DisplayName != "Root Team" {
		t.Errorf("displayName = %q, want %q", got.DisplayName, "Root Team")
	}
	if got.Name != "rootteam" {
		t.Errorf("name = %q, want %q", got.Name, "rootteam")
	}
	if got.Description != "local verification group" {
		t.Errorf("description = %q, want %q", got.Description, "local verification group")
	}
	if got.Owner != "00000000000000000000000000000000" {
		t.Errorf("owner = %q, want %q", got.Owner, "00000000000000000000000000000000")
	}
	if got.Type != 2 {
		t.Errorf("type = %d, want 2", got.Type)
	}
	if got.Status != 1 {
		t.Errorf("status = %d, want 1", got.Status)
	}
	if got.Created != 1792682255 {
		t.Errorf("created = %d, want 1792682255", got.Created)
	}
	// PHP 的 strtotime('0000-00-00 00:00:00')。
	if got.Updated != -62169984000 {
		t.Errorf("updated = %d, want -62169984000", got.Updated)
	}
	if got.Icon == nil || *got.Icon != avatar {
		t.Errorf("icon = %v, want %q", got.Icon, avatar)
	}
}

// TestPHPTimestamp 校验 MySQL 时间到 PHP strtotime 的换算。
//
// 驱动按本地时区解析时间列，PHP 侧按 UTC 解释 —— 因此取"墙上时钟"按 UTC 重新解释；
// 零值日期单独映射为 PHP 的 -62169984000。
func TestPHPTimestamp(t *testing.T) {
	cases := []struct {
		name string
		in   time.Time
		want int64
	}{
		{"零值日期", time.Time{}, -62169984000},
		{"纪元", time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC), 0},
		{"东八区的墙上时钟", time.Date(2026, 10, 22, 15, 17, 35, 0, time.FixedZone("CST", 8*3600)), 1792682255},
	}
	for _, c := range cases {
		if got := PHPTimestamp(c.in); got != c.want {
			t.Errorf("PHPTimestamp(%s) = %d, want %d", c.name, got, c.want)
		}
	}
}

// TestNormalizeGroupMemberMatchesPHP 校验成员归一化与 PHP 逐字一致。
//
// 关键点：用户的 u_role（职务）被组内角色覆盖成整数，
// creatorFlag / deleteFlag 分别标记创建者与已删除成员。
func TestNormalizeGroupMemberMatchesPHP(t *testing.T) {
	uRole := "manager"
	u := &User{
		Key:                "00000000000000000000000000000000",
		Email:              "root@codefever.cn",
		Name:               "codefever",
		Team:               nil,
		Role:               &uRole,
		NotificationStatus: 0,
		Admin:              true,
		Status:             CommonStatusNormal,
	}
	member := &GroupMember{
		Key:     "gm1",
		GKey:    "g1",
		UKey:    u.Key,
		Role:    int(RoleGuest),
		Created: time.Unix(1792682255, 0).UTC(),
		Status:  CommonStatusNormal,
	}

	got := NormalizeGroupMember(member, u, nil, "http://localhost", "git@localhost:")
	if got.Role != int(RoleGuest) {
		t.Errorf("role = %d, want %d（组内角色覆盖用户职务）", got.Role, RoleGuest)
	}
	if got.Joined != 1792682255 {
		t.Errorf("joined = %d, want 1792682255", got.Joined)
	}
	if !got.GroupMember {
		t.Error("groupMember = false, want true")
	}
	if got.CreatorFlag {
		t.Error("creatorFlag = true, want false")
	}
	if got.DeleteFlag {
		t.Error("deleteFlag = false, want false")
	}
	if got.Email != "root@codefever.cn" {
		t.Errorf("email = %q, want %q", got.Email, "root@codefever.cn")
	}
	if got.Host != "http://localhost" {
		t.Errorf("host = %q, want %q", got.Host, "http://localhost")
	}
	if got.SSH != "git@localhost:" {
		t.Errorf("ssh = %q, want %q", got.SSH, "git@localhost:")
	}
	if len(got.Emails) != 1 || got.Emails[0] != "root@codefever.cn" {
		t.Errorf("emails = %v, want [root@codefever.cn]", got.Emails)
	}

	// 已删除的成员：deleteFlag 为真。
	member.Status = 4
	if got := NormalizeGroupMember(member, u, nil, "", ""); !got.DeleteFlag {
		t.Error("deleteFlag = false, want true（gm_status != COMMON_STATUS_NORMAL）")
	}

	// 创建者：角色恒为 ROLE_OWNER，creatorFlag 为真。
	group := &Group{Key: "g1", Created: time.Unix(1792682255, 0).UTC()}
	creator := NormalizeGroupCreator(group, u, nil, "", "")
	if creator.Role != int(RoleOwner) {
		t.Errorf("creator role = %d, want %d", creator.Role, RoleOwner)
	}
	if !creator.CreatorFlag {
		t.Error("creator creatorFlag = false, want true")
	}
	if creator.Joined != 1792682255 {
		t.Errorf("creator joined = %d, want 1792682255", creator.Joined)
	}
}

// TestGroupPagingOffset 校验分页偏移量的计算。
//
// PHP 侧是 ($page - 1) * $pagesize；contract 包把 query 里的
// page/pagesize 归一化成 1/20，因此实际偏移量恒为 0。
func TestGroupPagingOffset(t *testing.T) {
	cases := []struct {
		page     int
		pagesize int
		want     int
	}{
		{1, 20, 0},
		{1, 1, 0},
		{2, 20, 20},
		{3, 10, 20},
	}
	for _, c := range cases {
		if got := GroupPagingOffset(c.page, c.pagesize); got != c.want {
			t.Errorf("GroupPagingOffset(%d, %d) = %d, want %d", c.page, c.pagesize, got, c.want)
		}
	}
}

// TestPHPIntArg 校验传给 PHP int 形参时的类型语义。
//
// 缺失（null）与非数值串都会让 PHP 8 抛 TypeError（致命错误 → HTTP 500）。
func TestPHPIntArg(t *testing.T) {
	cases := []struct {
		raw     string
		present bool
		want    int
		wantOK  bool
	}{
		{"1", true, 1, true},
		{"0", true, 0, true},
		{"20", true, 20, true},
		{"1.9", true, 1, true},
		{" 2 ", true, 2, true},
		{"", true, 0, false}, // 空串不是数值串
		{"abc", true, 0, false},
		{"1abc", true, 0, false},
		{"", false, 0, false}, // 缺失 → null → TypeError
	}
	for _, c := range cases {
		got, ok := PHPIntArg(c.raw, c.present)
		if ok != c.wantOK || got != c.want {
			t.Errorf("PHPIntArg(%q, %v) = (%d, %v), want (%d, %v)",
				c.raw, c.present, got, ok, c.want, c.wantOK)
		}
	}
}

// TestPHPStringArg 校验传给 PHP string 形参时的类型语义。
func TestPHPStringArg(t *testing.T) {
	if _, ok := PHPStringArg("", false); ok {
		t.Error("PHPStringArg 缺失参数应当触发 TypeError")
	}
	got, ok := PHPStringArg("", true)
	if !ok || got != "" {
		t.Errorf("PHPStringArg(\"\", true) = (%q, %v), want (\"\", true)", got, ok)
	}
}

// TestGroupActivityCategoryAllowed 校验动态分类参数的宽松比较语义。
func TestGroupActivityCategoryAllowed(t *testing.T) {
	cases := []struct {
		raw     string
		present bool
		want    bool
	}{
		{"1", true, true},
		{"2", true, true},
		{"3", true, true},
		{"0", true, true},  // PHP 里 "0" 是假值 → 不触发 0x0201
		{"", true, true},   // 空串同理
		{"5", true, false}, // 不在白名单里 → 0x0201
		{"abc", true, false},
		{"", false, true}, // 缺失 → 之后在模型调用处抛 TypeError
	}
	for _, c := range cases {
		if got := GroupActivityCategoryAllowed(c.raw, c.present); got != c.want {
			t.Errorf("GroupActivityCategoryAllowed(%q, %v) = %v, want %v",
				c.raw, c.present, got, c.want)
		}
	}
}

// TestGroupNameIsPreserved 校验保留 URI 判定。
func TestGroupNameIsPreserved(t *testing.T) {
	cases := map[string]bool{
		"admin":        true,
		"groups":       true,
		"static":       true,
		"static-12":    true,
		"API":          true,
		"user":         true,
		"rootteam":     false,
		"demorepo":     false,
		"administrato": false,
	}
	for input, want := range cases {
		if got := GroupNameIsPreserved(input); got != want {
			t.Errorf("GroupNameIsPreserved(%q) = %v, want %v", input, got, want)
		}
	}
}

// TestGroupActivityCategoryWhere 校验动态分类的 SQL 条件。
func TestGroupActivityCategoryWhere(t *testing.T) {
	if where, args := GroupActivityCategoryWhere(ActivityCategoryAll); where != "" || args != nil {
		t.Errorf("全部分类不应加条件，得到 (%q, %v)", where, args)
	}
	where, args := GroupActivityCategoryWhere(ActivityCategoryCommit)
	if where != "a_type IN (?, ?, ?, ?)" || len(args) != 4 {
		t.Errorf("提交分类条件 = (%q, %v)", where, args)
	}
	where, args = GroupActivityCategoryWhere(ActivityCategoryMergeRequest)
	if where != "(a_type > ? AND a_type < ?)" || len(args) != 2 {
		t.Errorf("合并请求分类条件 = (%q, %v)", where, args)
	}
	where, args = GroupActivityCategoryWhere(ActivityCategoryMember)
	if where != "(a_type < ? AND a_type NOT IN (?, ?, ?, ?))" || len(args) != 5 {
		t.Errorf("成员分类条件 = (%q, %v)", where, args)
	}
}
