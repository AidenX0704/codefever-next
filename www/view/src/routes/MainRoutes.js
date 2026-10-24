import React from 'react'
import { Switch, Route, withRouter } from 'react-router-dom'

// 性能整改 E4（2026-10）：路由级代码分割。
//
// 整改前这里的 34 个路由组件全部是静态 import，全仓库没有 React.lazy /
// import() / Suspense，webpack 因此只能产出两个文件：main（全部应用代码
// 568 KB）+ 一个 vendor chunk（1,131 KB）。后果是**只看登录页的访客也要
// 下载后台管理、Markdown 渲染、Prism 高亮、完整日期库的全部分量**。
//
// 现在改成 React.lazy + import()，并按**业务域**分组（webpackChunkName），
// 而不是每个组件一个 chunk —— 后者会产生 34 个只有几 KB 的碎片，
// 在 HTTP/1.1 的 6 连接限制下反而更慢。
//
// 分组：
//   admin          管理后台（5 个页面，绝大部分用户永远不会访问）
//   repository    仓库浏览主链路（文件树 / 提交 / 分支 / 标签）
//   repository-cfg 仓库创建与设置
//   merge-request  合并请求（详情页单文件 42 KB 源码）
//   group          仓库组
//   settings      个人设置
//   misc          其他（用户信息弹层、动态流）
//
// 注意：本项目全部是 class 组件、没有 hook，因此这里只能用 React.lazy +
// Suspense（React 17 已支持），不能改用路由级 hook。Suspense 的 fallback
// 必须给出可见的占位（见 PageLoading），否则 chunk 到达前会白屏。
const UserInfo = React.lazy(() => import(/* webpackChunkName: "misc" */ 'APPSRC/components/view/UserInfo'))
const ActivityList = React.lazy(() => import(/* webpackChunkName: "misc" */ 'APPSRC/components/view/ActivityList'))

const UserSettingSSHKey = React.lazy(() => import(/* webpackChunkName: "settings" */ 'APPSRC/components/view/UserSettingSSHKey'))
const UserSettingEmail = React.lazy(() => import(/* webpackChunkName: "settings" */ 'APPSRC/components/view/UserSettingEmail'))
const UserSettingGeneral = React.lazy(() => import(/* webpackChunkName: "settings" */ 'APPSRC/components/view/UserSettingGeneral'))
const UserSettingNotification = React.lazy(() => import(/* webpackChunkName: "settings" */ 'APPSRC/components/view/UserSettingNotification'))

const AdminDashboard = React.lazy(() => import(/* webpackChunkName: "admin" */ 'APPSRC/components/view/admin/Dashboard'))
const AdminUsers = React.lazy(() => import(/* webpackChunkName: "admin" */ 'APPSRC/components/view/admin/Users'))
const AdminGroups = React.lazy(() => import(/* webpackChunkName: "admin" */ 'APPSRC/components/view/admin/Groups'))
const AdminRepositories = React.lazy(() => import(/* webpackChunkName: "admin" */ 'APPSRC/components/view/admin/Repositories'))
const AdminSettings = React.lazy(() => import(/* webpackChunkName: "admin" */ 'APPSRC/components/view/admin/Settings'))

const RepositoryList = React.lazy(() => import(/* webpackChunkName: "repository-cfg" */ 'APPSRC/components/view/RepositoryList'))
const NewRepository = React.lazy(() => import(/* webpackChunkName: "repository-cfg" */ 'APPSRC/components/view/NewRepository'))
const NewRepositoryFork = React.lazy(() => import(/* webpackChunkName: "repository-cfg" */ 'APPSRC/components/view/NewRepositoryFork'))
const RepositorySettingGeneral = React.lazy(() => import(/* webpackChunkName: "repository-cfg" */ 'APPSRC/components/view/RepositorySettingGeneral'))
const RepositorySettingBranch = React.lazy(() => import(/* webpackChunkName: "repository-cfg" */ 'APPSRC/components/view/RepositorySettingBranch'))
const RepositorySettingWebhook = React.lazy(() => import(/* webpackChunkName: "repository-cfg" */ 'APPSRC/components/view/RepositorySettingWebhook'))
const RepositorySettingMembers = React.lazy(() => import(/* webpackChunkName: "repository-cfg" */ 'APPSRC/components/view/RepositorySettingMembers'))
const RepositorySettingAdvanced = React.lazy(() => import(/* webpackChunkName: "repository-cfg" */ 'APPSRC/components/view/RepositorySettingAdvanced'))

const GroupList = React.lazy(() => import(/* webpackChunkName: "group" */ 'APPSRC/components/view/GroupList'))
const NewGroup = React.lazy(() => import(/* webpackChunkName: "group" */ 'APPSRC/components/view/NewGroup'))
const GroupSettingGeneral = React.lazy(() => import(/* webpackChunkName: "group" */ 'APPSRC/components/view/GroupSettingGeneral'))
const GroupSettingMembers = React.lazy(() => import(/* webpackChunkName: "group" */ 'APPSRC/components/view/GroupSettingMembers'))
const GroupSettingAdvanced = React.lazy(() => import(/* webpackChunkName: "group" */ 'APPSRC/components/view/GroupSettingAdvanced'))

const FileTree = React.lazy(() => import(/* webpackChunkName: "repository" */ 'APPSRC/components/view/FileTree'))
const CommitList = React.lazy(() => import(/* webpackChunkName: "repository" */ 'APPSRC/components/view/CommitList'))
const BranchList = React.lazy(() => import(/* webpackChunkName: "repository" */ 'APPSRC/components/view/BranchList'))
const TagList = React.lazy(() => import(/* webpackChunkName: "repository" */ 'APPSRC/components/view/TagList'))
const CreateTag = React.lazy(() => import(/* webpackChunkName: "repository" */ 'APPSRC/components/view/CreateTag'))
const CreateBranch = React.lazy(() => import(/* webpackChunkName: "repository" */ 'APPSRC/components/view/CreateBranch'))
const CommitDetail = React.lazy(() => import(/* webpackChunkName: "repository" */ 'APPSRC/components/view/CommitDetail'))

const MergeRequest = React.lazy(() => import(/* webpackChunkName: "merge-request" */ 'APPSRC/components/view/MergeRequest'))
const CreateMergeRequest = React.lazy(() => import(/* webpackChunkName: "merge-request" */ 'APPSRC/components/view/CreateMergeRequest'))
const MergeRequestDetail = React.lazy(() => import(/* webpackChunkName: "merge-request" */ 'APPSRC/components/view/MergeRequestDetail'))

// PageLoading 是懒加载 chunk 到达前的占位。
//
// 刻意不引入 MUI 的 CircularProgress：这个组件会出现在**每一个**路由切换上，
// 用纯 CSS 动画可以避免把 MUI 变成 Suspense 边界的强制依赖。
// 高度给足，避免 chunk 到达后内容插入造成的布局跳动（CLS）。
const pageLoadingStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  minHeight: '240px',
  color: '#8a94a6',
  fontSize: '14px'
}

function PageLoading () {
  return <div style={pageLoadingStyle}>加载中…</div>
}

class MainRoutes extends React.Component {
  render () {
    return <React.Suspense fallback={<PageLoading />}>
      <Switch>
        <Route path='/userInfo' component={UserInfo} />

        <Route path='/settings'>
          <Switch>
            <Route exact path='/settings/profile' component={UserSettingGeneral} />
            <Route exact path='/settings/sshkey' component={UserSettingSSHKey} />
            <Route exact path='/settings/notification' component={UserSettingNotification} />
            <Route exact path='/settings/email' component={UserSettingEmail} />
            <Route path='/settings' component={UserSettingGeneral} />
          </Switch>
        </Route>

        <Route path='/admin'>
          <Switch>
            <Route exact path='/admin/users' component={AdminUsers} />
            <Route exact path='/admin/groups' component={AdminGroups} />
            <Route exact path='/admin/repositories' component={AdminRepositories} />
            <Route exact path='/admin/settings' component={AdminSettings} />
            <Route path='/admin' component={AdminDashboard} />
          </Switch>
        </Route>

        <Route path='/mergerequests'>
          <Switch>
            <Route exact path='/mergerequests' component={MergeRequest} />
            <Route exact path='/mergerequests/new' component={CreateMergeRequest} />
          </Switch>
        </Route>

        <Route path='/repositories'>
          <Switch>
            <Route exact path='/repositories/new' component={NewRepository} />
            <Route exact path='/repositories/fork/:repositoryID([0-9a-f]+)' component={NewRepositoryFork} />
            <Route exact path='/repositories/forklist/:repositoryID([0-9a-f]+)' component={RepositoryList} />
            <Route path='/repositories' component={RepositoryList} />
          </Switch>
        </Route>
        <Route path='/groups'>
          <Switch>
            <Route path='/groups/new' component={NewGroup} />
            <Route exact path='/groups/:groupName([A-Za-z0-9_]{5,})' component={RepositoryList} />
            <Route exact path='/groups/:groupName([A-Za-z0-9_]{5,})/repositories' component={RepositoryList} />
            <Route exact path='/groups/:groupName([A-Za-z0-9_]{5,})/members' component={GroupSettingMembers} />
            <Route exact path='/groups/:groupName([A-Za-z0-9_]{5,})/activities' component={ActivityList} />
            <Route exact path='/groups/:groupName([A-Za-z0-9_]{5,})/repositories/new' component={NewRepository} />
            <Route exact path='/groups/:groupName([A-Za-z0-9_]{5,})/mergerequests' component={MergeRequest} />
            <Route exact path='/groups/:groupName([A-Za-z0-9_]{5,})/mergerequests/new' component={CreateMergeRequest} />
            <Route exact path='/groups/:groupName([A-Za-z0-9_]{5,})/settings' component={GroupSettingGeneral} />
            <Route exact path='/groups/:groupName([A-Za-z0-9_]{5,})/settings/general' component={GroupSettingGeneral} />
            <Route exact path='/groups/:groupName([A-Za-z0-9_]{5,})/settings/advanced' component={GroupSettingAdvanced} />
            <Route path='/groups' component={GroupList} />
          </Switch>
        </Route>
        <Route path='/:groupName([A-Za-z0-9_]{5,})/:repositoryName([A-Za-z0-9_]+)'>
          <Switch>
            <Route exact path='/:groupName([A-Za-z0-9_]{5,})/:repositoryName([A-Za-z0-9_]+)/files' component={FileTree} />
            <Route exact path='/:groupName([A-Za-z0-9_]{5,})/:repositoryName([A-Za-z0-9_]+)/files/:rev([\w\-\.%|]+)' component={FileTree} />
            <Route exact path='/:groupName([A-Za-z0-9_]{5,})/:repositoryName([A-Za-z0-9_]+)/:type(files|blame)/:rev([\w\-\.%|]+):path(/.*)' component={FileTree} />

            <Route exact path='/:groupName([A-Za-z0-9_]{5,})/:repositoryName([A-Za-z0-9_]+)/commit/:hash([0-9a-f]{8})' component={CommitDetail} />
            <Route exact path='/:groupName([A-Za-z0-9_]{5,})/:repositoryName([A-Za-z0-9_]+)/commits' component={CommitList} />
            <Route exact path='/:groupName([A-Za-z0-9_]{5,})/:repositoryName([A-Za-z0-9_]+)/commits/:rev([\w\-\.%|]+)' component={CommitList} />
            <Route exact path='/:groupName([A-Za-z0-9_]{5,})/:repositoryName([A-Za-z0-9_]+)/commits/:rev([\w\-\.%|]+):path(/.*)' component={CommitList} />

            <Route exact path='/:groupName([A-Za-z0-9_]{5,})/:repositoryName([A-Za-z0-9_]+)/mergerequests' component={MergeRequest} />
            <Route exact path='/:groupName([A-Za-z0-9_]{5,})/:repositoryName([A-Za-z0-9_]+)/mergerequests/new' component={CreateMergeRequest} />
            <Route exact path='/:groupName([A-Za-z0-9_]{5,})/:repositoryName([A-Za-z0-9_]+)/mergerequests/detail/:sourceRepository([0-9a-f]{32})/:sourceBranch([\w\-\.%|]+)/:targetRepository([0-9a-f]{32})/:targetBranch([\w\-\.%|]+)' component={MergeRequestDetail} />
            <Route exact path='/:groupName([A-Za-z0-9_]{5,})/:repositoryName([A-Za-z0-9_]+)/mergerequests/:mid(\d+)' component={MergeRequestDetail} />

            <Route exact path='/:groupName([A-Za-z0-9_]{5,})/:repositoryName([A-Za-z0-9_]+)/branches' component={BranchList} />
            <Route exact path='/:groupName([A-Za-z0-9_]{5,})/:repositoryName([A-Za-z0-9_]+)/branches/new' component={CreateBranch} />

            <Route exact path='/:groupName([A-Za-z0-9_]{5,})/:repositoryName([A-Za-z0-9_]+)/tags' component={TagList} />
            <Route exact path='/:groupName([A-Za-z0-9_]{5,})/:repositoryName([A-Za-z0-9_]+)/tags/new' component={CreateTag} />

            <Route exact path='/:groupName([A-Za-z0-9_]{5,})/:repositoryName([A-Za-z0-9_]+)/members' component={RepositorySettingMembers} />

            <Route exact path='/:groupName([A-Za-z0-9_]{5,})/:repositoryName([A-Za-z0-9_]+)/activities' component={ActivityList} />

            <Route exact path='/:groupName([A-Za-z0-9_]{5,})/:repositoryName([A-Za-z0-9_]+)/settings' component={RepositorySettingGeneral} />
            <Route exact path='/:groupName([A-Za-z0-9_]{5,})/:repositoryName([A-Za-z0-9_]+)/settings/general' component={RepositorySettingGeneral} />
            <Route exact path='/:groupName([A-Za-z0-9_]{5,})/:repositoryName([A-Za-z0-9_]+)/settings/branch' component={RepositorySettingBranch} />
            <Route exact path='/:groupName([A-Za-z0-9_]{5,})/:repositoryName([A-Za-z0-9_]+)/settings/webhook' component={RepositorySettingWebhook} />
            <Route exact path='/:groupName([A-Za-z0-9_]{5,})/:repositoryName([A-Za-z0-9_]+)/settings/advanced' component={RepositorySettingAdvanced} />

            <Route component={FileTree} />
          </Switch>
        </Route>
      </Switch>
    </React.Suspense>
  }
}

export default withRouter(MainRoutes)
