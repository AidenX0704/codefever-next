package httpapi

import (
	"context"
	"net/http"

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

	// 鉴权（PHP 侧在控制器构造函数中执行，先于 _remap 的动作查找）。
	if err := s.authenticate(ctx, ctrl.Auth, p, r); err != nil {
		contract.WriteError(w, p.ID, err)
		return
	}
	if u, err := s.resolveUser(ctx, p, r); err == nil {
		req.User = u
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

	if err := s.authenticate(ctx, ctrl.Auth, p, r); err != nil {
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

// authenticate 执行鉴权。
//
// 对应 PHP 侧 APIAuth::auth() 的三类判定，顺序为
// internal_gateway → web_user → admin_web_user，任一命中即通过。
func (s *Server) authenticate(ctx context.Context, level AuthLevel, p *contract.Parsed, r *http.Request) error {
	switch level {
	case AuthInternalGateway:
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
		return nil

	case AuthWebUser:
		_, err := s.currentUser(ctx, p, r)
		return err

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
	return user, nil
}

// resolveUser 解析当前登录用户，失败时返回 nil（不视为错误）。
func (s *Server) resolveUser(ctx context.Context, p *contract.Parsed, r *http.Request) (*domain.User, error) {
	return s.currentUser(ctx, p, r)
}
