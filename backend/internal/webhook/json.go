package webhook

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// jsonObject 是**保序**的 JSON 对象。
//
// PHP 的 json_decode(..., TRUE) 得到的是关联数组，json_encode 之后
// 仍然保持原来的键顺序；Go 的 map 会把键排序，因此这里自己实现一份
// 有序对象，保证请求体与 PHP 侧逐字一致。
type jsonObject struct {
	keys []string
	vals []any
}

// set 追加一个键值对（同名键覆盖）。
func (o *jsonObject) set(key string, val any) {
	for i, k := range o.keys {
		if k == key {
			o.vals[i] = val
			return
		}
	}
	o.keys = append(o.keys, key)
	o.vals = append(o.vals, val)
}

// decodeJSON 解析 JSON 文本，并保留对象的键顺序。
//
// 数字用 json.Number 承接，保留原始字面量（大整数不会被转成浮点）。
func decodeJSON(raw string) (any, error) {
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()

	value, err := decodeJSONValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("JSON 尾部有多余内容")
		}
		return nil, err
	}
	return value, nil
}

// decodeJSONValue 递归解析一个 JSON 值。
func decodeJSONValue(dec *json.Decoder) (any, error) {
	token, err := dec.Token()
	if err != nil {
		return nil, err
	}

	delim, ok := token.(json.Delim)
	if !ok {
		// 标量：字符串 / 数字（json.Number）/ 布尔 / null。
		return token, nil
	}

	switch delim {
	case '{':
		obj := &jsonObject{}
		for dec.More() {
			keyToken, err := dec.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyToken.(string)
			if !ok {
				return nil, fmt.Errorf("JSON 对象的键不是字符串")
			}
			val, err := decodeJSONValue(dec)
			if err != nil {
				return nil, err
			}
			obj.set(key, val)
		}
		if _, err := dec.Token(); err != nil { // 消费 '}'
			return nil, err
		}
		return obj, nil
	case '[':
		arr := []any{}
		for dec.More() {
			val, err := decodeJSONValue(dec)
			if err != nil {
				return nil, err
			}
			arr = append(arr, val)
		}
		if _, err := dec.Token(); err != nil { // 消费 ']'
			return nil, err
		}
		return arr, nil
	}
	return nil, fmt.Errorf("非法的 JSON 定界符 %q", byte(delim))
}

// encodeJSON 按 PHP json_encode() 的语义编码一个值。
//
// 与 Go 的 encoding/json 有三处差异，这里逐一对齐：
//
//  1. 正斜杠转义成 \/（PHP 始终转义，Go 默认不转义）；
//  2. 非 ASCII 字符转义成 \uXXXX（PHP 默认转义，Go 输出原始 UTF-8）；
//  3. 空对象编码成 []（PHP 的空数组与空对象无法区分）。
func encodeJSON(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case string:
		return phpQuote(t)
	case json.Number:
		return t.String()
	case bool:
		if t {
			return "true"
		}
		return "false"
	case []string:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			parts = append(parts, encodeJSON(item))
		}
		return "[" + strings.Join(parts, ",") + "]"
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			parts = append(parts, encodeJSON(item))
		}
		return "[" + strings.Join(parts, ",") + "]"
	case *jsonObject:
		if len(t.keys) == 0 {
			return "[]"
		}
		parts := make([]string, 0, len(t.keys))
		for i, key := range t.keys {
			parts = append(parts, phpQuote(key)+":"+encodeJSON(t.vals[i]))
		}
		return "{" + strings.Join(parts, ",") + "}"
	default:
		raw, err := json.Marshal(t)
		if err != nil {
			return "null"
		}
		return string(raw)
	}
}

// phpQuote 按 PHP json_encode() 的规则把字符串编码成 JSON 字符串。
//
// 转义规则（与 internal/events 里的 phpJSONEscape 一致）：
//
//	" → \"        \ → \\        / → \/
//	控制字符 → \b \f \n \r \t 或 \u00xx
//	非 ASCII → \uXXXX（超出 BMP 的用代理对）
func phpQuote(s string) string {
	var sb strings.Builder
	sb.Grow(len(s) + 2)
	sb.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			sb.WriteString(`\"`)
		case '\\':
			sb.WriteString(`\\`)
		case '/':
			sb.WriteString(`\/`)
		case '\b':
			sb.WriteString(`\b`)
		case '\f':
			sb.WriteString(`\f`)
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		default:
			switch {
			case r < 0x20:
				fmt.Fprintf(&sb, `\u%04x`, r)
			case r < 0x80:
				sb.WriteRune(r)
			case r > 0xFFFF:
				// 代理对：\udXXX\udXXX（PHP 的 json_encode 也是这个形式）。
				r -= 0x10000
				fmt.Fprintf(&sb, `\u%04x\u%04x`, 0xD800+(r>>10), 0xDC00+(r&0x3FF))
			default:
				fmt.Fprintf(&sb, `\u%04x`, r)
			}
		}
	}
	sb.WriteByte('"')
	return sb.String()
}
