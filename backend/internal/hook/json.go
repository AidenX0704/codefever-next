// 本文件实现 **PHP 风格** 的 JSON 编码，用于钩子的请求体。
//
// 为什么不能直接用 encoding/json：
//
// PHP 的 json_encode 默认（未开启 JSON_UNESCAPED_UNICODE / JSON_UNESCAPED_SLASHES）
// 与 Go 标准库有两个差异：
//
//  1. 非 ASCII 字符被转成 \uXXXX；
//     PHP: {"stdin":"\u4e2d\u6587"}
//     Go : {"stdin":"中文"}
//
//  2. 斜杠 "/" 被转成 "\/"。
//
// 钩子的请求体是**跨进程契约**（后端 /apiv1/internal/hooks 要解析它），
// 迁移前后后端收到的字节应保持一致，因此这里按 PHP 的规则手工拼装。
//
// 与 internal/httpapi/json.go 的 phpJSONString 是**同一套规则的两份实现**：
// 那个文件服务于 web 控制器命名空间（未导出），钩子包独立，
// 避免为了一个字符串编码把整个 httpapi（服务端、鉴权、DB）拖进钩子二进制。
//
// 另外，encoding/json.Marshal 会对自定义 MarshalJSON 的输出再做一次
// HTML 转义（< > & → &lt; &gt; &amp;），
// 与 PHP 不同 —— 这也是这里不用 json.Marshal 的原因。
package hook

import (
	"encoding/json"
	"fmt"
	"strings"
)

// EncodePayload 按 PHP 的 json_encode 规则编码钩子请求体。
//
// 字段顺序与 PHP 的 $data 插入顺序一致：uid → args → stdin。
func EncodePayload(p Payload) []byte {
	var b strings.Builder
	b.WriteString(`{"uid":`)
	b.WriteString(encodeJSONValue(p.UID))
	b.WriteString(`,"args":`)
	b.WriteString(encodeJSONArray(p.Args))
	b.WriteString(`,"stdin":`)
	b.WriteString(encodeJSONString(p.Stdin))
	b.WriteString(`}`)
	return []byte(b.String())
}

// encodeJSONValue 编码任意值。
func encodeJSONValue(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case bool:
		if t {
			return "true"
		}
		return "false"
	case string:
		return encodeJSONString(t)
	case []string:
		return encodeJSONArray(t)
	case int:
		return fmt.Sprintf("%d", t)
	default:
		// 兜底：标准库编码（仅用于本包不直接构造的类型）。
		raw, err := json.Marshal(v)
		if err != nil {
			return "null"
		}
		return string(raw)
	}
}

// encodeJSONArray 编码字符串数组（PHP 的 json_encode 对连续下标数组输出 JSON 数组）。
func encodeJSONArray(items []string) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, encodeJSONString(item))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// encodeJSONString 按 PHP 的 json_encode 规则转义字符串（含首尾引号）。
//
// PHP 对非 BMP 字符（> U+FFFF）用 UTF-16 代理对表示，与 Go 的 rune 语义不同，
// 这里显式转换。
func encodeJSONString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '/':
			b.WriteString(`\/`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			switch {
			case r < 0x20:
				fmt.Fprintf(&b, `\u%04x`, r)
			case r < 0x80:
				b.WriteRune(r)
			case r > 0xFFFF:
				// 转成 UTF-16 代理对。
				v := r - 0x10000
				fmt.Fprintf(&b, `\u%04x\u%04x`, 0xD800+(v>>10), 0xDC00+(v&0x3FF))
			default:
				fmt.Fprintf(&b, `\u%04x`, r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}
