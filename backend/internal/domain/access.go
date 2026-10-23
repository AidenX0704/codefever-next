// Package domain 承载业务规则。
//
// 本包的内容全部来自 PHP 侧 application/config/access_controller.php 与
// application/libraries/service/AccessControl/AccessController.php，
// 是重写时必须**逐字搬运**的核心资产：权限矩阵的数值、角色继承链、
// git 动作映射，任何偏差都会导致越权或误拒绝。
package domain

// Permission 是权限码（PHP 侧的 UAC_* 常量）。
type Permission int

const (
	UACNoPermission Permission = 0x00

	// 仓库权限 0x01-0x07。
	UACRepoRead         Permission = 0x01
	UACRepoPush         Permission = 0x02
	UACRepoRemove       Permission = 0x03
	UACRepoChangeMember Permission = 0x04
	UACRepoChangeInfo   Permission = 0x05
	UACRepoChangeOwner  Permission = 0x06
	UACRepoChangeURL    Permission = 0x07

	// 合并请求权限 0x08-0x0B。
	UACMRRead  Permission = 0x08
	UACMRPush  Permission = 0x09
	UACMRMerge Permission = 0x0A
	UACMRClose Permission = 0x0B

	// 仓库组权限 0x10-0x15。
	UACGroupRemove       Permission = 0x10
	UACGroupChangeMember Permission = 0x11
	UACGroupCreateRepo   Permission = 0x12
	UACGroupChangeInfo   Permission = 0x13
	UACGroupChangeOwner  Permission = 0x14
	UACGroupChangeURL    Permission = 0x15

	// 仓库二级权限 0x18-0x1F。
	UACRepoBranchCreate              Permission = 0x18
	UACRepoBranchRemove              Permission = 0x19
	UACRepoTagCreate                 Permission = 0x1A
	UACRepoTagRemove                 Permission = 0x1B
	UACRepoDefaultBranchChange       Permission = 0x1C
	UACRepoProtectedBranchRuleCreate Permission = 0x1D
	UACRepoProtectedBranchRuleUpdate Permission = 0x1E
	UACRepoProtectedBranchRuleRemove Permission = 0x1F

	// Webhook 权限 0x22-0x23。
	UACRepoWebhookEdit   Permission = 0x22
	UACRepoWebhookRemove Permission = 0x23
)

// Role 是角色 ID（PHP 侧的 ROLE_* 常量）。
type Role int

const (
	RoleNoPermission Role = 0
	RoleGuest        Role = 1
	RoleReporter     Role = 2
	RoleDeveloper    Role = 3
	RoleMaintainer   Role = 4
	RoleOwner        Role = 5
	RoleNoBody       Role = 6
)

// permissionSetting 是角色 → 权限集合的映射。
//
// 逐字搬运自 application/config/access_controller.php 的 PERMISSION_SETTING。
// 注意 PHP 原版里 UAC_REPO_PROTECTED_BRANCH_RULE_REOMVE 存在拼写错误（REOMVE），
// 此处纠正为 Remove，但**数值 0x1F 保持不变**。
var permissionSetting = map[Role]map[Permission]bool{
	RoleGuest: {
		UACRepoRead: true,
	},
	RoleReporter: {
		UACRepoRead: true,
		UACMRRead:   true,
	},
	RoleDeveloper: {
		UACRepoRead: true,
		UACRepoPush: true,
		UACMRRead:   true,
		UACMRPush:   true,
	},
	RoleMaintainer: {
		UACRepoRead:          true,
		UACRepoPush:          true,
		UACRepoChangeMember:  true,
		UACRepoChangeInfo:    true,
		UACMRRead:            true,
		UACMRPush:            true,
		UACMRMerge:           true,
		UACMRClose:           true,
		UACGroupChangeMember: true,
		UACGroupCreateRepo:   true,
		UACGroupChangeInfo:   true,

		UACRepoBranchCreate:              true,
		UACRepoBranchRemove:              true,
		UACRepoTagCreate:                 true,
		UACRepoTagRemove:                 true,
		UACRepoDefaultBranchChange:       true,
		UACRepoProtectedBranchRuleCreate: true,
		UACRepoProtectedBranchRuleUpdate: true,
		UACRepoProtectedBranchRuleRemove: true,

		UACRepoWebhookEdit:   true,
		UACRepoWebhookRemove: true,
	},
	RoleOwner: {
		UACRepoRead:          true,
		UACRepoPush:          true,
		UACRepoRemove:        true,
		UACRepoChangeMember:  true,
		UACRepoChangeInfo:    true,
		UACRepoChangeOwner:   true,
		UACRepoChangeURL:     true,
		UACMRRead:            true,
		UACMRPush:            true,
		UACMRMerge:           true,
		UACMRClose:           true,
		UACGroupRemove:       true,
		UACGroupChangeMember: true,
		UACGroupCreateRepo:   true,
		UACGroupChangeInfo:   true,
		UACGroupChangeOwner:  true,
		UACGroupChangeURL:    true,

		UACRepoBranchCreate:              true,
		UACRepoBranchRemove:              true,
		UACRepoTagCreate:                 true,
		UACRepoTagRemove:                 true,
		UACRepoDefaultBranchChange:       true,
		UACRepoProtectedBranchRuleCreate: true,
		UACRepoProtectedBranchRuleUpdate: true,
		UACRepoProtectedBranchRuleRemove: true,

		UACRepoWebhookEdit:   true,
		UACRepoWebhookRemove: true,
	},
	RoleNoBody: {},
}

// repoActionMapping 把 git 动作映射到权限码。
//
// 逐字搬运自 REPO_ACTION_MAPPING。空字符串表示"普通读取"。
var repoActionMapping = map[string]Permission{
	"":                 UACRepoRead,
	"git-upload-pack":  UACRepoRead,
	"git-receive-pack": UACRepoPush,
}

// CheckPermission 判断角色是否具备某权限。
//
// 等价于 PHP 侧 AccessController::checkPermission()，包括它"权限码与角色都非 0"
// 的前置条件 —— 权限码 0 或角色 0 一律返回 false。
func CheckPermission(perm Permission, role Role) bool {
	if perm == UACNoPermission || role == RoleNoPermission {
		return false
	}
	perms, ok := permissionSetting[role]
	if !ok {
		return false
	}
	return perms[perm]
}

// CheckRepositoryAction 判断角色是否具备某 git 动作对应的权限。
//
// 等价于 PHP 侧 AccessController::checkRepositoryAction()。
// PHP 侧对未知动作会产生 undefined index notice 并按 null 处理（判为无权限），
// 此处显式返回 false。
func CheckRepositoryAction(role Role, action string) bool {
	if role == RoleNoPermission {
		return false
	}
	perm, ok := repoActionMapping[action]
	if !ok {
		return false
	}
	perms, ok := permissionSetting[role]
	if !ok {
		return false
	}
	return perms[perm]
}

// PermissionCodes 返回某角色拥有的全部权限码（按数值升序），用于自检与文档生成。
func PermissionCodes(role Role) []Permission {
	perms, ok := permissionSetting[role]
	if !ok {
		return nil
	}
	out := make([]Permission, 0, len(perms))
	for p := range perms {
		out = append(out, p)
	}
	// 简单插入排序，权限码数量很少，无需引入 sort 泛型技巧。
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
