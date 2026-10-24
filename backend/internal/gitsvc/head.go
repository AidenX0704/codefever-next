package gitsvc

import (
	"context"
	"fmt"
	"strings"
)

// EnsureHEAD 让裸仓库的 HEAD 指向一个真实存在的分支。
//
// 背景：`git init --bare` 的 HEAD 默认指向 refs/heads/master。如果用户
// 第一次推送的是 main，裸仓库的 HEAD 仍然指向不存在的 master，`git clone`
// 就会得到：
//
//	warning: remote HEAD refers to nonexistent ref, unable to checkout
//
// 并且工作区为空（只留下 .git）。修复方式是把 HEAD 指向默认分支。
//
// preferred 优先（通常来自 cc_repositories.r_default_branch_name）；为空或
// 分支不存在时，依次选择 main、master、第一个分支。返回最终选中的分支；
// 仓库还没有任何分支时返回空串。
func (s *Service) EnsureHEAD(ctx context.Context, repoPath, preferred string) (string, error) {
	if repoPath == "" {
		return "", fmt.Errorf("仓库路径不能为空")
	}
	branches, err := s.Branches(ctx, repoPath)
	if err != nil {
		return "", err
	}
	if len(branches) == 0 {
		return "", nil
	}

	chosen := ""
	preferred = strings.TrimPrefix(strings.TrimSpace(preferred), "refs/heads/")
	if preferred != "" && containsString(branches, preferred) {
		chosen = preferred
	}
	if chosen == "" {
		for _, candidate := range []string{"main", "master"} {
			if containsString(branches, candidate) {
				chosen = candidate
				break
			}
		}
	}
	if chosen == "" {
		chosen = branches[0]
	}
	if err := s.SetHEAD(ctx, repoPath, chosen); err != nil {
		return "", err
	}
	return chosen, nil
}

// SetHEAD 把裸仓库 HEAD 指向 refs/heads/<branch>。
//
// 要求分支已经存在，避免把 HEAD 指向一个不存在的 ref（会产生同样的
// clone 警告）。
func (s *Service) SetHEAD(ctx context.Context, repoPath, branch string) error {
	branch = strings.TrimPrefix(strings.TrimSpace(branch), "refs/heads/")
	if branch == "" {
		return fmt.Errorf("分支名为空")
	}
	if strings.ContainsAny(branch, " \t\r\n\x00") {
		return fmt.Errorf("非法分支名: %q", branch)
	}
	if err := rejectOptionLike(branch); err != nil {
		return err
	}
	exists, err := s.RefExists(ctx, repoPath, branch)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("分支不存在: %s", branch)
	}
	_, err = s.run(ctx, repoPath, "symbolic-ref", "HEAD", "refs/heads/"+branch)
	return err
}

// containsString 判断切片中是否包含指定字符串。
func containsString(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
