package handlers

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
	"codefever-next/backend/internal/gitsvc"
	"codefever-next/backend/internal/httpapi"
	"codefever-next/backend/internal/repo"
)

// ---------------------------------------------------------------------------
// 仓库写端点的单元测试
//
// 全部用假的仓储 / git / 事件总线，因此不需要数据库与 git 环境。
// 重点覆盖：
//
//   - 名称校验（分支名 / 标签名）与参数校验的错误码；
//   - 建仓 / 派生失败时的**回滚**与**磁盘清理**（PHP 侧的两处缺陷）；
//   - 事件是否与业务写入在**同一个事务**里发出；
//   - 建分支 / 建标签成功时**不再**返回 0x040C（PHP 侧的两处缺陷）。
// ---------------------------------------------------------------------------

const (
	testUserKey  = "00000000000000000000000000000000"
	testRepoKey  = "1031c9342d6539fd36ff79f75c4f8b7"
	testGroupKey = "3d5c1a1e9d1b4f0e8a2c7b6d5e4f3a2b"
)

// ---------------------------------------------------------------------------
// 假的依赖
// ---------------------------------------------------------------------------

// fakeStore 是假的仓储实现，同时充当事务内的仓储。
type fakeStore struct {
	repoRow  *domain.Repository
	groupRow *domain.RepositoryGroup

	// 事务状态。
	inTx       bool
	committed  bool
	rolledBack bool
	txHandle   *sql.Tx

	// 记录下来的写入。
	created []*domain.Repository
	updated []map[string]any
	removed []string

	// 预置的失败。
	failCreate error
}

// fakeTxHandle 是假的 *sql.Tx —— 只用来验证"事件与业务写入用的是同一个事务句柄"。
var fakeTxHandle = &sql.Tx{}

// WithTx 实现 RepositoryStore。
func (f *fakeStore) WithTx(ctx context.Context, fn func(tx repo.RepositoryTx, raw *sql.Tx) error) error {
	f.inTx = true
	f.committed = false
	f.rolledBack = false
	f.txHandle = fakeTxHandle
	err := fn(f, fakeTxHandle)
	f.inTx = false
	if err != nil {
		f.rolledBack = true
		return err
	}
	f.committed = true
	return nil
}

func (f *fakeStore) Get(ctx context.Context, rKey string) (*domain.Repository, error) {
	if f.repoRow == nil {
		return nil, repo.ErrNotFound
	}
	return f.repoRow, nil
}

func (f *fakeStore) RepositoryPath(ctx context.Context, rKey string) (string, error) {
	if f.repoRow == nil {
		return "", repo.ErrNotFound
	}
	return f.repoRow.Path, nil
}

func (f *fakeStore) SearchByName(ctx context.Context, name, gKey string) (*domain.Repository, error) {
	return nil, nil
}

func (f *fakeStore) SearchByDisplayName(ctx context.Context, displayName, gKey string) (*domain.Repository, error) {
	return nil, nil
}

func (f *fakeStore) GroupsByKeys(ctx context.Context, gKeys []string) (map[string]*domain.RepositoryGroup, error) {
	out := map[string]*domain.RepositoryGroup{}
	if f.groupRow != nil {
		out[f.groupRow.Key] = f.groupRow
	}
	return out, nil
}

func (f *fakeStore) ForkCounts(ctx context.Context, rKeys []string) (map[string]int, error) {
	return map[string]int{}, nil
}

func (f *fakeStore) MergeRequestCounts(ctx context.Context, rKeys []string) (map[string]domain.RepositoryMergeRequestCount, error) {
	return map[string]domain.RepositoryMergeRequestCount{}, nil
}

func (f *fakeStore) ListProtectedBranchRules(ctx context.Context, rKey string) ([]domain.ProtectedBranchRule, error) {
	return nil, nil
}

func (f *fakeStore) GetProtectedBranchRule(ctx context.Context, rKey, rule string) (*domain.ProtectedBranchRule, error) {
	return nil, nil
}

func (f *fakeStore) MergeRequestExists(ctx context.Context, a, b, c, d string) (bool, error) {
	return false, nil
}

func (f *fakeStore) NextMergeRequestID(ctx context.Context, targetRKey string) (int, error) {
	return 1, nil
}

func (f *fakeStore) GetMergeRequestByKey(ctx context.Context, mrKey string) (*domain.RepositoryMergeRequest, error) {
	return nil, nil
}

func (f *fakeStore) ReviewPending(ctx context.Context, mrKey string) (bool, error) {
	return false, nil
}

func (f *fakeStore) GetWebhook(ctx context.Context, rwKey string) (*domain.RepositoryWebhook, *string, error) {
	return nil, nil, nil
}

func (f *fakeStore) ListWebhooks(ctx context.Context, rKey string) ([]domain.RepositoryWebhook, map[string]*string, error) {
	return nil, nil, nil
}

func (f *fakeStore) ListWebhookLogs(ctx context.Context, rwKey string) ([]domain.RepositoryWebhookLog, error) {
	return nil, nil
}

func (f *fakeStore) GetWebhookLogData(ctx context.Context, id string) (*domain.RepositoryWebhookLog, error) {
	return nil, nil
}

// ---- 事务内的写操作 ----

func (f *fakeStore) Create(ctx context.Context, r *domain.Repository) error {
	if f.failCreate != nil {
		return f.failCreate
	}
	f.created = append(f.created, r)
	return nil
}

func (f *fakeStore) Update(ctx context.Context, rKey string, fields map[string]any) error {
	f.updated = append(f.updated, fields)
	return nil
}

func (f *fakeStore) RemoveRepository(ctx context.Context, rKey string) error {
	f.removed = append(f.removed, rKey)
	return nil
}

func (f *fakeStore) AddMember(ctx context.Context, uKey, rKey string) error { return nil }

func (f *fakeStore) SetMemberRole(ctx context.Context, uKey, rKey string, role int) error { return nil }

func (f *fakeStore) RemoveMember(ctx context.Context, uKey, rKey string) error { return nil }

func (f *fakeStore) CreateProtectedBranchRule(ctx context.Context, rKey, rule string, push, merge int) error {
	return nil
}

func (f *fakeStore) UpdateProtectedBranchRule(ctx context.Context, pbrKey string, fields map[string]any) error {
	return nil
}

func (f *fakeStore) CreateWebhook(ctx context.Context, w *domain.RepositoryWebhook) (string, error) {
	return "new-webhook", nil
}

func (f *fakeStore) UpdateWebhook(ctx context.Context, rwKey string, fields map[string]any) (int64, error) {
	return 1, nil
}

func (f *fakeStore) DeleteWebhook(ctx context.Context, rwKey string) (int64, error) { return 1, nil }

func (f *fakeStore) DeleteWebhookEventsByRwKey(ctx context.Context, rwKey string) error { return nil }

func (f *fakeStore) DeleteWebhookLogsByRwKey(ctx context.Context, rwKey string) error { return nil }

func (f *fakeStore) CreateMergeRequest(ctx context.Context, mr *domain.RepositoryMergeRequest) error {
	return nil
}

func (f *fakeStore) MergeRequestReview(ctx context.Context, mrKey, uKey string) error { return nil }

func (f *fakeStore) AssignReviewer(ctx context.Context, mrKey, uKey string) (string, error) {
	return "mrr", nil
}

func (f *fakeStore) DeleteReviewer(ctx context.Context, mrKey, uKey string) error { return nil }

func (f *fakeStore) MergeRequestClose(ctx context.Context, uKey, mrKey, s, t string) error {
	return nil
}

func (f *fakeStore) MergeRequestMerge(ctx context.Context, uKey, mrKey, s, t string) error {
	return nil
}

// fakeGit 是假的 git 实现。
type fakeGit struct {
	branches []string
	tags     []string
	// refExists 决定 RefExists 的返回值；nil 表示"都存在"。
	missingRefs map[string]bool

	initErr   error
	cloneErr  error
	branchErr error
	tagErr    error

	// 记录下来的调用。
	removedPaths    []string
	createdBranches []string
	createdTags     []string
}

func (f *fakeGit) RepoPath(rPath string) (string, error) { return "/fake/storage" + rPath, nil }

func (f *fakeGit) InitBare(ctx context.Context, repoPath string) error { return f.initErr }

func (f *fakeGit) CloneBare(ctx context.Context, repoPath, sourcePath string) error {
	return f.cloneErr
}

func (f *fakeGit) InstallHooks(ctx context.Context, repoPath string) error { return nil }

func (f *fakeGit) RemoveRepo(ctx context.Context, repoPath string) error {
	f.removedPaths = append(f.removedPaths, repoPath)
	return nil
}

func (f *fakeGit) RefExists(ctx context.Context, repoPath, ref string) (bool, error) {
	if f.missingRefs[ref] {
		return false, nil
	}
	return true, nil
}

func (f *fakeGit) Branches(ctx context.Context, repoPath string) ([]string, error) {
	return f.branches, nil
}

func (f *fakeGit) Tags(ctx context.Context, repoPath string) ([]string, error) {
	return f.tags, nil
}

func (f *fakeGit) LastCommit(ctx context.Context, repoPath, ref, path string, lastSHA ...string) (*domain.Commit, error) {
	return &domain.Commit{SHA: "0123456789abcdef0123456789abcdef01234567"}, nil
}

func (f *fakeGit) CommitListFromLocal(ctx context.Context, repoPath, source, target string) ([]domain.Commit, error) {
	return []domain.Commit{{SHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}, nil
}

func (f *fakeGit) CreateBranch(ctx context.Context, repoPath, name, origin string) error {
	if f.branchErr != nil {
		return f.branchErr
	}
	f.createdBranches = append(f.createdBranches, name)
	return nil
}

func (f *fakeGit) DeleteBranch(ctx context.Context, repoPath, name string) error { return nil }

func (f *fakeGit) CreateTag(ctx context.Context, repoPath, name, description, commitSHA, authorName, authorEmail string) error {
	if f.tagErr != nil {
		return f.tagErr
	}
	f.createdTags = append(f.createdTags, name)
	return nil
}

func (f *fakeGit) DeleteTag(ctx context.Context, repoPath, name string) error { return nil }

func (f *fakeGit) CanFastForward(ctx context.Context, sRepo, sSHA, tRepo, tSHA string) (bool, error) {
	return true, nil
}

func (f *fakeGit) MergeBranch(ctx context.Context, sRepo, sBranch, tRepo, tBranch, msg, name, email string, squash bool) (*gitsvc.MergeResult, error) {
	return &gitsvc.MergeResult{}, nil
}

func (f *fakeGit) CommitsSinceRemote(ctx context.Context, sRepo, sSHA, tRepo, tSHA string) ([]domain.Commit, error) {
	return []domain.Commit{{SHA: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}, nil
}

// fakeBus 是假的事件总线。
type fakeBus struct {
	store *fakeStore
	// events 记录所有发出的事件。
	events []recordedEvent
}

// recordedEvent 是一条被记录的事件。
type recordedEvent struct {
	Type domain.EventType
	Data map[string]any
	// tx 是发射时使用的事务句柄。
	tx *sql.Tx
	// inTx 表示发射时是否处于事务作用域内。
	inTx bool
}

func (b *fakeBus) Emit(ctx context.Context, tx *sql.Tx, eventType domain.EventType, desc domain.EventDataDesc, data map[string]any, generatedBy string) error {
	b.events = append(b.events, recordedEvent{
		Type: eventType,
		Data: data,
		tx:   tx,
		inTx: b.store != nil && b.store.inTx,
	})
	return nil
}

// fakeGuard 是假的权限守卫。
type fakeGuard struct {
	denied bool
}

func (g *fakeGuard) RequireRepository(ctx context.Context, rKey, uKey string, perm domain.Permission) error {
	if g.denied {
		return contract.NewError(contract.CodeActionNotAllowed)
	}
	return nil
}

func (g *fakeGuard) RequireGroup(ctx context.Context, gKey, uKey string, perm domain.Permission) error {
	if g.denied {
		return contract.NewError(contract.CodeActionNotAllowed)
	}
	return nil
}

func (g *fakeGuard) RepositoryRole(ctx context.Context, rKey, uKey string) (domain.Role, error) {
	return domain.RoleOwner, nil
}

// fakeUsers 是假的用户仓储。
type fakeUsers struct {
	byEmail map[string]*domain.User
}

func (u *fakeUsers) Get(ctx context.Context, uKey string) (*domain.User, error) {
	for _, item := range u.byEmail {
		if item.Key == uKey {
			return item, nil
		}
	}
	return nil, repo.ErrNotFound
}

func (u *fakeUsers) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	if item, ok := u.byEmail[email]; ok {
		return item, nil
	}
	return nil, repo.ErrNotFound
}

// ---------------------------------------------------------------------------
// 辅助
// ---------------------------------------------------------------------------

// newTestHandler 装配一个带全部假依赖的写端点处理器。
func newTestHandler(store *fakeStore, git *fakeGit, bus *fakeBus, guard *fakeGuard) *RepositoryWriteHandler {
	return NewRepositoryWriteHandler(store, &fakeUsers{byEmail: map[string]*domain.User{}}, guard, git, bus)
}

// defaultStore 返回一个装着演示仓库与仓库组的假仓储。
func defaultStore() *fakeStore {
	return &fakeStore{
		repoRow: &domain.Repository{
			Key:         testRepoKey,
			OwnerKey:    testUserKey,
			GroupKey:    testGroupKey,
			DisplayName: "Demo Repo",
			Name:        "demorepo",
			Path:        "/de7347a84f6236f06b0356ce6668c27d",
			Description: "demo",
			Status:      domain.CommonStatusNormal,
		},
		groupRow: &domain.RepositoryGroup{
			Key:         testGroupKey,
			OwnerKey:    testUserKey,
			Type:        0,
			DisplayName: "Root Team",
			Name:        "rootteam",
			Status:      domain.CommonStatusNormal,
		},
	}
}

// newRequest 构造一个带 body 参数的请求。
func newRequest(body map[string]any) *httpapi.Request {
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

// errorCodeOf 取出错误码。
func errorCodeOf(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		t.Fatal("期望有错误，实际为 nil")
	}
	apiErr := contract.AsAPIError(err)
	return int(apiErr.Code)
}

// ---------------------------------------------------------------------------
// 用例
// ---------------------------------------------------------------------------

// TestCreateRollsBackAndCleansUpOnGitFailure 覆盖【PHP 缺陷修复 ③】：
// 建仓失败必须回滚事务（不留数据库行）并清理磁盘目录。
//
// PHP 侧是"插库 → git init → 失败删行"的手工补偿，且不清理目录
// （application/models/repository_model.php:929-935）。
func TestCreateRollsBackAndCleansUpOnGitFailure(t *testing.T) {
	store := defaultStore()
	git := &fakeGit{initErr: errors.New("git init 失败")}
	bus := &fakeBus{store: store}
	h := newTestHandler(store, git, bus, &fakeGuard{})

	_, err := h.create(context.Background(), newRequest(map[string]any{
		"displayName": "New Repo",
		"name":        "newrepo",
		"group":       testGroupKey,
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
	if len(git.removedPaths) != 1 {
		t.Fatalf("应当清理 1 次磁盘目录，实际 %d 次", len(git.removedPaths))
	}
	if git.removedPaths[0] != "/fake/storage"+store.created[0].Path {
		t.Errorf("清理的目录 = %q，期望与新建仓库一致", git.removedPaths[0])
	}
	if len(bus.events) != 0 {
		t.Errorf("回滚后不应留下事件，实际 %d 条", len(bus.events))
	}
}

// TestCreateEmitsEventInsideTransaction 覆盖"事件与业务写入同事务"。
func TestCreateEmitsEventInsideTransaction(t *testing.T) {
	store := defaultStore()
	git := &fakeGit{}
	bus := &fakeBus{store: store}
	h := newTestHandler(store, git, bus, &fakeGuard{})

	if _, err := h.create(context.Background(), newRequest(map[string]any{
		"displayName": "New Repo",
		"name":        "newrepo",
		"group":       testGroupKey,
		"description": "desc",
	})); err != nil {
		t.Fatalf("建仓失败: %v", err)
	}
	if !store.committed {
		t.Fatal("事务应当被提交")
	}
	if len(bus.events) != 1 {
		t.Fatalf("应当发出 1 条事件，实际 %d 条", len(bus.events))
	}
	ev := bus.events[0]
	if ev.Type != domain.EventTypeREPO_CREATE {
		t.Errorf("事件类型 = %q, want %q", ev.Type, domain.EventTypeREPO_CREATE)
	}
	if !ev.inTx {
		t.Error("事件必须在事务作用域内发出")
	}
	if ev.tx != store.txHandle {
		t.Error("事件必须使用与业务写入相同的事务句柄")
	}
	if ev.Data["name"] != "Root Team/New Repo" {
		t.Errorf("事件 name = %v, want \"Root Team/New Repo\"", ev.Data["name"])
	}
}

// TestForkRollsBackOnCloneFailure 覆盖【PHP 缺陷修复 ④】：
// 派生失败必须回滚，不留"有数据库行、无磁盘仓库"的孤儿。
//
// PHP 侧 Repository_model::fork() 丢弃了 execCommand 的返回值
// （application/models/repository_model.php:974-976）。
func TestForkRollsBackOnCloneFailure(t *testing.T) {
	store := defaultStore()
	git := &fakeGit{cloneErr: errors.New("clone 失败")}
	bus := &fakeBus{store: store}
	h := newTestHandler(store, git, bus, &fakeGuard{})

	_, err := h.fork(context.Background(), newRequest(map[string]any{
		"displayName": "Forked",
		"name":        "forked",
		"group":       testGroupKey,
		"forkID":      testRepoKey,
		"description": "desc",
	}))
	if got := errorCodeOf(t, err); got != int(contract.CodeOperateFail) {
		t.Fatalf("错误码 = 0x%04X, want 0x%04X", got, int(contract.CodeOperateFail))
	}
	// 性能整改 C5 之后的契约：git clone 在**事务之外**执行，
	// 因此 clone 失败时根本不会开启事务（比"开了再回滚"更好：
	// 不再让一条池连接陪着 git 子进程一起等）。
	// 要保证的不变量有两条：
	//   1. 不留下仓库行（没有 created 记录，也没有事件）；
	//   2. 已经克隆出来的目录必须被清理，否则磁盘上会留下孤儿仓库。
	if store.committed {
		t.Error("clone 失败时不应提交事务")
	}
	if len(store.created) != 0 {
		t.Errorf("clone 失败时不应写入仓库行，实际 %d 条", len(store.created))
	}
	if len(git.removedPaths) != 1 {
		t.Fatalf("应当清理 1 次磁盘目录，实际 %d 次", len(git.removedPaths))
	}
	if len(bus.events) != 0 {
		t.Errorf("clone 失败时不应留下事件，实际 %d 条", len(bus.events))
	}
}

// TestCreateBranchSuccessReturnsEmptyObject 覆盖【PHP 缺陷修复 ①】：
// 建分支**成功**时返回空对象，而不是 0x040C。
//
// PHP 侧 application/controllers/api/repository.php:1442-1445 写的是
// `if ($result) { Response::reject(0x040C); }`，而 model 成功时返回 true。
func TestCreateBranchSuccessReturnsEmptyObject(t *testing.T) {
	store := defaultStore()
	git := &fakeGit{}
	bus := &fakeBus{store: store}
	h := newTestHandler(store, git, bus, &fakeGuard{})

	data, err := h.createBranch(context.Background(), newRequest(map[string]any{
		"repository": testRepoKey,
		"name":       "feature/x",
		"origin":     "master",
	}))
	if err != nil {
		t.Fatalf("建分支应当成功，实际错误: %v", err)
	}
	list, ok := data.([]any)
	if !ok || len(list) != 0 {
		t.Fatalf("响应应当是空数组，实际 %#v", data)
	}
	if len(git.createdBranches) != 1 || git.createdBranches[0] != "feature/x" {
		t.Errorf("创建的分支 = %v, want [feature/x]", git.createdBranches)
	}
	if len(bus.events) != 1 || bus.events[0].Type != domain.EventTypeBRANCH_CREATE {
		t.Errorf("应当发出 BRANCH_CREATE 事件，实际 %v", bus.events)
	}
}

// TestCreateTagSuccessReturnsEmptyObject 覆盖【PHP 缺陷修复 ①】的标签分支：
// application/controllers/api/repository.php:1952-1955 是同一处错误。
func TestCreateTagSuccessReturnsEmptyObject(t *testing.T) {
	store := defaultStore()
	git := &fakeGit{}
	bus := &fakeBus{store: store}
	h := newTestHandler(store, git, bus, &fakeGuard{})

	data, err := h.createTag(context.Background(), newRequest(map[string]any{
		"repository": testRepoKey,
		"name":       "v1.0",
		"origin":     "master",
	}))
	if err != nil {
		t.Fatalf("建标签应当成功，实际错误: %v", err)
	}
	list, ok := data.([]any)
	if !ok || len(list) != 0 {
		t.Fatalf("响应应当是空数组，实际 %#v", data)
	}
	if len(git.createdTags) != 1 || git.createdTags[0] != "v1.0" {
		t.Errorf("创建的标签 = %v, want [v1.0]", git.createdTags)
	}
}

// TestCreateBranchInvalidName 校验分支名校验的错误码是 0x0202。
func TestCreateBranchInvalidName(t *testing.T) {
	store := defaultStore()
	git := &fakeGit{}
	h := newTestHandler(store, git, &fakeBus{store: store}, &fakeGuard{})

	_, err := h.createBranch(context.Background(), newRequest(map[string]any{
		"repository": testRepoKey,
		"name":       "-bad name",
		"origin":     "master",
	}))
	if got := errorCodeOf(t, err); got != int(contract.CodeAmountOutOfRange) {
		t.Fatalf("错误码 = 0x%04X, want 0x%04X", got, int(contract.CodeAmountOutOfRange))
	}
	if len(git.createdBranches) != 0 {
		t.Error("名称非法时不应创建分支")
	}
}

// TestCreateTagInvalidName 校验标签名校验的错误码是 0x0202。
func TestCreateTagInvalidName(t *testing.T) {
	store := defaultStore()
	git := &fakeGit{}
	h := newTestHandler(store, git, &fakeBus{store: store}, &fakeGuard{})

	_, err := h.createTag(context.Background(), newRequest(map[string]any{
		"repository": testRepoKey,
		"name":       "v1.",
	}))
	if got := errorCodeOf(t, err); got != int(contract.CodeAmountOutOfRange) {
		t.Fatalf("错误码 = 0x%04X, want 0x%04X", got, int(contract.CodeAmountOutOfRange))
	}
}

// TestCreateBranchSourceRefMissing 覆盖【PHP 缺陷修复 ②】：
// 源 ref 不存在 → 0x040C（Old Target Not Exists）。
func TestCreateBranchSourceRefMissing(t *testing.T) {
	store := defaultStore()
	git := &fakeGit{missingRefs: map[string]bool{"nope": true}}
	h := newTestHandler(store, git, &fakeBus{store: store}, &fakeGuard{})

	_, err := h.createBranch(context.Background(), newRequest(map[string]any{
		"repository": testRepoKey,
		"name":       "feature/y",
		"origin":     "nope",
	}))
	if got := errorCodeOf(t, err); got != int(contract.CodeOldTargetNotExists) {
		t.Fatalf("错误码 = 0x%04X, want 0x%04X", got, int(contract.CodeOldTargetNotExists))
	}
	if len(git.createdBranches) != 0 {
		t.Error("源 ref 不存在时不应创建分支")
	}
}

// TestCreateBranchTargetExists 覆盖【PHP 缺陷修复 ②】：
// 新增目标已存在 → 0x0406（New Target Exists）。
func TestCreateBranchTargetExists(t *testing.T) {
	store := defaultStore()
	git := &fakeGit{branches: []string{"feature/z"}}
	h := newTestHandler(store, git, &fakeBus{store: store}, &fakeGuard{})

	_, err := h.createBranch(context.Background(), newRequest(map[string]any{
		"repository": testRepoKey,
		"name":       "feature/z",
		"origin":     "master",
	}))
	if got := errorCodeOf(t, err); got != int(contract.CodeTargetExists) {
		t.Fatalf("错误码 = 0x%04X, want 0x%04X", got, int(contract.CodeTargetExists))
	}
}

// TestCreateBranchTagConflict 校验标签名与已有分支冲突时也报 0x0406。
func TestCreateBranchTagConflict(t *testing.T) {
	store := defaultStore()
	git := &fakeGit{tags: []string{"v1.0"}}
	h := newTestHandler(store, git, &fakeBus{store: store}, &fakeGuard{})

	_, err := h.createTag(context.Background(), newRequest(map[string]any{
		"repository": testRepoKey,
		"name":       "v1.0",
	}))
	if got := errorCodeOf(t, err); got != int(contract.CodeTargetExists) {
		t.Fatalf("错误码 = 0x%04X, want 0x%04X", got, int(contract.CodeTargetExists))
	}
}

// TestMissingParameters 校验缺参数一律返回 0x0201。
//
// 这些路径与 backend/contract/manifest.yaml 里的契约用例一一对应。
func TestMissingParameters(t *testing.T) {
	store := defaultStore()
	git := &fakeGit{}
	h := newTestHandler(store, git, &fakeBus{store: store}, &fakeGuard{})

	cases := []struct {
		name string
		call func(ctx context.Context, req *httpapi.Request) (any, error)
	}{
		{"create", h.create},
		{"fork", h.fork},
		{"update", h.update},
		{"updateName", h.updateName},
		{"deleteRepository", h.deleteRepository},
		{"addMember", h.addMember},
		{"changeMemberRole", h.changeMemberRole},
		{"removeMember", h.removeMember},
		{"changeOwner", h.changeOwner},
		{"createBranch", h.createBranch},
		{"deleteBranch", h.deleteBranch},
		{"createTag", h.createTag},
		{"deleteTag", h.deleteTag},
		{"defaultBranch", h.defaultBranch},
		{"createProtectedBranchRule", h.createProtectedBranchRule},
		{"updateProtectedBranchRule", h.updateProtectedBranchRule},
		{"deleteProtectedBranchRule", h.deleteProtectedBranchRule},
		{"editWebhook", h.editWebhook},
		{"deleteWebhook", h.deleteWebhook},
		{"getWebhook", h.getWebhook},
		{"webhooks", h.webhooks},
		{"getRepositoryWebhookLogs", h.getRepositoryWebhookLogs},
		{"getRepositoryWebhookLogData", h.getRepositoryWebhookLogData},
		{"createMergeRequest", h.createMergeRequest},
		{"mergeRequestReview", h.mergeRequestReview},
		{"assignReviewer", h.assignReviewer},
		{"deleteReviewer", h.deleteReviewer},
		{"mergeRequestClose", h.mergeRequestClose},
		{"checkMergeType", h.checkMergeType},
		{"mergeBranch", h.mergeBranch},
	}
	for _, c := range cases {
		_, err := c.call(context.Background(), newRequest(map[string]any{}))
		if got := errorCodeOf(t, err); got != int(contract.CodeMissingParameter) {
			t.Errorf("[%s] 错误码 = 0x%04X, want 0x%04X", c.name, got, int(contract.CodeMissingParameter))
		}
	}
}

// TestPermissionDenied 校验无权限时返回 0x0106。
func TestPermissionDenied(t *testing.T) {
	store := defaultStore()
	git := &fakeGit{}
	h := newTestHandler(store, git, &fakeBus{store: store}, &fakeGuard{denied: true})

	_, err := h.createBranch(context.Background(), newRequest(map[string]any{
		"repository": testRepoKey,
		"name":       "feature/x",
		"origin":     "master",
	}))
	if got := errorCodeOf(t, err); got != int(contract.CodeActionNotAllowed) {
		t.Fatalf("错误码 = 0x%04X, want 0x%04X", got, int(contract.CodeActionNotAllowed))
	}
	if len(git.createdBranches) != 0 {
		t.Error("无权限时不应创建分支")
	}
}

// TestWriteActionsCoverContractManifest 校验写端点的动作表覆盖了
// backend/contract/manifest.yaml 里登记的全部写端点。
func TestWriteActionsCoverContractManifest(t *testing.T) {
	store := defaultStore()
	git := &fakeGit{}
	h := newTestHandler(store, git, &fakeBus{store: store}, &fakeGuard{})
	actions := h.WriteActions()

	expected := []string{
		"create", "fork", "update", "updateName", "deleteRepository", "uploadAvatar",
		"addMember", "changeMemberRole", "removeMember", "changeOwner",
		"createBranch", "deleteBranch", "createTag", "deleteTag", "defaultBranch",
		"createProtectedBranchRule", "updateProtectedBranchRule", "deleteProtectedBranchRule",
		"protectedBranchRules",
		"getWebhook", "webhooks", "editWebhook", "deleteWebhook",
		"getRepositoryWebhookLogs", "getRepositoryWebhookLogData",
		"createMergeRequest", "mergeRequestReview", "assignReviewer", "deleteReviewer",
		"mergeRequestClose", "checkMergeType", "mergeBranch",
	}
	for _, name := range expected {
		methods, ok := actions[name]
		if !ok {
			t.Errorf("动作 %q 未注册", name)
			continue
		}
		if len(methods) == 0 {
			t.Errorf("动作 %q 没有绑定任何 HTTP 方法", name)
		}
	}
	if len(actions) != len(expected) {
		t.Errorf("动作数量 = %d, want %d", len(actions), len(expected))
	}
}
