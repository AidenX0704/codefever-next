package handlers

import (
	"context"

	"codefever-next/backend/internal/domain"
	"codefever-next/backend/internal/repo"
)

// repositoryListView 把一批"对 viewer 可见"的仓库行补齐成 repository/list
// 的元素形态（role / members / forkCount / mrCount / starCount / starred）。
//
// 从 RepositoryHandler.list 抽出来的共享逻辑：个人主页
// （GET /api/user/profile，见 user_profile.go 的 profile）也要输出
// "该用户创建的仓库"，字段口径必须与 repository/list 完全一致，
// 否则前端同一套渲染组件会对不上。批量查询策略与原实现相同：
// 每类数据一条 IN 查询，绝不做逐仓库 N+1。
func repositoryListView(
	ctx context.Context,
	repos *repo.RepositoryRepo,
	stars *repo.StarRepo,
	viewerKey string,
	list []domain.Repository,
	host, ssh string,
) ([]domain.RepositoryListItem, error) {
	if len(list) == 0 {
		return []domain.RepositoryListItem{}, nil
	}

	rKeys := make([]string, 0, len(list))
	gKeys := make([]string, 0, len(list))
	for _, r := range list {
		rKeys = append(rKeys, r.Key)
		gKeys = append(gKeys, r.GroupKey)
	}

	groups, err := repos.GroupsByKeys(ctx, gKeys)
	if err != nil {
		return nil, err
	}
	forkCounts, err := repos.ForkCounts(ctx, rKeys)
	if err != nil {
		return nil, err
	}
	mrCounts, err := repos.MergeRequestCounts(ctx, rKeys)
	if err != nil {
		return nil, err
	}
	roles, err := repos.RolesFor(ctx, rKeys, viewerKey)
	if err != nil {
		return nil, err
	}
	members, err := membersOfRepos(ctx, repos, list, groups, host, ssh)
	if err != nil {
		return nil, err
	}
	starFields := starViewOf(ctx, stars, viewerKey, rKeys)

	out := make([]domain.RepositoryListItem, 0, len(list))
	for i := range list {
		r := list[i]
		out = append(out, domain.RepositoryListItem{
			NormalizedRepository: domain.NormalizeRepository(
				&r, groups[r.GroupKey], forkCounts[r.Key], mrCounts[r.Key]),
			Role:      roles[r.Key],
			Members:   members[r.Key],
			StarCount: starFields.count[r.Key],
			Starred:   starFields.starred[r.Key],
		})
	}
	return out, nil
}

// membersOfRepos 是 RepositoryHandler.membersOf 的包级实现（参数显式化），
// 让个人主页等无 RepositoryHandler 上下文的调用点复用同一份成员装配逻辑。
func membersOfRepos(
	ctx context.Context,
	repos *repo.RepositoryRepo,
	list []domain.Repository,
	groups map[string]*domain.RepositoryGroup,
	host, ssh string,
) (map[string][]domain.NormalizedRepositoryMember, error) {
	out := map[string][]domain.NormalizedRepositoryMember{}
	if len(list) == 0 {
		return out, nil
	}

	rKeys := make([]string, 0, len(list))
	gKeys := make([]string, 0, len(list))
	uKeys := make([]string, 0, len(list)*2)
	for _, r := range list {
		rKeys = append(rKeys, r.Key)
		gKeys = append(gKeys, r.GroupKey)
		uKeys = append(uKeys, r.OwnerKey)
		if g, ok := groups[r.GroupKey]; ok && g != nil {
			uKeys = append(uKeys, g.OwnerKey)
		}
	}

	repoMembers, err := repos.MembersOf(ctx, rKeys)
	if err != nil {
		return nil, err
	}
	groupMembers, err := repos.GroupMembersOf(ctx, gKeys)
	if err != nil {
		return nil, err
	}

	for _, members := range repoMembers {
		for _, m := range members {
			uKeys = append(uKeys, m.UKey)
		}
	}
	for _, members := range groupMembers {
		for _, m := range members {
			uKeys = append(uKeys, m.UKey)
		}
	}

	users, err := repos.UsersByKeys(ctx, uKeys)
	if err != nil {
		return nil, err
	}
	emails, err := repos.CommitEmailsByUser(ctx, uKeys)
	if err != nil {
		return nil, err
	}

	for _, r := range list {
		items := []domain.NormalizedRepositoryMember{}

		// 组成员（含组创建者）。
		groupItems := []domain.NormalizedRepositoryMember{}
		for _, m := range groupMembers[r.GroupKey] {
			u, ok := users[m.UKey]
			if !ok {
				continue
			}
			groupItems = append(groupItems,
				domain.NormalizeRepositoryGroupMember(&m, u, emails[m.UKey], host, ssh))
		}
		if g, ok := groups[r.GroupKey]; ok && g != nil {
			if u, ok := users[g.OwnerKey]; ok {
				groupItems = append(groupItems,
					domain.NormalizeRepositoryGroupCreator(g, u, emails[g.OwnerKey], host, ssh))
			}
		}
		items = append(items, mergeMembers(groupItems)...)

		// 仓库成员 + 仓库创建者。
		repoItems := []domain.NormalizedRepositoryMember{}
		for _, m := range repoMembers[r.Key] {
			u, ok := users[m.UKey]
			if !ok {
				continue
			}
			repoItems = append(repoItems,
				domain.NormalizeRepositoryMember(&m, u, emails[m.UKey], host, ssh))
		}
		if u, ok := users[r.OwnerKey]; ok {
			repoItems = append(repoItems,
				domain.NormalizeRepositoryCreator(&r, u, emails[r.OwnerKey], host, ssh))
		}
		items = append(items, repoItems...)

		out[r.Key] = mergeMembers(items)
	}
	return out, nil
}

// starViewOf 是 RepositoryHandler.newStarView 的包级实现。
func starViewOf(ctx context.Context, stars *repo.StarRepo, uKey string, rKeys []string) starView {
	view := starView{count: map[string]int{}, starred: map[string]bool{}}
	if stars == nil || len(rKeys) == 0 {
		return view
	}
	if counts, err := stars.StarCounts(ctx, rKeys); err == nil {
		view.count = counts
	}
	// 标星状态查询失败只影响 starred 字段（降级为 false），不让整个列表失败。
	if starred, err := stars.StarredKeys(ctx, uKey, rKeys); err == nil {
		view.starred = starred
	}
	return view
}
