# AGENTS.md

本文件是本仓库对 AI 协作代理（Agent）的**工作约定**。它描述的是"怎么做事"，
不是"项目是什么"——后者见 `README.md`、`docs/ARCHITECTURE_ANALYSIS.md`、
`PROJECT_MAP.md`。

适用范围：任何人（或代理）在本仓库做代码变更时都适用。规则是**默认约定**，
不是铁律，例外处理方式见 §7。

---

## 0. 术语

- **逻辑单元**：一件可以独立描述、独立验证、独立回滚的完整改动。
  例如"给列表接口加 pagesize 上限"是一个逻辑单元；"顺手把格式也整理了"不是。
- **门禁**：推送前必须全部通过的一组自动化检查（见 §8）。

---

## 1. 提交纪律：不主动 commit

**要求**

1. 完成一个**逻辑单元**后**停下来**，不要自行 `git commit`。
2. 提出**建议的 commit message**，并列出本次将纳入的文件/范围，等待用户确认。
3. 用户确认后，按确认的内容提交；用户改写的版本以用户的为准。
4. **例外**（可直接提交）：
   - 用户明确要求提交或推送；
   - 修复会让 CI 或默认分支立即变红的紧急缺陷——此时提交后**立刻说明原因**
     与影响面，而不是等确认。

**message 写法（本仓库既有惯例）**

- 中文正文 + Conventional Commits 前缀：`feat` / `fix` / `perf` / `refactor` /
  `docs` / `style` / `test` / `chore` / `build` / `ci`，冒号后一个空格，
  例：`fix(gitsvc): 去掉 LastCommit 里误用的 --left-only`。
- 标题只写**做了什么**；正文写**为什么这么做**（问题现象、根因、取舍），
  以及无法从代码直接看出的副作用。
- 一个提交只做一件事：格式化整理与行为变更必须分开。

**验收自检**：把 message 单独交给一个不了解上下文的人看，他能否说出
"改了什么、为什么改、影响哪里"。说不出就是正文太薄。

---

## 2. 变更日志：维护 `CHANGELOG.md`

**要求**

1. 任何**用户可见行为**的变更（功能、接口、性能、部署方式、构建命令）都必须
   记入根目录 `CHANGELOG.md`，与代码改动在同一个逻辑单元内完成。
2. 沿用既有格式：[Keep a Changelog 中文版](https://keepachangelog.com/zh-CN/1.1.0/)
   \+ SemVer。未发布的改动写在 `## Unreleased` 下，按
   `### 新增` / `### 变更` / `### 修复` / `### 移除` / `### 安全` / `### 性能` 归类。
3. 每条写清 **改了什么 + 为什么 + 影响面**（例如"admin 用户列表由 102 条查询
   降为 3 条"），不要只写文件名。
4. 纯内部重构、格式化、注释调整**不必**记；但若影响开发者的工作流
   （构建命令、环境变量、迁移方式），必须记在 `### 变更`。
5. 文件不存在时按同一格式创建。

**反例**：`- 修改 repository.go`（没有信息量）。

---

## 3. 项目地图：同步 `PROJECT_MAP.md` 与相关说明

**要求**

1. 出现以下任一变化时，在**同一逻辑单元**内更新 `PROJECT_MAP.md`：
   - 新增 / 删除 / 重命名模块、包、二进制、页面、组件目录；
   - 某个模块的**职责**发生变化（不只是改实现）；
   - **接口签名**变化（导出函数、HTTP 端点、组件 props、Redux action/state 字段）；
   - **调用链**变化（谁调用谁、经过哪几层、异步/事务边界变化）。
2. 同时同步相关说明文档：`docs/ARCHITECTURE_ANALYSIS.md`、
   `docs/IMPLEMENTATION_STATUS.md`（如涉及迁移进度）。
3. **保留历史**：废弃或迁移掉的条目**不要直接删除**，改为标注
   `（已废弃 YYYY-MM-DD：原因）` 或 `（已迁移至 X）`。
4. **保留人工备注**：`PROJECT_MAP.md` 里的"人工备注"段落由人维护，
   代理只能追加，不得重写或删改既有内容。
5. 文件不存在时创建（当前已有初始版本）。

---

## 4. 前端组件化：可复用能力必须组件化

**要求**

1. **禁止**在多个页面重复实现同一 UI 片段或交互逻辑。
2. 出现**第二次**就是抽取信号（不要等到第三次）。
3. 落位规则：
   | 内容 | 位置 |
   |---|---|
   | 通用 UI 原子件（按钮组、输入、空态、加载、复制、确认框） | `web/src/components/unit/` |
   | 业务域内共用件（仓库卡片、提交行、成员项） | `web/src/components/view/unit/` |
   | 纯逻辑（格式化、过滤、权限判定） | `web/src/helpers/` |
   | 常量与映射表（扩展名→语言、枚举、状态码、导航配置） | `web/src/config/` |
   | 数据访问（请求封装 + 端点） | `web/src/api/` |
   | 共享服务端状态 | `web/src/features/` |
   | 设计令牌（颜色/间距/圆角/阴影/字号的**唯一**字面值来源） | `web/src/theme/theme.css` |
4. 典型违规（本项目历史上真实出现过）：
   - 每个页面各写一遍"表格 + 分页 + 空态"；
   - 每个列表页各写一遍 200ms 搜索防抖；
   - 每个页面各写一遍 `JSON.stringify` 深比较做变更检测；
   - 每个页面各自 `new DateFnsUtils()` 做时间格式化。
5. 组件要**自包含**：样式、加载态、空态、错误态、边界值处理在组件内解决，
   不要把这些问题推给调用方。

---

## 5. 设计语言：所有页面遵守同一套视觉规范

**要求**

1. 下列视觉维度必须全站一致：**颜色、字体、字号、间距、圆角、边框、阴影、
   表格密度、按钮样式、弹窗样式、表单样式**。
2. 本项目的事实标准：**自研设计令牌 + 自研组件库**（2026-10 前端重写后：
   不再使用任何 UI 框架，MUI 已随旧 `www/view` 一起删除）。
   - 颜色/间距/圆角/阴影/字号**只允许**出现在 `web/src/theme/theme.css` 的 `:root`，
     组件与页面通过 `var(--cf-*)` 引用（TS 侧用 `web/src/theme/tokens.ts` 的类型化引用）；
   - 通用能力一律走 `web/src/components/unit/`，页面不得自绘按钮/输入框/表格/弹窗；
   - 间距用 `space(n)`（8px 单位，等价旧 `theme.spacing`）；栅格用现有 `group-grid` /
     `FormGrid`；断点写在组件 CSS 的 `@media` 里并复用既有断点值。
3. **禁止**硬编码视觉值：`#3455db`、`padding: 13px`、`border-radius: 5px`、
   自造阴影或表格行高等。需要新值时，先加到主题 / 样式变量，再在页面引用。
4. 按钮、弹窗、表单、表格一律复用既有组件或既有变体，**不要**为单个页面
   定制一套外观。
5. 有明确外部来源的常量（第三方品牌色、代码高亮主题）允许例外，
   但必须集中定义并注释来源。

**验收自检**：把两个不相干页面的截图并排放，按钮高度、圆角、间距节奏、
表头背景、弹窗标题字号应当看不出差异。

---

## 6. 页面职责：只写业务，不写通用能力

**要求**

1. 页面组件（`web/src/pages/**` 与 `web/src/components/view/**`）只负责三件事：
   - **业务字段**：这个页面展示哪些字段、怎么组装；
   - **业务动作**：提交什么、跳转到哪、调用哪些 data provider；
   - **模块差异**：该页与同族页面的不同之处。
2. 下列内容**必须**下沉，不得在页面内重新实现：
   - 通用交互：搜索、排序、分页、防抖、确认、复制、滚动、键盘快捷键；
   - 通用样式与布局脚手架：页面外壳、栅格结构、卡片容器、间距节奏；
   - 通用逻辑：格式化、解析、比较、权限判定。
3. 判定标准：页面文件中出现"与业务无关的结构性重复"即为违规。
4. 与 §4 的关系：§4 管"抽不抽"，
   §6 管"抽出去之后页面里还剩下什么"。抽出组件后，页面应当只剩下字段映射、
   动作绑定和少量差异分支。

---

## 7. 约定的效力：默认约定而非铁律

**要求**

1. 以上规则是**默认路径**，不是绝对约束。遇到合理例外可以偏离。
2. 偏离时必须**当场简要说明**，包含三要素：
   - 偏离了哪一条；
   - 为什么必须偏离（约束、风险、成本）；
   - 代价是什么，以及什么条件下应当回到规则路径。
3. **未说明的偏离视为违规**。
4. 同一个例外反复出现时，不要每次都临时解释——把它写成新规则或
   豁免清单的一次性条目，避免规则与现实的持续摩擦。

**示例**：`§5 要求颜色取主题变量，但本处必须用代码高亮主题里的 #032f62
（Prism 主题固定值），因此集中定义在 xxx 并注明来源。`

---

## 8. 推送前门禁：CI 与 Docker 必须完整跑通

**要求**

1. **任何推送之前**，本地必须逐条通过 CI 使用的**同一组**检查。
   不允许"先推上去看 CI 报什么"。
2. 后端 + 编排：**一条命令**跑完（与 `.github/workflows/ci.yml` 等价）：

   ```powershell
   pwsh -File scripts/verify.ps1              # gofmt / build / linux 交叉编译 / vet / test / compose config
   pwsh -File scripts/verify.ps1 -BuildImage  # 再加一步 docker build
   ```

   该脚本跑的是与 CI 相同的最小集，等价的手工命令如下（排查时用）：

   ```bash
   cd backend
   test -z "$(gofmt -l .)"                                 # 1. gofmt：非空即失败
   go build ./...                                          # 2. 本机编译
   GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./...     # 3. 交叉编译
   go vet ./...                                            # 4. vet
   go test ./internal/...                                  # 5. 单测
   cd .. && docker compose config > /dev/null              # 6. compose 校验
   ```

   > `verify.ps1` **不覆盖前端**。有 `web/**` 改动时必须另行执行第 3 条。

3. 前端（若有 `web/src`、`web/public`、`web/vite.config.ts` 等改动）：

   ```bash
   cd web
   npm run typecheck && npm run lint && npm run build
   ```

   **三条缺一不可**：`typecheck`（TS strict）+ `lint`（ESLint flat config）+
   `build`（Vite，产出 `web/build`）。旧版把 lint 藏在 webpack 构建期，
   换 Vite 后必须显式恢复成独立门禁，否则等于没有 lint。

   并且**必须把重建后的 `web/build` 一并提交**（见 §9）。

4. 涉及部署、镜像、数据库迁移的改动，额外要求：
   - `pwsh -File scripts/verify.ps1 -BuildImage` 成功；
   - `pwsh -File scripts/run-local.ps1 up` 起来后做冒烟验证：登录页 200、
     SPA 壳 200、未登录 `/api/user/info` 返回 401/403、迁移已记录在
     `cc_schema_migrations`、访问日志每个 `request_id` 只有一条；
   - 迁移在**已部署实例**上验证过，而不是只看全新安装。

5. 推送后**确认 CI 真的通过**（`gh run list --branch main` 或网页 Actions）；
   失败则立即修复，不允许把默认分支留在红色状态。

6. 需要真实 MySQL 的集成用例在 CI 里是 **skip** 的
   （`CODEFEVER_TEST_DSN` 未设置），因此**涉及 SQL / 事务 / 索引的改动
   必须在本地显式跑一遍集成用例**，不能以"CI 绿"作为通过依据：

   ```bash
   # 任选其一：本机 MySQL，或用容器接同一 compose 网络跑
   CODEFEVER_TEST_DSN="root:<pass>@tcp(127.0.0.1:3307)/codefever_smoke?parseTime=true" \
     go test ./internal/repo/ -run 'TestCountJoinedByUsersMatchesListJoined|TestRetentionServiceRun' -v
   ```


---

## 9. 本项目已知陷阱（踩过的坑，别再踩）

| 陷阱 | 正确做法 |
|---|---|
| 改了 `web/src` 却没重建 `build/` | `web/build` 是**入库产物**，Dockerfile 只 `COPY` 不构建前端；改 src 后必须 `npm run build` 并提交 |
| 从 `web/src/assets/icons/` 造 barrel 全量导入 | 会把 154 个图标全部打进产物（旧版踩过）。图标一律从 `web/src/config/icons.ts` 按需 import |
| 在页面/组件里写颜色、间距、圆角字面值 | 违反 §5：唯一允许的地方是 `web/src/theme/theme.css`；其他位置用 `var(--cf-*)` |
| 改了 Go 的页面 GET 却忘了 SPA 壳 | `/user/*`、`/doc/*` 的 GET 现在统一下发 `web/build/index.html`（`httpapi/spa.go`）；新增页面 GET 时直接调 `httpapi.ServeSPA` |
| 用 `view.type === 'tree'` 判断目录 | `repository/object` 在 `parent` 是**提交**时返回 `type: 'commit'`（最常见），只有 tree SHA 才返回 `'tree'`。判目录请用 `api/types.ts` 的 `isDirectoryView()`，否则文件列表会是空的（2026-10 真实踩过） |
| 壳被缓存导致发布白屏 | 壳必须 `Cache-Control: no-cache`（引用的是带哈希的资源名）；`/static/js|css` 才是一年 immutable |
| 把新索引写进 `backend/deploy/sql/db_structure.sql` | 该文件只在**全新安装**时导入；对已部署实例无效。新迁移必须放 `backend/internal/db/migrations/*.sql` |
| 想改 `testdata/pages/php-*.html` 让页面测试通过 | 该快照与 `pages_test.go` 已随 SSR 页面在 2026-10 前端重写中删除（`（已废弃 2026-10-24：前端重写）`）。API 契约基线仍在 `backend/internal/contract` + `testdata/contract*`，那才是现在的功能回归网，不要动它 |
| 用 `curl` 单点采样当性能基线 | 用 `backend/cmd/stress`；且必须先关 `general_log`（开启会把延迟放大 4–8 倍） |
| 在事务里做 git / 网络 / 大文件 IO | 会独占连接池（`MaxOpenConns=25`）导致全站阻塞；耗时操作必须在事务外 |
| 直接使用用户传入的 `pagesize` | 必须经 `domain.ClampPaging` 收敛（上限 100），否则可能拼出无上限 `LIMIT` |
| MySQL 多表 `DELETE ... JOIN ... LIMIT` | 语法不支持；改成单表 `DELETE` + `NOT EXISTS` 子查询 |
| 在 `shouldComponentUpdate` 里 dispatch / setState | 渲染前改 store 会形成重渲染回环；副作用放 `componentDidUpdate` |
| Redux reducer 对无关 action 返回 `{...state}` | 会让根引用每次变化，全部 connected 组件白重渲染；无关 action 原样 `return state` |

---

## 10. 提交前自查清单

复制到 PR / 提交说明里逐条打勾：

- [ ] 一个逻辑单元，没有混入无关改动
- [ ] 已提出 commit message 并**获得确认**（或属于 §1 的例外）
- [ ] `CHANGELOG.md` 已更新（若为用户可见变更）
- [ ] `PROJECT_MAP.md` 已同步（若模块 / 接口 / 调用链变化），历史条目只标注不删除
- [ ] 复用的 UI 与交互已组件化，没有在页面里重复实现
- [ ] 视觉值取自主题 / 样式变量，无硬编码颜色、间距、圆角
- [ ] 页面只含业务字段、业务动作、模块差异
- [ ] §8 的六个后端门禁本地全绿
- [ ] 前端改动：`npm run typecheck && npm run lint && npm run build` 全绿，`web/build` 已重建并纳入提交
- [ ] 涉及数据库：迁移落在 `migrations/`，集成用例本地跑过
- [ ] 涉及部署：`docker compose build` 成功且容器冒烟通过
- [ ] 若有偏离规则之处，已在说明中写清"哪一条 / 为什么 / 代价"
