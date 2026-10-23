package webhook

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// 测试替身
// ---------------------------------------------------------------------------

// memEvent 是内存队列里的一条事件。
type memEvent struct {
	Event
	status   int
	next     time.Time
	dead     bool
	attempts int
}

// memoryStore 是内存版队列存储，语义与 MySQLStore 一致。
type memoryStore struct {
	mu     sync.Mutex
	events map[string]*memEvent
	logs   []*Log
}

func newMemoryStore(events ...Event) *memoryStore {
	s := &memoryStore{events: map[string]*memEvent{}}
	for _, ev := range events {
		s.events[ev.Key] = &memEvent{Event: ev, status: StatusPending}
	}
	return s
}

// Claim 实现 Store。
func (s *memoryStore) Claim(_ context.Context, limit int, leaseUntil time.Time) ([]Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	var out []Event
	for _, ev := range s.events {
		if len(out) >= limit {
			break
		}
		if ev.dead {
			continue
		}
		if !ev.next.IsZero() && ev.next.After(now) {
			continue
		}
		ev.status = StatusDelivering
		ev.attempts++
		ev.next = leaseUntil
		ev.Event.Attempts = ev.attempts
		out = append(out, ev.Event)
	}
	return out, nil
}

// Delete 实现 Store。
func (s *memoryStore) Delete(_ context.Context, eventKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.events, eventKey)
	return nil
}

// Reschedule 实现 Store。
func (s *memoryStore) Reschedule(_ context.Context, eventKey string, next time.Time, dead bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ev, ok := s.events[eventKey]
	if !ok {
		return errors.New("事件不存在")
	}
	if dead {
		ev.dead = true
		ev.status = StatusDead
		ev.next = time.Time{}
		return nil
	}
	ev.status = StatusPending
	ev.next = next
	return nil
}

// AddLog 实现 Store。
func (s *memoryStore) AddLog(_ context.Context, l *Log) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs = append(s.logs, l)
	return nil
}

// expire 模拟退避到期：把可投递时间清空。
func (s *memoryStore) expire() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ev := range s.events {
		if !ev.dead {
			ev.next = time.Time{}
		}
	}
}

// state 返回某条事件的当前状态。
func (s *memoryStore) state(key string) (status int, attempts int, dead bool, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ev, found := s.events[key]
	if !found {
		return 0, 0, false, false
	}
	return ev.status, ev.attempts, ev.dead, true
}

// remaining 返回队列里还剩多少条事件。
func (s *memoryStore) remaining() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.events)
}

// discardLogger 返回丢弃全部日志的 logger。
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newTestEvent 造一条测试事件。
func newTestEvent(key, url string) Event {
	return Event{
		Key:       key,
		RepoKey:   "repo00000000000000000000000001",
		Webhook:   "hook00000000000000000000000001",
		Type:      "mergeRequest:create",
		Data:      `{"gKey":"group01","rKey":"repo00000000000000000000000001","id":7,"mrKey":"mr000000000000000000000000001","sourceBranch":"feature/x","targetBranch":"master"}`,
		URL:       url,
		Secret:    "s3cr3t",
		UserKey:   "user00000000000000000000000001",
		RepoName:  "demo",
		GroupName: "team",
		UserName:  "Bob",
	}
}

// ---------------------------------------------------------------------------
// 签名
// ---------------------------------------------------------------------------

// TestSignature 校验签名算法。
//
// 逐字对应 PHP 的 _signature()：md5($body . $secret)，
// 请求头里的取值是 'md5=' 前缀 + 十六进制摘要。
func TestSignature(t *testing.T) {
	body := `{"event":"mergeRequest:create","data":{}}`
	secret := "s3cr3t"

	sum := md5.Sum([]byte(body + secret))
	want := hex.EncodeToString(sum[:])

	if got := signature(body, secret); got != want {
		t.Errorf("签名 = %q，期望 %q", got, want)
	}
	if got := signature(body, secret); len(got) != 32 {
		t.Errorf("签名长度 = %d，期望 32", len(got))
	}
}

// TestSignatureEmptySecret 校验空密钥的签名。
//
// PHP 的 _signature() 用的是**假值**判断，
// 因此密钥为 "" 或 "0" 时都按空串拼接。
func TestSignatureEmptySecret(t *testing.T) {
	body := `{"event":"x"}`
	sum := md5.Sum([]byte(body))
	want := hex.EncodeToString(sum[:])

	if got := signature(body, ""); got != want {
		t.Errorf("空密钥签名 = %q，期望 %q", got, want)
	}
	if got := signature(body, "0"); got != want {
		t.Errorf("密钥 \"0\" 的签名 = %q，期望 %q", got, want)
	}
}

// TestBuildHeaders 校验请求头。
//
// 逐字对应 PHP 的 $headers：固定的四个头 + 三个 X-CodeFever-* 头。
func TestBuildHeaders(t *testing.T) {
	body := `{"event":"mergeRequest:create"}`
	headers := buildHeaders("http://127.0.0.1:9000/hook", "mergeRequest:create", "REQ-1", body, "s3cr3t")

	want := map[string]string{
		"Request-URL":           "http://127.0.0.1:9000/hook",
		"Request-Method":        "POST",
		"Content-Type":          "application/json",
		"Accept":                "application/json",
		"User-Agent":            "CodeFever-Webhook",
		"X-CodeFever-Id":        "REQ-1",
		"X-CodeFever-Event":     "mergeRequest:create",
		"X-CodeFever-Signature": "md5=" + signature(body, "s3cr3t"),
	}
	for k, v := range want {
		if got := headers[k]; got != v {
			t.Errorf("请求头 %s = %q，期望 %q", k, got, v)
		}
	}
	if len(headers) != len(want) {
		t.Errorf("请求头数量 = %d，期望 %d", len(headers), len(want))
	}
}

// ---------------------------------------------------------------------------
// 请求体
// ---------------------------------------------------------------------------

// TestNormalizeEventDataMapping 校验事件数据的字段映射。
//
// 逐字对应 PHP 的 _eventDataMap / _normalizeEventData()：
// rKey → repositoryId、gKey → groupId、mrKey → mergerequestId …，
// 没有命中的字段原样保留。
func TestNormalizeEventDataMapping(t *testing.T) {
	raw := `{"gKey":"group01","rKey":"repo01","id":7,"mrKey":"mr01","sourceRKey":"src01","sourceGKey":"sg01","mrrKey":"rv01","uid":"u01","other":"keep"}`

	got, err := normalizeEventData(raw)
	if err != nil {
		t.Fatalf("归一化失败: %v", err)
	}

	want := `{"groupId":"group01","repositoryId":"repo01","id":7,"mergerequestId":"mr01",` +
		`"sourceRepositoryId":"src01","sourceGroupId":"sg01","reviewId":"rv01","userId":"u01","other":"keep"}`
	if got != want {
		t.Errorf("事件数据 = %s\n期望        = %s", got, want)
	}
}

// TestBuildBody 校验请求体的结构与字段顺序。
//
// 逐字对应 PHP 的 json_encode([...])：
// event / data / repository / sender，顺序固定。
func TestBuildBody(t *testing.T) {
	body, err := buildBody("mergeRequest:create",
		`{"repositoryId":"repo01","groupId":"group01"}`,
		"repo01", "http://localhost/team/demo", "user01", "Bob")
	if err != nil {
		t.Fatalf("组装请求体失败: %v", err)
	}

	want := `{"event":"mergeRequest:create","data":{"repositoryId":"repo01","groupId":"group01"},` +
		`"repository":{"id":"repo01","url":"http:\/\/localhost\/team\/demo"},` +
		`"sender":{"id":"user01","name":"Bob"}}`
	if body != want {
		t.Errorf("请求体 = %s\n期望     = %s", body, want)
	}
}

// TestPHPJSONEncoding 校验 PHP 风格的 JSON 转义。
//
// PHP 的 json_encode() 把正斜杠转义成 \/、非 ASCII 转义成 \uXXXX；
// 空数组与空对象都编码成 []。
func TestPHPJSONEncoding(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"正斜杠", `{"url":"a/b"}`, `{"url":"a\/b"}`},
		{"中文", `{"name":"张三"}`, `{"name":"\u5f20\u4e09"}`},
		{"空对象", `{}`, `[]`},
		{"数字保持原样", `{"id":9007199254740993}`, `{"id":9007199254740993}`},
		{"嵌套保序", `{"b":1,"a":{"d":2,"c":3}}`, `{"b":1,"a":{"d":2,"c":3}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			value, err := decodeJSON(c.raw)
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			if got := encodeJSON(value); got != c.want {
				t.Errorf("编码 = %s，期望 %s", got, c.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 退避与死信
// ---------------------------------------------------------------------------

// TestBackoff 校验退避时长随尝试次数递增。
func TestBackoff(t *testing.T) {
	prev := time.Duration(0)
	for attempt := 1; attempt <= 6; attempt++ {
		got := Backoff(attempt)
		if got <= 0 {
			t.Fatalf("第 %d 次的退避时长 = %s，期望为正", attempt, got)
		}
		if got < prev {
			t.Errorf("第 %d 次的退避时长 = %s，比上一次（%s）短", attempt, got, prev)
		}
		prev = got
	}
	if got := Backoff(1); got != time.Minute {
		t.Errorf("首次失败的退避 = %s，期望 1m", got)
	}
}

// TestFailureReschedulesWithBackoff 校验失败后进入退避而不是立即重投。
//
// PHP 侧失败的事件会被下一次 cron 无条件重投（没有退避），
// 这里第一次失败后应当安排一个将来的重投时间。
func TestFailureReschedulesWithBackoff(t *testing.T) {
	ev := newTestEvent("ev0000000000000000000000000001", "http://127.0.0.1:1/hook")
	store := newMemoryStore(ev)
	sender := &RecordingSender{Status: 500, Body: "boom"}
	d := NewDelivererWithStore(store, sender, "http://localhost", discardLogger())

	n, err := d.ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("投递失败: %v", err)
	}
	if n != 0 {
		t.Errorf("成功投递数 = %d，期望 0", n)
	}

	status, attempts, dead, ok := store.state(ev.Key)
	if !ok {
		t.Fatal("事件应仍在队列里（PHP 侧失败不删事件）")
	}
	if dead {
		t.Error("第一次失败不应进入死信")
	}
	if status != StatusPending {
		t.Errorf("状态 = %d，期望 %d（待投递）", status, StatusPending)
	}
	if attempts != 1 {
		t.Errorf("尝试次数 = %d，期望 1", attempts)
	}

	// 退避未到期前不应再被领取。
	if n, _ := d.ProcessOnce(context.Background()); n != 0 {
		t.Errorf("退避期内又投递了 %d 条，期望 0", n)
	}

	// 日志必须如实记录失败状态码。
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.logs) != 1 {
		t.Fatalf("日志条数 = %d，期望 1", len(store.logs))
	}
	if store.logs[0].Status != 500 {
		t.Errorf("日志状态码 = %d，期望 500", store.logs[0].Status)
	}
	if !strings.Contains(store.logs[0].Request, "X-CodeFever-Signature") {
		t.Errorf("日志请求里应有签名头：%s", store.logs[0].Request)
	}
}

// TestDeadLetterAfterMaxAttempts 校验超过最大投递次数后进入死信。
func TestDeadLetterAfterMaxAttempts(t *testing.T) {
	ev := newTestEvent("ev0000000000000000000000000002", "http://127.0.0.1:1/hook")
	store := newMemoryStore(ev)
	sender := &RecordingSender{Status: 503}
	d := NewDelivererWithStore(store, sender, "http://localhost", discardLogger())

	for i := 1; i <= MaxAttempts; i++ {
		store.expire() // 模拟退避到期
		if _, err := d.ProcessOnce(context.Background()); err != nil {
			t.Fatalf("第 %d 轮投递出错: %v", i, err)
		}
	}

	status, attempts, dead, ok := store.state(ev.Key)
	if !ok {
		t.Fatal("死信事件应保留在队列里以便排查")
	}
	if !dead || status != StatusDead {
		t.Errorf("状态 = %d（dead = %v），期望死信 %d", status, dead, StatusDead)
	}
	if attempts != MaxAttempts {
		t.Errorf("尝试次数 = %d，期望 %d", attempts, MaxAttempts)
	}

	// 死信之后不应再被投递。
	store.expire()
	if n, _ := d.ProcessOnce(context.Background()); n != 0 {
		t.Errorf("死信事件又被投递了 %d 次", n)
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.logs) != MaxAttempts {
		t.Errorf("日志条数 = %d，期望 %d", len(store.logs), MaxAttempts)
	}
	for _, l := range store.logs {
		if l.Status != 503 {
			t.Errorf("日志状态码 = %d，期望 503（失败要如实记录）", l.Status)
		}
	}
}

// TestTransportErrorRecordedAsFailure 校验传输失败被记成失败而不是成功。
//
// PHP 侧的异常分支把状态码记成 400，但结果依然是"失败"；
// 这里记 0（没有拿到 HTTP 响应），并触发退避重试。
func TestTransportErrorRecordedAsFailure(t *testing.T) {
	ev := newTestEvent("ev0000000000000000000000000003", "http://127.0.0.1:1/hook")
	store := newMemoryStore(ev)
	sender := &RecordingSender{Err: errors.New("connection refused")}
	d := NewDelivererWithStore(store, sender, "http://localhost", discardLogger())

	if n, _ := d.ProcessOnce(context.Background()); n != 0 {
		t.Errorf("成功投递数 = %d，期望 0", n)
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.logs) != 1 {
		t.Fatalf("日志条数 = %d，期望 1", len(store.logs))
	}
	if store.logs[0].Status != 0 {
		t.Errorf("日志状态码 = %d，期望 0（没有 HTTP 响应）", store.logs[0].Status)
	}
	if !strings.Contains(store.logs[0].Response, "connection refused") {
		t.Errorf("日志响应里应记录错误原因：%s", store.logs[0].Response)
	}
}

// ---------------------------------------------------------------------------
// 成功路径
// ---------------------------------------------------------------------------

// TestSuccessDeletesEventAndWritesLog 校验成功后删除事件并写入日志。
func TestSuccessDeletesEventAndWritesLog(t *testing.T) {
	ev := newTestEvent("ev0000000000000000000000000004", "http://127.0.0.1:9000/hook")
	store := newMemoryStore(ev)
	sender := &RecordingSender{Status: 200, Body: "ok"}
	d := NewDelivererWithStore(store, sender, "http://localhost", discardLogger())

	n, err := d.ProcessOnce(context.Background())
	if err != nil {
		t.Fatalf("投递失败: %v", err)
	}
	if n != 1 {
		t.Fatalf("成功投递数 = %d，期望 1", n)
	}
	if store.remaining() != 0 {
		t.Error("成功后事件行应被删除")
	}

	sent := sender.Requests()
	if len(sent) != 1 {
		t.Fatalf("发送次数 = %d，期望 1", len(sent))
	}
	if sent[0].URL != ev.URL {
		t.Errorf("投递地址 = %q，期望 %q", sent[0].URL, ev.URL)
	}
	if !strings.Contains(sent[0].Body, `"event":"mergeRequest:create"`) {
		t.Errorf("请求体缺少事件类型：%s", sent[0].Body)
	}
	if !strings.Contains(sent[0].Body, `"repositoryId":"repo00000000000000000000000001"`) {
		t.Errorf("请求体缺少映射后的字段：%s", sent[0].Body)
	}
	if !strings.Contains(sent[0].Body, `"url":"http:\/\/localhost\/team\/demo"`) {
		t.Errorf("请求体缺少仓库地址：%s", sent[0].Body)
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.logs) != 1 {
		t.Fatalf("日志条数 = %d，期望 1", len(store.logs))
	}
	log := store.logs[0]
	if log.Status != 200 {
		t.Errorf("日志状态码 = %d，期望 200", log.Status)
	}
	if log.Webhook != ev.Webhook {
		t.Errorf("日志 webhook = %q，期望 %q", log.Webhook, ev.Webhook)
	}
	if log.ID == "" {
		t.Error("日志主键不应为空")
	}
	if !strings.Contains(log.Request, log.ID) {
		t.Errorf("日志请求里应带上与 X-CodeFever-Id 相同的主键：%s", log.Request)
	}
	if log.End < log.Start {
		t.Errorf("结束时间 %v 早于开始时间 %v", log.End, log.Start)
	}
}

// TestMissingRepositoryIsFailure 校验仓库缺失时不投递且记录失败。
func TestMissingRepositoryIsFailure(t *testing.T) {
	ev := newTestEvent("ev0000000000000000000000000005", "http://127.0.0.1:9000/hook")
	ev.RepoName = ""
	store := newMemoryStore(ev)
	sender := &RecordingSender{Status: 200}
	d := NewDelivererWithStore(store, sender, "http://localhost", discardLogger())

	if n, _ := d.ProcessOnce(context.Background()); n != 0 {
		t.Errorf("成功投递数 = %d，期望 0（仓库缺失）", n)
	}
	if len(sender.Requests()) != 0 {
		t.Error("仓库缺失时不应发出请求")
	}
	if store.remaining() != 1 {
		t.Error("失败的事件应保留在队列里")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.logs) != 1 {
		t.Fatalf("日志条数 = %d，期望 1", len(store.logs))
	}
	if store.logs[0].Status != 0 {
		t.Errorf("日志状态码 = %d，期望 0（没有发出请求）", store.logs[0].Status)
	}
}

// TestExhaustedAttemptsAreDeadLettered 校验投递次数已经超限的事件被标记死信。
//
// 这种情况出现在"上一次投递进程恰好崩在最后一次投递里"：
// 事件停在投递中，租约到期后被重新领出，但次数已经超限 ——
// 此时必须直接进死信，否则它会永远卡在队列里。
func TestExhaustedAttemptsAreDeadLettered(t *testing.T) {
	ev := newTestEvent("ev0000000000000000000000000006", "http://127.0.0.1:9000/hook")
	store := newMemoryStore(ev)
	store.events[ev.Key].attempts = MaxAttempts // 模拟"崩溃在最后一次投递里"

	sender := &RecordingSender{Status: 200}
	d := NewDelivererWithStore(store, sender, "http://localhost", discardLogger())

	if _, err := d.ProcessOnce(context.Background()); err != nil {
		t.Fatalf("投递出错: %v", err)
	}

	if len(sender.Requests()) != 0 {
		t.Error("次数已超限的事件不应再发出请求")
	}
	status, attempts, dead, ok := store.state(ev.Key)
	if !ok {
		t.Fatal("事件应保留在队列里以便排查")
	}
	if !dead || status != StatusDead {
		t.Errorf("状态 = %d（dead = %v），期望死信 %d", status, dead, StatusDead)
	}
	if attempts != MaxAttempts+1 {
		t.Errorf("尝试次数 = %d，期望 %d", attempts, MaxAttempts+1)
	}
}

// ---------------------------------------------------------------------------
// 并发
// ---------------------------------------------------------------------------

// TestConcurrentClaimNoDuplicateDelivery 校验并发投递不会重复投递同一条事件。
//
// 多个投递器同时领取队列时，FOR UPDATE SKIP LOCKED 让它们互相跳过
// 已被占住的行，因此每条事件只会被投递一次。
func TestConcurrentClaimNoDuplicateDelivery(t *testing.T) {
	const (
		eventCount = 20
		deliverers = 4
	)

	events := make([]Event, 0, eventCount)
	for i := 0; i < eventCount; i++ {
		ev := newTestEvent("ev"+strings.Repeat("0", 30)+string(rune('a'+i)), "http://127.0.0.1:9000/hook")
		ev.RepoName = "repo-" + string(rune('a'+i))
		events = append(events, ev)
	}

	store := newMemoryStore(events...)
	sender := &RecordingSender{Status: 200}
	d := NewDelivererWithStore(store, sender, "http://localhost", discardLogger())

	var wg sync.WaitGroup
	wg.Add(deliverers)
	for i := 0; i < deliverers; i++ {
		go func() {
			defer wg.Done()
			for {
				n, err := d.ProcessOnce(context.Background())
				if err != nil {
					return
				}
				if n == 0 {
					return
				}
			}
		}()
	}
	wg.Wait()

	sent := sender.Requests()
	if len(sent) != eventCount {
		t.Fatalf("发送次数 = %d，期望 %d（每条事件恰好一次）", len(sent), eventCount)
	}
	counts := map[string]int{}
	for _, req := range sent {
		counts[req.Body]++
	}
	for body, count := range counts {
		if count != 1 {
			t.Errorf("同一条事件被投递了 %d 次：%s", count, body)
		}
	}
	if store.remaining() != 0 {
		t.Errorf("队列里还剩 %d 条事件", store.remaining())
	}
}
