package gitsvc

import (
	"context"
	"fmt"
	"strings"
)

// FileContent 读取仓库中某个版本下的文件内容。
//
// 对应 PHP 侧 repository_model 的"按 branch/SHA + 路径取文件"语义。
// PHP 原实现调用了一个**不存在的方法名**（geFilesByBranchOrSHA1Hash），
// 导致 /api/repository/fileContent 恒 500；这里用 git 的原生语法实现：
//
//	git cat-file blob <sha>:<path>
//
// sha 可以是 commit / tree / tag 等任意可解析到 tree 的对象，
// path 是相对仓库根的路径。
func (s *Service) FileContent(ctx context.Context, repoPath, sha, path string) (string, error) {
	cleaned := strings.TrimPrefix(strings.TrimSpace(path), "/")
	if cleaned == "" {
		return "", fmt.Errorf("文件路径为空")
	}
	spec := sha + ":" + cleaned
	out, err := s.run(ctx, repoPath, "cat-file", "blob", spec)
	if err != nil {
		return "", fmt.Errorf("读取文件 %s 失败: %w", spec, err)
	}
	return out, nil
}
