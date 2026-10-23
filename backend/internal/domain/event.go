package domain

import (
	"fmt"
	"strconv"
	"strings"
)

// 事件校验的错误语义，逐字对应 application/libraries/Exception/Exception.php 的内部码：
//
//	2001 未知事件类型
//	2002 未知事件数据描述
//	2003 事件数据与描述不符
//	2004 事件数据描述格式非法
type EventError struct {
	Code int
	Msg  string
}

// Error 实现 error 接口。
func (e *EventError) Error() string { return fmt.Sprintf("事件校验失败(%d): %s", e.Code, e.Msg) }

// 事件数据的基础类型标记。
const (
	eventTypeInt    = "int"
	eventTypeFloat  = "float"
	eventTypeBool   = "bool"
	eventTypeString = "string"
	eventTypeArray  = "array"
	eventTypeAny    = "*"
)

// ValidateEventData 按事件数据描述校验事件载荷。
//
// 逐字复刻 application/libraries/Service.php:74-115 的 _validateEvent()：
//   - 描述项为空 → 跳过；
//   - 描述项不是 "name:type" 两段 → 2004；
//   - 字段存在 → 类型前缀匹配则通过，否则 2003；
//   - 字段缺失 → 类型标记里带 "?" 则通过，否则 2003。
//
// 描述中的类型是**前缀匹配**（PHP 用 strpos(...) === 0），所以 "int" 也能匹配 "int?"。
func ValidateEventData(desc EventDataDesc, data map[string]any) error {
	for _, item := range strings.Split(string(desc), "|") {
		if item == "" {
			// 空项：直接通过。
			continue
		}
		parts := strings.SplitN(item, ":", 2)
		if len(parts) != 2 {
			return &EventError{Code: 2004, Msg: fmt.Sprintf("事件数据描述项 %q 格式非法", item)}
		}
		name, typ := parts[0], parts[1]

		value, present := data[name]
		if present {
			if matchEventType(typ, value) {
				continue
			}
			return &EventError{Code: 2003, Msg: fmt.Sprintf("字段 %q 的类型与描述 %q 不符", name, typ)}
		}
		// 字段缺失：类型标记里带 "?" 才允许。
		if strings.Contains(typ, "?") {
			continue
		}
		return &EventError{Code: 2003, Msg: fmt.Sprintf("缺少必填字段 %q（描述 %q）", name, typ)}
	}
	return nil
}

// matchEventType 判断值的类型是否符合描述标记（前缀匹配）。
func matchEventType(typ string, value any) bool {
	switch {
	case strings.HasPrefix(typ, eventTypeAny):
		// 通配类型，接受任何值。
		return true
	case strings.HasPrefix(typ, eventTypeInt):
		switch value.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			return true
		}
		return false
	case strings.HasPrefix(typ, eventTypeFloat):
		switch value.(type) {
		case float32, float64:
			return true
		}
		return false
	case strings.HasPrefix(typ, eventTypeBool):
		_, ok := value.(bool)
		return ok
	case strings.HasPrefix(typ, eventTypeString):
		_, ok := value.(string)
		return ok
	case strings.HasPrefix(typ, eventTypeArray):
		switch value.(type) {
		case []any, []string, []int, []map[string]any:
			return true
		}
		return false
	default:
		return false
	}
}

// EventPayload 是入队的事件内容。
//
// 与 PHP 侧 Event::getContent() 的输出结构一致：
// id / type / data / generatedBy / timestamp / language。
type EventPayload struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	Data        map[string]any `json:"data"`
	GeneratedBy string         `json:"generatedBy"`
	Timestamp   int64          `json:"timestamp"`
	Language    string         `json:"language"`
}

// LanguageCN 与 LanguageEN 是事件语言标记。
const (
	LanguageCN = "zh-cn"
	LanguageEN = "en-us"
)

// NormalizeEventData 把事件载荷里的数值统一成 JSON 可序列化的形态。
//
// PHP 侧的事件数据来自控制器，多为 string / int；这里补齐 float64 → int 的收敛，
// 避免 JSON 编码时出现 "1" 与 1 混用。
func NormalizeEventData(data map[string]any) map[string]any {
	out := make(map[string]any, len(data))
	for k, v := range data {
		switch t := v.(type) {
		case int:
			out[k] = int64(t)
		case float64:
			if t == float64(int64(t)) {
				out[k] = int64(t)
			} else {
				out[k] = t
			}
		default:
			out[k] = v
		}
	}
	return out
}

// FormatEventDataDebug 把事件载荷格式化成便于日志阅读的字符串。
func FormatEventDataDebug(data map[string]any) string {
	parts := make([]string, 0, len(data))
	for k, v := range data {
		parts = append(parts, k+"="+strconv.Quote(fmt.Sprint(v)))
	}
	return strings.Join(parts, " ")
}
