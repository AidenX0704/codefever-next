package handlers

import (
	"context"

	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
	"codefever-next/backend/internal/httpapi"
	"codefever-next/backend/internal/repo"
)

// 全局搜索（阶段一）的取数上限与关键词约束。
const (
	// searchBucketLimit 是每一类结果最多返回多少条。
	//
	// 三类各 10 条足够判断"是不是我要找的东西"；再多就该细化关键词了。
	searchBucketLimit = 10
	// searchKeywordMaxLength 是关键词的字符上限（按 rune 计）。
	//
	// 超过它的输入几乎必然是误粘贴（整段日志/URL），拒绝比截断更诚实：
	// 截断后的搜索结果与用户输入不对应，反而更难理解。
	searchKeywordMaxLength = 100
)

// SearchAPIHandler 实现 `/api/search/*`。
//
// 与 `home` 同理：搜索跨 user / repository / group 三个域，塞进任何一个既有控制器
// 都会让那个控制器的语义变糊，因此单独成一个 API 控制器。**新增控制器不影响
// 123 条 PHP 契约基线**（基线只比对已记录的端点）。
type SearchAPIHandler struct {
	Search *repo.SearchRepo
}

// NewSearchAPIHandler 创建搜索处理器。
func NewSearchAPIHandler(search *repo.SearchRepo) *SearchAPIHandler {
	return &SearchAPIHandler{Search: search}
}

// Register 把 `/api/search/*` 注册到服务器。
func (h *SearchAPIHandler) Register(s *httpapi.Server) {
	s.Register(&httpapi.Controller{
		Name: "search",
		Auth: httpapi.AuthWebUser,
		Actions: map[string]map[string]httpapi.ActionHandler{
			"all": {"GET": h.all},
		},
	})
}

// all 实现 GET /api/search/all?keyword=…
//
// 三类结果一次返回：仓库 / 仓库组 / 合并请求。全部只返回**我可见**的对象
// （SQL 里 JOIN 成员表，见 repo/search.go）。
func (h *SearchAPIHandler) all(ctx context.Context, req *httpapi.Request) (any, error) {
	u := req.User
	if u == nil {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	keyword := repo.TrimSearchKeyword(req.Parsed.QueryString("keyword"))
	if keyword == "" {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}
	if len([]rune(keyword)) > searchKeywordMaxLength {
		return nil, contract.NewError(contract.CodeAmountOutOfRange)
	}

	// 每类多取一条，用来判断"还有更多"。
	fetchLimit := searchBucketLimit + 1

	repositories, err := h.Search.SearchRepositories(ctx, u.Key, keyword, fetchLimit)
	if err != nil {
		return nil, err
	}
	groups, err := h.Search.SearchGroups(ctx, u.Key, keyword, fetchLimit)
	if err != nil {
		return nil, err
	}
	mergeRequests, err := h.Search.SearchMergeRequests(ctx, u.Key, keyword, fetchLimit)
	if err != nil {
		return nil, err
	}

	results := domain.SearchResults{Keyword: keyword}
	results.Repositories, results.Truncated.Repositories = clipSearchBucket(repositories)
	results.Groups, results.Truncated.Groups = clipSearchBucket(groups)
	results.MergeRequests, results.Truncated.MergeRequests = clipSearchBucket(mergeRequests)
	return results, nil
}

// clipSearchBucket 把"多取一条"的切片裁回上限，并报告是否被截断。
//
// 泛型放在这里而不是每个类型写一遍：三类结果的裁剪逻辑必须完全一致，
// 否则前端会出现"某类说还有更多、另一类其实没有"的诡异差异。
func clipSearchBucket[T any](list []T) ([]T, bool) {
	if len(list) > searchBucketLimit {
		return list[:searchBucketLimit], true
	}
	if list == nil {
		return []T{}, false
	}
	return list, false
}
