// Package httpapi —— SPA 壳（shell）下发。
//
// 背景（新前端切换，2026-10）：
//
//	旧前端是"服务端渲染页面 + React SPA"双栈：/user/login、/doc/* 等 GET 由 Go 用
//	html/template 渲染；其余路径由 nginx 的 try_files 回落到 /app/index.html。
//	新前端把**所有界面**都收进 SPA 后，这些 GET 必须改为下发 SPA 壳，否则用户会
//	看到旧页面 —— 而旧页面正是本次要删除的东西。
//
// 设计要点：
//  1. 壳的路径由 main 在装配时注入（`ConfigureSPA`），httpapi 不依赖 config/appdir；
//  2. 文件内容只读一次并缓存在内存里（镜像内产物不可变；开发时改产物需重启，可接受）；
//  3. 找不到壳文件时返回 false，调用方回退到原有行为（删除前的这段时期必须可回滚）；
//  4. 响应头固定 `Cache-Control: no-cache` —— 壳引用的是**带内容哈希**的资源名，
//     壳本身若被缓存，发布后用户会一直加载旧壳并 404（旧 nginx 注释里记过这个坑）。
package httpapi

import (
	"net/http"
	"os"
	"sync"
)

var (
	spaMu      sync.RWMutex
	spaPath    string
	spaContent []byte
	spaLoadErr error
	spaLoaded  bool
)

// ConfigureSPA 指定 SPA 壳文件（index.html）的绝对路径。
//
// 传空串表示禁用（调用方回退到旧行为）。路径变更会清空缓存。
func ConfigureSPA(path string) {
	spaMu.Lock()
	defer spaMu.Unlock()
	if spaPath == path && spaLoaded {
		return
	}
	spaPath = path
	spaContent = nil
	spaLoadErr = nil
	spaLoaded = false
}

// spaShell 返回壳文件内容（首次调用时读取并缓存）。
func spaShell() ([]byte, bool) {
	spaMu.Lock()
	defer spaMu.Unlock()
	if spaLoaded {
		return spaContent, spaLoadErr == nil && len(spaContent) > 0
	}
	spaLoaded = true
	if spaPath == "" {
		spaLoadErr = os.ErrNotExist
		return nil, false
	}
	data, err := os.ReadFile(spaPath)
	if err != nil {
		spaLoadErr = err
		return nil, false
	}
	spaContent = data
	return spaContent, true
}

// ServeSPA 下发 SPA 壳；返回 false 表示没有可用的壳。
func ServeSPA(w http.ResponseWriter, r *http.Request) bool {
	return ServeSPAStatus(w, r, http.StatusOK)
}

// ServeSPAStatus 以指定状态码下发 SPA 壳（404/400 场景仍让前端渲染自己的提示页）。
func ServeSPAStatus(w http.ResponseWriter, r *http.Request, status int) bool {
	data, ok := spaShell()
	if !ok {
		return false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		// 非 GET 请求不该走到壳（例如表单 POST 由各自的 handler 处理）。
		return false
	}
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = w.Write(data)
	}
	return true
}

// SPAEnabled 报告当前是否配置了可用的 SPA 壳（供启动日志使用）。
func SPAEnabled() bool {
	_, ok := spaShell()
	return ok
}

// SPAPath 返回当前配置的壳路径（供启动日志使用）。
func SPAPath() string {
	spaMu.RLock()
	defer spaMu.RUnlock()
	return spaPath
}
