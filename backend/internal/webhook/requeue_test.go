package webhook

import (
	"strings"
	"testing"
)

// 本文件覆盖"按投递日志重新入队"的**纯逻辑**部分（不需要数据库）。
//
// 关键不变量：
//   - 能从 rwl_request（{headers, body}）里还原 event / data / sender / repository；
//   - data 重新序列化后仍是 PHP 风格的 JSON（键序保留、正斜杠转义）；
//   - 缺少事件类型 / 缺少请求体时报错而不是产出生而为死信的事件。

// TestParseRequeuePayload 校验载荷还原。
func TestParseRequeuePayload(t *testing.T) {
	// 与投递器写日志时的真实形态一致：encodeLogPayload + buildBody 的产物。
	data := `{"repositoryId":"r1","groupId":"g1","action":"opened"}`
	body, err := buildBody("repo:create", data, "repo-key", "http://host/g/r", "u1", "alice")
	if err != nil {
		t.Fatalf("构造请求体失败: %v", err)
	}
	request := encodeLogPayload(buildHeaders("http://host/hook", "repo:create", "req-1", body, "secret"), body)

	payload, err := parseRequeuePayload(request)
	if err != nil {
		t.Fatalf("解析载荷失败: %v", err)
	}
	if payload.Type != "repo:create" {
		t.Errorf("Type = %q, want repo:create", payload.Type)
	}
	if payload.UserKey != "u1" {
		t.Errorf("UserKey = %q, want u1", payload.UserKey)
	}
	if payload.RepoKey != "repo-key" {
		t.Errorf("RepoKey = %q, want repo-key", payload.RepoKey)
	}
	// data 字段必须原样保留（重新入队后投递器再次 normalize 仍得到相同输出）。
	if !strings.Contains(payload.Data, `"repositoryId":"r1"`) {
		t.Errorf("Data 丢失字段映射结果: %s", payload.Data)
	}
}

// TestParseRequeuePayloadMissingPieces 校验残缺日志的处理。
func TestParseRequeuePayloadMissingPieces(t *testing.T) {
	if _, err := parseRequeuePayload(`{"headers":{},"body":"not-json"}`); err == nil {
		t.Error("请求体不是 JSON 应当报错")
	}
	if _, err := parseRequeuePayload(`[]`); err == nil {
		t.Error("请求载荷不是对象应当报错")
	}
	if _, err := parseRequeuePayload(`{"headers":{}}`); err == nil {
		t.Error("缺少 body 应当报错")
	}
	// data 缺失不算错误：入队时给空对象形态，投递器能组装请求体。
	payload, err := parseRequeuePayload(`{"headers":{},"body":"{\"event\":\"repo:create\"}"}`)
	if err != nil {
		t.Fatalf("缺 data 不应报错: %v", err)
	}
	if payload.Data != "[]" {
		t.Errorf("缺 data 时应回退为空对象形态, got %q", payload.Data)
	}
}
