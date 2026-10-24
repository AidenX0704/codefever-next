// Package domain —— 统一分页参数的取值口径。
//
// 2026-10 性能整改 C1。
//
// ## 问题
//
// 整改前，全站所有列表端点的 pagesize 都只有**下限**、没有上限：
//
//	// handlers/repository.go
//	func paging(req *httpapi.Request) (int, int) {
//	    return req.Parsed.QueryInt("page"), req.Parsed.QueryInt("pagesize")
//	}
//	// repo/repository.go（合并请求列表）
//	if pagesize < 1 { pagesize = 20 }        // 只兜底，不封顶
//
// domain 里其实早就声明了 AdminPageSizeMax = 100，但**全仓库没有任何引用**
// （grep 只有声明本身）。结果 `?pagesize=20000` 会被原样拼进 `LIMIT`，而
// 合并请求列表对**每一行**都要跑一次 `git log -1` 子进程
// （handlers/repository.go 的 normalizeMergeRequests），
// 即单个 HTTP 请求可拉起两万个 git 进程 —— 这是"一个请求打挂全站"的量级。
//
// ## 口径
//
// page 至少 1；pagesize 落在 [1, PageSizeMax]。
// PageSizeMax 取 100：与 domain.AdminPageSizeMax 一致，同时覆盖前端现有的
// 最大用法（CommitList 每页 20 累加、admin 列表默认 20）。
package domain

const (
	// PageSizeDefault 是缺省每页条数（与 PHP 侧 $pagesize ? $pagesize : 20 一致）。
	PageSizeDefault = 20

	// PageSizeMax 是任何列表端点允许的最大每页条数。
	//
	// 之所以是 100 而不是更小：前端 CommitList 的"加载更多"是累加式翻页，
	// 单次请求仍是 20；管理端下拉与成员列表也都在 20–50。100 足够宽松，
	// 同时把最坏情况（每行一个子进程）压到可控范围。
	PageSizeMax = 100
)

// ClampPaging 把分页参数收进合法区间。
//
// 所有列表端点都必须经过它——新增端点若直接使用用户传入的 pagesize，
// 等于重新引入"无上限 LIMIT"这一缺陷。
func ClampPaging(page, pagesize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pagesize < 1 {
		pagesize = PageSizeDefault
	}
	if pagesize > PageSizeMax {
		pagesize = PageSizeMax
	}
	return page, pagesize
}
