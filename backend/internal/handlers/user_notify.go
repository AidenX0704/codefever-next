package handlers

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
	"codefever-next/backend/internal/httpapi"
	"codefever-next/backend/internal/repo"
)

// 注册 user 域的"通知设置 / 国家码"端点。
func (h *UserHandler) registerNotificationActions(actions map[string]map[string]httpapi.ActionHandler) {
	actions["notifications"] = map[string]httpapi.ActionHandler{"GET": h.notifications}
	actions["setNotificationRead"] = map[string]httpapi.ActionHandler{"POST": h.setNotificationRead}
	actions["deleteNotification"] = map[string]httpapi.ActionHandler{"POST": h.deleteNotification}
	actions["userNotificationSetting"] = map[string]httpapi.ActionHandler{"POST": h.userNotificationSetting}
	actions["notificationRefused"] = map[string]httpapi.ActionHandler{"GET": h.notificationRefused}
	actions["setGroupOrRepoNotification"] = map[string]httpapi.ActionHandler{"POST": h.setGroupOrRepoNotification}
	actions["getCountriesCode"] = map[string]httpapi.ActionHandler{"POST": h.getCountriesCode}
}

// notifications 实现 GET /api/user/notifications。
//
// 逐字对应 application/controllers/api/user.php:444-464：
// category 必须属于 [0,1,2,3]，否则 0x0201。
//
// 注意分页：PHP 从 query 取 page，而 query 的 page 被强制归一化为 1，
// pagesize 则由模型默认 20。
func (h *UserHandler) notifications(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	category := req.Parsed.QueryInt("category")
	switch category {
	case repo.NotificationCategoryAll, repo.NotificationCategoryUnread,
		repo.NotificationCategorySystem, repo.NotificationCategoryMentionMe:
	default:
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	page := req.Parsed.QueryInt("page")
	if page <= 0 {
		page = 1
	}

	list, err := h.Notifications.List(ctx, u.Key, category, page, 20)
	if err != nil {
		return nil, err
	}

	out := make([]any, 0, len(list))
	for i := range list {
		out = append(out, map[string]any{
			"id":         list[i].ID,
			"type":       list[i].Type,
			"group":      list[i].Group,
			"repository": list[i].Repository,
			// data 必须是 **JSON 字符串** —— 前端会对其做 JSON.parse。
			"data":    list[i].Data,
			"created": list[i].Created,
			"isRead":  list[i].IsRead,
		})
	}
	return out, nil
}

// setNotificationRead 实现 POST /api/user/setNotificationRead。
//
// 逐字对应 application/controllers/api/user.php:466-481：
// all 与 id **有且只能有一个**（都传或都不传都报 0x0201）。
func (h *UserHandler) setNotificationRead(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	all := phpTruthy(req.Parsed.Body["all"])
	nuKey := req.Parsed.BodyString("id")
	if (all && nuKey != "") || (!all && nuKey == "") {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	if err := h.Notifications.SetRead(ctx, u.Key, all, nuKey); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return []any{}, nil
}

// deleteNotification 实现 POST /api/user/deleteNotification。
//
// 逐字对应 application/controllers/api/user.php:483-498。
// all=true 时只删除**已读**的通知（PHP 模型里多了一个 nu_is_read = NOTIFY_READ 条件）。
func (h *UserHandler) deleteNotification(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	all := phpTruthy(req.Parsed.Body["all"])
	nuKey := req.Parsed.BodyString("id")
	if (all && nuKey != "") || (!all && nuKey == "") {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	if err := h.Notifications.DeleteReaded(ctx, u.Key, all, nuKey); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return []any{}, nil
}

// userNotificationSetting 实现 POST /api/user/userNotificationSetting。
//
// 逐字对应 application/controllers/api/user.php:500-523：
// offset 必须是 1（合并请求通知）或 2（邮件通知），否则 0x0201；
// 然后用 XOR 翻转对应位 —— **位 = 1 表示关闭**。
func (h *UserHandler) userNotificationSetting(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	offset := req.Parsed.BodyInt("offset")
	switch offset {
	case repo.NotificationStatusMR, repo.NotificationStatusEmail:
	default:
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	status := u.NotificationStatus ^ (1 << uint(offset-1))
	if err := h.Users.UpdateFields(ctx, u.Key, map[string]any{
		"u_notification_status": status,
		"u_updated":             time.Now(),
	}); err != nil {
		return nil, err
	}
	return []any{}, nil
}

// notificationRefused 实现 GET /api/user/notificationRefused。
//
// 逐字对应 application/controllers/api/user.php:525-533。
func (h *UserHandler) notificationRefused(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	list, err := h.Notifications.ListRefused(ctx, u.Key)
	if err != nil {
		return nil, err
	}
	out := make([]any, 0, len(list))
	for i := range list {
		out = append(out, map[string]any{
			"id":     list[i].ID,
			"type":   list[i].Type,
			"target": list[i].Target,
		})
	}
	return out, nil
}

// setGroupOrRepoNotification 实现 POST /api/user/setGroupOrRepoNotification。
//
// 逐字对应 application/controllers/api/user.php:535-563：
// type 必须是 1（仓库组）或 2（仓库），target 必填，否则 0x0201。
//
// 级联语义（notification_model.php:180-258）：
//   - 关闭**组**通知 → 该组下所有尚未单独关闭的仓库也一并关闭；
//   - 恢复**组**通知 → 该组下所有仓库的关闭记录一并删除；
//   - 恢复**仓库**通知 → 同时恢复它所属组的关闭记录。
func (h *UserHandler) setGroupOrRepoNotification(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	typ := req.Parsed.BodyInt("type")
	target := req.Parsed.BodyString("target")
	if (typ != repo.NotificationRefuseGroup && typ != repo.NotificationRefuseRepository) || target == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	open := phpTruthy(req.Parsed.Body["open"])

	if open {
		if err := h.Notifications.SetReceive(ctx, u.Key, typ, target); err != nil {
			return nil, contract.NewError(contract.CodeOperateFail)
		}
		return []any{}, nil
	}

	if err := h.Notifications.SetRefused(ctx, u.Key, typ, target); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return []any{}, nil
}

// getCountriesCode 实现 POST /api/user/getCountriesCode。
//
// 逐字对应 application/controllers/api/user.php:98-106：
// 返回整个 countries 配置项。
//
// 实测 PHP 侧的 data 结构是：
//
//	{"countries": {"common": [...], "0": {...}, "1": {...}, ...}}
//
// 即 countries 是一个**混合键数组**（字符串键 "common" + 数字键），
// 因此 JSON 编码后是对象而不是数组。Go 的 map 会按键名排序，
// 与 PHP 的插入顺序不同，因此这里用自定义类型保持 "common" 在前、数字键递增的顺序。
func (h *UserHandler) getCountriesCode(ctx context.Context, req *httpapi.Request) (any, error) {
	return map[string]any{
		"countries": newCountriesPayload(),
	}, nil
}

// countryPayload 是单个国家的对外形态。
//
// JSON 键名逐字对应 PHP 的 iso_code / calling_code / name_cn / name。
type countryPayload struct {
	ISOCode     string `json:"iso_code"`
	CallingCode string `json:"calling_code"`
	NameCN      string `json:"name_cn"`
	Name        string `json:"name"`
}

// countriesPayload 保持 PHP 数组的键顺序："common" 在前，数字键递增。
type countriesPayload struct {
	common  []countryPayload
	ordered []countryPayload
}

// newCountriesPayload 从 domain 的国家数据构造。
func newCountriesPayload() *countriesPayload {
	p := &countriesPayload{
		common:  make([]countryPayload, 0, len(domain.CommonCountries)),
		ordered: make([]countryPayload, 0, len(domain.Countries)),
	}
	for _, c := range domain.CommonCountries {
		p.common = append(p.common, toCountryPayload(c))
	}
	for _, c := range domain.Countries {
		p.ordered = append(p.ordered, toCountryPayload(c))
	}
	return p
}

// toCountryPayload 转换单个国家。
func toCountryPayload(c domain.Country) countryPayload {
	return countryPayload{
		ISOCode:     c.ISOCode,
		CallingCode: c.CallingCode,
		NameCN:      c.NameCN,
		Name:        c.Name,
	}
}

// MarshalJSON 按 PHP 的键顺序输出。
func (p *countriesPayload) MarshalJSON() ([]byte, error) {
	var buf []byte
	buf = append(buf, '{')

	key, err := json.Marshal("common")
	if err != nil {
		return nil, err
	}
	buf = append(buf, key...)
	buf = append(buf, ':')
	common, err := json.Marshal(p.common)
	if err != nil {
		return nil, err
	}
	buf = append(buf, common...)

	for i, c := range p.ordered {
		buf = append(buf, ',')
		key, err := json.Marshal(strconv.Itoa(i))
		if err != nil {
			return nil, err
		}
		buf = append(buf, key...)
		buf = append(buf, ':')
		value, err := json.Marshal(c)
		if err != nil {
			return nil, err
		}
		buf = append(buf, value...)
	}

	buf = append(buf, '}')
	return buf, nil
}

// phpTruthy 复刻 PHP 的布尔真值判断。
//
// 关键点：字符串 "0" 在 PHP 里是**假值**。
func phpTruthy(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case string:
		return t != "" && t != "0"
	case int:
		return t != 0
	case int64:
		return t != 0
	case float64:
		return t != 0
	case []any:
		return len(t) > 0
	case map[string]any:
		return len(t) > 0
	default:
		return true
	}
}
