# 更新日志

本文件记录 CodeFever Next 的重要变更。

格式遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

---

## Unreleased

尚未发布的变更集中在此。

### 变更

- **仓库结构收敛为单一 Go module**：`http-gateway/` 与 `ssh-gateway/` 并入
  `backend/cmd/codefever-http-gateway`、`backend/cmd/codefever-ssh-gateway`；
  `misc/` 收敛到 `backend/deploy/{sql,docker}`。构建统一为
  `cd backend && go build ./...`，不再需要 GOPATH 模式或构建期 `sed` 改写 import。
- **镜像改为 5 个二进制**：`codefever-api`、`codefever-worker`、`codefever-hook`、
  `codefever-http-gateway`、`codefever-ssh-gateway`；旧路径
  `misc/hooks`、`ssh-gateway/shell/main` 保留兼容软链。
- **配置持久化**：`config.yaml` / `env.yaml` 改为 `<base>/config/` 的软链，
  默认落在仓库根目录的 `codefever-data/config/`；管理后台修改的主机名、邮件设置等
  不再因容器重建丢失。
- **数据目录改为 `codefever-data/`**：MySQL、裸仓库、附件、配置全部绑定到仓库根目录
  （`codefever-data/{mysql,git-storage,file-storage,config}`），可用
  `CODEFEVER_DATA_DIR` 覆盖；不再使用 Docker 命名卷，备份/迁移只需处理一个目录。
- **站点地址自动生成**：入口脚本按 `CODEFEVER_HTTP_PORT` / `CODEFEVER_SSH_PORT`
  写入 `host` / `ssh`，支持 `CODEFEVER_SITE_HOST` / `CODEFEVER_SITE_SSH` 覆盖。
- **邮件 SMTP**：新增 `CODEFEVER_SMTP_*`；未配置时回退系统 `mail`（镜像不含 MTA）。
- **安全默认值**：镜像构建期随机生成 `session.key`、`totp.salt`、`gateway.token`；
  `config.yaml` / `env.yaml` 权限降为 `0660`，不再用 `sed` 拼接 DB 口令；
  数据库参数同时支持 `CODEFEVER_MYSQL_*` 与旧 `DB_*`。
- **迁移收尾修复**：
  - `post-receive` 现在写入 `HOOK_POST_RECEIVE` 事件，push 后活动流 / 通知 / webhook 恢复；
  - `group` 域事件总线在生产装配中正确地注入（此前 10 个 group 事件被静默丢弃）；
  - MR 详情返回评审人；fork MR 的跨仓库 `fileChanges` / `mergeRequestCommits` 已实现；
  - 禁用账号不能登录/继续使用旧会话，改密/管理员重置口令会吊销会话；
  - 事件 worker 增加租约回收与 panic 兜底，避免事件永久卡在 Processing；
  - MR 合并使用 MySQL 命名锁串行化，避免并发重复合并；
  - HTTP 网关删除危险的"内部模式"鉴权旁路，POST push 会按 `git-receive-pack`
    校验 PUSH 权限；SSH 公钥写入拒绝换行/NUL 注入并改为原子替换；
  - 请求体上限、git 命令兜底超时、数据库连接/读写超时、`/api/*` 尾斜杠兼容；
  - **默认分支为 main 的仓库 clone 修复**：`git init --bare` 的 HEAD 默认指向
    `master`，首次 push `main` 后 clone 会报 `remote HEAD refers to nonexistent ref`
    且工作区为空。现在 push / 修改默认分支 / 服务启动时会自动把 HEAD 指向真实分支。
- **依赖**：`backend/go.mod` 降到 `go 1.23.0`（Docker 仍用 Go 1.25 构建）。
- **文档进镜像**：Dockerfile `COPY doc/` 与 `COPY LICENSE`，应用内文档中心可用。
- **CI 已恢复**。`.github/workflows/ci.yml` 提供 Go 1.23 的 gofmt / build /
  Linux 交叉编译 / vet / test 与 `docker compose config`；本地对应门禁为
  `scripts/verify.ps1`（同样的最小集，可选 `-BuildImage`）。Dependabot 仍未配置。
- **README/安装/管理/安全文档按当前结构重写**：删除 `misc/`、`http-gateway/`、
  `ssh-gateway/`、`Dockerfile.local`、`docker-compose.local.yml`、`CODEFEVER_BACKEND`
  等旧路径/旧开关；英文安装文档与 Webhook 页面同步更新。
- **新前端批 8：逐页核对旧版后修正的语义偏差**（与旧版行为对齐，见
  `docs/FRONTEND_FIELD_DIFF_2026-10-24.md`）：
  - **个人资料的 `role` 恢复为可编辑**并随 `user/updateBasicInfo` 提交
    （旧版资料页可编辑 `role`；新版一度改成只读）；
  - **保护分支「允许 Push / Merge 角色」恢复角色集合语义**：
    旧版表达的是「最低角色及以上全部可操作」，新版压扁成了单个角色标签；
  - **MR 详情的「使用 Squash」恢复为可选**：新版此前硬编码为压缩合并。
    顺带修掉旧版的一个隐藏缺陷 —— 旧前端把 `usingSquash` 以布尔下发，
    PHP 侧 `trim(false) === ''` 导致「取消勾选」实际无效；新版传字符串
    `'false'`，后端 `PHPTrim(...) != "false"` 才能真正识别；
  - **通知开关恢复双向**：此前只能"打开"不能"关闭"；
  - **MR 列表搜索改回回车/点按钮触发**（旧版 `searchMergeRequests()`），
    不再逐键打后端；
  - **提交列表每页 10 → 20 条**（旧版 `CommitList.js` 的 pageSize）；
  - **合并请求状态徽标配色回到旧版**：待合并=success、已合并=info、已关闭=warning
    （为此给 `Tag` 组件补了 `info` tone，色值取主题令牌 `--cf-color-secondary`）；
  - **仓库组设置的组名校验**（5–30 字 + 字符集）、描述 200 字计数与上限、
    `0x0404` 组名重复提示。
- **新前端批 9：仓库文件页与侧边栏对齐旧版视觉**：
  - **文件列表恢复为带表头的表格**（名称 30% / 最近提交 / 最近更新右对齐，外框圆角），
    并复用组件库 `Table` 原语，不再自绘 div 行；提交信息列恢复为**可点击的详情链接**
    （旧版 `TableList` 行为，此前只是纯文本）；
  - **文件页顶部"当前路径最近提交"恢复卡片外观**（1px 边框圆角盒）、提交标题恢复
    旧版默认链接主色、短哈希旁补回**一键复制**按钮（对齐旧版 `CommitHashChip`）；
  - **分支/标签选择器恢复 outlined 外框**（对齐旧版 `RevisionSelector`）；
    空仓库不再渲染文件工具条（旧版语义）；
  - **侧边栏收起后图标在 88px 内水平居中**（此前贴左）；收起时隐藏设置子菜单
    （纯文字子项此前会溢出侧栏），与旧版 `DrawerList` 的收起行为一致；
  - **修复侧边栏激活态不随页面切换**：「文件」项的 active 正则误带了 `(/.*)?`
    通配后缀，导致它在仓库的所有子页（提交/分支/标签等）都命中高亮；
    现按旧版 `DrawerConfig` 的模式逐条对齐（仓库根 / files / blame），
    并顺带对齐组导航的「仓库」项（`/groups/{组}/repositories/new` 也应命中）；
  - **提交列表页布局对齐旧版 `CommitList` + `TitleList`**：
    每个日期组恢复为 1px 边框盒 + 灰底标题条（`yyyy-MM-dd [N 提交]`，行高 40px、
    左右内边距与行对齐），组内提交行补回左右 24px 内边距；标题格式从圆括号改回
    旧版的方括号（补充词表新增 `label._N_commit`，自动表里是轻量 t() 渲染不了的
    ICU 复数串；英文放弃单复数区分，n=1 显示 "1 commits"）；
  - **提交列表分页恢复旧版「查看更多」**：页码分页改回居中的「查看更多 / 处理中 /
    没有更多」按钮，跨页同日期合并进同一组；分支/标签下拉改为写 URL
    （`/commits/{rev}`，旧版行为），换 ref / 子目录时重置页码与搜索；
  - **提交列表工具条对齐旧版**：分支选择器补 outlined 外框；搜索框改回**回车提交**
    （旧版 onKeyDown Enter，此前逐键请求后端），去掉旧版没有的「重置」按钮；
    空仓库不再渲染工具条；子目录提交列表点提交恢复跳「该提交的目录视图」
    （`/files/{子路径}?rev={sha}`，旧版 linkPathBase='files'+path 语义）；
  - 预览 mock（`web/scripts/serve-preview.mjs`）的 `commitList` 端点改为按
    `page`/`pagesize`/`keyword`（GET query）真实切片，并补足一页以上的数据量，
    使「查看更多」的追加与跨页日期合并可被视觉验证。
  - 预览 mock（`web/scripts/serve-preview.mjs`）的 `repository/object` 响应补上
    真实后端本就返回的 `commit` 字段，文件页顶部卡片可被视觉验证覆盖。
- **公开 API 错误体的 `url` 改为指向真实文档页**（`/doc/cn/api/overview.md`，
  由 `backend/internal/httpapi/publicapi.DefaultDocPath` 定义）：此前拼成 `<站点>/api/v1`，
  而该前缀只服务 JSON 端点，浏览器直接访问只会得到 404 —— 错误提示里等于给了一条死链。
  英文文档页 `/doc/en/api/overview.md` 内容一一对应。

### 新增

- **站内帮助中心 `/help`**：页头帮助菜单此前四项全跳站外，站内没有"遇到问题去哪儿"的落点。
  新页面把**技术支持 / 提交反馈 / 为 CodeFever Next 贡献代码 / 关于 CodeFever Next**
  四个入口收在一页，并新增「API 接口文档」区块（端点总览、令牌三种传法、错误体示例，均可一键复制）。
  - 页头帮助菜单改为站内优先：帮助中心 → 文档中心 → API 接口文档，之后才是站外入口；
  - 站外地址口径由上游 `PGYER/codefever` 改为本仓库 `AidenX0704/codefever-next`
    （上游仓库与官网仍在「关于」里交代来源），地址集中在 `web/src/config/helpLinks.ts`；
  - `GET /help` 由精确页面路由下发 SPA 壳（**200**）；走兜底路由会命中 CI 的 nopage 分支、
    以 404 状态码下发壳，浏览器/爬虫会当成"页面不存在"；
  - 新增通用组件 `components/unit/CodeBlock`（文档片段：等宽 + 语言标签 + 复制，无行号），
    与面向仓库文件的 `CodeViewer` 分工，供后续设置页/文档类页面复用。

- **文档中心新增「API 接口」章节（中英双语各 5 篇）**：`doc/{zh-cn,en-us}/api/` —
  概览、认证与访问令牌、端点参考、错误与限制。内容以
  `backend/internal/httpapi/publicapi/` 的实现为准，逐项写明 4 个端点的鉴权要求、
  字段、状态码与「令牌管理 UI 尚未实现」等限制，避免使用者按猜测对接。

- **公开 REST API `/api/v1`（新命名空间，与内部 `/api/*` 完全隔离）**：外部 CI / 脚本
  现在可以用 API 令牌访问 CodeFever，响应是**裸 JSON**、错误体是 `{"message","url"}`，
  不再暴露内部 RPC 的 `{request-id,code,message,data}` 信封。
  - **首批端点**：`GET /api/v1/user`（令牌必需）、`GET /api/v1/user/repos`（令牌必需）、
    `GET /api/v1/users/{username}`（令牌可选）、`GET /api/v1/repos/{owner}/{repo}`（令牌可选）；
  - **`{owner}` 同时接受用户名与仓库组名**（CodeFever 仓库挂在组下）；
  - **私有仓库越权返回 404 而不是 403**，避免泄露私有资源的存在性；
  - **为什么要新开命名空间**：现有 `/api/*` 是 PHP 兼容契约（123 条快照基线），
    动词式路径 + multipart 编码 + 固定信封都是为自家 SPA 定制的；
    改造它等于重写 111 个端点并破坏基线，因此公开 API 只复用 repo 层与权限判定，
    响应包络、鉴权方式、分页语义全部自建；
  - **路由隔离方式**：公开 API **自建 mux** 并在进入主 mux **之前**分流 ——
    主 mux 上的 `/api/{ctrl}/{action}` 与 `/api/v1/` 在 Go 的 `ServeMux` 下构成
    歧义模式（**注册即 panic**），因此公开 API 不能挂在主 mux 上；
- **API 访问令牌体系**：新增 `cc_access_tokens` 表（迁移
  `backend/internal/db/migrations/010_access_tokens.sql`）与令牌仓储。
  - **库里只存 SHA-256 摘要，不存明文**；`domain.AccessToken` 类型**刻意不含明文字段**，
    从类型层面保证明文无法被持久化；明文仅在创建时返回一次；
  - 令牌采用 **令牌 ≡ 用户** 模型（`at_scope` 列预留，第一版不实现 scope）；
  - 令牌明文为 32 字节随机数转 base64url（43 字符），支持
    `Authorization: token <明文>` / `Bearer <明文>` / `?access_token=` 三种传法；
  - 吊销只允许操作**自己名下**的令牌，吊销后立即失效；
  - 吊销、列表、重名检测等接口与集成用例已就绪；**令牌管理 UI 与内部端点
    排在 Phase 1**，本轮**尚未**提供。

- **新前端骨架 `web/`（React 19 + TypeScript + Vite）**：与旧前端并存的第二批前端，
  按 `docs/FRONTEND_REWRITE_PLAN.md` 分批替代 `www/view`（React 17 + webpack 4）与
  `www/static`（jQuery/Bootstrap 的 SSR 页面资源）。本批只搭骨架，不影响线上：
  - 设计令牌层 `web/src/theme/theme.css` 成为**全仓库唯一的视觉字面值来源**
    （颜色/间距/圆角/阴影/字号逐项抄自旧 `@pgyer/mui-theme-preset` 与 `theme1.js`，
    例如 `--cf-color-primary: #3455db`、表格行高 40px、圆角 4px），
    组件与页面一律引用 `var(--cf-*)`；
  - 自研组件库首批 21 个原子件（按钮/输入/选择/复选/开关/表单/表格+分页/弹窗/确认/
    页签/气泡/消息/头像/标签/卡片/加载/空态/复制/链接），**零 UI 框架依赖**；
  - 统一请求层 `web/src/api/client.ts` 复刻旧版线上契约（multipart 的 `key[sub]`/`key[]`
    编码、剥 `_SAFE`/`_BINARY`、GET 追加 `requestTime`、`codefever-end-*` 头、
    错误码文案表、`0x0101` 跳登录）；
  - 设计系统预览页 `/__design`，作为后续每批的视觉比对基准。
  - 为什么这么做：旧前端 webpack 4 + CRA fork 已 EOL（依赖 Node `--openssl-legacy-provider`），
    且 MUI v4 的 JSS 运行时占了首屏 JS 的绝大部分；换自研令牌 + 组件库后首屏只依赖
    React 本身（骨架产物 vendor 98 KB gzip，全站 116 KB gzip）。
- **新前端批 1：认证四页**（登录 / 注册 / 二次验证 / 找回密码）落地到 `web/`：
  - 文案改成**自动生成**的 i18n（`web/src/i18n/*.ts`，从 Go 语言表抽 54 键 × 2 语言，
    脚本 `web/scripts/extract-lang.mjs`），从此前端与后端各有一份词表的重复问题消失；
  - 新增 `postForm()` 请求通道：认证端点用的是旧表单页的 `{code,message,extra}` 信封
    （业务码 1..8/100，其中 100 表示"需要 2FA"而不是错误），与 `/api/*` 信封区分开，
    避免把 2FA 中间态当异常处理；
  - 视觉几何与旧页逐项对齐（实测）：卡片 1184×644、左图 644、表单列 360、背景 `#f7f8fa`、
    提交按钮 `#0035EC`；三个视口下卡片实测 1184×644 / 540×644 / 450×644，
    与旧 CSS 的 `width:80%;max-width:540px`、`90%` 规则一致；
  - 登录页首图改用 `<picture>` 优先 WebP（30.5 KB vs 原 PNG 147.9 KB），沿用旧性能整改 A4；
  - 本批**不改变线上行为**：Go 侧"GET → SPA 壳"与 nginx/Docker 切换顺延到批 7，
    原因与回归条件记在 `docs/FRONTEND_REWRITE_PLAN.md` §10 批 1 的偏离表。
- **新前端批 2：应用外壳 + 仓库列表 + 仓库组列表**：
  - **i18n 改为"执行旧词表"生成**：旧 SPA 的文案是运行时拼词
    （如 `newRepository = Phrase.new + Term.repository`），静态解析会漏；
    脚本现在会真正执行旧 lang 模块再按命名空间合成，得到 **961 键 × 2 语言**，
    并支持 `{s}`/`{n}` 占位符替换；
  - **图标彻底本地化**：`@pgyer/icons` 的 154 个图标数据落成 TS 模块，新增 `Icon`
    组件直接渲染 `<svg><path/></svg>`，**移除 `@fortawesome/*` 依赖**，
    也不再复现"barrel 一次打包全部 154 个图标"的体积问题；
  - 新增 `Dropdown` / `SquareIconButton` / `Marker` / `FormattedTime` 四个组件，
    页头（仓库、仓库组、新建、语言、帮助、管理后台、用户菜单）与列表行全部复用；
  - 数据层新增 `AppDataProvider`（对应旧 Redux `DataStore` 的用户/仓库/组三份共享数据），
    避免页头与列表页重复请求同一批接口；
  - 布局对齐旧 `Main.js`：固定页头 64px + 白底内容区 + `padding: 24px 48px`
    （像素量测：页头 64px、仓库行节距 71px = 行高 72px + `margin-bottom:-1px`）；
  - 视觉验证新增本地 mock 通道（`web/scripts/serve-preview.mjs` 内置三个端点的 mock），
    本机没有后端也能校验列表在有数据时的行高、对齐与截断。
- **新前端批 3（上）：仓库上下文导航 + 文件浏览 + 代码高亮**：
  - 仓库页外壳 `RepositoryLayout`：按「组名 + 仓库名」从全局列表解析仓库主键并加载
    `repository/config`，找不到就渲染 404（替代 react-router 5 的路径正则约束）；
  - 左侧上下文导航逐字移植旧 `DrawerConfig`：8 个一级项（文件/提交/合并请求/分支/标签/
    成员/动态/设置）+ 设置下 4 个子项，含计数、展开 320px / 收起 88px、右缘折叠按钮；
  - 文件浏览页：分支/标签下拉、面包屑（`pathStack`）、目录行带最近提交与时间、
    blob 按类型分流（文本→代码高亮、图片→data URL、过大/不支持→提示）；
  - 代码高亮沿用旧版 `prism-coy` 主题与「整文件高亮后按行切分 + LRU 缓存」策略，
    语言清单与扩展名映射严格对齐（36 种）；
  - 修掉两个 Vite 下的 Prism 陷阱：`prism-tsx` 必须在 `jsx`+`typescript` 之后加载、
    `prism-vbnet` 依赖 `basic`（否则整页白屏）；`public/` 补上 favicon/manifest/sitemap；
  - 构建切片修正：`manualChunks` 只把框架层放进 vendor，prism 落到懒加载的页面块
    （vendor 139 → 98.8 KB gzip，首屏 142 KB gzip）。
- **新前端批 3（下）：提交 / 分支 / 标签 / diff**：
  - 提交列表：分支·标签下拉 + **服务端**关键字搜索 + 分页，行内展示作者（按提交邮箱
    反查仓库成员，未命中标记为临时用户）、时间与短哈希；
  - 提交详情：`commitDetail` + `fileChanges`，父提交为空时由后端补全空树 SHA；
    diff **直接用后端解析好的结构化数据**（`DiffFile[]`），前端只做行号推进与着色；
  - 分支/标签列表：保护标记、最近提交、时间、短哈希，名称可点进对应 rev 的文件树；
  - 新增 `i18n/supplement.ts`：旧词表确实缺失的 8 个文案单独成表，运行时覆盖自动生成的
    词表，避免手改生成文件被下次生成冲掉；
  - 视觉验证：commit 详情的 diff 三色实测命中 `#e6ffed`/`#ffeef0`/`#f1f8ff`，
    与旧主题 preset 完全一致；prism 独立成 34.5 KB gzip 异步块，只在看代码/diff 时加载。
  - 顺延：Blame 侧栏、视频类 ObjectViewer 富视图（原因与回归条件见计划 §10 批 3 下）。
- **新前端批 4（上）：个人设置三页**：
  - `/settings/{profile,email,sshkey}` 落地，左侧 4 项导航（个人资料 / 邮箱 / SSH Key / 通知）
    的 active 正则逐字移植旧 `DrawerConfig`；
  - 资料页：基本信息（`user/updateBasicInfo`）、头像上传（multipart 字段 `avatar`）、
    修改口令（`current`/`new`）、MFA（读取/设置/移除，移除前二次确认）；
  - 邮箱页：主邮箱更换（发验证码 → 换绑）+ 提交邮箱增删改验（4 个端点全部接通），
    主邮箱取 `user/info` 的 `emails[0]` 与旧版一致；
  - SSH Key 页：列表 + 新增 + 删除，并在前端预校验"公钥不含空格/换行"
    （后端同样拒绝，属 authorized_keys 注入防护）；
  - 补充词表扩到 21 条（旧词表缺失的成功/失败提示、验证状态等）。
  - 顺延：通知设置页（`offset` 按位语义待逐位核对）、MFA 二维码渲染、头像即时预览，
    以及仓库/组设置族（批 4 下半）。
- **新前端批 4（中）：通知设置 + 仓库基本信息 + 仓库成员**：
  - 通知设置页先核对了位语义再实现：`offset` 是位序号（1=合并请求、2=邮件），
    且 `u_notification_status` **位=1 表示关闭**，因此开关选中 = `(notification & bit) === 0`；
    下半页展示被拒收的仓库组/仓库并可一键恢复（级联由后端处理）；
  - 仓库基本信息：显示名 + 描述（`description` 始终发送 —— 后端缺参数会按 PHP 历史行为报 500）
    + 头像上传（multipart）；
  - 仓库成员：按邮箱添加、改角色、移除、转移所有权；操作列按 `REPO_CHANGE_MEMBER` 权限显隐，
    所有者行只显示徽标（需先转移所有权才能变更）。
  - 顺延：仓库默认分支、组设置三页、Webhook 六端点、保护分支四端点（批 4 剩余）。
- **新前端批 4（下）：仓库分支设置与 Webhook**：
  - 分支设置：默认分支切换（`repository/defaultBranch`，`old`+`branch`）+ 保护分支规则
    四条端点（列表/新建/更新/删除）；角色下拉只提供后端白名单接受的取值，避免
    "前端能选、后端报 0x0201"；
  - Webhook：列表 / 新建（不带 rwKey）/ 更新（带 rwKey，`active` 必须 1 或 2）/ 删除 /
    投递日志列表与日志原文；25 个事件与后端 `WebhookEventMatches` 的逗号精确匹配语义对齐
    （事件清单逐字移植旧 `WebhookEventConfig`）。
  - 顺延：分支/标签的新建删除动作、组设置三页、仓库高级（改名/删除）。
- **新前端批 4（尾）：组设置三页与高级设置**：
  - 新增组上下文外壳 `GroupLayout` + `GroupProvider`（组名 → group/config → 5 项导航），
    `/groups/{组名}/repositories` 只显示该组仓库、首 tab 文案改为「仓库」、新建按钮指向组内路径；
  - 组设置：`group/update`（displayName + description 必传）、头像上传、四个成员端点、
    `group/updateName` 与 `group/deleteGroup`；
  - 仓库高级：`repository/updateName` + `repository/deleteRepository`；
  - **抽出两个共用组件**（AGENTS §4「第二处重复即抽取」）：`MembersManager`（仓库成员与
    组成员共用，含角色下拉/移除/转移所有权）与 `AdvancedSettings`（改名 + 危险操作区），
    仓库成员页由 200 行降到 ~100 行，两处行为完全一致。
  - 顺延：分支/标签的新建删除动作（随分支页交互补）、组内 MR/动态（批 5）、
    改名后自动跳转（批 7 与"旧地址失效"提示统一处理）。
- **新前端批 5（上）：合并请求列表与详情**：
  - 列表：状态页签（待合并/已合并/已关闭，对齐后端 `MergeRequestStatus*`）+ 关键字搜索，
    行内展示 `#编号 标题`、`源分支 → 目标分支`、状态标签与更新时间；
  - 详情：`mergeRequestDetail` + 带 `mergeRequest` 参数的 `fileChanges`（比较目标是
    「目标分支当前提交 → 源分支当前提交」），diff 复用 `DiffViewer`；
  - 动作：评审（后端只要 `id`）、关闭、合并（先 `checkMergeType` 再 `mergeBranch`，
    提交信息默认取标题、压缩合并）；
  - 顺延：MR 创建页、评审人指派/移除 UI、MR 版本与提交列表端点、活动流、通知抽屉。
- **新前端批 5（下）：合并请求创建与通知抽屉**：
  - 创建页：`createMergeRequest`（源/目标分支、标题、描述、评审人），响应 `{id}` 是业务编号，
    成功后直接跳详情；
  - 通知抽屉：页头铃铛（未读角标）→ 四个分类列表 + 单条已读 / 全部已读 / 清空；
    后端要求 `all` 与 `id` 互斥，因此"全部"走 `all=1`、单条走 `id`；`data` 是 JSON 字符串，
    做容错解析后展示；
  - 验证缺口（如实记录）：抽屉需点击展开，headless 渲染只验证了铃铛/角标/接口数据，
    抽屉渲染并入批 7 的人工视觉回归；
  - 顺延：活动流（旧版 682 行、40 余种类型的渲染表，单独一轮移植）、评审人指派 UI、
    MR 版本与提交列表端点。
- **新前端批 6（上）：管理后台四页**：
  - AdminLayout + 5 项导航（仪表盘/用户/仓库组/仓库/系统设置），active 正则逐字移植旧配置；
  - 仪表盘：用量计数（用户/仓库/组）、CPU/内存/磁盘、nginx/codefever/sendmail 运行状态；
  - 用户管理：启用/禁用、设为/取消管理员、关闭 MFA、重置口令（管理员重置无需旧口令）；
  - 仓库组与仓库：只读列表 + 搜索 + 分页（`{count, list}` 信封）。
  - 顺延：系统设置页（`admin/config` 结构化表单 + YAML 预览）、文档中心（需新增两个 Go 端点）。
- **新前端批 6（下）：文档中心（含新增 Go 端点）与系统设置**：
  - **后端新增两个只读端点**：`GET /api/doc/tree?lang=cn|en`（菜单树，含二级）与
    `GET /api/doc/content?lang=&path=`（Markdown 原文）；两者复用 SSR 页面的
    `docRoot` / `extractMenu` / `withSubmenu` / `resolveDocPath`，**路径穿越校验与旧页同源**，
    注册为字面量路由（与 `/api/web-vitals` 同手法），文档为公开只读内容；
  - 前端文档页 `/doc/{lang}/{path}`：左侧菜单（支持二级）+ Markdown 渲染
    （`react-markdown` + `remark-gfm`，样式沿用内置 `markdown-body.css`），URL 保持可分享；
  - 管理后台系统设置：受控 JSON 编辑器（加载美化 → 编辑 → 本地校验 → 二次确认 → 保存），
    服务端做浅合并 + 结构校验 + 原子写 + 热更新；
  - 验证：Go 五项门禁（gofmt/build/交叉编译/vet/test）全绿，前端三项门禁全绿。
- **新前端批 5（尾）：活动流与评审人指派**：
  - 新增 `ActivityFeed`：把旧 `config/Activity.js`（682 行、41 个分支）压缩成一张
    「活动码 → 文案键 + 主体来源 + 细节类型」的码表，41 个码全部登记，未登记码走通用回退；
  - 主体/细节语义按旧实现对齐：分支/标签/MR/Webhook 类活动的 `{s}` 是「组名/仓库名」，
    分支名与 MR 编号进 `detail`；成员类活动的成员名进 `detail`；
  - 仓库动态与组动态两页共用 `ActivityFeed`；
  - MR 详情增加评审人管理（成员下拉指派 + 逐个移除，仅待合并状态可见）。
  - 未接：`mergeRequestVersionList`、`mergeRequestCommits` 两个只读端点（可选增强）。
- **新前端切换（批 7 一）：SPA 壳下发与镜像布局**：
  - 新增 `httpapi/spa.go`：`/user/login|register|verify|resetPassword`、`/doc/*` 的 Markdown 页
    与未知路径 404 **统一下发 SPA 壳**，壳内容只读一次缓存、响应 `no-cache`；
    **拿不到壳时回退旧 SSR**，因此这一步可回滚；壳路径按
    `CODEFEVER_SPA_INDEX` → `www/app` → `web/build` → `www/view/build` 顺序解析；
  - 镜像：`COPY web/build/ ./www/app/`（真实目录取代软链）、`web/public` 的
    favicon/manifest/sitemap 进 `www/`；`.dockerignore` 排除 `web/{node_modules,src,scripts}`；
  - nginx 行为不变（`/static/js|css` immutable 与 `try_files /app$uri` 已匹配新布局），仅更新注释；
  - **真机验证**：`docker build` + `docker compose up` 后，`/user/login` 与
    `/doc/cn/start/welcome.md` 均返回 SPA 壳并由 React 渲染（登录页文案、文档 Markdown 均正确），
    `/api/doc/tree|content` 返回真实文档数据，`/api/user/info` 未登录 403。
- **新前端切换（批 7 二）：删除旧前端与 Go 服务端渲染层**：
  - **移除**：`www/view/`（React 17 + Redux + MUI v4 + webpack 4，195 个跟踪文件）、
    `www/static/`（52 个文件，jQuery/Bootstrap 资源）、`www/app` 软链；
  - **移除**：Go 的 SSR 页面层 —— `internal/httpapi/{pages.go,pages_test.go,templates/}`、
    `testdata/pages/`、`cmd/{page-compare,pagediff}`、`internal/handlers/web_captcha.go`
    （captcha 全站零引用，属死代码）；`httpapi.Server.PageDataProvider` 一并移除；
  - 表单 AJAX 信封（`AjaxEnvelope`/`WriteAjaxJSON`）从 `pages.go` 拆到 `httpapi/ajax.go` 保留 ——
    登录/注册/二次验证/找回密码的 POST 仍返回 PHP 风格信封，契约不变；
  - `/user/*`、`/doc/*` 的 GET 只下发 SPA 壳（旧 SSR 回退分支删除）；未知控制器与
    未知路径统一交给前端 NotFound；`/lang/select` 参数错误改为回首页；
  - 镜像布局收敛：Dockerfile 用 `web/public` + `web/build` 生成 `www/`，不再 COPY 旧前端；
  - **文档同步**：`AGENTS.md` §4/§5/§6/§8.3/§9/§10 全部改为新前端口径（组件落位、设计令牌、
    三条前端门禁、图标与 SPA 壳陷阱）；`PROJECT_MAP.md` 旧前端条目改为已删除并追加人工备注；
  - **功能回归网**：API 契约基线（`internal/contract` + `testdata/contract*`）保留未动；
    Go 五项门禁 + 前端三项门禁 + `docker build` + 容器冒烟全绿。
- **新前端 UI 对齐修复（用户反馈驱动）**：
  - **按钮**：默认变体改回旧 MUI 的 `text`（此前一律实心蓝），列表主 CTA / 表单保存 /
    弹窗确认显式用 `contained`，次要动作回到 text —— 与旧版逐页一致；
  - **布局**：侧边导航改回"贴左、满高"（此前被挤在内容内边距里，实测侧栏区间
    旧 `0..318` vs 新 `0..318` 完全一致）；修掉侧栏布局里外层+子页两次内边距叠加
    （内容偏右 64px）；设置页区块由"浮起卡片"改为扁平"白底 + 1px 分隔线"；
  - **悬浮提示全局关闭**：新增 `web/src/config/ui.ts` 的 `UI_CONFIG.tooltipsEnabled=false`，
    `Tooltip` 不再渲染气泡（`aria-label` 保留），并去掉图标按钮的原生 `title`；
  - **补齐占位页**：全局 `/mergerequests` 与组内 `/groups/{名}/mergerequests` 接入真实数据
    （`repository/relatedMergeRequests` / `group/mergeRequests`），抽出 `MergeRequestRow` 三处共用，
    删除 `ComingSoon` 页面；
  - **页头**"仓库 / 仓库组"恢复为带文字的按钮（旧版行为）；
  - **个人资料页**按旧页字段补齐（用户名/邮箱/团队/角色）；
  - 验证手段沉淀：可从 git 历史取回旧产物、与新版并排跑同一套 mock，做像素级对比
    （方法记录在 `docs/FRONTEND_REWRITE_PLAN.md` §10 批 7（三））。
- **前端结构整理与尺寸校准**：
  - **组件目录化**：`components/unit/<Name>/index.tsx`（27 个）与
    `components/view/unit/<Name>/index.tsx`（18 个），一组件一目录；既有导入路径不变；
  - **样式集中**：全部组件/布局/页面样式迁到 `src/styles/`，由唯一入口 `styles/index.css`
    按固定顺序引入，组件内不再各自 import CSS；设计令牌仍只放 `theme/theme.css`；
  - **按旧版 computed style 校准**（用注入探针实测，不是目测）：text 按钮文字色改为
    `rgba(0,0,0,.87)`、padding `4px 24px 4px 8px`；图标按钮 34×32；输入框高度
    **32px**（上一轮误设为 52px）、`box-sizing: border-box`；contained 104×32、
    Tab 40px 高、表头 padding 0 16px —— 均与旧版一致；
  - **修复"仓库文件列表不显示"**：`repository/object` 在 `parent` 是提交时返回
    `type: 'commit'`（不是 `tree`），此前用 `type === 'tree'` 判断目录导致根目录永远空白；
    新增 `isDirectoryView()` 守卫并在 FileTree 三处使用，已按真实 API 契约验证
    （根目录 commit / 子目录 tree / 文件 blob 三种形态），并登记为 AGENTS §9 陷阱；
  - **修掉一个构建级 bug**：组件目录化时清理脚本误删了 `main.tsx` 的全局样式入口，
    导致构建不产出 CSS、页面退化为浏览器默认控件样式；已恢复并把
    "`build/static/js/index-*.css` 必须存在" 记为构建校验点。
- **Web 性能指标上报**：前端接入 `web-vitals`（CLS / FID / FCP / LCP / TTFB），
  通过 `sendBeacon` 发到新增的 `POST /api/web-vitals`，只写日志不入库。
  之所以要它：此前所有前端性能结论只能靠人工复测，无法在发布前后做同口径对比。
- **追加表保留期清理**：新增每日任务，清理已处理事件（30 天）、死信事件（60 天）、
  已读站内信及其孤儿内容行（90 天）、webhook 投递日志（90 天），全部按批 `LIMIT`，
  天数可用 `CODEFEVER_RETENTION_*` 覆盖。**待处理与处理中事件永不删除**，
  `cc_activities`（用户可见的仓库动态）默认不清理。
  此前除 `cc_sessions` 外没有任何清理，这五张表会无限增长。
- **新前端批 8：合并请求列表 / 看板 / diff 视图对齐旧版**：
  - **仓库内列表**（`/{组}/{仓库}/mergerequests`）：补回「全部」页签并**默认选中**
    （旧版 `tabValue: 0`，新版此前默认「待合并」）、页签数量角标（仓库内取
    `mergeRequestCount`；有关键字搜索时不显示）、asc/desc 排序（默认 desc）、
    每页 10 条 +「查看更多」追加下一页；
  - **搜索交互**：由"每次输入即时请求"改回**回车或点搜索按钮才触发**（旧版
    `searchMergeRequests()`），不再逐键打后端；
  - **行组件**：目标仓库显示**真实仓库名**（此前显示主键前 8 位十六进制）、源/目标分支
    变为可点链接（跳该仓库该分支的提交列表）、补「最后提交人」（源仓库成员 → 目标仓库
    成员 → 提交邮箱）与「编辑于」时间、补评审人头像组（已/未评审角标）；
  - **状态徽标配色**回到旧版口径：待合并=success、已合并=primary（旧版为 info，
    新版 `Tag` 无 info 色，取最接近的 primary）、已关闭=warning；
  - **看板**（全局 `/mergerequests`、组内 `/groups/{名}/mergerequests`）：补「全部」页签
    与角标（全局按 `relatedRepository` 命中的仓库累加、组内按同组仓库累加）、搜索、
    排序、分页，以及旧版本就有的「创建合并请求」入口；
  - **抽出共用件**（AGENTS §4）：`MergeRequestToolbar`（页签+搜索+排序+主操作）、
    `MergeRequestListBody`（行列表+空态+查看更多）、`MergeRequestReviewers`（评审人
    头像组，入参为「评审记录 + 成员」，MR 详情页可复用），三处页面共用一套；
  - **diff 视图**（`DiffViewer`）：补「所有文件 N 个 / +add / -del」汇总条、
    inline / side-by-side 布局切换、逐文件折叠面板、复制路径 / 查看 @hash /
    展开完整文件 / mode 变更提示；`files` 入参保持不变，既有调用点（提交详情、
    MR 详情）无需改动。
- **新前端批 8（续）：仓库浏览页与仓库设置对齐旧版**：
  - **仓库头**（新 `RepositoryHeader`）：补 Fork 按钮与 fork 数、HTTPS/SSH 克隆地址
    （可复制）、提交/分支/标签/**体积**/**未合并 MR**/成员 6 项计数条、fork 来源链接。
    其中体积与未合并 MR 数此前**完全没有**展示，数据本来就在 `repository/config` 里；
  - **分支 / 标签列表**（新 `RefListView` 共用件）：补默认分支标记、已合并标记
    `isMerge`、更新者、删除分支 / 删除标签、新建分支 / 新建标签入口，以及关键字搜索、
    排序、活跃页签与每页 10 条分页（旧版 `RefFilter` + `RefList` 的能力）；
  - **仓库动态**：补分类页签（全部/提交/MR/成员）、关键字搜索、每页 20 条翻页；
  - **提交列表 / 详情**：每页 10 → 20 条、按 `yyyy-MM-dd` 分组、补父提交展开列表与
    「浏览文件」跳转，并恢复子目录提交路径 `/{组}/{仓库}/commits/{rev}/*`；
  - **文件浏览**：补文件名过滤、README 自动渲染、submodule 条目、当前目录最后提交行、
    空仓库初始化引导、blame 模式，以及复制路径 / 复制代码 / Markdown 渲染切换 / 历史入口；
  - **仓库设置**：补名称 30 字 / 描述 200 字校验与字数计数、slug 校验、创建者下拉、
    删除仓库两级确认，以及成员页的「仓库成员 / 仓库组成员」分表、已删除成员过滤、
    自身权限粗粒度判定与 `0x0407`/`0x0408` 提示；
  - **抽出共用件**（AGENTS §4）：`FilterBar`（页签+搜索+排序的工具条）、
    `RefListView`（分支/标签共用）、`RepositoryHeader`、`ObjectViewer`、`MarkdownView`、
    `RepositoryEmpty`、`CommitRefs`。
- **新前端批 8（续）：管理后台对齐旧版**：
  - **用户管理**：补四个分类页签（全部/已开 MFA/未开 MFA/已禁用）、排序下拉、
    「添加用户」弹窗与**初始口令回显**（可一键复制）、设管理员前的**本人口令校验**
    （`admin/checkPassword`）、重置口令的两次输入校验；
  - **仓库组 / 仓库**：补图标列、创建时间列、排序、成员管理弹窗（邀请 / 移除 /
    设为所有者），仓库侧额外区分「组继承成员」（继承成员在仓库侧不可移除）；
  - **系统设置**：由裸 JSON 编辑器改为**结构化表单**（站点地址 / SSH 前缀 /
    发件人名称与地址 / 开放注册开关）并补 7 条前端校验；
  - **仪表盘**：统计卡可点击跳转、CPU/内存/磁盘按阈值配色、数值千分位与单位分离、
    图标换回旧版对应项（`psMeter` / `psNewFeature`）；
  - **抽出共用件**：`AdminMembersDialog`（组与仓库成员弹窗共用）、
    `useDebouncedValue`（三处列表页的 200ms 搜索防抖不再各写一遍）。
- **新前端批 8（续）：个人设置对齐旧版**：
  - **通知设置**：补仓库组 / 仓库级通知开关矩阵（组内仓库可折叠、每行角色徽标，
    角色低于「开发者」的条目开关禁用），并修正开关方向（此前恒传 `open: true`，
    无法关闭）；
  - **个人资料**：补 MFA 二维码与「更换 MFA 设备」入口，MFA 状态判定改回
    `user.mfaEnabled`（此前读后端不返回的 `mfa.status`，已开启 MFA 也显示设置表单）；
  - **SSH Key**：补公钥指纹列，支持粘贴整行 `ssh-rsa AAAA... user@host`
    （自动剥离注释并预填名称）；
  - **邮箱**：关联邮箱列表纳入主邮箱并标注，重发验证码加 120 秒倒计时。

### 性能

- **首屏 JavaScript 减半**：`1,699,347 B → 871,055 B`（-48.7%），gzip
  `454,176 B → 263,310 B`（-42%），前端产物目录 `7.9 MB → 1.8 MB`。三项来源：
  1. 路由级代码分割：34 个页面全部改为 `React.lazy`，按业务域切成 7 个 chunk
     （此前是 2 个文件、登录页也要下载后台与 Markdown 全家桶）；
  2. 依赖裁剪：`@pgyer/icons` 引入方式收敛到项目内的 `src/config/icons.js`
     （该包无 `sideEffects`，barrel 会把 154 个图标全部打进包，实际只用 59 个；
     该注册表按路径引入后统一包装成 FontAwesome 图标定义，是**唯一**允许的
     图标入口）；Prism 语言 56 → 36 种，
     并补齐了原本静默回落 markdown 高亮的 8 种语言；
  3. `react-markdown`（约 172 KB）改为按需加载，`github-markdown-css` 本地内置
     （不再从第三方 CDN 取样式）。
- **静态资源开启 gzip 与分级缓存**：此前全站无压缩、无 `Cache-Control`。
  现在 gzip 使 vendor chunk 传输 `719,449 B → 227,452 B`；静态资源按是否带内容
  哈希分档缓存（哈希产物一年 immutable，其余 7 天 + `stale-while-revalidate`，
  SPA 壳 `no-cache`）。二次访问不再重复下载。
- **SPA 脚本改为 `defer`**：此前两个 `<script>` 阻塞 HTML 解析直到 1.7 MB
  下载并执行完毕。
- **登录页关键路径瘦身**：`marked.min.js`（47.6 KB）改为仅文档页加载；
  登录页 LCP 大图由 147.9 KB 的 PNG 改为 WebP（30.5 KB，-79%）。
- **后端查询与连接池**：
  - 列表接口 `pagesize` 增加全站上限（经 `domain.ClampPaging`，上限 100）。
    此前只设下限，`?pagesize=20000` 会拼出无上限 `LIMIT`，而合并请求列表对每行
    都要跑一次 `git log` 子进程 —— 单个请求可拉起两万个进程；
  - 管理端用户列表由 `2 + 20×5 = 102` 条查询降为 **3** 条；
  - 通知 worker 的拒收过滤由「每收件人 2 次 `COUNT(*)`」降为 **1** 条批量查询
    （200 人群组事件：400 次 → 1 次）；
  - 鉴权不再重复查库：每个已登录请求对 `cc_sessions` / `cc_users` 的查询
    由各 2 次降为各 1 次；
  - 6 个迁移文件补齐 12 项索引（覆盖 9 张表），其中 `cc_users.u_tel` / `u_status`
    此前**完全没有索引**，手机号注册与按状态统计走全表扫描；
  - `git clone` 移出数据库事务，事务内跨连接查询清零。此前 25 路并发 fork 即可
    抽干连接池（`MaxOpenConns=25`），随后全站阻塞在等连接上。
- **MySQL 慢查询可观测**：开启 `slow_query_log`、`long_query_time` 10s → 0.2s。
  此前生产环境没有任何慢查询数据。

### 修复

- **公开 API 的方法与未知路径不再"静默放行"**：`/api/v1` 的路由此前是纯路径模式，
  方法完全不设限 —— 带合法令牌的 `POST /api/v1/user` 会正常返回 `200` 与数据，而文档
  写的是 `GET`；未命中的路径则回 Go 默认的 `text/plain` `404 page not found`，调用方
  按 `{message,url}` 解析会直接失败。
  - 四个端点统一加 `requireGet` 闸门：只放行 `GET` / `HEAD`，其余方法回
    `405` + `Allow: GET, HEAD` + JSON 错误体，且**方法判定先于鉴权**
    （否则调用方会把"方法不对"误读成"令牌有问题"）；
  - 命名空间根与未知子路径兜底为 JSON `404`（`/api/v1`、`/api/v1/`、`/api/v1/nope`）；
  - 为什么不用 Go 1.22 的 `"GET /api/v1/..."` 模式：它自带的 405 响应体是
    `text/plain`，会打破"错误永远是 JSON"的契约，要保住 JSON 反而更绕；
  - `doc/{zh-cn,en-us}/api/` 同步补充 `405` 与"路径不存在"的语义。

- **输入框自动填充时文字底部出现蓝色背景带（多邮箱页的邮件 / 密码输入框最明显）**：
  Chrome / Edge 的 UA 样式表对 `:-internal-autofill-selected` 强制
  `background-color: rgb(232,240,254) !important`（浅蓝）。本仓库的输入框是
  **两层结构** —— 外层 `.cf-field__box`（32px，有底色）+ 内层 `.cf-field__input`
  （19px，`background: transparent`）—— UA 的背景色只作用在 19px 的 input 盒子上，
  于是自动填充时**只有文字所在那一行变蓝**，上下内边距仍是原底色，
  视觉上输入区被割裂成三段。
  - 修复落在**组件层**（`web/src/styles/components.css`）而非某个页面，
    对 `.cf-field__input` / `.cf-field__select` 全局中和自动填充样式：
    用 `box-shadow: 0 0 0 1000px var(--cf-field-surface) inset`
    （inset 阴影绘制在背景色之上、文字之下）盖住 Chrome 的 `!important` 蓝底，
    并复位 `-webkit-text-fill-color` / `caret-color`，
    用 `transition: background-color 0s 600000s` 消除填充瞬间的闪烁；
  - 新增设计令牌 `--cf-field-surface`（`web/src/theme/theme.css`）作为
    **输入区表面色的唯一来源**，并按变体分别给出正确盖色：
    `--cf-field--underline` 用 `--cf-auth-panel-bg`（认证页输入框落在 auth 面板上）、
    `--cf-field--disabled` 用 `--cf-color-bg-main`；
  - 影响面：**全站所有输入框**（登录、注册、设置、多邮箱等），
    此前各页面对 Chrome 自动填充的视觉表现不一致。
- **admin 端点访问日志双写**：17 个管理端点此前被注册为 page route 并二次分发，
  每个请求走两层中间件，同一 `request_id` 出现两条完全相同的日志
  （请求体限制与 panic 恢复也各套两层）。根因（`contract.Parse` 先读完请求体
  导致 multipart 字段丢失）早已修复，现直接删除该补丁。
- **确认框队列「移除」从未生效**：`confirmation.remove` 分支用
  `typeof id === 'number'` 判断，而取到的是对象，条件恒为 false，是死代码；
  同时原实现用 `delete` 删除数组元素会留下空洞，后续 `shift()` 可能返回
  `undefined` 并在读取 `.data.title` 时抛异常。现改为按 key 过滤的不可变更新。
- **仓库头像更新事件里的 `from` 取值错误**：原先在事务内跨连接重读仓库行，
  读不到未提交的 `UPDATE`，导致事件中 `from ≠ to`，与既有语义（同一连接
  update 后重读，`from == to`）不符。
- **检索关键字未转义**：活动流与合并请求搜索的两处 `LIKE` 未转义通配符，
  `?keyword=%` 会退化为全表扫描。
- **管理端分页排序不稳定**：`sort` 未命中白名单时不拼 `ORDER BY`，导致
  `LIMIT/OFFSET` 无序扫描、翻页可能重复或漏行；现回落到创建时间并追加主键兜底。
- **前端若干渲染期问题**：三个 Redux reducer 对无关 action 也返回新对象，
  使每次 API 响应都触发全部 connected 组件重渲染；`Main.js` 在
  `shouldComponentUpdate` 里 dispatch 且无 `groupName` 的路由每次渲染都触发；
  `CodeViewer` 每次渲染都重新高亮整个文件；管理端列表的搜索防抖定时器
  在卸载时未清理；滚动监听未声明 `passive`。
- **新前端批 8：逐页核对旧版后修掉的 13 个功能回归**（核对报告见
  `docs/FRONTEND_FIELD_DIFF_2026-10-24.md`）：
  - **Webhook 新建/编辑必然 500**：请求体缺 `secret`，后端 `editWebhook` 因
    `rw_secret` 是 NOT NULL 列直接 `phpFatalError()`。补上 secret 输入框、列表列与
    URL/Secret 复制，Webhook 功能从"完全不可用"恢复；
  - **保护分支「允许合并角色」多给 DEVELOPER**：后端白名单只有
    NOBODY/OWNER/MAINTAINER，选 DEVELOPER 必得 `0x0201`。同时把"允许 Push/Merge 角色"
    从单角色标签恢复为旧版的**角色集合**语义（最低角色及以上全部可操作）；
  - **全局 MR 看板取数形状错误导致白屏**：后端返回的是
    `{mergeRequests, relatedRepository}` 对象而非数组，前端把整个 `data` 当数组，
    有数据时 `items.map` 抛 TypeError；
  - **仓库内 MR 列表 / 组内 MR 看板必然报错**：后端把 `sort` 当必填
    （既非 `asc` 也非 `desc` 即 `0x0201`），`group/mergeRequests` 还要求
    `keyword`/`page`/`pagesize` 必须显式存在（缺失即 500）；前端一个都没传；
  - **评审人数据结构不符**：后端返回 `{id, user, isReview}`，前端按
    `{id,name,email,icon}` 用 → 姓名空白；删除评审人传的是评审记录主键而非
    **用户主键**，后端按 `u_key` 匹配 → **静默无效**。同时后端拒绝删除已评审的
    评审人，前端补前置拦截与"已/未评审"徽标；
  - **合并冲突被当成功**：后端冲突时返回 `{"conflict": true}` 且 `code=0`，
    前端只在 `code≠0` 时抛错 → 提示"更新成功"并跳列表。改为先判 `conflict`，
    并补旧版的 4 步 git 冲突指引；
  - **i18n key 拼错**：`label.createAt`（词表只有 `label.createdAt`）→ 页面直接
    显示字面量；
  - **错误码无中文**：`0x0405`/`0x0406`/`0x040F` 等关键业务错误回落英文原文，
    改为按页面处理（旧版是各页 `ValidatorGenerator.codeValidator`，不是全局表）；
  - **Webhook 日志详情直接渲染对象**：后端 `Request`/`Response` 是 `any`，
    前端声明 `string` 并直接进 `<pre>` → React 抛
    "Objects are not valid as a React child"；
  - **新建仓库 / 新建组 / 全局新建 MR 指向不存在的路由**：页头与列表页的按钮
    点下去即 404。补齐 `/repositories/new`、`/repositories/fork/{id}`、
    `/groups/new`、`/groups/{组}/repositories/new`、`/mergerequests/new`、
    `/groups/{组}/mergerequests/new`、`/{组}/{仓库}/branches/new`、
    `/{组}/{仓库}/tags/new` 与 `/repositories/forklist/{id}` 共 9 条路由与对应页面；
  - **文件浏览进入子目录仍显示仓库根目录内容**：`parent` 恒传 rev，未取当前目录
    的 git 对象；
  - **全局新建 MR 页直接白屏**：表单复用了仓库上下文，但 `/mergerequests/new`
    不在 `RepositoryProvider` 内，`useRepository()` 抛错。改为可选上下文。

## 0.1.0 — 2026-10-23

CodeFever Next 的首个公开版本。基于
[PGYER/codefever](https://github.com/PGYER/codefever) 二次开发，
**后端已从 PHP 完整重写为 Go**。

### 破坏性变更

- **后端运行时下线 PHP**。镜像内不再存在 PHP 解释器、php-fpm、PHP 配置、PHP 入口脚本
  与 crond；`application/`、`system/`、`www/index.php` 等 PHP 源码已从仓库中删除。
  - `GET /api/admin/systemStatus` 的 `service` 段不再返回 `php` / `crond` 两个键
    （管理端仪表盘同步去掉了 PHP-FPM 与 Crond 两张卡片）。
  - `GET /api/repository/fileContent` 由 PHP 时代的**恒定 500** 修正为 **200**
    （PHP 侧方法名拼写错误导致的缺陷）。
- **会话存储位置变更**。PHP 会话在客户端 Cookie（CI 加密），Go 会话在服务端 `cc_sessions` 表，
  两者互不认得。**从旧版本切过来后需要重新登录一次**（不影响任何业务数据）。
- **数据库结构未变**，无需数据迁移。
- **数据库版本要求提高到 MySQL 8.0.x（或 MariaDB 10.6+）**。
  事件 worker 与 webhook 投递器用 `FOR UPDATE SKIP LOCKED` 领取任务行，
  该语法自 MySQL 8.0.1 起才支持；在 5.7 上这两条查询直接报 SQL 语法错误，
  会让**活动流、站内信、邮件通知与 webhook 静默失效**
  （症状是日志里每 2 秒刷一次 `SKIP LOCKED` 报错）。
  `docker-compose.yml` 的默认数据库镜像为 `mysql:8.0`，并追加
  `--default-authentication-plugin=mysql_native_password`
  ——MySQL 8 的默认插件 `caching_sha2_password` 不在应用容器内 MariaDB 客户端的插件目录里，
  不改这一项的话建库会直接失败。该参数在 MySQL 8.4+ 已移除，因此不支持 8.4/9.x。
- **站点品牌名变更**。站点名由 `CodeFever Community` 改为 `CodeFever Next`（仓库名 `codefever-next`）。
  Logo 与其余视觉资产保持不变。受影响的位置：
  - `config.yaml` 的 `email.name` 默认值（页面标题、邮件发件人名）——
    `/api/admin/config` 的 `data.email.name` 随之变为 `CodeFever Next`，
    生效契约基线 `backend/testdata/contract/admin.config.json` 已同步。
  - 服务端渲染页面的 `<meta name="description">`。
  - 通知邮件标题、保护分支拒绝提示、git 钩子提示语等用户可见文案。
  - 前端「关于 / 贡献代码」菜单项与 PWA `manifest.json` 的 `name`
    （`www/view/build` 产物已重建）。
  - 其余位置仍与 PHP 基线逐字节一致，详见 [`backend/testdata/README.md`](backend/testdata/README.md)。

### 新增

- Go 后端（`backend/`）：108 个 `/api/*` 端点 + 4 个 `/apiv1/internal` 回调；
  历史契约回归记录为 **123/123**，复跑方法见
  [`docs/IMPLEMENTATION_STATUS.md`](docs/IMPLEMENTATION_STATUS.md)。
- 43 条服务端渲染路由 + 5 个页面；历史对照记录为 **43/43**（品牌文案属有意变更）。
- git 钩子 Go 化（`bin/codefever-hook`）：替代原 PHP 钩子脚本，
  stdout / 退出码 / 请求体与 PHP 版**逐字节一致**，并修复了 PHP 侧 stdin 非阻塞读可能截断的问题。
- SSH 公钥同步 Go 化：Go 后端直接写 `authorized_keys`，下线了原 PHP 守护进程与 FIFO 协议。
- 事件 / 站内信 / 邮件 / webhook 投递由独立的 Go worker 承担，替代原 PHP 定时任务调度器。
- 口令哈希支持 `argon2id`（通过 `CODEFEVER_PASSWORD_HASH` 切换，默认保持 `legacy` 以兼容存量账号）。

### 变更

- **应用镜像从本地源码构建**（`Dockerfile`）；构建期仍需要两个基础镜像、Debian 归档源与
  Go module 代理，不是“完全离线”。
- `docker-compose.local.yml` 合并进 `docker-compose.yml`，编排文件新增 `build` 段，
  `docker compose up -d` 即可完成构建与启动。
- 编排层配置统一走环境变量：`CODEFEVER_HTTP_PORT`、`CODEFEVER_SSH_PORT`、`CODEFEVER_DB_PASS`、
  `CODEFEVER_IMAGE`、`CODEFEVER_MYSQL_IMAGE`、`CODEFEVER_BASE_RUNTIME`、`CODEFEVER_BASE_GO`。
- 数据卷改用命名卷（`db-data` / `git-storage` / `file-storage`），
  保证 hooks 软链在容器重建后依然有效；后续版本新增 `config-data`。
  **当前版本已改为仓库根目录的 `codefever-data/` 绑定目录**，上述命名卷仅适用于
  0.1.0 及更早版本。
- 0.1.0 时仓库侧不再保留 `git-storage/hooks/{main,pre-receive,post-receive,update}`
  的脚本本体；当前版本进一步把运行时目录全部交给 Dockerfile/entrypoint 创建，
  新裸仓库的钩子链走 `deploy/hooks`，旧 `misc/hooks` 保留兼容软链。
- **修复启动时的数据库迁移竞态**。`codefever-api` 与 `codefever-worker` 都会在启动时
  执行 `db.Migrate()`，并发时双方会同时查到「未迁移」并同时写入迁移记录，
  后提交的一方撞上 `Duplicate entry` 而退出（日志里表现为 worker 启动失败、
  由 supervisor 重启一次才恢复）。现在整个迁移过程在 MySQL 命名锁
  （`GET_LOCK`）下串行化，后到的进程拿到锁后重查即可正确跳过。
- 构建配方修复了若干上游阻塞点：
  - Debian buster 已 EOL，apt 源改指归档源（默认 `archive.debian.org`，可用
    `CODEFEVER_APT_MIRROR` / `--build-arg APT_MIRROR` 覆盖）；
  - Go 网关改为多阶段构建 + module 模式（上游的 GOPATH 模式在 Go ≥ 1.22 已不可用）；
  - 构建期不再 `git clone`，直接 COPY 本地源码，不需要 GitHub；
  - Windows 检出时恢复 git 符号链接；
  - 构建期统一行尾规范化，避免 `sh: $'\r': command not found`。
- 仓库卫生：移除未使用的第三方前端静态资源（约 23 MB → 3.7 MB），
  修正根目录 `package.json` 元数据，补齐 `.gitattributes` / `.editorconfig`
  等开源项目标准文件。
- **构建配方更名为 `Dockerfile`**（原 `Dockerfile.local`）。`Dockerfile.local` 这个名字
  是"上游有 `Dockerfile`、本地另建一份"的产物；本仓库已没有上游配方，直接用
  `Dockerfile` 更符合惯例，`docker build .` 即可。
- Go module 路径由 `codefever-community/backend` 改为 `codefever-next/backend`。
- 仓库地址统一指向 <https://github.com/AidenX0704/codefever-next>。

### 修复

- `ssh-gateway` 打不开日志文件时 panic —— forced command 以 `git` 用户运行、工作目录是
  `/home/git`，一旦日志文件属主不对或家目录不可写，**所有 SSH push / clone 全部失败**，
  且客户端只能看到 "Permission denied"。现改为打不开就退回 stderr，绝不 panic。
- `.dockerignore` 未排除 `www/view/node_modules/`，本地安装的前端依赖（数百 MB）
  会被整个打进镜像。
- 文档读取限制在语言根目录内，阻断 `../` 穿越与符号链接逃逸。
- 创建 tag 时对参数做转义，避免命令注入。

### 性能

| 并发 | QPS | 错误率 | p95 | p99 |
| --- | --- | --- | --- | --- |
| 8 | 615 | 0 | 23.8 ms | 36.1 ms |
| 16 | 866 | 0 | 33.6 ms | 40.4 ms |
| 32 | 965 | 0 | 65.4 ms | 82.7 ms |
| 64 | 978 | 0 | 141.6 ms | 177.7 ms |

QPS 在 ~970 处饱和。当次压测环境使用 MySQL 5.7，瓶颈是单库而非 Go 后端；
64 并发下 p99 仍只有 178 ms，没有出现排队塌陷。1 核 CPU / 1 GB 内存即可运行。

### 已知问题

- 404 页面引用了上游从未提供的图片 `/static/assets/images/404_pic.png`，会显示一个破图。
- SSH 公钥类型识别沿用上游正则，部分非 RSA 公钥可能识别不稳定；建议使用 RSA。
- 英文用户文档仍在逐步翻译，部分 `doc/en-us/` 页面暂时保留中文。

详见 [SECURITY.md](SECURITY.md)。
