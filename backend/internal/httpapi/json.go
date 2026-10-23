package httpapi

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// 本文件实现 **PHP 风格** 的 JSON 编码。
//
// 为什么不能直接用 encoding/json：
//
// PHP 的 json_encode 默认（未开启 JSON_UNESCAPED_UNICODE / JSON_UNESCAPED_SLASHES）
// 有两个与 Go 标准库不同的行为：
//
//  1. 非 ASCII 字符被转成 \uXXXX；
//     PHP: {"code":1,"message":"\u8bf7\u8f93\u5165\u90ae\u7bb1","extra":[]}
//     Go : {"code":1,"message":"请输入邮箱","extra":[]}
//
//  2. 斜杠 "/" 被转成 "\/"；
//     PHP: {"code":0,"message":"","extra":{"redirect":"\/repositories"}}
//     Go : {"code":0,"message":"","extra":{"redirect":"/repositories"}}
//
// 两者对前端没有功能影响（JSON.parse 的结果相同），但**字节级比对**要求与 PHP
// 一致，因此 web 控制器命名空间（_ajax_message 信封）必须使用这里的编码。
//
// 注意：/api/* 命名空间用的是 contract.WriteJSON（标准库 json.Marshal），
// 那是另一个命名空间，不在本次改动范围内。

// phpJSONString 按 PHP 的 json_encode 规则转义字符串（含首尾引号）。
//
// PHP 对非 BMP 字符（> U+FFFF）用 UTF-16 代理对表示，与 Go 的 rune 语义不同，
// 这里显式转换。
func phpJSONString(s string) string {
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

// phpJSONEncode 按 PHP 的 json_encode 默认行为编码任意值。
//
// 支持的类型：nil、string、int、bool、[]any、map[string]any、map[string]string、
// AjaxObject。其它类型退回标准库编码（此时不做 \\uXXXX 转义）。
func phpJSONEncode(v any) (string, error) {
	switch t := v.(type) {
	case nil:
		return "null", nil
	case string:
		return phpJSONString(t), nil
	case int:
		return strconv.Itoa(t), nil
	case bool:
		if t {
			return "true", nil
		}
		return "false", nil
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			s, err := phpJSONEncode(item)
			if err != nil {
				return "", err
			}
			parts = append(parts, s)
		}
		return "[" + strings.Join(parts, ",") + "]", nil
	case AjaxObject:
		parts := make([]string, 0, len(t))
		for _, f := range t {
			s, err := phpJSONEncode(f.Value)
			if err != nil {
				return "", err
			}
			parts = append(parts, phpJSONString(f.Key)+":"+s)
		}
		return "{" + strings.Join(parts, ",") + "}", nil
	case map[string]any:
		return phpJSONEncodeMap(t)
	case map[string]string:
		conv := make(map[string]any, len(t))
		for k, val := range t {
			conv[k] = val
		}
		return phpJSONEncodeMap(conv)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("序列化 JSON 失败: %w", err)
		}
		return string(b), nil
	}
}

// phpJSONEncodeMap 编码 map。
//
// PHP 的 json_encode 保持数组的**插入顺序**，而 Go 的 map 没有顺序，
// 因此这里按键名排序以保证输出稳定。当前调用方只传单键对象
// （redirect / key），排序结果与 PHP 完全一致。
func phpJSONEncodeMap(m map[string]any) (string, error) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		s, err := phpJSONEncode(m[k])
		if err != nil {
			return "", err
		}
		parts = append(parts, phpJSONString(k)+":"+s)
	}
	return "{" + strings.Join(parts, ",") + "}", nil
}

// AjaxField 是 AJAX 信封 extra 对象里的一个键值对。
type AjaxField struct {
	Key   string
	Value any
}

// AjaxObject 是**保持插入顺序**的 JSON 对象。
//
// PHP 侧的 extra 是数组，json_encode 后是对象时保持插入顺序；
// 用它可以避免 Go map 的随机迭代顺序导致响应体不可复现。
type AjaxObject []AjaxField
