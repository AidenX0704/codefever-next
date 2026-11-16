package gitsvc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
)

// archiveArgs 构造 `git archive` 的 argv。
//
// 单独拆出来与 commitListArgs 的目的相同：能在不跑真实 git 的单测里
// 断言"ref 只占一个 argv 元素"，证明以 "-" 开头的 ref 不会被当成选项。
//
//	--format=<fmt>   zip 或 tar（tar.gz 由调用方在输出侧再包 gzip）
//	--               路径类参数分隔符：ref 永远不会被 git 解释成选项
func archiveArgs(format, ref string) []string {
	args := []string{"archive", "--format=" + format}
	if ref != "" {
		args = append(args, "--", ref)
	}
	return args
}

// ArchiveFormats 是归档下载支持的格式（与 /api/repository/archive 的 format 参数对应）。
// ArchiveFormats 是 Archive 可接受的 **git --format 值**（"zip" / "tar"），
// 不是 HTTP 的 format 参数（"tar.gz"）：handler 把 "tar.gz" 翻译成
// gitFormat="tar" 传进来，再由调用方在输出侧包 gzip.Writer。
// 白名单若漏了 "tar"，tar.gz 请求会在这里被拒 —— gzip 只写出空的头尾，
// 客户端拿到 23 字节的空文件（部署冒烟实测踩过）。
var ArchiveFormats = map[string]bool{
	"zip": true,
	"tar": true,
}

// Archive 把某个修订号的归档流式写入 w。
//
// format 只接受 zip / tar（tar.gz 由调用方用 gzip.Writer 包住 w 后传 tar），
// 拼装完整格式串的责任在调用方 —— 这里只做白名单校验与 argv 拆分。
//
// 与包内其它命令的差异：输出**不落内存缓冲**，cmd.Stdout 直连 w。
// 归档是"整个仓库内容"级别的输出，多 GB 仓库走 bytes.Buffer 会把
// 进程内存打满；流式写出让内存占用与归档大小无关。
//
// 失败语义：git 在写出过程中失败（ref 不存在 / 仓库损坏）会以非零退出码结束，
// 此时 HTTP 响应头可能已经写出 —— 调用方（handlers）必须先校验 ref 存在
// 再启动归档，把可预见的失败挡在流式响应之前。
func (s *Service) Archive(ctx context.Context, repoPath, format, ref string, w io.Writer) error {
	if repoPath == "" {
		return errors.New("仓库路径不能为空")
	}
	if !ArchiveFormats[format] {
		return fmt.Errorf("归档格式不支持: %q", format)
	}
	if ref == "" {
		return errors.New("ref 不能为空")
	}
	if err := rejectOptionLike(ref); err != nil {
		return err
	}

	ctx, cancel := commandContext(ctx)
	defer cancel()

	env, err := s.buildEnv()
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, s.binPath, archiveArgs(format, ref)...)
	cmd.Dir = repoPath
	cmd.Env = env
	cmd.Stdout = w

	// stderr 保留收集（复用 ExitError 的错误文案）；git 的报错只有几行，
	// 不会像 stdout 那样撑大内存。
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return &ExitError{Code: ee.ExitCode(), Stderr: stderr.String()}
		}
		return err
	}
	return nil
}
