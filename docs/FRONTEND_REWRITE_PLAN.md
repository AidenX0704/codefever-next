# CodeFever Next —— 前端重写方案（React 19 + TS + Vite，B 方案）

> 状态：**已批准，执行中**（2026-10-24 决策）
> 决策记录：
> 1. 样式路线取 **B 方案**：自研设计令牌（tokens）+ 自研组件库，**不引入任何 UI 框架**（MUI 全部退出）。
> 2. 认证页接受"失去无 JS 首屏"（服务端直出 → React 客户端渲染）。
> 3. 文档中心（`/doc/*`）**纳入**本次重写，`web_doc.go` 同时改造为 JSON API。
> 4. 旧 React（`www/view`）、旧非 React 前端（`www/static` + Go SSR 模板）**彻底删除**，不留兼容层。

本文件是本轮改造的**唯一事实源**：删除清单、令牌口径、分批顺序、验收口径以本文为准。
仓库级约定（提交纪律、CHANGELOG、项目地图、组件化、设计语言、门禁）仍以
`AGENTS.md` 为准；本次改造会连带修订 AGENTS 的若干条款，见 §9。

---

## 0. 一页结论

| 项 | 结论 |
|---|---|
| 可行性 | 可行。改造面清晰、可分批交付、每批可独立回滚 |
| 前端清理 | `www/` 下 **251 个 git 跟踪文件 → 删 248、迁走 3** |
| 后端清理 | **删 27 个文件 / ~3,600 行**，改写 4 个文件 |
| 后端新增 | **3 项能力**（文档 JSON API、SPA 壳下发、captcha 下线） |
| 新前端规模 | **40 个页面**（SPA 30 + 认证 4 + 文档 1 + 状态页 4 + NotFound 1） |
| 接口面 | **107 个 `/api` 动作**（user 24 / group 13 / repository 52 / admin 18），契约不变 |
| 工时 | **38~48 人日**（单人 8~10 周，两人 4~5 周） |
| 最大风险 | ①丢掉 PHP 保真快照回归网 ②视觉对齐没有量化基线 ③URL 正则语义变化 |

---

## 1. 改造前的全貌（现状基线）

### 1.1 两套渲染栈并存

| | 服务端渲染（删除） | SPA（重写） |
|---|---|---|
| 实现 | Go `html/template` + Bootstrap 4.4.1 + jQuery 3.5 + jquery-validate + intlTelInput + toastr + marked | React 17.0.2 + Redux 7 + MUI v4（`withStyles`/JSS）+ react-router 5 |
| 页面 | `/user/login`、`/user/register`、`/user/verify`、`/user/resetPassword`、`/doc/*`、`/lang/*`、`/captcha/*`、404/状态页 | `/repositories`、`/groups/*`、`/{group}/{repo}/*`、`/settings/*`、`/admin/*`、`/userInfo` |
| 代码量 | 模板 15 文件 / 1,329 行；handler 1,272 行 | 146 文件 / 29,928 行 JS |
| 资源 | `www/static/`（52 文件 / 4.2 MB） | `www/view/build/`（30 文件 / 1.78 MB，经 `www/app` 软链暴露） |

### 1.2 请求链路（nginx `backend/deploy/docker/vhost-go.conf-template`）

```
*.git                      → 127.0.0.1:27555  codefever-http-gateway（git-http-backend）
/static/js|css（^~）        → www/static/… 优先，回退 /app$uri（= www/view/build/…），immutable 1 年
/static/（^~）              → 同上，7 天 + stale-while-revalidate
/user|file|doc|lang|captcha|api… → 127.0.0.1:9001 Go（SSR 页面 + JSON）
其余（/repositories 等）     → try_files /app/index.html（SPA 壳，no-cache）
/                          → Go home#index：登录 302 /repositories，未登录 302 /user/login
```

### 1.3 接口面（新前端唯一数据源，契约不可变）

- 统一信封：`{ code, message, data, extra }`（`backend/internal/contract/envelope.go`）
- 错误码语义：`0x0101` 登录失效→跳登录；`0x0105` 动作不存在；`0x0106` 无权限；
  `0x0200/0x0201/0x0202` 资源为空/输入错误/超范围；`0x0301/0x0302` 无此组/仓库。
- 请求形态（必须复刻，见 `www/view/src/data_providers/main.js`）：
  - `POST` 用 **multipart/form-data**；嵌套对象编码为 `key[sub]` / `key[]`；
    键名后缀 `_SAFE` / `_BINARY` 会被剥掉（二进制字段标记）。
  - `GET` 追加 `requestTime=<ms>`。
  - 头：`codefever-end-env: codefever-app`、`codefever-end-lang: <zh-cn|en-us>`、`Accept: application/json`。
- 动作统计（实测）：`user` 24 / `group` 13 / `repository` 52 / `admin` 18 = **107**；
  另有 `/file/fetch/{storage}/{key}`、`/api/web-vitals`。
- **死端点**：SPA 仍在调 `user/getCode`、`user/confirmTel`，后端从未实现（永远 0x0105）→ 重写时删除。

---

## 2. 删除清单（前端，精确到文件）

`git ls-files www` = **251**。目标：删 **248**，保留并迁移 **3**。

| 路径 | 文件数 | 规模 | 处置 |
|---|---:|---|---|
| `www/view/src` | 146 | 29,928 行 JS | 删除（仅作行为/视觉参考，代码不搬） |
| `www/view/build` | 30 | 1.78 MB | 删除（入库产物改由 `web` 产出） |
| `www/view/config` | 10 | webpack/CRA 配置 ~4.6k 行 | 删除 → `web/vite.config.ts` |
| `www/view/scripts` | 3 | start/build/test | 删除 |
| `www/view/public` | 3 | favicon/manifest/index.html | 迁至 `web/public` + `web/index.html` |
| `www/view/{package.json,yarn.lock,.gitignore}` | 3 | lock 576 KB | 删除（新 lock：`web/package-lock.json`） |
| `www/app`（软链 → `view/build`） | 1 | — | 删除；镜像内软链改指 `web/build` |
| `www/static/vendor` | 27 | 4.0 MB | 全删（jQuery / Bootstrap / FontAwesome CSS / intlTelInput / toastr） |
| `www/static/css` | 5 | 41 KB | 全删（`markdown.css` 26 KB 的规则并入新样式层） |
| `www/static/script` | 4 | 52 KB | 全删（`marked` → `react-markdown`；`common.js` 的区号选择/滚动动画 → hooks） |
| `www/static/images` | 16 | 180 KB | **全部迁入 `web/src/assets/images`**（无一多余，见 §2.1） |
| `www/{favicon.ico,manifest.json,sitemap.xml}` | 3 | — | 迁入 `web/public` |

### 2.1 复用的 16 张素材（"使用相同素材"的落地口径）

| 用途 | 文件 |
|---|---|
| SPA 空态（`EmptyListNotice` → `EmptyState`） | `repositories-empty.png`、`branches-empty.png`、`tags-empty.png`、`commits-empty.png`、`mergerequests-empty.png`、`branch-rules-empty.png` |
| 头像/品牌 | `default_avatar.png`、`logo.png`、`logo-ico.png`、`logo-community.png`、`header-codefever-logo.png` |
| 语言切换 | `language_chinese.png`、`language_english.png` |
| 文档中心 | `logo-community-doc.png` |
| 登录页视觉（LCP 元素） | `activity_pgyer_codefever_.png`（144 KB）+ `activity_pgyer_codefever_.webp`（**改为只发 WebP**） |

图标：`@pgyer/icons` 的 154 个图标数据（raw array + `helpers/pgyerIcon.js` 包装）**内联进
`web/src/assets/icons/`**，保留包装层语义（直接 `import x from '.../icons/x'` 拿到的是数据数组，
必须经包装组件渲染，见 AGENTS §9）。

---

## 3. 删除 / 改写清单（后端）

### 3.1 直接删除（27 文件 / ~3,600 行）

| 文件 | 规模 |
|---|---|
| `backend/internal/httpapi/templates/*.html` | 15 文件 / 1,329 行 |
| `backend/internal/httpapi/pages.go` | 505 行（11 个 `Render*` + 113×2 条语言表） |
| `backend/internal/httpapi/pages_test.go` | 531 行 |
| `backend/testdata/pages/php-*.html` | 6 文件 / 72 KB |
| `backend/cmd/page-compare` | 344 行 |
| `backend/cmd/pagediff` | 70 行 |
| `backend/internal/handlers/web_captcha.go` | 160 行（全站零引用，已是死代码） |

### 3.2 保留并改写（4 文件）

| 文件 | 改法 |
|---|---|
| `internal/handlers/web_user.go` | 删 4 个 GET 的 HTML 渲染分支（≈ −120 行）；**POST 逻辑与 JSON 信封原样保留**（`writeAjax`） |
| `internal/handlers/web.go` | 保留 `home#index` 302 语义；`handleNotFound` 改为下发 SPA 壳 |
| `internal/handlers/web_doc.go` | 改造为 JSON API（目录解析、`doc/` 路径穿越校验、`^\w{32}\.png$` 配图逻辑全部保留） |
| `internal/handlers/web_lang.go` | 保留（`codefever_lang` Cookie） |

### 3.3 新增能力（3 项）

1. `GET /api/doc/tree?lang=cn|en` → 菜单树（复用 `web_doc.go` 现有解析）。
2. `GET /api/doc/content?lang=&path=` → Markdown 原文 + 相邻页导航。
3. SPA 壳下发：`GET /user/login|register|verify|resetPassword`、`/doc/*`、404 → 返回
   `web/build/index.html`（`Cache-Control: no-cache`）；`POST` 分支保持原 JSON 信封。

---

## 4. 设计令牌（B 方案基石，逐项来自现有主题源码）

> 出处：`www/view/node_modules/@pgyer/mui-theme-preset/{style,component,color-presets}`
> ＋ `www/view/src/config/colors/theme1.js` 的覆盖值。MUI v4 默认 `spacing = 8px`、`shape.borderRadius = 4px`。
> 这些值**只能**存在于 `web/src/theme/theme.css`（`:root` 变量），TS 侧只引用 `var(--cf-*)`。

### 4.1 颜色

| 令牌 | 值 | 备注 |
|---|---|---|
| `--cf-color-primary` | `#3455db` | theme1 覆盖 preset 的 `#0035ec` |
| `--cf-color-primary-light` / `-dark` / `-lighter` | `#4d72f2` / `#002dbd` / `#e5ebfd` | hover / active / 文本按钮底色 |
| `--cf-color-secondary` | `#319ffc`（light `#5ab2fc`，dark `#226fb0`，lighter `#eaf5ff`） | 与 info 同值 |
| `--cf-color-success` | `#25bd21`（light `#50ca4d`，dark `#198417`，lighter `#ebfaea`） | |
| `--cf-color-warning` | `#f6b811`（light `#f7c640`，dark `#ac800b`） | |
| `--cf-color-danger` | `#ea4848`（light `#ee6c6c`，dark `#a33232`，lighter `#fdecec`） | MUI 的 error |
| `--cf-color-border` | `#dee2ec` | 表格/输入框/分割线 |
| `--cf-color-text` | `#2f354d` | 主文本 |
| `--cf-color-text-light` | `#7a859e` | 次要文本、按钮文字、图标 |
| `--cf-color-text-lighter` | `#b1b6c3` | caption |
| `--cf-color-bg` | `#ffffff` | 卡片/输入框/表格底 |
| `--cf-color-bg-main` | `#F1F3F7` | 页面底、表头、抽屉、AccordionSummary |
| `--cf-color-bg-dark` | `#E9EDF4` | 表格行 hover |
| diff 专用 | addition `#e6ffed` / deletion `#ffeef0` / label `#f1f8ff`；深色档 `#cdffd8`/`#ffdce0`、`#acf2bd`/`#fdb8c0`、`#dbedff` | 见 diff 组件 |
| `--cf-color-backdrop` | `rgba(75, 79, 74, 0.4)` | 弹窗遮罩 |

### 4.2 字体与字号

- `--cf-font-family`：`PingFang SC, Helvetica Neue, Hiragino Sans GB, Microsoft Yahei, WenQuanYi Micro Hei, Roboto, sans-serif`
  （theme1 追加 `system-ui, -apple-system, "Segoe UI", Arial, "Noto Sans", "Liberation Sans"` 等回退栈，取 theme1 全量）。
- `--cf-font-size` 基准 **14px**；`h1~h3` 20/700；`h4~h6` 16/700；`subtitle` 14/700；
  `body` 14/400；`caption` **14/400**（theme1 把 preset 的 12 改为 14；`text.lighter` 色）；
  `small` 12（ListSubheader / Tooltip）。
- 代码：`SFMono-Regular, Consolas, Liberation Mono, Menlo, monospace` 12px，`white-space: break-spaces`。

### 4.3 间距与尺寸

`--cf-space` = **8px**；`spacing(n) = calc(var(--cf-space) * n)`。

| 组件 | 尺寸 |
|---|---|
| Button | 高 `32px`（spacing 4）、`min-width 104px`（spacing 13）；small `min-width 64px`；contained/outlined padding `2px 24px`；text padding `4px 8px`、`min-width 16px`；圆角 4px |
| Button（text）hover/active/focus | 文字 `primary`，底色 `primary-lighter` |
| Button（outlined） | 边框 `border`、文字 `text-light`；hover 边框+文字 `primary-dark`；active `primary-light` |
| 输入框 | 内边距 `6.5px 16px`；行高/高 `19px`；字号 14；圆角 4；边框 `border`；多行同 padding |
| 表格 | 外框 `1px solid border`、圆角 **4px**（spacing 0.5）、`overflow:hidden`、`border-collapse: inherit` |
| 表格单元格 | 高 **40px**（spacing 5）、上下 padding 0、`text-align:left`、末列右对齐、底边框 `1px solid border`；body 末行无边框 |
| 表头行 | 底色 `bg-main`，hover 不变色 |
| 表格行 hover | `bg-dark` |
| Tab | 高 `40px`、padding `0 24px`、sm 以上 `min-width: 0`；选中加粗 600、文字 `text`；未选 `text-light` |
| Dialog 标题 | padding `20px 24px`、居中 |
| Menu | paper `max-height: 50%`、`hr` margin `8px 0`；MenuItem 高 **32px**（theme1 覆盖） |
| 抽屉 | paper 底色 `bg-main`、padding `8px 24px`、左侧停靠右边框 `1px solid border` |
| 头像（方形） | 圆角 4px |
| 列表项 | `min-width: 160px`；图标 `min-width: 32px`、色 `text-light`；分组头 12px、opacity .8、行高 32px |
| 分页项 hover | 文字 `primary`、底色 `primary-lighter` |

### 4.4 阴影（仅 4 档，其余一律 none）

| 令牌 | 值 | 用途 |
|---|---|---|
| `--cf-shadow-default` | `0px 8px 16px 0px rgba(63, 70, 62, 0.20)` | Paper elevation2/8、Popover |
| `--cf-shadow-card` | `0px 2px 8px 0px rgba(122, 133, 158, 0.20)` | 卡片 |
| `--cf-shadow-card-hover` | `0px 10px 30px 0px rgba(122, 133, 158, 0.20)` | 卡片 hover |
| `--cf-shadow-bar` | `0px 2px 10px 0px rgba(122,133,158,0.15)` | Toast/Snackbar |

Tooltip：底色 `text`、12px/13px、padding `6px 16px`、12×12 旋转 45° 的箭头。

---

## 5. 目标结构（`web/`）

```
web/
├── index.html  vite.config.ts  tsconfig.json  tsconfig.node.json  eslint.config.js  package.json
├── public/                     favicon.ico  manifest.json  sitemap.json(sitemap.xml)
└── src/
    ├── main.tsx  App.tsx  router.tsx
    ├── theme/                  tokens.ts（var(--cf-*) 的类型化引用 + spacing()）  theme.css（唯一字面值处）  global.css
    ├── components/unit/        自研组件库（无 UI 框架）：Button Input Textarea Select Checkbox Switch Radio
    │                           Form/FormItem Table(+Pagination/Empty/Loading) Dialog Confirm Tabs Tooltip
    │                           Menu/Popover Toast(+Provider) Avatar Tag Card CopyButton Loading EmptyState
    ├── components/layout/      AppShell Header Drawer PageContainer SettingsLayout AdminLayout AuthLayout
    ├── components/view/        页面级组件（按业务域：auth/ repository/ group/ mr/ admin/ settings/ doc/）
    ├── components/view/unit/   业务域共用件（RepoCard CommitRow MemberItem MRRow RefSelector …）
    ├── api/                    client.ts（信封/错误码/FormData 编码）  user.ts group.ts repository.ts admin.ts doc.ts
    ├── features/<domain>/      TanStack Query hooks（承接旧 reducers 的缓存职责）
    ├── hooks/                  useDebounce useConfirm useCopy usePagination useScrollTrigger useZodForm
    ├── i18n/                   zh-cn.ts en-us.ts（合并旧 `lang/` 17 文件 + Go 侧 113×2 条词表）
    ├── config/                 constants.ts（扩展名→高亮语言、状态枚举、UAC）、routes.ts
    └── assets/                 images/（16 张）  icons/（154 个 pgyer 图标数据）
```

### 5.1 AGENTS §4 落位表的对应关系（改名，不是消失）

| 旧 | 新 | 说明 |
|---|---|---|
| `data_providers/` | `api/` + `features/` | 传输层（api）与缓存层（TanStack Query）分离 |
| `reducers/` | `features/` | 服务端状态不再进全局 store |
| `helpers/` | `helpers/` + `hooks/` | 纯函数 / 有状态的 React 逻辑分开 |
| `config/colors/theme1.js` | `theme/theme.css` + `theme/tokens.ts` | 令牌唯一来源 |

---

## 6. 分批执行计划

| 批 | 内容 | 页面 | 估时 | 依赖 | 状态 |
|---|---|---:|---:|---|---|
| 0 | 脚手架（Vite+TS+ESLint）、`theme.css`/`tokens.ts`、首批 unit 组件、`api/client.ts`、路由骨架、设计系统预览页 | — | 4~5 d | — | **已完成（2026-10-24）** |
| 1 | 认证 4 页 + 状态/错误页 + `/` 跳转（含 Go GET→壳、captcha 下线） | 9 | 3~4 d | 0 | **前端已完成 2026-10-24；Go 侧顺延批 7** |
| 2 | AppShell（Header/Drawer/通知/确认框）+ 仓库列表 + 组列表 + 用户信息 | 6 | 4~5 d | 0 | **已完成 2026-10-24（通知铃铛顺延批 5）** |
| 3 | 仓库浏览主链路：文件树/ObjectViewer/CodeViewer/Blame/提交列表/提交详情/分支/标签 | 8 | 8~10 d | 2 | **已完成 2026-10-24**（Blame 侧栏与视频类 ObjectViewer 顺延） |
| 4 | 仓库设置 + 组设置 + 成员 + Webhook + 保护分支 + 高级 | 8 | 6~8 d | 2 | **已完成 2026-10-24**（分支/标签的新建删除动作顺延到批 3 分支页交互） |
| 5 | 合并请求 3 页 + 活动流 | 4 | 5~6 d | 3 | **已完成 2026-10-24**（MR 版本/提交列表端点仍未接） |
| 6 | 管理后台 5 页 + 文档中心（含新 API）+ 通知设置 | 6 | 5~6 d | 2 | **已完成 2026-10-24** |
| 7 | 收尾：删 `www/`、删 Go SSR、删静态资源、改 nginx/Dockerfile/AGENTS/PROJECT_MAP/CHANGELOG、全量视觉回归 | — | 3~4 d | 全部 | **已完成 2026-10-24** |

**工时合计 38~48 人日。**

### 6.1 每批的完成定义（DoD）

1. 该批页面全部可在 `vite dev` 下连通真实后端（或 mock）走通主流程；
2. `tsc --noEmit` + `eslint` + `vite build` 全绿，产物入库（`web/build`）；
3. 该批页面与旧版截图逐一比对，差异（若有）在本文 §8 白名单登记；
4. `CHANGELOG.md`、`PROJECT_MAP.md` 同步；
5. 提出 commit message 并等待确认（AGENTS §1）。

---

## 7. 验收口径

- **视觉**：40 页 × 3 视口（1440 / 1024 / 390）截图 diff；布局位移 0，颜色/字号/圆角/间距 0 差异，例外必须登记。
  基线来源：改造前用当前 v4 应用逐页截图存档（`docs/assets/frontend-baseline/<batch>/`）。
- **功能**：107 个动作全部接通；`internal/contract` 契约测试保持绿；`cmd/smoke-write` 写路径通过。
- **性能**：首屏 gzip ≤ 200 KB（自研组件库无 JSS 运行时，预期显著低于现状 264 KB）；
  LCP/CLS/INP 由 `/api/web-vitals` 采集，与改造前对比。
- **门禁**：`npm run typecheck && npm run lint && npm run build` + `go test ./...` + `pwsh -File scripts/verify.ps1`。

---

## 8. 已知代价与偏离登记（AGENTS §7）

| # | 偏离/代价 | 原因 | 回归条件 |
|---|---|---|---|
| 1 | **放弃 PHP 逐字节保真基线**（删 `testdata/pages`、`pages_test.go`、`page-compare`、`pagediff`） | SSR 页面被 React 取代，基线对象不复存在 | 永久；功能回归改由 API 契约测试 + 截图比对承担 |
| 2 | 认证页**失去无 JS 首屏**（原为服务端直出表单） | 客户端渲染 | 若要求首屏可用，需引入 SSR/预渲染（另立议题） |
| 3 | **URL 正则语义变化**：react-router 8 不支持路径正则 | v6+ 移除了 path 正则 | 组名/仓库名/rev 的合法性校验下沉到页面与 API 层，非法时渲染 404 页 |
| 4 | 视觉值集中在新 `theme.css`，**不再是 MUI 主题** | B 方案（自研 tokens） | 永久 |
| 5 | 旧文案/常量层（`lang/`、`config/Constants.js`、`helpers/` 纯函数）**逐字搬运**而非重写 | 与组件无关，重写只增加漂移风险 | 永久 |
| 6 | `www/view/build` 入库流程迁移到 `web/build` | 新构建体系 | 永久 |

---

## 9. 需要同步修改的仓库级文件

| 文件 | 改动 |
|---|---|
| `AGENTS.md` §4 | 落位表 `www/view/src/...` → `web/src/...`；`data_providers/` → `api/` + `features/` |
| `AGENTS.md` §5 | 事实标准由 "MUI v4 + `withStyles`" 改为 "`theme/theme.css` 令牌 + `components/unit` 组件库"；禁止硬编码的适用范围不变 |
| `AGENTS.md` §8.3 | 门禁由 `yarn build` 改为 `npm run typecheck && npm run lint && npm run build`（产物 `web/build` 入库） |
| `AGENTS.md` §9 | 废弃"PHP 快照保真"条目标注；新增"图标数据必须经包装组件渲染"（沿用）；`ERR_OSSL_EVP_UNSUPPORTED` 条目随 webpack 4 一起退役 |
| `AGENTS.md` §10 | 自查清单同步前端门禁 |
| `PROJECT_MAP.md` | 新增 `web/` 段落；`www/view`、`www/static` 标注 `（已废弃 2026-xx-xx：前端重写）`，不删历史 |
| `CHANGELOG.md` | 每批一条用户可见变更 |
| `README.md` / `docs/*.md` | 30+ 处 `www/view` 引用同步 |
| `Dockerfile` / `vhost-go.conf-template` | `COPY web/`、软链 `www/app → web/build`、`/static/js|css` 缓存分支对齐 Vite `assetsDir` |
| `scripts/verify.ps1` | 增加前端门禁（或新增 `scripts/verify-web.ps1` 并在 CI 调用） |

---

## 10. 执行记录

### 批 0 —— 脚手架与设计令牌（完成 2026-10-24）

工具链实测（`npm` 走 npmmirror，版本按 registry 当前稳定版 pin）：

| 依赖 | 版本 |
|---|---|
| react / react-dom | 19.3.1 |
| react-router | 8.5.0 |
| vite / @vitejs/plugin-react | 8.3.5 / 6.1.2 |
| typescript | 6.0.3 |
| eslint / typescript-eslint | 10.13.0 / 8.72.0 |

已落地：

1. `web/{package.json,tsconfig.json,vite.config.ts,index.html,eslint.config.js,.gitignore}`；
2. `src/theme/{theme.css,tokens.ts,global.css}` —— 令牌唯一来源 + 类型化引用；
3. `src/components/unit/` 21 个原子件 + `unit.css`（只引用 var）；
4. `src/api/client.ts`（信封、错误码、multipart 编码、`0x0101` 跳登录）；
5. `src/components/layout/AppShell.tsx` + `layout.css`（批 2 补 Drawer/通知）；
6. `src/router.tsx`（全部 lazy，`/__design` 预览页，其余路由占位）；
7. `src/assets/images/` 迁入 16 张素材（含登录页 WebP）+ `index.ts` 类型化图名枚举。

门禁结果：

| 门禁 | 结果 |
|---|---|
| `npx tsc --noEmit` | 通过 |
| `npx eslint .` | 通过（0 error / 0 warning） |
| `vite build` | 通过，**187 ms**（旧 webpack 4 为 60~90 s） |
| SPA 回退 | `vite preview` 下 `/` 与 `/__design` 均 200，`/static/js/*` 均 200 |

产物体积（骨架，尚不含业务页面，仅作基线）：

| 项 | 数值 |
|---|---|
| `vendor-*.js`（React + React Router） | 312.7 KB raw / **98.4 KB gzip** |
| 全部 js + css | 365.7 KB raw / **115.9 KB gzip** |
| 首屏 `index.html` | **0.56 KB gzip** |
| 图片 | 按需加载，不阻塞首屏 |

对比旧前端：旧首屏 264 KB gzip（其中 vendor 226.6 KB，MUI v4 + JSS 占大头）；
新骨架的 vendor 只有 98.4 KB gzip，差值即"去掉 MUI/JSS/formatjs/date-fns"的直接收益。

遗留到后续批次：`docs/` 与 `AGENTS.md` 的引用改写（批 7）、`Dockerfile`/nginx 切换（批 7）、
TanStack Query / i18n / icons 内联（批 2 起按需引入）。

### 批 1 —— 认证四页（前端完成 2026-10-24；Go 侧顺延至批 7）

已落地：

1. `src/i18n/{zh-cn.ts,en-us.ts}` + `i18n/index.tsx`：从 `backend/internal/httpapi/pages.go`
   的 `langZhCN` / `langEnglish` **自动抽取 54 键 × 2 语言**（脚本 `scripts/extract-lang.mjs`，
   逐字搬运，零手改）；`I18nProvider` 与 api client 共用 `localStorage['lang']`，
   请求头 `codefever-end-lang` 随之切换。
2. `src/api/client.ts` 增补 `postForm()`（`application/x-www-form-urlencoded`）与
   `AjaxResult` / `extraField()` —— 认证端点用的是旧服务端表单页的 `{code,message,extra}` 信封，
   **不是** `/api/*` 的 `{code,message,data}` 信封，且 `code` 是业务码（1..8、100），不能当异常抛。
3. `src/api/auth.ts`：`login / register / verifyTwoFA / requestResetCode / requestResetTicket /
   applyNewPassword / logout`，端点与字段逐字对齐旧模板。
4. `src/components/view/auth/AuthLayout.tsx` + `auth.css`：卡片 1184×644、左图 644 宽（WebP 优先）、
   表单列 360 宽、≤1184px 隐藏左图（宽 80%/上限 540px）、≤768px 90%、≤576px 内边距 60/20。
5. 四个页面：`pages/auth/{Login,Register,Verify,ResetPassword}.tsx`，客户级校验规则、错误文案、
   成功/2FA/失效三分支与旧模板一致；重置口令页保留 `?mode=reset&key=` 双模式与 120 秒重发倒计时。
6. `router.tsx`：`/user/{login,register,verify,resetPassword}` 为**不套 AppShell** 的独立全屏页。

门禁与验证：

| 项 | 结果 |
|---|---|
| `tsc --noEmit` / `eslint .` / `vite build` | 全绿（构建 172~196 ms） |
| headless Chrome DOM 断言 | 4 页文案全部命中（欢迎登录/欢迎注册/找回密码/重置密码、按钮、占位符、页脚链接） |
| 截图几何校验（1440 / 1100 / 500 视口） | 卡片 **1184×644 / 540×644 / 450×644**，与旧 CSS `width:80%;max-width:540px`、`90%`、固定高 644 **完全一致**（测量含 11px 阴影扩散） |
| 背景与主色 | 背景 `#f7f8fa`（旧 `.login-body`）✓；提交按钮 `#0035EC`（旧 `.btn-self-primary`）✓ |
| 首图 | `<picture>` WebP 实际加载（hero 区域 2990 种颜色，非纯色）✓ |
| 产物体积 | vendor 98.7 KB gzip；全部 js+css 126.3 KB gzip；四个认证页各自 0.9~1.6 KB gzip |

偏离与顺延（AGENTS §7 登记）：

| # | 项 | 原因 | 回归条件 |
|---|---|---|---|
| 1 | **Go 侧"GET→SPA 壳"与 `/captcha` 下线顺延到批 7** | 若现在改，镜像内的 `www/app` 仍指向旧 `view/build`，会出现"壳指向未入镜产物"的悬挂状态 | 批 7 与 `Dockerfile`/nginx 同批切换 |
| 2 | 状态页从 4 张收敛为 **0 张新增路由** | 实测 Go 版 `RenderSuccessMessage` / `RenderDisableMessage` **已无调用点**（仅 doc 404 与 `/lang` 参数错误用到），404 页已存在 | 永久 |
| 3 | 不实现密码强度条（旧 `.pwd-strength`） | 其 JS（`checkPwdStrength`）与 CSS 在旧仓库中缺失（`www/static/assets/*` 全为缺失文件），当前部署下该控件本就无样式、无逻辑 | 若产品要求，需先定义强度规则 |
| 4 | `AllowRegister` 开关的"去注册"链接始终显示 | 旧 SSR 由服务端决定是否渲染；新前端暂无公开配置接口，注册关闭时由服务端 302 拦截 | 批 7 增加公开 site-config 接口 |
| 5 | 语言偏好沿用 `localStorage['lang']`，不复刻 `codefever_lang` Cookie | 该 Cookie 为 HttpOnly，前端读不到；SSR 时代由服务端读取 | 同上，需公开接口 |

另：本机未起 Go 后端时 `vite preview` 会继承 `server.proxy`，`/user/*` 直接 502，
因此新增 `web/scripts/serve-preview.mjs`（SPA 回退静态服务器）作为视觉验证工具，不进生产。

### 批 2 —— AppShell + 仓库列表 + 仓库组列表（完成 2026-10-24）

已落地：

1. **i18n 升级为"真正执行旧词表"**：旧 SPA 的文案是运行时拼词
   （`newRepository: [Phrase.new, Term.repository].join('')`），静态正则解析会漏掉；
   `scripts/extract-lang.mjs` 现在把 lang 目录 stage 到临时目录、改写 `APPSRC/...` 导入后
   真正 `import()` 执行，再按旧版 `composeNamespacedData` 规则加命名空间前缀。
   结果：**961 键 × 2 语言**（Go 54 + 旧 SPA 907），`t(key, values)` 支持 `{s}`/`{n}` 占位符。
2. **图标本地化**：`scripts/extract-icons.mjs` 把 `@pgyer/icons@0.0.16` 的 154 个图标
   数据落成 `src/assets/icons/*.ts`，新增 `components/unit/Icon.tsx` 直接渲染
   `<svg><path/></svg>`。**彻底去掉 `@fortawesome/*` 与上游 barrel 的全量打包**
   （旧版 barrel 会把 154 个图标全打进产物，AGENTS §9 记录过这个坑）。
3. 组件库补齐：`Icon` / `Dropdown`（页头四处菜单共用）/ `SquareIconButton`（含未读角标）/
   `Marker`（角色徽标，「info / containedInfo」两态）/ `FormattedTime`
   （相对/绝对切换、英文 12h→24h 与旧版 `p → H:mm` 对齐）。
4. 业务共用件：`components/view/unit/{RepositoryRow,GroupCard}.tsx` + `list.css`。
5. 数据层：`api/{types,user,group,repository}.ts` + `features/app-data/AppDataProvider.tsx`
   （对应旧 Redux `DataStore` 的三份共享数据，一次并发拉取；后续接 TanStack Query 时
   只换这一处）。
6. 布局：`AppShell` 改为「固定页头 + 白底内容区 + `padding: 24px 48px`」，
   与旧 `components/Main.js` 的 `theme.mixins.toolbar` 占位 + `spacing(3) spacing(6)` 一致。
7. 页面：`/repositories`（页签 全部/我参与的/我创建的 + 权限过滤 + `updated` 倒序 + 空态）、
   `/groups`（同页签口径 + 卡片栅格 + 每卡仓库数由前端统计）。

门禁与验证：

| 项 | 结果 |
|---|---|
| `tsc --noEmit` / `eslint .` / `vite build` | 全绿（构建 213 ms） |
| headless DOM 断言 `/repositories` | 3 个仓库、两组的 displayName、角色（所有者/维护者/开发者）、页签、`更新于`×3 全部渲染 |
| headless DOM 断言 `/groups` | 2 个组、描述、角色、页签、`新建仓库组` 全部渲染 |
| 像素量测（1440×900 截图） | 页头高 **64px**（边框线 y=63）；仓库行节距 **71px**（行高 72px + `margin-bottom:-1px`），与旧版 `height: spacing(9)` 完全一致 |
| 产物体积 | vendor **104.5 KB gzip**（含 date-fns）；全部 js+css 165.6 KB gzip；`RepositoryList` 2.1 KB、`GroupList` 1.1 KB gzip |
| mock 验证通道 | `scripts/serve-preview.mjs` 增加 `/api/user/info`、`/api/repository/list`、`/api/group/list` 的 mock（信封与真实后端一致），用于本地视觉校验 |

本批偏离/顺延：

| # | 项 | 原因 | 回归条件 |
|---|---|---|---|
| 1 | **未读通知铃铛暂不渲染** | 通知抽屉属于批 5；提前放一个没有目标的按钮会成为死交互。`unReadNotification` 数据已在 `useAppData()` 备好 | 批 5 |
| 2 | 页头滚动阴影（旧 `withScrollTrigger` 的 elevation 0→2）暂缺省 | 依赖 `.app-content` 滚动监听，随批 3 的仓库页布局一起补 | 批 3 |
| 3 | 页头"仓库/仓库组"菜单不再带"全部"项 | 核实旧版 `GroupRepositoryMenu` 只有条目列表、没有"全部"入口，之前计划里那一项属于臆测 | 永久 |
| 4 | 新增运行时依赖 `date-fns@4` | 旧版 `FormattedTime` 用 date-fns 的 `format`/`formatRelative`，为保证相对时间文案一致而保留 | 若改为 `Intl.RelativeTimeFormat` 需重新比对英文文案 |

### 批 3（上）—— 仓库上下文导航 + 文件浏览 + 代码高亮（2026-10-24）

已落地：

1. **仓库上下文**：`features/repository/RepositoryProvider.tsx` —— 用「组名 + 仓库名」在全局仓库列表里
   解析出仓库主键（旧版同样做法），再拉 `repository/config`（仓库 + 组 + 成员 + 分支/标签 + 计数）。
   找不到即 `notFound` → 渲染 404，替代 react-router 5 的路径正则约束。
2. **左侧上下文导航**：`components/layout/Sidebar.tsx` + `config/drawer.ts` —— 逐字移植旧
   `DrawerConfig.makeRepositoryDrawerConfig` 的 8 个一级项与设置下 4 个子项（含提交/分支/标签/成员计数），
   active 判定沿用旧版正则语义；展开 320px / 收起 88px、右缘折叠按钮。
3. **文件浏览**：`pages/repository/FileTree.tsx` —— 分支/标签下拉（`config.branches|tags`）、
   面包屑（`repository/pathStack`）、目录行（`repository/object` 的 tree entries + 最近提交）、
   文件内容按 blob 类型分流：文本 → `CodeViewer`；图片 → data URL；`oversize`/`unsupport` → 提示。
4. **代码高亮**：`helpers/prism.ts` + `components/unit/CodeViewer.tsx` —— 沿用旧版 `prism-coy` 主题与
   「整文件高亮后按行切分」的策略（含 8 条 LRU 缓存）；语言清单与 `config/languages.ts` 的
   扩展名映射严格对齐（旧版 36 种）。
5. **构建切片修正**：`manualChunks` 从"全部 node_modules 进 vendor"改为"仅框架层进 vendor"，
   否则 prismjs 会被带回首屏（实测 vendor 139 KB → 98.8 KB gzip，prism 落到懒加载的 FileTree 块）。

门禁与验证：

| 项 | 结果 |
|---|---|
| `tsc --noEmit` / `eslint .` / `vite build` | 全绿（构建 251 ms） |
| headless DOM `/codefever_team/codefever-community/files` | 仓库标题、8 个导航项、3 个目录/文件行、提交摘要与时间全部渲染，控制台无错误 |
| headless DOM `.../files/README.md` | 行号 8 行、Prism token（`token code/function/punctuation/title`）均生成 → 高亮生效 |
| 像素量测 1440×900 | 左侧导航宽 **319px**（≈320 = spacing 40）；文件行节距 **40px**，与 `--cf-table-row-height` 一致 |
| 体积 | 首屏 **142 KB gzip**（vendor 97.7 KB + i18n 22.3 KB + 其余）；`FileTree` 懒加载块 **36.4 KB gzip**（含 prism 与 37 种语言） |

踩到并修掉的两个 Prism 陷阱（供后续批次参考）：

| 现象 | 原因 | 处理 |
|---|---|---|
| 页面整块白屏、控制台 `Cannot set properties of undefined (setting 'comment')` | Vite 没有 `babel-plugin-prismjs` 的依赖解析：`prism-tsx` 必须在 `jsx`+`typescript` **之后**加载，`prism-vbnet` 依赖 `basic` | 显式按依赖顺序 import（`prism-basic` 在 `vbnet` 前、`tsx` 在 `jsx` 后），并在 `helpers/prism.ts` 注明顺序不可调整 |
| `manifest.json` 404 / 站点图标缺失 | `web/public/` 之前没建，`index.html` 引用的 manifest 与 favicon 未随构建产出 | 从 `www/` 迁入 `public/{favicon.ico,manifest.json,sitemap.xml}` |

批 3 剩余（下一轮）：提交列表/提交详情、分支列表、标签列表、Blame 侧栏、ObjectViewer（图片/二进制/超大文件视图）、
以及页头的滚动阴影。

### 批 3（下）—— 提交 / 分支 / 标签 / diff（完成 2026-10-24）

已落地：

1. `pages/repository/CommitList.tsx` —— 分支/标签下拉 + 服务端关键字搜索 + 分页
   （`commitList?repository&revision&keyword&path&page&pagesize`），行组件 `CommitRow`
   （标题→详情、作者头像+姓名、「编辑于」时间、8 位短哈希）。
2. `pages/repository/CommitDetail.tsx` —— `commitDetail` + `fileChanges`（父提交为空时
   交给后端补全 0 SHA），命中"文件数 > 100"时展示 `message.diffFileToMany`。
3. `components/unit/DiffViewer.tsx` —— **后端已把 diff 解析成结构**（`DiffFile[]`，
   逐行 `line` 数组），前端只做行号推进与着色：hunk 头 / 新增 / 删除 / 上下文；
   逐行按文件扩展名取 Prism 语言做高亮（复用 `config/languages.ts`，不再本地维护副本）。
4. `pages/repository/BranchList.tsx` / `TagList.tsx` —— 行结构对齐旧 `RefList`：
   图标 + 名称 + 保护标记 + 最近提交 + 时间 + 短哈希；分支/标签名可点进对应 rev 的文件树
   （`/files?rev=`）。
5. `helpers/members.ts` —— 提交邮箱 → 成员解析（旧 `getUserInfo`），未命中标记为临时用户。
6. **i18n 补充表** `src/i18n/supplement.ts`：旧词表确实缺失的 8 个键（`label.search` 等）
   单独成表并在运行时覆盖自动生成的表，避免手改生成文件被下次生成冲掉。

门禁与验证：

| 项 | 结果 |
|---|---|
| `tsc --noEmit` / `eslint .` / `vite build` | 全绿（构建 234 ms） |
| headless DOM `/commits` | 3 条提交（标题、作者 root/alice/bob、短哈希）、搜索框、重置按钮、ref 下拉均渲染 |
| headless DOM `/commit/aaaaaaaa` | 提交标题、父提交、"新的描述行/新增的说明行/旧的描述行"、`oversize` 文件的"文件过大"提示 |
| headless DOM `/branches` `/tags` | 分支 main/dev + 保护标记；标签 v1.0.0 + 注释说明 |
| 像素量测（commit 详情 1440×1000） | 新增行底色 `#e6ffed` 命中 7233、删除行 `#ffeef0` 3624、hunk 头 `#f1f8ff` 3906 —— 与旧 preset 的 diff 三色**完全一致** |
| 体积 | 首屏 **143 KB gzip**；prism 独立成异步块 **34.5 KB gzip**（只在打开代码/diff 时加载）；提交/分支/标签页各 0.9~1.9 KB gzip |

本批偏离/顺延：

| # | 项 | 原因 | 回归条件 |
|---|---|---|---|
| 1 | **Blame 侧栏未实现** | 依赖 `blameInfo` + 侧栏布局，属独立视图；先把主链路交付 | 下一轮补 `/files/blame` 路由与 `blameInfo` 渲染 |
| 2 | ObjectViewer 富视图只覆盖文本/图片 | 旧版对视频、二进制、超大文件有更多展示分支 | 下一轮补 video/base64 分支 |
| 3 | diff 采用**逐行**高亮（旧版按侧全文高亮） | 逐行高亮能把"未变行"和"改动行"分开着色；代价是跨行 token（块注释、多行字符串）在 diff 里颜色可能不完整，代码正文视图（CodeViewer）仍是全文高亮，不受影响 | 若产品要求逐字一致，改为按侧全文高亮 + 行内切片 |
| 4 | `deletions` 展示取绝对值 | 后端沿用 PHP 的 `deletions--` 写法，值可能是负数 | 永久（后端不改） |
| 5 | 提交列表分页用"满页则可翻下一页"近似 | `commitList` 不返回总数（旧版同样靠返回条数判断） | 后端增加 total 字段后改为精确页码 |

### 批 4（上）—— 个人设置三页（2026-10-24）

已落地：

1. `components/layout/SettingsLayout.tsx` + `config/drawer.ts` 的 `settingsDrawerEntries()` ——
   左侧 4 项导航（个人资料 / 邮箱 / SSH Key / 通知），active 正则逐字移植旧 `makeDrawerConfig()`。
2. `pages/settings/Profile.tsx`：三段表单 —— 基本信息（`user/updateBasicInfo`：name 必填、
   email 必填、team 可选）、头像上传（`user/uploadAvatar`，multipart 字段 `avatar`）、
   修改口令（`user/updatePassword`：`current`/`new`）、MFA（`getMFAData`/`updateMFAData`/
   `revokeMFAData`，含"移除需二次确认"）。
3. `pages/settings/Email.tsx`：主邮箱更换（`getEmailCode` 发码 → `changeEmail`）+
   提交邮箱管理（`addCommitEmail` / `resentCommitEmailCode` / `validationCommitEmailCode` /
   `deleteCommitEmail`），主邮箱取 `user/info` 的 `emails[0]`（`id` 为空）与旧版一致。
4. `pages/settings/SSHKey.tsx`：`listKey` 列表 + `addSSHKey`（name/key）+ `removeSSHKey`（id）；
   前端预校验"公钥不能含空格/换行"（后端同样拒绝，属 authorized_keys 注入防护）。
5. `src/i18n/supplement.ts` 扩充：13 个旧词表缺失的设置页文案（成功/失败提示、验证状态等）。

门禁与验证：

| 项 | 结果 |
|---|---|
| `tsc --noEmit` / `eslint .` / `vite build` | 全绿（构建 255 ms；`useEffect` 依赖告警已修） |
| headless DOM `/settings/profile` | 基本信息 / 上传头像 / 密码（旧·新·确认）/ 安全性 / 未开启2FA / 设置 MFA 设备 / 保存×3 全部渲染 |
| headless DOM `/settings/email` | 主邮箱、关联邮箱列表、添加提交邮箱、保存、侧栏 4 项 |
| headless DOM `/settings/sshkey` | 两条 mock 公钥（id_ed25519 / work-laptop）、创建时间、删除×2、新增 SSH Key 表单 |
| 体积 | 首屏 **145 KB gzip**；Profile 1.6 / Email 1.5 / SSHKey 1.1 KB gzip（均为懒加载页） |

本批偏离/顺延：

| # | 项 | 说明 |
|---|---|---|
| 1 | 通知设置页顺延 | `userNotificationSetting` 的 `offset` 语义（按位开关）需与旧页逐位核对，下一轮单独处理 |
| 2 | MFA 用 secret 文本 + 两组验证码 | 旧版展示二维码图片；二维码生成属于待定后端能力（`qrcode` handler 已存在但未接到该接口），先给 secret 便于手工录入，下一轮补二维码渲染 |
| 3 | 头像上传做了前端无预览 | 旧版有即时预览；本轮以"上传成功 + 刷新用户信息"为准，预览随批 4 后续补 |
| 4 | 仓库/组设置、成员、Webhook、保护分支仍未开始 | 批 4 下半 |

### 批 4（中）—— 通知设置 + 仓库基本信息 + 仓库成员（2026-10-24）

已落地：

1. `pages/settings/Notification.tsx` —— **先核对了 `offset` 语义再实现**：
   后端 `userNotificationSetting` 把 `offset` 当位序号，`status ^= 1 << (offset-1)`，
   且 `u_notification_status` 的**位=1 表示关闭**（`repo.NotificationStatusMR=1`、
   `Email=2`）——这三点与旧页面的开关方向一致：开关选中 = `(notification & bit) === 0`。
   下半页展示 `user/notificationRefused` 的拒收列表（`type` 1=组、2=仓库），
   名称从全局组/仓库列表解析，恢复走 `setGroupOrRepoNotification`（级联由后端处理）。
2. `pages/repository/SettingsGeneral.tsx` —— `repository/update`（`displayName` 必填、
   `description` **始终发送**，因为后端缺参会按 PHP 历史行为报 500）+ `uploadAvatar`（multipart）。
3. `pages/repository/SettingsMembers.tsx` —— `addMember`（email）/ `changeMemberRole`（userID+roleID）/
   `removeMember`（userID）/ `changeOwner`（userID），操作列按 `REPO_CHANGE_MEMBER` 权限显隐，
   所有者行只显示徽标、不允许改角色/删除（需先转移所有权）。
4. `api/user.ts` + `api/repository.ts` 新增 10 个写方法；`i18n/supplement.ts` 再补 3 条
   （`label.addMember` / `label.changeOwner` / `label.open`）。

门禁与验证：

| 项 | 结果 |
|---|---|
| `tsc --noEmit` / `eslint .` / `vite build` | 全绿（构建 350 ms） |
| headless DOM `/settings/notification` | 两个开关（合并请求通知 / 邮件通知）+「接收通知状态」+ 拒收列表（gitsvc / 基础设施组）+ 恢复按钮 ×2 |
| headless DOM `.../settings/general` | 基本信息 / 上传头像 / 仓库名 / 仓库描述 / 保存 |
| headless DOM `.../members` | root/alice/bob + 角色徽标（所有者/维护者/开发者）+ 添加成员 + 转移所有权 + 删除 |
| 体积 | 首屏 **145.7 KB gzip**；通知 1.0 / 仓库基本信息 1.0 / 成员 1.3 KB gzip |

偏离/顺延：

| # | 项 | 说明 |
|---|---|---|
| 1 | 仓库"默认分支"设置未纳入本页 | `repository/defaultBranch` 属分支设置范畴，随保护分支页一起做 |
| 2 | 组设置（general/advanced/members）未开始 | 批 4 下半剩余 |
| 3 | Webhook 6 个端点、保护分支 4 个端点未开始 | 同上；这两块表单最多，单独一轮 |

### 批 4（下）—— 仓库分支设置 + Webhook（完成 2026-10-24）

已落地：

1. `config/webhookEvents.ts` —— 逐字移植旧 `WebhookEventConfig.js` 的 **25 个事件**（含分组标题行），
   事件名与后端 `domain.WebhookEventMatches` 的逗号精确匹配语义对应；
   `parseEvents`/`joinEvents` 与 `strings.Split(events, ",")` 对称。
2. `pages/repository/SettingsBranch.tsx` ——
   默认分支（`repository/defaultBranch`：`repository` + `old` + `branch`，后端要求 old ≠ branch）+
   保护分支四条端点（列表/新建/更新/删除）。角色下拉**只给后端白名单接受的取值**
   （push：无权限/所有者/维护者/开发者），避免"前端能选、后端报 0x0201"。
3. `pages/repository/SettingsWebhook.tsx` —— 列表（`webhooks`，POST）+ 新建/更新（`editWebhook`，
   **不带 rwKey 即新建**，`active` 必须 1/2）+ 删除 + 投递日志（`getRepositoryWebhookLogs` 的
   `webhook` 参数、`getRepositoryWebhookLogData` 的 `id` 参数），日志原始报文用等宽块展示。

门禁与验证：

| 项 | 结果 |
|---|---|
| `tsc --noEmit` / `eslint .` / `vite build` | 全绿（构建 344 ms；清掉两处未使用 import） |
| headless DOM `.../settings/webhook` | 添加 Webhook / mock URL / 事件摘要（推送、合并请求）/ 启用标签 / 编辑·日志·删除 |
| headless DOM `.../settings/branch` | 默认分支选择 + 保护分支规则两条（main、release/*）+ 推送角色 / 合并角色 / 维护者·所有者 + 编辑·删除 |
| 体积 | 首屏 **146.1 KB gzip**；分支设置 1.5 / Webhook 2.2 KB gzip |

偏离/顺延：

| # | 项 | 说明 |
|---|---|---|
| 1 | `getWebhook` 端点未使用 | 列表已返回全部字段（url/events/active/secret），编辑直接取行数据；端点保留在 api 层备用 |
| 2 | 分支/标签的"新建/删除"动作未放进来 | 属分支列表页的动作（`createBranch`/`deleteBranch`/`createTag`/`deleteTag`），随分支页交互下一轮补 |
| 3 | 组设置三页（general/advanced/members）与仓库高级（改名/删除）未开始 | 批 4 剩余，独立一轮 |

### 批 4（尾）—— 组设置三页 + 高级设置（完成 2026-10-24）

已落地：

1. `features/group/GroupProvider.tsx` + `components/layout/GroupLayout.tsx` —— 组上下文外壳：
   用组名解析组主键 → `group/config`（组 + 成员 + 成员数）→ 左侧 5 项导航
   （仓库 / 合并请求 / 成员 / 动态 / 设置），`config/drawer.ts` 的 `groupDrawerEntries` 逐字移植
   旧 `makeGroupDrawerConfig` 的 active 正则。
2. `pages/group/{SettingsGeneral,SettingsMembers,SettingsAdvanced}.tsx` ——
   `group/update`（displayName + description 必传）、`group/uploadAvatar`、
   四个成员端点、`group/updateName` 与 `group/deleteGroup`。
3. `pages/repository/SettingsAdvanced.tsx` —— `repository/updateName` + `repository/deleteRepository`。
4. **抽了两个共用组件**（AGENTS §4 的"第二处重复即抽取"）：
   - `components/view/unit/MembersManager.tsx`：仓库成员与组成员共用（列表 + 角色下拉 +
     移除 + 转移所有权），页面只注入端点与权限码；
   - `components/view/unit/AdvancedSettings.tsx`：仓库/组高级页共用（改名表单 + 危险操作区）。
   重构后仓库成员页从 200 行降到 ~100 行，两处行为完全一致。
5. `pages/RepositoryList.tsx` 支持组上下文：`/groups/{组名}/repositories` 只显示该组仓库、
   首页签文案改为"仓库"、新建按钮指向组内路径（对齐旧版 `currentGroupKey` 行为）。

门禁与验证：

| 项 | 结果 |
|---|---|
| `tsc --noEmit` / `eslint .` / `vite build` | 全绿（构建 309 ms） |
| headless DOM `/groups/codefever_team/repositories` | 组标题 + 仅该组两个仓库（另一组的 web 未出现）+ 页签 + 新建仓库 + 5 项导航 |
| headless DOM `/groups/codefever_team/members` | root/alice/bob + 角色徽标 + 添加成员 |
| headless DOM `/groups/codefever_team/settings/{general,advanced}` | 基本信息/组名/组描述；修改名称 + 删除仓库组 |
| headless DOM `.../settings/advanced`（仓库） | 修改名称 + 删除仓库 |
| 体积 | 首屏 **147.5 KB gzip**；MembersManager 0.96、两个高级页各 0.4 KB gzip |

偏离/顺延：

| # | 项 | 说明 |
|---|---|---|
| 1 | 分支/标签的新建与删除动作仍未做 | 属分支/标签页的行内动作，随批 3 的分支页交互补齐 |
| 2 | 组内合并请求 / 组动态仍为占位 | 批 5 |
| 3 | 改名成功后未自动跳转到新 URL | 旧版会跳转；这里先刷新数据，跳转逻辑与"改名后旧地址失效"的提示一起在批 7 收尾时统一处理 |

### 批 5（上）—— 合并请求列表与详情（2026-10-24）

已落地：

1. `pages/repository/MergeRequestList.tsx` —— 状态页签（待合并 1 / 已合并 2 / 已关闭 3，对齐
   `domain.MergeRequestStatus*`）+ 关键字搜索 + 行内展示 `#{编号} 标题`、`源分支 → 目标分支`、
   状态标签、更新时间、空态插图（`mergerequests-empty.png`）。
2. `pages/repository/MergeRequestDetail.tsx` —— `mergeRequestDetail?repository&mid`（`mid` 是
   业务编号 number）+ `fileChanges` 带 `mergeRequest` 参数比较
   「目标分支当前提交 → 源分支当前提交」，diff 复用 `DiffViewer`；
   动作：评审（`mergeRequestReview`，**后端只要 `id`**）、关闭（`mergeRequestClose`）、
   合并（先 `checkMergeType` 再 `mergeBranch`，提交信息默认取标题、`usingSquash=true`）。
3. `api/repository.ts` 新增 10 个方法（列表/详情/变更/关闭/评审/指派与移除评审人/合并类型/合并）。

门禁与验证：

| 项 | 结果 |
|---|---|
| `tsc --noEmit` / `eslint .` / `vite build` | 全绿（构建 358 ms） |
| headless DOM `/mergerequests` | 页签三项 + `#12`/`#9` 两条 + 分支流向 + 新建按钮 |
| headless DOM `/mergerequests/12` | 标题、提交人、评审人 alice、待合并标签、打开文件树、diff 行（新的描述行/旧的描述行）、评审/合并/关闭按钮 |
| 体积 | 首屏 **147.9 KB gzip**；MR 列表 1.2 / 详情 1.6 KB gzip |

顺延：MR 创建页（`createMergeRequest`：source/target 仓库与分支、标题、描述、评审人）、
评审人指派/移除 UI（端点已封装）、MR 版本列表与提交列表两个端点、活动流、通知抽屉。

### 批 5（下）—— 合并请求创建 + 通知抽屉（2026-10-24）

已落地：

1. `pages/repository/MergeRequestCreate.tsx` —— `createMergeRequest` 的 body 逐项对齐后端
   （`sourceRepository` / `targetRepository` / `sourceBranch` / `targetBranch` / `title` /
   `description` / `reviewers` 逗号分隔的用户主键），响应 `{id}` 是**业务编号**，直接跳详情。
2. `components/layout/NotificationDrawer.tsx` + Header 铃铛 —— 通知抽屉：
   四个分类（全部/未读/系统/提及，对应 `NotificationCategory 0..3`）、
   单条标记已读与全部已读、清空；`setNotificationRead` / `deleteNotification` 的
   `all` 与 `id` **有且只能有一个**（后端强校验），因此"全部"走 `all=1`、单条走 `id`；
   `data` 是 JSON **字符串**，做容错解析后取 `message`/`content`/`title` 展示。
3. `api/{repository,user}.ts` 新增 4 个方法；i18n 补充表 +5 条（全部已读/标记已读/清空/系统/提及我）。

门禁与验证：

| 项 | 结果 |
|---|---|
| `tsc --noEmit` / `eslint .` / `vite build` | 全绿（构建 216 ms） |
| headless DOM `.../mergerequests/new` | 创建合并请求、源分支/目标分支下拉（main、dev）、评审人（root/alice/bob）、取消 |
| headless DOM 页头 | 铃铛按钮存在且角标为 **7**（来自 mock 的 `unReadNotification`） |
| 通知接口 | `curl /api/user/notifications?category=0` 返回两条 mock，`data` 为 JSON 字符串（与后端约定一致） |
| 体积 | 首屏 **148.8 KB gzip**（通知抽屉改为懒加载后单独 1.2 KB 块）；MR 创建页 1.2 KB gzip |

**验证缺口（诚实记录）**：通知抽屉需要点击铃铛才展开，headless `--dump-dom` 无法触发点击，
因此本轮只验证了"铃铛 + 角标 + 接口数据"，抽屉的实际渲染留到批 7 的人工视觉回归一并覆盖。

顺延：活动流（`repository/activities`、`group/activities`，旧版 `config/Activity.js` 是 682 行
的 40 余种类型渲染表，需要单独一轮逐类移植）、评审人指派/移除 UI、MR 版本与提交列表端点。

### 批 6（上）—— 管理后台四页（2026-10-24）

已落地：

1. `components/layout/AdminLayout.tsx` + `config/drawer.ts` 的 `adminDrawerEntries()` ——
   左侧 5 项导航（仪表盘/用户/仓库组/仓库/系统设置），active 正则逐字移植旧 `makeAdminDrawerConfig()`。
2. `pages/admin/Dashboard.tsx` —— `admin/systemStatus` 三段：`usage`（用户/仓库/组计数卡片）、
   `vm`（cpu / memory 百分比 + 磁盘用量）、`service`（nginx / codefever / sendmail 运行状态标签）。
3. `pages/admin/Users.tsx` —— `admin/userList`（**POST** + `{category, keyword, sort, page, pagesize}`
   → `{count, list}`），行内动作：启用/禁用（`updateUserStatus`）、设为/取消管理员
   （`setAdministrator`）、关闭 MFA（`closeUserMFA`）、重置口令（`resetPassword`，弹窗输入新口令）。
4. `pages/admin/Groups.tsx` / `Repositories.tsx` —— 只读列表 + 搜索 + 分页（`groupList` / `repositoryList`）。

门禁与验证：

| 项 | 结果 |
|---|---|
| `tsc --noEmit` / `eslint .` / `vite build` | 全绿（构建 237 ms） |
| headless DOM `/admin/dashboard` | 三张计数卡 + CPU/内存/系统资源 + 服务状态（nginx/codefever 运行中、sendmail 已停止，与 mock 一致） |
| headless DOM `/admin/users` | 三个账号、管理员标记、启用/禁用与设为/取消管理员动作、共 3 条 |
| headless DOM `/admin/groups` `/admin/repositories` | 组与仓库列表 + 共 N 条 + 搜索框 |
| 体积 | 首屏 **148.8 KB gzip**；管理后台四页各 1.1~2.3 KB gzip（懒加载） |

顺延：系统设置页（`admin/config` 的 GET/POST —— 返回整份 `config.yaml` 快照、写入走
浅合并 + 原子写 + 校验，前端要做一个"结构化表单 + YAML 预览"的页面，单独一轮）、
文档中心（需要新增 Go 侧 `doc/tree` 与 `doc/content` 两个 JSON 端点）。

### 批 6（下）—— 文档中心（含 Go 端点）+ 系统设置（2026-10-24）

**本轮第一次改动后端**，先定接口再实现：

| 端点 | 语义 |
|---|---|
| `GET /api/doc/tree?lang=cn\|en` | 文档菜单树（含二级）；`lang` 沿用旧 URL 语义 cn/en，Go 侧映射到目录 `zh-cn`/`en-us` |
| `GET /api/doc/content?lang=cn\|en&path=…` | 单篇 Markdown 原文；`path` 相对文档根，**复用 `resolveDocPath`** 做符号链接与路径穿越校验，非 `.md` 一律 404 |
| `GET/POST /api/admin/config` | 已存在：GET 返回整份 config 快照；POST `{data:"<JSON 字符串>"}` 走浅合并 + 结构校验 + 原子写 + 热更新 |

实现要点：

1. 两个 doc 端点注册为**字面量路由**（`RegisterPage`，与 `/api/web-vitals` 同一手法），
   绕过 `/api/{ctrl}/{action}` 分发器的鉴权与 Content-Type 校验 —— 文档是公开只读内容；
   响应信封沿用 `request-id`/`code`/`message`/`data` 四字段。
2. `api/doc.ts` + `pages/Doc.tsx`：左侧菜单（支持二级）+ 右侧 Markdown（`react-markdown` +
   `remark-gfm`，样式沿用内置 `markdown-body.css`），顶栏保留品牌与中英切换；
   URL 保持 `/doc/{lang}/{path}` 可分享。
3. `pages/admin/Settings.tsx`：受控 JSON 编辑器（加载美化 → 编辑 → 本地 `JSON.parse` 校验 →
   二次确认 → 保存）。**偏离**：旧版是按字段的结构化表单；先保证"能安全读写整份配置"，
   逐字段表单待按 `config.template.yaml` 补齐（避免漏字段写坏配置）。

门禁与验证：

| 项 | 结果 |
|---|---|
| Go：`gofmt -l`、`go build ./...`、linux 交叉编译、`go vet`、`go test ./internal/...` | **全绿** |
| 前端：`tsc --noEmit`、`eslint .`、`vite build` | 全绿 |
| headless DOM `/doc/cn` | 菜单（快速开始/功能说明）、`<h1>` 与表格（引擎）渲染成功、中英切换存在 |
| headless DOM `/admin/settings` | JSON 编辑器含 mock 配置（`allowRegister`、`noreply@example.com`）、保存/重置、提示文案 |
| 体积 | 首屏 **149.6 KB gzip**；文档页 chunk **46.5 KB gzip**（157 KB raw，含 react-markdown/remark-gfm 全家桶，懒加载）+ CSS 3.8 KB；系统设置页 1.0 KB gzip |

说明：`react-markdown` + `remark-gfm` 的依赖链（unified / micromark / mdast）体积不小
（157 KB raw / 46.5 KB gzip），但**只在文档页懒加载块里**，不进首屏；若后续要压这 46 KB，
可改为"服务端渲染 Markdown → 下发 HTML"（复用旧版 marked 的等价能力），属单独优化项。

### 批 5（尾）—— 活动流 + 评审人指派（2026-10-24）

**为什么把这一步放在批 7 之前**：批 7 要删除 `www/view`（当前的参考实现）。活动流的
语义（41 个活动码 → 文案 + 主体 + 细节）只能从旧 `config/Activity.js` 读出来，
所以必须在删除前补齐，否则会丢掉唯一的事实源。

已落地：

1. `components/view/unit/ActivityFeed.tsx` —— 把旧 `config/Activity.js`（682 行、41 个分支）
   压缩成**一张码表**：`活动码 → 文案键 + 主体来源 + 细节类型`，
   逐条对照旧代码的 `message.activity.*` 键。41 个码全部登记，未登记码走通用回退（不静默丢条目）。
2. 主体/细节语义**按旧实现对齐**（这一步我第一版做错了）：
   - 分支/标签/合并请求/Webhook 类活动（0x04xx/0x05xx/0x07xx/0x09xx）的 `{s}` 是
     **"组名/仓库名"**，分支名、标签名、MR 编号是 `detail`；
   - 仓库/组管理类活动（0x03xx/0x02xx）的 `{s}` 是仓库名/组名；
   - 成员类活动把成员名放进 `detail`。
3. `pages/repository/ActivityList.tsx` + `pages/group/ActivityList.tsx` —— 两个动态页，
   共用 `ActivityFeed`（同一套码表），成员名/图标与组、仓库名从当前上下文解析。
4. `MergeRequestDetail` 增加**评审人管理**：成员下拉指派（`assignReviewer`）、
   逐个移除（`deleteReviewer`），只在待合并状态显示。

门禁与验证：

| 项 | 结果 |
|---|---|
| `tsc --noEmit` / `eslint .` / `vite build` | 全绿 |
| headless DOM 仓库/组动态 | 四类活动文案与旧语义一致：`alice 推送提交到仓库 CodeFever 团队/codefever-community 的分支 · main`、`bob 在仓库 … 打开合并请求 · #12`、`root 向仓库 codefever-community 添加了成员 · alice` |
| headless DOM MR 详情 | 评审人区（当前 alice）+ 成员下拉 + 添加成员按钮 |
| 体积 | 首屏 **149.6 KB gzip**（不变）；动态页 0.65 KB、MR 详情 1.8 KB |

仍未接（记录在案）：`mergeRequestVersionList`（MR 版本对比）与 `mergeRequestCommits`（MR 提交列表）
两个只读端点 —— 旧版用于"多版本 diff"与"提交列表"，属可选增强。

### 批 7（一）—— 部署切换：SPA 壳下发 + 镜像布局（完成 2026-10-24）

**这一步刻意做成"可回滚"**：旧 SSR 渲染函数全部保留，只在它们前面加一层
"能拿到 SPA 壳就下发壳，拿不到就按旧逻辑渲染"。

Go 侧（新增 `internal/httpapi/spa.go`）：

1. `ConfigureSPA(path)` 由 main 注入壳路径；`ServeSPA(w, r)` 下发壳，
   内容只读一次并缓存在内存（镜像内产物不可变），响应固定 `Cache-Control: no-cache`
   —— 壳引用带哈希的资源名，壳若被缓存，发布会白屏。
2. 壳路径解析顺序：`CODEFEVER_SPA_INDEX` → `<base>/www/app/index.html`（镜像布局）→
   `<base>/web/build/index.html`（仓库内直跑）→ `<base>/www/view/build/index.html`（旧布局回滚）。
   全都找不到时记一条 warn，各 GET 回退到旧 SSR。
3. 改造点（均带回退）：`/user/login`（main 的 loginPageHandler）、`/user/register`、
   `/user/verify`、`/user/resetPassword`、`/doc/*` 的 Markdown 页、未知路径的 404 页。
4. 文档配图（`\w{32}.png`）与文档 302 跳转链保持原样 —— 旧 URL 语义不变
   （`/doc/cn` → `/doc/cn/` → … → `/doc/cn/start/welcome.md`）。

镜像与站点（Dockerfile / nginx）：

- `COPY web/build/ ./www/app/`：新产物直接铺成**真实目录**，取代 `www/app -> view/build` 软链；
  前置 `RUN rm -rf ./www/app` 处理 Windows 检出把软链落成 10 字节普通文件的问题
  （否则 COPY 报 `not a directory`）。
- `COPY web/public/{favicon.ico,manifest.json,sitemap.xml} ./www/`：
  图标/PWA/站点地图改由新前端产出。
- `.dockerignore` 增加 `web/{node_modules,src,scripts,.tmp-lang,package-lock.json}`。
- nginx：功能无需改动（`/static/js|css` 一年 immutable 已匹配 Vite 的 `assetsDir=static/js`；
  `try_files $uri /app$uri` 与 `/app/index.html` 回退照旧），只更新注释说明新布局与壳下发。

验证（**真实镜像 + 真实容器**，非 mock）：

| 项 | 结果 |
|---|---|
| `docker build -t codefever-next:local .` | 成功 |
| `docker compose up -d` | db healthy、app 启动 |
| `GET /user/login` | **200 + SPA 壳**（引用 `/static/js/index-*.js`） |
| `GET /static/js/index-*.js` | 200，48 KB |
| `GET /doc/cn` → 跟随 302 → `/doc/cn/start/welcome.md` | 200 + SPA 壳 |
| `GET /api/doc/tree?lang=cn` | 200 JSON，菜单树来自容器内 `doc/` 目录 |
| `GET /api/doc/content?lang=cn&path=index.md` | 200 JSON，返回真实 Markdown |
| `GET /api/user/info`（未登录） | 403 JSON |
| `GET /` | 302（home 控制器语义不变） |
| headless 渲染 `/user/login` | React 登录页：欢迎登录 / 请输入邮箱 / 忘记密码 / 登 录 / 还没有账号 / 去注册 |
| headless 渲染 `/doc/cn/start/welcome.md` | Markdown 渲染成功（含 `markdown-body`、h1、真实文档内容） |
| Go 五项门禁 | gofmt / build / 交叉编译 / vet / test **全绿** |

下一步（批 7 二）：删除 `www/view`（195 文件）、`www/static`（52 文件）、Go SSR
（模板 15 + `pages.go` 505 + `pages_test.go` 531 + `testdata/pages` 6 + 两个比对工具 +
`web_captcha.go`），并同步 `AGENTS.md` §4/§5/§8.3/§9/§10、`README`、6 份 `docs/*.md` 的引用，
最后做 40 页 × 3 视口的视觉回归。

### 批 7（二）—— 删除旧前端与 Go SSR 层（完成 2026-10-24）

删除清单（全部为**不可逆**操作，单独一次提交）：

| 对象 | 规模 |
|---|---|
| `www/view/`（React 17 + Redux + MUI v4 + webpack 4） | 195 个跟踪文件 |
| `www/static/`（jQuery/Bootstrap/FontAwesome/intlTelInput/toastr + 16 张图） | 52 个文件 |
| `www/app`（软链残留） | 1 |
| `backend/internal/httpapi/templates/` | 15 文件 / 1,329 行 |
| `backend/internal/httpapi/pages.go` | 505 行 |
| `backend/internal/httpapi/pages_test.go` | 531 行 |
| `backend/testdata/pages/php-*.html` | 6 文件 / 72 KB |
| `backend/cmd/page-compare` + `cmd/pagediff` | 344 + 70 行 |
| `backend/internal/handlers/web_captcha.go` | 160 行（全站零引用，死代码） |

保留与迁移（关键，避免误删）：

- `AjaxEnvelope` / `WriteAjaxJSON` / `AjaxObject` 原在 `pages.go` 里，**拆到 `httpapi/ajax.go` 保留**
  —— 登录/注册/二次验证/找回密码的表单 POST 仍返回 PHP 风格信封（`{code,message,extra}`，
  无 `data`、无 request-id），契约不变；
- `DocMenuItem` 拆到 `httpapi/docmenu.go`（文档目录解析仍被 `/api/doc/tree` 使用）；
- `notFoundHTML` 常量留在 `httpapi/fallback.go`：没有 SPA 壳时兜底；
- **API 契约基线**（`internal/contract` + `testdata/contract*`）完全未动 —— 这是重写后唯一的
  功能回归网。

行为变化（有意）：

| 位置 | 旧行为 | 新行为 |
|---|---|---|
| `/user/*`、`/doc/*` 的 GET | SSR HTML | SPA 壳（`ServeSPA`），仅保留 302 语义（登录态跳转、doc 目录下钻） |
| 未知控制器（如 `/nope123` 之外的 `/user/nope`） | 404 HTML 页 | 方法缺失仍返回 JSON `code=501`（原契约）；控制器整体未知走 SPA 的 NotFound |
| `/lang/select` 参数错误 | 渲染"参数错误"静态页 | 302 回 `/`（由 SPA 决定去登录页或仓库列表） |
| `/captcha/*` | 存在但全站零引用 | 删除（含注册） |

门禁与回归：

| 项 | 结果 |
|---|---|
| Go：gofmt / build / linux 交叉编译 / vet / `go test ./internal/...` | **全绿**（doc 相关的 3 个测试已改为断言"下发壳 + `Cache-Control: no-cache`"） |
| 前端：`npm run typecheck && npm run lint && npm run build` | 全绿 |
| `docker build -t codefever-next:local .` | 成功（镜像不再包含旧前端） |
| `docker compose up -d` + 冒烟 | `/` 302、`/user/login` 200 壳、`/user/register` 200 壳、`/doc/cn` → 302 链 → 200 壳、`/repositories` 200 壳、`/healthz` 200、`/api/doc/tree` 200 JSON、`/api/user/info` 403、`/nope123` 200 壳（前端 NotFound） |
| headless 渲染（真机） | 登录页（欢迎登录/请输入邮箱/登 录）与文档页（`markdown-body` + h1 + 真实正文）均正确 |
| 视觉回归（真机 × 3 视口） | 登录卡片 **1195×659 / 551×659 / 461×659**（1184×644 + 阴影，与改造前一致）；文档页页头 **64px**（三视口一致） |

视觉回归的边界（如实记录）：需要登录态的 30+ 页面无法在无凭据的 headless 环境覆盖，
已在开发期用 mock 服务逐页做过 DOM 断言（见各批记录）；真机全量人工目检仍建议在
有测试账号的环境做一次。

### 批 7（三）—— UI 对齐修复（2026-10-24，用户反馈驱动）

用户反馈"UI 与旧版相差太大、布局错误、按钮颜色不对、hover tooltip 需要全局关闭"。
为避免凭感觉改，**从 git 历史里取回旧的 `www/view/build` 产物**，与新版并排跑在同一套 mock API 上，
逐页做像素/DOM 对比，再按差异修。

复现手段（可复用）：

```bash
git archive HEAD www/view/build www/static | tar -x -C /tmp/oldui
PREVIEW_ROOT=/tmp/oldui/www/view/build node web/scripts/serve-preview.mjs 5310   # 旧版
node web/scripts/serve-preview.mjs 5311                                        # 新版
```

已修复的**系统性**问题（都能用像素量测复验）：

| # | 问题 | 根因 | 修复 | 验证 |
|---|---|---|---|---|
| 1 | 按钮"全是实心蓝" | 我的 `Button` 默认 `variant='contained'`，而旧 MUI `Button` 默认是 **text** | 默认改回 `text`；逐页按旧版语义显式标注（列表主 CTA / 表单保存 / 弹窗确认 = contained，次要动作 = text） | 新 `/repositories`：contained×1 + text×2（与旧一致）；旧设置页"保存"实测是 contained，已按其修正 |
| 2 | 侧边导航被挤在内容里 | 侧栏渲染在 `.app-main` 的 24/48 内边距内 | `.page` 承担内边距、侧栏布局用 `.page--full` 满幅；`.cf-repo-main` 提供内容内边距 | 侧栏连续区间：旧 `0..318(319px)` / 新 `0..318(319px)` **完全一致** |
| 3 | 内容被推右 64px | 侧栏布局里"外层 page + 子 page"两次叠加内边距 | `.cf-repo-main > .page { padding: 0 }` | 首个实心按钮 x：旧 368 / 新 369（1px） |
| 4 | 设置页每块是"浮起的卡片" | `.section` 用了卡片阴影 | 改为**扁平**：白底 + 底部 1px 分隔线（与旧设置页一致）；需要卡片时用 `<Card>` | 截图对比：不再有阴影块 |
| 5 | hover tooltip 到处弹 | `Tooltip` 无条件渲染气泡 | 新增 `src/config/ui.ts` 的 `UI_CONFIG.tooltipsEnabled=false`，全局关闭（组件本体保留，改回 `true` 即恢复）；`IconButton` 同时去掉原生 `title` | `/repositories` DOM：tooltip 节点 **0**（旧版 26） |
| 6 | 页头"仓库/仓库组"只有图标 | 旧版是**带文字标签**的 text 按钮 | Header 改为 icon + 文字 | DOM 中可见"仓库""仓库组" |
| 7 | 表单按钮右对齐 | `FormActions` 用了 `flex-end` | 改 `flex-start`（旧设置页"保存"与表单左对齐） | 按钮 x 与旧版一致 |
| 8 | 两个页面还是"按计划在批 5 落地"的占位 | 全局 `/mergerequests` 与组内 `/groups/{名}/mergerequests` 未接 | 新增 `pages/MergeRequestBoard.tsx`（全局走 `repository/relatedMergeRequests`、组内走 `group/mergeRequests`）+ 抽出 `MergeRequestRow` 三处共用；删除 `ComingSoon` 页面 | 两个路由 DOM 中"按计划在"出现 **0** 次；列表正常渲染 `#12/#9` 等 |
| 9 | 个人资料页字段不全 | 只凭 i18n 猜字段，没读旧页 | 读 `git show HEAD:www/view/src/components/view/UserSettingGeneral.js`：旧页字段是 用户名/邮箱/团队/角色（只读），已按此补齐 | 字段数与旧一致 |
| 10 | 输入框过矮、整页偏短 | 只算了输入框内边距（盒子 33px），没算 MUI 默认 52px 的盒子高度 | 新增令牌 `--cf-input-height: 52px` 作为 `.cf-field__box` 的 `min-height`；表单节距 8→16px | 输入框高度：旧 52/54 → 新 51/51；行间距：旧 31 → 新 33（像素量测） |

尚未完成（需要逐页对齐，已具备可量测的对比手段）：

- 各页的**字段间距节奏**：旧版 MUI 表单每行约 72px（label + 输入 + helper），
  当前实现约 56px，导致整页高度偏短（例如个人资料页首个按钮 y：旧 492 / 新 342）。
  修法是把 `FormItem` 的行距调到旧值（或让 `Form` 的 gap 与 `TextField` 高度对齐旧尺寸）。
- 其余页面（仓库设置、Webhook、管理后台、MR 详情等）的字段集合需要按
  `git show HEAD:www/view/src/components/view/*.js` 逐个核对，而不是靠 i18n 键推断。

### 批 8 —— 旧版能力对齐（字段级核对落地，2026-10-24）

起因：用户要求"对齐旧版确保功能不变"。先做**逐页字段集合核对**（产物：
`docs/FRONTEND_FIELD_DIFF_2026-10-24.md`，设置族 → 仓库页 → 管理后台 → MR 四段，
626 行），再按核对结果补回旧版能力。核对报告同时找出 **13 个新版自带的缺陷**
（不是"缺旧版 UI"，是新版自己引入的），其中 4 个让整页不可用。

已落地（详见 `CHANGELOG.md` 的「新前端批 8」四条）：

- 设置族：MFA 二维码 + 更换设备、通知的组/仓库级开关矩阵、SSH 指纹与整行公钥容忍、
  `role` 恢复可编辑、主邮箱标注 + 120 秒重发倒计时；
- 仓库页：新建/Fork/新建组/建分支/建标签 6 个页面、仓库头 Fork + 克隆地址 + 6 项计数、
  分支·标签的增删与筛选、动态分类页签与翻页、文件页 README/submodule/blame/空仓库引导、
  提交列表日期分组与子目录路径；
- 管理后台：用户分类页签与排序、添加用户与初始口令回显、设管理员口令校验、
  组/仓库成员管理弹窗、系统设置结构化表单；
- MR：详情三 Tab + 右侧活动栏 + 版本对比 + 合并信息与 Squash + 评审状态徽标，
  列表/看板的「全部」页签与角标、排序分页、行字段、diff 汇总条与布局切换。

本批的偏离登记（AGENTS §7）：

| # | 偏离/代价 | 原因 | 回归条件 |
|---|---|---|---|
| 1 | ** Squash 勾选在新版才真正生效** | 旧前端把 `usingSquash` 以布尔下发，PHP 侧 `trim(false) === ''` → 恒走 squash，"取消勾选"是无效控件；新版传字符串 `'false'`，后端 `PHPTrim(...) != "false"` 才能识别 | 永久（后端不改；旧行为是缺陷而非基线） |
| 2 | **分支详情字段 `detail` 未补** | 后端 `domain.Branch` 没有 `detail` 列，旧版 `RefList.js:162` 对分支恒渲染空值 = 死代码 | 后端加 `detail` 列后补 |
| 3 | **blame 用 `?blame=1` 而非旧版 `/blame/{rev}/{path}` 路径** | `files/*` 单段路由放不下第二段 | 若要旧 URL 形状，加 `blame/*` 路由并改 `FileTree` 读 `useParams` |
| 4 | **图片 diff 并排对照未实现** | 后端 `DiffSide` 只有 `name/mode/sha`，diff 响应里没有图片 blob，无法取两侧图片 | 后端 diff 响应补图片地址后补 |
| 5 | **MR 详情无文件树侧栏与锚点跳转** | 需要「变更文件列表」数据源与锚点滚动；旧版 `MergeRequestDiff.js:148,323` | 后端补"MR 变更文件"端点后补 |
| 6 | **提交/活动分页用"多取 1 条"探测是否还有下一页** | `commitList` / `activities` 只返回数组，没有总数 | 后端加 `total` 字段后改为精确页码 |
| 7 | **`/userInfo` 整页不复原** | 后端 `user/update` 的 `field` 白名单只有 `name`/`tel`/`location`（`job`/`company` 因数据库无列被剔除），旧页核心字段无法提交 | 后端放开字段白名单后补 |
| 8 | **电话验证码链路整体下线** | 后端确实没有 `user/confirmTel`、`user/getCode` 两个端点 | 后端补端点后补（属新需求） |
| 9 | **仓库 Job(CI) 设置页不补** | 旧版 `RepositorySettingJob.js` 本身即死代码（无路由、无 API、无常量，渲染即崩），后端亦无端点 | 要 CI 属**新需求**而非补旧版 |
| 10 | **新建仓库组不提供"组类型"下拉** | 旧版判定是"无个人组则强制 type=2、组名取邮箱前缀"，没有可选项 | 后端放开组类型后补 |
| 11 | **子目录提交 URL 形状** | 保留旧版 `/{组}/{仓库}/commits/{rev}/*`；MR 行的分支链接同时支持 `?rev=` | 永久（两套并存） |
### 批 7（四）—— 组件目录化 + 按旧版实测值校准尺寸（2026-10-24）

用户反馈"按钮和输入框不是太大就是太小、没对齐、少东西"，并要求"每个组件一个目录，样式集中到 styles/"。

**1) 结构（已完成）**

```
src/components/unit/<Name>/index.tsx      # 27 个原子件，一组件一目录
src/components/view/unit/<Name>/index.tsx # 18 个业务共用件，同样一组件一目录
src/styles/index.css                      # 样式总入口（唯一 import 点）
src/styles/{global,components,layout,pages-*}.css
```
- 组件内部不再各自 `import './x.css'`，统一由 `src/styles/index.css` 按固定顺序引入
  （tokens → global → components → layout → pages），层叠顺序稳定；
- 设计令牌仍只在 `src/theme/theme.css`（AGENTS §5 的唯一字面值来源）；
- 目录化后 `@/components/unit/Button` 这类既有导入**不需要改**（目录 index 解析）。

**2) 尺寸校准（关键）**

做法：把旧产物从 git 取回，**注入探针脚本读出真实 computed style**（不是看图猜），
再对同一批选择器量新版，逐项对齐：

| 元素 | 旧版实测 | 校准前 | 校准后 |
|---|---|---|---|
| text 按钮 | h32 / pad `4px 24px 4px 8px` / color `rgba(0,0,0,.87)` | color `#7a859e`、pad `4px 8px` | **一致** |
| contained 按钮 | 104×32 / pad `2px 24px` / bg `rgb(52,85,219)` / 白字 | 104×32（颜色一致） | **一致** |
| 图标按钮 | 34×32 / pad `7px 8px` / color `rgba(0,0,0,.87)` | 34×34、color `#7a859e` | **一致** |
| 输入框 | 32px 高 / pad `6.5px 16px` | 52px（上一轮误放大）→ 34px | **32px**（border-box + 固定高） |
| Tab | h40 / pad `0 24px` / fs14 | 一致 | **一致** |
| 表头单元格 | pad `0 16px`、bg `#f1f3f7` | 一致 | **一致** |

顺带修掉的两个真 bug：

- 组件目录化后**全局样式入口丢失**（`main.tsx` 的 CSS import 被清理脚本一起删掉，
  构建不再产出 CSS，页面退化成浏览器默认按钮样式）——已恢复并加了构建产物校验点
  （`build/static/js/index-*.css` 必须存在）。
- 输入框高度改为 `box-sizing: border-box` 固定 32px（只靠 padding 会得到 34px）。

**3) 仍未对齐的部分（内容级，需要逐页做）**

"少东西 / 没对齐"多数是**字段与区块缺失**，不是 CSS 问题。方法已经有了：读
`git show HEAD:www/view/src/components/view/*.js` 的 JSX 逐页核对字段集合。已按此法修过
个人资料页（补 用户名/邮箱/团队/角色）。剩余页面（仓库设置族、Webhook、管理后台、MR 详情等）
需要逐页做，建议按"看板差异 → 补字段 → 复测"的顺序推进。

### 批 7（五）—— 修复"仓库文件列表不显示"（2026-10-24）

**现象**：进入仓库文件页，列表区域空白（mock 下正常，真实数据下空白）。

**根因**（用真实容器 API 实测确认，不是推断）：
`repository/object` 返回的 `type` **不是固定 `tree`**：

| 请求 | 真实返回 |
|---|---|
| `parent=main`（分支/提交） | `type: "commit"`，`object` 是条目数组 |
| `parent=<tree sha>`（进入子目录） | `type: "tree"`，`object` 是条目数组 |
| `parent=<blob sha>`（文件） | `type: "blob"`，`object` 是 `{mime,raw,…}` |

页面此前用 `view.type === 'tree'` 判断"是否目录"，于是**只有在子目录里才会显示列表**，
仓库根目录（type=commit）永远是空的。

**修复**：在 `api/types.ts` 增加类型守卫

```ts
export function isDirectoryView(view): view is ObjectView & { object: ObjectEntry[] } {
  return Boolean(view) && view.type !== 'blob' && Array.isArray(view.object)
}
```

`FileTree` 的三处判断（README 自动渲染、条目列表、目录/空态）全部改用守卫。

**验证**：

| 项 | 结果 |
|---|---|
| 真实容器 API 契约 | 根目录 `type=commit` + `[DESIGN.md(blob), styles(tree)]`；子目录 `type=tree` + 7 个文件；`pathStack?path=styles` 正常 |
| mock 已改为真实形状（`type:'commit'`）后渲染 | `README.md`×2（列表 + README 渲染）、`backend`、`go.mod` 均出现，列表恢复 |
| 类型/门禁 | `tsc` / `eslint` / `vite build` 全绿；镜像已重建并重启 |

**同时登记为陷阱**（AGENTS §9）：判断目录必须用 `isDirectoryView`，
不要用 `type === 'tree'`。
