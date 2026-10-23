// Package webhook 负责把已入队的仓库 webhook 事件真正投递出去。
//
// 入队侧是 internal/events/webhook.go：领域事件发生时按 cc_repository_webhooks.rw_events
// 匹配后写入 cc_repository_webhook_events。本包是它的另一半 —— 领取队列里的事件、
// 组装请求、签名、POST 出去，并把投递结果写进 cc_repository_webhook_logs。
//
// 协议来源（逐字对齐 application/controllers/backend/repository_webhook.php）：
// PHP 侧由 cron 定时拉起本控制器，这里改由 worker 进程常驻驱动。
//
// 相对 PHP 实现修正了三点：
//
//  1. 失败不再无限重试：按退避策略重试，超过最大投递次数标记为死信；
//  2. 投递结果真实反映成功/失败（PHP 的日志写入恒定返回成功，且只把 200 当成功）；
//  3. 并发保护：领取使用 FOR UPDATE SKIP LOCKED，多个投递器不会重复投递同一条事件。
package webhook

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"codefever-next/backend/internal/contract"
)

// 投递状态（对应 cc_repository_webhook_events.rwe_status）。
const (
	// StatusPending 待投递。
	StatusPending = 1
	// StatusDelivering 投递中（已被某个投递器领取，租约未过期）。
	StatusDelivering = 2
	// StatusDead 超过最大投递次数，进入死信，不再投递。
	StatusDead = 3
)

// MaxAttempts 单条事件的最大投递次数（含首次投递）。
const MaxAttempts = 5

// DefaultTimeout 是单次 HTTP 投递的超时。
//
// 逐字对应 PHP 侧的 'timeout' => 30（Guzzle 的超时单位是秒）。
const DefaultTimeout = 30 * time.Second

// DefaultLeaseDuration 是领取事件后持有的租约时长。
//
// 投递进程崩溃时事件会停在"投递中"，租约到期后自动回到队列，
// 因此既不会丢事件，也不会永久卡住。
const DefaultLeaseDuration = 2 * time.Minute

// Deliverer 消费 webhook 事件队列并把事件投递出去。
type Deliverer struct {
	store  Store
	sender Sender
	log    *slog.Logger

	// Host 是站点地址（cfg.Site.Host），用于拼出仓库地址。
	Host string

	// PollInterval 是轮询间隔。
	PollInterval time.Duration
	// BatchSize 是每轮领取的事件数。
	BatchSize int
	// MaxAttempts 是单条事件的最大投递次数。
	MaxAttempts int
	// LeaseDuration 是领取事件后持有的租约时长。
	LeaseDuration time.Duration

	// now 取当前时间（测试可替换）。
	now func() time.Time
}

// NewDeliverer 创建 webhook 投递器。
//
// db 用于读取队列与写入日志；host 是站点地址（cfg.Site.Host）；
// sender 为 nil 时使用默认的 HTTP 投递器。
func NewDeliverer(db *sql.DB, host string, sender Sender, log *slog.Logger) *Deliverer {
	if log == nil {
		log = slog.Default()
	}
	if sender == nil {
		sender = NewHTTPSender(DefaultTimeout)
	}
	return &Deliverer{
		store:         NewMySQLStore(db),
		sender:        sender,
		log:           log,
		Host:          strings.TrimRight(host, "/"),
		PollInterval:  2 * time.Second,
		BatchSize:     20,
		MaxAttempts:   MaxAttempts,
		LeaseDuration: DefaultLeaseDuration,
		now:           time.Now,
	}
}

// NewDelivererWithStore 用自定义的存储与发送器创建投递器（测试用）。
func NewDelivererWithStore(store Store, sender Sender, host string, log *slog.Logger) *Deliverer {
	if log == nil {
		log = slog.Default()
	}
	if sender == nil {
		sender = NewHTTPSender(DefaultTimeout)
	}
	return &Deliverer{
		store:         store,
		sender:        sender,
		log:           log,
		Host:          strings.TrimRight(host, "/"),
		PollInterval:  2 * time.Second,
		BatchSize:     20,
		MaxAttempts:   MaxAttempts,
		LeaseDuration: DefaultLeaseDuration,
		now:           time.Now,
	}
}

// Run 持续消费 webhook 事件队列，直到 ctx 被取消。
//
// 与 internal/events.Worker.Run 是同一套骨架：取一批、处理一批，空转时按
// PollInterval 等待，收到退出信号立即返回。
func (d *Deliverer) Run(ctx context.Context) error {
	d.log.Info("webhook 投递器已启动", "interval", d.PollInterval, "batch", d.BatchSize)
	for {
		select {
		case <-ctx.Done():
			d.log.Info("webhook 投递器收到退出信号")
			return nil
		default:
		}

		n, err := d.ProcessOnce(ctx)
		if err != nil {
			d.log.Error("投递 webhook 批次失败", "error", err)
		}
		if n == 0 {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(d.PollInterval):
			}
		}
	}
}

// ProcessOnce 投递一批事件，返回本轮**成功投递**的事件数。
func (d *Deliverer) ProcessOnce(ctx context.Context) (int, error) {
	batch, err := d.store.Claim(ctx, d.BatchSize, d.now().Add(d.LeaseDuration))
	if err != nil {
		return 0, err
	}
	if len(batch) == 0 {
		return 0, nil
	}

	delivered := 0
	for _, ev := range batch {
		if ctx.Err() != nil {
			return delivered, ctx.Err()
		}
		if err := d.deliverOne(ctx, ev); err != nil {
			d.log.Error("webhook 投递失败", "event", ev.Type, "key", ev.Key, "error", err)
			continue
		}
		delivered++
	}
	return delivered, nil
}

// deliverOne 投递单条事件。
//
// 逐字对应 PHP 的投递循环：组装请求 → 发送 → 写日志 →
// 成功就删掉事件行，失败就保留下来等待重试。
// 区别在于失败不再无限重试：退避重试，超过上限标记为死信。
func (d *Deliverer) deliverOne(ctx context.Context, ev Event) error {
	// 领取时尝试次数已经超限（例如上一次投递进程恰好崩在最后一次投递里）：
	// 直接标记死信，不再投递 —— 否则这条事件会永远卡在队列里。
	if ev.Attempts > d.MaxAttempts {
		err := fmt.Errorf("投递次数已超过上限 %d", d.MaxAttempts)
		d.reschedule(ctx, ev, err)
		return err
	}

	// PHP 侧同一个 UUID 既做 X-CodeFever-Id 又做日志主键，这里保持一致。
	requestID := contract.NewRequestID()
	start := d.now()

	body, headers, err := d.buildRequest(ev, requestID)
	if err != nil {
		// 请求组装不出来（仓库 / 用户已被删除等）同样要留下痕迹。
		d.writeLog(ctx, ev, requestID, nil, "", Response{}, err, start)
		d.reschedule(ctx, ev, err)
		return err
	}

	resp, sendErr := d.sender.Send(ctx, Request{URL: ev.URL, Headers: headers, Body: body})
	d.writeLog(ctx, ev, requestID, headers, body, resp, sendErr, start)

	if sendErr != nil {
		d.reschedule(ctx, ev, sendErr)
		return sendErr
	}
	if !isSuccess(resp.Status) {
		err := fmt.Errorf("webhook 返回状态 %d", resp.Status)
		d.reschedule(ctx, ev, err)
		return err
	}

	// 成功：删除事件行（逐字对应 PHP 的 deleteRepositoryWebhookEvent）。
	if err := d.store.Delete(ctx, ev.Key); err != nil {
		return fmt.Errorf("删除 webhook 事件失败: %w", err)
	}
	return nil
}

// buildRequest 组装请求体与请求头。
func (d *Deliverer) buildRequest(ev Event, requestID string) (string, map[string]string, error) {
	if ev.URL == "" {
		return "", nil, errors.New("webhook 地址为空")
	}
	if ev.RepoName == "" || ev.GroupName == "" {
		return "", nil, errors.New("仓库或仓库组不存在")
	}
	if ev.UserName == "" {
		return "", nil, errors.New("触发用户不存在")
	}

	data, err := normalizeEventData(ev.Data)
	if err != nil {
		return "", nil, err
	}

	// 逐字对应 PHP 的 YAML_HOST . '/' . $group['g_name'] . '/' . $repository['r_name']。
	repoURL := d.Host + "/" + ev.GroupName + "/" + ev.RepoName

	body, err := buildBody(ev.Type, data, ev.RepoKey, repoURL, ev.UserKey, ev.UserName)
	if err != nil {
		return "", nil, err
	}
	return body, buildHeaders(ev.URL, ev.Type, requestID, body, ev.Secret), nil
}

// reschedule 记录一次投递失败：安排下次投递，或标记为死信。
//
// 这是 PHP 侧缺失的部分 —— PHP 失败后只是把事件留在表里，
// 下一次 cron 会**无条件重投**，既没有退避也没有次数上限。
func (d *Deliverer) reschedule(ctx context.Context, ev Event, cause error) {
	dead := ev.Attempts >= d.MaxAttempts
	next := d.now().Add(Backoff(ev.Attempts))
	if dead {
		next = time.Time{}
	}
	if err := d.store.Reschedule(ctx, ev.Key, next, dead); err != nil {
		d.log.Error("更新 webhook 事件状态失败", "key", ev.Key, "error", err)
		return
	}
	if dead {
		d.log.Error("webhook 事件进入死信", "event", ev.Type, "key", ev.Key,
			"attempts", ev.Attempts, "error", cause)
	}
}

// writeLog 写入一条投递日志。
//
// 逐字对应 PHP 的 addRepositoryWebhookLog()：请求与响应各存一份 JSON，
// 记录起止时间与 HTTP 状态码。
// 区别是状态码**如实反映**投递结果：传输失败记 0（没有拿到 HTTP 响应），
// 不再像 PHP 那样把异常一律当成 400 之外的"成功"。
func (d *Deliverer) writeLog(ctx context.Context, ev Event, requestID string,
	headers map[string]string, body string, resp Response, cause error, start time.Time) {
	l := &Log{
		ID:       requestID,
		Webhook:  ev.Webhook,
		Request:  encodeLogPayload(headers, body),
		Response: encodeLogResponse(resp, cause),
		Start:    float64(start.UnixNano()) / 1e9,
		End:      float64(d.now().UnixNano()) / 1e9,
		Status:   resp.Status,
	}
	if cause != nil {
		l.Status = 0
	}
	if err := d.store.AddLog(ctx, l); err != nil {
		d.log.Error("写入 webhook 投递日志失败", "key", ev.Key, "error", err)
	}
}

// Backoff 返回第 attempts 次投递失败后的等待时长。
//
// 采用阶梯退避：越往后间隔越长，避免对着不可用的地址死磕。
// 超过 MaxAttempts 的事件直接进死信，不再等待。
func Backoff(attempts int) time.Duration {
	switch {
	case attempts <= 1:
		return time.Minute
	case attempts == 2:
		return 5 * time.Minute
	case attempts == 3:
		return 15 * time.Minute
	case attempts == 4:
		return 30 * time.Minute
	default:
		return time.Hour
	}
}

// isSuccess 判断 HTTP 状态码是否算投递成功。
//
// PHP 侧只认 200，因此 201 / 204 这类成功响应会被当成失败并无限重投；
// 这里按 2xx 判定成功。
func isSuccess(status int) bool {
	return status >= 200 && status < 300
}
