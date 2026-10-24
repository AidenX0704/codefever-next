// Package mailer 负责发送邮件。
//
// 现有 PHP 实现（application/libraries/service/MessageService/Email/Driver/Mail.php）
// 调用系统的 mail 命令，并且**无条件返回 true** —— 发送失败上层完全感知不到。
//
// 新实现沿用同样的"系统 mail 命令"通道（不引入 SMTP 配置，保持零新增基础设施），
// 但：
//  1. 用 argv 数组调用，不经 shell；
//  2. 返回真实的退出状态，调用方可以记录失败；
//  3. 提供一个可注入的 Sender，便于测试与将来切换到 SMTP。
package mailer

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Message 是一封待发送的邮件。
type Message struct {
	To      string
	Subject string
	Body    string
	From    string
}

// Sender 是邮件发送接口。
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// LocalMailer 通过系统的 mail 命令发送邮件。
//
// 对应 PHP 侧的 `mail -r <from> -s <subject> <to>`，正文通过标准输入传入。
type LocalMailer struct {
	// Bin 是 mail 命令的路径（默认 "mail"）。
	Bin string
	// Timeout 是单次发送的超时（默认 30 秒）。
	Timeout time.Duration
}

// NewLocalMailer 创建基于系统 mail 命令的发送器。
func NewLocalMailer() *LocalMailer {
	return &LocalMailer{Bin: "mail", Timeout: 30 * time.Second}
}

// Send 发送一封邮件。
func (m *LocalMailer) Send(ctx context.Context, msg Message) error {
	bin := m.Bin
	if bin == "" {
		bin = "mail"
	}
	timeout := m.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 邮件头字段不允许换行（头注入）；收件人不能以 "-" 开头（会被 mail
	// 当成命令行选项）。
	for name, value := range map[string]string{"From": msg.From, "To": msg.To, "Subject": msg.Subject} {
		if err := validateHeaderField(name, value); err != nil {
			return err
		}
	}
	if strings.HasPrefix(strings.TrimSpace(msg.To), "-") {
		return errors.New("收件人地址非法")
	}

	args := []string{"-r", msg.From, "-s", msg.Subject, msg.To}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdin = strings.NewReader(msg.Body)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("发送邮件失败: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ErrNoSender 表示没有配置任何发送器。
var ErrNoSender = errors.New("未配置邮件发送器")

// RecordingSender 记录发送内容但不真正发出，用于开发与测试环境。
type RecordingSender struct {
	// Sent 保存已发送的邮件。
	Sent []Message
	// Err 是发送时要返回的错误（用于模拟失败）。
	Err error
}

// Send 记录邮件。
func (s *RecordingSender) Send(ctx context.Context, msg Message) error {
	if s.Err != nil {
		return s.Err
	}
	s.Sent = append(s.Sent, msg)
	return nil
}
