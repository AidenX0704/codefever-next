package webhook

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// maxResponseBytes 是响应体的最大读取长度。
//
// 投递结果只需要状态码与少量正文，超长正文直接截断，避免异常的大响应撑爆内存。
const maxResponseBytes = 64 * 1024

// Request 是一次 webhook 投递请求。
type Request struct {
	// URL 是投递地址（cc_repository_webhooks.rw_url）。
	URL string
	// Headers 是请求头（含签名）。
	Headers map[string]string
	// Body 是请求体（JSON 文本）。
	Body string
}

// Response 是一次 webhook 投递响应。
type Response struct {
	// Status 是 HTTP 状态码。
	Status int
	// Headers 是响应头。
	Headers map[string][]string
	// Body 是响应正文。
	Body string
}

// Sender 负责把 webhook 请求发出去。
//
// 与 internal/mailer.Sender 同样的思路：把"怎么发"抽象成接口，
// 测试时可以换成内存实现，不必真的发 HTTP 请求。
type Sender interface {
	Send(ctx context.Context, req Request) (Response, error)
}

// HTTPSender 通过 HTTP POST 投递。
type HTTPSender struct {
	// Client 是 HTTP 客户端（为空时按 Timeout 现建一个）。
	Client *http.Client
	// Timeout 是单次投递的超时。
	Timeout time.Duration
}

// NewHTTPSender 创建 HTTP 投递器。
func NewHTTPSender(timeout time.Duration) *HTTPSender {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &HTTPSender{Client: &http.Client{Timeout: timeout}, Timeout: timeout}
}

// Send 投递一次请求。
func (s *HTTPSender) Send(ctx context.Context, req Request) (Response, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, req.URL, strings.NewReader(req.Body))
	if err != nil {
		return Response{}, fmt.Errorf("构造 webhook 请求失败: %w", err)
	}

	// 直接写 map 而不是用 Header.Set —— Go 的 Header.Set 会把键规范化成
	// "X-Codefever-Signature"，而 PHP 的 cURL 按原样发送 "X-CodeFever-Signature"。
	// HTTP 头名大小写不敏感，但保持逐字一致更便于对端与日志核对。
	httpReq.Header = make(http.Header, len(req.Headers))
	for k, v := range req.Headers {
		httpReq.Header[k] = []string{v}
	}
	httpReq.ContentLength = int64(len(req.Body))

	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: s.Timeout}
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("投递 webhook 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return Response{}, fmt.Errorf("读取 webhook 响应失败: %w", err)
	}
	return Response{Status: resp.StatusCode, Headers: resp.Header, Body: string(body)}, nil
}

// RecordingSender 记录发送内容但不真正发出，用于测试与本地调试。
type RecordingSender struct {
	// Sent 保存已发送的请求。
	Sent []Request
	// Status 是返回的状态码（默认 200）。
	Status int
	// Body 是返回的响应正文。
	Body string
	// Err 是要返回的错误（用于模拟失败）。
	Err error

	mu sync.Mutex
}

// Send 记录请求并返回预设结果。
func (s *RecordingSender) Send(_ context.Context, req Request) (Response, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Sent = append(s.Sent, req)
	if s.Err != nil {
		return Response{}, s.Err
	}
	status := s.Status
	if status == 0 {
		status = http.StatusOK
	}
	return Response{Status: status, Headers: map[string][]string{"Content-Type": {"application/json"}}, Body: s.Body}, nil
}

// Requests 返回已发送的请求副本。
func (s *RecordingSender) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Request, len(s.Sent))
	copy(out, s.Sent)
	return out
}
