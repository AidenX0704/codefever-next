package handlers

import (
	"context"
	"errors"
	"strings"
	"time"

	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
	"codefever-next/backend/internal/httpapi"
	"codefever-next/backend/internal/repo"
)

// 站点内容（更新日志 / 公告）的管理端点。
//
// 挂在**现有 admin 控制器**上（AuthAdminWebUser），不新建控制器 ——
// 与"这些动作的鉴权等级与 admin 域完全一致"的 registerTokenActions
// 取舍同理，避免 controllerNames 出现空壳。
//
// 参数风格对齐 admin 域的既有约定：
//   - id 缺失/非数字 → 0x0201；
//   - 目标不存在 → 0x0405（deleteToken 的同款语义：不区分"不存在"与"失败"）；
//   - 长度超限 → 0x0202（AmountOutOfRange）。

// 站点内容各字段的长度上限。
//
// 与迁移 012 的列宽对应，但**主动收窄**：title 列宽 255，
// 正文 mediumtext 上限 16MB —— 更新说明不需要那么大，
// 10000 字已经远超一次变更说明的合理长度。
const (
	changelogVersionMaxLength = 32
	changelogTitleMaxLength   = 255
	changelogBodyMaxLength    = 10000
	announcementBodyMaxLength = 512
)

// registerSiteActions 把更新日志 / 公告的管理端点挂到 admin 控制器。
//
// Site 未注入时**不注册**这些动作：请求落到 0x0105（动作不存在），
// 比"路由存在但必然 500"更容易排查（与 registerTokenActions 一致）。
func (h *AdminHandler) registerSiteActions(actions map[string]map[string]httpapi.ActionHandler) {
	if h.Site == nil {
		return
	}
	actions["changelogSave"] = map[string]httpapi.ActionHandler{"POST": h.changelogSave}
	actions["changelogDelete"] = map[string]httpapi.ActionHandler{"POST": h.changelogDelete}
	actions["announcementSave"] = map[string]httpapi.ActionHandler{"POST": h.announcementSave}
	actions["announcementDelete"] = map[string]httpapi.ActionHandler{"POST": h.announcementDelete}
}

// changelogSave 实现 POST /api/admin/changelogSave。
//
// 入参：{id?, version, title, body, releasedAt(YYYY-MM-DD 可空)}。
// id 缺省或为 0 → 新建；否则更新。
func (h *AdminHandler) changelogSave(ctx context.Context, req *httpapi.Request) (any, error) {
	p := req.Parsed

	item := &domain.Changelog{
		Version: strings.TrimSpace(p.ParamString("version")),
		Title:   strings.TrimSpace(p.ParamString("title")),
		Body:    p.ParamString("body"),
	}
	// releasedAt 可空：空串 → NULL（"未定发布日期"）。
	releasedAt := strings.TrimSpace(p.ParamString("releasedAt"))
	if releasedAt != "" && !isDateOnly(releasedAt) {
		// 日期格式必须是 YYYY-MM-DD：列是 DATE，写入其它形态会被
		// MySQL 静默截断成 0000-00-00，排序随之失真。
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	item.ReleasedAt = releasedAt

	// id 允许缺省（新建）与显式 0（同样视为新建）。
	if p.HasParam("id") {
		if id := atoiOrZero(p.ParamString("id")); id > 0 {
			item.ID = id
		}
	}

	if len([]rune(item.Version)) > changelogVersionMaxLength ||
		len([]rune(item.Title)) > changelogTitleMaxLength ||
		len([]rune(item.Body)) > changelogBodyMaxLength {
		return nil, contract.NewError(contract.CodeAmountOutOfRange)
	}
	// 标题是列表里唯一能区分条目的字段；正文反而可以为空
	// （占位的"即将发布"条目），因此只有 title 必填。
	if item.Title == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	if err := h.Site.SaveChangelog(ctx, item); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, contract.NewError(contract.CodeOperateFail)
		}
		return nil, err
	}
	return map[string]any{"id": item.ID}, nil
}

// changelogDelete 实现 POST /api/admin/changelogDelete。
//
// 入参：{id}。物理删除；目标不存在同样返回 0x0405。
func (h *AdminHandler) changelogDelete(ctx context.Context, req *httpapi.Request) (any, error) {
	p := req.Parsed

	id := atoiOrZero(p.ParamString("id"))
	if id <= 0 {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Site.DeleteChangelog(ctx, id); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, contract.NewError(contract.CodeOperateFail)
		}
		return nil, err
	}
	return map[string]any{"id": id}, nil
}

// announcementSave 实现 POST /api/admin/announcementSave。
//
// 入参：{id?, type(security|privacy|support), body, enabled(bool)}。
func (h *AdminHandler) announcementSave(ctx context.Context, req *httpapi.Request) (any, error) {
	p := req.Parsed

	anType := strings.TrimSpace(p.ParamString("type"))
	if anType == "" {
		// type 缺省按列默认值 support 处理（与迁移定义一致）。
		anType = string(domain.AnnouncementTypeSupport)
	}
	if !domain.AnnouncementTypeAllowed(anType) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	body := strings.TrimSpace(p.ParamString("body"))
	if body == "" {
		// 公告只有正文，没有"空公告"的意义。
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if len([]rune(body)) > announcementBodyMaxLength {
		return nil, contract.NewError(contract.CodeAmountOutOfRange)
	}

	item := &domain.Announcement{
		Type:    anType,
		Body:    body,
		Enabled: bodyParamBool(p, "enabled", true),
	}
	if p.HasParam("id") {
		if id := atoiOrZero(p.ParamString("id")); id > 0 {
			item.ID = id
		}
	}

	if err := h.Site.SaveAnnouncement(ctx, item); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, contract.NewError(contract.CodeOperateFail)
		}
		return nil, err
	}
	return map[string]any{"id": item.ID}, nil
}

// announcementDelete 实现 POST /api/admin/announcementDelete。
func (h *AdminHandler) announcementDelete(ctx context.Context, req *httpapi.Request) (any, error) {
	p := req.Parsed

	id := atoiOrZero(p.ParamString("id"))
	if id <= 0 {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Site.DeleteAnnouncement(ctx, id); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, contract.NewError(contract.CodeOperateFail)
		}
		return nil, err
	}
	return map[string]any{"id": id}, nil
}

// bodyParamBool 读取布尔参数（body / JSON / query 依次查找）。
//
// 三种来源的取值形态不同：JSON 是真布尔，multipart 是 "1"/"0" 字符串，
// query 恒为字符串。def 是参数缺省时的取值
// （enabled 缺省按开处理，避免"想发布公告却忘了传 enabled"）。
func bodyParamBool(p *contract.Parsed, key string, def bool) bool {
	if !p.HasParam(key) {
		return def
	}
	var raw any
	if v, ok := p.Body[key]; ok {
		raw = v
	} else if v, ok := p.JSON[key]; ok {
		raw = v
	} else {
		raw = p.Query[key]
	}
	switch t := raw.(type) {
	case nil:
		return def
	case bool:
		return t
	case float64:
		return t != 0
	case string:
		// 字符串按 JSON 的 true/false 与数字 1/0 判定；
		// "0" / "false" / 空串为假，其余为真。
		switch strings.TrimSpace(t) {
		case "false", "0", "":
			return false
		}
		return true
	default:
		return def
	}
}

// isDateOnly 判断是否是 YYYY-MM-DD 日期（changelogSave 的 releasedAt 白名单）。
// `0000-00-00` 是既有数据里的 MySQL 零日期哨兵，保持放行；
// 其余值要求月份 / 日期落在真实日历范围 —— Go 标准库 time.Parse 会把
// 2026-13-01 规范化成次年 1 月，因此先做数值上界再交给 Parse。
func isDateOnly(s string) bool {
	if len(s) != 10 || s[4] != '-' || s[7] != '-' {
		return false
	}
	for i, c := range s {
		if i == 4 || i == 7 {
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
	}
	if s == "0000-00-00" {
		return true
	}
	if s[5] > '1' || (s[5] == '1' && s[6] > '2') {
		return false
	}
	if s[8] > '3' || (s[8] == '3' && s[9] > '1') {
		return false
	}
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return false
	}
	return true
}
