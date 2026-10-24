package mailer

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"
)

// FromEnv 根据环境变量选择邮件发送器。
//
//	CODEFEVER_SMTP_HOST      非空时启用 SMTP；否则回退系统 mail 命令
//	CODEFEVER_SMTP_PORT      默认 587（implicit TLS 时 465）
//	CODEFEVER_SMTP_USER / CODEFEVER_SMTP_PASSWORD
//	CODEFEVER_SMTP_FROM      默认发件人（Message.From 为空时使用）
//	CODEFEVER_SMTP_TLS       starttls（默认）| implicit | none
//
// 把选择逻辑集中在这里，API 与 worker 用同一套配置。
func FromEnv(defaultFrom string) Sender {
	host := strings.TrimSpace(os.Getenv("CODEFEVER_SMTP_HOST"))
	if host == "" {
		return NewLocalMailer()
	}
	port, _ := strconv.Atoi(strings.TrimSpace(os.Getenv("CODEFEVER_SMTP_PORT")))
	tlsMode := strings.ToLower(strings.TrimSpace(os.Getenv("CODEFEVER_SMTP_TLS")))
	implicit := tlsMode == "implicit" || tlsMode == "implicit-tls" || port == 465
	if port == 0 {
		if implicit {
			port = 465
		} else {
			port = 587
		}
	}
	from := strings.TrimSpace(os.Getenv("CODEFEVER_SMTP_FROM"))
	if from == "" {
		from = defaultFrom
	}
	return NewSMTPMailer(host, port,
		os.Getenv("CODEFEVER_SMTP_USER"),
		os.Getenv("CODEFEVER_SMTP_PASSWORD"),
		from, implicit)
}

// SMTPMailer 通过外部 SMTP 服务器发送邮件。
//
// 镜像里不再预装 MTA：PHP 时代依赖基础镜像的 sendmail，删除 PHP 后
// 该链路并不存在。SMTP 是跨平台、可测试且不依赖宿主机配置的方案；
// 未配置 SMTP 时回退到系统 mail 命令（LocalMailer）。
type SMTPMailer struct {
	// Host / Port 是 SMTP 服务器地址（例如 smtp.example.com:587）。
	Host string
	Port int
	// Username / Password 为空时不做认证（仅建议用于本机 relay）。
	Username string
	Password string
	// From 是默认发件人（Message.From 为空时使用）。
	From string
	// ImplicitTLS 为 true 时直接以 TLS 连接（常见于 465 端口）；
	// 否则使用 STARTTLS（常见于 587 端口，由 net/smtp 自动协商）。
	ImplicitTLS bool
	// Timeout 是单次发送超时（默认 30 秒）。
	Timeout time.Duration
}

// NewSMTPMailer 创建 SMTP 发送器。
func NewSMTPMailer(host string, port int, username, password, from string, implicitTLS bool) *SMTPMailer {
	if port <= 0 {
		if implicitTLS {
			port = 465
		} else {
			port = 587
		}
	}
	return &SMTPMailer{
		Host:        strings.TrimSpace(host),
		Port:        port,
		Username:    username,
		Password:    password,
		From:        from,
		ImplicitTLS: implicitTLS,
		Timeout:     30 * time.Second,
	}
}

// Send 实现 Sender。
func (m *SMTPMailer) Send(ctx context.Context, msg Message) error {
	if m.Host == "" {
		return errors.New("SMTP 主机为空")
	}
	from := strings.TrimSpace(msg.From)
	if from == "" {
		from = strings.TrimSpace(m.From)
	}
	if from == "" {
		return errors.New("发件人为空")
	}
	if err := validateHeaderField("To", msg.To); err != nil {
		return err
	}
	if err := validateHeaderField("From", from); err != nil {
		return err
	}
	if err := validateHeaderField("Subject", msg.Subject); err != nil {
		return err
	}

	timeout := m.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	addr := net.JoinHostPort(m.Host, strconv.Itoa(m.Port))
	envelopeFrom := envelopeAddress(from)

	var data strings.Builder
	data.WriteString("From: " + from + "\r\n")
	data.WriteString("To: " + msg.To + "\r\n")
	data.WriteString("Subject: " + msg.Subject + "\r\n")
	data.WriteString("MIME-Version: 1.0\r\n")
	data.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	data.WriteString("\r\n")
	data.WriteString(msg.Body)

	var auth smtp.Auth
	if m.Username != "" {
		auth = smtp.PlainAuth("", m.Username, m.Password, m.Host)
	}

	if m.ImplicitTLS {
		return m.sendImplicitTLS(ctx, addr, auth, envelopeFrom, msg.To, []byte(data.String()))
	}

	// smtp.SendMail 会在服务器支持时自动 STARTTLS；ctx 取消通过 goroutine
	// 关闭连接实现（SendMail 本身不接受 context）。
	errCh := make(chan error, 1)
	go func() {
		errCh <- smtp.SendMail(addr, auth, envelopeFrom, []string{msg.To}, []byte(data.String()))
	}()
	select {
	case <-ctx.Done():
		return fmt.Errorf("发送邮件超时: %w", ctx.Err())
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("发送邮件失败: %w", err)
		}
		return nil
	}
}

// sendImplicitTLS 处理 465（SMTPS）这类"先 TLS 再 SMTP"的服务器。
func (m *SMTPMailer) sendImplicitTLS(ctx context.Context, addr string, auth smtp.Auth, from, to string, data []byte) error {
	dialer := &net.Dialer{}
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
		ServerName: m.Host,
		MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		return fmt.Errorf("连接 SMTP 服务器失败: %w", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(m.Timeout))

	client, err := smtp.NewClient(conn, m.Host)
	if err != nil {
		return fmt.Errorf("创建 SMTP 客户端失败: %w", err)
	}
	defer client.Close()
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP 认证失败: %w", err)
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("SMTP MAIL FROM 失败: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("SMTP RCPT TO 失败: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA 失败: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("写入邮件内容失败: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("提交邮件内容失败: %w", err)
	}
	return client.Quit()
}

// validateHeaderField 拒绝邮件头注入（CR/LF）。
func validateHeaderField(name, value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s 不能包含换行", name)
	}
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s 不能为空", name)
	}
	return nil
}

// envelopeAddress 从 "Name <a@b>" 形式中取出纯邮箱地址。
func envelopeAddress(from string) string {
	if i := strings.LastIndex(from, "<"); i >= 0 {
		if j := strings.Index(from[i:], ">"); j > 0 {
			return strings.TrimSpace(from[i+1 : i+j])
		}
	}
	return strings.TrimSpace(from)
}
