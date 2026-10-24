// Package gateway 实现两个 Go 网关（Git over HTTP / Git over SSH）与后端
// /apiv1/internal/* 之间的鉴权客户端。
//
// 这是跨进程硬契约：请求方法、JSON 字段名、Token 头与错误码语义都来自
// 既有实现，不能改动。把这段逻辑收敛为一个包，避免原先两份
// JsonHttpClient（http-gateway 与 ssh-gateway 各一份）继续复制粘贴。
package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"codefever-next/backend/internal/config"
)

const (
	// defaultTimeout 是单次鉴权请求的上限。
	//
	// 鉴权只做一次数据库查询，正常在毫秒级；10 秒足够覆盖数据库抖动，
	// 又不会让 git 客户端在鉴权服务不可用时无限等待。
	defaultTimeout = 10 * time.Second

	// maxResponseBytes 是响应体上限，避免后端异常时把网关内存打满。
	maxResponseBytes = 1 << 20
)

// AuthHTTPRequest 是 authHTTP 的请求体。
//
// 字段顺序与既有 JSON 保持一致（PHP 侧按数组插入顺序，Go 侧按结构体顺序）。
type AuthHTTPRequest struct {
	User   string `json:"user"`
	Pass   string `json:"pass"`
	Repo   string `json:"repo"`
	Action string `json:"action"`
}

// AuthHTTPResult 是 authHTTP 成功响应里的 data。
type AuthHTTPResult struct {
	Storage string `json:"storage"`
	UID     string `json:"uid"`
	User    string `json:"user"`
	Addr    string `json:"addr"`
	Path    string `json:"path"`
}

// AuthSSHRequest 是 authSSH 的请求体。
type AuthSSHRequest struct {
	UserID string `json:"userID"`
	Repo   string `json:"repo"`
	Action string `json:"action"`
}

// AuthSSHResult 是 authSSH 成功响应里的 data。
type AuthSSHResult struct {
	UID  string `json:"uid"`
	Path string `json:"path"`
}

// envelope 是后端统一响应信封。
type envelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// Error 表示一次被后端拒绝的鉴权请求。
//
// Status 是后端返回的 HTTP 状态码；Code 是业务码（0 表示成功）。
// HTTP 网关需要按状态码区分"仓库不存在（404）"与"凭据错误（其余 → 401）"，
// 这个语义来自 PHP 时代，不能丢。
type Error struct {
	Status  int
	Code    int
	Message string
}

// Error 实现 error。
func (e *Error) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("网关鉴权失败（HTTP %d，code %d）：%s", e.Status, e.Code, e.Message)
	}
	return fmt.Sprintf("网关鉴权失败（HTTP %d，code %d）", e.Status, e.Code)
}

// IsNotFound 判断错误是否代表"仓库不存在"。
func IsNotFound(err error) bool {
	var ge *Error
	return errors.As(err, &ge) && ge.Status == http.StatusNotFound
}

// Client 是后端内部回调的客户端。
//
// 并发安全：底层 http.Client 可被多个请求复用；字段在构造后只读。
type Client struct {
	http    *http.Client
	token   string
	authURL string
	sshURL  string
	log     *slog.Logger
}

// NewClient 根据共享的 env.yaml 构造客户端。
//
// 只依赖 gateway.* 与 http 超时，不需要 config.yaml。
func NewClient(env *config.Env, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		// 显式配置连接池，避免每个 git 请求新建 TCP 连接。
		http: &http.Client{
			Timeout: defaultTimeout,
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				MaxIdleConns:          32,
				MaxIdleConnsPerHost:   16,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: time.Second,
			},
		},
		token:   env.Gateway.Token,
		authURL: strings.TrimSpace(env.Gateway.AuthHTTP),
		sshURL:  strings.TrimSpace(env.Gateway.AuthSSH),
		log:     logger,
	}
}

// Validate 检查两个网关运行所需的全部配置。
func (c *Client) Validate() error {
	if err := c.validateToken(); err != nil {
		return err
	}
	if c.authURL == "" {
		return errors.New("env.yaml 缺少 gateway.authHTTP")
	}
	if c.sshURL == "" {
		return errors.New("env.yaml 缺少 gateway.authSSH")
	}
	return nil
}

// ValidateHTTP 只检查 Git over HTTP 网关所需配置。
func (c *Client) ValidateHTTP() error {
	if err := c.validateToken(); err != nil {
		return err
	}
	if c.authURL == "" {
		return errors.New("env.yaml 缺少 gateway.authHTTP")
	}
	return nil
}

// ValidateSSH 只检查 Git over SSH 网关所需配置。
func (c *Client) ValidateSSH() error {
	if err := c.validateToken(); err != nil {
		return err
	}
	if c.sshURL == "" {
		return errors.New("env.yaml 缺少 gateway.authSSH")
	}
	return nil
}

func (c *Client) validateToken() error {
	if strings.TrimSpace(c.token) == "" {
		return errors.New("env.yaml 缺少 gateway.token")
	}
	return nil
}

// AuthHTTP 调用 /apiv1/internal/authHTTP。
func (c *Client) AuthHTTP(ctx context.Context, req AuthHTTPRequest) (*AuthHTTPResult, error) {
	var out AuthHTTPResult
	if err := c.do(ctx, c.authURL, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AuthSSH 调用 /apiv1/internal/authSSH。
func (c *Client) AuthSSH(ctx context.Context, req AuthSSHRequest) (*AuthSSHResult, error) {
	var out AuthSSHResult
	if err := c.do(ctx, c.sshURL, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// do 发送一次 GET + JSON body 的内部回调请求。
//
// 方法必须是 GET：PHP 侧 internal 控制器只为 GET 注册了 authHTTP/authSSH，
// 改成 POST 会得到 0x0105。
func (c *Client) do(ctx context.Context, url string, payload, out any) error {
	if url == "" {
		return errors.New("未配置内部回调地址")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化鉴权请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("构造鉴权请求失败: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Token", c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("请求鉴权服务失败: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("读取鉴权响应失败: %w", err)
	}

	var env envelope
	decodeErr := json.Unmarshal(raw, &env)
	if resp.StatusCode != http.StatusOK || decodeErr != nil || env.Code != 0 {
		msg := env.Message
		if msg == "" && decodeErr != nil {
			msg = strings.TrimSpace(string(raw))
			if len(msg) > 200 {
				msg = msg[:200] + "…"
			}
		}
		return &Error{Status: resp.StatusCode, Code: env.Code, Message: msg}
	}
	if len(env.Data) == 0 || string(env.Data) == "null" {
		return &Error{Status: resp.StatusCode, Code: env.Code, Message: "鉴权响应缺少 data"}
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return fmt.Errorf("解析鉴权响应失败: %w", err)
	}
	return nil
}
