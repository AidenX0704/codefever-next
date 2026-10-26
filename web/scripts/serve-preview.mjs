/**
 * 视觉验证用静态服务器 + 最小 mock API（仅用于本地验证，不进生产）。
 *
 * 为什么需要它：
 *   1. `vite preview` 会继承 `server.proxy`，把 `/user/*`、`/api/*` 转发给 Go 后端；
 *      本机没有起后端时这些路径直接 502，无法在浏览器里看页面；
 *   2. 视觉/布局校验需要**真实数据量级**的列表（空列表看不出行高、对齐、截断问题）。
 *
 * 用法：
 *   node scripts/serve-preview.mjs            # 端口 4920，/api/* 返回 mock
 *   node scripts/serve-preview.mjs 4921       # 指定端口
 *
 * ⚠️ mock 数据只覆盖批 2 已用到的三个端点；其余 `/api/*` 返回 `{code:0, data:[]}`。
 */
import { createServer } from 'node:http'
import { readFile, stat } from 'node:fs/promises'
import { extname, join, normalize, resolve, dirname } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
// 允许用 PREVIEW_ROOT 指向任意构建目录（例如从 git 历史里取出的旧产物，
// 用于改版前后的逐页对比）。默认仍是 web/build。
const root = process.env.PREVIEW_ROOT ? resolve(process.env.PREVIEW_ROOT) : resolve(here, '../build')
const port = Number(process.argv[2] ?? 4920)

const MIME = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.png': 'image/png',
  '.webp': 'image/webp',
  '.ico': 'image/x-icon',
  '.json': 'application/json; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.woff2': 'font/woff2',
}

const NOW = Math.floor(Date.now() / 1000)

const group = {
  id: 'a'.repeat(32),
  icon: null,
  displayName: 'CodeFever 团队',
  name: 'codefever_team',
  description: '社区版维护组',
  owner: 'me'.padEnd(32, '0'),
  type: 1,
  created: NOW - 86400 * 120,
  updated: NOW - 3600,
  status: 1,
}

const otherGroup = {
  ...group,
  id: 'b'.repeat(32),
  displayName: '基础设施组',
  name: 'infra_team',
  description: '构建与部署',
}

const member = {
  id: 'me'.padEnd(32, '0'),
  icon: null,
  name: 'root',
  email: 'root@codefever.cn',
  phoneCode: null,
  phoneNumber: null,
  team: null,
  role: 5,
  notification: 1,
  mfaEnabled: false,
  admin: true,
  emails: [],
  unReadNotification: 0,
  status: true,
  host: 'http://localhost',
  ssh: 'ssh://git@localhost',
  joined: NOW - 86400 * 30,
}

const MEMBERS = [
  member,
  { ...member, id: 'm2'.padEnd(32, '0'), name: 'alice', email: 'alice@codefever.cn', role: 4 },
  { ...member, id: 'm3'.padEnd(32, '0'), name: 'bob', email: 'bob@codefever.cn', role: 3 },
]

const repoBase = {
  icon: null,
  description: '轻量级代码仓库管理平台',
  forkFrom: null,
  defaultBranch: 'main',
  status: 1,
  owner: member.id,
  forkCount: 3,
  mergeRequestCount: { open: 2, merged: 12, closed: 1 },
}

const REPOSITORIES = [
  {
    ...repoBase,
    id: 'c'.repeat(32),
    displayName: 'codefever-community',
    name: 'codefever-community',
    created: NOW - 86400 * 200,
    updated: NOW - 60 * 30,
    group,
    role: 5,
    members: MEMBERS,
  },
  {
    ...repoBase,
    id: 'd'.repeat(32),
    displayName: 'gitsvc',
    name: 'gitsvc',
    created: NOW - 86400 * 150,
    updated: NOW - 86400 * 3,
    group,
    forkCount: 1,
    role: 4,
    members: MEMBERS.slice(0, 2),
  },
  {
    ...repoBase,
    id: 'e'.repeat(32),
    displayName: 'web',
    name: 'web',
    description: '',
    created: NOW - 86400 * 30,
    updated: NOW - 86400 * 40,
    group: otherGroup,
    forkCount: 0,
    mergeRequestCount: { open: 0, merged: 0, closed: 0 },
    role: 3,
    members: MEMBERS.slice(2),
  },
]

const CURRENT_USER = {
  ...member,
  name: 'root',
  unReadNotification: 7,
  emails: [{ id: '', email: 'root@codefever.cn', isCheck: true }],
}


/* ------------------------------ 批 3：仓库浏览的 mock ------------------------------ */

const TREE_FILES = [
  { mode: '100644', type: 'blob', object: 'f'.repeat(40), name: 'README.md', url: '/README.md', commit: { sha: 'f'.repeat(40), commit: 'docs: 更新说明', detail: '', email: 'root@codefever.cn', time: NOW - 3600 } },
  { mode: '040000', type: 'tree', object: 'e'.repeat(40), name: 'backend', url: '/backend', commit: { sha: 'e'.repeat(40), commit: 'feat: 后端骨架', detail: '', email: 'root@codefever.cn', time: NOW - 7200 } },
  { mode: '100644', type: 'blob', object: 'd'.repeat(40), name: 'go.mod', url: '/go.mod', commit: { sha: 'd'.repeat(40), commit: 'chore: 依赖升级', detail: '', email: 'alice@codefever.cn', time: NOW - 86400 * 9 } },
]
const FILE_CONTENT = ['# CodeFever Next', '', '轻量级代码仓库管理平台。', '', '```bash', 'make build', '```', ''].join(String.fromCharCode(10))

const REPOSITORY_CONFIG = {
  repository: REPOSITORIES[0],
  group,
  members: MEMBERS,
  branches: [{ id: 'main', name: 'main' }, { id: 'dev', name: 'dev' }],
  tags: [{ id: 'v1.0.0', name: 'v1.0.0' }],
  count: { commit: 128, branch: 2, tag: 1, file: 42 },
}


const MOCK = {
  '/api/user/info': CURRENT_USER,
  '/api/repository/list': REPOSITORIES,
  '/api/group/list': [
    { ...group, role: 5, members: MEMBERS },
    { ...otherGroup, role: 4, members: MEMBERS.slice(0, 2) },
  ],
  '/api/user/notifications': [],
}

// 批 3：仓库浏览端点（文件内容按 path 动态返回，见下方 handler）
MOCK['/api/repository/config'] = REPOSITORY_CONFIG
MOCK['/api/repository/object'] = { type: 'commit', object: TREE_FILES }
MOCK['/api/repository/pathStack'] = [{ mode: '040000', type: 'tree', object: 'e'.repeat(40), name: 'backend' }]

/* ------------------------------ 批 3（下）：提交 / 分支 / 标签 / diff ------------------------------ */

const COMMITS = [
  { sha: 'a'.repeat(40), commit: 'feat: 支持合并请求评审', detail: '评审人可指派', email: 'root@codefever.cn', time: NOW - 600 },
  { sha: 'b'.repeat(40), commit: 'fix: 修正分页上限', detail: '', email: 'alice@codefever.cn', time: NOW - 86400 * 2 },
  { sha: 'c'.repeat(40), commit: 'docs: 增加部署说明', detail: '', email: 'bob@codefever.cn', time: NOW - 86400 * 30 },
  // 补足一页以上（每页 20 条），且每天 3 条，验证"查看更多"跨页追加与日期合并。
  ...Array.from({ length: 22 }, (_, i) => ({
    sha: String(i).padStart(2, '0').repeat(20),
    commit: `chore: 批量提交 #${i + 1}`,
    detail: '',
    email: i % 3 === 0 ? 'root@codefever.cn' : i % 3 === 1 ? 'alice@codefever.cn' : 'bob@codefever.cn',
    time: NOW - 86400 * (31 + Math.floor(i / 3)) - (i % 3) * 3600,
  })),
]

const DIFF_LINES = [
  '@@ -1,4 +1,5 @@',
  ' # CodeFever Next',
  ' ',
  '-旧的描述行',
  '+新的描述行',
  '+新增的说明行',
  ' ## 快速开始',
]

MOCK['/api/repository/commitList'] = COMMITS
MOCK['/api/repository/commitDetail'] = {
  sha: COMMITS[0].sha,
  commit: COMMITS[0].commit,
  parent: 'b'.repeat(40),
  email: COMMITS[0].email,
  time: COMMITS[0].time,
}
MOCK['/api/repository/fileChanges'] = {
  count: 2,
  detail: [
    {
      original: { name: 'README.md' },
      modified: { name: 'README.md' },
      diff: { raw: DIFF_LINES.join(String.fromCharCode(10)), line: DIFF_LINES, additions: 2, deletions: -1 },
      type: 'text',
    },
    {
      original: { name: 'docs/intro.md' },
      modified: { name: 'docs/guide.md' },
      diff: { raw: null, line: [], additions: 0, deletions: 0 },
      type: 'oversize',
    },
  ],
}
/* 批 4：个人设置 */
MOCK['/api/user/listKey'] = [
  { id: 'k1', creator: 'me', name: 'id_ed25519', hash: 'SHA256:abc123', type: 'ssh-ed25519', created: NOW - 86400 * 20, deleteFlag: false },
  { id: 'k2', creator: 'me', name: 'work-laptop', hash: 'SHA256:def456', type: 'ssh-rsa', created: NOW - 86400 * 3, deleteFlag: false },
]
MOCK['/api/user/getMFAData'] = { status: false, secret: 'JBSWY3DPEHPK3PXP' }
/* 批 4（下）：Webhook 与保护分支 */
/* 批 5：合并请求 */
const MR_LIST = [
  {
    id: 'm1',
    sourceRepository: REPOSITORIES[0].id,
    sourceBranch: 'feature/login',
    targetRepository: REPOSITORIES[0].id,
    targetBranch: 'main',
    number: 12,
    title: 'feat: 支持合并请求评审',
    status: 1,
    create: NOW - 7200,
    update: NOW - 600,
    reviewers: [],
    commit: COMMITS[0],
  },
  {
    id: 'm2',
    sourceRepository: REPOSITORIES[1].id,
    sourceBranch: 'fix/paging',
    targetRepository: REPOSITORIES[1].id,
    targetBranch: 'main',
    number: 9,
    title: 'fix: 分页上限',
    status: 2,
    create: NOW - 86400 * 5,
    update: NOW - 86400 * 4,
    reviewers: [],
    commit: COMMITS[1],
  },
]
/* 批 5 遗：活动流 */
const ACTIVITIES = [
  { id: 'a1', type: 0x0301, creator: member.id, group: group.id, repository: null, content: { name: 'codefever-community' }, time: NOW - 1800 },
  { id: 'a2', type: 0x0401, creator: MEMBERS[1].id, group: group.id, repository: REPOSITORIES[0].id, content: { name: 'main' }, time: NOW - 3600 },
  { id: 'a3', type: 0x0701, creator: MEMBERS[2].id, group: group.id, repository: REPOSITORIES[0].id, content: { id: 12 }, time: NOW - 7200 },
  { id: 'a4', type: 0x0306, creator: member.id, group: group.id, repository: REPOSITORIES[0].id, content: { uid: MEMBERS[1].id }, time: NOW - 86400 },
]
MOCK['/api/repository/activities'] = ACTIVITIES
MOCK['/api/group/activities'] = ACTIVITIES

/* 批 6（下）：文档中心与管理后台配置 */
MOCK['/api/doc/tree'] = {
  menu: [
    { name: '快速开始', path: 'quickstart/intro.md' },
    { name: '功能说明', path: 'feature/intro.md' },
  ],
}
MOCK['/api/doc/content'] = { path: 'quickstart/intro.md', content: ['# 快速开始','','本页来自 mock 服务。','','| 项 | 值 |','| --- | --- |','| 引擎 | Git |',''].join(String.fromCharCode(10)) }
MOCK['/api/admin/config'] = {
  site: { name: 'CodeFever Next', host: 'http://localhost', allowRegister: 1 },
  mail: { from: 'noreply@example.com' },
}
/* 批 6：管理后台 */
const ADMIN_USERS = [
  { id: 'u1', email: 'root@codefever.cn', name: 'root', avatar: null, mfaEnabled: false, created: '2026-09-01 10:00:00', status: true, admin: true, projects: 3 },
  { id: 'u2', email: 'alice@codefever.cn', name: 'alice', avatar: null, mfaEnabled: true, created: '2026-09-12 12:00:00', status: true, admin: false, projects: 2 },
  { id: 'u3', email: 'bob@codefever.cn', name: 'bob', avatar: null, mfaEnabled: false, created: '2026-10-02 08:30:00', status: false, admin: false, projects: 0 },
]
MOCK['/api/admin/userList'] = { count: ADMIN_USERS.length, list: ADMIN_USERS }
MOCK['/api/admin/groupList'] = { count: 2, list: [group, otherGroup] }
MOCK['/api/admin/repositoryList'] = { count: REPOSITORIES.length, list: REPOSITORIES }
MOCK['/api/admin/systemStatus'] = {
  usage: { user: ADMIN_USERS.length, repository: REPOSITORIES.length, group: 2 },
  vm: { cpu: 12, memory: 37, disk: [{ name: '/dev/sda1', usage: 41 }] },
  service: { nginx: true, codefever: true, sendmail: false },
}
MOCK['/api/user/notifications'] = [
  { id: 'n1', type: 1, group: null, repository: REPOSITORIES[0].id, data: JSON.stringify({ message: 'alice 在 codefever-community 提交了新的合并请求' }), created: NOW - 300, isRead: false },
  { id: 'n2', type: 2, group: group.id, repository: null, data: JSON.stringify({ message: '系统通知：本周仓库备份已完成' }), created: NOW - 86400, isRead: true },
]
MOCK['/api/repository/mergeRequests'] = MR_LIST
MOCK['/api/repository/relatedMergeRequests'] = MR_LIST
MOCK['/api/group/mergeRequests'] = MR_LIST
MOCK['/api/repository/mergeRequestDetail'] = {
  id: 'm1',
  user: member.id,
  sourceRepository: REPOSITORIES[0].id,
  sourceBranch: 'feature/login',
  sourceObject: COMMITS[0].sha,
  targetRepository: REPOSITORIES[0].id,
  targetBranch: 'main',
  targetObject: COMMITS[1].sha,
  submitter: CURRENT_USER,
  sourceRepoInfo: { ...REPOSITORIES[0], members: MEMBERS },
  title: 'feat: 支持合并请求评审',
  description: '本 MR 增加评审人指派与评审状态。',
  number: 12,
  handler: null,
  status: 1,
  create: NOW - 7200,
  update: NOW - 600,
  close: 0,
  merge: 0,
  reviewers: [{ id: MEMBERS[1].id, name: 'alice', email: 'alice@codefever.cn', icon: null }],
  email: 'root@codefever.cn',
}

/* 批 4（尾）：组配置 */
MOCK['/api/group/config'] = { group, members: MEMBERS, count: { member: MEMBERS.length } }

MOCK['/api/repository/webhooks'] = [
  {
    id: 'w1',
    user: 'root',
    url: 'https://example.com/hooks/codefever',
    secret: '',
    events: 'hook:postReceive,mergeRequest:merge',
    active: 1,
    updated: NOW - 3600,
  },
]
MOCK['/api/repository/getRepositoryWebhookLogs'] = [
  { id: 'l1', webhook: 'w1', start: NOW - 600, end: NOW - 599, status: 200, success: true, created: '2026-10-24 17:00:00' },
]
MOCK['/api/repository/protectedBranchRules'] = [
  { id: 'p1', rule: 'main', rolePush: 4, roleMerge: 4 },
  { id: 'p2', rule: 'release/*', rolePush: 5, roleMerge: 5 },
]
MOCK['/api/user/notificationRefused'] = [
  { id: 'r1', type: 2, target: REPOSITORIES[1].id },
  { id: 'g1', type: 1, target: otherGroup.id },
]

MOCK['/api/repository/branchList'] = [
  { id: 'main', name: 'main', commit: COMMITS[0], isMerge: false, protected: true },
  { id: 'dev', name: 'dev', commit: COMMITS[1], isMerge: false, protected: false },
]
MOCK['/api/repository/tagList'] = [
  { id: 'v1.0.0', name: 'v1.0.0', detail: '首个正式版本', commit: COMMITS[2] },
  { id: 'v0.9.0', name: 'v0.9.0', commit: COMMITS[2] },
]

async function tryFile(path) {
  try {
    const info = await stat(path)
    return info.isFile() ? path : null
  } catch {
    return null
  }
}

function sendJSON(res, payload, status = 200) {
  res.writeHead(status, { 'Content-Type': MIME['.json'], 'Cache-Control': 'no-store' })
  res.end(JSON.stringify(payload))
}

createServer(async (req, res) => {
  const urlPath = decodeURIComponent((req.url ?? '/').split('?')[0])

  if (urlPath.startsWith('/api/')) {
    // 提交列表：GET + query 参数，真正按 page/pagesize/keyword 切片，
    // 才能验证"查看更多"的追加与日期合并。
    if (urlPath.startsWith('/api/repository/commitList')) {
      const query = new URL(req.url ?? '/', 'http://127.0.0.1').searchParams
      const page = Math.max(1, Number(query.get('page')) || 1)
      const pagesize = Math.min(Math.max(Number(query.get('pagesize')) || 20, 1), 100)
      const keyword = query.get('keyword') ?? ''
      const revision = query.get('revision') ?? ''
      let list = COMMITS
      if (keyword) {
        list = list.filter((commit) => commit.commit.toLowerCase().includes(keyword.toLowerCase()))
      }
      // revision 只有形如短 SHA 前缀时才过滤（分支/标签名直接返回全部）。
      if (/^[0-9a-f]{6,40}$/i.test(revision)) {
        list = list.filter((commit) => commit.sha.startsWith(revision))
      }
      const start = (page - 1) * pagesize
      sendJSON(res, { code: 0, message: 'ok', data: list.slice(start, start + pagesize) })
      return
    }
    // 文件内容：/api/repository/object?...&path=README.md → blob
    if (urlPath.startsWith('/api/repository/object')) {
      const query = new URL(req.url ?? '/', 'http://127.0.0.1').searchParams
      const filePath = query.get('path') ?? ''
      if (filePath) {
        sendJSON(res, {
          code: 0,
          message: 'ok',
          data: { type: 'blob', object: { mime: 'text/markdown', size: FILE_CONTENT.length, raw: FILE_CONTENT } },
        })
        return
      }
      // 与真实后端一致：object 响应总是带回当前路径的最后一次提交（FileTree 顶部卡片靠它渲染）。
      sendJSON(res, { code: 0, message: 'ok', data: { type: 'commit', object: TREE_FILES, commit: COMMITS[0] } })
      return
    }
    const key = Object.keys(MOCK).find((path) => urlPath.startsWith(path))
    // 与真实后端一致的信封：{code, message, data}
    sendJSON(res, { code: 0, message: 'ok', data: key ? MOCK[key] : [] })
    return
  }

  const safe = normalize(urlPath).replace(/^(\.\.[/\\])+/, '')
  const filePath = await tryFile(join(root, safe))

  if (!filePath) {
    const index = await readFile(join(root, 'index.html'))
    res.writeHead(200, { 'Content-Type': MIME['.html'], 'Cache-Control': 'no-cache' })
    res.end(index)
    return
  }

  const body = await readFile(filePath)
  res.writeHead(200, {
    'Content-Type': MIME[extname(filePath)] ?? 'application/octet-stream',
    'Cache-Control': 'no-cache',
  })
  res.end(body)
}).listen(port, '127.0.0.1', () => {
  console.log(`preview server: http://127.0.0.1:${port}  (root=${root}, mock API: ${Object.keys(MOCK).join(', ')})`)
})
