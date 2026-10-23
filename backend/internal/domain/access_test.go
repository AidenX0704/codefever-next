package domain

import (
	"testing"
)

// permissionSnapshot 是从 PHP 运行时提取的权限矩阵基线。
//
// 数据来源：容器内 `php -r` 反射 service\AccessControl\UserAccessController，
// 输出 application/config/access_controller.php 的 PERMISSION_SETTING。
//
// 这份快照是权限模型的**唯一权威基线** —— PHP 与 Go 两套实现必须逐值一致，
// 否则会出现越权（放行本该拒绝的操作）或误拒绝。
var permissionSnapshot = map[Role][]Permission{
	RoleGuest:      {0x01},
	RoleReporter:   {0x01, 0x08},
	RoleDeveloper:  {0x01, 0x02, 0x08, 0x09},
	RoleMaintainer: {0x01, 0x02, 0x04, 0x05, 0x08, 0x09, 0x0A, 0x0B, 0x11, 0x12, 0x13, 0x18, 0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F, 0x22, 0x23},
	RoleOwner:      {0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x18, 0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F, 0x22, 0x23},
	RoleNoBody:     {},
}

// TestPermissionMatrixMatchesPHP 逐值比对 Go 与 PHP 的权限矩阵。
func TestPermissionMatrixMatchesPHP(t *testing.T) {
	for role, want := range permissionSnapshot {
		got := PermissionCodes(role)
		if len(got) != len(want) {
			t.Errorf("role %d 权限数量 = %d, want %d", role, len(got), len(want))
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("role %d 第 %d 个权限 = 0x%02X, want 0x%02X", role, i+1, got[i], want[i])
			}
		}
	}
}

// TestPermissionCodeValues 锁定各权限码的数值。
func TestPermissionCodeValues(t *testing.T) {
	cases := map[string]struct {
		got  Permission
		want Permission
	}{
		"UAC_NO_PERMISSION":                     {UACNoPermission, 0x00},
		"UAC_REPO_READ":                         {UACRepoRead, 0x01},
		"UAC_REPO_PUSH":                         {UACRepoPush, 0x02},
		"UAC_REPO_REMOVE":                       {UACRepoRemove, 0x03},
		"UAC_REPO_CHANGE_MEMBER":                {UACRepoChangeMember, 0x04},
		"UAC_REPO_CHANGE_INFO":                  {UACRepoChangeInfo, 0x05},
		"UAC_REPO_CHANGE_OWNER":                 {UACRepoChangeOwner, 0x06},
		"UAC_REPO_CHANGE_URL":                   {UACRepoChangeURL, 0x07},
		"UAC_MR_READ":                           {UACMRRead, 0x08},
		"UAC_MR_PUSH":                           {UACMRPush, 0x09},
		"UAC_MR_MERGE":                          {UACMRMerge, 0x0A},
		"UAC_MR_CLOSE":                          {UACMRClose, 0x0B},
		"UAC_GROUP_REMOVE":                      {UACGroupRemove, 0x10},
		"UAC_GROUP_CHANGE_MEMBER":               {UACGroupChangeMember, 0x11},
		"UAC_GROUP_CREATE_REPO":                 {UACGroupCreateRepo, 0x12},
		"UAC_GROUP_CHANGE_INFO":                 {UACGroupChangeInfo, 0x13},
		"UAC_GROUP_CHANGE_OWNER":                {UACGroupChangeOwner, 0x14},
		"UAC_GROUP_CHANGE_URL":                  {UACGroupChangeURL, 0x15},
		"UAC_REPO_BRANCH_CREATE":                {UACRepoBranchCreate, 0x18},
		"UAC_REPO_BRANCH_REMOVE":                {UACRepoBranchRemove, 0x19},
		"UAC_REPO_TAG_CREATE":                   {UACRepoTagCreate, 0x1A},
		"UAC_REPO_TAG_REMOVE":                   {UACRepoTagRemove, 0x1B},
		"UAC_REPO_DEFAULT_BRANCH_CHANGE":        {UACRepoDefaultBranchChange, 0x1C},
		"UAC_REPO_PROTECTED_BRANCH_RULE_CREATE": {UACRepoProtectedBranchRuleCreate, 0x1D},
		"UAC_REPO_PROTECTED_BRANCH_RULE_UPDATE": {UACRepoProtectedBranchRuleUpdate, 0x1E},
		"UAC_REPO_PROTECTED_BRANCH_RULE_REOMVE": {UACRepoProtectedBranchRuleRemove, 0x1F},
		"UAC_REPO_WEBHOOK_EDIT":                 {UACRepoWebhookEdit, 0x22},
		"UAC_REPO_WEBHOOK_REMOVE":               {UACRepoWebhookRemove, 0x23},
	}
	for name, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = 0x%02X, want 0x%02X", name, int(c.got), int(c.want))
		}
	}
}

// TestRoleValues 锁定角色数值。
func TestRoleValues(t *testing.T) {
	cases := map[string]struct {
		got  Role
		want Role
	}{
		"ROLE_NO_PERMISSION": {RoleNoPermission, 0},
		"ROLE_GUEST":         {RoleGuest, 1},
		"ROLE_REPORTER":      {RoleReporter, 2},
		"ROLE_DEVELOPER":     {RoleDeveloper, 3},
		"ROLE_MAINTAINER":    {RoleMaintainer, 4},
		"ROLE_OWNER":         {RoleOwner, 5},
		"ROLE_NO_BODY":       {RoleNoBody, 6},
	}
	for name, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", name, int(c.got), int(c.want))
		}
	}
}

// TestCheckPermissionMatchesPHPSemantics 锁定 checkPermission 的判定语义。
//
// PHP 侧 AccessController::checkPermission 的实现：
//
//	if ($permissionCode && $role && in_array($role, array_keys(PERMISSION_SETTING))) {
//	    if (in_array($permissionCode, PERMISSION_SETTING[$role])) return TRUE;
//	}
//	return FALSE;
//
// 关键点：权限码为 0 或角色为 0 时直接 false。
func TestCheckPermissionMatchesPHPSemantics(t *testing.T) {
	cases := []struct {
		perm Permission
		role Role
		want bool
	}{
		{UACRepoRead, RoleGuest, true},
		{UACRepoPush, RoleGuest, false},
		{UACMRRead, RoleReporter, true},
		{UACRepoPush, RoleReporter, false},
		{UACRepoPush, RoleDeveloper, true},
		{UACMRMerge, RoleDeveloper, false},
		{UACMRMerge, RoleMaintainer, true},
		{UACRepoRemove, RoleMaintainer, false},
		{UACRepoRemove, RoleOwner, true},
		{UACGroupChangeOwner, RoleOwner, true},
		{UACGroupChangeOwner, RoleMaintainer, false},
		{UACRepoWebhookEdit, RoleMaintainer, true},
		{UACRepoWebhookEdit, RoleDeveloper, false},
		// 权限码 0 → false（PHP 的 $permissionCode && 前置条件）。
		{UACNoPermission, RoleOwner, false},
		// 角色 0 → false（PHP 的 $role && 前置条件）。
		{UACRepoRead, RoleNoPermission, false},
		// ROLE_NO_BODY 没有任何权限。
		{UACRepoRead, RoleNoBody, false},
	}
	for _, c := range cases {
		if got := CheckPermission(c.perm, c.role); got != c.want {
			t.Errorf("CheckPermission(0x%02X, role %d) = %v, want %v", int(c.perm), int(c.role), got, c.want)
		}
	}
}

// TestCheckRepositoryAction 锁定 git 动作到权限的映射与判定。
func TestCheckRepositoryAction(t *testing.T) {
	cases := []struct {
		role   Role
		action string
		want   bool
	}{
		{RoleGuest, "git-upload-pack", true},
		{RoleGuest, "git-receive-pack", false},
		{RoleDeveloper, "git-receive-pack", true},
		{RoleReporter, "git-receive-pack", false},
		{RoleGuest, "", true},
		{RoleNoPermission, "git-upload-pack", false},
		// 未知动作 → false（PHP 侧为 undefined index，按 null 处理）。
		{RoleOwner, "git-unknown-pack", false},
	}
	for _, c := range cases {
		if got := CheckRepositoryAction(c.role, c.action); got != c.want {
			t.Errorf("CheckRepositoryAction(role %d, %q) = %v, want %v", int(c.role), c.action, got, c.want)
		}
	}
}
