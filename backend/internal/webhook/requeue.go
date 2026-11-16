package webhook

import (
	"context"
	"fmt"

	"codefever-next/backend/internal/contract"
)

// 重新投递（按投递日志重建事件并入队）。
//
// 入队路径与领域事件一致（INSERT INTO cc_repository_webhook_events），
// 投递由常驻投递器按既有退避策略执行 —— 本文件只负责"把日志还原成
// 一条可投递的事件"，不绕过队列直接发请求。

// RequeuePayload 是从投递日志里还原出的可重投载荷。
type RequeuePayload struct {
	// Type 是事件类型（原始请求头 X-CodeFever-Event / body.event）。
	Type string
	// Data 是事件数据（body.data 的原文，保留 PHP 风格的字段映射结果）。
	Data string
	// UserKey 是原始触发者（body.sender.id）。
	UserKey string
	// RepoKey 是仓库主键（body.repository.id），仅用于日志与校验。
	RepoKey string
}

// parseRequeuePayload 从 rwl_request 的内容（{headers, body}）还原载荷。
//
// body 的结构见 buildBody()：{"event","data","repository","sender"}。
// 解析用 decodeJSON（保序、数字用 json.Number），data 再用 encodeJSON
// 重新序列化 —— 落库形态与事件入队时的 rwe_data 完全同构。
func parseRequeuePayload(requestJSON string) (*RequeuePayload, error) {
	root, err := decodeJSON(requestJSON)
	if err != nil {
		return nil, fmt.Errorf("解析投递日志请求失败: %w", err)
	}
	obj, ok := root.(*jsonObject)
	if !ok {
		return nil, fmt.Errorf("投递日志请求不是对象")
	}

	var bodyText string
	for i, key := range obj.keys {
		if key == "body" {
			if s, ok := obj.vals[i].(string); ok {
				bodyText = s
			}
			break
		}
	}
	if bodyText == "" {
		return nil, fmt.Errorf("投递日志缺少请求体")
	}

	body, err := decodeJSON(bodyText)
	if err != nil {
		return nil, fmt.Errorf("解析投递请求体失败: %w", err)
	}
	bodyObj, ok := body.(*jsonObject)
	if !ok {
		return nil, fmt.Errorf("投递请求体不是对象")
	}

	payload := &RequeuePayload{}
	for i, key := range bodyObj.keys {
		switch key {
		case "event":
			if s, ok := bodyObj.vals[i].(string); ok {
				payload.Type = s
			}
		case "data":
			payload.Data = encodeJSON(bodyObj.vals[i])
		case "repository":
			if repo, ok := bodyObj.vals[i].(*jsonObject); ok {
				payload.RepoKey = objectStringField(repo, "id")
			}
		case "sender":
			if sender, ok := bodyObj.vals[i].(*jsonObject); ok {
				payload.UserKey = objectStringField(sender, "id")
			}
		}
	}
	if payload.Type == "" {
		return nil, fmt.Errorf("投递日志缺少事件类型")
	}
	if payload.Data == "" {
		// data 字段缺失与"空数据"是两回事：入队时一律给空对象形态，
		// 让投递器能组装出请求体。
		payload.Data = "[]"
	}
	return payload, nil
}

// objectStringField 读取 jsonObject 里的一个字符串字段。
func objectStringField(obj *jsonObject, key string) string {
	for i, k := range obj.keys {
		if k == key {
			if s, ok := obj.vals[i].(string); ok {
				return s
			}
			return ""
		}
	}
	return ""
}

// RequeueFromLog 按投递日志重建一条 webhook 事件并入队，返回新事件主键。
//
// 触发者缺失时回退到 fallbackUser（当前操作者）：rwe_user 为空的行在
// 投递时必然失败（buildRequest 报"触发用户不存在"），宁可记操作者
// 也不要产出生而为死信的事件。
//
// webhook 已被删除时 INSERT 仍会成功（表上没有外键），但投递器领取时
// INNER JOIN cc_repository_webhooks 会把它过滤掉 —— 调用方必须先确认
// webhook 存在（handlers 侧已做）。
func (s *MySQLStore) RequeueFromLog(ctx context.Context, webhookKey, requestJSON, fallbackUser string) (string, error) {
	payload, err := parseRequeuePayload(requestJSON)
	if err != nil {
		return "", err
	}
	user := payload.UserKey
	if user == "" {
		user = fallbackUser
	}

	eventKey := contract.NewKey()
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO cc_repository_webhook_events (rwe_key, rwe_user, rw_key, rwe_type, rwe_data)
		VALUES (?, ?, ?, ?, ?)`,
		eventKey, user, webhookKey, payload.Type, payload.Data); err != nil {
		return "", fmt.Errorf("重新入队 webhook 事件失败: %w", err)
	}
	return eventKey, nil
}
