// Package handlers 实现各业务域的 API 端点。
//
// 每个 handler 只负责业务逻辑，协议解析、鉴权与响应封装由 httpapi 包统一处理。
package handlers

import (
	"context"
	"encoding/base64"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"codefever-next/backend/internal/config"
	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
	"codefever-next/backend/internal/gitsvc"
	"codefever-next/backend/internal/httpapi"
	"codefever-next/backend/internal/repo"
)

// RepositoryHandler 实现 /api/repository/* 的只读端点。
//
// 写端点（create / fork / update / delete / merge 等）在 RepositoryWriteHandler，
// 两者注册到同一个 controller（PHP 侧也是同一个文件）。
type RepositoryHandler struct {
	Repos *repo.RepositoryRepo
	Git   *gitsvc.Service
	Guard *Guard
	// Cfg 提供站点 host / ssh —— 成员归一化形态需要这两个字段
	// （PHP 侧的 YAML_HOST / YAML_SSH）。
	Cfg *config.Config
}

// RepositoryOption 是 RepositoryHandler 的可选依赖。
type RepositoryOption func(*RepositoryHandler)

// WithRepositorySiteConfig 注入站点配置。
func WithRepositorySiteConfig(cfg *config.Config) RepositoryOption {
	return func(h *RepositoryHandler) { h.Cfg = cfg }
}

// NewRepositoryHandler 创建仓库域处理器。
//
// 站点配置通过 Option 注入：成员列表需要用户归一化形态，
// 其中 host / ssh 两个字段来自站点配置。
func NewRepositoryHandler(repos *repo.RepositoryRepo, git *gitsvc.Service, guard *Guard, opts ...RepositoryOption) *RepositoryHandler {
	h := &RepositoryHandler{Repos: repos, Git: git, Guard: guard}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// Register 把仓库域端点注册到服务器。
func (h *RepositoryHandler) Register(s *httpapi.Server) {
	actions := map[string]map[string]httpapi.ActionHandler{
		"list":                    {"GET": h.list},
		"config":                  {"GET": h.config},
		"commitList":              {"GET": h.commitList},
		"commitDetail":            {"GET": h.commitDetail},
		"object":                  {"GET": h.object},
		"pathStack":               {"GET": h.pathStack},
		"branchList":              {"GET": h.branchList},
		"tagList":                 {"GET": h.tagList},
		"blameInfo":               {"GET": h.blameInfo},
		"activities":              {"GET": h.activities},
		"mergeRequests":           {"GET": h.mergeRequests},
		"relatedMergeRequests":    {"GET": h.relatedMergeRequests},
		"mergeRequestDetail":      {"GET": h.mergeRequestDetail},
		"mergeRequestVersionList": {"GET": h.mergeRequestVersionList},
		"mergeRequestCommits":     {"GET": h.mergeRequestCommits},
		"targetRepository":        {"GET": h.targetRepository},
		"lastCommitLog":           {"GET": h.lastCommitLog},
		"refListContainSHA":       {"GET": h.refListContainSHA},
		"fileChanges":             {"GET": h.fileChanges},
		"fileContent":             {"GET": h.fileContent},
	}

	s.Register(&httpapi.Controller{
		Name:    "repository",
		Auth:    httpapi.AuthWebUser,
		Actions: actions,
	})
}

// ---------------------------------------------------------------------------
// 公共辅助
// ---------------------------------------------------------------------------

// host 返回站点 host（PHP 侧的 YAML_HOST）。
func (h *RepositoryHandler) host() string {
	if h.Cfg == nil {
		return ""
	}
	return h.Cfg.SiteConfig().Host
}

// ssh 返回站点 ssh 地址（PHP 侧的 YAML_SSH）。
func (h *RepositoryHandler) ssh() string {
	if h.Cfg == nil {
		return ""
	}
	return h.Cfg.SiteConfig().SSH
}

// repoPath 解析仓库对应的裸仓库绝对路径。
func (h *RepositoryHandler) repoPath(ctx context.Context, rKey string) (string, error) {
	rPath, err := h.Repos.RepositoryPath(ctx, rKey)
	if err != nil {
		return "", err
	}
	return h.Git.RepoPath(rPath)
}

// trimSlashes 去掉首尾的 "/"（PHP 侧的 trim($path, '/')）。
func trimSlashes(s string) string { return strings.Trim(s, "/") }

// paging 读取分页参数。
//
// PHP 侧的 Request::parse() 会把 query 中的 page / pagesize 强制改写为
// page=1 / pagesize=20（因为 $_GET 的值恒为字符串，is_int 判定必然失败），
// 这里照搬该行为。
//
// 性能整改 C1（2026-10）：额外收敛到 domain.ClampPaging 的口径 ——
// 严格保留了 PHP 的"缺省 20"语义，同时补上**上限**。整改前这里完全不封顶，
// 而下游的合并请求列表对每一行都要跑一次 `git log -1` 子进程，
// 因此 `?pagesize=20000` 等价于让单个请求拉起两万个 git 进程。
func paging(req *httpapi.Request) (int, int) {
	return domain.ClampPaging(req.Parsed.QueryInt("page"), req.Parsed.QueryInt("pagesize"))
}

// ---------------------------------------------------------------------------
// repository/list
// ---------------------------------------------------------------------------

// list 实现 GET /api/repository/list。
//
// 逐字对应 application/controllers/api/repository.php:149-161：
// listJoined() → normalize() → 再补 role 与 members。
//
// PHP 的 normalize() 对每个仓库额外查 2 次 SQL（组信息、fork 数），
// list_get() 又对每个仓库查 2 次（角色、成员列表）；这里全部改为批量查询，
// 输出顺序与字段保持一致。
func (h *RepositoryHandler) list(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	if uKey == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	repos, err := h.Repos.ListJoined(ctx, uKey)
	if err != nil {
		return nil, err
	}
	if len(repos) == 0 {
		return []any{}, nil
	}

	rKeys := make([]string, 0, len(repos))
	gKeys := make([]string, 0, len(repos))
	for _, r := range repos {
		rKeys = append(rKeys, r.Key)
		gKeys = append(gKeys, r.GroupKey)
	}

	groups, err := h.Repos.GroupsByKeys(ctx, gKeys)
	if err != nil {
		return nil, err
	}
	forkCounts, err := h.Repos.ForkCounts(ctx, rKeys)
	if err != nil {
		return nil, err
	}
	mrCounts, err := h.Repos.MergeRequestCounts(ctx, rKeys)
	if err != nil {
		return nil, err
	}
	roles, err := h.Repos.RolesFor(ctx, rKeys, uKey)
	if err != nil {
		return nil, err
	}
	members, err := h.membersOf(ctx, repos, groups)
	if err != nil {
		return nil, err
	}

	out := make([]any, 0, len(repos))
	for i := range repos {
		r := repos[i]
		item := domain.RepositoryListItem{
			NormalizedRepository: domain.NormalizeRepository(
				&r, groups[r.GroupKey], forkCounts[r.Key], mrCounts[r.Key]),
			Role:    roles[r.Key],
			Members: members[r.Key],
		}
		out = append(out, item)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// repository/config
// ---------------------------------------------------------------------------

// config 实现 GET /api/repository/config。
//
// 逐字对应 application/controllers/api/repository.php:163-207。
func (h *RepositoryHandler) config(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	rKey := req.Parsed.QueryString("rKey")
	if uKey == "" || rKey == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoRead); err != nil {
		return nil, err
	}

	repoInfo, err := h.Repos.Get(ctx, rKey)
	if err != nil {
		return nil, err
	}
	groups, err := h.Repos.GroupsByKeys(ctx, []string{repoInfo.GroupKey})
	if err != nil {
		return nil, err
	}
	group := groups[repoInfo.GroupKey]

	members, err := h.membersOf(ctx, []domain.Repository{*repoInfo}, groups)
	if err != nil {
		return nil, err
	}

	repoPath, err := h.repoPath(ctx, rKey)
	if err != nil {
		return nil, err
	}

	branchNames, err := h.Git.Branches(ctx, repoPath)
	if err != nil {
		return nil, err
	}
	tagNames, err := h.Git.Tags(ctx, repoPath)
	if err != nil {
		return nil, err
	}

	branches := make([]domain.NormalizedBranchRef, 0, len(branchNames))
	for _, name := range branchNames {
		branches = append(branches, domain.NormalizedBranchRef{ID: name, Name: name})
	}
	tags := make([]domain.NormalizedBranchRef, 0, len(tagNames))
	for _, name := range tagNames {
		tags = append(tags, domain.NormalizedBranchRef{ID: name, Name: name})
	}

	// commit 计数用默认分支；没有默认分支时退回到第一个分支。
	countBranch := ""
	if repoInfo.DefaultBranch != nil {
		countBranch = *repoInfo.DefaultBranch
	}
	if countBranch == "" && len(branches) > 0 {
		countBranch = branches[0].Name
	}
	commitCount := 0
	if countBranch != "" {
		commitCount, err = h.Git.CommitCount(ctx, repoPath, countBranch)
		if err != nil {
			return nil, err
		}
	}

	objects, err := h.Git.ObjectsCount(ctx, repoPath)
	if err != nil {
		return nil, err
	}

	mrCounts, err := h.Repos.MergeRequestCounts(ctx, []string{rKey})
	if err != nil {
		return nil, err
	}
	forkCounts, err := h.Repos.ForkCounts(ctx, []string{rKey})
	if err != nil {
		return nil, err
	}

	return domain.RepositoryConfig{
		Repository: domain.NormalizeRepository(repoInfo, group, forkCounts[rKey], mrCounts[rKey]),
		Group:      domain.NormalizeRepositoryGroup(group),
		Members:    members[rKey],
		Branches:   branches,
		Tags:       tags,
		Count: domain.RepositoryConfigCount{
			Commit:       commitCount,
			Branch:       len(branches),
			Tag:          len(tags),
			File:         (objects.Size + objects.SizePack + objects.SizeGarbage) * 1024,
			MergeRequest: mrCounts[rKey],
		},
	}, nil
}

// ---------------------------------------------------------------------------
// repository/commitList
// ---------------------------------------------------------------------------

// commitList 实现 GET /api/repository/commitList。
//
// 逐字对应 application/controllers/api/repository.php:834-862。
//
// 用户输入的搜索关键词以 `--grep=<keyword>` 单个 argv 元素传给 git；
// PHP 侧是把它原样拼进 shell 字符串（Command.php:201-203），存在命令注入面。
func (h *RepositoryHandler) commitList(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	rKey := req.Parsed.QueryString("repository")
	if uKey == "" || rKey == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoRead); err != nil {
		return nil, err
	}

	revision := req.Parsed.QueryString("revision")
	keyword := req.Parsed.QueryString("keyword")
	filePath := trimSlashes(req.Parsed.QueryString("path"))
	page, pagesize := paging(req)

	repoPath, err := h.repoPath(ctx, rKey)
	if err != nil {
		return nil, err
	}

	// PHP 侧 getCommits() 会先校验 revision：必须是已存在的分支/标签，
	// 或者是一个 8 位小写十六进制短 SHA，否则直接返回空列表。
	// 这里照搬该校验（它同时挡住了以 "-" 开头的选项注入）。
	exists, err := h.revisionExists(ctx, repoPath, revision)
	if err != nil {
		return nil, err
	}
	if !exists {
		return []any{}, nil
	}

	commits, err := h.Git.Commits(ctx, repoPath, revision, filePath, keyword, page, pagesize)
	if err != nil {
		return nil, err
	}
	out := make([]any, 0, len(commits))
	for _, c := range commits {
		out = append(out, c)
	}
	return out, nil
}

// revisionExists 判断 revision 是否是已存在的分支/标签或 8 位短 SHA。
//
// 逐字对应 application/models/repository_model.php:1609-1612。
func (h *RepositoryHandler) revisionExists(ctx context.Context, repoPath, revision string) (bool, error) {
	if revision == "" {
		return false, nil
	}
	branches, err := h.Git.Branches(ctx, repoPath)
	if err != nil {
		return false, err
	}
	tags, err := h.Git.Tags(ctx, repoPath)
	if err != nil {
		return false, err
	}
	for _, name := range branches {
		if name == revision {
			return true, nil
		}
	}
	for _, name := range tags {
		if name == revision {
			return true, nil
		}
	}
	return shortSHARe.MatchString(revision), nil
}

// shortSHARe 对应 PHP 的 preg_match('/^[a-z0-9]{8}$/', $revision)。
var shortSHARe = regexp.MustCompile(`^[a-z0-9]{8}$`)

// ---------------------------------------------------------------------------
// repository/commitDetail
// ---------------------------------------------------------------------------

// commitDetail 实现 GET /api/repository/commitDetail。
//
// 逐字对应 application/controllers/api/repository.php:864-877。
//
// 【有意差异 —— 安全修复】PHP 侧该端点**没有资源级鉴权**，
// 任意登录用户都能读任意仓库的提交详情。这里补上 UAC_REPO_READ 校验，
// 仓库不存在或无权限时返回 0x0106。
func (h *RepositoryHandler) commitDetail(ctx context.Context, req *httpapi.Request) (any, error) {
	rKey := req.Parsed.QueryString("repository")
	if rKey == "" {
		return nil, contract.NewError(contract.CodeEmpty)
	}
	uKey := req.UserKey()
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoRead); err != nil {
		return nil, err
	}

	commitSHA := req.Parsed.QueryString("commitSHA")
	if commitSHA == "" {
		// PHP 侧 getCommitDetail() 返回 FALSE，normalizeCommitDetail() 会把
		// 四个字段读成 null、time 读成 0。
		return map[string]any{
			"sha": nil, "commit": nil, "parent": nil, "email": nil, "time": 0,
		}, nil
	}

	repoPath, err := h.repoPath(ctx, rKey)
	if err != nil {
		return nil, err
	}
	detail, err := h.Git.CommitDetail(ctx, repoPath, commitSHA)
	if err != nil {
		return nil, err
	}
	return detail, nil
}

// ---------------------------------------------------------------------------
// repository/object
// ---------------------------------------------------------------------------

// object 实现 GET /api/repository/object。
//
// 逐字对应 application/controllers/api/repository.php:1053-1144。
func (h *RepositoryHandler) object(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	rKey := req.Parsed.QueryString("repository")
	parent := req.Parsed.QueryString("parent")
	if rKey == "" || parent == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoRead); err != nil {
		return nil, err
	}

	filePath := trimSlashes(req.Parsed.QueryString("path"))
	rev := req.Parsed.QueryString("revision")

	repoPath, err := h.repoPath(ctx, rKey)
	if err != nil {
		return nil, err
	}
	obj, err := h.Git.Object(ctx, repoPath, parent)
	if err != nil {
		return nil, err
	}

	view := domain.ObjectView{Type: obj.Type}
	if obj.Type == "blob" {
		view.Object = parseObjectToFile(obj.Raw)
	} else {
		view.Object = obj.Entries
	}

	if rev != "" {
		commit, cerr := h.Git.LastCommit(ctx, repoPath, rev, filePath)
		if cerr != nil {
			return nil, cerr
		}
		view.Commit = commit
		pathCopy := filePath
		view.Path = &pathCopy

		if obj.Type != "blob" {
			// addLastCommitToObjectList()：给每个条目补上它自己的最后提交。
			entries := obj.Entries
			for i := range entries {
				child := entries[i].Name
				if filePath != "" {
					child = filePath + "/" + entries[i].Name
				}
				c, cerr := h.Git.LastCommit(ctx, repoPath, rev, child)
				if cerr != nil {
					continue
				}
				entries[i].Commit = c
			}
			view.Object = entries
		}
	}

	// 子模块：若 tree 里有 type == "commit" 的条目，从 .gitmodules 里取 url。
	if entries, ok := view.Object.([]domain.ObjectEntry); ok && len(entries) > 0 {
		hasSubmodule := false
		for _, e := range entries {
			if e.Type == "commit" {
				hasSubmodule = true
				break
			}
		}
		if hasSubmodule {
			urlMap := h.submoduleURLs(ctx, repoPath, rev)
			for i := range entries {
				if entries[i].Type == "commit" {
					key := entries[i].Name
					if filePath != "" {
						key = filePath + "/" + entries[i].Name
					}
					entries[i].URL = urlMap[key]
				} else {
					entries[i].URL = ""
				}
			}
			view.Object = entries
		}
	}

	return view, nil
}

// gitModuleRE 对应 PHP 侧 object_get() 的
// '/\[.*\]\r?\n\s*path\s*=\s*(.*)\r?\n\s*url\s*=\s*(.*)\r?\n?/m'。
var gitModuleRE = regexp.MustCompile(`(?m)\[.*\]\r?\n\s*path\s*=\s*(.*)\r?\n\s*url\s*=\s*(.*)\r?\n?`)

// submoduleURLs 解析 .gitmodules，返回 "子模块路径 → url" 映射。
func (h *RepositoryHandler) submoduleURLs(ctx context.Context, repoPath, rev string) map[string]string {
	out := map[string]string{}
	if rev == "" {
		return out
	}
	root, err := h.Git.Object(ctx, repoPath, rev)
	if err != nil {
		return out
	}
	for _, e := range root.Entries {
		if e.Name != ".gitmodules" {
			continue
		}
		file, err := h.Git.Object(ctx, repoPath, e.Object)
		if err != nil {
			return out
		}
		for _, m := range gitModuleRE.FindAllStringSubmatch(file.Raw, -1) {
			out[strings.TrimSpace(m[1])] = strings.TrimSpace(m[2])
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// repository/pathStack
// ---------------------------------------------------------------------------

// pathStack 实现 GET /api/repository/pathStack。
//
// 逐字对应 application/controllers/api/repository.php:1146-1194。
func (h *RepositoryHandler) pathStack(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	rKey := req.Parsed.QueryString("repository")
	revision := req.Parsed.QueryString("revision")
	filePath := req.Parsed.QueryString("path")
	if rKey == "" || revision == "" || filePath == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoRead); err != nil {
		return nil, err
	}

	repoPath, err := h.repoPath(ctx, rKey)
	if err != nil {
		return nil, err
	}

	segments := strings.Split(trimSlashes(filePath), "/")
	current, err := h.Git.Object(ctx, repoPath, revision)
	if err != nil {
		return nil, err
	}

	out := make([]any, 0, len(segments))
	for i, name := range segments {
		if current.Type == "blob" {
			// PHP 侧此时整个循环体被跳过，输出空数组。
			return []any{}, nil
		}
		var matched *domain.ObjectEntry
		for idx := range current.Entries {
			if current.Entries[idx].Name == name {
				matched = &current.Entries[idx]
				break
			}
		}
		if matched == nil {
			// PHP 侧 Response::output([]) 会立即结束请求。
			return []any{}, nil
		}
		out = append(out, domain.PathStackEntry{
			Mode:   matched.Mode,
			Type:   matched.Type,
			Object: matched.Object,
			Name:   matched.Name,
		})
		if matched.Type != "blob" {
			current, err = h.Git.Object(ctx, repoPath, matched.Object)
			if err != nil {
				return nil, err
			}
		} else if i < len(segments)-1 {
			return []any{}, nil
		}
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// repository/branchList
// ---------------------------------------------------------------------------

// branchList 实现 GET /api/repository/branchList。
//
// 逐字对应 application/controllers/api/repository.php:1385-1405
// 与 application/models/repository_model.php:1635-1704。
func (h *RepositoryHandler) branchList(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	rKey := req.Parsed.QueryString("repository")
	if rKey == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoRead); err != nil {
		return nil, err
	}

	repoInfo, err := h.Repos.Get(ctx, rKey)
	if err != nil {
		return nil, err
	}
	repoPath, err := h.repoPath(ctx, rKey)
	if err != nil {
		return nil, err
	}

	names, err := h.Git.Branches(ctx, repoPath)
	if err != nil {
		return nil, err
	}
	rules, err := h.Repos.ProtectedBranchRules(ctx, rKey)
	if err != nil {
		return nil, err
	}

	base := "master"
	if repoInfo.DefaultBranch != nil && *repoInfo.DefaultBranch != "" {
		base = *repoInfo.DefaultBranch
	}

	out := make([]any, 0, len(names))
	for _, name := range names {
		commits, cerr := h.Git.Commits(ctx, repoPath, name, "", "", 1, 1)
		if cerr != nil {
			return nil, cerr
		}
		branch := domain.Branch{ID: name, Name: name}
		if len(commits) > 0 {
			branch.Commit = commits[0]
		}
		diff, cerr := h.Git.BranchCompareCount(ctx, repoPath, base, name)
		if cerr != nil {
			return nil, cerr
		}
		branch.IsMerge = diff == 0
		branch.Protected = isProtectedBranch(rules, name)
		out = append(out, branch)
	}
	return out, nil
}

// isProtectedBranch 判断分支是否命中保护规则。
//
// 逐字对应 application/models/repository_model.php:1681-1704 的 isProtected()：
// 形如 /^...$/i 的规则按正则匹配，其余按全等匹配。
func isProtectedBranch(rules []domain.ProtectedBranchRule, name string) bool {
	for _, rule := range rules {
		if protectedRuleRe.MatchString(rule.Rule) {
			if re, err := regexp.Compile(rule.Rule); err == nil && re.MatchString(name) {
				return true
			}
			continue
		}
		if rule.Rule == name {
			return true
		}
	}
	return false
}

// protectedRuleRE 对应 PHP 的 '/^\/\^?([^\^\$\/]|\\/){1,30}\$?\/[igm]*$/'。
var protectedRuleRe = regexp.MustCompile(`^/\^?([^\^$/]|\\/){1,30}\$?/[igm]*$`)

// ---------------------------------------------------------------------------
// repository/tagList
// ---------------------------------------------------------------------------

// tagList 实现 GET /api/repository/tagList。
//
// 逐字对应 application/controllers/api/repository.php:1902-1918
// 与 application/models/repository_model.php:1771-1821。
func (h *RepositoryHandler) tagList(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	rKey := req.Parsed.QueryString("repository")
	if rKey == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoRead); err != nil {
		return nil, err
	}

	repoPath, err := h.repoPath(ctx, rKey)
	if err != nil {
		return nil, err
	}
	names, err := h.Git.Tags(ctx, repoPath)
	if err != nil {
		return nil, err
	}

	out := make([]any, 0, len(names))
	for _, name := range names {
		tag := domain.Tag{ID: name, Name: name}
		if detail, derr := h.Git.TagDetail(ctx, repoPath, name); derr == nil {
			detailText := detail.Detail
			tag.Detail = &detailText
			commit := detail.Commit
			tag.Commit = &commit
		}
		out = append(out, tag)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// repository/blameInfo
// ---------------------------------------------------------------------------

// blameInfo 实现 GET /api/repository/blameInfo。
//
// 逐字对应 application/controllers/api/repository.php:1810-1832。
//
// 【已知缺陷 —— 复刻】PHP 侧这里误用了 UAC_MR_READ（应为 UAC_REPO_READ）。
// 为保持前端可见行为一致，这里**原样复刻**该权限判定，并在注释中标出。
func (h *RepositoryHandler) blameInfo(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	rKey := req.Parsed.QueryString("repository")
	revision := req.Parsed.QueryString("revision")
	filePath := req.Parsed.QueryString("path")

	// 【已知缺陷】PHP 侧用的是 UAC_MR_READ，这里照搬。
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACMRRead); err != nil {
		return nil, err
	}

	repoPath, err := h.repoPath(ctx, rKey)
	if err != nil {
		return nil, err
	}
	blame, err := h.Git.Blame(ctx, repoPath, revision, filePath)
	if err != nil {
		return nil, err
	}
	return blame, nil
}

// ---------------------------------------------------------------------------
// repository/activities
// ---------------------------------------------------------------------------

// activities 实现 GET /api/repository/activities。
//
// 逐字对应 application/controllers/api/repository.php:209-242
// 与 application/models/repository_model.php:787-830。
func (h *RepositoryHandler) activities(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	rKey := req.Parsed.QueryString("repository")
	categoryRaw := req.Parsed.QueryString("category")
	keyword := req.Parsed.QueryString("keyword")
	mrKey := req.Parsed.QueryString("mergeRequest")
	page, pagesize := paging(req)

	category, cerr := parseCategory(categoryRaw)
	if cerr != nil {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if rKey == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoRead); err != nil {
		return nil, err
	}

	var (
		list []domain.RepositoryActivity
		err  error
	)
	if mrKey != "" {
		list, err = h.Repos.ListMergeRequestActivities(ctx, rKey, mrKey)
	} else {
		list, err = h.Repos.ListActivities(ctx, rKey, category, keyword, page, pagesize)
	}
	if err != nil {
		return nil, err
	}

	out := make([]any, 0, len(list))
	for i := range list {
		out = append(out, activityView(&list[i]))
	}
	return out, nil
}

// parseCategory 解析活动分类。
//
// PHP 侧的判定是 ($category && !in_array($category, [1,2,3]))，
// 其中 $category 来自 query（字符串），因此非数字值一律判为非法。
func parseCategory(raw string) (int, error) {
	if raw == "" {
		return domain.ActivityCategoryAll, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("category 非法: %q", raw)
	}
	switch n {
	case domain.ActivityCategoryAll, domain.ActivityCategoryCommit,
		domain.ActivityCategoryMergeRequest, domain.ActivityCategoryMember:
		return n, nil
	}
	return 0, fmt.Errorf("category 非法: %q", raw)
}

// ---------------------------------------------------------------------------
// repository/mergeRequests
// ---------------------------------------------------------------------------

// mergeRequests 实现 GET /api/repository/mergeRequests。
//
// 逐字对应 application/controllers/api/repository.php:1196-1238。
func (h *RepositoryHandler) mergeRequests(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	rKey := req.Parsed.QueryString("repository")
	statusRaw := req.Parsed.QueryString("status")
	keyword := req.Parsed.QueryString("keyword")
	sort := req.Parsed.QueryString("sort")
	page, pagesize := paging(req)

	status, serr := parseMergeRequestStatus(statusRaw)
	if serr != nil {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if rKey == "" || (sort != "desc" && sort != "asc") {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACMRRead); err != nil {
		return nil, err
	}

	list, err := h.Repos.MergeRequests(ctx, []string{rKey}, status, keyword, sort, page, pagesize)
	if err != nil {
		return nil, err
	}
	views, err := h.normalizeMergeRequests(ctx, list)
	if err != nil {
		return nil, err
	}
	out := make([]any, 0, len(views))
	for _, v := range views {
		out = append(out, v)
	}
	return out, nil
}

// parseMergeRequestStatus 解析合并请求状态。
func parseMergeRequestStatus(raw string) (int, error) {
	if raw == "" {
		return domain.MergeRequestStatusNone, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("status 非法: %q", raw)
	}
	switch n {
	case domain.MergeRequestStatusNone, domain.MergeRequestStatusOpen,
		domain.MergeRequestStatusMerged, domain.MergeRequestStatusClosed:
		return n, nil
	}
	return 0, fmt.Errorf("status 非法: %q", raw)
}

// normalizeMergeRequests 把合并请求行转换为对外形态。
//
// 逐字对应 application/models/repository_model.php:173-194 的 normalizeMergeRequests()。
func (h *RepositoryHandler) normalizeMergeRequests(ctx context.Context, list []domain.RepositoryMergeRequest) ([]domain.RepositoryMergeRequestView, error) {
	if len(list) == 0 {
		return []domain.RepositoryMergeRequestView{}, nil
	}
	mrKeys := make([]string, 0, len(list))
	for _, mr := range list {
		mrKeys = append(mrKeys, mr.Key)
	}
	reviewers, err := h.Repos.MergeRequestReviewers(ctx, mrKeys)
	if err != nil {
		return nil, err
	}

	out := make([]domain.RepositoryMergeRequestView, 0, len(list))
	for _, mr := range list {
		view := domain.RepositoryMergeRequestView{
			ID:               mr.Key,
			SourceRepository: mr.SourceRKey,
			SourceBranch:     mr.SourceBranch,
			TargetRepository: mr.TargetRKey,
			TargetBranch:     mr.TargetBranch,
			Number:           mr.ID,
			Title:            mr.Title,
			Status:           mr.ProceedStatus,
			Create:           phpTimeOf(mr.Created),
			Update:           phpTimeOf(mr.Updated),
			Reviewers:        reviewersOf(reviewers[mr.Key]),
		}
		// commit 字段需要查源仓库的最后一次提交。
		repoPath, perr := h.repoPath(ctx, mr.SourceRKey)
		if perr == nil {
			if commit, cerr := h.Git.LastCommit(ctx, repoPath, mr.SourceBranch, ""); cerr == nil {
				view.Commit = *commit
			}
		}
		out = append(out, view)
	}
	return out, nil
}

// reviewersOf 把评审人行转成前端需要的对象数组。
func reviewersOf(list []domain.RepositoryMergeRequestReviewer) []any {
	out := make([]any, 0, len(list))
	for _, rv := range list {
		out = append(out, map[string]any{
			"id":       rv.Key,
			"user":     rv.UKey,
			"isReview": rv.IsReview,
		})
	}
	return out
}

// ---------------------------------------------------------------------------
// repository/relatedMergeRequests
// ---------------------------------------------------------------------------

// relatedMergeRequests 实现 GET /api/repository/relatedMergeRequests。
//
// 逐字对应 application/controllers/api/repository.php:1240-1301。
func (h *RepositoryHandler) relatedMergeRequests(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	statusRaw := req.Parsed.QueryString("status")
	keyword := req.Parsed.QueryString("keyword")
	sort := req.Parsed.QueryString("sort")
	page, pagesize := paging(req)

	status, serr := parseMergeRequestStatus(statusRaw)
	if serr != nil {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if sort != "desc" && sort != "asc" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	repos, err := h.Repos.ListJoined(ctx, uKey)
	if err != nil {
		return nil, err
	}
	rKeys := make([]string, 0, len(repos))
	for _, r := range repos {
		rKeys = append(rKeys, r.Key)
	}
	roles, err := h.Repos.RolesFor(ctx, rKeys, uKey)
	if err != nil {
		return nil, err
	}

	// 只保留角色 > ROLE_GUEST(1) 的仓库。
	related := make([]string, 0, len(repos))
	for _, r := range repos {
		if roles[r.Key] > int(domain.RoleGuest) {
			related = append(related, r.Key)
		}
	}
	if len(related) == 0 {
		return domain.RelatedMergeRequests{
			RelatedRepository: []string{},
			MergeRequests:     []domain.RepositoryMergeRequestView{},
		}, nil
	}

	list, err := h.Repos.MergeRequests(ctx, related, status, keyword, sort, page, pagesize)
	if err != nil {
		return nil, err
	}
	views, err := h.normalizeMergeRequests(ctx, list)
	if err != nil {
		return nil, err
	}
	return domain.RelatedMergeRequests{
		RelatedRepository: related,
		MergeRequests:     views,
	}, nil
}

// ---------------------------------------------------------------------------
// repository/mergeRequestDetail
// ---------------------------------------------------------------------------

// mergeRequestDetail 实现 GET /api/repository/mergeRequestDetail。
//
// 逐字对应 application/controllers/api/repository.php:1834-1864
// 与 application/models/repository_model.php:196-227 的 normalizeMergeRequest()。
func (h *RepositoryHandler) mergeRequestDetail(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	rKey := req.Parsed.QueryString("repository")
	mIDRaw := req.Parsed.QueryString("mid")
	if rKey == "" || mIDRaw == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACMRRead); err != nil {
		return nil, err
	}
	mID, err := strconv.Atoi(mIDRaw)
	if err != nil {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	mr, err := h.Repos.MergeRequestByIDAndKey(ctx, mID, rKey)
	if err != nil {
		return nil, err
	}
	return h.normalizeMergeRequest(ctx, mr)
}

// normalizeMergeRequest 把合并请求行转换为对外形态。
func (h *RepositoryHandler) normalizeMergeRequest(ctx context.Context, mr *domain.RepositoryMergeRequest) (any, error) {
	users, err := h.Repos.UsersByKeys(ctx, []string{mr.UKey})
	if err != nil {
		return nil, err
	}
	emails, err := h.Repos.CommitEmailsByUser(ctx, []string{mr.UKey})
	if err != nil {
		return nil, err
	}

	submitter := domain.NormalizedUser{}
	if u, ok := users[mr.UKey]; ok {
		submitter = domain.NormalizeUser(u, emails[mr.UKey], false, 0, h.host(), h.ssh())
	}

	// 评审人来自 cc_merge_request_reviewers；列表接口已实现，详情接口
	// 曾经硬编码为空数组，导致前端 MR 详情页看不到 / 无法管理评审人。
	reviewerRows, err := h.Repos.MergeRequestReviewers(ctx, []string{mr.Key})
	if err != nil {
		return nil, err
	}
	reviewers := reviewersOf(reviewerRows[mr.Key])

	sourceRepo, err := h.Repos.Get(ctx, mr.SourceRKey)
	if err != nil {
		return nil, err
	}
	groups, err := h.Repos.GroupsByKeys(ctx, []string{sourceRepo.GroupKey})
	if err != nil {
		return nil, err
	}
	members, err := h.membersOf(ctx, []domain.Repository{*sourceRepo}, groups)
	if err != nil {
		return nil, err
	}
	forkCounts, err := h.Repos.ForkCounts(ctx, []string{mr.SourceRKey})
	if err != nil {
		return nil, err
	}
	mrCounts, err := h.Repos.MergeRequestCounts(ctx, []string{mr.SourceRKey})
	if err != nil {
		return nil, err
	}
	sourceInfo := domain.RepositorySourceInfo{
		NormalizedRepository: domain.NormalizeRepository(sourceRepo, groups[sourceRepo.GroupKey],
			forkCounts[mr.SourceRKey], mrCounts[mr.SourceRKey]),
		Members: members[mr.SourceRKey],
	}

	view := domain.RepositoryMergeRequestDetail{
		ID:               mr.Key,
		User:             mr.UKey,
		SourceRepository: mr.SourceRKey,
		SourceBranch:     mr.SourceBranch,
		SourceObject:     mr.SourceHash,
		TargetRepository: mr.TargetRKey,
		TargetBranch:     mr.TargetBranch,
		TargetObject:     mr.TargetHash,
		Submitter:        submitter,
		SourceRepoInfo:   sourceInfo,
		Title:            mr.Title,
		Description:      mr.Description,
		Number:           mr.ID,
		Handler:          mr.MergeUKey,
		Status:           mr.ProceedStatus,
		Create:           phpTimeOf(mr.Created),
		Update:           phpTimeOf(mr.Updated),
		Close:            phpTimeOf(mr.CloseTime),
		Merge:            phpTimeOf(mr.MergeTime),
		Reviewers:        reviewers,
	}

	// 控制器在最后追加提交者的邮箱。
	if u, ok := users[mr.UKey]; ok {
		email := u.Email
		view.Email = &email
	}
	return view, nil
}

// ---------------------------------------------------------------------------
// repository/mergeRequestVersionList
// ---------------------------------------------------------------------------

// mergeRequestVersionList 实现 GET /api/repository/mergeRequestVersionList。
//
// 逐字对应 application/controllers/api/repository.php:1457-1487
// 与 application/models/repository_model.php:1213-1261 的 getMergeRequestVersionList()。
func (h *RepositoryHandler) mergeRequestVersionList(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	mrKey := trimSlashes(req.Parsed.QueryString("mergeRequest"))
	if mrKey == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	mr, err := h.Repos.MergeRequestByKey(ctx, mrKey)
	if err != nil {
		return nil, err
	}
	if err := h.Guard.RequireRepository(ctx, mr.TargetRKey, uKey, domain.UACMRRead); err != nil {
		return nil, err
	}

	versionList, err := h.mergeRequestVersions(ctx, mr)
	if err != nil {
		return nil, err
	}

	baseVersion := domain.Commit{}
	targetPath, perr := h.repoPath(ctx, mr.TargetRKey)
	if perr == nil {
		if c, cerr := h.Git.LastCommit(ctx, targetPath, mr.TargetBranch, ""); cerr == nil {
			baseVersion = *c
		}
	}

	baseVersionList := make([]domain.Commit, len(versionList))
	copy(baseVersionList, versionList)
	if len(baseVersionList) > 0 {
		baseVersionList[0] = baseVersion
	}

	return domain.RepositoryMergeRequestVersionList{
		MergeVersionList: versionList,
		BaseVersion:      baseVersion.SHA,
		BaseVersionList:  baseVersionList,
	}, nil
}

// mergeRequestVersions 计算合并请求的版本序列。
//
// 逐字对应 application/models/repository_model.php:1213-1261：
// 取 MR 的起始提交、源分支的最后一个提交，再取两者之间的提交列表，
// 最后把起始提交追加到末尾。
func (h *RepositoryHandler) mergeRequestVersions(ctx context.Context, mr *domain.RepositoryMergeRequest) ([]domain.Commit, error) {
	sourcePath, err := h.repoPath(ctx, mr.SourceRKey)
	if err != nil {
		return nil, err
	}
	initCommit, err := h.Git.CommitInfoByHash(ctx, sourcePath, mr.StartHash)
	if err != nil {
		return nil, err
	}
	sourceLast, err := h.Git.LastCommit(ctx, sourcePath, mr.SourceBranch, "")
	if err != nil {
		return nil, err
	}
	commits, err := h.Git.RevisionRange(ctx, sourcePath, mr.StartHash, sourceLast.SHA, mr.SourceBranch)
	if err != nil {
		return nil, err
	}
	if len(commits) == 0 {
		return []domain.Commit{*sourceLast}, nil
	}
	return append(commits, *initCommit), nil
}

// ---------------------------------------------------------------------------
// repository/mergeRequestCommits
// ---------------------------------------------------------------------------

// mergeRequestCommits 实现 GET /api/repository/mergeRequestCommits。
//
// 逐字对应 application/controllers/api/repository.php:1303-1329。
//
// 【有意差异 —— 安全修复】PHP 侧该端点**没有资源级鉴权**，
// 这里补上源仓库的 UAC_REPO_READ 校验。
func (h *RepositoryHandler) mergeRequestCommits(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	mrKey := req.Parsed.QueryString("mergeRequest")
	sourceRKey := req.Parsed.QueryString("sourceRepository")
	sourceBranch := req.Parsed.QueryString("sourceBranch")
	targetRKey := req.Parsed.QueryString("targetRepository")
	targetBranch := req.Parsed.QueryString("targetBranch")

	if sourceRKey == "" || sourceBranch == "" || targetRKey == "" || targetBranch == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	// 【安全修复】PHP 侧无资源级鉴权。
	if err := h.Guard.RequireRepository(ctx, sourceRKey, uKey, domain.UACRepoRead); err != nil {
		return nil, err
	}

	sourceObject, targetObject, err := h.mergeRequestLastCommitSHA(ctx, mrKey, sourceRKey, sourceBranch, targetRKey, targetBranch)
	if err != nil {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	sourcePath, err := h.repoPath(ctx, sourceRKey)
	if err != nil {
		return nil, err
	}

	var commits []domain.Commit
	if targetRKey != sourceRKey {
		// 跨仓库（fork MR）：在临时工作区 fetch 目标仓库后比较。
		// 对应 PHP 的 getCommitWithRemote()。
		targetPath, perr := h.repoPath(ctx, targetRKey)
		if perr != nil {
			return nil, perr
		}
		commits, err = h.Git.CommitsSinceRemote(ctx, sourcePath, sourceObject, targetPath, targetObject)
	} else {
		commits, err = h.Git.CommitListFromLocal(ctx, sourcePath, targetObject, sourceObject)
	}
	if err != nil {
		return nil, err
	}
	out := make([]any, 0, len(commits))
	for _, c := range commits {
		out = append(out, c)
	}
	return out, nil
}

// mergeRequestLastCommitSHA 取合并请求两端的最后一个提交。
//
// 逐字对应 application/models/repository_model.php:1263-1286 的
// getMergeRequestLastCommitSha()。
func (h *RepositoryHandler) mergeRequestLastCommitSHA(ctx context.Context, mrKey, sourceRKey, sourceBranch, targetRKey, targetBranch string) (string, string, error) {
	var sourceObject, targetObject *string
	if mrKey != "" {
		mr, err := h.Repos.MergeRequestByKey(ctx, mrKey)
		if err != nil {
			return "", "", err
		}
		sourceObject = mr.SourceHash
		targetObject = mr.TargetHash
	}
	if sourceObject == nil || targetObject == nil || *sourceObject == "" || *targetObject == "" {
		sourcePath, err := h.repoPath(ctx, sourceRKey)
		if err != nil {
			return "", "", err
		}
		sourceCommit, err := h.Git.LastCommit(ctx, sourcePath, sourceBranch, "")
		if err != nil {
			return "", "", err
		}
		targetPath, err := h.repoPath(ctx, targetRKey)
		if err != nil {
			return "", "", err
		}
		targetCommit, err := h.Git.LastCommit(ctx, targetPath, targetBranch, "")
		if err != nil {
			return "", "", err
		}
		return sourceCommit.SHA, targetCommit.SHA, nil
	}
	return *sourceObject, *targetObject, nil
}

// ---------------------------------------------------------------------------
// repository/targetRepository
// ---------------------------------------------------------------------------

// targetRepository 实现 GET /api/repository/targetRepository。
//
// 逐字对应 application/controllers/api/repository.php:1998-2032。
//
// 【有意差异 —— 安全修复】PHP 侧该端点**没有资源级鉴权**，
// 这里补上 UAC_REPO_READ 校验。
func (h *RepositoryHandler) targetRepository(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	rKey := req.Parsed.QueryString("repository")
	if rKey == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	// 【安全修复】PHP 侧无资源级鉴权。
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoRead); err != nil {
		return nil, err
	}

	repoInfo, err := h.Repos.Get(ctx, rKey)
	if err != nil {
		return nil, err
	}
	groups, err := h.Repos.GroupsByKeys(ctx, []string{repoInfo.GroupKey})
	if err != nil {
		return nil, err
	}

	result := []any{domain.TargetRepository{
		RKey:     rKey,
		Name:     groups[repoInfo.GroupKey].DisplayName + "/" + repoInfo.DisplayName,
		ForkFlag: false,
	}}

	if repoInfo.ForkFromKey != nil && *repoInfo.ForkFromKey != "" {
		forkKey := *repoInfo.ForkFromKey
		forkRepo, ferr := h.Repos.Get(ctx, forkKey)
		if ferr == nil {
			forkGroups, gerr := h.Repos.GroupsByKeys(ctx, []string{forkRepo.GroupKey})
			if gerr == nil {
				result = append(result, domain.TargetRepository{
					RKey:     forkKey,
					Name:     forkGroups[forkRepo.GroupKey].DisplayName + "/" + forkRepo.DisplayName,
					ForkFlag: true,
				})
			}
		}
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// repository/lastCommitLog
// ---------------------------------------------------------------------------

// lastCommitLog 实现 GET /api/repository/lastCommitLog。
//
// 逐字对应 application/controllers/api/repository.php:1374-1383。
//
// 【有意差异 —— 安全修复】PHP 侧该端点**没有资源级鉴权**，
// 这里补上 UAC_REPO_READ 校验。
func (h *RepositoryHandler) lastCommitLog(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	rKey := req.Parsed.QueryString("repository")
	ref := req.Parsed.QueryString("ref")
	filePath := trimSlashes(req.Parsed.QueryString("path"))
	// 【安全修复】PHP 侧无资源级鉴权。
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoRead); err != nil {
		return nil, err
	}

	repoPath, err := h.repoPath(ctx, rKey)
	if err != nil {
		return nil, err
	}
	commit, err := h.Git.LastCommit(ctx, repoPath, ref, filePath)
	if err != nil {
		return nil, err
	}
	return commit, nil
}

// ---------------------------------------------------------------------------
// repository/refListContainSHA
// ---------------------------------------------------------------------------

// refListContainSHA 实现 GET /api/repository/refListContainSHA。
//
// 逐字对应 application/controllers/api/repository.php:968-1000。
//
// 【有意差异 —— 安全修复】PHP 侧该端点**没有资源级鉴权**，
// 这里补上 UAC_REPO_READ 校验。
func (h *RepositoryHandler) refListContainSHA(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	rKey := req.Parsed.QueryString("repository")
	commitHash := req.Parsed.QueryString("hash")
	if uKey == "" || rKey == "" || commitHash == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	// 【安全修复】PHP 侧无资源级鉴权。
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoRead); err != nil {
		return nil, err
	}

	repoPath, err := h.repoPath(ctx, rKey)
	if err != nil {
		return nil, err
	}
	detail, err := h.Git.CommitDetail(ctx, repoPath, commitHash)
	if err != nil {
		return nil, err
	}

	// 合并提交有两个父节点时，只有同时包含两个父节点的分支/标签才算命中。
	parents := strings.Fields(detail.Parent)
	if len(parents) > 1 {
		branch0, err := h.Git.BranchesContainingSHA(ctx, repoPath, parents[0])
		if err != nil {
			return nil, err
		}
		branch1, err := h.Git.BranchesContainingSHA(ctx, repoPath, parents[1])
		if err != nil {
			return nil, err
		}
		tag0, err := h.Git.TagsContainingSHA(ctx, repoPath, parents[0])
		if err != nil {
			return nil, err
		}
		tag1, err := h.Git.TagsContainingSHA(ctx, repoPath, parents[1])
		if err != nil {
			return nil, err
		}
		return domain.RefList{
			Branch: intersect(branch0, branch1),
			Tag:    intersect(tag0, tag1),
		}, nil
	}

	branches, err := h.Git.BranchesContainingSHA(ctx, repoPath, commitHash)
	if err != nil {
		return nil, err
	}
	tags, err := h.Git.TagsContainingSHA(ctx, repoPath, commitHash)
	if err != nil {
		return nil, err
	}
	return domain.RefList{Branch: branches, Tag: tags}, nil
}

// intersect 取两个字符串切片的交集（保留第一个切片的顺序）。
//
// 对应 PHP 侧的 array_intersect()。
func intersect(a, b []string) []string {
	set := make(map[string]bool, len(b))
	for _, v := range b {
		set[v] = true
	}
	out := make([]string, 0, len(a))
	for _, v := range a {
		if set[v] {
			out = append(out, v)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// repository/fileChanges
// ---------------------------------------------------------------------------

// fileChanges 实现 GET /api/repository/fileChanges。
//
// 逐字对应 application/controllers/api/repository.php:879-965。
func (h *RepositoryHandler) fileChanges(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	rKey := req.Parsed.QueryString("repository")
	original := req.Parsed.QueryString("original")
	modified := req.Parsed.QueryString("modified")
	mrKey := req.Parsed.QueryString("mergeRequest")
	targetRKey := req.Parsed.QueryString("targetRepository")

	if original == "" {
		original = "0000000000000000000000000000000000000000"
	}
	if uKey == "" || rKey == "" || modified == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	// PHP 侧只在没有 mergeRequest 时做仓库鉴权；带 MR 的跨仓库比较另行处理。
	if mrKey == "" {
		if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoRead); err != nil {
			return nil, err
		}
	}

	repoPath, err := h.repoPath(ctx, rKey)
	if err != nil {
		return nil, err
	}

	// 跨仓库（fork MR）比较：在临时工作区 fetch 目标仓库后执行。
	// 对应 PHP 的 getDiffWithRemote()。
	var count int
	var diff string
	if targetRKey != "" && targetRKey != rKey {
		targetPath, perr := h.repoPath(ctx, targetRKey)
		if perr != nil {
			return nil, perr
		}
		count, err = h.Git.DiffFileCountWithRemote(ctx, repoPath, original, targetPath, modified)
		if err != nil {
			return nil, err
		}
		if count > maxDisplayDiffFile {
			return domain.FileChangesLarge{Large: true, Count: count, Detail: ""}, nil
		}
		diff, err = h.Git.DiffWithRemote(ctx, repoPath, original, targetPath, modified)
	} else {
		count, err = h.Git.DiffFileCount(ctx, repoPath, original, modified)
		if err != nil {
			return nil, err
		}
		if count > maxDisplayDiffFile {
			return domain.FileChangesLarge{Large: true, Count: count, Detail: ""}, nil
		}
		diff, err = h.Git.Diff(ctx, repoPath, original, modified)
	}
	if err != nil {
		return nil, err
	}

	parsed := gitsvc.ParseDiff(diff)
	return domain.FileChanges{Count: len(parsed), Detail: parsed}, nil
}

// maxDisplayDiffFile 对应 PHP 常量 MAX_DISPLAY_DIFF_FILE。
const maxDisplayDiffFile = 100

// ---------------------------------------------------------------------------
// 成员装配
// ---------------------------------------------------------------------------

// membersOf 批量装配仓库成员列表。
//
// 逐字对应 application/models/repository_model.php:2100-2122 的 getMembers()：
// 组成员（含组创建者）+ 仓库成员 + 仓库创建者，
// 再用 Helper::getUniqueMemberList() 去重。
func (h *RepositoryHandler) membersOf(ctx context.Context, repos []domain.Repository, groups map[string]*domain.RepositoryGroup) (map[string][]domain.NormalizedRepositoryMember, error) {
	out := map[string][]domain.NormalizedRepositoryMember{}
	if len(repos) == 0 {
		return out, nil
	}

	rKeys := make([]string, 0, len(repos))
	gKeys := make([]string, 0, len(repos))
	uKeys := make([]string, 0, len(repos)*2)
	for _, r := range repos {
		rKeys = append(rKeys, r.Key)
		gKeys = append(gKeys, r.GroupKey)
		uKeys = append(uKeys, r.OwnerKey)
		if g, ok := groups[r.GroupKey]; ok && g != nil {
			uKeys = append(uKeys, g.OwnerKey)
		}
	}

	repoMembers, err := h.Repos.MembersOf(ctx, rKeys)
	if err != nil {
		return nil, err
	}
	groupMembers, err := h.Repos.GroupMembersOf(ctx, gKeys)
	if err != nil {
		return nil, err
	}

	for _, list := range repoMembers {
		for _, m := range list {
			uKeys = append(uKeys, m.UKey)
		}
	}
	for _, list := range groupMembers {
		for _, m := range list {
			uKeys = append(uKeys, m.UKey)
		}
	}

	users, err := h.Repos.UsersByKeys(ctx, uKeys)
	if err != nil {
		return nil, err
	}
	emails, err := h.Repos.CommitEmailsByUser(ctx, uKeys)
	if err != nil {
		return nil, err
	}

	for _, r := range repos {
		items := []domain.NormalizedRepositoryMember{}

		// 组成员（含组创建者）。
		groupItems := []domain.NormalizedRepositoryMember{}
		for _, m := range groupMembers[r.GroupKey] {
			u, ok := users[m.UKey]
			if !ok {
				continue
			}
			groupItems = append(groupItems,
				domain.NormalizeRepositoryGroupMember(&m, u, emails[m.UKey], h.host(), h.ssh()))
		}
		if g, ok := groups[r.GroupKey]; ok && g != nil {
			if u, ok := users[g.OwnerKey]; ok {
				groupItems = append(groupItems,
					domain.NormalizeRepositoryGroupCreator(g, u, emails[g.OwnerKey], h.host(), h.ssh()))
			}
		}
		items = append(items, mergeMembers(groupItems)...)

		// 仓库成员 + 仓库创建者。
		repoItems := []domain.NormalizedRepositoryMember{}
		for _, m := range repoMembers[r.Key] {
			u, ok := users[m.UKey]
			if !ok {
				continue
			}
			repoItems = append(repoItems,
				domain.NormalizeRepositoryMember(&m, u, emails[m.UKey], h.host(), h.ssh()))
		}
		if u, ok := users[r.OwnerKey]; ok {
			repoItems = append(repoItems,
				domain.NormalizeRepositoryCreator(&r, u, emails[r.OwnerKey], h.host(), h.ssh()))
		}
		items = append(items, repoItems...)

		out[r.Key] = mergeMembers(items)
	}
	return out, nil
}

// mergeMembers 按 id 去重成员。
//
// 逐字对应 application/libraries/service/Utility/Helper.php:59-71
// 的 getUniqueMemberList()：
// 已存在且新条目是"已删除的组成员"时保留旧值，否则覆盖（位置保持首次出现）。
func mergeMembers(items []domain.NormalizedRepositoryMember) []domain.NormalizedRepositoryMember {
	index := map[string]int{}
	out := make([]domain.NormalizedRepositoryMember, 0, len(items))
	for _, item := range items {
		if i, ok := index[item.ID]; ok {
			if item.GroupMember && item.DeleteFlag {
				continue
			}
			out[i] = item
			continue
		}
		index[item.ID] = len(out)
		out = append(out, item)
	}
	return out
}

// ---------------------------------------------------------------------------
// 文件内容
// ---------------------------------------------------------------------------

// maxDisplayFileSize 对应 PHP 常量 MAX_DISPLAY_FILE_SIZE（2M）。
const maxDisplayFileSize = 0x200000

// parseObjectToFile 把 blob 内容转成文件视图。
//
// 逐字对应 application/libraries/service/Utility/Helper.php:309-341
// 的 parseObjectToFile()：超大返回 oversize；图片额外给出宽高与 base64；
// 文本类给出 raw；其余标记 unsupport。
//
// 与 PHP 的一处**有意差异**：size 用的是 blob 的**真实字节数**。
// PHP 侧拿到的内容来自 Command::batch() —— 它用 exec($cmd, $output) 收集输出，
// 而 PHP 的 exec() 会**按行拆分并丢掉行尾换行**，随后 implode("\n", $output)
// 再拼回去，于是文件末尾的换行被吞掉，strlen() 比真实大小少 1。
// 实测：28 字节的 README.md，PHP 返回 size=27，Go 返回 size=28。
// 这里以 git 的真实大小为准 —— 显示一个错误的小数会误导用户。
func parseObjectToFile(content string) map[string]any {
	if len(content) > maxDisplayFileSize {
		return map[string]any{"oversize": true}
	}
	mime := detectMime(content)
	out := map[string]any{
		"mime": mime,
		"size": len(content),
	}
	switch {
	case strings.HasPrefix(mime, "image/"):
		out["base64Encoded"] = base64Encode(content)
		if w, ht, ok := imageSize(content); ok {
			out["width"] = w
			out["height"] = ht
		}
	case mime == "video/mp4":
		out["base64Encoded"] = base64Encode(content)
	case strings.HasPrefix(mime, "text/"), mime == "text/plain",
		mime == "application/xml", strings.HasSuffix(mime, "+xml"):
		out["raw"] = content
	default:
		out["unsupport"] = true
	}
	return out
}

// imageSize 读取 PNG / GIF 的像素尺寸。
func imageSize(content string) (int, int, bool) {
	if len(content) < 24 {
		return 0, 0, false
	}
	switch {
	case strings.HasPrefix(content, "\x89PNG\r\n\x1a\n"):
		w := int(content[16])<<24 | int(content[17])<<16 | int(content[18])<<8 | int(content[19])
		h := int(content[20])<<24 | int(content[21])<<16 | int(content[22])<<8 | int(content[23])
		return w, h, true
	case strings.HasPrefix(content, "GIF8"):
		w := int(content[6]) | int(content[7])<<8
		h := int(content[8]) | int(content[9])<<8
		return w, h, true
	}
	return 0, 0, false
}

// detectMime 粗略识别 MIME 类型。
//
// PHP 侧用 mime_content_type()（基于文件魔数），Go 侧没有等价实现，
// 这里按内容魔数 + 文本特征给出与 PHP 常见取值一致的字符串。
func detectMime(content string) string {
	switch {
	case strings.HasPrefix(content, "\x89PNG\r\n\x1a\n"):
		return "image/png"
	case strings.HasPrefix(content, "\xff\xd8\xff"):
		return "image/jpeg"
	case strings.HasPrefix(content, "GIF8"):
		return "image/gif"
	case strings.HasPrefix(content, "%PDF-"):
		return "application/pdf"
	case strings.HasPrefix(content, "PK\x03\x04"):
		return "application/zip"
	case strings.HasPrefix(content, "\x00\x00\x00") && len(content) > 8 && content[4] == 'f':
		return "video/mp4"
	}
	trimmed := strings.TrimLeft(content, " \t\r\n")
	if strings.HasPrefix(trimmed, "<?xml") {
		return "text/xml"
	}
	if isPrintableText(content) {
		return "text/plain"
	}
	return "application/octet-stream"
}

// isPrintableText 判断内容是否像纯文本。
func isPrintableText(content string) bool {
	if content == "" {
		return true
	}
	for i := 0; i < len(content); i++ {
		c := content[i]
		if c == 0 {
			return false
		}
	}
	return true
}

// base64Encode 是标准 base64 编码。
func base64Encode(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

// ---------------------------------------------------------------------------
// 杂项
// ---------------------------------------------------------------------------

// phpTimeOf 把 MySQL 时间字符串转成 PHP strtotime 的秒数。
func phpTimeOf(raw string) int64 { return domain.PhpTime(raw) }
