package handlers

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestIsSecureRequest 锁定 Cookie 的 Secure 判定依据。
//
// 这是切流演练抓到的真实缺陷：原先按"环境名是否为 production"决定 Secure，
// 而本项目的默认部署是**明文 HTTP**（容器内 nginx 监听 80）。
// 于是登录接口返回成功，但客户端不会回传带 Secure 标志的 Cookie，
// 后续每个请求都是 0x0101 —— 表现为"切流后全部 403"。
//
// 因此 Secure 必须由**请求协议**决定。
func TestIsSecureRequest(t *testing.T) {
	// 明文 HTTP → 不打 Secure。
	plain := httptest.NewRequest(http.MethodGet, "http://localhost:9080/user/login", nil)
	if isSecureRequest(plain) {
		t.Error("明文 HTTP 请求不应设置 Secure")
	}

	// 直连 TLS → 打 Secure。
	tlsReq := httptest.NewRequest(http.MethodGet, "https://localhost/user/login", nil)
	tlsReq.TLS = &tls.ConnectionState{}
	if !isSecureRequest(tlsReq) {
		t.Error("TLS 请求应设置 Secure")
	}

	// 前置 TLS 终止代理 → 打 Secure。
	proxied := httptest.NewRequest(http.MethodGet, "http://localhost/user/login", nil)
	proxied.Header.Set("X-Forwarded-Proto", "https")
	if !isSecureRequest(proxied) {
		t.Error("X-Forwarded-Proto: https 时应设置 Secure")
	}

	// 大小写不敏感。
	upper := httptest.NewRequest(http.MethodGet, "http://localhost/user/login", nil)
	upper.Header.Set("X-Forwarded-Proto", "HTTPS")
	if !isSecureRequest(upper) {
		t.Error("X-Forwarded-Proto 应大小写不敏感")
	}

	// nil 安全。
	if isSecureRequest(nil) {
		t.Error("nil 请求不应设置 Secure")
	}
}
