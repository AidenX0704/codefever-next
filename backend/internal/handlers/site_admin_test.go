package handlers

import (
	"testing"

	"codefever-next/backend/internal/contract"
)

// 站点内容管理端点的**纯逻辑**用例（不需要数据库）。

// TestIsDateOnly 锁定 releasedAt 的日期白名单。
//
// 列是 DATE：写入非 YYYY-MM-DD 形态会被 MySQL 静默截断成
// 0000-00-00，排序随之失真，因此必须在参数层拦住。
func TestIsDateOnly(t *testing.T) {
	for _, ok := range []string{"2026-10-01", "2026-01-31", "0000-00-00"} {
		if !isDateOnly(ok) {
			t.Errorf("%q 应当是合法日期", ok)
		}
	}
	for _, bad := range []string{
		"", "2026/10/01", "2026-10-1", "2026-1-01", "2026-10-01T00:00:00",
		"2026-13-01", "abcd-ef-gh", "2026-10-011",
	} {
		if isDateOnly(bad) {
			t.Errorf("%q 不应当是合法日期", bad)
		}
	}
}

// TestBodyParamBool 锁定布尔参数的三来源取值。
//
// JSON 是真布尔、multipart 是 "1"/"0" 字符串、query 恒为字符串；
// def 只在参数缺省时生效。
func TestBodyParamBool(t *testing.T) {
	parsed := func(body map[string]any) *contract.Parsed {
		return &contract.Parsed{Body: body, JSON: map[string]any{}, Query: map[string]any{}}
	}

	// 缺省 → def。
	if got := bodyParamBool(parsed(map[string]any{}), "enabled", true); !got {
		t.Error("参数缺省时应返回 def=true")
	}
	// JSON 布尔。
	if got := bodyParamBool(parsed(map[string]any{"enabled": false}), "enabled", true); got {
		t.Error("JSON false 应为 false")
	}
	if got := bodyParamBool(parsed(map[string]any{"enabled": true}), "enabled", false); !got {
		t.Error("JSON true 应为 true")
	}
	// multipart 字符串形态。
	if got := bodyParamBool(parsed(map[string]any{"enabled": "1"}), "enabled", true); !got {
		t.Error(`"1" 应为 true`)
	}
	if got := bodyParamBool(parsed(map[string]any{"enabled": "0"}), "enabled", true); got {
		t.Error(`"0" 应为 false`)
	}
	if got := bodyParamBool(parsed(map[string]any{"enabled": "false"}), "enabled", true); got {
		t.Error(`"false" 应为 false`)
	}
	// 数字形态（JSON number）。
	if got := bodyParamBool(parsed(map[string]any{"enabled": float64(0)}), "enabled", true); got {
		t.Error("JSON 数字 0 应为 false")
	}
	if got := bodyParamBool(parsed(map[string]any{"enabled": float64(1)}), "enabled", false); !got {
		t.Error("JSON 数字 1 应为 true")
	}
}
