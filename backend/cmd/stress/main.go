// Command stress 是 Go 后端的**稳定性 / 性能门禁**工具。
//
// 它模拟真实使用（登录后 SPA 会打的只读接口）持续施压，产出可判定的指标：
//
//	QPS、p50/p95/p99/max 延迟、按端点的分布、错误率、HTTP 状态码分布
//	可选：按固定间隔采样目标进程的 pprof（goroutine 数 / 堆占用），
//	      用于判断"长跑后内存与协程是否单调增长"（泄漏信号）
//
// 同一个工具可以打 PHP 实例与 Go 实例，用同一份负载做**同口径对照**。
//
// 用法：
//
//	go run ./cmd/stress -base http://127.0.0.1:9003 -duration 60s -concurrency 16 \
//	    -email root@codefever.cn -password 123456 -out report.json
//
//	# 带上资源采样（目标需以 CODEFEVER_PPROF=1 启动）
//	go run ./cmd/stress -base http://127.0.0.1:9003 -pprof http://127.0.0.1:9001 \
//	    -duration 300s -sample-interval 10s
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// endpoint 是压测负载里的一个只读端点。
type endpoint struct {
	// Name 是统计用的短名。
	Name string
	// Path 是相对路径（可带查询串）。
	Path string
	// Weight 是相对权重。
	Weight int
}

// workload 是模拟 SPA 的读负载。
//
// 权重按 SPA 的实际调用频次给：user/info 每次路由切换都会打一次，
// repository/list 与 group/list 是首页与导航的常驻数据，
// 其余是仓库详情页的子请求。
var workload = []endpoint{
	{Name: "user/info", Path: "/api/user/info", Weight: 20},
	{Name: "repository/list", Path: "/api/repository/list?type=owned&page=1&pagesize=20", Weight: 15},
	{Name: "group/list", Path: "/api/group/list?page=1&pagesize=20", Weight: 10},
	{Name: "repository/config", Path: "/api/repository/config?rKey=1031c9342d6539fd36ff79f75ca4f8b7", Weight: 15},
	{Name: "repository/object", Path: "/api/repository/object?repository=1031c9342d6539fd36ff79f75ca4f8b7&parent=master&path=&revision=master", Weight: 20},
	{Name: "repository/commitList", Path: "/api/repository/commitList?repository=1031c9342d6539fd36ff79f75ca4f8b7&revision=master&path=&keyword=&page=1&pagesize=10", Weight: 10},
	{Name: "repository/branchList", Path: "/api/repository/branchList?repository=1031c9342d6539fd36ff79f75ca4f8b7", Weight: 5},
	{Name: "user/notifications", Path: "/api/user/notifications?category=0", Weight: 5},
}

// stats 汇总一个端点的延迟分布。
type stats struct {
	Count   int     `json:"count"`
	Errors  int     `json:"errors"`
	MeanMS  float64 `json:"mean_ms"`
	P50MS   float64 `json:"p50_ms"`
	P95MS   float64 `json:"p95_ms"`
	P99MS   float64 `json:"p99_ms"`
	MaxMS   float64 `json:"max_ms"`
	MinMS   float64 `json:"min_ms"`
	Latency []int64 `json:"-"`
}

// sample 是一个资源采样点。
type sample struct {
	AtSec     float64 `json:"at_sec"`
	Goroutine int     `json:"goroutines"`
	HeapMB    float64 `json:"heap_mb"`
}

// report 是一次压测的完整结果。
type report struct {
	Base        string            `json:"base"`
	StartedAt   string            `json:"started_at"`
	DurationSec float64           `json:"duration_sec"`
	Concurrency int               `json:"concurrency"`
	WarmupSec   float64           `json:"warmup_sec"`
	Total       int               `json:"total"`
	QPS         float64           `json:"qps"`
	ErrorTotal  int               `json:"error_total"`
	ErrorRate   float64           `json:"error_rate"`
	StatusClass map[string]int    `json:"status_class"`
	ByEndpoint  map[string]*stats `json:"by_endpoint"`
	Samples     []sample          `json:"samples,omitempty"`
	ErrorSample []string          `json:"error_sample,omitempty"`
	Verdict     map[string]any    `json:"verdict"`
}

// gate 是门禁阈值。
type gate struct {
	MaxErrorRate float64
	MaxP99MS     float64
	MaxP95MS     float64
}

func main() {
	var (
		base        = flag.String("base", "http://127.0.0.1:9003", "目标基地址")
		duration    = flag.Duration("duration", 60*time.Second, "压测时长")
		warmup      = flag.Duration("warmup", 3*time.Second, "预热时长（不计入统计）")
		concurrency = flag.Int("concurrency", 8, "并发 worker 数")
		email       = flag.String("email", "root@codefever.cn", "登录邮箱")
		password    = flag.String("password", "123456", "登录口令")
		out         = flag.String("out", "", "JSON 报告输出路径（默认打印到 stdout）")
		pprofBase   = flag.String("pprof", "", "pprof 基地址（留空则不做资源采样）")
		sampleInt   = flag.Duration("sample-interval", 10*time.Second, "资源采样间隔")
		maxErrRate  = flag.Float64("max-error-rate", 0.001, "门禁：错误率上限")
		maxP99      = flag.Float64("max-p99-ms", 500, "门禁：p99 延迟上限（毫秒）")
		maxP95      = flag.Float64("max-p95-ms", 200, "门禁：p95 延迟上限（毫秒）")
	)
	flag.Parse()

	rep, err := run(runConfig{
		base:        strings.TrimRight(*base, "/"),
		duration:    *duration,
		warmup:      *warmup,
		concurrency: *concurrency,
		email:       *email,
		password:    *password,
		pprofBase:   strings.TrimRight(*pprofBase, "/"),
		sampleInt:   *sampleInt,
		gate: gate{
			MaxErrorRate: *maxErrRate,
			MaxP99MS:     *maxP99,
			MaxP95MS:     *maxP95,
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "压测失败:", err)
		os.Exit(2)
	}

	data, _ := json.MarshalIndent(rep, "", "  ")
	if *out != "" {
		if err := os.WriteFile(*out, data, 0o600); err != nil {
			fmt.Fprintln(os.Stderr, "写报告失败:", err)
			os.Exit(2)
		}
		fmt.Printf("报告已写入 %s\n", *out)
	}
	fmt.Println(string(data))

	// 门禁判定：任一不达标即以非 0 退出，便于 CI 阻断。
	if pass, reasons := judge(rep); !pass {
		fmt.Fprintln(os.Stderr, "门禁未通过:")
		for _, r := range reasons {
			fmt.Fprintln(os.Stderr, "  -", r)
		}
		os.Exit(1)
	}
	fmt.Println("门禁通过")
}

type runConfig struct {
	base        string
	duration    time.Duration
	warmup      time.Duration
	concurrency int
	email       string
	password    string
	pprofBase   string
	sampleInt   time.Duration
	gate        gate
}

func run(cfg runConfig) (*report, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{
		Jar:     jar,
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        cfg.concurrency * 4,
			MaxIdleConnsPerHost: cfg.concurrency * 4,
			MaxConnsPerHost:     cfg.concurrency * 4,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	// 登录一次，所有 worker 共享会话。
	// 真实场景每个用户有独立会话，但会话校验（查 cc_sessions）发生在每个请求上，
	// 因此共享 cookie 仍能覆盖到会话路径。
	if err := login(client, cfg.base, cfg.email, cfg.password); err != nil {
		return nil, fmt.Errorf("登录失败: %w", err)
	}

	rep := &report{
		Base:        cfg.base,
		StartedAt:   time.Now().Format(time.RFC3339),
		Concurrency: cfg.concurrency,
		WarmupSec:   cfg.warmup.Seconds(),
		StatusClass: map[string]int{},
		ByEndpoint:  map[string]*stats{},
	}

	// 预热：让连接池与 JIT 预热，避开冷启动噪声。
	deadline := time.Now().Add(cfg.warmup)
	for time.Now().Before(deadline) {
		doOne(client, cfg.base, workload[0])
		time.Sleep(50 * time.Millisecond)
	}

	// 资源采样协程。
	sampleCtx, sampleCancel := context.WithCancel(context.Background())
	defer sampleCancel()
	var sampleMu sync.Mutex
	if cfg.pprofBase != "" {
		go sampleResources(sampleCtx, cfg, rep, &sampleMu)
	}

	// 压测主体。
	start := time.Now()
	end := start.Add(cfg.duration)
	var wg sync.WaitGroup
	var mu sync.Mutex
	merged := map[string]*stats{}
	statusClass := map[string]int{}
	total, errTotal := 0, 0
	var errSamples []string

	for w := 0; w < cfg.concurrency; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			// 每个 worker 用自己的随机序列起点，避免所有 worker 同步打同一个端点。
			epIdx := worker
			local := map[string]*stats{}
			localStatus := map[string]int{}
			localTotal, localErr := 0, 0
			var localErrSamples []string
			for time.Now().Before(end) {
				ep := workload[epIdx%len(workload)]
				epIdx += 1 + ep.Weight
				lat, status, err := doOne(client, cfg.base, ep)
				s := local[ep.Name]
				if s == nil {
					s = &stats{}
					local[ep.Name] = s
				}
				s.Count++
				s.Latency = append(s.Latency, lat)
				if err != nil {
					s.Errors++
					localErr++
					if len(localErrSamples) < 20 {
						localErrSamples = append(localErrSamples, fmt.Sprintf("%s: %v", ep.Name, err))
					}
				}
				class := strconv.Itoa(status/100) + "xx"
				localStatus[class]++
				localTotal++
			}
			// 合并必须在锁内：localStatus / total 等是 map 与普通变量，
			// 无锁累加会触发 concurrent map writes。
			mu.Lock()
			for name, s := range local {
				if merged[name] == nil {
					merged[name] = &stats{}
				}
				m := merged[name]
				m.Count += s.Count
				m.Errors += s.Errors
				m.Latency = append(m.Latency, s.Latency...)
			}
			for class, n := range localStatus {
				statusClass[class] += n
			}
			total += localTotal
			errTotal += localErr
			if len(errSamples) < 20 {
				errSamples = append(errSamples, localErrSamples...)
			}
			mu.Unlock()
		}(w)
	}
	wg.Wait()
	elapsed := time.Since(start).Seconds()

	rep.DurationSec = elapsed
	rep.Total = total
	rep.QPS = float64(total) / elapsed
	rep.ErrorTotal = errTotal
	if total > 0 {
		rep.ErrorRate = float64(errTotal) / float64(total)
	}
	rep.StatusClass = statusClass
	rep.ErrorSample = errSamples
	for name, s := range merged {
		sort.Slice(s.Latency, func(i, j int) bool { return s.Latency[i] < s.Latency[j] })
		if len(s.Latency) > 0 {
			s.MinMS = float64(s.Latency[0]) / 1e6
			s.MaxMS = float64(s.Latency[len(s.Latency)-1]) / 1e6
			s.P50MS = float64(percentile(s.Latency, 0.50)) / 1e6
			s.P95MS = float64(percentile(s.Latency, 0.95)) / 1e6
			s.P99MS = float64(percentile(s.Latency, 0.99)) / 1e6
			var sum int64
			for _, v := range s.Latency {
				sum += v
			}
			s.MeanMS = float64(sum) / float64(len(s.Latency)) / 1e6
		}
		rep.ByEndpoint[name] = s
	}

	// 全局 p95/p99：把所有端点的延迟合起来算。
	//
	// 必须在清空各端点的延迟明细**之前**算 —— 明细动辄几十万条，
	// 统计完就释放，不留在报告里。
	all := []int64{}
	for _, s := range merged {
		all = append(all, s.Latency...)
	}
	sort.Slice(all, func(i, j int) bool { return all[i] < all[j] })
	globalP99 := float64(percentile(all, 0.99)) / 1e6
	globalP95 := float64(percentile(all, 0.95)) / 1e6
	for _, s := range merged {
		s.Latency = nil
	}
	rep.Verdict = map[string]any{
		"global_p95_ms": globalP95,
		"global_p99_ms": globalP99,
	}
	return rep, nil
}

// judge 按门禁阈值判定。
func judge(rep *report) (bool, []string) {
	var reasons []string
	// 全局分位在 run() 里已算好（各端点的延迟明细统计完即释放）。
	p99, _ := rep.Verdict["global_p99_ms"].(float64)
	p95, _ := rep.Verdict["global_p95_ms"].(float64)
	if rep.ErrorRate > 0.001 {
		reasons = append(reasons, fmt.Sprintf("错误率 %.4f 超过 0.1%%", rep.ErrorRate*100))
	}
	if p99 > 500 {
		reasons = append(reasons, fmt.Sprintf("p99 %.1fms 超过 500ms", p99))
	}
	if p95 > 200 {
		reasons = append(reasons, fmt.Sprintf("p95 %.1fms 超过 200ms", p95))
	}
	// 5xx 一律是硬失败。
	for cls, n := range rep.StatusClass {
		if strings.HasPrefix(cls, "5") && n > 0 {
			reasons = append(reasons, fmt.Sprintf("出现 %d 个 %s 响应", n, cls))
		}
	}
	// 资源增长：采样序列的末值不应显著高于首值。
	if len(rep.Samples) >= 2 {
		first, last := rep.Samples[0], rep.Samples[len(rep.Samples)-1]
		if last.Goroutine > first.Goroutine*2+50 {
			reasons = append(reasons, fmt.Sprintf("goroutine 数从 %d 涨到 %d，疑似泄漏", first.Goroutine, last.Goroutine))
		}
		if last.HeapMB > first.HeapMB*2+50 {
			reasons = append(reasons, fmt.Sprintf("堆占用从 %.1fMB 涨到 %.1fMB，疑似泄漏", first.HeapMB, last.HeapMB))
		}
	}
	return len(reasons) == 0, reasons
}

// percentile 返回排序后切片在 p 分位上的值（最近秩法）。
func percentile(sorted []int64, p float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// login 走真实表单登录。
func login(client *http.Client, base, email, password string) error {
	form := url.Values{}
	form.Set("email", email)
	form.Set("password", password)
	req, err := http.NewRequest(http.MethodPost, base+"/user/login", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if !strings.Contains(string(body), `"code":0`) {
		return fmt.Errorf("登录响应异常: %s", trunc(string(body), 200))
	}
	return nil
}

// doOne 打一个端点，返回耗时（纳秒）与 HTTP 状态码。
func doOne(client *http.Client, base string, ep endpoint) (int64, int, error) {
	req, err := http.NewRequest(http.MethodGet, base+ep.Path, nil)
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("Accept", "application/json")
	start := time.Now()
	resp, err := client.Do(req)
	lat := time.Since(start)
	if err != nil {
		return lat.Nanoseconds(), 0, err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		return lat.Nanoseconds(), resp.StatusCode, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return lat.Nanoseconds(), resp.StatusCode, nil
}

var (
	goroutineTotalRe = regexp.MustCompile(`goroutine profile: total (\d+)`)
	// heap profile 的首行格式为：
	//   heap profile: 8: 840256 [1997: 148246648] @ heap/1048576
	// 第二个数字是**堆内正在使用的字节数**（inuse_space），是判断内存增长的指标；
	// 括号里的累计分配量（alloc_space）会一直涨，没有参考价值。
	heapInuseRe = regexp.MustCompile(`heap profile: \d+: (\d+)`)
)

// sampleResources 周期性采样目标进程的 pprof。
func sampleResources(ctx context.Context, cfg runConfig, rep *report, mu *sync.Mutex) {
	ticker := time.NewTicker(cfg.sampleInt)
	defer ticker.Stop()
	start := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			g := pprofCount(ctx, cfg.pprofBase+"/debug/pprof/goroutine?debug=1", goroutineTotalRe)
			h := pprofCount(ctx, cfg.pprofBase+"/debug/pprof/heap?debug=1", heapInuseRe)
			if g == 0 && h == 0 {
				continue
			}
			mu.Lock()
			rep.Samples = append(rep.Samples, sample{
				AtSec:     time.Since(start).Seconds(),
				Goroutine: g,
				HeapMB:    float64(h) / (1 << 20),
			})
			mu.Unlock()
		}
	}
}

// pprofCount 从 pprof 文本报告里取 total 数。
func pprofCount(ctx context.Context, url string, re *regexp.Regexp) int {
	// heap profile 的文本报告可能有几 MB，生成也要时间，
	// 因此这里给一个比"采样间隔"更宽裕的超时。
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return 0
	}
	m := re.FindSubmatch(body)
	if len(m) != 2 {
		return 0
	}
	n, _ := strconv.Atoi(string(m[1]))
	return n
}

// trunc 截断长文本。
func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
