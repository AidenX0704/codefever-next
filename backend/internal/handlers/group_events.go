// 仓库组域的**事务接入**与**事件载荷构造**。
//
// PHP 侧 application/controllers/api/group.php 的 9 个写端点在业务写入
// **之后**同步调用 $this->service->newEvent(...)
// （application/libraries/Service.php:19），事件在请求线程里就地派发。
//
// 新实现把事件改为与业务写入**同一个事务**入队（cc_events 表），
// 由后台 worker 异步消费，从而保证：
//
//	业务提交 → 事件必在；业务回滚 → 事件消失。
//
// 本文件负责三件事：
//
//  1. GroupStore / GroupTx：仓库组域的读写抽象（便于单元测试替换成假实现）；
//  2. 事件载荷构造：逐字对应 PHP 的 newEvent(...) 调用点；
//  3. 事务接入：把业务写入与事件入队绑到同一个 *sql.Tx 上。
package handlers

import (
	"context"
	"database/sql"

	"codefever-next/backend/internal/domain"
	"codefever-next/backend/internal/repo"
)

// ---------------------------------------------------------------------------
// 读写抽象
// ---------------------------------------------------------------------------

// GroupStore 是仓库组域的读写抽象。
//
// 抽成接口有两个目的：
//
//  1. 单元测试可以注入假的实现，覆盖"事件入队失败要回滚业务写入"这类分支；
//  2. 把"哪些操作必须在事务里"显式化 —— 处理器拿到的永远是事务内的仓储。
type GroupStore interface {
	// WithTx 在一个事务里执行 fn。
	//
	// fn 收到两样东西：绑定到该事务的仓储（tx）与**同一个事务对象**（raw）。
	// 事件用 raw 入队，从而保证"业务提交则事件在、业务回滚则事件消失"。
	WithTx(ctx context.Context, fn func(tx GroupTx, raw *sql.Tx) error) error

	// 只读查询
	Get(ctx context.Context, gKey string) (*domain.Group, error)
	ListOwned(ctx context.Context, uKey string) ([]domain.Group, error)
	ListJoined(ctx context.Context, uKey string) ([]domain.Group, error)
	ListMembers(ctx context.Context, gKey string) ([]domain.GroupMember, error)
	SearchByName(ctx context.Context, name string) (*domain.Group, error)
	SearchByDisplayName(ctx context.Context, displayName string) (*domain.Group, error)
	MergeRequests(ctx context.Context, gKey string, status int, keyword, sort string,
		page, pagesize int) ([]domain.MergeRequest, error)
	MergeRequestReviewers(ctx context.Context, mrKey string) ([]domain.MergeRequestReviewer, error)
	Activities(ctx context.Context, gKey string, category int, keyword string,
		page, pagesize int) ([]domain.Activity, error)
	RepositoriesInGroup(ctx context.Context, gKey string) (int, error)
}

// GroupTx 是**事务内**可用的仓库组写操作集合。
//
// 与 GroupStore 的读方法分开，是为了在类型层面表达"写操作必须带着事务"。
type GroupTx interface {
	Create(ctx context.Context, g *domain.Group) error
	Update(ctx context.Context, gKey string, fields map[string]any) error
	AddMember(ctx context.Context, uKey, gKey string) error
	SetMemberRole(ctx context.Context, uKey, gKey string, role int) error
	RemoveMember(ctx context.Context, uKey, gKey string) error
	RemoveGroup(ctx context.Context, gKey string) error
}

// GroupGuard 是仓库组域用到的权限校验集合。
//
// *Guard 天然满足该接口。
type GroupGuard interface {
	RequireGroup(ctx context.Context, gKey, uKey string, perm domain.Permission) error
	GroupRole(ctx context.Context, gKey, uKey string) (domain.Role, error)
}

// GroupUsers 是仓库组域用到的用户查询集合。
//
// *repo.UserRepo 天然满足该接口。
type GroupUsers interface {
	Get(ctx context.Context, uKey string) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	CommitEmails(ctx context.Context, uKey string, isCheck bool) ([]domain.CommitEmail, error)
}

// ---------------------------------------------------------------------------
// 仓储适配
// ---------------------------------------------------------------------------

// repoGroupStore 用 repo.GroupRepo 实现 GroupStore。
type repoGroupStore struct {
	groups *repo.GroupRepo
}

// newRepoGroupStore 创建基于仓储的仓库组读写实现。
func newRepoGroupStore(groups *repo.GroupRepo) GroupStore {
	return &repoGroupStore{groups: groups}
}

// WithTx 实现 GroupStore。
//
// 事务由仓储开启（repo.GroupRepo.WithTx），fn 返回错误即回滚 —— 事件入队
// 失败时业务写入同样回滚，两者不会分叉。
func (s *repoGroupStore) WithTx(ctx context.Context, fn func(tx GroupTx, raw *sql.Tx) error) error {
	return s.groups.WithTxBoth(ctx, func(groups *repo.GroupRepo, raw *sql.Tx) error {
		return fn(&repoGroupTx{groups: groups}, raw)
	})
}

// Get 实现 GroupStore。
func (s *repoGroupStore) Get(ctx context.Context, gKey string) (*domain.Group, error) {
	return s.groups.Get(ctx, gKey)
}

// ListOwned 实现 GroupStore。
func (s *repoGroupStore) ListOwned(ctx context.Context, uKey string) ([]domain.Group, error) {
	return s.groups.ListOwned(ctx, uKey)
}

// ListJoined 实现 GroupStore。
func (s *repoGroupStore) ListJoined(ctx context.Context, uKey string) ([]domain.Group, error) {
	return s.groups.ListJoined(ctx, uKey)
}

// ListMembers 实现 GroupStore。
func (s *repoGroupStore) ListMembers(ctx context.Context, gKey string) ([]domain.GroupMember, error) {
	return s.groups.ListMembers(ctx, gKey)
}

// SearchByName 实现 GroupStore。
func (s *repoGroupStore) SearchByName(ctx context.Context, name string) (*domain.Group, error) {
	return s.groups.SearchByName(ctx, name)
}

// SearchByDisplayName 实现 GroupStore。
func (s *repoGroupStore) SearchByDisplayName(ctx context.Context, displayName string) (*domain.Group, error) {
	return s.groups.SearchByDisplayName(ctx, displayName)
}

// MergeRequests 实现 GroupStore。
func (s *repoGroupStore) MergeRequests(ctx context.Context, gKey string, status int,
	keyword, sort string, page, pagesize int) ([]domain.MergeRequest, error) {
	return s.groups.MergeRequests(ctx, gKey, status, keyword, sort, page, pagesize)
}

// MergeRequestReviewers 实现 GroupStore。
func (s *repoGroupStore) MergeRequestReviewers(ctx context.Context, mrKey string) ([]domain.MergeRequestReviewer, error) {
	return s.groups.MergeRequestReviewers(ctx, mrKey)
}

// Activities 实现 GroupStore。
func (s *repoGroupStore) Activities(ctx context.Context, gKey string, category int,
	keyword string, page, pagesize int) ([]domain.Activity, error) {
	return s.groups.Activities(ctx, gKey, category, keyword, page, pagesize)
}

// RepositoriesInGroup 实现 GroupStore。
func (s *repoGroupStore) RepositoriesInGroup(ctx context.Context, gKey string) (int, error) {
	return s.groups.RepositoriesInGroup(ctx, gKey)
}

// repoGroupTx 把事务内的 repo.GroupRepo 适配成 GroupTx。
type repoGroupTx struct {
	groups *repo.GroupRepo
}

// Create 实现 GroupTx。
func (t *repoGroupTx) Create(ctx context.Context, g *domain.Group) error {
	return t.groups.Create(ctx, g)
}

// Update 实现 GroupTx。
func (t *repoGroupTx) Update(ctx context.Context, gKey string, fields map[string]any) error {
	return t.groups.Update(ctx, gKey, fields)
}

// AddMember 实现 GroupTx。
func (t *repoGroupTx) AddMember(ctx context.Context, uKey, gKey string) error {
	return t.groups.AddMember(ctx, uKey, gKey)
}

// SetMemberRole 实现 GroupTx。
func (t *repoGroupTx) SetMemberRole(ctx context.Context, uKey, gKey string, role int) error {
	return t.groups.SetMemberRole(ctx, uKey, gKey, role)
}

// RemoveMember 实现 GroupTx。
func (t *repoGroupTx) RemoveMember(ctx context.Context, uKey, gKey string) error {
	return t.groups.RemoveMember(ctx, uKey, gKey)
}

// RemoveGroup 实现 GroupTx。
func (t *repoGroupTx) RemoveGroup(ctx context.Context, gKey string) error {
	return t.groups.RemoveGroup(ctx, gKey)
}

// ---------------------------------------------------------------------------
// 事件载荷
// ---------------------------------------------------------------------------

// groupEvent 是一次待发射的仓库组事件。
type groupEvent struct {
	// Type 是事件类型（domain.EventType*）。
	Type domain.EventType
	// Desc 是事件数据描述（domain.EventDataDesc*_D）。
	Desc domain.EventDataDesc
	// Data 是事件载荷：字段名与类型由 Desc 决定。
	Data map[string]any
}

// groupCreateEvent 构造 GROUP_CREATE 事件。
//
// 逐字对应 application/controllers/api/group.php:79-82：
//
//	newEvent('GROUP_CREATE', ['gKey' => $gKey, 'name' => $displayName], $uKey)
func groupCreateEvent(gKey, displayName string) groupEvent {
	return groupEvent{
		Type: domain.EventTypeGROUP_CREATE,
		Desc: domain.EventDataDescGROUP_CREATE_D,
		Data: map[string]any{
			"gKey": gKey,
			"name": displayName,
		},
	}
}

// groupUpdateAvatarEvent 构造 GROUP_UPDATE_AVATAR 事件。
//
// 逐字对应 application/controllers/api/group.php:212-216：
//
//	newEvent('GROUP_UPDATE_AVATAR',
//	    ['gKey' => $gKey, 'from' => $oldAvatar, 'to' => $key], $uKey)
func groupUpdateAvatarEvent(gKey, oldAvatar, newAvatar string) groupEvent {
	return groupEvent{
		Type: domain.EventTypeGROUP_UPDATE_AVATAR,
		Desc: domain.EventDataDescGROUP_UPDATE_AVATAR_D,
		Data: map[string]any{
			"gKey": gKey,
			"from": oldAvatar,
			"to":   newAvatar,
		},
	}
}

// groupUpdateNameEvent 构造 GROUP_UPDATE_NAME 事件。
//
// 逐字对应 application/controllers/api/group.php:255-259：
//
//	newEvent('GROUP_UPDATE_NAME',
//	    ['gKey' => $gKey, 'from' => $oldName, 'to' => $displayName], $uKey)
func groupUpdateNameEvent(gKey, oldName, newName string) groupEvent {
	return groupEvent{
		Type: domain.EventTypeGROUP_UPDATE_NAME,
		Desc: domain.EventDataDescGROUP_UPDATE_NAME_D,
		Data: map[string]any{
			"gKey": gKey,
			"from": oldName,
			"to":   newName,
		},
	}
}

// groupUpdateDescriptionEvent 构造 GROUP_UPDATE_DESCRIPTION 事件。
//
// 逐字对应 application/controllers/api/group.php:263-265：
//
//	newEvent('GROUP_UPDATE_DESCRIPTION', ['gKey' => $gKey], $uKey)
func groupUpdateDescriptionEvent(gKey string) groupEvent {
	return groupEvent{
		Type: domain.EventTypeGROUP_UPDATE_DESCRIPTION,
		Desc: domain.EventDataDescGROUP_UPDATE_DESCRIPTION_D,
		Data: map[string]any{
			"gKey": gKey,
		},
	}
}

// groupAddMemberEvent 构造 GROUP_ADD_MEMBER 事件。
//
// 逐字对应 application/controllers/api/group.php:308-313：
//
//	newEvent('GROUP_ADD_MEMBER',
//	    ['gKey' => $gKey, 'uid' => $uKey, 'name' => $name, 'email' => $email], $uKey)
func groupAddMemberEvent(gKey, uid, name, email string) groupEvent {
	return groupEvent{
		Type: domain.EventTypeGROUP_ADD_MEMBER,
		Desc: domain.EventDataDescGROUP_ADD_MEMBER_D,
		Data: map[string]any{
			"gKey":  gKey,
			"uid":   uid,
			"name":  name,
			"email": email,
		},
	}
}

// groupChangeMemberRoleEvent 构造 GROUP_CHANGE_MEMBER_ROLE 事件。
//
// 逐字对应 application/controllers/api/group.php:360-366：
//
//	newEvent('GROUP_CHANGE_MEMBER_ROLE',
//	    ['gKey' => $gKey, 'uid' => $uKey, 'name' => $name, 'email' => $email,
//	     'to' => (int) $roleID], $uKey)
func groupChangeMemberRoleEvent(gKey, uid, name, email string, role int) groupEvent {
	return groupEvent{
		Type: domain.EventTypeGROUP_CHANGE_MEMBER_ROLE,
		Desc: domain.EventDataDescGROUP_CHANGE_MEMBER_ROLE_D,
		Data: map[string]any{
			"gKey":  gKey,
			"uid":   uid,
			"name":  name,
			"email": email,
			"to":    role,
		},
	}
}

// groupRemoveMemberEvent 构造 GROUP_REMOVE_MEMBER 事件。
//
// 逐字对应 application/controllers/api/group.php:407-412：
//
//	newEvent('GROUP_REMOVE_MEMBER',
//	    ['gKey' => $gKey, 'uid' => $uKey, 'name' => $name, 'email' => $email], $uKey)
func groupRemoveMemberEvent(gKey, uid, name, email string) groupEvent {
	return groupEvent{
		Type: domain.EventTypeGROUP_REMOVE_MEMBER,
		Desc: domain.EventDataDescGROUP_REMOVE_MEMBER_D,
		Data: map[string]any{
			"gKey":  gKey,
			"uid":   uid,
			"name":  name,
			"email": email,
		},
	}
}

// groupChangeOwnerEvent 构造 GROUP_CHANGE_OWNER 事件。
//
// 逐字对应 application/controllers/api/group.php:470-475：
//
//	newEvent('GROUP_CHANGE_OWNER',
//	    ['gKey' => $gKey, 'uid' => $uKey, 'name' => $name, 'email' => $email], $uKey)
//
// 注意 uid / name / email 是**新创建者**的信息（PHP 侧的 $memberInfo）。
func groupChangeOwnerEvent(gKey, uid, name, email string) groupEvent {
	return groupEvent{
		Type: domain.EventTypeGROUP_CHANGE_OWNER,
		Desc: domain.EventDataDescGROUP_CHANGE_OWNER_D,
		Data: map[string]any{
			"gKey":  gKey,
			"uid":   uid,
			"name":  name,
			"email": email,
		},
	}
}

// groupChangeURLEvent 构造 GROUP_CHANGE_URL 事件。
//
// 逐字对应 application/controllers/api/group.php:516-520：
//
//	newEvent('GROUP_CHANGE_URL',
//	    ['gKey' => $gKey, 'from' => $oldName, 'to' => $name], $uKey)
func groupChangeURLEvent(gKey, oldName, newName string) groupEvent {
	return groupEvent{
		Type: domain.EventTypeGROUP_CHANGE_URL,
		Desc: domain.EventDataDescGROUP_CHANGE_URL_D,
		Data: map[string]any{
			"gKey": gKey,
			"from": oldName,
			"to":   newName,
		},
	}
}

// groupRemoveEvent 构造 GROUP_REMOVE 事件。
//
// 逐字对应 application/controllers/api/group.php:560-563：
//
//	newEvent('GROUP_REMOVE', ['gKey' => $gKey, 'name' => $displayName], $uKey)
func groupRemoveEvent(gKey, displayName string) groupEvent {
	return groupEvent{
		Type: domain.EventTypeGROUP_REMOVE,
		Desc: domain.EventDataDescGROUP_REMOVE_D,
		Data: map[string]any{
			"gKey": gKey,
			"name": displayName,
		},
	}
}

// emit 把仓库组事件发到事件总线。
//
// raw 必须是产生该事件的业务写入所在的**同一个事务**，从而保证
// "业务提交则事件在、业务回滚则事件消失"。
//
// Bus 为 nil 时静默跳过（与仓库写端点一致，仅用于单元测试）。
func (h *GroupHandler) emit(ctx context.Context, raw *sql.Tx, ev groupEvent, generatedBy string) error {
	if h.Bus == nil || raw == nil {
		return nil
	}
	return h.Bus.Emit(ctx, raw, ev.Type, ev.Desc, ev.Data, generatedBy)
}
