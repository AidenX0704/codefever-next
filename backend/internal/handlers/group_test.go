// 仓库组域的**事件发射**单元测试。
//
// 全部用假的仓储 / 权限守卫 / 用户仓储 / 事件总线，因此不需要数据库。
// 重点覆盖：
//
//   - 10 个仓库组事件的**载荷字段**与 PHP 的 newEvent(...) 调用点逐字一致；
//   - 事件入队失败 → 业务事务回滚（数据库无残留）；
//   - 事件与业务写入在**同一个事务**里（用的是同一个事务句柄）。
//
// PHP 侧对照文件：application/controllers/api/group.php。
package handlers

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"

	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
	"codefever-next/backend/internal/httpapi"
	"codefever-next/backend/internal/repo"
)

// ---------------------------------------------------------------------------
// 假的依赖
// ---------------------------------------------------------------------------

const (
	// groupTestMemberKey 是被操作用户（成员）的 u_key。
	groupTestMemberKey = "11111111111111111111111111111111"
	// groupTestMemberMail 是被操作用户的邮箱。
	groupTestMemberMail = "member@codefever.cn"
)

// groupTxHandle 是假的 *sql.Tx —— 只用来验证"事件与业务写入用的是同一个事务句柄"。
var groupTxHandle = &sql.Tx{}

// fakeGroupStore 是假的仓库组仓储，同时充当事务内的仓储。
type fakeGroupStore struct {
	// group 是仓库组行。
	group *domain.Group
	// member 是被操作用户的行。
	member *domain.User

	// 事务状态。
	inTx       bool
	committed  bool
	rolledBack bool
	txHandle   *sql.Tx

	// 记录下来的写入（回滚时会被撤销，模拟"数据库无残留"）。
	created        []*domain.Group
	updated        []map[string]any
	addedMembers   []string
	memberRoles    []int
	removedMembers []string
	removedGroups  []string

	// failWrite 是预置的写入失败。
	failWrite error
}

// WithTx 实现 GroupStore。
func (f *fakeGroupStore) WithTx(ctx context.Context, fn func(tx GroupTx, raw *sql.Tx) error) error {
	f.inTx = true
	f.committed = false
	f.rolledBack = false
	f.txHandle = groupTxHandle

	// 记录事务开始前的写入条数：回滚时要把它们撤销（数据库无残留）。
	created, updated, added := len(f.created), len(f.updated), len(f.addedMembers)
	roles := len(f.memberRoles)
	removed, removedGroups := len(f.removedMembers), len(f.removedGroups)
	group := f.group

	err := fn(f, groupTxHandle)
	f.inTx = false
	if err != nil {
		f.rolledBack = true
		f.created = f.created[:created]
		f.updated = f.updated[:updated]
		f.addedMembers = f.addedMembers[:added]
		f.memberRoles = f.memberRoles[:roles]
		f.removedMembers = f.removedMembers[:removed]
		f.removedGroups = f.removedGroups[:removedGroups]
		f.group = group
		return err
	}
	f.committed = true
	return nil
}

// Get 实现 GroupStore。
func (f *fakeGroupStore) Get(ctx context.Context, gKey string) (*domain.Group, error) {
	if f.group == nil {
		return nil, repo.ErrNotFound
	}
	return f.group, nil
}

// ListOwned 实现 GroupStore。
func (f *fakeGroupStore) ListOwned(ctx context.Context, uKey string) ([]domain.Group, error) {
	return nil, nil
}

// ListJoined 实现 GroupStore。
func (f *fakeGroupStore) ListJoined(ctx context.Context, uKey string) ([]domain.Group, error) {
	return nil, nil
}

// ListMembers 实现 GroupStore。
func (f *fakeGroupStore) ListMembers(ctx context.Context, gKey string) ([]domain.GroupMember, error) {
	return nil, nil
}

// SearchByName 实现 GroupStore。
func (f *fakeGroupStore) SearchByName(ctx context.Context, name string) (*domain.Group, error) {
	if f.group != nil && f.group.Name == name {
		return f.group, nil
	}
	return nil, repo.ErrNotFound
}

// SearchByDisplayName 实现 GroupStore。
func (f *fakeGroupStore) SearchByDisplayName(ctx context.Context, displayName string) (*domain.Group, error) {
	if f.group != nil && f.group.DisplayName == displayName {
		return f.group, nil
	}
	return nil, repo.ErrNotFound
}

// MergeRequests 实现 GroupStore。
func (f *fakeGroupStore) MergeRequests(ctx context.Context, gKey string, status int,
	keyword, sort string, page, pagesize int) ([]domain.MergeRequest, error) {
	return nil, nil
}

// MergeRequestReviewers 实现 GroupStore。
func (f *fakeGroupStore) MergeRequestReviewers(ctx context.Context, mrKey string) ([]domain.MergeRequestReviewer, error) {
	return nil, nil
}

// Activities 实现 GroupStore。
func (f *fakeGroupStore) Activities(ctx context.Context, gKey string, category int,
	keyword string, page, pagesize int) ([]domain.Activity, error) {
	return nil, nil
}

// RepositoriesInGroup 实现 GroupStore。
func (f *fakeGroupStore) RepositoriesInGroup(ctx context.Context, gKey string) (int, error) {
	return 0, nil
}

// Create 实现 GroupTx。
func (f *fakeGroupStore) Create(ctx context.Context, g *domain.Group) error {
	if f.failWrite != nil {
		return f.failWrite
	}
	f.created = append(f.created, g)
	f.group = g
	return nil
}

// Update 实现 GroupTx。
func (f *fakeGroupStore) Update(ctx context.Context, gKey string, fields map[string]any) error {
	if f.failWrite != nil {
		return f.failWrite
	}
	f.updated = append(f.updated, fields)
	return nil
}

// AddMember 实现 GroupTx。
func (f *fakeGroupStore) AddMember(ctx context.Context, uKey, gKey string) error {
	if f.failWrite != nil {
		return f.failWrite
	}
	f.addedMembers = append(f.addedMembers, uKey)
	return nil
}

// SetMemberRole 实现 GroupTx。
func (f *fakeGroupStore) SetMemberRole(ctx context.Context, uKey, gKey string, role int) error {
	if f.failWrite != nil {
		return f.failWrite
	}
	f.memberRoles = append(f.memberRoles, role)
	return nil
}

// RemoveMember 实现 GroupTx。
func (f *fakeGroupStore) RemoveMember(ctx context.Context, uKey, gKey string) error {
	if f.failWrite != nil {
		return f.failWrite
	}
	f.removedMembers = append(f.removedMembers, uKey)
	return nil
}

// RemoveGroup 实现 GroupTx。
func (f *fakeGroupStore) RemoveGroup(ctx context.Context, gKey string) error {
	if f.failWrite != nil {
		return f.failWrite
	}
	f.removedGroups = append(f.removedGroups, gKey)
	return nil
}

// fakeGroupGuard 是假的仓库组权限守卫。
type fakeGroupGuard struct {
	denied bool
}

// RequireGroup 实现 GroupGuard。
func (g *fakeGroupGuard) RequireGroup(ctx context.Context, gKey, uKey string, perm domain.Permission) error {
	if g.denied {
		return contract.NewError(contract.CodeActionNotAllowed)
	}
	return nil
}

// GroupRole 实现 GroupGuard。
func (g *fakeGroupGuard) GroupRole(ctx context.Context, gKey, uKey string) (domain.Role, error) {
	return domain.RoleOwner, nil
}

// fakeGroupUsers 是假的用户仓储。
type fakeGroupUsers struct {
	member *domain.User
}

// Get 实现 GroupUsers。
func (u *fakeGroupUsers) Get(ctx context.Context, uKey string) (*domain.User, error) {
	if u.member != nil && u.member.Key == uKey {
		return u.member, nil
	}
	return nil, repo.ErrNotFound
}

// GetByEmail 实现 GroupUsers。
func (u *fakeGroupUsers) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	if u.member != nil && u.member.Email == email {
		return u.member, nil
	}
	return nil, repo.ErrNotFound
}

// CommitEmails 实现 GroupUsers。
func (u *fakeGroupUsers) CommitEmails(ctx context.Context, uKey string, isCheck bool) ([]domain.CommitEmail, error) {
	return nil, nil
}

// groupRecordedEvent 是被记录下来的一条事件。
type groupRecordedEvent struct {
	Type        domain.EventType
	Desc        domain.EventDataDesc
	Data        map[string]any
	GeneratedBy string
	// tx 是发射时使用的事务句柄。
	tx *sql.Tx
	// inTx 表示发射时是否处于事务作用域内。
	inTx bool
}

// fakeGroupBus 是假的事件总线。
type fakeGroupBus struct {
	store *fakeGroupStore
	// events 记录所有发出的事件。
	events []groupRecordedEvent
	// err 是预置的入队失败。
	err error
}

// Emit 实现 EventBus。
func (b *fakeGroupBus) Emit(ctx context.Context, tx *sql.Tx, eventType domain.EventType,
	desc domain.EventDataDesc, data map[string]any, generatedBy string) error {
	if b.err != nil {
		return b.err
	}
	b.events = append(b.events, groupRecordedEvent{
		Type:        eventType,
		Desc:        desc,
		Data:        data,
		GeneratedBy: generatedBy,
		tx:          tx,
		inTx:        b.store != nil && b.store.inTx,
	})
	return nil
}

// ---------------------------------------------------------------------------
// 辅助
// ---------------------------------------------------------------------------

// defaultGroupStore 返回一个装着演示仓库组与成员的假仓储。
func defaultGroupStore() *fakeGroupStore {
	return &fakeGroupStore{
		group: &domain.Group{
			Key:         testGroupKey,
			OwnerKey:    "22222222222222222222222222222222",
			Type:        domain.GroupTypeNormal,
			DisplayName: "Root Team",
			Name:        "rootteam",
			Description: "demo group",
			Status:      domain.CommonStatusNormal,
		},
		member: &domain.User{
			Key:   groupTestMemberKey,
			Name:  "Member",
			Email: groupTestMemberMail,
		},
	}
}

// newGroupTestHandler 装配一个带全部假依赖的仓库组处理器。
func newGroupTestHandler(store *fakeGroupStore, bus *fakeGroupBus) *GroupHandler {
	return &GroupHandler{
		Groups: store,
		Guard:  &fakeGroupGuard{},
		Users:  &fakeGroupUsers{member: store.member},
		Bus:    bus,
	}
}

// newGroupRequest 构造一个带 body 参数的请求。
func newGroupRequest(body map[string]any) *httpapi.Request {
	return &httpapi.Request{
		Parsed: &contract.Parsed{
			ID:     "TEST-REQUEST-ID-0000",
			Method: "POST",
			Query:  map[string]any{},
			Body:   body,
			JSON:   map[string]any{},
			Files:  map[string][]*contract.UploadedFile{},
		},
		User: &domain.User{Key: testUserKey, Email: "root@codefever.cn", Name: "root"},
	}
}

// ---------------------------------------------------------------------------
// 用例
// ---------------------------------------------------------------------------

// TestGroupEventPayloadsMatchPHP 锁定 10 个仓库组事件的载荷。
//
// 每个用例的 wantData 都是 PHP 侧 newEvent(...) 调用点里的数组**逐字**搬过来的：
//
//	application/controllers/api/group.php:79 / 212 / 255 / 263 /
//	308 / 360 / 407 / 470 / 516 / 560
func TestGroupEventPayloadsMatchPHP(t *testing.T) {
	cases := []struct {
		name     string
		ev       groupEvent
		wantType domain.EventType
		wantDesc domain.EventDataDesc
		wantData map[string]any
	}{
		{
			name:     "GROUP_CREATE",
			ev:       groupCreateEvent("gk-1", "Root Team"),
			wantType: domain.EventTypeGROUP_CREATE,
			wantDesc: domain.EventDataDescGROUP_CREATE_D,
			wantData: map[string]any{"gKey": "gk-1", "name": "Root Team"},
		},
		{
			name:     "GROUP_UPDATE_AVATAR",
			ev:       groupUpdateAvatarEvent("gk-1", "old-avatar", "new-avatar"),
			wantType: domain.EventTypeGROUP_UPDATE_AVATAR,
			wantDesc: domain.EventDataDescGROUP_UPDATE_AVATAR_D,
			wantData: map[string]any{"gKey": "gk-1", "from": "old-avatar", "to": "new-avatar"},
		},
		{
			name:     "GROUP_UPDATE_NAME",
			ev:       groupUpdateNameEvent("gk-1", "Old Name", "New Name"),
			wantType: domain.EventTypeGROUP_UPDATE_NAME,
			wantDesc: domain.EventDataDescGROUP_UPDATE_NAME_D,
			wantData: map[string]any{"gKey": "gk-1", "from": "Old Name", "to": "New Name"},
		},
		{
			name:     "GROUP_UPDATE_DESCRIPTION",
			ev:       groupUpdateDescriptionEvent("gk-1"),
			wantType: domain.EventTypeGROUP_UPDATE_DESCRIPTION,
			wantDesc: domain.EventDataDescGROUP_UPDATE_DESCRIPTION_D,
			wantData: map[string]any{"gKey": "gk-1"},
		},
		{
			name:     "GROUP_ADD_MEMBER",
			ev:       groupAddMemberEvent("gk-1", "uid-1", "Member", "member@codefever.cn"),
			wantType: domain.EventTypeGROUP_ADD_MEMBER,
			wantDesc: domain.EventDataDescGROUP_ADD_MEMBER_D,
			wantData: map[string]any{
				"gKey": "gk-1", "uid": "uid-1",
				"name": "Member", "email": "member@codefever.cn",
			},
		},
		{
			name:     "GROUP_CHANGE_MEMBER_ROLE",
			ev:       groupChangeMemberRoleEvent("gk-1", "uid-1", "Member", "member@codefever.cn", 3),
			wantType: domain.EventTypeGROUP_CHANGE_MEMBER_ROLE,
			wantDesc: domain.EventDataDescGROUP_CHANGE_MEMBER_ROLE_D,
			wantData: map[string]any{
				"gKey": "gk-1", "uid": "uid-1",
				"name": "Member", "email": "member@codefever.cn", "to": 3,
			},
		},
		{
			name:     "GROUP_REMOVE_MEMBER",
			ev:       groupRemoveMemberEvent("gk-1", "uid-1", "Member", "member@codefever.cn"),
			wantType: domain.EventTypeGROUP_REMOVE_MEMBER,
			wantDesc: domain.EventDataDescGROUP_REMOVE_MEMBER_D,
			wantData: map[string]any{
				"gKey": "gk-1", "uid": "uid-1",
				"name": "Member", "email": "member@codefever.cn",
			},
		},
		{
			name:     "GROUP_CHANGE_OWNER",
			ev:       groupChangeOwnerEvent("gk-1", "uid-1", "Member", "member@codefever.cn"),
			wantType: domain.EventTypeGROUP_CHANGE_OWNER,
			wantDesc: domain.EventDataDescGROUP_CHANGE_OWNER_D,
			wantData: map[string]any{
				"gKey": "gk-1", "uid": "uid-1",
				"name": "Member", "email": "member@codefever.cn",
			},
		},
		{
			name:     "GROUP_CHANGE_URL",
			ev:       groupChangeURLEvent("gk-1", "oldname", "newname"),
			wantType: domain.EventTypeGROUP_CHANGE_URL,
			wantDesc: domain.EventDataDescGROUP_CHANGE_URL_D,
			wantData: map[string]any{"gKey": "gk-1", "from": "oldname", "to": "newname"},
		},
		{
			name:     "GROUP_REMOVE",
			ev:       groupRemoveEvent("gk-1", "Root Team"),
			wantType: domain.EventTypeGROUP_REMOVE,
			wantDesc: domain.EventDataDescGROUP_REMOVE_D,
			wantData: map[string]any{"gKey": "gk-1", "name": "Root Team"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.ev.Type != tc.wantType {
				t.Errorf("事件类型 = %q, want %q", tc.ev.Type, tc.wantType)
			}
			if tc.ev.Desc != tc.wantDesc {
				t.Errorf("数据描述 = %q, want %q", tc.ev.Desc, tc.wantDesc)
			}
			if !reflect.DeepEqual(tc.ev.Data, tc.wantData) {
				t.Errorf("载荷 = %#v, want %#v", tc.ev.Data, tc.wantData)
			}
			// 载荷必须能通过 domain 的校验（与 PHP 的 _validateEvent 对齐）。
			if err := domain.ValidateEventData(tc.ev.Desc, domain.NormalizeEventData(tc.ev.Data)); err != nil {
				t.Fatalf("载荷未通过事件数据校验: %v", err)
			}
		})
	}
}

// TestGroupCreateEmitsEventInsideTransaction 覆盖"事件与业务写入同事务"。
func TestGroupCreateEmitsEventInsideTransaction(t *testing.T) {
	store := defaultGroupStore()
	bus := &fakeGroupBus{store: store}
	h := newGroupTestHandler(store, bus)

	if _, err := h.create(context.Background(), newGroupRequest(map[string]any{
		"displayName": "New Group",
		"name":        "newgroup",
		"type":        "1",
		"description": "desc",
	})); err != nil {
		t.Fatalf("建组失败: %v", err)
	}

	if !store.committed {
		t.Fatal("事务应当被提交")
	}
	if len(store.created) != 1 {
		t.Fatalf("应当建 1 个组，实际 %d 个", len(store.created))
	}
	if len(bus.events) != 1 {
		t.Fatalf("应当发出 1 条事件，实际 %d 条", len(bus.events))
	}

	ev := bus.events[0]
	if ev.Type != domain.EventTypeGROUP_CREATE {
		t.Errorf("事件类型 = %q, want %q", ev.Type, domain.EventTypeGROUP_CREATE)
	}
	if !ev.inTx {
		t.Error("事件必须在事务作用域内发出")
	}
	if ev.tx != store.txHandle {
		t.Error("事件必须使用与业务写入相同的事务句柄")
	}
	if ev.GeneratedBy != testUserKey {
		t.Errorf("generatedBy = %q, want %q", ev.GeneratedBy, testUserKey)
	}
	want := map[string]any{"gKey": store.created[0].Key, "name": "New Group"}
	if !reflect.DeepEqual(ev.Data, want) {
		t.Errorf("载荷 = %#v, want %#v", ev.Data, want)
	}
}

// TestGroupEmitFailureRollsBackTransaction 覆盖"事件入队失败 → 业务回滚"。
//
// 事件与业务写入同生共死：事件入不了队就不能让建组提交（数据库无残留）。
func TestGroupEmitFailureRollsBackTransaction(t *testing.T) {
	store := defaultGroupStore()
	bus := &fakeGroupBus{store: store, err: errors.New("事件入队失败")}
	h := newGroupTestHandler(store, bus)

	_, err := h.create(context.Background(), newGroupRequest(map[string]any{
		"displayName": "New Group",
		"name":        "newgroup",
		"type":        "1",
		"description": "desc",
	}))
	if got := errorCodeOf(t, err); got != int(contract.CodeOperateFail) {
		t.Fatalf("错误码 = 0x%04X, want 0x%04X", got, int(contract.CodeOperateFail))
	}
	if !store.rolledBack {
		t.Error("事务应当被回滚")
	}
	if store.committed {
		t.Error("事务不应当被提交")
	}
	if len(store.created) != 0 {
		t.Errorf("回滚后数据库不应当留下组记录，实际 %d 条", len(store.created))
	}
	if len(bus.events) != 0 {
		t.Errorf("回滚后不应留下事件，实际 %d 条", len(bus.events))
	}
}

// TestGroupUpdateEmitsConditionalEvents 覆盖"改了什么才发什么事件"。
//
// 逐字对应 application/controllers/api/group.php:254-266：
// 显示名变了才发 GROUP_UPDATE_NAME，描述变了才发 GROUP_UPDATE_DESCRIPTION。
func TestGroupUpdateEmitsConditionalEvents(t *testing.T) {
	cases := []struct {
		name        string
		displayName string
		description string
		wantTypes   []domain.EventType
	}{
		{
			name:        "都没变",
			displayName: "Root Team",
			description: "demo group",
			wantTypes:   nil,
		},
		{
			name:        "只改显示名",
			displayName: "New Team",
			description: "demo group",
			wantTypes:   []domain.EventType{domain.EventTypeGROUP_UPDATE_NAME},
		},
		{
			name:        "只改描述",
			displayName: "Root Team",
			description: "new desc",
			wantTypes:   []domain.EventType{domain.EventTypeGROUP_UPDATE_DESCRIPTION},
		},
		{
			name:        "两个都改",
			displayName: "New Team",
			description: "new desc",
			wantTypes: []domain.EventType{
				domain.EventTypeGROUP_UPDATE_NAME,
				domain.EventTypeGROUP_UPDATE_DESCRIPTION,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := defaultGroupStore()
			bus := &fakeGroupBus{store: store}
			h := newGroupTestHandler(store, bus)

			if _, err := h.update(context.Background(), newGroupRequest(map[string]any{
				"group":       testGroupKey,
				"displayName": tc.displayName,
				"description": tc.description,
			})); err != nil {
				t.Fatalf("更新失败: %v", err)
			}

			if len(bus.events) != len(tc.wantTypes) {
				t.Fatalf("应当发出 %d 条事件，实际 %d 条", len(tc.wantTypes), len(bus.events))
			}
			for i, want := range tc.wantTypes {
				if bus.events[i].Type != want {
					t.Errorf("事件[%d] 类型 = %q, want %q", i, bus.events[i].Type, want)
				}
				if bus.events[i].tx != store.txHandle {
					t.Errorf("事件[%d] 必须使用与业务写入相同的事务句柄", i)
				}
			}
			if len(tc.wantTypes) > 0 && tc.wantTypes[0] == domain.EventTypeGROUP_UPDATE_NAME {
				want := map[string]any{
					"gKey": testGroupKey, "from": "Root Team", "to": tc.displayName,
				}
				if !reflect.DeepEqual(bus.events[0].Data, want) {
					t.Errorf("GROUP_UPDATE_NAME 载荷 = %#v, want %#v", bus.events[0].Data, want)
				}
			}
			if len(tc.wantTypes) > 0 && tc.wantTypes[0] == domain.EventTypeGROUP_UPDATE_DESCRIPTION {
				want := map[string]any{"gKey": testGroupKey}
				if !reflect.DeepEqual(bus.events[0].Data, want) {
					t.Errorf("GROUP_UPDATE_DESCRIPTION 载荷 = %#v, want %#v", bus.events[0].Data, want)
				}
			}
		})
	}
}

// TestGroupMemberEvents 覆盖三个成员事件（加入 / 改角色 / 移除）。
func TestGroupMemberEvents(t *testing.T) {
	t.Run("addMember", func(t *testing.T) {
		store := defaultGroupStore()
		bus := &fakeGroupBus{store: store}
		h := newGroupTestHandler(store, bus)

		if _, err := h.addMember(context.Background(), newGroupRequest(map[string]any{
			"group": testGroupKey,
			"email": groupTestMemberMail,
		})); err != nil {
			t.Fatalf("加入成员失败: %v", err)
		}
		if len(bus.events) != 1 || bus.events[0].Type != domain.EventTypeGROUP_ADD_MEMBER {
			t.Fatalf("应当发出 GROUP_ADD_MEMBER，实际 %#v", bus.events)
		}
		want := map[string]any{
			"gKey": testGroupKey, "uid": groupTestMemberKey,
			"name": "Member", "email": groupTestMemberMail,
		}
		if !reflect.DeepEqual(bus.events[0].Data, want) {
			t.Errorf("载荷 = %#v, want %#v", bus.events[0].Data, want)
		}
		if bus.events[0].tx != store.txHandle {
			t.Error("事件必须使用与业务写入相同的事务句柄")
		}
	})

	t.Run("changeMemberRole", func(t *testing.T) {
		store := defaultGroupStore()
		bus := &fakeGroupBus{store: store}
		h := newGroupTestHandler(store, bus)

		if _, err := h.changeMemberRole(context.Background(), newGroupRequest(map[string]any{
			"group":  testGroupKey,
			"userID": groupTestMemberKey,
			"roleID": "3",
		})); err != nil {
			t.Fatalf("改角色失败: %v", err)
		}
		if len(bus.events) != 1 || bus.events[0].Type != domain.EventTypeGROUP_CHANGE_MEMBER_ROLE {
			t.Fatalf("应当发出 GROUP_CHANGE_MEMBER_ROLE，实际 %#v", bus.events)
		}
		want := map[string]any{
			"gKey": testGroupKey, "uid": groupTestMemberKey,
			"name": "Member", "email": groupTestMemberMail, "to": 3,
		}
		if !reflect.DeepEqual(bus.events[0].Data, want) {
			t.Errorf("载荷 = %#v, want %#v", bus.events[0].Data, want)
		}
	})

	t.Run("removeMember", func(t *testing.T) {
		store := defaultGroupStore()
		bus := &fakeGroupBus{store: store}
		h := newGroupTestHandler(store, bus)

		if _, err := h.removeMember(context.Background(), newGroupRequest(map[string]any{
			"group":  testGroupKey,
			"userID": groupTestMemberKey,
		})); err != nil {
			t.Fatalf("移除成员失败: %v", err)
		}
		if len(bus.events) != 1 || bus.events[0].Type != domain.EventTypeGROUP_REMOVE_MEMBER {
			t.Fatalf("应当发出 GROUP_REMOVE_MEMBER，实际 %#v", bus.events)
		}
		want := map[string]any{
			"gKey": testGroupKey, "uid": groupTestMemberKey,
			"name": "Member", "email": groupTestMemberMail,
		}
		if !reflect.DeepEqual(bus.events[0].Data, want) {
			t.Errorf("载荷 = %#v, want %#v", bus.events[0].Data, want)
		}
	})
}

// TestGroupChangeOwnerEvent 覆盖"更换拥有者"事件。
//
// 载荷里的 uid / name / email 是**新创建者**的信息（PHP 侧 $memberInfo）。
func TestGroupChangeOwnerEvent(t *testing.T) {
	store := defaultGroupStore()
	bus := &fakeGroupBus{store: store}
	h := newGroupTestHandler(store, bus)

	if _, err := h.changeOwner(context.Background(), newGroupRequest(map[string]any{
		"group":  testGroupKey,
		"userID": groupTestMemberKey,
	})); err != nil {
		t.Fatalf("更换拥有者失败: %v", err)
	}
	if len(bus.events) != 1 || bus.events[0].Type != domain.EventTypeGROUP_CHANGE_OWNER {
		t.Fatalf("应当发出 GROUP_CHANGE_OWNER，实际 %#v", bus.events)
	}
	want := map[string]any{
		"gKey": testGroupKey, "uid": groupTestMemberKey,
		"name": "Member", "email": groupTestMemberMail,
	}
	if !reflect.DeepEqual(bus.events[0].Data, want) {
		t.Errorf("载荷 = %#v, want %#v", bus.events[0].Data, want)
	}
	if bus.events[0].tx != store.txHandle {
		t.Error("事件必须使用与业务写入相同的事务句柄")
	}
	// 三步写入必须都在同一个事务里。
	if len(store.memberRoles) != 1 || len(store.updated) != 1 || len(store.removedMembers) != 1 {
		t.Errorf("应当有 1 次改角色 + 1 次改组 + 1 次移除成员，实际 %d/%d/%d",
			len(store.memberRoles), len(store.updated), len(store.removedMembers))
	}
}

// TestGroupUpdateNameEmitsChangeURLEvent 覆盖"改路径"事件。
func TestGroupUpdateNameEmitsChangeURLEvent(t *testing.T) {
	store := defaultGroupStore()
	bus := &fakeGroupBus{store: store}
	h := newGroupTestHandler(store, bus)

	if _, err := h.updateName(context.Background(), newGroupRequest(map[string]any{
		"group": testGroupKey,
		"name":  "newurl",
	})); err != nil {
		t.Fatalf("改路径失败: %v", err)
	}
	if len(bus.events) != 1 || bus.events[0].Type != domain.EventTypeGROUP_CHANGE_URL {
		t.Fatalf("应当发出 GROUP_CHANGE_URL，实际 %#v", bus.events)
	}
	want := map[string]any{"gKey": testGroupKey, "from": "rootteam", "to": "newurl"}
	if !reflect.DeepEqual(bus.events[0].Data, want) {
		t.Errorf("载荷 = %#v, want %#v", bus.events[0].Data, want)
	}
}

// TestGroupDeleteEmitsRemoveEvent 覆盖"删除仓库组"事件。
func TestGroupDeleteEmitsRemoveEvent(t *testing.T) {
	store := defaultGroupStore()
	bus := &fakeGroupBus{store: store}
	h := newGroupTestHandler(store, bus)

	if _, err := h.deleteGroup(context.Background(), newGroupRequest(map[string]any{
		"group": testGroupKey,
	})); err != nil {
		t.Fatalf("删除仓库组失败: %v", err)
	}
	if len(bus.events) != 1 || bus.events[0].Type != domain.EventTypeGROUP_REMOVE {
		t.Fatalf("应当发出 GROUP_REMOVE，实际 %#v", bus.events)
	}
	want := map[string]any{"gKey": testGroupKey, "name": "Root Team"}
	if !reflect.DeepEqual(bus.events[0].Data, want) {
		t.Errorf("载荷 = %#v, want %#v", bus.events[0].Data, want)
	}
}

// TestGroupWriteFailureDoesNotEmitEvent 覆盖"业务写入失败 → 不发事件"。
func TestGroupWriteFailureDoesNotEmitEvent(t *testing.T) {
	store := defaultGroupStore()
	store.failWrite = errors.New("写库失败")
	bus := &fakeGroupBus{store: store}
	h := newGroupTestHandler(store, bus)

	_, err := h.create(context.Background(), newGroupRequest(map[string]any{
		"displayName": "New Group",
		"name":        "newgroup",
		"type":        "1",
		"description": "desc",
	}))
	if got := errorCodeOf(t, err); got != int(contract.CodeOperateFail) {
		t.Fatalf("错误码 = 0x%04X, want 0x%04X", got, int(contract.CodeOperateFail))
	}
	if !store.rolledBack {
		t.Error("事务应当被回滚")
	}
	if len(bus.events) != 0 {
		t.Errorf("业务写入失败时不应发出事件，实际 %d 条", len(bus.events))
	}
}
