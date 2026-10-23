package events

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"codefever-next/backend/internal/config"
)

// BuildHandlers 构造默认的事件处理器集合。
//
// 对应 PHP 侧 application/config/event_dispatchers.php 里 GeneralEventDispatcher
// 的四个 handler（TestHandler / ActivityHandler / UserNotificationHandler / WebhookHandler）：
//
//	ActivityHandler        → 写 cc_activities（活动流）
//	UserNotificationHandler → 写 cc_notification_content / cc_notification_users + 发邮件
//	WebhookHandler         → 写 cc_repository_webhook_events（由投递器异步发出）
//
// PHP 侧还有一个 TestHandler，它只是把事件写进日志用于调试；
// 新实现用结构化日志替代，不再单独建 handler。
func BuildHandlers(db *sql.DB, cfg *config.Config, logger *slog.Logger) []Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return []Handler{
		NewActivityHandler(db, logger),
		NewNotificationHandler(db, cfg, logger),
		NewWebhookHandler(db, logger),
	}
}

// strPtr 返回指向 s 的指针。
//
// 用于区分空字符串（PHP 侧写入的是空串）与 NULL（PHP 侧 insert 时不带该字段）——
// cc_activities 的 a_relative_g_key / a_relative_r_key / a_relative_mr_key
// 都允许 NULL，两者在语义上并不等价。
func strPtr(s string) *string { return &s }

// marshalJSONValue 编码一个 JSON 值。
//
// 与 PHP 的 json_encode() 对齐：不做 HTML 转义（PHP 不转义 < > &），
// 也不额外转义正斜杠。
func marshalJSONValue(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// phpJSONEncode 按 PHP 的 json_encode() 语义编码一个值。
//
// PHP 的 json_encode() 与 Go 的 encoding/json 有三处差异，这里逐一对齐，
// 让落库内容与 PHP 侧**逐字节一致**：
//
//  1. PHP 把正斜杠转义成 \/（Go 不转义）；
//  2. PHP 把非 ASCII 字符转义成 \uXXXX（Go 输出原始 UTF-8）；
//  3. PHP 的 JSON_HEX_TAG 会把 < > & 转义成 \u003C \u003E \u0026
//     （活动流 a_data 不带该标志，站内信 nc_data 带）。
//
// 这一点是**实测**确认的：库里的 PHP 历史数据是
// {"name":"Root Team\/demo repo"} —— 正斜杠被转义了。
func phpJSONEncode(v any, hexTag bool) (string, error) {
	raw, err := marshalJSONValue(v)
	if err != nil {
		return "", err
	}
	return phpJSONEscape(raw, hexTag), nil
}

// phpJSONEscape 把 Go 的 JSON 输出转换成 PHP json_encode() 的转义形态。
//
// 转义规则（已用容器内的 PHP 7.4 实测确认）：
//
//	正斜杠       → \/          （大小写无关，始终转义）
//	非 ASCII     → \uXXXX      （小写十六进制）
//	< 与 >       → \u003C / \u003E（**大写**十六进制，仅 JSON_HEX_TAG 时）
//
// 注意：PHP 的 JSON_HEX_TAG **不转义 &**（那是单独的 JSON_HEX_AMP 标志），
// 而 json_encode($data, TRUE) 的第二个参数正是 JSON_HEX_TAG。
func phpJSONEscape(raw []byte, hexTag bool) string {
	var sb strings.Builder
	sb.Grow(len(raw) + 16)
	for i := 0; i < len(raw); i++ {
		b := raw[i]
		switch {
		case b == '/':
			sb.WriteString(`\/`)
		case hexTag && b == '<':
			writeUnicodeEscapeUpper(&sb, 0x3C)
		case hexTag && b == '>':
			writeUnicodeEscapeUpper(&sb, 0x3E)
		case b < 0x80:
			sb.WriteByte(b)
		default:
			// 非 ASCII：按 UTF-8 解码后转成 \uXXXX（必要时拆成代理对）。
			r, size := utf8.DecodeRune(raw[i:])
			if r == utf8.RuneError && size <= 1 {
				sb.WriteString(`\ufffd`)
				continue
			}
			if r > 0xFFFF {
				high, low := utf16.EncodeRune(r)
				writeUnicodeEscape(&sb, uint16(high))
				writeUnicodeEscape(&sb, uint16(low))
			} else {
				writeUnicodeEscape(&sb, uint16(r))
			}
			i += size - 1
		}
	}
	return sb.String()
}

// writeUnicodeEscape 写出 \uXXXX 形式的转义（小写十六进制）。
func writeUnicodeEscape(sb *strings.Builder, r uint16) {
	const hexDigits = "0123456789abcdef"
	sb.WriteString(`\u`)
	sb.WriteByte(hexDigits[(r>>12)&0xF])
	sb.WriteByte(hexDigits[(r>>8)&0xF])
	sb.WriteByte(hexDigits[(r>>4)&0xF])
	sb.WriteByte(hexDigits[r&0xF])
}

// writeUnicodeEscapeUpper 写出大写十六进制的 \uXXXX 转义。
//
// PHP 的 JSON_HEX_TAG 用的是**大写**十六进制（\u003C / \u003E），
// 而通用的非 ASCII 转义用小写 —— 两者不一致，这里如实复刻。
func writeUnicodeEscapeUpper(sb *strings.Builder, r uint16) {
	const hexDigits = "0123456789ABCDEF"
	sb.WriteString(`\u`)
	sb.WriteByte(hexDigits[(r>>12)&0xF])
	sb.WriteByte(hexDigits[(r>>8)&0xF])
	sb.WriteByte(hexDigits[(r>>4)&0xF])
	sb.WriteByte(hexDigits[r&0xF])
}

// jsonObject 是保持插入顺序的 JSON 对象。
//
// PHP 的 json_encode() 保留数组的插入顺序，而 Go 的 map 序列化会按键名排序。
// 活动流内容（a_data）与站内信内容（nc_data）都直接交给前端 JSON.parse，
// 字段名一致即可正确渲染，但保持顺序能让两套实现的落库内容逐字节可比。
type jsonObject struct {
	keys   []string
	values []any
}

// set 追加一个键值对（同名键会被覆盖，位置保持不变）。
//
// 返回自身，便于链式构造。
func (o *jsonObject) set(key string, value any) *jsonObject {
	for i, k := range o.keys {
		if k == key {
			o.values[i] = value
			return o
		}
	}
	o.keys = append(o.keys, key)
	o.values = append(o.values, value)
	return o
}

// len 返回键值对数量。
func (o *jsonObject) len() int { return len(o.keys) }

// marshal 编码成 JSON 文本（PHP 的 json_encode() 语义）。
//
// 空对象编码为 "[]" —— PHP 侧的 json_encode([]) 就是 "[]" 而不是 "{}"
// （活动流里"更新描述"这类事件用的是空数组）。
func (o *jsonObject) marshal() (string, error) {
	return o.marshalWith(false)
}

// marshalHexTag 编码成 JSON 文本（PHP 的 json_encode($data, TRUE) 语义）。
//
// 第二个参数 TRUE 在 PHP 里等价于 JSON_HEX_TAG 标志：< > & 会被转义成
// \u003C \u003E \u0026。站内信的 nc_data 就是这样落库的。
func (o *jsonObject) marshalHexTag() (string, error) {
	return o.marshalWith(true)
}

// marshalWith 按给定的标志位编码。
func (o *jsonObject) marshalWith(hexTag bool) (string, error) {
	if len(o.keys) == 0 {
		return "[]", nil
	}
	var sb strings.Builder
	sb.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			sb.WriteByte(',')
		}
		kb, err := marshalJSONValue(k)
		if err != nil {
			return "", err
		}
		vb, err := marshalJSONValue(o.values[i])
		if err != nil {
			return "", err
		}
		sb.Write(kb)
		sb.WriteByte(':')
		sb.Write(vb)
	}
	sb.WriteByte('}')
	return phpJSONEscape([]byte(sb.String()), hexTag), nil
}

// eventDataString 读取事件数据里的字符串字段。
//
// 事件数据在 Emit 时已按数据描述校验过类型，因此这里只需要处理
// "字段缺失"（描述里带 ? 的可选字段）与 JSON 数字的字符串化。
func eventDataString(data map[string]any, key string) string {
	raw, ok := data[key]
	if !ok {
		return ""
	}
	return scalarString(raw)
}

// scalarString 把任意标量转成字符串（与 PHP 的 sprintf('%s', ...) 对齐）。
//
// 关键点：PHP 的 true 会渲染成 "1"，false 与 null 渲染成空字符串。
func scalarString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		if t {
			return "1"
		}
		return ""
	case int:
		return itoa(t)
	case int64:
		return itoa64(t)
	case float64:
		return trimFloat(t)
	case uint:
		return itoa64(int64(t))
	case uint64:
		return itoa64(int64(t))
	default:
		return ""
	}
}

// zeroDateTime 是 PHP 侧未显式赋值时 MySQL 非严格模式下的默认值。
//
// cc_notification_content.nc_updated / nc_deleted 与
// cc_notification_users.nu_updated / nu_deleted 都是 NOT NULL 且无默认值，
// PHP 连接是非严格模式（application/config/database.php 的 stricton = FALSE），
// 缺省值会落成零日期；Go 连接是严格模式，必须显式写入。
const zeroDateTime = "0000-00-00 00:00:00"

// nowString 返回当前时间（PHP 的 date('Y-m-d H:i:s') 形态）。
func nowString() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// itoa 把 int 转成十进制字符串。
func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// itoa64 把 int64 转成十进制字符串。
func itoa64(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// trimFloat 把浮点数转成整数值的字符串（PHP 的 json_decode 后可能是字符串）。
func trimFloat(v float64) string {
	if v == float64(int64(v)) {
		return itoa64(int64(v))
	}
	return strings.TrimRight(strings.TrimRight(formatFloat(v), "0"), ".")
}

// formatFloat 输出浮点数（保留最多 6 位小数，与 PHP 的默认精度接近）。
func formatFloat(v float64) string {
	s := itoa64(int64(v))
	frac := v - float64(int64(v))
	if frac == 0 {
		return s
	}
	digits := ""
	for i := 0; i < 6; i++ {
		frac *= 10
		d := int(frac)
		digits += string(rune('0' + d))
		frac -= float64(d)
		if frac == 0 {
			break
		}
	}
	return s + "." + digits
}

// handlerContextIsDone 判断 ctx 是否已经结束（用于长循环里尊重取消）。
func handlerContextIsDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}
