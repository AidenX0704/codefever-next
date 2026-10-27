package handlers

import (
	"encoding/json"

	"codefever-next/backend/internal/domain"
)

// activityView 把 `cc_activities` 的一行转成前端 `ActivityItem` 形态。
//
// 为什么抽出来：仓库动态（`/api/repository/activities`）与首页"团队动态"
// （`/api/home/overview`）都要这段映射，逐字重复会出现两份不一致的 JSON 形状
// （最典型的是 `time`：这里必须走 `phpTimeOf` 把 MySQL 时间串转成秒级时间戳）。
func activityView(a *domain.RepositoryActivity) domain.RepositoryActivityView {
	view := domain.RepositoryActivityView{
		ID:         a.Key,
		Type:       a.Type,
		Creator:    a.UKey,
		Group:      a.GroupKey,
		Repository: a.Repository,
		Time:       phpTimeOf(a.Created),
	}
	if a.Data != nil {
		var content any
		if err := json.Unmarshal([]byte(*a.Data), &content); err == nil {
			view.Content = content
		}
	}
	return view
}
