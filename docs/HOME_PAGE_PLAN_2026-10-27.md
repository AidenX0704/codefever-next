# 首页（`/home`）方案与项目完善清单

> 日期：2026-10-27 ｜ 状态：**方案（未实施）** ｜ 读者：产品/前端/后端
>
> 本文回答两件事：
> 1. **这个项目现在还有什么需要完善的**（第一部分，逐条带证据）；
> 2. **新增 `/home` 员工首页怎么做**（第二部分，从信息架构到文件级改造清单）。
>
> 全文事实以仓库当前 `main`（`c574262`）为准。凡是"现状"，都能在括号里的文件找到；
> 凡是我判断的，都标了 `[判断]`，不冒充事实。

---

## 0. 一页结论

| # | 结论 |
|---|---|
| 1 | **后端能力已经不弱**：仓库/分支/标签/合并请求/Webhook/权限/事件流/公开 API 全部落地，123 条 PHP 契约基线锁定。**缺的不是"能力"，而是"入口"** —— 最典型的是**公开 API 令牌已实现但 Web 端没有管理界面**，等于死功能。 |
| 2 | **最该补的三件事**：① 代码评审的**行级评论**（内部平台的核心，现在只能"通过/拒绝"）；② **全局搜索**（页头连搜索框都没有）；③ **企业 SSO（OIDC / LDAP）**（公司内接入的硬门槛）。 |
| 3 | 当前 `/` 直接 302 到 `/repositories`，`home/index` 也只是重定向 —— **登录后第一屏是"仓库列表"，其次是列表，再次还是列表**。员工看不到"我该做什么"，也看不到"平台能做什么"。 |
| 4 | `/home` 定位为**双层结构**：上半屏「个人工作台」（待我评审 / 我发起的 / 最近访问 / 我的仓库），下半屏「平台门户」（平台数据、能力矩阵、使用规范、四步入門、帮助入口）。首屏 3 秒回答"我要做什么"，滚动 3 屏回答"这个平台能给我什么"。**形态是列表优先**：首屏 = 一行 KPI 条 + 两张待办表 + 右窄栏，卡片只出现在下半屏门户区（依据 §2.2 的行业结论，见取舍 D9 / D10）。 |
| 5 | 数据以**一个聚合端点** `GET /api/home/overview` 承载（5 条带索引的 SQL：待办列表 ×2、KPI 总数 ×1、动态 ×2，平台概览 1 条合成查询），首屏总请求数 4（含 AppData 已有的 3 个），新增迁移 `011_perf_index_home.sql`（4 个索引）。工作量约 **3 个逻辑单元 / 3 天**，可分批上线。 |

---

# 第一部分：项目现状体检

## 1.1 已经做扎实的部分（不要再重复建设）

| 维度 | 现状 |
|---|---|
| 后端 | 单一 Go module，5 个二进制（api / worker / hook / http-gateway / ssh-gateway），PHP 已彻底移除 |
| 契约 | 123 条 PHP 兼容契约基线（`backend/internal/contract` + `testdata/contract*`）+ 40 个测试文件 |
| 数据访问 | 事务封装、批量查询（消除 N+1）、保留期清理、10 个版本化迁移、`MaxOpenConns=25` 已明确为真实瓶颈 |
| 事件 | 与业务写同事务入队 + `FOR UPDATE SKIP LOCKED` 领取 + 租约回收 → 通知/活动流/Webhook 三路分发 |
| 前端 | React 19 + Vite + **自研令牌与组件库**（28 个 unit 组件），零 UI 框架，路由级 lazy，`build/` 入库 |
| 门禁 | `scripts/verify.ps1`（= CI）+ 前端 `typecheck && lint && build` + `backend/cmd/stress` 性能门禁 |
| 对外 | `/api/v1` 公开 REST API（独立 mux、令牌鉴权、SHA-256 存储、越权 404）+ 文档中心 + 帮助中心 |

结论：**基础设施不用再造**。下面的清单全部是"在现有底座上补口子"，不是重构。

---

## 1.2 需要完善的清单

优先级定义：**P0 = 影响"能不能被公司正式用起来"或"已有功能是死的"**；**P1 = 内部平台的基本体验**；**P2 = 加分项**。

### A. 产品能力（对照阿里 Codeup / 腾讯 CODING / 小米 GitLab 的基线）

> **行业佐证**：百度 iCode 把 **CR 机制（Gerrit 风格）、代码搜索、操作审计** 直接列为核心能力；
> 腾讯工蜂有 **OWNERS、划词评审**；云效 Codeup 首页就有「代码检测」。→ A1 / A2 与 C3 不是加分项，
> 是同类平台的**门槛级能力**，这也解释了为什么它们排在 P0/P1 的最前面。

| # | 缺口 | 证据（现状） | 建议 | 成本 | 优先级 |
|---|---|---|---|---|---|
| A1 | **无行级代码评审评论** | `db_structure.sql` 只有 `cc_merge_requests`（标题/描述）+ `cc_merge_request_reviewers`（是否已评审），**没有任何评论表**；MR 只能"通过/合并/关闭" | 新增 `cc_merge_request_comments`（`mr_key` / `u_key` / `file_path` / `line` / `side` / `content` / `parent_key` 回复）+ `/api/repository/mrComment*` 三个动作 + DiffViewer 行内评论挂载 | 3–4 天 | **P0** |
| A2 🚧 | **无全局搜索**（**阶段一已完成 2026-10-27**：`/search` + `/api/search/all`，仅元数据；代码内容搜索仍待做） | 无 `/search` 路由；`web/src` 里 `search` 只出现在列表页筛选框；后端 `repo` 层无跨域搜索 | 阶段一：仓库/组名 + 合并请求标题（纯 SQL，`LIKE` + 索引前缀）；阶段二：文件内容（`git grep` 走 `gitsvc`，需限流与超时） | 1 天 / 3 天 | **P0**（阶段一） |
| A3 ✅ | **公开 API 令牌无管理界面**（**已完成 2026-10-27**：`/settings/tokens` + 三个内部端点） | `PROJECT_MAP.md` §8 2026-10-25 条目明确"**令牌管理 UI 与内部端点尚未实现**"；`cc_access_tokens` 表已建、`/api/v1` 已能鉴权 | 个人设置新增 `settings/tokens` 页：列表（前缀/创建时间/最后使用）+ 创建（明文仅返回一次）+ 吊销 | 0.5 天 | **P0** |
| A4 | 无仓库回收站 | `SettingsAdvanced` 删除仓库是两级确认后**直接删除**，无恢复入口（Codeup 有回收站） | `cc_repositories.r_deleted` 已有软删列的话改软删 + 后台"回收站"页 + 30 天保留任务 | 2 天 | P1 |
| A5 | 无 Release / Tag 发布说明 | 有 `TagList`，无 release 说明/附件 | 新表 `cc_releases` + 标签页右侧"发布说明" | 2 天 | P1 |
| A6 | 无 Issue / 需求关联 | 无 issues 表 | 与外部 Jira/需求平台对接（MR 描述里贴链接）比自建更划算 | 1 天（对接） | P1 |
| A7 | 无收藏/置顶、无最近访问 | 仓库列表只有"全部/我参与的/我创建的"三页签（`RepositoryList.tsx`） | 收藏落库 `cc_repository_stars`；最近访问先用 localStorage（首页需要，见 §2.1） | 0.5 天 / 0.5 天 | P1 |
| A8 | 无代码检测/扫描 | 无相关表与端点 | 可先做"Webhook 触发外部 CI"的文档化，不做引擎 | — | P2 |
| A9 | 无 Git LFS | `backend` 内无 lfs 关键字 | 大文件仓库会直接痛苦，但优先级低 | 3 天 | P2 |
| A10 | 无用户主页 | 路由无 `/{用户名}`，成员只能跳仓库 | 公开资料页（头像/组/仓库/近期活动） | 1 天 | P2 |

### B. 工程与质量

| # | 缺口 | 证据 | 建议 | 优先级 |
|---|---|---|---|---|
| B1 | **前端零单测** | `web/package.json` 无任何测试框架（devDependencies 只有 eslint / tsc / vite）；视觉回归靠 `scripts/shot.mjs` 手工截图 | 引入 vitest + Testing Library，先覆盖 `helpers/`（slug、format、members、prism LRU）与 3 个高频组件（DataTable / Dropdown / FilterBar） | **P0** |
| B2 | **缺 3 个基础表单件** | `PROJECT_MAP.md` §5.1 明确"`TextareaField` / `Checkbox` / `Switch` **尚未实现**（此前清单误列）" | A1（评论）、A5（Release）落地前必须先补，否则又要页面自绘 | **P0**（A1/A5 前置） |
| B3 | 无 E2E | 无 playwright | 登录 → 建仓 → push → MR → 合并 一条主链路 | P1 |
| B4 | `AppDataProvider` 无条件拉全量 | `features/app-data/AppDataProvider.tsx` 每次进入 AppShell 都 `fetchRepositoryList()` + `fetchGroupList()`（页头菜单需要全量） | `repository/list` 增加 `limit` / `keyword`；页头菜单只取前 N，列表页自行分页 | P1 |
| B5 | 无前端错误采集 | 只有 `/api/web-vitals`（`handlers/webvitals.go`） | `window.onerror` / `unhandledrejection` 上报到同一端点 | P2 |
| B6 | 无暗色模式 | `theme.css` 只有 `:root`，无 `prefers-color-scheme` | 令牌已集中，加一套 `[data-theme="dark"]` 是低成本高观感项 | P2 |
| B7 | Dependabot 未配置 | CHANGELOG 已记录"仍未配置" | 开 `dependabot.yml`（gomod + npm） | P2 |
| B8 | 页面外壳未下沉 | 11 个 `styles/pages-*.css`，各页自己写标题/间距（违反 AGENTS §6 第 2 条"布局脚手架必须下沉"） | 抽 `components/unit/PageShell`（标题 + 描述 + 右侧操作 + 内容槽），**首页落地前先抽**，否则首页会再抄一遍 | P1（首页前置） |

### C. 安全与合规（决定"公司敢不敢用"）

| # | 缺口 | 证据 | 建议 | 优先级 |
|---|---|---|---|---|
| C1 | **无 SSO / LDAP / OIDC** | 全仓库 grep `oauth / ldap / saml / oidc` 无命中；登录只有口令 + MFA | 优先 OIDC（对齐公司 IdP），其次 LDAP bind；保留口令登录作后路 | **P0** |
| C2 | 令牌无 scope | `cc_access_tokens.at_scope` 列已预留但"第一版不实现 —— 令牌 ≡ 用户"（`PROJECT_MAP.md` §6） | 加 `read` / `write` / `repo:<key>` 三种 scope，公开 API 侧按 scope 拒绝 | P1 |
| C3 | 后台操作无审计 | `domain/event_types.go` 42 个事件类型里**没有 user/admin 家族**；后台重置口令、改系统配置、加管理员都不可追溯 | 新增 `admin:*` 事件 + 后台"审计日志"页（读 `cc_activities` + `cc_events`） | P1 |
| C4 | MFA 无强制策略 | 用户可自行开启（`settings/Profile`），管理员无法强制 | 站点配置加 `security.requireMFAForAdmin`，登录时校验 | P1 |
| C5 | 无推送规则/提交校验 | `pre-receive` 只有权限校验 | 提交信息规范（Conventional Commits）、禁止大文件、禁止密钥（正则可配） | P2 |

### D. 建议排期

```
冲刺 1（≈1 周，P0 清账）
  A3 令牌管理 UI（0.5d，已有后端，纯前端）
  B2 三个表单件（0.5d）
  B1 vitest + helpers 覆盖（1d）
  B8 PageShell 抽取（0.5d）
  → 首页 H0 批（0.5d，见 §2.8）
冲刺 2（≈2 周，内部平台体验）
  H1 + H2 首页主体（2d）
  A1 行级评论（4d）
  A2 搜索阶段一（1d）
冲刺 3（企业接入）
  C1 OIDC（3d）
  C3 审计（2d）
  A4 回收站（2d）
```

---

# 第二部分：`/home` 首页方案

## 2.1 目标、验收标准、边界

### 定位

`/home` = **公司 Git 平台的门面 + 员工的每日入口**。它同时承担两个角色：

1. **工作台**（上半屏，个人化）：我今天要评审什么、我发起的 MR 到哪了、我最近在哪个仓库。
2. **门户**（下半屏，全员一致）：这个平台有哪些能力、我们团队的规范是什么、新人怎么上手、有问题找谁。

### 三个"3 秒问题"

| 问题 | 由谁回答 | 位置 |
|---|---|---|
| 我现在要做什么？ | KPI（待我评审 / 我发起的）+ 两个 MR 列表 | 首屏左上 |
| 我最近在做什么？ | 最近访问 + 我的仓库 | 首屏左中 / 右下 |
| 这个平台能做什么、怎么用？ | 能力矩阵 + 四步入門 + 规范三卡 + 帮助入口 | 第二屏 |

### 验收标准（可测）

- [ ] 登录后 `/` 落到 `/home`（302 或前端路由重定向，二选一，见 §2.6.5）
- [ ] 首屏（1440×900，无滚动）能看到：问候 + 4 个 KPI + 待我评审（≥3 条）+ 最近访问
- [ ] 首屏网络请求 ≤ 4 个（`user/info`、`repository/list`、`group/list`、`home/overview`），聚合端点 p95 < 120ms（本地压测 `cmd/stress`）
- [ ] 四个状态齐备：加载骨架 / 空态引导 / 错误重试 / 无权限降级
- [ ] 深色无关：全部视觉值取自 `--cf-*`（0 个字面值）
- [ ] `npm run typecheck && lint && build` 全绿；`web/build` 已重建入库

### 明确**不做**的（避免首页变成第二个列表页 / 一个卡片墙）

- 不做仓库全量列表（那是 `/repositories` 的职责）
- **不做卡片墙**（KPI 不做四个方块卡、我的仓库不用卡片网格；理由见 §2.2 风格结论与 D9）
- 不做轮播大图 / 宣传视频（与 DeepSeek 的极简取向相悖，也拖慢首屏）
- 不做角色化模板 / 可拖拽自定义布局（云效 16 卡片、CODING 3 套模板都有；我们需要配置中心，成本 3 倍于收益，见 D8）
- 不展示需要实时 `git` 调用的指标（提交数/分支数/语言分布要调 git，首页只放**库内已有的字段**，见 D2）

---

## 2.2 对标拆解：我们参考了什么，丢掉了什么

> 事实来源见 `docs/RESEARCH_HOME_DASHBOARD_2026-10-27.md`（逐条带 URL 的调研原文）。
> **凡标注"无公开资料"的，就是真的没有**，不用推测冒充事实。

| 平台 | 首页/工作台首屏结构 | 我们取 | 我们不取 |
|---|---|---|---|
| **阿里 云效 Codeup**（对外，有官方文档） | **首屏就是代码库列表**：顶部搜索 + 导入 + 新建，Tab「我参与的 / 全部」，每行显示提交数 / 分支数 / 标签数 / 语言分布，「我参与的」按更新时间倒序、**收藏置顶**；导航栏另有代码组 / 合并请求（可按评审人、发起人、WIP-评审中筛选）/ 代码检测 / 回收站 | 仓库列表的**信息密度**、「我参与的 / 全部」页签心智、收藏置顶（P1）、MR 可按评审人筛选 | 首屏直接给**全量**仓库列表（我们要先给"待办"）；提交数与语言分布（要逐库调 git） |
| **阿里 云效 个人工作台** | 首次登录**先选角色**（研发 / 负责人 / 产品 / 项目 / 测试）→ 生成模板布局 → 可自选 **16 张卡片**（工作项、迭代、工作小结、合并请求、流水线、度量…） | "首屏可角色化"这个**方向**（我们用"管理员多看到一块平台概览"表达，见 M1/M9） | 16 卡片自选布局（要配置中心 + 每用户布局持久化，成本 3× 收益，见 §2.9 D8） |
| **腾讯 CODING 工作台** | **登录团队首页默认进入个人工作台**，三套角色模板；研发模板 = 我的事项 / **我的合并请求（我发起的 / 我评审的）** / 我的构建计划 / 我的待确认发布；另有自定义菜单与常用链接 | 「我发起的 / 我评审的」**双栏拆分语义**（直接照抄）；"常用链接"→ 我们的「快捷入口」 | 构建记录 / 发布单（我们没有 CI 引擎）；角色模板 |
| **腾讯 CODING 的首屏本质**（调研原话） | **"未完成事项表 + 待我评审的 MR 表"，不是卡片墙** | **这一条直接改掉了本方案的第一版**：首屏改为"紧凑 KPI 条 + 两张待办表"，卡片墙只留在下半屏门户区（见 §2.3 与 §2.9 D9） | — |
| **腾讯 工蜂 TGit** | 主页导航「项目 / 项目组」+ 顶部搜索 + 「+创建项目」；**「待办」集中所有与你相关的待办与消息**；项目内左侧导航 + 仪表板 + 里程碑；关注 / 标星；OWNERS、划词评审 | 「待办」作为一等公民（= 我们的 M2/M3）；组→仓库前缀呈现 | 多层级组织（最多 21 层，我们只有 group/repo，不自造） |
| **小米**（无首页 IA 公开资料） | 公开信息只到**工具链构成**：GitLab + Phabricator + Jenkins on K8s + Nexus，自研"研发效能平台"的定位是**标准化整合各工具、一键接入**（第三方研效案例研究）；另有厂商案例页称其用极狐 GitLab 私有化 + 多产品线并行 | **"平台是整合层/统一入口"这个定位** —— 所以首页要有"门户"这一层，而不是只做仓库列表 | 任何"小米内部首页长什么样"的具体设计（**无公开资料，不编造**） |
| **DeepSeek**（无公开资料） | 内部 Git 平台**明确无公开资料**。可循证的只有对外工程取向：仓库命名/分层统一、全英文 README、几乎无装饰 | 无装饰图形；命令与 hash 用等宽字体直给；**空态给"下一步动作"而不是插画**；规范直接写"分支怎么起名、commit 怎么写" | 任何"AI 风"渐变、发光、插图；以及任何声称"参考了 DeepSeek 内部首页"的说法（那是编的） |
| **GitHub Dashboard** | 旧版：Recent activity（近两周最多 4 条）/ Top repositories（一年未交互自动掉出）/ Feed；**新版 Home：AI 入口（Copilot / Agent sessions）占首屏第一位**，其后才是 PR 与 Issue | 「最近 + 置顶 + 动态」骨架；"Top repositories 自动生成、长期不动就掉出"→ 我们 localStorage 最近访问照此衰减 | Feed（我们没有 follow 关系）；AI 首屏（本平台无 AI 会话能力） |
| **GitLab「Home」** | 默认落地页；右侧 MR 卡（我发起 / 指派给我 / **待我评审，含 Review requested 页签**）；「Items that need your attention」待处理清单（可按 Failed builds 过滤）；Quick access（Recently viewed / Projects） | 待办优先排序；**"待我评审"独立成卡**（而不是塞进总 MR 列表） | Failed builds 过滤（无 CI）；左侧再挂一条导航（我们已有页头 + Drawer） |
| **华为 CodeArts** | 工作台页签 = 我的工作项 / 我的文件 / 我的 Wiki / 我的测试；Repo 列表按「**我关注的 / 我参与的 / 我创建的**」三维度 | 仓库页签补第 4 维「**我收藏的**」（P1，配合 A7） | Wiki / 测试模块（我们没有） |
| **百度 iCode** | 公开只有 CR 机制（Gerrit 风格）、**代码搜索**、**操作审计**、iScan/iPipe 打通；首页 IA 无公开资料 | 作为 A1 / A2 / C3 三条缺口的**行业佐证** | 首页参考（无资料） |

**风格结论（对我们的直接约束）**：调研原文的结论是——**对内平台几乎全部"列表优先于卡片"，首屏密度优先；仪表盘/卡片只出现在"项目管理"语境**（CODING 三套模板、云效 16 卡片）；色彩克制，单主色 + 语义色，无插画与运营 banner；GitHub 新版把 AI 入口放首屏是唯一显著的卡片化例外。

因此本方案**第一版（卡片栅格首屏）被修正为**：

- 首屏 = **一行紧凑 KPI 条（无卡片外框，用分隔线）+ 两张待办表（待我评审 / 我发起的）+ 右窄栏（最近访问 / 公告）**；
- 「我的仓库」用**表格**（`unit/Table`），不是卡片网格 —— 与全站表格密度（`--cf-table-row-height: 40px`）一致；
- 卡片只用于**下半屏门户区**，且每张卡内部仍是列表或表格；
- 唯一"有色块"仍是 HERO 底（`--cf-color-primary-lighter`），不引入第二处。

**一句话结论**：阿里给"列表的信息密度"，腾讯给"我发起的 / 我评审的 + 列表优先"，小米给"平台是整合层的门户定位"，DeepSeek 给"极简与规范前置"，GitHub/GitLab 给"待办 + 最近 + 动态"的骨架。我们的首页 = 这五者的并集，排序为「待办 > 最近 > 我的仓库 > 概览 > 介绍」。

---

## 2.3 信息架构（线框，**列表优先**版）

> 这一版是按 §2.2「风格结论」修正后的版本：首屏是**两张待办表 + 一条紧凑 KPI 条**，
> 不是卡片墙。卡片只出现在下半屏门户区，且卡内仍是列表/表格。

```
┌──────────────────────────────────────────────────────────────────────────┐
│ Header（现有）：首页 · 仓库▾ · 仓库组▾ · 合并请求    ＋ · 语言 · 帮助 · 头像 │
└──────────────────────────────────────────────────────────────────────────┘
┌──────────────────────────────────────────────┬───────────────────────────┐
│ HERO（通栏，--cf-color-primary-lighter 底，      │                           │
│  高度 128px，不放图）                            │                           │
│  下午好，张三 · 这里是 XX 公司代码平台            │                           │
│  [＋ 新建仓库] [新建仓库组] [发起合并请求]        │                           │
│  git clone git@git.corp:team/repo.git  [复制]  │                           │
├──────────────────────────────────────────────┴───────────────────────────┤
│  KPI 条（无卡片外框，四项用竖分隔线，数字 20px/700 + 标签 12px）              │
│   我的仓库 12  │  我参与的组 3  │  待我评审 4  │  我发起的 2                │
├────────────────────────────────────────────────┬─────────────────────────┤
│ ▌待我评审                              (4) 全部 →│ ▌最近访问（≤6 行，紧凑）  │
│ ┌────────────────────────────────────────────┐│  ▢ 支付网关/order-svc 2h  │
│ │仓库        标题          分支     发起人 更新││  ▢ 基础架构/dns-proxy 昨天│
│ │支付网关/… !128 …          feat/x→main 李四 3h││  ▢ …（localStorage）      │
│ │…（≤5 行，复用 MergeRequestRow）              ││                          │
│ └────────────────────────────────────────────┘│──────────────────────────│
│ ▌我发起的合并请求                             全部 →│ ▌平台公告                │
│ ┌────────────────────────────────────────────┐│  · 维护窗口 …             │
│ │…（≤5 行，行尾显示"待合并 N 天"）              ││  · 规范更新 …             │
│ └────────────────────────────────────────────┘│  · 新能力上线 …           │
├────────────────────────────────────────────────┴─────────────────────────┤
│ ▌我的仓库  [最近更新|我创建的|我参与的|我收藏的(P1)]          查看全部 →     │
│ ┌──────────────────────────────────────────────────────────────────────┐ │
│ │仓库            描述            待合并 成员 更新于      操作              │ │
│ │▢ 支付网关/order-svc  订单服务    3   ▲4  2 小时前   [复制地址]           │ │
│ │…（unit/Table，≤8 行，与全站表格密度一致：行高 40px）                     │ │
│ └──────────────────────────────────────────────────────────────────────┘ │
├──────────────────────────────────────────────────────────────────────────┤
│ ▌团队动态（10 条，复用 ActivityFeed；默认折叠为 5 条「展开更多」）           │
├──────────────────────────────────────────────────────────────────────────┤
│ 平台门户区（全员一致，「了解平台」—— 卡片只在这里出现）                       │
│ ┌────────────────────────┐ ┌────────────────────┐ ┌───────────────────┐ │
│ │ ▌平台概览                │ │ ▌四步入門           │ │ ▌团队规范           │ │
│ │ 仓库 128 · 组 14 · 成员 96│ │ 1 配 SSH 密钥       │ │ 分支命名 / Commit   │ │
│ │ 本月新建 9 · 待合并 21    │ │ 2 clone 第一个仓库  │ │ 评审 / 保护分支     │ │
│ │ （管理员多两行）          │ │ 3 开第一个合并请求  │ │ 全部 → /doc/cn      │ │
│ │                          │ │ 4 配 Webhook / CI  │ │                    │ │
│ └────────────────────────┘ └────────────────────┘ └───────────────────┘ │
│ ┌──────────────────────────────────────────────────────────────────────┐ │
│ │ ▌平台能力（10 项，两列列表；图标 + 名称 + 一行说明，点击进对应页/文档）    │ │
│ │ 代码托管 · 分支/标签 · 合并请求评审 · 分支保护 · 成员与角色               │ │
│ │ Webhook · 公开 API · SSH/HTTP 双协议 · 通知与活动流 · 文档中心            │ │
│ │ ── 常驻链接行（M7 快捷入口）：SSH 密钥 · 访问令牌 · 文档中心 · 公开 API   │ │
│ │    · Webhook 指南 · 帮助中心                                          │ │
│ └──────────────────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────────────┘
```

栅格：`1280px` 容器，**12 列**，主列 8 / 侧列 4，间距 `space(3)` = 24px；`<1080px` 降为单列（侧列下沉到主列之后）；`<768px` 表格改为两列信息（仓库 + 更新时间），其余字段进"更多"。

**为什么右栏窄、左栏宽**：右栏两块的共同特征是"短、可扫、不需要横向对比"（最近访问 6 行、公告 3 行），放窄栏刚好；待办与仓库列表需要横向对齐（仓库 / 标题 / 分支 / 人 / 时间），必须占主列。

---

## 2.4 模块说明书

> 每个模块统一给：**目的 / 数据源 / 交互 / 四态**。四态 = 加载 / 空 / 错误 / 无权限。

### M1 HERO + KPI 条（两个元素，**不是一张大卡**）

| 项 | 内容 |
|---|---|
| 目的 | 建立"这是我的工作台"的归属感，并给出 3 个最高频动作 |
| HERO 字段 | 问候语（按本地时间：早上/下午/晚上）+ 用户名；平台一句话定位（可配）；3 个 CTA；克隆地址（取"我的仓库"里 `updated` 最新那个，一键复制） |
| HERO 形态 | 通栏色块（`--cf-color-primary-lighter`），高 128px，**无图、无渐变、无动效**；左右两段式（左文案 + 右 CTA/克隆行） |
| KPI 形态 | **独立一行，无卡片外框**：四项之间用 `1px solid var(--cf-color-border)` 竖线分割，数字 `--cf-font-size-h1` / 700，标签 12px `--cf-color-text-light`。**不做四个方块卡**（方块卡是最典型的"用装饰冒充信息量"） |
| KPI 数据 | `我的仓库`（`repositories.length`）、`我参与的组`（`groups.length`）、`待我评审`（overview.counts.reviewRequests）、`我发起的`（overview.counts.myOpenMergeRequests）；后两个**可点击**跳到 MR 看板并带筛选 |
| 交互 | CTA → `/repositories/new`、`/groups/new`、`/mergerequests/new`；克隆地址用 `CopyButton`（已有组件） |
| 四态 | 加载：KPI 数字用 `--cf-color-bg-dark` 占位条；无仓库：克隆行替换为"还没有仓库，[新建第一个]"；错误：KPI 显示 `—`，不阻塞其余区块 |

### M2 待我评审（首屏最重要的一块）

| 项 | 内容 |
|---|---|
| 目的 | 回答"我现在要做什么"，这是内部平台工作台的第一价值 |
| 数据 | `home/overview.reviewRequests`（≤5 条，按 `mr_updated` 倒序） |
| 行内容 | 仓库名（组/仓）· `!编号` · 标题 · `源分支 → 目标分支` · 发起人头像+名 · 更新时间 · `去评审` 按钮 |
| 复用 | **直接复用 `components/view/unit/MergeRequestRow`**（已支持 `showRepository`），不新写行组件 |
| 空态 | "没有待你评审的合并请求 · [去 MR 看板看看]"（**给动作**，不给插画 —— DeepSeek 取向） |
| 无权限 | 该仓库角色 ≤ guest 时后端**不返回**（不是前端过滤），列表里自然消失 |

### M3 我发起的合并请求

同 M2 结构，数据 `overview.myMergeRequests`（`mr_proceed_status = 1`，≤5 条），行尾显示"待合并 N 天"。

### M4 最近访问

| 项 | 内容 |
|---|---|
| 数据 | **localStorage**（`cf:recent-repos:v1`，最多 6 条，{组名, 仓库名, 时间}），进入仓库页时写入 |
| 为什么不用服务端 | 服务端记录需要新表 + 每次访问一次写（写放大），而"最近访问"是纯个人态、跨设备价值低。**这是有意的取舍**，写入逻辑集中在 `helpers/recentRepos.ts`，将来换服务端只改这一个文件 |
| 过滤 | 渲染前与 `repositories` 求交集（仓库被删/权限被收回后自动消失，不出现点进去 404 的死链） |

### M5 公告

| 项 | 内容 |
|---|---|
| 数据 | 站点配置 `site.announcement`（字符串数组，管理后台可编辑）；为空时展示内置"使用须知"三条（安全规范 / 保密提醒 / 支持渠道） |
| 为什么放配置而不是新表 | 公告是低频、少量、全站一致的内容，进 `config.yaml` 即可，不需要 CRUD 表与后台列表页。**P2 批**再做后台编辑表单 |

### M6 我的仓库（**表格，不是卡片网格**）

| 项 | 内容 |
|---|---|
| 数据 | `useAppData().repositories`（已有，不新增请求） |
| 形态 | **`unit/Table`（`Table/THead/TBody/TR/TH/TD`）**，行高 `var(--cf-table-row-height)` = 40px，与全站表格密度一致。**不用卡片网格** —— 卡片网格会让我们自己成为"唯一一个卡片化的自建 Git 平台"，与 §2.2 的行业结论相悖 |
| 列 | 仓库（方形 `Avatar` + `组/仓`）· 描述（1 行截断）· 待合并（`mergeRequestCount.open`，0 时显示 `—`）· 成员（`members.length`）· 更新于（`FormattedTime`）· 操作（`复制克隆地址`，用 `CopyButton`） |
| 页签 | 最近更新（默认，按 `updated` 倒序）/ 我创建的（`owner === user.id`）/ 我参与的（`role > guest`）/ **我收藏的**（P1，配合 A7 与华为 CodeArts 的三维度） |
| 数量 | 8 行 + "查看全部 →"（跳 `/repositories`，不重复实现完整列表与分页） |
| 空态 | "你还没有仓库 · [新建] / [从已有仓库 Fork]" |
| 备注 | **不做**提交数/分支数/语言分布（要逐库调 git，首页会变成 N 次 IO）——这是与云效 Codeup 的**有意差异**，理由记入 §2.9 D2 |

### M7 快捷入口（**常驻链接行，不单独占卡**）

`config/homeQuickLinks.ts` 单一来源：SSH 密钥（`/settings/sshkey`）、访问令牌（`/settings/tokens` ← A3 落地后）、文档中心（`/doc/cn`）、公开 API（`/doc/cn/api/overview.md`）、Webhook 指南、帮助中心（`/help`）。

放在「平台能力」卡片底部的常驻链接行（见 §2.3）——**不单独占一张卡**，因为它本质是"6 个链接"，为它开一张卡属于用装饰掩盖信息量不足。

### M8 团队动态

`overview.recentActivities`（≤10 条）→ **复用 `components/view/unit/ActivityFeed`**（41 个活动码的表驱动渲染已经存在，别重写）。

**默认只显示 5 条，「展开更多」再显示其余** —— 调研结论是"动态 feed 是加分项，但内部噪音大，建议默认折叠"（§2.2），照此执行。

### M9 平台门户区（下半屏）

| 卡片 | 内容 | 数据 |
|---|---|---|
| 平台概览 | 仓库数 / 组数 / 成员数 / 待合并 MR 数；管理员额外看到"本月新建" | `overview.platform`（全站聚合，1 条 `COUNT`） |
| 四步入門 | 静态步骤卡，每步一个"去做"链接 | `config/onboarding.ts` |
| 团队规范 | 分支命名 / Commit 规范 / 评审与保护分支 → 文档中心对应页 | 静态配置 + `/doc/cn/*` |
| 平台能力 | 10 个能力条目（图标 + 名称 + 一行说明），点击进对应页或文档 | 静态配置 |
| 帮助与支持 | 技术支持 / 反馈 / 贡献 / 关于 | **复用 `config/helpLinks.ts`** |

---

## 2.5 视觉规范（全部取现有令牌，**零新增令牌**）

| 元素 | 取值 | 备注 |
|---|---|---|
| 页面容器 | `max-width: 1280px; margin: 0 auto; padding: space(3) space(6)` | 与 `.app-main` 现有内边距一致 |
| HERO 底 | `background: var(--cf-color-primary-lighter)`；`border-radius: var(--cf-radius)`；高 128px | 全页**唯一**"有色块"，克制；不做渐变（令牌里没有渐变，自造即违规） |
| KPI 条 | **无边框、无底色**，只靠 `1px solid var(--cf-color-border)` 竖线分隔四项 | 不做四个方块卡（见 M1 说明） |
| 区块容器（待办/仓库/动态） | **不加外框**，只靠"区块标题 + 1px 分隔线"分层 | 列表优先：容器本身不应比内容更抢眼 |
| 门户区卡片 | 白底 + `1px solid var(--cf-color-border)` + `var(--cf-radius)`；hover 加 `var(--cf-shadow-card)` | 卡片**只**在这一层出现（§2.2 结论） |
| 区块标题 | `14px / var(--cf-font-weight-bold)` + 左侧 3px `var(--cf-color-primary)` 竖条（`::before`）+ 右上"全部 →"链接 | 全站统一"区块标题"形态 |
| KPI 数字 | `20px / 700`（`--cf-font-size-h1`）；标签 `12px / var(--cf-color-text-light)` | 不新造字号 |
| 行高（列表行） | `var(--cf-table-row-height)` = 40px | 与表格密度一致 |
| 克隆地址 / 命令 | `var(--cf-font-family-mono)` + `var(--cf-color-bg-main)` 底 | DeepSeek 取向：命令直给 |
| 状态色 | 待合并 `success` / 已合并 `secondary` / 已关闭 `warning`（复用 `MergeRequestRow` 的 `statusTone`） | 与 MR 页一致 |
| 断点 | `@media (max-width: 1200px)` 主/侧列 8-4 → 12；`@media (max-width: 768px)` 单列 + 表格降列 | 复用既有断点值 |
| 文件 | 新增 `web/src/styles/pages-home.css`，在 `styles/index.css` 里 import | 与 `pages-*.css` 惯例一致 |

**验收自检（AGENTS §5）**：把 `/home` 与 `/repositories` 截图并排，按钮高度（32px）、圆角（4px）、间距节奏（8 的倍数）、卡片标题字号（14/700）必须看不出差异。

---

## 2.6 后端设计

### 2.6.1 端点

```
GET /api/home/overview        （AuthWebUser，会话 Cookie）
```

> **为什么新建 `home` 控制器而不是塞进 `user`**：语义上它跨 `user / repository / group` 三个域，
> 塞进 `user` 会让"user 控制器 = 用户自身"的边界失真。新增控制器只影响 `controllerNames`
> （`httpapi/server.go`），不影响 123 条契约基线（基线只比对已记录的端点）。
> 也要注意：`home` **已经是 web 控制器名**（`handlers/web.go` 的 `HomeHandler`），
> 但两者在不同 map（`s.controllers` vs `s.webControllers`），**不冲突**；
> 落地时在 `page_spa_test.go` 同批补一条断言，防止将来有人把 `api/home` 和 web `home` 混为一谈。

### 2.6.2 响应结构（字段名与 Go 侧逐字一致，前端 `types.ts` 照抄）

```json
{
  "counts": { "repositories": 12, "groups": 3, "reviewRequests": 4, "myOpenMergeRequests": 2 },
  "reviewRequests": [{
    "id": 128, "key": "…", "title": "…", "status": 1,
    "sourceRepository": "…", "targetRepository": "…",
    "sourceBranch": "feat/x", "targetBranch": "main",
    "creator": { "name": "李四", "email": "…" },
    "updated": 1761600000
  }],
  "myMergeRequests": [ /* 同上 */ ],
  "recentActivities": [ /* 复用 repository/activities 的 item 结构 */ ],
  "platform": { "repositories": 128, "groups": 14, "members": 96, "openMergeRequests": 21 }
}
```

`reviewRequests` / `myMergeRequests` 的条目**复用 `domain.RepositoryMergeRequestView`**（`normalizeMergeRequests` 已经产出该结构），前端直接喂给 `MergeRequestRow`。

### 2.6.3 SQL 口径

> 状态值统一取 `domain.CommonStatusNormal = 1`（`domain/user.go:178`）；
> `mr.u_key` 是**发起人**（`scanMergeRequests` → `mr.UKey`，`repo/repository.go:1054`）；
> `mrr_is_review`：`1` = 已评审，`2` = 未评审（`repository_write.go:676` 注释与 `GlobalFalse`）。
>
> **以下口径均在已部署实例（MySQL 8.0）上用 EXPLAIN 核对过**，见 §2.6.4 末尾的验证记录。

```sql
-- 1) 待我评审（mrr_is_review = 2 表示"未评审"，与 domain 常量一致）
SELECT mr.mr_key, mr.mr_id, mr.mr_title, mr.mr_proceed_status,
       mr.mr_source_r_key, mr.mr_target_r_key, mr.mr_source_branch, mr.mr_target_branch,
       mr.u_key, mr.mr_updated
FROM cc_merge_request_reviewers mrr
JOIN cc_merge_requests mr ON mr.mr_key = mrr.mr_key AND mr.mr_status = 1
WHERE mrr.u_key = ? AND mrr.mrr_is_review = 2 AND mrr.mrr_status = 1
  AND mr.mr_proceed_status = 1
ORDER BY mr.mr_updated DESC
LIMIT 5;
-- 取出后过 repo.RolesFor(该批 rKey, uKey)，丢弃 role <= guest 的行（越权不返回，与公开 API 的 404 口径一致）

-- 2) 我发起的
SELECT … FROM cc_merge_requests
WHERE u_key = ? AND mr_status = 1 AND mr_proceed_status = 1
ORDER BY mr_updated DESC LIMIT 5;

-- 3) 团队动态：**拆成两条**，各自走自己的索引；不要写成 OR 子查询
--    （实测 `WHERE (r_key IN (子查询) OR g_key IN (子查询)) ORDER BY a_created`
--     会让 MySQL 对 cc_activities 全表扫描 —— type=ALL、Using filesort，
--     因为 OR 跨两列 + 排序，优化器用不上任何一条索引）
-- 3a) 先取"我参与的仓库主键"与"我所在的组主键"（两次走成员表单列索引的小查询）
SELECT r_key FROM cc_repository_members WHERE u_key = ? AND rm_status = 1;
SELECT g_key FROM cc_group_members      WHERE u_key = ? AND gm_status  = 1;
-- 3b) 仓库维度（走 idx_act_repo_created，EXPLAIN type=range）
SELECT a_key, a_type, u_key, a_relative_g_key, a_relative_r_key, a_relative_mr_key,
       a_data, CAST(a_created AS CHAR), a_status
FROM cc_activities
WHERE a_relative_r_key IN (?, ?, …) AND a_status = 1
ORDER BY a_created DESC LIMIT 10;
-- 3c) 组维度（走 idx_act_group_created）
SELECT … FROM cc_activities
WHERE a_relative_g_key IN (?, ?, …) AND a_status = 1
ORDER BY a_created DESC LIMIT 10;
-- 3d) Go 侧按 a_key 去重（仓库动态同时带 g_key，组也在我的列表里时会重复一次），
--     再按 CAST(a_created AS CHAR) 的字典序倒排取前 10。
-- 注：现有 activities 查询（repo/repository.go、repo/group.go）都只用
-- a_status = 1，不过滤 a_deleted（该列历史语义为软删标记，业务查询未使用），
-- 这里保持一致，不引入新的过滤口径。

-- 4) 平台概览（管理员才显示"本月新建"）
SELECT COUNT(*) FROM cc_repositories WHERE r_status = 1;
SELECT COUNT(*) FROM cc_groups WHERE g_status = 1;
SELECT COUNT(*) FROM cc_merge_requests WHERE mr_status = 1 AND mr_proceed_status = 1;
```

全部走 `repo` 层新增方法（`HomeRepo` 或挂在既有 repo 上），**在事务外**执行（AGENTS 陷阱表：事务内不做重 IO）。

### 2.6.4 索引迁移 `011_perf_index_home.sql`

```sql
-- 首页"待我评审 / 我发起的"两条查询此前无可用索引（cc_merge_request_reviewers 无组合索引，
-- cc_merge_requests 无 (u_key, 状态) 组合索引），数据量上千后会全表扫。
ALTER TABLE cc_merge_request_reviewers
    ADD INDEX idx_mrr_user_review (u_key, mrr_is_review, mr_key);

ALTER TABLE cc_merge_requests
    ADD INDEX idx_mr_creator_status (u_key, mr_proceed_status, mr_updated);

ALTER TABLE cc_activities
    ADD INDEX idx_act_repo_created (a_relative_r_key, a_created);

ALTER TABLE cc_activities
    ADD INDEX idx_act_group_created (a_relative_g_key, a_created);
```

 > 四个索引都是 `ADD INDEX`，对已部署实例**必须走 migrations**（写进 `db_structure.sql` 无效 —— AGENTS §9）。

**验证记录（2026-10-27，本机已部署实例 `codefever-next-db` / MySQL 8.0）**：

- 迁移已在已部署实例上执行，`cc_schema_migrations` 记录 `011_perf_index_home.sql`；
- `idx_mr_creator_status`：EXPLAIN `type=range` + `Backward index scan`，**无 filesort**；
- `idx_act_repo_created` / `idx_act_group_created`：字面量 IN 列表时 `type=range`；
  而 OR 子查询写法会退化成 `type=ALL`（这就是 §2.6.3 第 3 条拆成两条的原因）；
- `idx_mrr_user_review`：现有数据量太小（个位数行），优化器仍可能选单列 `u_key`，
  属正常的"小表不必用组合索引"，组合索引保留给数据量上来之后。

### 2.6.5 `/home` 的路由与重定向

| 位置 | 改动 |
|---|---|
| `backend/cmd/codefever-api/main.go` | 在 `/help` 注册处（约 L284）追加 `srv.RegisterPage("/home", func(w,r){ httpapi.ServeSPA(w,r) })` —— **必须 200 下发壳**，否则浏览器与爬虫会当成 404（`page_spa_test.go` 的注释已经写清这个坑） |
| `backend/internal/handlers/web.go` `HomeHandler.index` | 已登录的 302 目标由 `/repositories` 改为 `/home`；未登录仍 302 `/user/login`（保持"不登录看不到任何平台信息"的安全口径） |
| `web/src/router.tsx` L91 | `{ index: true, element: <Navigate to="/home" replace /> }`，并新增 `{ path: 'home', element: suspense(<Home />) }` |
| `web/src/components/layout/Header.tsx` | logo 点击 `/repositories` → `/home`；左侧菜单首位加"首页"文本入口（当前路径打勾规则与仓库/组菜单一致） |

> 只改前端路由不改 Go 的 302 会留下"直接访问 `/` 时跳到 `/repositories`"的旧行为；
> 两处都要改，且**先改 Go 再改前端**（否则发布瞬间 302 与路由不匹配）。

### 2.6.6 性能预算

| 项 | 预算 |
|---|---|
| 聚合端点 SQL 条数 | 6（待办列表 ×2、KPI 总数 ×1、动态 ×2、平台概览 ×1） |
| 聚合端点 p95 | < 120ms（本地 `cmd/stress` 目标端点） |
| 首页首屏 JS | 不新增依赖；Home 页 `lazy`，与 `RepositoryList` 同量级 |
| 首屏请求数 | 4（3 个 AppData + 1 聚合） |
| 非首屏 | 「团队动态」与「门户区」用 `IntersectionObserver` 懒请求/懒渲染（若聚合端点已含动态则只做懒渲染） |

---

## 2.7 前端文件级改造清单

### 新增

| 文件 | 内容 |
|---|---|
| `web/src/pages/Home.tsx` | 页面装配：读 `useAppData()` + `fetchHomeOverview()`，按 §2.3 拼区块 |
| `web/src/components/view/home/HomeHero.tsx` | M1（问候 + CTA + 克隆地址，通栏色块） |
| `web/src/components/view/home/StatStrip.tsx` | KPI 一行四项（无外框，竖线分隔；用 `unit/Stat`） |
| `web/src/components/view/home/MergeRequestPanel.tsx` | M2/M3 共用（标题 + 角标 + "全部→" + `MergeRequestRow` 列表 + 空态） |
| `web/src/components/view/home/RecentRepositories.tsx` | M4 最近访问（紧凑列表 ≤6 行） |
| `web/src/components/view/home/MyRepositoriesTable.tsx` | M6 我的仓库（**`unit/Table`** + 页签 + 8 行 + 查看全部） |
| `web/src/components/view/home/QuickLinks.tsx` | M7 常驻链接行（渲染在门户区「平台能力」卡底部） |
| `web/src/components/view/home/PlatformSection.tsx` | M9 门户区（概览 / 四步入門 / 规范 / 能力四块） |
| `web/src/components/view/home/home.css` | 只放首页区块级样式（布局、栅格），**不放字面值** |
| `web/src/components/unit/Stat/index.tsx` | 数字 + 标签 + 可选链接（**出现 ≥2 次即刻组件化**，AGENTS §4） |
| `web/src/components/unit/SectionCard/index.tsx` | 区块容器：标题 + 左侧竖条 + 右上操作槽 + 内容槽；`variant='plain'`（无外框，用于待办/仓库/动态）与 `variant='card'`（有边，用于门户区）两种 —— 首页 8 个区块共用，避免重写 8 遍 |
| `web/src/api/home.ts` | `fetchHomeOverview()` |
| `web/src/helpers/recentRepos.ts` | localStorage 最近访问（读写/去重/LRU 6 条/容错解析） |
| `web/src/config/homeQuickLinks.ts` | 快捷入口清单 |
| `web/src/config/onboarding.ts` | 四步入門 |
| `web/src/config/platformCapabilities.ts` | 平台能力矩阵（图标取 `config/icons.ts` 按需 import —— **不要 barrel**，AGENTS §9） |
| `web/src/styles/pages-home.css` | 页面级（容器宽度、栅格、断点） |

### 修改

| 文件 | 改动 |
|---|---|
| `web/src/router.tsx` | index 重定向 + `home` 路由 |
| `web/src/components/layout/Header.tsx` | logo → `/home`；加"首页"入口 |
| `web/src/api/types.ts` | 新增 `HomeOverview` / `HomeMergeRequestItem` / `HomeActivityItem` / `HomePlatformStats` |
| `web/src/i18n/supplement.ts` | 追加首页词条（见附录 A） |
| `web/src/styles/index.css` | import `pages-home.css` |
| `web/src/pages/repository/FileTree.tsx`（及 RepositoryLayout） | 进入仓库时写 `recentRepos.record(组名, 仓库名)` |
| `backend/internal/handlers/home.go`（新） | `HomeHandler.Register` + `overview` |
| `backend/internal/repo/home.go`（新） | 4 条查询 + `RolesFor` 过滤 |
| `backend/internal/db/migrations/011_perf_index_home.sql` | 见 §2.6.4 |
| `backend/cmd/codefever-api/main.go` | 装配 `new(HomeHandler)` + `RegisterPage("/home")` |
| `backend/internal/handlers/web.go` | `home/index` 302 目标改 `/home` |
| `backend/internal/domain/home.go`（新） | `HomeOverview` 结构与 normalize |
| `PROJECT_MAP.md` / `CHANGELOG.md` | 新增页面/控制器/迁移 → **同一逻辑单元内**更新（AGENTS §2/§3） |

### 复用（**不要重写**）

`MergeRequestRow`、`ActivityFeed`、**`Table` / `THead` / `TBody` / `TR` / `TH` / `TD`**（我的仓库列表）、`Avatar`、`Card`（仅门户区）、`CopyButton`、`FormattedTime`、`EmptyState`、`Tabs`、`Tag`、`Button`、`AppLink`、`resolveMember`、`useAppData`。

---

## 2.8 分批实施计划

| 批 | 范围 | 交付物 | 预估 | 验收 |
|---|---|---|---|---|
| **H0 骨架** | 路由 + Go 页面路由 + logo/首页入口 + 全部区块用**现有数据**占位（`AppDataProvider` 的 repo/group/user） | `/home` 可打开，KPI/我的仓库/仓库组 三块真数据，其余空态 | 0.5d | 200 下发壳；`typecheck`/`lint`/`build` 绿；空态四态齐 |
| **H1 工作台** | `GET /api/home/overview` + 迁移 011 + M2/M3/M8 + M1 的 KPI 接真数据 | 待我评审 / 我发起的 / 团队动态 / KPI | 1d | 端点 p95 < 120ms；越权仓库不出现（补 1 条 Go 测试）；迁移在已部署实例验证 |
| **H2 门户** | M4 最近访问 + M5 公告（配置项）+ M6 表格与页签 + M7 + M9 门户区四块 | 完整首页 | 1d | 截图对比（1440/1280/390 三档）；零字面值（自查 `grep -n "#\|px" pages-home.css` 只允许 var） |
| **H3 可选** | 公告后台编辑 + 收藏置顶（`cc_repository_stars`）+ 管理员"平台概览"扩展 | — | 1d | — |

**每批的门禁（AGENTS §8）**：`pwsh -File scripts/verify.ps1`；前端 `npm run typecheck && npm run lint && npm run build`；`web/build` 重建并入库。**不做 `git commit`，每批结束把建议的 commit message 交给用户确认**（AGENTS §1）。

---

## 2.9 风险、取舍与偏离登记（AGENTS §7）

| # | 取舍 / 风险 | 为什么 | 代价 | 何时回到"标准路径" |
|---|---|---|---|---|
| D1 | 最近访问用 **localStorage** 而非服务端表 | 服务端需要新表 + 每次访问一次写；个人态价值低 | 换设备/换浏览器看不到历史 | 若产品要求跨设备 → 建 `cc_repository_visits` 并只改 `helpers/recentRepos.ts` |
| D2 | 首页**不展示**提交数/分支数/语言分布（与 Codeup 有差异） | 三者都要逐库调 `git`，8 个仓库 = 8 次子进程，首屏会掉到秒级 | 信息密度低于 Codeup | 若已在 `cc_repositories` 落快照列（配合 worker 定期刷新）再加 |
| D3 | `/home` 与 `/repositories` 职责重叠 | 有意重叠：home = 聚合 + 门户（≤8 行），列表页 = 全量 + 筛选 | 需要文档写清分工，否则后人会把列表搬回首页 | 已在 §2.1「明确不做」固化 |
| D4 | 新增 `home` **API 控制器**，与既有 web 控制器同名 | 语义正确（跨域聚合）；两个 map 隔离，不冲突 | 后来者可能误解 | 在 `home.go` 注释 + `PROJECT_MAP` §3 写清"web home ≠ api home" |
| D5 | 抽 `unit/SectionCard` / `unit/Stat` 两个新组件 | 首页 8 个区块共用，AGENTS §4「第二次就抽」 | 组件库 +2 | 无需回退 |
| D6 | H0 批先上线"半截首页" | 让路由与入口先正确，避免一次性大改动 | 需 1 天窗口内容偏少 | H1/H2 合并发布即可 |
| D7 | 公告进 `config.yaml` 而非新表 | 低频少量全站一致 | 改公告要重启服务（配置热加载不含此项） | 若公告要按组/多语言 → 建表 + 后台页 |
| **D8** | **不做角色化模板 / 可配置卡片布局**（云效 5 角色 16 卡片、CODING 3 套模板都有） | 需要配置中心 + 每用户布局持久化 + 卡片注册表，成本约 3 倍于收益；本平台用户同质（都是研发/测试），差异化收益低 | 产品经理/项目经理看到的工作台与研发一样 | 若公司出现"非研发角色大量使用"的证据 → 先做**后端下发的固定模板**（无拖拽），再考虑自选 |
| **D9** | **我的仓库用表格，不用卡片网格**（本方案第一版是卡片栅格，已修正） | 调研结论：对内平台几乎全部"列表优先于卡片"，卡片只出现在项目管理语境；用卡片网格会让我们成为唯一一个卡片化的自建 Git 平台，也破坏全站表格密度一致性 | 仓库缩略图/图标展示面积变小 | 若产品要求"仓库门户"视觉 → 在 `/repositories` 里加一个可切换的"卡片视图"（列表仍是默认） |
| **D10** | 首屏**不**照抄云效"首屏即全量仓库表格" | 云效的"入口即列表"成立是因为它还有 MR/检测/回收站等一级导航做分流；我们首屏若直接上全量列表，等于把 `/repositories` 复制一遍，员工仍然不知道"我该做什么" | 与云效首页形态不同 | 若日后导航栏扩展到 5+ 一级入口，可把"仓库列表"提升为首屏主模块 |

**与 AGENTS 的关系**：本方案**不偏离** §4（组件化）、§5（零字面值）、§6（页面只写业务）；§9 的三条相关陷阱（`build/` 入库、图标不 barrel、迁移不放 `db_structure.sql`）已在 §2.7 / §2.6.4 显式约束。

---

## 2.10 后续可选增强（P2，不在本方案范围）

- 贡献榜 / 月度活跃（需要 git 提交聚合，建议 worker 预计算到快照表）
- 首页可拖拽自定义 + 每个用户的布局偏好（CODING 式，需配置中心）
- 未登录访客的公开落地页（当前策略：未登录一律 `/user/login`）
- 暗色模式（令牌已集中，加 `[data-theme="dark"]` 即可）
- 代码扫描结果卡片（依赖 A8）

---

## 附录 A：i18n 键清单（追加到 `web/src/i18n/supplement.ts`）

```
home.greetingMorning / home.greetingAfternoon / home.greetingEvening
home.tagline
home.kpi.repositories / home.kpi.groups / home.kpi.reviewRequests / home.kpi.myOpenMergeRequests
home.section.reviewRequests / home.section.myMergeRequests / home.section.recent / home.section.mine（表格标题）
home.section.announcement / home.section.quickLinks / home.section.activities / home.section.platform
home.section.overview / home.section.onboarding / home.section.rules / home.section.capabilities
home.tab.recentUpdated / home.tab.createdByMe / home.tab.joined / home.tab.favorite（P1）
home.empty.reviewRequests / home.empty.myMergeRequests / home.empty.recent / home.empty.repositories
home.action.goReview / home.action.viewAll / home.action.copyClone
home.cloneLabel / home.quickStart.step1…step4 / home.announcement.default1…3
```

> 已有等价键优先用旧键（`label.repository`、`label.mergeRequest`、`message._S_listEmpty` 等），
> 只在旧词表确实没有等价表达时新增（`supplement.ts` 文件头的约定）。

## 附录 B：参考资料

> **完整调研原文**（逐条带 URL、含"无公开资料"的明确标注、含 12 条跨平台共性结论）：
> `docs/RESEARCH_HOME_DASHBOARD_2026-10-27.md`。下表只是索引。

| 主题 | 出处 |
|---|---|
| 阿里云 云效 Codeup 首页（代码库/代码组/合并请求/代码检测/回收站；"我参与的"默认更新时间倒序、收藏置顶） | https://help.aliyun.com/zh/yunxiao/user-guide/home-overview |
| 阿里云 云效 个人工作台（先选角色 → 5 套模板 → 16 张卡片自选） | https://help.aliyun.com/zh/yunxiao/user-guide/personal-workbench |
| 阿里 Aone（服务 2 万+工程师的端到端平台，公开资料只讲后端架构） | https://developer.aliyun.com/article/605149 |
| 腾讯 CODING 工作台（登录默认落地；三套角色模板；「我发起的」「我评审的」；自定义菜单与常用链接） | https://coding.net/help/docs/member/workbench.html |
| 腾讯工蜂 TGit（项目/项目组 + 待办聚合 + 关注标星 + OWNERS + 划词评审） | https://code.tencent.com/production/ ／ https://code.tencent.com/help/productionDoc/project |
| 小米 研效工具链（GitLab + Phabricator + Jenkins on K8s + Nexus，自研平台定位为"标准化整合、一键接入"；**首页 IA 无公开资料**） | https://devops.phodal.com/case-study/xiaomi/ |
| 小米 × 极狐 GitLab（厂商案例页：私有化部署、多产品线并行） | https://gitlab.cn/cases/cmifrmxlq001h42jgqcyw5gyr |
| 小米开源一站式研发平台 Mone | https://github.com/xiaomi/mone |
| GitHub 个人 Dashboard（Recent activity / Top repositories / Feed；新版 Home 把 AI 入口放首屏第一） | https://docs.github.com/en/account-and-profile/reference/personal-dashboard |
| GitLab 个人首页（待我评审卡、Items that need your attention、Quick access 最近浏览） | https://docs.gitlab.com/tutorials/personal_homepage/ |
| Gitea / Forgejo dashboard（个人↔组织上下文切换、Issue/PR 聚合视图、活动 feed） | https://deepwiki.com/go-gitea/gitea/7.1-user-dashboard-and-activity-feeds |
| 华为 CodeArts（工作台页签；仓库列表「我关注的/我参与的/我创建的」三维度） | https://support.huaweicloud.com/intl/zh-cn/usermanual-devcloud/devcloud_01_0024.html |
| 百度 iCode（CR 机制 / 代码搜索 / 操作审计；首页 IA 无公开资料） | https://cloud.baidu.com/product/icode.html |
| DeepSeek 内部 Git 平台 | **无公开资料**。仅 `github.com/deepseek-ai` 与官网可循其工程取向（极简、命名统一、几乎无装饰），**仅作风格参照，不作为产品事实** |

---

## 附录 C：本方案的验证方式（落地时逐条打勾）

- [ ] `GET /home` 返回 200 + SPA 壳（不是 404 壳）
- [ ] 登录后 `/` → `/home`；未登录访问 `/home` → `/user/login`
- [ ] **形态自检**：首屏无卡片墙 —— KPI 是一条无外框的数字行，「我的仓库」是 `<table>`，卡片只出现在门户区
- [ ] `GET /api/home/overview` 未登录返回 0x0101；无权仓库不出现在待评审列表
- [ ] 迁移 011 在**已部署实例**执行成功，`cc_schema_migrations` 有记录
- [ ] 首页 p95 < 120ms（`cmd/stress`，且**先关 `general_log`**）
- [ ] 三档截图（1440 / 1280 / 390）无布局断裂
- [ ] `pages-home.css` 与组件 CSS 中无颜色/间距/圆角字面值
- [ ] `PROJECT_MAP.md`（新增页面/控制器/迁移/组件）、`CHANGELOG.md`（用户可见变更）已同步
