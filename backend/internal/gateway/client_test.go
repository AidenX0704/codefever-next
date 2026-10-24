package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"codefever-next/backend/internal/config"
)

// newTestClient 构造指向测试服务器的客户端。
func newTestClient(t *testing.T, url string) *Client {
	t.Helper()
	return NewClient(&config.Env{
		Gateway: config.Gateway{
			Token:    "test-token",
			AuthHTTP: url + "/authHTTP",
			AuthSSH:  url + "/authSSH",
		},
	}, nil)
}

// TestAuthHTTPRequestShape 锁定 GET + JSON body + Token 头的跨进程契约。
func TestAuthHTTPRequestShape(t *testing.T) {
	var gotMethod, gotToken, gotAccept, gotCT string
	var gotBody AuthHTTPRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotToken = r.Header.Get("Token")
		gotAccept = r.Header.Get("Accept")
		gotCT = r.Header.Get("Content-Type")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		_, _ = w.Write([]byte(`{"request-id":"X","code":0,"message":"Success","data":` +
			`{"storage":"http://127.0.0.1:27555","uid":"u|r","user":"root","addr":"codefever.cn","path":"/git-storage/abc"}}`))
	}))
	defer srv.Close()

	client := newTestClient(t, srv.URL)
	res, err := client.AuthHTTP(context.Background(), AuthHTTPRequest{
		User: "root@codefever.cn", Pass: "123456", Repo: "/rootteam/demorepo.git", Action: "git-upload-pack",
	})
	if err != nil {
		t.Fatalf("AuthHTTP 失败: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Fatalf("method = %s, 期望 GET", gotMethod)
	}
	if gotToken != "test-token" || gotAccept != "application/json" || gotCT != "application/json" {
		t.Fatalf("请求头不符: token=%q accept=%q content-type=%q", gotToken, gotAccept, gotCT)
	}
	if gotBody.User != "root@codefever.cn" || gotBody.Repo != "/rootteam/demorepo.git" || gotBody.Action != "git-upload-pack" {
		t.Fatalf("请求体 = %+v", gotBody)
	}
	if res.Path != "/git-storage/abc" || res.UID != "u|r" || res.User != "root" {
		t.Fatalf("响应 = %+v", res)
	}
}

// TestAuthSSHResult 验证 authSSH 的响应解析。
func TestAuthSSHResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"message":"Success","data":{"uid":"u|r","path":"/git-storage/abc"}}`))
	}))
	defer srv.Close()

	res, err := newTestClient(t, srv.URL).AuthSSH(context.Background(), AuthSSHRequest{
		UserID: "u", Repo: "/rootteam/demorepo.git", Action: "git-receive-pack",
	})
	if err != nil {
		t.Fatalf("AuthSSH 失败: %v", err)
	}
	if res.UID != "u|r" || res.Path != "/git-storage/abc" {
		t.Fatalf("响应 = %+v", res)
	}
}

// TestAuthNotFound 验证 404 能被识别（HTTP 网关据此保留 404，其余失败改 401）。
func TestAuthNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":770,"message":"Repository Not Found","data":[]}`))
	}))
	defer srv.Close()

	_, err := newTestClient(t, srv.URL).AuthHTTP(context.Background(), AuthHTTPRequest{Repo: "x"})
	if err == nil {
		t.Fatal("期望返回错误")
	}
	if !IsNotFound(err) {
		t.Fatalf("IsNotFound = false, err = %v", err)
	}
	var ge *Error
	if !errors.As(err, &ge) || ge.Code != 770 {
		t.Fatalf("期望业务码 770，实际 %+v", err)
	}
}

// TestAuthBadCredential 验证非 404 失败不被误判为仓库不存在。
func TestAuthBadCredential(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"code":257,"message":"Invalid Login Credential","data":[]}`))
	}))
	defer srv.Close()

	_, err := newTestClient(t, srv.URL).AuthHTTP(context.Background(), AuthHTTPRequest{Repo: "x"})
	if err == nil || IsNotFound(err) {
		t.Fatalf("期望非 404 错误，实际 %v", err)
	}
}

// TestValidate 验证缺失配置会在启动时暴露，而不是运行时 panic。
func TestValidate(t *testing.T) {
	c := NewClient(&config.Env{}, nil)
	if err := c.Validate(); err == nil {
		t.Fatal("空配置应校验失败")
	}
}
