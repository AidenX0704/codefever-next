package webhook

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

// eventDataMap 是事件数据字段的对外名称映射。
//
// 逐字对应 application/controllers/backend/repository_webhook.php 的
// private $_eventDataMap —— 事件数据里的短字段名换成对外的名字，
// 没有在映射表里的字段原样保留。
var eventDataMap = map[string]string{
	"rKey":       "repositoryId",
	"gKey":       "groupId",
	"uid":        "userId",
	"mrKey":      "mergerequestId",
	"sourceRKey": "sourceRepositoryId",
	"sourceGKey": "sourceGroupId",
	"mrrKey":     "reviewId",
}

// headerOrder 是请求头的固定顺序（写入日志时按这个顺序输出）。
//
// 逐字对应 PHP 侧 $headers 数组的字面量顺序。
var headerOrder = []string{
	"Request-URL",
	"Request-Method",
	"Content-Type",
	"Accept",
	"User-Agent",
	"X-CodeFever-Id",
	"X-CodeFever-Event",
	"X-CodeFever-Signature",
}

// buildBody 组装 webhook 请求体。
//
// 逐字对应 PHP 的 json_encode([...])：
//
//	{"event": 事件类型,
//	 "data": 字段映射后的事件数据,
//	 "repository": {"id": 仓库主键, "url": 仓库地址},
//	 "sender": {"id": 用户主键, "name": 用户名}}
//
// data 已经是编码好的 JSON 文本（见 normalizeEventData）。
func buildBody(evType, data, repoKey, repoURL, userKey, userName string) (string, error) {
	var sb strings.Builder
	sb.WriteString(`{"event":`)
	sb.WriteString(phpQuote(evType))
	sb.WriteString(`,"data":`)
	sb.WriteString(data)
	sb.WriteString(`,"repository":{"id":`)
	sb.WriteString(phpQuote(repoKey))
	sb.WriteString(`,"url":`)
	sb.WriteString(phpQuote(repoURL))
	sb.WriteString(`},"sender":{"id":`)
	sb.WriteString(phpQuote(userKey))
	sb.WriteString(`,"name":`)
	sb.WriteString(phpQuote(userName))
	sb.WriteString(`}}`)
	return sb.String(), nil
}

// buildHeaders 组装请求头。
//
// 逐字对应 PHP 的 $headers：
//
//	Request-URL / Request-Method / Content-Type / Accept / User-Agent
//	X-CodeFever-Id / X-CodeFever-Event / X-CodeFever-Signature
func buildHeaders(url, evType, requestID, body, secret string) map[string]string {
	return map[string]string{
		"Request-URL":           url,
		"Request-Method":        http.MethodPost,
		"Content-Type":          "application/json",
		"Accept":                "application/json",
		"User-Agent":            "CodeFever-Webhook",
		"X-CodeFever-Id":        requestID,
		"X-CodeFever-Event":     evType,
		"X-CodeFever-Signature": "md5=" + signature(body, secret),
	}
}

// signature 计算请求签名：md5(body . secret)。
//
// 逐字对应 PHP 的 _signature()：把 body 与密钥拼接后取 md5，
// 密钥为空时按空串拼接。
// 注意 PHP 的三元判断是**假值**判断，因此密钥为 "0" 时同样按空串处理。
func signature(body, secret string) string {
	if secret == "" || secret == "0" {
		secret = ""
	}
	sum := md5.Sum([]byte(body + secret))
	return hex.EncodeToString(sum[:])
}

// normalizeEventData 把事件数据映射成 webhook 载荷并编码。
//
// 逐字对应 PHP 的 _normalizeEventData(json_decode($event['rwe_data'], TRUE))：
// 短字段名换成对外名字，未命中的字段原样保留，字段顺序不变。
func normalizeEventData(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "[]", nil
	}
	value, err := decodeJSON(raw)
	if err != nil {
		return "", fmt.Errorf("解析事件数据失败: %w", err)
	}

	obj, ok := value.(*jsonObject)
	if !ok {
		// 不是对象就原样输出（PHP 侧 json_decode 失败时 data 也是空的）。
		return encodeJSON(value), nil
	}

	mapped := &jsonObject{}
	for i, key := range obj.keys {
		name := key
		if renamed, ok := eventDataMap[key]; ok {
			name = renamed
		}
		mapped.set(name, obj.vals[i])
	}
	return encodeJSON(mapped), nil
}

// encodeLogPayload 生成 rwl_request 的内容。
//
// 逐字对应 PHP 的 json_encode(['headers' => $headers, 'body' => $body])。
func encodeLogPayload(headers map[string]string, body string) string {
	obj := &jsonObject{}
	hdr := &jsonObject{}
	for _, key := range headerOrder {
		if v, ok := headers[key]; ok {
			hdr.set(key, v)
		}
	}
	obj.set("headers", hdr)
	obj.set("body", body)
	return encodeJSON(obj)
}

// encodeLogResponse 生成 rwl_response 的内容。
//
// 逐字对应 PHP 的 json_encode(['headers' => $responseHeaders, 'body' => $responseBody])：
// 响应头是"名字 → 多个取值"的数组；传输失败时 PHP 记空数组与空串，
// 这里把错误信息写进 body，方便排查。
func encodeLogResponse(resp Response, cause error) string {
	obj := &jsonObject{}
	hdr := &jsonObject{}
	keys := make([]string, 0, len(resp.Headers))
	for k := range resp.Headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		hdr.set(k, resp.Headers[k])
	}
	obj.set("headers", hdr)

	body := resp.Body
	if cause != nil {
		body = cause.Error()
	}
	obj.set("body", body)
	return encodeJSON(obj)
}
