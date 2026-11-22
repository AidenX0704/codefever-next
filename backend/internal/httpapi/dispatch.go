package httpapi

import (
	"context"
	"net/http"
	"time"

	"codefever-next/backend/internal/auth"
	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
	"codefever-next/backend/internal/session"
)

// Request 是一次 API 请求的完整上下文。
type Request struct {
	// Parsed 是协议解析结果。
	Parsed *contract.Parsed
	// User 是当前登录用户（internal_gateway 端点为 nil）。
	User *domain.User
}

// UserKey 返回当前登录用户的主键。
func (r *Request) UserKey() string {
	if r == nil || r.User == nil {
		return ""
	}
	return r.User.Key
}

// contextKey 是本包使用的 context 键类型。
type contextKey string

const (
	ctxKeyRequest   contextKey = "codefever.request"
	ctxKeyRequestID contextKey = "codefever.request_id"
)

// WithRequest 把请求上下文存入 context。
func WithRequest(ctx context.Context, req *Request) context.Context {
	ctx = context.WithValue(ctx, ctxKeyRequest, req)
	if req != nil && req.Parsed != nil {
		ctx = context.WithValue(ctx, ctxKeyRequestID, req.Parsed.ID)
	}
	return ctx
}

// RequestFrom 从 context 取回请求上下文。
func RequestFrom(ctx context.Context) *Request {
	req, _ := ctx.Value(ctxKeyRequest).(*Request)
	return req
}

// requestIDFrom 从 context 取回 request-id。
func requestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(ctxKeyRequestID).(string)
	return id
}

// newRequestID 生成一个新的 request-id。
func newRequestID() string { return contract.NewRequestID() }

// writeInternalError 写出内部错误响应。
func writeInternalError(w http.ResponseWriter, requestID string, cause any) {
	contract.WriteJSON(w, requestID, http.StatusInternalServerError, &contract.Envelope{
		RequestID: requestID,
		Code:      int(contract.CodeRequestError),
		Message:   contract.CodeRequestError.Message(),
		Data:      []any{},
	})
}

// handleAPI 分发 /api/<controller>/<action> 请求。
func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	ctrlName := r.PathValue("ctrl")
	actionName := r.PathValue("action")
	if actionName == "" {
		actionName = "index"
	}

	ctrl, ok := s.controllers[ctrlName]
	if !ok {
		if !s.controllerNames[ctrlName] {
			// PHP 侧：控制器不存在时，CodeIgniter 路由失败并回落到
			// application/views/nopage/index.php，返回 HTML 404 页而非 JSON。
			// 实测 GET /api/nope/nope → 404 + Content-Type: text/html。
			s.handleNotFound(w, r)
			return
		}
		// 控制器存在但尚未移植动作 → 与 PHP 一致返回 JSON 0x0105。
		p, perr := contract.Parse(r)
		if perr != nil {
			contract.WriteError(w, p.ID, perr)
			return
		}
		contract.WriteError(w, p.ID, contract.NewError(contract.CodeEndpointNotFound))
		return
	}

	p, err := contract.Parse(r)
	if err != nil {
		contract.WriteError(w, p.ID, err)
		return
	}
	req := &Request{Parsed: p}
	ctx := WithRequest(r.Context(), req)
	// 把 request-id 写回请求本身，否则外层 logMiddleware / recoverMiddleware
	// 读到的仍是原始 ctx，日志里的 request_id 恒为空，排障断链。
	*r = *r.WithContext(ctx)

	// 鉴权（PHP 侧在控制器构造函数中执行，先于 _remap 的动作查找）。
	//
	// 性能整改 C6（2026-10）：鉴权与"取当前用户"合并为一次解析。
	//
	// 整改前这里是两次独立调用：
	//
	//	if err := s.authenticate(ctx, ctrl.Auth, p, r); err != nil { … }
	//	if u, err := s.resolveUser(ctx, p, r); err == nil { req.User = u }
	//
	// 而 authenticate 对 AuthWebUser / AuthAdminWebUser 内部都会调 currentUser()，
	// resolveUser 又只是 currentUser() 的别名 —— 于是每个已登录请求把
	// cc_sessions + cc_users 各查两遍（通用日志实测确认：同一请求里
	// 两条 SELECT cc_sessions、两条 SELECT cc_users）。
	//
	// 现在只在需要用户身份时解析一次，并把结果同时用于鉴权判定与 req.User。
	if err := s.authenticateWithUser(ctx, ctrl.Auth, p, r, req); err != nil {
		contract.WriteError(w, p.ID, err)
		return
	}

	// 动作查找：PHP 侧 method_exists 失败 → 0x0105。
	methods, ok := ctrl.Actions[actionName]
	if !ok {
		contract.WriteError(w, p.ID, contract.NewError(contract.CodeEndpointNotFound))
		return
	}
	handler, ok := methods[p.Method]
	if !ok {
		contract.WriteError(w, p.ID, contract.NewError(contract.CodeEndpointNotFound))
		return
	}

	data, err := handler(ctx, req)
	if err != nil {
		contract.WriteError(w, p.ID, err)
		return
	}
	contract.WriteSuccess(w, p.ID, data)
}

// handleInternal 分发 /apiv1/internal/<action> 请求。
func (s *Server) handleInternal(w http.ResponseWriter, r *http.Request) {
	actionName := r.PathValue("action")

	ctrl, ok := s.controllers["internal"]
	if !ok {
		if !s.controllerNames["internal"] {
			s.handleNotFound(w, r)
			return
		}
		p, perr := contract.Parse(r)
		if perr != nil {
			contract.WriteError(w, p.ID, perr)
			return
		}
		contract.WriteError(w, p.ID, contract.NewError(contract.CodeEndpointNotFound))
		return
	}

	p, err := contract.Parse(r)
	if err != nil {
		contract.WriteError(w, p.ID, err)
		return
	}
	req := &Request{Parsed: p}
	ctx := WithRequest(r.Context(), req)
	// 把 request-id 写回请求本身，否则外层 logMiddleware / recoverMiddleware
	// 读到的仍是原始 ctx，日志里的 request_id 恒为空，排障断链。
	*r = *r.WithContext(ctx)

	// /apiv1/internal/* 只接受网关 Token，不解析用户身份
	// （req.User 保持 nil，与整改前一致）。
	if err := s.authenticateWithUser(ctx, ctrl.Auth, p, r, req); err != nil {
		contract.WriteError(w, p.ID, err)
		return
	}

	methods, ok := ctrl.Actions[actionName]
	if !ok {
		contract.WriteError(w, p.ID, contract.NewError(contract.CodeEndpointNotFound))
		return
	}
	handler, ok := methods[p.Method]
	if !ok {
		contract.WriteError(w, p.ID, contract.NewError(contract.CodeEndpointNotFound))
		return
	}

	data, err := handler(ctx, req)
	if err != nil {
		contract.WriteError(w, p.ID, err)
		return
	}
	contract.WriteSuccess(w, p.ID, data)
}

// authenticateWithUser 执行鉴权，并在需要用户身份时把解析结果写入 req.User。
//
// 对应 PHP 侧 APIAuth::auth() 的三类判定，顺序为
// internal_gateway → web_user → admin_web_user，任一命中即通过。
//
// 性能整改 C6（2026-10）：本函数取代了原先的 authenticate + resolveUser 两次调用。
// 语义差异只有一处且是收敛的：整改前 resolveUser 的失败被**静默忽略**
// （`if u, err := s.resolveUser(...); err == nil`），但 authenticate 对
// web_user / admin_web_user 已经用同样的凭据做过一次判定，因此
// "鉴权通过但解析失败"在原实现里不可能发生 —— 两次调用查的是同一份
// Cookie、同一个 session 行。合并后不再有这个空窗。
func (s *Server) authenticateWithUser(ctx context.Context, level AuthLevel, p *contract.Parsed, r *http.Request, req *Request) error {
	switch level {
	case AuthInternalGateway:
		// 内部网关用独立 Token，不解析用户身份（req.User 保持 nil）。
		if auth.CompareToken(p.Token, s.cfg.Env.Gateway.Token) {
			return nil
		}
		return contract.NewError(contract.CodeInvalidCredential)

	case AuthAdminWebUser:
		user, err := s.currentUser(ctx, p, r)
		if err != nil {
			return err
		}
		if !user.Admin {
			return contract.NewError(contract.CodeInvalidCredential)
		}
		req.User = user
		return nil

	case AuthWebUser:
		user, err := s.currentUser(ctx, p, r)
		if err != nil {
			return err
		}
		req.User = user
		return nil

	default:
		return contract.NewError(contract.CodeInvalidCredential)
	}
}

// currentUser 解析当前登录用户。
func (s *Server) currentUser(ctx context.Context, p *contract.Parsed, r *http.Request) (*domain.User, error) {
	key := session.ReadCookie(r, s.cfg.Env.Session.Name)
	if key == "" {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	sess, err := s.sessions.Get(ctx, key)
	if err != nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	user, err := s.users.Get(ctx, sess.UKey)
	if err != nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	// 已软删除/禁用的账号立即失效，即使它手里还有未过期的会话 Cookie。
	if user.Status == domain.CommonStatusDelete {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	// 滑动续期：每 5 分钟最多刷新一次，避免每个请求都写库。
	// 与 PHP CI2 会话的滑动过期语义对齐（持续活跃用户不会在第 10 天被强制登出）。
	if time.Since(sess.LastSeen) > 5*time.Minute {
		_ = s.sessions.Touch(ctx, sess.Key)
	}
	return user, nil
}
