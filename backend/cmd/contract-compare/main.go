// Command contract-compare 比对两个契约采集结果目录，输出差异报告。
//
// 用法：
//
//	go run ./cmd/contract-compare -baseline testdata/contract -actual testdata/contract-go
//
// baseline 是基准（通常是对现有 PHP 后端采集的结果），
// actual 是待验证实现（通常是 Go 后端）的采集结果。
//
// 分类：
//
//	MATCH             状态码、业务码、响应体完全一致
//	DIFF              双方都有响应但内容不一致 —— 需要逐条排查
//	NOT_IMPLEMENTED   实际实现里该端点尚未提供（返回 0x0105 / 404）
//	BASELINE_ONLY     基准里有、实际实现里没有采集记录
//	ACTUAL_ONLY       实际实现里多出���采集记录
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

// fixture 与 contract-collect 产出的 JSON 结构保持一致。
//
// 只取比对所需的字段，其余字段忽略。
type fixture struct {
	ID          string          `json:"id"`
	Method      string          `json:"method"`
	Path        string          `json:"path"`
	Status      int             `json:"status"`
	ContentType string          `json:"content_type"`
	ActualCode  *int            `json:"actual_code"`
	Body        json.RawMessage `json:"body"`
}

// summary 是整体比对结果。
type summary struct {
	Total          int
	Match          int
	Diff           int
	NotImplemented int
	BaselineOnly   int
	ActualOnly     int
}

// diffItem 记录一条差异。
type diffItem struct {
	ID       string
	Reason   string
	Detail   string
	Baseline *fixture
	Actual   *fixture
}

func main() {
	baselineDir := flag.String("baseline", "testdata/contract", "基准采集结果目录")
	actualDir := flag.String("actual", "testdata/contract-verify", "实际采集结果目录（先用 contract-collect 生成）")
	verbose := flag.Bool("verbose", false, "打印每条用例的比对细节")
	failOnDiff := flag.Bool("fail-on-diff", false, "存在 DIFF 时以非 0 退出码结束")
	flag.Parse()

	baseline, err := loadFixtures(*baselineDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "contract-compare:", err)
		os.Exit(2)
	}
	actual, err := loadFixtures(*actualDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "contract-compare:", err)
		os.Exit(2)
	}

	sum := summary{}
	var diffs []diffItem

	for _, id := range sortedKeys(baseline) {
		b := baseline[id]
		sum.Total++

		a, ok := actual[id]
		if !ok {
			sum.BaselineOnly++
			diffs = append(diffs, diffItem{ID: id, Reason: "BASELINE_ONLY", Detail: "实际实现中缺少该用例的采集记录", Baseline: &b})
			continue
		}

		switch {
		case isNotImplemented(a):
			sum.NotImplemented++
			if *verbose {
				diffs = append(diffs, diffItem{ID: id, Reason: "NOT_IMPLEMENTED", Detail: "实际实现尚未提供该端点", Baseline: &b, Actual: &a})
			}
		case a.Status == b.Status && intPtrEq(a.ActualCode, b.ActualCode) && jsonEqual(a.Body, b.Body):
			sum.Match++
			if *verbose {
				diffs = append(diffs, diffItem{ID: id, Reason: "MATCH", Detail: ""})
			}
		default:
			sum.Diff++
			diffs = append(diffs, diffItem{ID: id, Reason: "DIFF", Detail: describe(b, a), Baseline: &b, Actual: &a})
		}
	}

	for _, id := range sortedKeys(actual) {
		if _, ok := baseline[id]; !ok {
			sum.ActualOnly++
			diffs = append(diffs, diffItem{ID: id, Reason: "ACTUAL_ONLY", Detail: "基准中没有该用例"})
		}
	}

	printReport(sum, diffs, *verbose)

	if sum.Diff > 0 && *failOnDiff {
		os.Exit(1)
	}
}

// printReport 输出比对报告。
func printReport(sum summary, diffs []diffItem, verbose bool) {
	fmt.Println("=== 契约比对报告 ===")
	fmt.Printf("总计 %d 条：MATCH %d | DIFF %d | NOT_IMPLEMENTED %d | BASELINE_ONLY %d | ACTUAL_ONLY %d\n",
		sum.Total, sum.Match, sum.Diff, sum.NotImplemented, sum.BaselineOnly, sum.ActualOnly)

	var show []diffItem
	for _, d := range diffs {
		if d.Reason == "MATCH" && !verbose {
			continue
		}
		show = append(show, d)
	}
	if len(show) == 0 {
		fmt.Println("全部一致。")
		return
	}
	fmt.Println()
	for _, d := range show {
		fmt.Printf("[%s] %s\n", d.Reason, d.ID)
		if d.Detail != "" {
			fmt.Printf("    %s\n", d.Detail)
		}
	}
}

// describe 生成差异描述。
func describe(b fixture, a fixture) string {
	var sb strings.Builder
	if b.Status != a.Status {
		fmt.Fprintf(&sb, "HTTP 状态: 基准 %d → 实际 %d; ", b.Status, a.Status)
	}
	if !intPtrEq(b.ActualCode, a.ActualCode) {
		fmt.Fprintf(&sb, "业务码: 基准 %s → 实际 %s; ", intPtrStr(b.ActualCode), intPtrStr(a.ActualCode))
	}
	if b.ContentType != a.ContentType {
		fmt.Fprintf(&sb, "Content-Type: 基准 %q → 实际 %q; ", b.ContentType, a.ContentType)
	}
	if !jsonEqual(b.Body, a.Body) {
		fmt.Fprintf(&sb, "响应体不同")
	}
	return sb.String()
}

// isNotImplemented 判断实际实现是否表示"端点尚未提供"。
//
// 契约层对"已知 controller + 未知动作"返回 0x0105（404）。
func isNotImplemented(f fixture) bool {
	return f.ActualCode != nil && *f.ActualCode == 0x0105 && f.Status == 404
}

// loadFixtures 读取一个目录下的全部采集结果。
func loadFixtures(dir string) (map[string]fixture, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("读取目录 %s 失败: %w", dir, err)
	}
	out := map[string]fixture{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("读取 %s 失败: %w", e.Name(), err)
		}
		var f fixture
		if err := json.Unmarshal(raw, &f); err != nil {
			return nil, fmt.Errorf("解析 %s 失败: %w", e.Name(), err)
		}
		out[f.ID] = f
	}
	return out, nil
}

// jsonEqual 比较两个 JSON 值是否语义相等。
func jsonEqual(a, b json.RawMessage) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	var va, vb any
	if err := json.Unmarshal(a, &va); err != nil {
		return string(a) == string(b)
	}
	if err := json.Unmarshal(b, &vb); err != nil {
		return string(a) == string(b)
	}
	return reflect.DeepEqual(va, vb)
}

// intPtrEq 比较两个可空整数。
func intPtrEq(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// intPtrStr 把可空整数转成可读字符串。
func intPtrStr(v *int) string {
	if v == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%d", *v)
}

// sortedKeys 返回有序的键列表。
func sortedKeys(m map[string]fixture) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
