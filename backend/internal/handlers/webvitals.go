// Package handlers —— Web 性能指标（Core Web Vitals）上报端点。
//
// 2026-10 性能整改 B1。
//
// 背景：前端 www/view 早已在 package.json 声明 web-vitals 依赖，但从未接线，
// 导致上一轮性能审查的所有前端结论都无法持续追踪。本端点承接
// navigator.sendBeacon 上报的 CWV 数据。
//
// 设计取舍：
//   - **只写日志，不入库**。CWV 是运维观测数据，不是业务数据；入库需要建表、
//     写迁移、加保留期清理（见 004 索引迁移与保留期任务的工作量），而日志已经
//     足以支撑"发布前后对比"这一唯一用途。若将来需要聚合看板，再补一张表。
//   - **不鉴权**。指标会在登录页（未登录状态）产生，鉴权会丢掉最关键的首屏数据；
//     端点只接受极小的 JSON 且只写日志，无业务副作用。
//   - **不在业务 API 契约内**。走 page route 而非 /api/{ctrl}/{action}，
//     因此不受 contract 信封、Accept 头校验约束（sendBeacon 发的是
//     Content-Type: text/plain、Accept: */*，进 contract.Parse 会被
//     CodeHeaderRequired 拒掉）。
//   - **限长**。单个上报超过 4 KB 直接丢弃，避免日志被灌爆。
package handlers

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"codefever-next/backend/internal/httpapi"
)

// webVitalsPath 是上报端点路径。
//
// 与 /api/{ctrl}/{action} 形状相同，但注册为**字面量 page route**：
// httpapi.Server 的字面量路由优先于通配路由，因此不会被 API 分发器截走。
const webVitalsPath = "/api/web-vitals"

// webVitalsMaxBody 是允许的最大上报体积。
const webVitalsMaxBody = 4 << 10

// WebVitalsHandler 承接前端 CWV 上报。
type WebVitalsHandler struct {
	logger *slog.Logger
}

// NewWebVitalsHandler 创建上报处理器。
func NewWebVitalsHandler(logger *slog.Logger) *WebVitalsHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &WebVitalsHandler{logger: logger}
}

// Register 注册路由。
func (h *WebVitalsHandler) Register(s *httpapi.Server) {
	s.RegisterPage(webVitalsPath, h.report)
}

// webVitalsPayload 是前端上报的字段。
//
// 与 www/view/src/index.js 的 report() 一一对应。
type webVitalsPayload struct {
	Name   string  `json:"name"`
	Value  float64 `json:"value"`
	Rating string  `json:"rating"`
	ID     string  `json:"id"`
	Path   string  `json:"path"`
}

// report 处理 POST /api/web-vitals。
//
// 无论上报内容是否可解析都返回 204：前端用 sendBeacon，拿不到响应；
// 返回非 2xx 只会污染访问日志的错误率统计。
func (h *WebVitalsHandler) report(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, webVitalsMaxBody+1))
	if err != nil || len(body) == 0 || len(body) > webVitalsMaxBody {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var p webVitalsPayload
	if err := json.Unmarshal(body, &p); err != nil || p.Name == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	h.logger.Info("web-vitals",
		"metric", p.Name,
		"value", p.Value,
		"rating", p.Rating,
		"metric_id", p.ID,
		"path", p.Path,
	)
	w.WriteHeader(http.StatusNoContent)
}
