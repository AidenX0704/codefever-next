package handlers

import (
	"context"
	"strings"

	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
	"codefever-next/backend/internal/httpapi"
)

// fileContent 实现 GET /api/repository/fileContent。
//
// 对应 application/controllers/api/repository.php:1002-1051。
//
// ⚠️ 与 PHP 的差异（有意修复的缺陷）：
//
//	PHP 侧调用了 repository_model 中**不存在**的方法
//	（geFilesByBranchOrSHA1Hash，拼写错误），因此该端点恒返回 HTTP 500。
//	新实现用 git 的原生语法 `cat-file blob <sha>:<path>` 取到文件内容，
//	并保留 PHP 的其余语义：参数校验、仓库读权限、按 from/to 切片、找不到文件报 0x040E。
//
// 该端点目前**前端没有调用点**（www/view/src/data_providers/RepositoryData.js 中
// 定义了但未使用），因此行为变更不会影响现有前端。
func (h *RepositoryHandler) fileContent(ctx context.Context, req *httpapi.Request) (any, error) {
	uKey := req.UserKey()
	rKey := req.Parsed.QueryString("repository")
	sha := req.Parsed.QueryString("sha")
	filePath := strings.Trim(req.Parsed.QueryString("path"), "/")

	if rKey == "" || sha == "" || filePath == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Guard.RequireRepository(ctx, rKey, uKey, domain.UACRepoRead); err != nil {
		return nil, err
	}

	from := req.Parsed.QueryInt("from")
	to := req.Parsed.QueryInt("to")

	repoPath, err := h.repoPath(ctx, rKey)
	if err != nil {
		return nil, err
	}

	raw, err := h.Git.FileContent(ctx, repoPath, sha, filePath)
	if err != nil {
		// 文件不存在（或 sha/path 无法解析）→ 0x040E File Not Found。
		return nil, contract.NewError(contract.CodeFileNotFound)
	}

	if from != 0 || to != 0 {
		if from > 0 && to > 0 && to > from {
			return map[string]any{"raw": sliceLines(raw, from, to)}, nil
		}
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	return map[string]any{"raw": raw}, nil
}

// sliceLines 按 1 起始的行号区间切片，并重新用换行连接。
//
// 对应 PHP 的 array_slice(explode("\n", $result), $from - 1, $to - $from)。
func sliceLines(content string, from, to int) string {
	lines := strings.Split(content, "\n")
	start := from - 1
	if start < 0 {
		start = 0
	}
	end := start + (to - from)
	if end > len(lines) {
		end = len(lines)
	}
	if start > len(lines) {
		start = len(lines)
	}
	return strings.Join(lines[start:end], "\n")
}
