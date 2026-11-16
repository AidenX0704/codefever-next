# DESIGN.md（Git Platform Design System v4）落地注记

> **（2026-11-08）设计基线已切换至 `docs/design-v5/DESIGN.md`（格·代码 Cell Code v1.0，
> 同源 Cell Workbench v5.6）。** 本文件记录的 v4 落地状态（令牌命名映射、实现扩展键、
> 偏离清单）保留作为历史参考与现状事实——`web/src` 实现迁移到 v5 之前，
> theme.css 现行令牌仍是界面实现的唯一字面值来源。

> 本文件是「按 DESIGN.md 全面重构 UI」工程的落地记录。设计基线为 DESIGN.md 本体
> （2026-11 引入，取代 `docs/design-v3/DESIGN_TOKENS_V3.md`，后者保留作为历史参考、
> 不再更新）。

## 1. 令牌命名：DESIGN.md 命名已全量落地

v3 的语义令牌（`--fg-strong` / `--ac-primary` / `--st-merged` 等）已按 DESIGN.md
命名全量改名（585 处引用 + theme.css 重写）。映射表：

| v3（旧） | v4 = DESIGN.md（新） |
|---|---|
| `--bg-surface` / `--bg-sunken` / `--bg-hover` | `--bg-subtle`（灰区块） |
| `--bg-canvas` / `--bg-raised` / `--bg-overlay` | `--bg-canvas`（白画布） |
| `--bg-selected` / `--bg-selected-hover` / `--bg-active` | `--bg-muted`（选中行 / 标签底） |
| `--fg-strong` / `--fg` | `--text-primary` |
| `--fg-muted` | `--text-secondary` |
| `--fg-subtle` | `--text-tertiary` |
| `--bd-subtle` / `--bd` | `--line`（全站唯一分割线） |
| `--bd-strong` | `--line-strong`（扩展：输入框 hover / 强分割） |
| `--bd-focus` | `--brand`（输入框聚焦边框） |
| `--ac-primary` / `-hover` / `-active` / `-tint` / `-border` | `--brand` / `--brand-hover` / `--brand-active` / `--brand-subtle` / `--brand-border` |
| `--ac-danger*` | `--danger*` |
| `--st-success*` / `--st-warning*` / `--st-danger*` | `--success*` / `--warning*` / `--danger*` |
| `--st-merged*` | `--purple*` |
| `--st-info*` | 并入 `--brand*` 族（info 色与品牌色合一，DESIGN.md 不设独立 info） |
| `--chrome-*` | 并入 `--bg-*` / `--text-*`（v4 无独立壳层色） |

## 2. 实现所需扩展键（DESIGN.md 未定义）

`--text-disabled` / `--text-inverse`、`--line-strong`、`--brand-active` /
`--brand-border` / `--brand-text`、`--danger-hover` / `-active` / `-border` /
`-text`、`--success-text` / `--success-border`、`--warning-text` / `--warning-border`、
`--purple-text` / `--purple-border`、`--row-h-compact`、
`--z-overlay/--z-drawer/--z-tooltip`（DESIGN.md §3.4 只有 5 档，实现需更多层级）。

## 3. 与 DESIGN.md 的有意偏离（AGENTS §7）

| 偏离 | 原因 | 回归条件 |
|---|---|---|
| 标签文字用 `-text` 变体压 `-subtle` 底（DESIGN.md §6.3 是同色文字） | 同色组合在 warning 等色上不足 4.5:1（2026-10 曾因此出过 1.41:1 事故，check:ui 现在强制拦截） | DESIGN.md 明确接受 AA 以下对比度时 |
| `--danger` 用 #d63636（DESIGN.md 为 #D93F3F） | #D93F3F 配白色按钮文字只有 4.41:1，不达 AA | 允许大字号-only 实心危险按钮时 |
| 字体沿用系统栈，不自托管 Inter + HarmonyOS Sans（§3.2） | 离线部署零外部请求（既有决策，theme.css 注释） | 需要精确复刻西文字形且允许字体文件入库时 |
| 间距阶梯沿用 12 级（2–64px，含 2/6/20/40 微步） | 覆盖 DESIGN.md §3.3 全部锚点值（4 8 12 16 24 32 48 64），改动面小 | 宣传页需要 96px 大节距时用页面令牌（pages-landing.css） |
| 代码高亮 / diff 用 GitHub Light/Dark 官方色板字面值 | 外部来源常量（AGENTS §5.11 例外），集中在 theme.css | — |

## 4. 图标：Phosphor Icons 已落地

- 依赖 `@phosphor-icons/react@2`；`web/src/config/icons.ts` 是**白名单**（业务代码
  不得直接 import 该包），canonical 语义名（`repositoryIcon` / `branchIcon` /
  `mergeRequestIcon` …）与 v3 旧名别名（`plRepository` / `psBranch` …）并存。
- `Icon` 组件（`components/unit/Icon`）包装 Phosphor：`weight` 支持
  regular（默认）/ duotone（空态、功能介绍）/ fill（选中态星标）。
- 旧 `web/src/assets/icons/`（155 个文件）与 `scripts/extract-icons.mjs` 已删除。

## 5. DESIGN.md 之外新增的页面与组件

| 项 | 说明 |
|---|---|
| CommandPalette（⌘K） | `components/layout/CommandPalette`；分组最近访问/仓库/组/命令，`>` 前缀命令模式 |
| CloneBox | `components/unit/CloneBox`，SSH\|HTTPS 分段 + 复制 |
| Timeline | `components/unit/Timeline`，更新日志 / 时间线 |
| `/settings/appearance` | 外观（浅/深/跟随系统）与语言 |
| `/admin/announcements` | 平台公告与更新日志维护（写 `/api/admin/*`） |
| MR「讨论」Tab | 行内评论时间线（`/api/repository/mergeRequestComments`） |
| Star | 仓库二级头部 + 列表行内星标按钮（`/api/repository/star`） |
| 归档下载 | 仓库下载 zip / tar.gz（`/api/repository/archive` 二进制流） |

## 6. 本轮未做（DESIGN.md 有、本轮明确延后）

- **AI 助手**（顶栏 AI、首页 AI 提问框、AI 侧栏）：需要后端 LLM 代理与配置面。
- **/explore 公开仓库浏览**：需要仓库可见性模型（visibility 列 + 匿名可读链路）。
- **Web 端文件新建/编辑**：需要 git blob 写入链路（`gitsvc` 无此能力）。
- **未登录宣传页 `/`**：路由 `/` 目前仍 302 到 `/home`；宣传页涉及匿名顶栏变体
  与公开统计（`/api/site/stats` 已就绪），下轮落地。
- **rebase 合并方式**：`gitsvc` 只有 ff / non-ff / squash 三种合并实现。
- **流水线日志页**：平台无 CI 概念。

## 7. 后端配套端点（本轮新增）

| 端点 | 说明 |
|---|---|
| `GET /api/site/stats` | 匿名公开统计（仓库/组/成员/已合并 MR 计数） |
| `GET /api/site/changelog`、`GET /api/site/announcement` | 匿名更新日志（分页）与公告 |
| `POST /api/admin/changelogSave|Delete`、`announcementSave|Delete` | 管理端维护 |
| `POST /api/repository/star|unstar` | 星标（幂等）；list/config 响应含 `starCount`/`starred` |
| `GET/POST /api/repository/mergeRequestComments|addMergeRequestComment` | MR 讨论（评论通知创建者与评审人） |
| `POST /api/repository/mergeBranch` 新增 `deleteSourceBranch` | 合并后删源分支 |
| `repository/config` 新增 `allowMergeCommit`/`allowSquash`/`minReviewers` | 合并设置；minReviewers=0 保持「全部评审人通过」 |
| `POST /api/repository/redeliverWebhookLog` | webhook 投递重发 |
| `GET /api/repository/archive?repositoryId&ref&format=zip\|tar.gz` | 归档下载（二进制流） |
| `cc_users.u_bio` + `user/info`、`updateBasicInfo` | 个人简介 |
| `admin/config` 新增 `siteName` 与 `smtp.*` | 站点名（页头 AppName 读取）与 SMTP 界面化（密码只写不回读，env 兜底） |

迁移：`012_changelogs_announcements.sql` ~ `016_user_bio.sql`
（只进 `backend/internal/db/migrations/`，未触碰 `deploy/sql/db_structure.sql`）。
