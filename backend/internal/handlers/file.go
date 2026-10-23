package handlers

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/httpapi"
	"codefever-next/backend/internal/storage"
)

// FileHandler 实现 /file/fetch/<storage>/<key> 文件下载。
//
// 逐字对应 application/controllers/file.php:13-48：
//
//	存储名不在白名单 → 输出 "Not Allowed"
//	key 为空或文件不存在 → 输出 "Not Found"
//	否则 → Content-type: <mime> + 二进制内容
//
// 注意：PHP 用 exit('...') 输出这些文本，**HTTP 状态码是 200**（PHP 默认）。
// 这是既有行为，前端把 <img> 加载失败当作"没有头像"处理，因此新后端
// 逐字复刻（200 + 文本），不改成 404。
type FileHandler struct {
	Disk *storage.Disk
}

// NewFileHandler 创建文件下载处理器。
func NewFileHandler(disk *storage.Disk) *FileHandler { return &FileHandler{Disk: disk} }

// Register 注册文件下载路由。
func (h *FileHandler) Register(s *httpapi.Server) {
	s.RegisterPattern("/file/fetch/{storage}/{key}", h.fetch)
}

// RegisterWeb 把 file 控制器注册到服务器。
//
// 完整形态的 /file/fetch/<storage>/<key> 由 RegisterPattern 注册的精确路由处理
// （它比 web 分发器的通配更具体，优先级更高）；这里只处理段数不足的形态：
//
//	/file/fetch            → 无存储名 → "No storage"
//	/file/fetch/avatar     → 无文件名 → "Not Found"
//
// 逐字对应 application/controllers/file.php:13-48。
func (h *FileHandler) RegisterWeb(s *httpapi.Server) {
	s.RegisterWebController(&httpapi.WebController{
		Name: "file",
		Methods: map[string]http.HandlerFunc{
			"fetch": h.fetchWeb,
		},
	})
}

// allowedStorages 是允许下载的存储类型白名单。
//
// 逐字对应 PHP 的 $allowedStorage = ['avatar']。
var allowedStorages = map[string]bool{"avatar": true}

// fetch 处理文件下载请求。
func (h *FileHandler) fetch(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r.PathValue("storage"), r.PathValue("key"))
}

// fetchWeb 处理走 web 分发器的文件下载请求。
//
// PHP 侧用 $this->uri->segment(3) / segment(4) 取存储名与文件名，
// 多余的路径段被忽略，这里保持同样的取法。
func (h *FileHandler) fetchWeb(w http.ResponseWriter, r *http.Request) {
	storageName, fileKey := fileSegments(r.URL.Path)
	h.serve(w, storageName, fileKey)
}

// fileSegments 从路径里取出存储名与文件名。
//
// 逐字对应 application/controllers/file.php:15、20：
//
//	$storageName = trim($this->uri->segment(3));
//	$fileKey = trim($this->uri->segment(4));
func fileSegments(path string) (storageName, fileKey string) {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	if len(segments) > 2 {
		storageName = strings.TrimSpace(segments[2])
	}
	if len(segments) > 3 {
		fileKey = strings.TrimSpace(segments[3])
	}
	return storageName, fileKey
}

// serve 输出文件内容。
//
// 逐字对应 application/controllers/file.php:15-47：
// 无存储名 → "No storage"；无文件名 → "Not Found"；
// 存储名不在白名单 → "Not Allowed"；文件不存在 → "Not Found"。
//
// 注意这些文本响应的 HTTP 状态码都是 200（PHP 用 exit('...') 输出）。
func (h *FileHandler) serve(w http.ResponseWriter, storageName, fileKey string) {
	if storageName == "" {
		writePlain(w, "No storage")
		return
	}
	if fileKey == "" {
		writePlain(w, "Not Found")
		return
	}
	if !allowedStorages[storageName] {
		writePlain(w, "Not Allowed")
		return
	}

	content, err := h.Disk.Read(storageName, fileKey)
	if err != nil {
		// ErrNotFound / ErrPathEscape 都返回 "Not Found"，
		// 与 PHP 的 file_exists 判断一致（路径越界同样落到"文件不存在"）。
		_ = errors.Is(err, storage.ErrNotFound)
		writePlain(w, "Not Found")
		return
	}

	// MIME 类型按扩展名推断（PHP 用 finfo 检测实际内容）。
	// 头像只会是图片，这里用扩展名判断足以覆盖，且避免引入内容嗅探。
	contentType := mimeByExtension(filepath.Ext(fileKey))
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(content)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

// writePlain 输出纯文本（复刻 PHP 的 exit('...')）。
func writePlain(w http.ResponseWriter, text string) {
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(text)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(text))
}

// mimeByExtension 按扩展名返回 Content-Type。
func mimeByExtension(ext string) string {
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".bmp":
		return "image/bmp"
	case ".ico":
		return "image/x-icon"
	default:
		return "application/octet-stream"
	}
}

// maxAvatarSize 是头像大小上限（字节）。
//
// PHP 侧**没有任何校验**（application/controllers/api/user.php:182-200），
// 这是已知缺陷；新后端加上限制，避免任意大文件写入磁盘。
const maxAvatarSize = 5 << 20 // 5 MiB

// uploadAvatar 实现 POST /api/user/uploadAvatar。
//
// 逐字对应 application/controllers/api/user.php:182-200：
// 文件字段名恰为 "avatar"（前端 VaribleHelper.js:21 会剥掉 _BINARY 后缀）；
// 缺文件 → 0x0201；写入 u_avatar = 新建的 key。
//
// 与原实现的有意差异：增加了 MIME 与大小校验（原实现完全没有）。
func (h *UserHandler) uploadAvatar(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	file := req.Parsed.File("avatar")
	if file == nil {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if file.Size > maxAvatarSize {
		return nil, contract.NewError(contract.CodeAmountOutOfRange)
	}
	if !isAllowedImageType(file.MIMEType, file.Filename) {
		return nil, contract.NewError(contract.CodeAmountOutOfRange)
	}

	content, err := file.ReadAll()
	if err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	key := contract.NewKey()
	if err := h.Disk.Save("avatar", key, content); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}

	if err := h.Users.UpdateFields(ctx, u.Key, map[string]any{
		"u_avatar":  key,
		"u_updated": time.Now(),
	}); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return []any{}, nil
}

// isAllowedImageType 判断上传内容是否是允许的图片类型。
func isAllowedImageType(mimeType, filename string) bool {
	switch mimeType {
	case "image/png", "image/jpeg", "image/gif", "image/webp", "image/bmp", "image/svg+xml":
		return true
	}
	// 浏览器可能不提供 MIME 类型，此时退回按扩展名判断。
	switch filepath.Ext(filename) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".svg":
		return true
	}
	return false
}
