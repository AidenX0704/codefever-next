package handlers

import (
	"compress/gzip"
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"codefever-next/backend/internal/config"
	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
	"codefever-next/backend/internal/httpapi"
	"codefever-next/backend/internal/repo"
	"codefever-next/backend/internal/session"
)

// ArchiveHandler 实现 GET /api/repository/archive 的归档下载（二进制流）。
//
// 为什么走 RegisterPattern 专用路由而不是 Controller 动作表：
// ActionHandler 的返回值会被序列化进 JSON 信封（contract.WriteSuccess），
// 没有给二进制流留通道 —— 任何"信封机制输出二进制"的尝试都得先绕过
// Content-Type: application/json 与 Content-Length 的预计算。
// 因此仿照 /file/fetch 的既有先例注册字面量路由，自己写响应头与流。
//
// 鉴权：保持**会话鉴权、匿名拒绝** —— 与 /api/* 家族同一套会话 Cookie
// （session.ReadCookie + cc_sessions），未登录返回 JSON 信封 0x0101；
// 与 API 分发器不同的是这里不校验 Accept / Content-Type 头
// （浏览器 <a href> 下载不带 Accept: application/json）。
type ArchiveHandler struct {
	Cfg      *config.Config
	Sessions *session.Store
	Users    *repo.UserRepo
	Repos    *repo.RepositoryRepo
	Git      archiveGit
	Guard    *Guard
}

// NewArchiveHandler 创建归档下载处理器。
func NewArchiveHandler(
	cfg *config.Config,
	sessions *session.Store,
	users *repo.UserRepo,
	repos *repo.RepositoryRepo,
	git archiveGit,
	guard *Guard,
) *ArchiveHandler {
	return &ArchiveHandler{
		Cfg: cfg, Sessions: sessions, Users: users, Repos: repos, Git: git, Guard: guard,
	}
}

// archiveGit 是归档下载需要的最小 git 能力集合。
//
// *gitsvc.Service 天然满足该接口；收窄接口是为了单测可以注入假实现。
type archiveGit interface {
	RepoPath(rPath string) (string, error)
	RefExists(ctx context.Context, repoPath, ref string) (bool, error)
	Archive(ctx context.Context, repoPath, format, ref string, w io.Writer) error
}

// Register 注册归档下载路由。
//
// 字面量路径 /api/repository/archive 在 ServeMux 上比 /api/{ctrl}/{action}
// 更具体，优先级更高，因此不会被 API 分发器截走
// （与 /api/home/overview 用 page route 绕开分发器的既有思路一致）。
func (h *ArchiveHandler) Register(s *httpapi.Server) {
	s.RegisterPattern("/api/repository/archive", h.archive)
}

// archiveFormats 是 format 参数的白名单（含各格式对应的 git archive
// --format 值与响应 Content-Type）。
var archiveFormats = map[string]struct {
	gitFormat string
	mime      string
	ext       string
}{
	"zip":    {gitFormat: "zip", mime: "application/zip", ext: ".zip"},
	"tar.gz": {gitFormat: "tar", mime: "application/gzip", ext: ".tar.gz"},
}

// archive 处理 GET /api/repository/archive?repositoryId=&ref=&format=。
func (h *ArchiveHandler) archive(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1) 会话鉴权：匿名拒绝（0x0101，与 /api/* 的未登录响应一致）。
	user, ok := sessionUser(ctx, r, h.Sessions, h.Users, h.Cfg.Env.Session.Name)
	if !ok {
		writeArchiveError(w, contract.NewError(contract.CodeInvalidCredential))
		return
	}

	// 2) 参数。
	rKey := strings.TrimSpace(r.URL.Query().Get("repositoryId"))
	ref := strings.TrimSpace(r.URL.Query().Get("ref"))
	format := strings.TrimSpace(r.URL.Query().Get("format"))
	if rKey == "" || ref == "" || format == "" {
		writeArchiveError(w, contract.NewError(contract.CodeMissingParameter))
		return
	}
	fmtSpec, ok := archiveFormats[format]
	if !ok {
		// format 不在白名单里按缺参处理：前端下拉里只有两种取值。
		writeArchiveError(w, contract.NewError(contract.CodeMissingParameter))
		return
	}

	// 3) 权限：能读到该仓库即可下载（与 config 端点同款守卫）。
	if err := h.Guard.RequireRepository(ctx, rKey, user.Key, domain.UACRepoRead); err != nil {
		writeArchiveError(w, err)
		return
	}
	repoInfo, err := h.Repos.Get(ctx, rKey)
	if err != nil {
		writeArchiveError(w, err)
		return
	}
	repoPath, err := h.Git.RepoPath(repoInfo.Path)
	if err != nil {
		writeArchiveError(w, err)
		return
	}

	// 4) ref 必须真实存在（branch / tag / commit 均可）。
	//
	// 归档是流式响应：一旦开始写响应头就无法再改状态码，
	// 因此"ref 不存在"这类可预见的失败必须挡在流式输出**之前**。
	exists, err := h.Git.RefExists(ctx, repoPath, ref)
	if err != nil {
		writeArchiveError(w, err)
		return
	}
	if !exists {
		// ref 非法 → 0x0405（OperateFail）：目标不存在的"操作类"失败，
		// 与 deleteBranch 对不存在分支的既有语义一致。
		writeArchiveError(w, contract.NewError(contract.CodeOperateFail))
		return
	}

	// 5) 响应头：文件名 "<repo>-<ref><ext>"。
	//    ref 已通过存在性校验，但文件名里仍不能有引号/换行/路径分隔符。
	filename := sanitizeArchiveFilename(repoInfo.Name) + "-" + sanitizeArchiveFilename(ref) + fmtSpec.ext
	w.Header().Set("Content-Type", fmtSpec.mime)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)

	// 6) 流式输出。
	//
	// tar.gz：git 输出裸 tar，由 gzip.Writer 在响应侧实时压缩 ——
	// 两者都不落内存缓冲，多 GB 仓库的内存占用仍然恒定。
	//
	// 注意 gzip.Close() 必须在 Archive 成功后**显式立即**调用：
	// gzip 尾部（CRC + 长度）要 flush 进响应才算完整文件，而 defer 会在
	// 函数返回时才 Close —— 届时响应可能已被冲刷，客户端只会拿到一个
	// 只有 gzip 头的 0 字节流（部署冒烟实测踩过：zip 正常、tar.gz 空文件）。
	if format == "tar.gz" {
		gz := gzip.NewWriter(w)
		err := h.Git.Archive(ctx, repoPath, fmtSpec.gitFormat, ref, gz)
		if cerr := gz.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			// 响应头已写出，只能中断连接（客户端会看到截断的文件）。
			// 可预见的失败已在上面的 RefExists 一拦；走到这里的是磁盘 / git 级故障。
			slog.Error("归档下载中途失败",
				"repositoryId", rKey, "ref", ref, "format", format, "error", err)
		}
		return
	}
	if err := h.Git.Archive(ctx, repoPath, fmtSpec.gitFormat, ref, w); err != nil {
		slog.Error("归档下载中途失败",
			"repositoryId", rKey, "ref", ref, "format", format, "error", err)
	}
}

// writeArchiveError 输出 JSON 错误信封。
//
// 归档端点的失败响应与 /api/* 家族同构（同一套错误码与信封形状），
// 前端可以复用 NetworkHelper 的错误分支处理。
func writeArchiveError(w http.ResponseWriter, err error) {
	contract.WriteError(w, contract.NewRequestID(), err)
}

// sanitizeArchiveFilename 去掉文件名里不允许出现的字符。
//
// Content-Disposition 是引号包裹的字符串：引号、反斜杠、换行会破坏响应头；
// "/" 与 "\\" 会把浏览器导向子目录。其余字符（含中文）原样保留。
func sanitizeArchiveFilename(s string) string {
	replaced := strings.Map(func(r rune) rune {
		switch r {
		case '"', '\\', '\r', '\n', '/', '\x00':
			return '-'
		}
		return r
	}, s)
	// 全部被替换或本来就空时给一个确定的名字。
	if strings.Trim(replaced, "-. ") == "" {
		return "archive"
	}
	return replaced
}
