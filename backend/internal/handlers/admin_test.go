package handlers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codefever-next/backend/internal/config"
	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
	"codefever-next/backend/internal/httpapi"
)

// siteTemplate 是 config.template.yaml 的内容（逐字搬运）。
const adminSiteTemplate = `---
host: http://localhost
ssh: 'git@localhost:'
allowRegister: true
email:
  name: CodeFever Next
  from: git@mail.sender
...
`

// newTestAdminHandler 在指定目录里放一份合法的 config.yaml 并构造处理器。
func newTestAdminHandler(t *testing.T, dir string) *AdminHandler {
	t.Helper()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(adminSiteTemplate), 0o644); err != nil {
		t.Fatalf("写入 config.yaml 失败: %v", err)
	}
	cfg := &config.Config{BaseDir: dir}
	h, err := NewAdminHandler(cfg, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("构造处理器失败: %v", err)
	}
	return h
}

// parsedWithData 构造一个只带 data 字段的请求。
func parsedWithData(data string) *httpapi.Request {
	return &httpapi.Request{
		Parsed: &contract.Parsed{Body: map[string]any{"data": data}},
	}
}

// TestAdminSelfCheckRejectsBrokenConfig 校验启动自检拒绝不可解析 /
// 非映射的 config.yaml —— 这正是 PHP 侧 P0 缺陷留下的文件形态
// （config.yaml 变成 "--- ~"，随后登录与全部 API 永久 500）。
func TestAdminSelfCheckRejectsBrokenConfig(t *testing.T) {
	cases := map[string]string{
		"空文档":     "",
		"null 文档": "---\n~\n...\n",
		"标量文档":    "---\nhello\n...\n",
		"列表文档":    "---\n- a\n- b\n...\n",
		"非法 YAML": "host: [unclosed\n",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o644); err != nil {
				t.Fatalf("写入失败: %v", err)
			}
			cfg := &config.Config{BaseDir: dir}
			if _, err := NewAdminHandler(cfg, nil, nil, nil, nil, nil); err == nil {
				t.Fatalf("期望启动自检失败，实际通过")
			}
		})
	}
}

// TestAdminSelfCheckRejectsMissingConfig 校验缺少 config.yaml 时拒绝启动。
func TestAdminSelfCheckRejectsMissingConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{BaseDir: dir}
	if _, err := NewAdminHandler(cfg, nil, nil, nil, nil, nil); err == nil {
		t.Fatalf("期望启动自检失败，实际通过")
	}
}

// TestAdminSelfCheckAcceptsValidConfig 校验合法配置能通过启动自检。
func TestAdminSelfCheckAcceptsValidConfig(t *testing.T) {
	dir := t.TempDir()
	h := newTestAdminHandler(t, dir)
	doc := h.siteDocument()
	if doc == nil {
		t.Fatalf("内存中没有配置快照")
	}
	if (*doc)["host"] != "http://localhost" {
		t.Errorf("host = %v", (*doc)["host"])
	}
}

// TestConfigPostRejectsNullPayloadAndKeepsFile 是 P0 缺陷的回归测试：
// payload 为 null 时必须直接拒绝，且**不得修改** config.yaml。
func TestConfigPostRejectsNullPayloadAndKeepsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	original := adminSiteTemplate
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	h := newTestAdminHandler(t, dir)

	// 覆盖写入一份可识别的原始内容，便于比对"文件未被修改"。
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}

	for _, payload := range []string{"null", "", "[]", "\"text\"", "123", "{invalid"} {
		_, err := h.configPost(context.Background(), parsedWithData(payload))
		if err == nil {
			t.Fatalf("payload %q 应当被拒绝", payload)
		}
		ae := contract.AsAPIError(err)
		if ae.Code != contract.CodeMissingParameter {
			t.Errorf("payload %q 的错误码 = %d, want %d", payload, ae.Code, contract.CodeMissingParameter)
		}
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if string(after) != string(before) {
		t.Errorf("config.yaml 被修改了:\n前: %q\n后: %q", string(before), string(after))
	}
	if len(after) == 0 {
		t.Errorf("config.yaml 变成空文件（P0 缺陷的表征）")
	}

	// 校验失败时不应产生备份文件。
	if _, err := os.Stat(path + siteBackupSuffix); !os.IsNotExist(err) {
		t.Errorf("校验失败时不应生成 .bak")
	}
}

// TestConfigPostWritesAtomicallyAndBacksUp 校验正常载荷：
// 原子写入 + 生成 .bak + 热更新内存配置。
func TestConfigPostWritesAtomicallyAndBacksUp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	h := newTestAdminHandler(t, dir)

	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}

	out, err := h.configPost(context.Background(), parsedWithData(`{"host":"https://git.example.com"}`))
	if err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	doc, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("返回类型 = %T, want map", out)
	}
	if doc["host"] != "https://git.example.com" {
		t.Errorf("host = %v", doc["host"])
	}
	// 缺字段按 PHP 的 merge 语义沿用现值。
	if doc["ssh"] != "git@localhost:" {
		t.Errorf("ssh = %v, want git@localhost:", doc["ssh"])
	}
	if doc["allowRegister"] != true {
		t.Errorf("allowRegister = %v, want true", doc["allowRegister"])
	}

	// 文件已更新且可再次解析。
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	parsed, err := domain.ParseSiteDocument(updated)
	if err != nil {
		t.Fatalf("写回的文件无法解析: %v\n%s", err, string(updated))
	}
	if parsed["host"] != "https://git.example.com" {
		t.Errorf("文件里的 host = %v", parsed["host"])
	}

	// 备份保存的是写入前的内容。
	backup, err := os.ReadFile(path + siteBackupSuffix)
	if err != nil {
		t.Fatalf("读取备份失败: %v", err)
	}
	if string(backup) != string(original) {
		t.Errorf("备份内容不对:\n%s", string(backup))
	}

	// 内存配置已热更新。
	if h.Cfg.Site.Host != "https://git.example.com" {
		t.Errorf("内存配置 host = %q", h.Cfg.Site.Host)
	}
	if h.Cfg.Site.SSH != "git@localhost:" {
		t.Errorf("内存配置 ssh = %q", h.Cfg.Site.SSH)
	}
	if !h.Cfg.Site.AllowRegister {
		t.Errorf("内存配置 allowRegister = false")
	}
	if h.Cfg.Site.Email.Name != "CodeFever Next" {
		t.Errorf("内存配置 email.name = %q", h.Cfg.Site.Email.Name)
	}
}

// TestConfigPostRejectsInvalidMergedDocument 校验合并结果不满足模板结构时
// 直接拒绝且不写文件。
func TestConfigPostRejectsInvalidMergedDocument(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	h := newTestAdminHandler(t, dir)

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}

	// 浅合并会把 email 整个替换掉 → 缺 from → 结构非法。
	_, err = h.configPost(context.Background(), parsedWithData(`{"email":{"name":"X"}}`))
	if err == nil {
		t.Fatalf("期望被拒绝，实际通过")
	}
	ae := contract.AsAPIError(err)
	if ae.Code != contract.CodeMissingParameter {
		t.Errorf("错误码 = %d, want %d", ae.Code, contract.CodeMissingParameter)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if string(after) != string(before) {
		t.Errorf("config.yaml 被修改了")
	}
}

// TestWriteSiteDocumentAtomicKeepsOriginalOnBackupFailure 校验备份失败时
// 原有 config.yaml 完好无损。
func TestWriteSiteDocumentAtomicKeepsOriginalOnBackupFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	original := adminSiteTemplate
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	// 让备份路径变成目录 → 复制必然失败。
	if err := os.Mkdir(path+siteBackupSuffix, 0o755); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}

	doc := map[string]any{
		"host": "https://x", "ssh": "s", "allowRegister": true,
		"email": map[string]any{"name": "n", "from": "f"},
	}
	if err := writeSiteDocumentAtomic(path, doc); err == nil {
		t.Fatalf("期望写入失败，实际成功")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if string(got) != original {
		t.Errorf("原文件被破坏:\n%s", string(got))
	}
}

// TestWriteSiteDocumentAtomicLeavesNoTempFile 校验写入失败时不残留临时文件。
func TestWriteSiteDocumentAtomicLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	// 目标是一个目录 → rename 必然失败。
	target := filepath.Join(dir, "config.yaml")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}

	doc := map[string]any{
		"host": "https://x", "ssh": "s", "allowRegister": true,
		"email": map[string]any{"name": "n", "from": "f"},
	}
	if err := writeSiteDocumentAtomic(target, doc); err == nil {
		t.Fatalf("期望写入失败，实际成功")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读取目录失败: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".config.yaml") {
			t.Errorf("残留临时文件: %s", entry.Name())
		}
	}
}

// TestMergeAdminMembersFollowsHelper 校验成员去重与 PHP 的
// Helper::getUniqueMemberList() 一致：已存在且**旧条目是组成员**、
// 而**新条目已删除**时保留旧值；否则覆盖，位置保持首次出现。
func TestMergeAdminMembersFollowsHelper(t *testing.T) {
	groupMember := domain.NormalizedRepositoryMember{
		ID: "u1", GroupMember: true, CreatorFlag: true, DeleteFlag: false,
	}
	deletedRepoMember := domain.NormalizedRepositoryMember{
		ID: "u1", GroupMember: false, CreatorFlag: false, DeleteFlag: true,
	}
	repoCreator := domain.NormalizedRepositoryMember{
		ID: "u1", GroupMember: false, CreatorFlag: true, DeleteFlag: false,
	}

	// 组员 + 已删除的仓库成员 → 保留组员。
	got := mergeAdminMembers([]domain.NormalizedRepositoryMember{groupMember, deletedRepoMember})
	if len(got) != 1 || !got[0].GroupMember {
		t.Errorf("已删除的仓库成员不应覆盖组成员: %+v", got)
	}

	// 组员 + 仓库创建者 → 覆盖为仓库创建者（groupMember 变 false）。
	got = mergeAdminMembers([]domain.NormalizedRepositoryMember{groupMember, repoCreator})
	if len(got) != 1 || got[0].GroupMember || !got[0].CreatorFlag {
		t.Errorf("仓库创建者应覆盖组成员: %+v", got)
	}

	// 位置保持首次出现。
	got = mergeAdminMembers([]domain.NormalizedRepositoryMember{
		{ID: "u1"}, {ID: "u2"}, {ID: "u1", CreatorFlag: true},
	})
	if len(got) != 2 || got[0].ID != "u1" || got[1].ID != "u2" {
		t.Errorf("去重后顺序不对: %+v", got)
	}
}
