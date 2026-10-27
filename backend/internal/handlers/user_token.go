package handlers

import (
	"context"
	"strings"

	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
	"codefever-next/backend/internal/httpapi"
)

// 访问令牌（公开 API `/api/v1` 的凭据）的管理端点。
//
// 背景：令牌的**存储与校验**在 2026-10-25 随公开 API 一起落地
// （`repo.AccessTokenRepo` + `cc_access_tokens` + 只存 SHA-256），
// 但一直**没有 Web 端的创建/吊销入口** —— 也就是"能力存在、用户拿不到"。
// 本批补上三个端点，把这条死功能接通。
//
// 安全约定（与 repo 层一致，这里只做透传，不重新发明）：
//   - 明文**只在创建响应里出现一次**，库里只有 SHA-256；
//   - 列表**绝不返回 at_sha256** —— 摘要虽不可逆，但没必要给前端；
//   - 吊销只能作用于自己名下的令牌（仓储层用 `u_key = ?` 约束，
//     不匹配返回 ErrNotFound，不泄露"该令牌存在但不属于我"）。
const (
	// tokenNameMaxLength 是令牌名的长度上限。
	//
	// 列宽是 255，但令牌名是给人看的标签，64 已经足够；限制在这里而不是跟着列宽走，
	// 是因为超长名字在列表里必然被截断，写进去也没有意义。
	tokenNameMaxLength = 64
	// tokenMaxPerUser 是每人最多持有的令牌数。
	//
	// 不设上限的话，脚本化创建会把表刷满；20 个足够覆盖"笔记本 + CI + 临时脚本"。
	tokenMaxPerUser = 20
)

// accessTokenView 是列表/创建响应里的令牌对象。
//
// 字段口径与其它内部端点一致：时间用**秒级时间戳**（`phpTimeOf`），
// 主键叫 `id`（与前端 `api/types.ts` 的其它实体同名习惯一致）。
type accessTokenView struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Scope    string `json:"scope"`
	Created  int64  `json:"created"`
	LastUsed *int64 `json:"lastUsed"`
}

func tokenView(t *domain.AccessToken) accessTokenView {
	view := accessTokenView{
		ID:      t.Key,
		Name:    t.Name,
		Scope:   t.Scope,
		Created: phpTimeOf(t.Created),
	}
	if t.LastUsed != nil && *t.LastUsed != "" {
		used := phpTimeOf(*t.LastUsed)
		view.LastUsed = &used
	}
	return view
}

// registerTokenActions 把令牌管理端点挂到 `user` 控制器上。
//
// 为什么不新开一个控制器：这三个动作的鉴权等级与 `user` 域完全一致
// （都要登录、都只作用于"我"），新开控制器只会让 `controllerNames` 多一个空壳。
func (h *UserHandler) registerTokenActions(actions map[string]map[string]httpapi.ActionHandler) {
	if h.Tokens == nil {
		// 未注入仓储时**不注册**这些动作：请求会落到 0x0105（动作不存在），
		// 比"路由存在但必然 500"更容易排查。
		return
	}
	actions["listToken"] = map[string]httpapi.ActionHandler{"GET": h.listToken}
	actions["createToken"] = map[string]httpapi.ActionHandler{"POST": h.createToken}
	actions["deleteToken"] = map[string]httpapi.ActionHandler{"POST": h.deleteToken}
}

// listToken 实现 GET /api/user/listToken。
func (h *UserHandler) listToken(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	list, err := h.Tokens.List(ctx, u.Key)
	if err != nil {
		return nil, err
	}
	out := make([]accessTokenView, 0, len(list))
	for i := range list {
		out = append(out, tokenView(&list[i]))
	}
	return out, nil
}

// createToken 实现 POST /api/user/createToken。
//
// 入参：`name`（必填，≤64 字，同一用户内唯一）。
// 出参：`{token, item}` —— `token` 是**明文，只此一次**。
func (h *UserHandler) createToken(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	name := strings.TrimSpace(req.Parsed.ParamString("name"))
	if name == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if len([]rune(name)) > tokenNameMaxLength {
		return nil, contract.NewError(contract.CodeAmountOutOfRange)
	}

	// 先查重再查配额：重名是用户最常犯的错，先报它更友好。
	exists, err := h.Tokens.NameExists(ctx, u.Key, name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, contract.NewError(contract.CodeTargetExists)
	}

	list, err := h.Tokens.List(ctx, u.Key)
	if err != nil {
		return nil, err
	}
	if len(list) >= tokenMaxPerUser {
		return nil, contract.NewError(contract.CodeAmountOutOfRange)
	}

	token, plaintext, err := h.Tokens.Create(ctx, u.Key, name)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"token": plaintext,
		"item":  tokenView(token),
	}, nil
}

// deleteToken 实现 POST /api/user/deleteToken。
//
// 入参：`id`（令牌主键）。只能吊销自己名下的令牌；
// 不存在 / 不属于我，都返回同一个错误码，不区分。
func (h *UserHandler) deleteToken(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	atKey := strings.TrimSpace(req.Parsed.ParamString("id"))
	if atKey == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if err := h.Tokens.Delete(ctx, u.Key, atKey); err != nil {
		return nil, contract.NewError(contract.CodeOperateFail)
	}
	return map[string]any{"id": atKey}, nil
}
