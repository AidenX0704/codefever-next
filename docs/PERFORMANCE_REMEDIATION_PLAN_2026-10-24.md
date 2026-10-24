# CodeFever Next —— 性能问题整改方案

- **编制日期**：2026-10-24
- **输入**：`docs/PERFORMANCE_AUDIT_2026-10-24.md`（审查报告，874 行）
- **方法**：对报告逐条源码复核 + 对运行中容器（`codefever-next-app` / `codefever-next-db`）重新实测 + 三方独立验证（后端 / 前端 / 传输层）
- **代码基线**：`1fd3246`（工作树仅多出审查报告本身）
- **实测环境**：MySQL 8.0.46（3 用户 / 4 仓库 / 13 事件 / 13 活动 / 0 MR），nginx 1.22.0，Go 连接池 25

---

## 0. 一页结论

**报告的诊断方向基本正确，但基线数字有 3 处测量错误、6 处优先级错配，另有 3 个更高危的问题被漏掉。按原优先级执行会浪费约 60% 的工时在收益 < 1 ms 的项目上，同时把真正的风险（任意请求可拉起 2 万个 git 子进程、连接池耗尽）排在第 7 位。**

修正后的判断：

1. **当下唯一影响用户体感的是传输层**：1,699 KB 未压缩 JS + 零 `Cache-Control`。nginx 主配置里 gzip 是注释状态（`#gzip  on;`），改 6 行配置 + 1 行 webpack 配置，**首屏从 1,699 KB 降到 445 KB（-74%）**。
2. **后端"性能问题"当前不成立，但"规模风险"成立**：服务端实测 2–16 ms，不在用户可感知区间。真正的后端第一风险不是报告定的 B3，而是**全站 `pagesize` 无上限**（`?pagesize=20000` ⇒ 2 万次 SELECT + 2 万个 `git log` 子进程），以及 **git 操作在事务内 + git 超时 10 min 而 HTTP `WriteTimeout` 120 s**。
3. **报告 §5.1「构建产物与源码不同步」是误判，必须撤销**。source map 的 `sourcesContent` 证明 131 个被打包模块与当前 `src/` 逐字节一致（含 `MuiPickersUtilsProvider` 接线）；报告用「minified 产物里搜 `pickers` 0 命中」推断陈旧，实际是 terser 混淆了标识符。**阶段 0「重新构建前端」可以从计划里删掉**，报告的 bundle 数字有效。
4. **报告 §1.4 的「DB 单次往返 4.24 ms」是测量方法错误**（50 次独立 `mysql` CLI 进程 ≈ 5.8 ms/次进程启动开销）。同一连接内 2,000 条 `SELECT 1` 耗时 147 ms = **0.07 ms/条**。因此「67 ms ÷ 4.24 ms ≈ 16 次串行往返」这个核心推论不成立，**每多一条查询的真实成本 < 0.5 ms**。
5. **报告 §1.5 的 API 延迟被放大约 4–8 倍**，高度可能是**在 `general_log = ON` 状态下采样的**（报告 §8 自己要求开启 general_log 做验证）。我实测同一容器：开启 general_log 时 `/api/repository/list` = 64–139 ms，关闭后 = **p50 16 ms**。

**一句话**：先做传输层（0.5–1 人日拿 74% 收益），再做后端规模风险（防雪崩），最后才做前端分包与渲染（当前体感已被前两步吃掉大半）。

---

## 1. 复测结果：报告数字 vs 实测数字

所有实测均在 `general_log = OFF` 状态下采集（这是关键前提，见 §1.2）。

### 1.1 DB 往返：4.24 ms → 0.07 ms

| 测量方式 | 结果 | 说明 |
|---|---:|---|
| 报告：50 次独立 `mysql -e "SELECT 1"` | 212 ms ⇒ 4.24 ms/次 | **含进程启动 + 连接建立**，不是往返 |
| 本次：50 次独立 `mysql` 进程 | 291 ms ⇒ 5.8 ms/次 | 复现了报告的测量条件，量级吻合 |
| 本次：**同一连接**连发 2,000 条 | 147 ms ⇒ **0.07 ms/条** | 这才是 `database/sql` 池内复用的真实成本 |

> **结论**：Go 侧多一次池内查询 ≈ 0.07 ms 网络 + 驱动/解析开销，实测端到端每 SQL ≈ 0.3–0.5 ms。报告 B3 的「单页 430 ms」应为 **~50 ms**；B1 的「+8.5 ms」应为 **~1 ms**。

### 1.2 API 延迟：55–133 ms → 2–16 ms

`general_log` 与 `log_output=TABLE` 对每请求 20+ 条 SQL 的写入开销是数量级的：

| 端点 | 报告 | 实测 general_log=ON | 实测 **general_log=OFF** | 服务端自报 `duration_ms` |
|---|---:|---:|---:|---:|
| `/api/user/info` | 55 ms | — | 12–18 ms | **2 ms** |
| `/api/group/list` | 56 ms | — | 10–17 ms | **4–5 ms** |
| `/api/repository/list` | 67 ms | 64–139 ms | **p50 16 ms** | **9–12 ms** |
| `/api/admin/userList` | ~430 ms（推算） | 77–121 ms | **p50 14 ms** | 9 ms |
| `/api/admin/systemStatus` | 133 ms | — | 17–22 ms | **12–16 ms** |

（含 nginx 的 curl 端到端 = 服务端 + 8~12 ms nginx/docker 网络开销）

> **结论**：服务端 handler 耗时 2–16 ms，**不在用户可感知区间**。报告基于 55–133 ms 得出的"后端 P0"定级不成立。
> **同时这是一条整改纪律**：今后任何性能基线必须在 `general_log = OFF` 下采集，且优先用 `cmd/stress`（见阶段 B）而非 curl。

### 1.3 首屏体积：报告数字基本准确

| 文件 | 原始 | 实测 gzip -6 | 实测 brotli -q5 |
|---|---:|---:|---:|
| `1.aa438dd3.chunk.js`（vendor） | 1,131,004 | 353,609 | 323,913 |
| `main.3a8cdea3.js`（应用） | 568,343 | 99,160 | 86,581 |
| `1.75563d44.chunk.css` | 3,628 | 1,407 | 1,242 |
| **合计** | **1,702,975** | **454,176（-73.3%）** | **411,736（-75.8%）** |

报告说 448.8 KB gzip，实测 454.2 KB，偏差 1.2%，可接受。

### 1.4 传输层：报告的 P0-1 / P0-2 完全确认

```
nginx -V   → --with-http_gzip_static_module --with-http_v2_module（无 brotli 模块）
nginx -T   → 第 30 行：`#gzip  on;`   ← 基础镜像里是注释状态
curl -I    → Content-Encoding: (无)   Cache-Control: (无)   只有 ETag/Last-Modified
```

- `/static/` location 只有 `rewrite` + `try_files`，无任何缓存头 ⇒ 每次访问都走一次协商（304）。
- `build/index.html` 末尾两个 `<script>` 无 `defer`。
- 登录页 19 个子资源全部阻塞：8 CSS（bootstrap 159 KB、fontawesome 72.7 KB）+ 8 JS（jQuery 89.5 KB、intlTelInput 83.7 KB、bootstrap.bundle 80.7 KB、marked 47.6 KB）+ 图片（`activity_pgyer_codefever_.png` 147.9 KB，LCP 元素）。

### 1.5 报告需要修正的 6 处判断

| # | 报告原文 | 修正 |
|---|---|---|
| 1 | §5.1 构建产物不同步，需先重新构建 | **误判，撤销**。131 个模块 `sourcesContent` 与磁盘一致；`pickers` 搜不到是 terser 混淆所致 |
| 2 | B6「pagesize 固定 20 ⇒ 20 查询 + 20 子进程」 | **前提错误**。`paging()` 只设下限不设上限（`handlers/repository.go:125-127`、`repo/repository.go:685-687`），`?pagesize=5000` 就是 5000 次 SELECT + 5000 个 git 进程 |
| 3 | B12「admin 分页 pagesize 无上限」 | 范围过窄，**全站所有端点都无上限**（`handlers/group.go:345,435` 同样） |
| 4 | B4 建仓与 fork 同等风险 | **建仓无风险**（`git init --bare` 毫秒级），只有 fork 的 `CloneBare` 有连接池风险 |
| 5 | B5 事务内跨连接 1 处 | 实际 **5 处**（`:2313`、`:2372`、`:2425`、`:2477/2479`、`:2647/2651`） |
| 6 | F8「原地 `.sort()` 修改 state」 | 该 `.sort()` 作用在 `.filter()` 副本上，**不改 state**；真实成本是每渲染重排序 + 重建全部行 JSX |

另有两处技术性错误：**B14** 的 `DISTINCT` 去掉的是 JOIN 扇出（一个组含多个命中仓库），不是「g_key 已是主键」；**B11** 的 11 项缺索引里 9 项只是组合索引（单列索引已存在），真正的全表扫描只有 `cc_users.u_tel` 与 `u_status`。

### 1.6 报告漏掉的 4 个更高危问题

| # | 问题 | 证据 | 风险 |
|---|---|---|---|
| **N1** | **`pagesize` 全站无上限** | `handlers/repository.go:125-127`、`repo/repository.go:685-687`、`handlers/group.go:345,435`；`domain/admin.go:59` 的 `AdminPageSizeMax` 全仓库 0 引用 | `GET /api/repository/mergeRequests?pagesize=20000` ⇒ 2 万行 + 2 万次 `r_path` 查询 + **2 万个 `git log` 子进程**。单请求即可打死机器 |
| **N2** | **git 超时与 HTTP 超时不匹配** | `gitsvc/service.go:40` `defaultCommandTimeout = 10 * time.Minute` vs `cmd/codefever-api/main.go:251` `WriteTimeout: 120s` | 卡住的 clone/merge 会在客户端已断开后继续占住 25 个池连接中的 1 个，长达 10 分钟 |
| **N3** | **4 张追加型表全无保留期** | grep `DELETE FROM`：只有 `cc_sessions` 有清理（`main.go:106`）。`cc_events`、`cc_activities`、`cc_notification_content/users`、`cc_repository_webhook_logs` 永久增长 | 报告 B8 只说了 `cc_events`，实际是 5 张表 |
| **N4** | **LIKE 通配符未转义** | `repo/repository.go:623`、`:681` 直接 `"%"+keyword+"%"`（同库其它位置都用 `escapeLike`） | `?keyword=%` 变成裸通配符，把已收窄的查询重新全表扫 |

前端另有一处**潜在运行时崩溃**：`reducers/NotificationStates.js:56` 用 `delete arr[i]` 在数组上留空洞，`:60` 的 `shift()` 可能返回 `undefined`，`:62` 的 `newItem.data.title` 会抛异常。

---

## 2. 修正后的优先级矩阵

| 优先级 | 项 | 当前实测状态 | 为什么是这个优先级 | 工作量 |
|---|---|---|---|---|
| **P0** | I1 静态资源压缩 | `Content-Encoding` 为空，nginx 主配置 gzip 注释 | 1,699→454 KB，6 行配置，零代码风险 | 0.5 d |
| **P0** | I2 `Cache-Control` 分级 | 只有 ETag，无 Cache-Control | 二次访问从"全量协商"变"零请求" | 0.5 d |
| **P0** | I3 SPA 脚本 `defer` | index.html 两个阻塞 script | 解阻塞 1.7 MB 下载 | 0.5 d |
| **P0** | I4 登录页 19 阻塞资源 | 已核实 785 KB / 19 资源 | 未登录访客的第一屏，也是注册转化路径 | 1 d |
| **P1** | **N1 pagesize 全站上限** | 无上限 | 单请求可拉起 2 万子进程；**必做** | 0.5 d |
| **P1** | B4/B5/N2 git 出事务 + 超时对齐 | 5 处跨连接，git 超时 10 min | 连接池耗尽 = 全站阻塞（唯一会"整体挂掉"的项） | 1.5 d |
| **P1** | B3 admin userList N+1 | 5 SQL/用户，且只为取 `len()` | 管理端随用户数线性劣化 | 1 d |
| **P1** | B7 通知 worker 2×COUNT/人 | 确认，无短路 | 200 人群组 = 400 查询/事件，worker 是单点 | 0.5 d |
| **P1** | N3/B8 保留期清理 | 5 张表无 DELETE | 数据单调增长，越晚做越痛 | 1 d |
| **P2** | B11/B10 索引与检索 | 真缺 11 项（9 项为组合索引） | 当前数据量无感，是给增长买的保险 | 1 d |
| **P2** | B1 鉴权双查 | 确认（general_log 有直接证据） | 只省 ~1 ms，但改动 5 行且顺带清理语义 | 0.5 d |
| **P2** | B2 admin 双中间件链 | 确认；**根因已在 `contract.Parse` 修复** | 整段可**删除**，不是重构 | 0.5 d |
| **P3** | F1 路由分割 | 0 lazy / 2 个 chunk | 454→~250 KB，但要先有阶段 A 兜底 | 2 d |
| **P3** | F6 依赖裁剪 | `@pgyer/icons` 154/154、prism 56 语言 | ~220 KB raw，逐个可验证 | 2 d |
| **P3** | F3/F4/F5/F9 渲染 | 0 memo、20+ 处 `JSON.stringify` 深比较 | INP 问题，需 Profile 驱动 | 3 d |
| **不做** | I5 换基础镜像、I10 SW、F13 虚拟化、F19 `loading=lazy` | — | 见 §6 | — |

---

## 3. 分阶段实施方案

### 阶段 A —— 传输层（P0，约 2 人日，零 Go 代码改动）

> 收益：首屏 JS **1,699 KB → 454 KB（-73%）**；二次访问静态资源 **0 请求**。这是整个方案里唯一"立刻能感知"的一步。

#### A1. nginx 开启 gzip（`backend/deploy/docker/vhost-go.conf-template`，`server {` 内）

```nginx
    # 压缩：基础镜像主 nginx.conf 里 gzip 是注释状态（nginx -T 可见 `#gzip  on;`），
    # 因此必须在 vhost 显式开启。镜像已带 --with-http_gzip_static_module，
    # 若后续在构建期预压 .gz，可省掉每请求的压缩 CPU。
    gzip              on;
    gzip_vary         on;
    gzip_comp_level   6;
    gzip_min_length   1024;
    gzip_proxied      any;
    gzip_types        text/plain text/css text/xml
                      application/javascript application/json application/xml
                      image/svg+xml application/font-woff application/font-woff2
                      application/vnd.ms-fontobject;
    gzip_static       on;
```

**关于 brotli**：`nginx -V` 显示**没有 brotli 模块**，而实测 brotli 相对 gzip 只多省 42 KB（454→412 KB，-9%），不值得为它重编 nginx 或换基础镜像。**本方案不做 brotli。**

> ❌ 不要把 `image/png`、`image/jpeg`、`font/woff`（非 woff2）放进 `gzip_types` —— 这些格式已压缩，再压反而变大。

#### A2. `Cache-Control` 分级（同一文件，替换现有 `/static/` location）

```nginx
    # 带内容哈希的构建产物 —— 永久缓存（文件名变 ⇒ URL 变）
    location ^~ /static/js/ {
        rewrite "(.*)\.map(.*)" "/static/not/found" last;
        add_header Cache-Control "public, max-age=31536000, immutable" always;
        try_files $uri /app$uri 404;
    }
    location ^~ /static/css/ {
        rewrite "(.*)\.map(.*)" "/static/not/found" last;
        add_header Cache-Control "public, max-age=31536000, immutable" always;
        try_files $uri /app$uri 404;
    }

    # 其余静态资源（含登录页的 /static/vendor/ 与 /static/{ReleaseNumber}/）
    location ^~ /static/ {
        rewrite "(.*)\.map(.*)" "/static/not/found" last;
        rewrite "^/static/\d{14}/(.*)$" "/static/$1" last;
        add_header Cache-Control "public, max-age=604800, stale-while-revalidate=86400" always;
        try_files $uri /app$uri 404;
    }

    # SPA 回退 —— index.html 必须每次都校验，否则发布后用户拿到旧壳
    location / {
        add_header Cache-Control "no-cache" always;
        try_files $uri /app/index.html;
    }
```

**两个必须注意的 nginx 语义**：

1. **不要同时写 `expires` 和 `add_header Cache-Control`** —— 会输出两条 `Cache-Control`。只用 `add_header`。
2. `add_header` **不继承**：location 里一旦写了 `add_header`，server 级的 `add_header` 就全部失效。当前 vhost 没有 server 级 `add_header`，所以安全；以后新增安全头（如 `X-Content-Type-Options`）必须同步加到每个 location 或改用 `headers-more` 模块。

#### A3. SPA 脚本 `defer`（`www/view/config/webpack.config.js:593`）

```js
      new HtmlWebpackPlugin(
        Object.assign(
          {},
          {
            inject: true,
            scriptLoading: 'defer',   // ← 新增，一行
            template: paths.appHtml,
          },
```

已确认 `html-webpack-plugin@4.5.0` 支持该选项（`node_modules/html-webpack-plugin/index.js:712` `defer: this.options.scriptLoading !== 'blocking'`），默认值是 `'blocking'`，正是当前问题的来源。

**重建前端的环境要求（关键，否则构建会失败）**：本机 `node v24.13.0` + `webpack 4.44.2` 会因 md4 哈希报 `ERR_OSSL_EVP_UNSUPPORTED`，必须：

```bash
cd www/view
GENERATE_SOURCEMAP=false NODE_OPTIONS=--openssl-legacy-provider yarn build
```

> 顺带解决 I6/F11：`GENERATE_SOURCEMAP=false` 让镜像少带 6.5 MB 的 `.map`（`main 1.53 MB` + `vendor 4.97 MB` + `css 7.3 KB`）。这些 `.map` 目前由 nginx `rewrite` 挡住不外泄，但仍随镜像分发并暴露源码。

#### A4. 登录页关键路径（`backend/internal/httpapi/templates/`）

`footer_include.html:8-15` 的 8 个 vendor script 全部阻塞渲染。**但 `login.html:74` 的内联脚本在解析期就调用 `jQuery.validator` / `intlTelInput` / `$`**，所以不能简单加 `defer`。分两步：

**第 1 步（无逻辑风险，先做）**
- `marked.min.js`（47.6 KB）只在 doc 页需要，从 `footer_include.html` 移到 `doc_detail.html`；
- `intlTelInput.js`（83.7 KB）+ `intlTelInput.css`（24.5 KB）只在注册/登录/重置密码的输手机号页面需要；
- `activity_pgyer_codefever_.png`（147.9 KB，LCP 元素）→ WebP（同视觉质量约 30–40 KB），并加 `fetchpriority="high"`；
- `flags@2x.png`（174 KB）+ `flags.png`（71 KB）改为 `loading="lazy"`（在 `intlTelInput` 下拉里，非首屏可见）。

**第 2 步（需回归测试）**
- 先把 `login.html:74` / `register.html` / `reset_password.html` 的内联脚本整体包进 `document.addEventListener('DOMContentLoaded', function () { … })`；
- 再给 `footer_include.html` 全部 8 个 script 加 `defer`。

#### A5. 验收（必须逐条打勾）

```bash
# 1) 压缩生效
curl -sI -H 'Accept-Encoding: gzip' http://localhost:9080/static/js/1.aa438dd3.chunk.js \
  | grep -iE 'content-encoding|content-length|cache-control|vary'
# 期望：content-encoding: gzip，content-length ≈ 353609，cache-control: public, max-age=31536000, immutable

# 2) 首次访问总量（应 ≤ 460 KB）
curl -s -H 'Accept-Encoding: gzip' -o /dev/null -w '%{size_download}\n' \
  http://localhost:9080/static/js/1.aa438dd3.chunk.js

# 3) 二次访问命中强缓存（带 If-None-Match 应仍返回 200 且不带 body，因为 immutable）
curl -sI http://localhost:9080/static/js/main.3a8cdea3.js | grep -i cache-control

# 4) index.html 不被缓存
curl -sI http://localhost:9080/admin/client | grep -i cache-control   # 期望 no-cache

# 5) 登录页资源数与体积
curl -s http://localhost:9080/user/login | grep -oE '(src|href)="/static/[^"]+"' | wc -l
```

**回滚**：只改模板文件，`git checkout backend/deploy/docker/vhost-go.conf-template` 后 `docker compose up -d --build codefever` 即可。

---

### 阶段 B —— 建立度量门禁（P0.5，约 1 人日）

> 报告 F10 说得对：没有 CWV 数据，所有前端结论都无法持续追踪。但项目里**已经有比报告所用方法更好的工具**——`cmd/stress` 自带 QPS / p50 / p95 / p99 / 按端点分布 / pprof 采样。**先把门禁建起来，再做后续优化**，否则无法证明任何改动有效。

#### B1. 接入 `web-vitals`（`www/view/src/index.js`，约 10 行）

```js
import { getCLS, getFID, getFCP, getLCP, getTTFB } from 'web-vitals'

function report(metric) {
  const body = JSON.stringify({
    name: metric.name, value: metric.value, id: metric.id,
    rating: metric.rating, path: location.pathname,
  })
  navigator.sendBeacon && navigator.sendBeacon('/api/web-vitals', body)
}
getCLS(report); getFID(report); getFCP(report); getLCP(report); getTTFB(report)
```

需要在 Go 侧加一个收口端点（`POST /api/web-vitals`，写日志即可，不必入库），否则 `sendBeacon` 会 404。

#### B2. 开启慢查询日志（`docker-compose.yml` 的 `db.command`）

```yaml
      - "--slow_query_log=1"
      - "--long_query_time=0.2"
      - "--slow_query_log_file=/var/lib/mysql/slow.log"
```

> 报告 I7 完全正确：当前 `slow_query_log=0` + `long_query_time=10s`，等于**生产无任何慢查询可观测性**。

#### B3. 用 `cmd/stress` 固化基线（**不要再用 curl 单点采样**）

```bash
cd backend
go run ./cmd/stress -base http://127.0.0.1:9080 -duration 60s -concurrency 16 \
    -email root@codefever.cn -password 123456 -out baseline-before.json
```

**纪律（本次复测最大的教训）**：
- 采基线前必须 `SET GLOBAL general_log = OFF;`（`general_log=ON` 会把 `/api/repository/list` 从 16 ms 抬到 64–139 ms）；
- 同一 URL 至少 3 轮取中位数；
- 前后对照用同一个工具、同一份负载、同一并发。

#### B4. 后端回归工具（阶段 C/D 的安全网）

- `backend/cmd/smoke-write`：以 **multipart/form-data** 驱动全部写端点成功路径并断言 → 阶段 A4 / C6 / C7 的回归依据；
- `backend/cmd/contract-compare` / `contract-collect`：契约基线，保证 B1/B2 这类"改鉴权路径"的改动不改变错误码语义。

---

### 阶段 C —— 后端规模风险（P1，约 4 人日）

> 目标不是"更快"（当前 2–16 ms 已经不慢），而是**取消"单请求打挂全站"的可能性**。

#### C1. 【最高优先】`pagesize` 全站上限

**现状**：`domain/admin.go:59` 声明了 `AdminPageSizeMax = 100`，**全仓库 0 引用**；`paging()` 只保证下限。

**改法**：在 `internal/domain` 新增统一入口，三个调用点全部改用它。

```go
// internal/domain/paging.go（新增）
const (
    PageSizeDefault = 20
    PageSizeMax     = 100   // 任何端点都不得超过
)

// Clamp 把分页参数收进合法区间。page 至少 1，pagesize 落在 [1, PageSizeMax]。
func Clamp(page, pagesize int) (int, int) {
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
```

替换点：
- `handlers/repository.go:125-127` `paging()`
- `handlers/group.go:345-350`、`:435-440`
- `domain/admin.go:203-211` `AdminPaging()`

> ⚠️ 这是**行为变更**：前端若有用 `pagesize>100` 的地方会开始被截断。先 `grep -rn "pagesize" www/view/src` 确认（当前前端只用默认 20/N×20 累加，安全）。同时 `contract-compare` 需要更新对应基线。

#### C2. B3 —— admin userList 的 N+1 与"取全量只为 `len()`"

**现状**（`handlers/admin.go:231-233`）：每个用户调一次 `ListJoined`（5 条 SQL），**但结果只用来 `len(projects)`**（`:241`）—— 把用户可见的全部仓库行都传回进程只为数个数。

**改法**：新增一个批量计数方法，一次查询拿全部用户的仓库数。

```go
// internal/repo/repository.go（新增）
// CountJoinedByUsers 批量统计每个用户可见的仓库数（自有 + 组内 + 成员）。
// 语义与 ListJoined 的去重口径保持一致，但只返回计数。
func (r *Repository) CountJoinedByUsers(ctx context.Context, uKeys []string) (map[string]int, error) {
    if len(uKeys) == 0 {
        return map[string]int{}, nil
    }
    // 注意：inPlaceholders 返回 ([]string, []any)，调用方需自行 Join
    // （与 repository.go:270/296/324 的既有写法一致）。
    ph, args := inPlaceholders(uKeys)
    in := strings.Join(ph, ", ")
    q := `
        SELECT u_key, COUNT(DISTINCT r_key) FROM (
            SELECT g.u_key AS u_key, r.r_key AS r_key
              FROM cc_repositories r
              JOIN cc_groups g ON g.g_key = r.g_key
             WHERE r.r_status = 1 AND g.g_status = 1 AND g.u_key IN (` + in + `)
            UNION ALL
            SELECT rm.u_key, r.r_key
              FROM cc_repository_members rm
              JOIN cc_repositories r ON r.r_key = rm.r_key
             WHERE rm.rm_status = 1 AND r.r_status = 1 AND rm.u_key IN (` + in + `)
            UNION ALL
            SELECT r.u_key, r.r_key
              FROM cc_repositories r
             WHERE r.r_status = 1 AND r.u_key IN (` + in + `)
        ) t
        GROUP BY u_key`
    allArgs := append(append(append([]any{}, args...), args...), args...)
    rows, err := r.db.QueryContext(ctx, q, allArgs...)
    /* …扫描成 map… */
}
```

（若认为上述 SQL 与 `ListJoined` 的去重口径难以逐字对齐，**退化方案**：只把组内仓库与成员仓库两次查询批量化，`ListJoined` 保持不变，也能把 `2 + 20×5 = 102` 降到 `2 + 3 = 5`。）

改完 `handlers/admin.go:231-241` 改为一次调用 + `counts[u.Key]`。

**验收**：`POST /api/admin/userList` 的 SQL 条数从 `2 + N×5` 降到 `3`；返回的 `projects` 数值与改造前逐用户比对一致。

#### C3. B7 —— 通知 worker 的 2×COUNT(*)/收件人

**现状**（`events/notification.go:609-621`）：`for _, u := range affected` 内两次 `HasRefused`，各是一条 `COUNT(*)`，且**没有短路**（`gKey`/`rKey` 为空时也查）。

**改法**：一次批量取回所有被拒绝的组合，在内存里判定。

```go
// internal/repo/notification.go（新增）
// RefusedPairs 批量返回 u_key → {组级拒绝, 仓库级拒绝}
func (r *Notification) RefusedPairs(ctx context.Context, uKeys []string, gKey, rKey string) (map[string]struct{ G, R bool }, error) {
    out := make(map[string]struct{ G, R bool }, len(uKeys))
    if len(uKeys) == 0 || (gKey == "" && rKey == "") {
        return out, nil   // ← 短路，修掉报告的"空 gKey/rKey 也查两次"
    }
    in, args := inPlaceholders(uKeys)
    inStr := strings.Join(in, ", ")
    cond := []string{}
    if gKey != "" {
        cond = append(cond, "(nr_type = 1 AND g_key = ?)")
        args = append(args, gKey)
    }
    if rKey != "" {
        cond = append(cond, "(nr_type = 2 AND r_key = ?)")
        args = append(args, rKey)
    }
    q := `SELECT u_key, nr_type FROM cc_notification_refuse
           WHERE nr_status = 1 AND u_key IN (` + inStr + `) AND (` + strings.Join(cond, " OR ") + `)`
    /* …扫描，按 nr_type 填 G/R… */
    return out, nil
}
```

**验收**：200 人仓库的一次通知事件，SQL 条数从 `400+` 降到 `1`。

#### C4. B18 —— `uKeys` 未去重就进 `IN (...)`

`handlers/repository.go:1564-1598` 把仓库 owner + 组 owner + 全部成员逐个 append（组成员在多个仓库下会被重复加入）。在 `inPlaceholders`（`repo/repository.go:900-908`，注意当前签名是 `func inPlaceholders(keys []string) ([]string, []any)`）里做一次去重即可，**同时保护所有调用方**：

```go
func inPlaceholders(keys []string) ([]string, []any) {
    seen := make(map[string]struct{}, len(keys))
    placeholders := make([]string, 0, len(keys))
    args := make([]any, 0, len(keys))
    for _, k := range keys {
        if _, ok := seen[k]; ok {
            continue
        }
        seen[k] = struct{}{}
        placeholders = append(placeholders, "?")
        args = append(args, k)
    }
    return placeholders, args
}
```

#### C5. B4 / B5 / N2 —— git 出事务 + 超时对齐

**(a) fork 的 `CloneBare` 移出事务**（`handlers/repository_write.go:568-586`）——保留"无仓无行"语义：

```go
// 先建裸仓库（耗时操作，不持事务）
if err := h.Git.CloneBare(ctx, absPath, sourcePath); err != nil {
    _ = h.Git.RemoveRepo(ctx, absPath)
    return nil, contract.NewError(contract.CodeOperateFail)
}
// 再用短事务写库
err := h.Repos.WithTx(ctx, func(tx repo.RepositoryTx, raw *sql.Tx) error {
    if err := tx.Create(ctx, newRepo); err != nil { return err }
    return h.emit(ctx, raw, domain.EventTypeREPO_CREATE, …)
})
if err != nil {
    _ = h.Git.RemoveRepo(ctx, absPath)   // 回滚补偿：库失败则删仓
    return nil, contract.NewError(contract.CodeOperateFail)
}
```

**(b) 建仓（`:443-459`）保持原样** —— `git init --bare` 是毫秒级，改动它只有风险没有收益。

**(c) 5 处事务内跨连接查询**（`:2313`、`:2372`、`:2425`、`:2477/2479`、`:2647/2651`）—— 把 `h.groupKeyOf(...)` / `h.repository(...)` 的求值**提到 `WithTx` 之前**：

```go
// 事务外先取
targetGKey, sourceGKey := h.groupKeyOf(ctx, mr.TargetRKey), h.groupKeyOf(ctx, mr.SourceRKey)
if err := h.Repos.WithTx(ctx, func(tx repo.RepositoryTx, raw *sql.Tx) error {
    if err := tx.MergeRequestMerge(ctx, uKey, mrKey, sourceSHA, targetSHA); err != nil { return err }
    return h.emit(ctx, raw, domain.EventTypeMERGE_REQUEST_MERGE, …, map[string]any{
        "gKey": targetGKey, "sourceGKey": sourceGKey,
    })
}); err != nil { … }
```

**(d) 超时对齐**（N2）：`gitsvc/service.go:40` 的 `10 * time.Minute` 明显长于 `main.go:251` 的 `WriteTimeout: 120s`。建议：

```go
// gitsvc/service.go
defaultCommandTimeout = 90 * time.Second   // 必须 < HTTP WriteTimeout(120s)
```

并对 clone/fetch 这一类可能超长的命令单独放大（通过 `run(ctx, timeout, …)` 显式传入，而不是改全局默认）。

**验收**：并发 30 路 fork 时，`/api/user/info` 的 p99 不劣化（用 `cmd/stress -concurrency 16` 同时施加 fork 压力）。

#### C6. B1 —— 鉴权复用已解析用户（顺带，5 行）

**现状已用 general_log 直接证实**：一次 `/api/repository/list` 中 `cc_sessions` 与 `cc_users` **各被执行两次**（`06:43:22.461721` / `.475229` 与 `.470797` / `.477486`）。

```go
// httpapi/dispatch.go:110-118
var current *domain.User
if ctrl.Auth == AuthInternalGateway {
    if err := s.authenticate(ctx, ctrl.Auth, p, r); err != nil {
        contract.WriteError(w, p.ID, err); return
    }
} else {
    u, err := s.currentUser(ctx, p, r)          // 只解析一次
    if err != nil { contract.WriteError(w, p.ID, err); return }
    if ctrl.Auth == AuthAdminWebUser && !u.Admin {
        contract.WriteError(w, p.ID, contract.NewError(contract.CodeInvalidCredential)); return
    }
    current = u
}
req.User = current
```

> 注意保留原语义：`AuthAdminWebUser` 非管理员返回 `CodeInvalidCredential`；`resolveUser` 原先"失败不报错"（`err != nil` 时 `req.User` 为 nil），新写法在鉴权阶段已经失败返回，不改变对外行为。用 `contract-compare` 回归。

#### C7. B2 —— 直接删除 admin 双中间件链（不是重构）

**这是本方案相对报告最重要的简化**。`handlers/admin.go:140-156` 的注释自己写着「待 contract 层修复（Parse 解析 multipart 前复位请求流）后，本段即可删除」。而 `contract/parse.go:160-168` **已经实现了复位**：

```go
body, err := io.ReadAll(r.Body)
…
defer r.Body.Close()
r.Body = io.NopCloser(bytes.NewReader(body))   // ← 复位已存在
…
case ContentForm:
    if err := r.ParseMultipartForm(32 << 20); err == nil { … }
```

**旁证**：`handlers/user_profile.go`、`user_email.go`、`user_notify.go` 都读 body 字段，且**都不在 `adminBodyPaths` 里**，前端所有写操作又都是 multipart（见 `cmd/smoke-write/main.go:418` 注释）——它们能正常工作，说明 `contract.Parse` 单独就能解析 multipart。

**改法**：删除 `handlers/admin.go` 的 `apiMux := s.Handler()` / `adminBodyPaths` / `adminMultipartPreParse`（`:140-198`），`admin.Register` 恢复为只注册 API 路由。运行时可验证的收益：`/api/admin/config` 等路径的访问日志从 2 条变 1 条（当前日志中同一 `request_id` 恒有两条）。

> 报告提议的「在 `handleAPI` 入口统一 multipart 预解析」是次优解：它保留了一层额外机制。**根因已修，应该删代码而不是加代码。**
> 回归依据：`cmd/smoke-write` 覆盖 17 个 adminBodyPaths 中的建用户/改配置等写端点。

---

### 阶段 D —— 数据层（P1+P2，约 2 人日）

#### D1. 索引迁移 `backend/internal/db/migrations/004_perf_indexes.sql`

**落盘位置非常重要**：`db/db.go:19-20` 用 `//go:embed migrations/*.sql`，`Migrate`（`:92-125`）**只执行该目录**；`deploy/sql/db_structure.sql` 只在全新安装时由 `create_db.sh:88-94` 导入，**对已部署实例无效**。报告这一条警告是对的。

```sql
-- 004_perf_indexes.sql —— 2026-10 性能整改：补齐高频查询索引
-- 说明：9 项为组合索引（单列索引已存在），目的是把回表与 filesort 消掉。

-- 真正的全表扫描（这两项收益最大）
ALTER TABLE cc_users ADD INDEX idx_users_tel (u_tel), ADD INDEX idx_users_status (u_status);

-- 事件 worker 领取（B8）
ALTER TABLE cc_events ADD INDEX idx_events_claim (ev_status, ev_created);

-- 成员/仓库归属判定（B11）
ALTER TABLE cc_group_members      ADD INDEX idx_gm_user_group (u_key, g_key);
ALTER TABLE cc_repository_members  ADD INDEX idx_rm_user_repo  (u_key, r_key);
ALTER TABLE cc_repositories        ADD INDEX idx_repo_group_status (g_key, r_status),
                                   ADD INDEX idx_repo_user_status  (u_key, r_status);

-- 通知（B7 / B11）
ALTER TABLE cc_notification_refuse  ADD INDEX idx_nr_user_type_group (u_key, nr_type, g_key),
                                    ADD INDEX idx_nr_user_type_repo  (u_key, nr_type, r_key);
ALTER TABLE cc_notification_users   ADD INDEX idx_nu_user_status_read (u_key, nu_status, nu_is_read);
ALTER TABLE cc_notification_content ADD INDEX idx_nc_scope_created (nc_scope, nc_created);

-- 活动与合并请求（B10 收窄查询后才会真正用上）
ALTER TABLE cc_activities       ADD INDEX idx_act_repo_mr (a_relative_r_key, a_relative_mr_key);
ALTER TABLE cc_proteced_branch_rules ADD INDEX idx_pbr_repo_status (r_key, pbr_status, pbr_updated);
```

**执行注意**：
- `cc_repository_webhook_logs` 已有 `rw_key` + `rwl_created` 两个单列索引，组合索引收益边缘，**本批不做**；
- MySQL 8.0 的 `ADD INDEX` 支持 `ALGORITHM=INPLACE, LOCK=NONE`，但 `cc_activities`/`cc_events` 若已很大，仍建议低峰执行并逐个 `ALTER`（一次一条，便于回滚）；
- 迁移器在一个事务里跑整个文件，**大表 DDL 会拉长事务**——若目标实例数据量大，把 004 拆成 `004a`…`004e` 多个文件。

#### D2. 保留期清理（N3 / B8）

当前只有 `cc_sessions` 有清理（`cmd/codefever-api/main.go:106,423-432`）。照同一模式新增一个日任务：

```go
// 建议保留期（可按部署策略调整）
//   cc_events                   已处理(ev_status=3) 且超 30 天
//   cc_activities               超 180 天
//   cc_notification_content     超 90 天（连同其 cc_notification_users）
//   cc_repository_webhook_logs  超 90 天
```

```sql
DELETE FROM cc_events WHERE ev_status = 3 AND ev_created < DATE_SUB(NOW(), INTERVAL 30 DAY) LIMIT 5000;
DELETE FROM cc_activities WHERE a_created < DATE_SUB(NOW(), INTERVAL 180 DAY) LIMIT 5000;
```

**必须分批 + `LIMIT`**（避免一次删空表带来的长时间锁与主从延迟），并在 worker 侧循环直到影响行数 < 批次大小。

#### D3. 检索改造（B10 + N4）

1. **先修通配符未转义**（安全 + 性能双收益）：`repo/repository.go:623`、`:681` 改用同库已有的 `escapeLike(keyword)`；
2. 管理端 `g_display_name` / `r_display_name` 的搜索：如果是前缀语义就改 `LIKE 'kw%'` 并加 `KEY (g_display_name(191))`；
3. `cc_activities.a_data`、`cc_merge_requests.mr_title/mr_description` 的自由文本搜索：数据量上来后改 `FULLTEXT` + `MATCH … AGAINST`（**当前数据量（13 行）下不要提前做**）。

#### D4. 排序稳定性（B12 另一半）

`repo/admin.go:104-108` 在 `sort` 不在白名单时**不拼 `ORDER BY`**，导致 `LIMIT/OFFSET` 变成无序扫描。改为始终追加稳定排序：

```go
order := domain.AdminUserSortColumn(sort)
if order == "" {
    order = "u_created"     // 兜底默认序，而不是无 ORDER BY
}
q += " ORDER BY " + order + " DESC, u_key DESC"   // 追加主键破除同值歧义
```

`ORDER BY FIELD(g_key, …)`（`repo/repository.go:162`，B15）建议**直接去掉 SQL 侧的 FIELD 排序**，改在 Go 里按 `g_key` 顺序重排（行数很小），顺带消掉重复的占位符集合（`:156-165`）。

---

### 阶段 E —— 前端体积（P3，约 4 人日，**必须在阶段 A 之后**）

> 为什么排在 A 后面：A 已经用 1 行配置把 1,699 KB 变成 454 KB。E 的目标是 454 → ~250 KB（gzip），**边际收益只有 200 KB**，但工作量是 A 的 5 倍。

#### E1. `@pgyer/icons` 按路径导入（**单项最大收益，约 -110 KB raw / -22 KB gzip**）

`node_modules/@pgyer/icons/package.json` **没有 `sideEffects` 字段**，`index.js:1-351` 一次性 `import` 了全部 **154** 个图标模块并 re-export ⇒ 154/154 进包。而 `src` 实际只用 **59 个**图标（47 个文件、named import 形式）。

优先尝试**零代码改动**的方案：给该包打 patch 补 `"sideEffects": false`（`patch-package`）。

```jsonc
// node_modules/@pgyer/icons/package.json  →  patches/@pgyer+icons+X.Y.Z.patch
{
  "main": "index.js",
  "sideEffects": false
}
```

若 webpack 4 + terser 对顶层 re-export 仍不摇树（必要时验证：`grep -c "plSearch" build/static/js/*.js` 与 `yarn build` 后的 vendor 体积对比），再退化为按路径导入：`import plSearch from '@pgyer/icons/icons/plSearch'`（47 个文件、59 个图标）。

#### E2. prismjs 语言裁剪（约 -120 KB raw / -25 KB gzip）

`config/webpack.config.js:431-436` 的 `babel-plugin-prismjs` 配了 **56 种语言**，实际仓库内容以 shell/go/js/ts/json/yaml/markdown/sql/html/css/python/java/c/cpp/diff 为主。

```js
// config/webpack.config.js:432 —— 从 56 种裁到 15 种
languages: ['bash','shell','go','javascript','jsx','typescript','tsx','json',
            'yaml','markdown','sql','html','css','python','java','c','cpp','diff'],
```

同时删除重复主题：当前 3.6 KB 的 CSS chunk 同时含 `prismjs` 的 `coy` 主题与 `highlight.js/styles/atom-one-light.css`，而 `highlight.js` 的 JS 已被注释掉（`CodeViewer.js:10`、`CodeDiffViewer.js:10`）—— 删掉后者。

#### E3. Markdown 渲染按需加载

`react-markdown` + `remark-gfm` + micromark/mdast/hast 全家桶约 172 KB（raw），只在 `components/unit/ObjectViewer.js:20-21` 用到。改为：

```js
const ReactMarkdown = React.lazy(() => import('react-markdown'))
// 渲染处：
<Suspense fallback={<div className="markdown-loading" />}>
  <ReactMarkdown remarkPlugins={REMARK_PLUGINS} children={object.object.raw} />
</Suspense>
```

同时把 `remarkPlugins={[remarkGfm]}`（`ObjectViewer.js:221`）提为**模块常量** `const REMARK_PLUGINS = [remarkGfm]`（F15），并把 `ObjectViewer.js:220` 那个 `cdnjs` 的 `github-markdown-css` **移入本地 CSS**（F16：第三方 CDN 的 DNS/TLS 落在关键路径上）。

#### E4. 路由分割（F1）

`src/routes/MainRoutes.js:5-44` 的 34 个静态 import 全部改成 `React.lazy`，`MainRoutes.js:46` 的 `render()` 外包一层 `<Suspense>`：

```js
const AdminClient = React.lazy(() => import('APPSRC/components/view/admin/Client'))
// …其余 33 个…

<Suspense fallback={<PageLoading />}>
  <Switch>{/* …原 Route 列表不变… */}</Switch>
</Suspense>
```

**必须注意两点**（报告未提）：
1. 全仓库**没有任何 hook**（`useState`/`useEffect` 0 处，78 个 `React.Component`）→ `React.lazy` + `Suspense` 是类组件可用的，但 `React.Suspense` 需要 React ≥ 16.6，当前 React 17 满足；
2. 这个项目重写了 `shouldComponentUpdate` 44 处且**没有 memo 习惯**（F2），分包后在 chunk 加载间隙会有可见白屏 → 必须提供有意义的 `fallback`（不要空 div）。

**拆分优先级**（按源码体积）：`admin/*`（10 个页面）→ `MergeRequestDetail`（单文件 42 KB）→ `CommitList`/`FileTree`。

#### E5. 语言包按需（可选，-29 KB）

`src/lang/**` 当前 zh-cn + en-us 全量打入。若确认单语部署，去掉另一种即可。

---

### 阶段 F —— 渲染性能（P3，约 3 人日，Profile 驱动）

> 这一阶段**必须以 `web-vitals` + React Profiler 的数据驱动**，不要凭直觉改。

#### F1. 先修 reducer 的两个真问题（不只是性能）

1. **引用恒变**（F3）：`reducers/DataStore.js:31-33`、`NotificationStates.js:21-23`、`DrawerStates.js:9-11` 的 bail-out 全部改成 `return state`（原样返回，不 `{...state}`）；`NotificationStates.js:78` 只在 `confirmation` 真正变化时重建。
2. **原地改 state + 数组空洞**（潜在崩溃）：`NotificationStates.js:27-75` 用 `++`、`push`、`delete queueList[id]` 直接改 state；`:56` 的 `delete` 会在数组上留空洞，`:60` 的 `shift()` 可能返回 `undefined`，`:62` `newItem.data.title` 会抛异常。改为不可变更新（`filter` 代替 `delete`，`[...arr, x]` 代替 `push`）。

**为什么这项排第一**：`helpers/NetworkHelper.js:5` 在**每次 API 调用的入口**（148 个 `getJSONData` 调用点，且 `response.ok` 尚未检查）dispatch `network.connection.ok`，该 action 不匹配任何 slice 前缀 ⇒ 三个 slice 全部返回新对象 ⇒ 根引用变化 ⇒ 89 个 `connect()` 组件全部重跑 `mapStateToProps`。

#### F2. 把 `JSON.stringify` 深比较换成引用比较（F4，实测 20+ 处而非报告说的 4 处）

报告只列了 4 处，实际还有：`ActivityList.js:71,78`、`BranchList.js:57`、`CommitDetail.js:103`、`CreateBranch.js:138`、`CreateMergeRequest.js:90`、`CreateTag.js:146`、`GroupSettingGeneral.js:115`、`MergeRequest.js:86,91`、`RepositorySettingBranch.js:138`、`RepositorySettingGeneral.js:114`、`TagList.js:145`、`UserSettingEmail.js:164`、`UserSettingNotification.js:176-179`。

reducer 本来就是整体替换对象，**引用比较就是有效快路径**：

```js
shouldComponentUpdate(nextProps, nextState) {
  if (nextProps.currentRepositoryConfig !== this.props.currentRepositoryConfig) return true
  if (nextState.pathStack !== this.state.pathStack) return true
  return false
}
```

#### F3. `CodeViewer`（F5，最热的长任务）

`components/unit/CodeViewer.js:81-83` 的 `shouldComponentUpdate` **无条件 `return true`**，导致父组件任何重渲染都重新 `join` + `Prism.highlight` 整个文件 + `split('\n')`（`:96`）；`:104` 的 `blame.filter(b => b.start === key+1)` 是 O(行数 × blame 段)。

```js
// 1) SCU 改为按关键 props 比较
shouldComponentUpdate(nextProps, nextState) {
  return nextProps.raw !== this.props.raw
      || nextProps.lang !== this.props.lang
      || nextProps.blame !== this.props.blame
      || nextState !== this.state
}
// 2) 高亮结果缓存（模块级 LRU，键 = lang + '\u0000' + raw）
const highlightCache = new Map()
// 3) blame 建索引：Map<startLine, blameItem>，一次性构建，替代每行 filter
```

#### F4. `Main.js` 的 SCU 内 dispatch（F9）

`components/Main.js:82-133` 在 `shouldComponentUpdate` 里 dispatch 1–4 个 action，并且用 `{}` 重置 config（`:100-101`、`:128`）。改为：dispatch 移到 `componentDidUpdate`，按参数实际变化触发，值未变时**不 dispatch**，且不要用 `{}` 重置（用 `null` 或保留上次值，避免下游触发两次 prop 变更）。

#### F5. 其余（按 Profile 结果取舍）

- `FileTree.js:360` 的 `makeFileListData` 移出 render body（`getDerivedStateFromProps` 或 `componentDidUpdate` 里按 `search` 变化重算）；
- `helpers/FilterGenerator.js:114-118` 的 `keyword.toLowerCase()` 提到谓词外；
- `CommitList.js:122,126,132` 的 `new DateFnsUtils()` 提为模块级单例；
- `RefList.js:227` 的 `refListsDeal()` 从 `render()` 移出（它遍历**全部** ref 并对每项构建 JSX + 查用户信息，只有渲染是分页的），并停止在 render 里改 props（`:115,122-126`）；
- 列表 `key`：报告说 36 处，实测**≥64**（36 处 `key={index}` + 28 处 `key={key}`），追加式列表（`Notification`、`CommitList`）必须换成稳定 id；
- `withScrollTrigger.js:21` 加 `{ passive: true }`；
- `admin/Users.js:157-168`、`Groups.js:116-126`、`Repositories.js:116-126` 的防抖定时器在 `componentWillUnmount` 里 `clearTimeout`。

---

## 4. 排期建议

| 周 | 内容 | 里程碑（可验证） |
|---|---|---|
| **W1 上半** | 阶段 A（A1–A3） | 首屏 JS 454 KB gzip；二次访问静态资源 0 请求；`index.html` 不被缓存 |
| **W1 下半** | 阶段 A4 + 阶段 B | 登录页资源数 19 → ~13、体积 785 → ~400 KB；`cmd/stress` 基线 json 落盘；`web-vitals` 有数据 |
| **W2** | 阶段 C1–C4 | `pagesize` 全站上限生效；admin userList `102 → 3` 查询；通知 worker `400 → 1`；`smoke-write` 全绿 |
| **W3** | 阶段 C5–C7 + 阶段 D | git 出事务、5 处跨连接清零、超时对齐；`004_perf_indexes.sql` 上线；保留期任务运行 |
| **W4–W5** | 阶段 E | 首屏 454 → ~250 KB gzip；路由 chunk 数 2 → 10+ |
| **W6** | 阶段 F | Profiler 中 `JSON.stringify` / `Prism.highlight` 从 top-10 消失 |

**如果只有 2 天**：只做阶段 A1+A2+A3（约 1.5 人日），拿到本方案 74% 的收益。
**如果只有 1 周**：阶段 A + B + C1（`pagesize` 上限）—— 拿到收益，同时把"单请求打挂全站"的风险关掉。

---

## 5. 验收口径（统一，避免再次出现 §1 的测量事故）

| 项 | 工具 | 通过标准 |
|---|---|---|
| 传输层 | `curl -H 'Accept-Encoding: gzip'` | `content-encoding: gzip`；首屏 ≤ 460 KB；二次访问命中 `immutable` |
| API 延迟 | **`cmd/stress`**（`general_log=OFF`） | 阶段 C 后各端点 p95 不劣化；`repository/list` p50 ≤ 20 ms |
| SQL 条数 | `general_log`（**临时**开启，测完立刻关） | admin userList ≤ 3；通知事件 ≤ 1；`/api/repository/list` ≤ 15 |
| 前端体积 | `yarn build` 后 `du -b build/static/js/*.js` | gzip 后 ≤ 260 KB |
| CWV | `web-vitals` → `/api/web-vitals` | LCP 记录基线并逐周对比 |
| 功能回归 | `cmd/smoke-write` + `cmd/contract-compare` | 全绿，错误码语义无变化 |

---

## 6. 明确不做的事（避免过度优化）

| 项 | 报告建议 | 本方案结论 |
|---|---|---|
| I5 换 `nginx:alpine` 基础镜像（1.67 GB → 200 MB） | 建议换 | **不做**。只影响部署分发速度，不影响运行时性能；需要重做 supervisor 编排与 nginx 配置回归。若确实在意镜像体积，改为**另立议题**，独立评估 |
| I10 上 Service Worker | 建议补 | **不做**。与阶段 A2 的 `immutable` 长期缓存收益重叠；SW 会引入"发布后用户拿旧壳"的缓存中毒风险，收益/风险不划算 |
| F13 列表虚拟化 | 建议做 | **暂不做**。当前列表规模（3 用户 / 4 仓库）无收益；等阶段 C1 的 `pagesize` 上限生效、数据增长后再评估 |
| F19 `<img loading="lazy">` | 建议做 | **顺手做**（只有 6 个 `<img>`，其中 4 个是 `data:` URI），不值得单列一项 |
| B14 去掉 `SELECT DISTINCT` | 报告称"g_key 是主键所以多余" | **可做但理由不同**：`DISTINCT` 去的是 JOIN 扇出（一个组含多个命中仓库），去掉它安全只是因为结果进了 map。收益微小，随阶段 D 顺手处理 |
| B16 通知 `INSERT` 批量化 | 建议做 | **随阶段 C3 一起做**。顺带发现：N 个收件人写 N 条内容完全相同的 `cc_notification_content`，可改为 1 条内容 + N 条 `cc_notification_users` |

---

## 7. 结论

报告的**方法论是可靠的**（先测量、再定位、给可复现证据），前端与基础设施部分（P0-1/P0-2/I1/I2/I3/I4）**结论完全成立，可以直接执行**。

需要修正的是**后端部分的量化与定级**：三个错误基线（4.24 ms/往返、55–133 ms/端点、构建产物不同步）导致"后端 P0"这一判断不成立；而真正的高危项（`pagesize` 无上限、git 超时与 HTTP 超时不匹配、5 张表无保留期）被低估或漏掉。

把顺序调整为 **传输层 → 规模风险 → 数据层 → 前端体积 → 渲染** 之后，同样的工作量可以拿到远高于原计划的收益，并且在此期间系统不会再出现"一个请求打挂全站"的故障模式。

---

## 8. 执行记录（2026-10-24 全部落地）

六个阶段全部完成并已在运行容器上验证：`codefever-next:local` 镜像已重建、
`codefever-next-app` 已滚动更新、迁移 001–009 全部应用。

### 8.1 阶段 A —— 传输层（完成）

| 项 | 改动 | 运行时验证 |
|---|---|---|
| A1 gzip | `vhost-go.conf-template` 开启 gzip（主配置里是 `#gzip on;` 注释态）+ `gzip_static` | `Content-Encoding: gzip`、`Vary: Accept-Encoding`；vendor chunk 实测 **719,449 B → 227,452 B** |
| A2 Cache-Control | 三档：`/static/js|css` immutable 一年、其余 `/static/` 7 天 + swr、SPA 壳 `no-cache` | 三条 `Cache-Control` 各自命中；`/admin/client` 返回 `no-cache` |
| A3 defer | `webpack.config.js` 增加 `scriptLoading: 'defer'` | 服务端 `index.html`：`<script defer="defer" ...>` |
| A4 登录页 | `marked.min.js`（47.6 KB）只在 `doc_detail.html` 加载；LCP 大图 PNG 147.9 KB → WebP 30.5 KB（`<picture>` + `fetchpriority=high`） | 登录页已无 `marked.min.js`；`<source srcset="...webp">` 就位 |

**未按原计划做**：`intlTelInput.js/css` 的按页拆分**已撤销** —— 需要它的
login/register/reset_password/verify 正是访问量最高的四个页面，拆分没有收益，
只会增加模板分叉与 PHP 保真回归的风险（理由已写入模板注释）。

### 8.2 阶段 B —— 可观测性（完成）

- `www/view/src/index.js` 接入 `web-vitals`（CLS/FID/FCP/LCP/TTFB），**动态 `import()`**，不进首屏；
- 新增 `POST /api/web-vitals`（`internal/handlers/webvitals.go`）：字面量 page route，
  绕开 contract 的 Accept/Content-Type 校验，只写日志不入库，4 KB 限长。
  实测：`sendBeacon` 形态（`text/plain` + `*/*`）→ **204**，日志落盘；`GET` → 405；
- `docker-compose.yml` 开启 `slow_query_log=1`、`long_query_time=0.2`（实测已生效）；
- 基线工具改用项目自带的 `cmd/stress`，实测 **p95 15.5 ms / p99 23.6 ms**。

### 8.3 阶段 C —— 后端规模风险（完成，含审计外两项）

| 项 | 关键改动 | 验证 |
|---|---|---|
| C1 `pagesize` 上限 | 新增 `domain.ClampPaging`（上限 100），handler 三处 + **repo 层再兜一次**（防新增调用方绕过） | 单元测试 7 例；`AdminPaging(0,1000000)→(1,100)` |
| C2 admin N+1 | 新增 `repo.CountJoinedByUsers`：一条 `UNION ALL + COUNT(DISTINCT r_key)` 复刻 `ListJoined` 四来源并集 | **集成测试**逐用户比对 `len(ListJoined())` 与批量计数一致；`userList` 的 `projects` 值与改造前一致 |
| C3 通知 N+1 | 新增 `repo.RefusedFor`：一条批量查询替代 2×N 次 `COUNT(*)` | `fakeRefuse` 逐条等价；events 测试全绿 |
| C4 `IN` 去重 | `inPlaceholders` 去重（占位符与参数同步收敛） | 全部 repo 测试通过 |
| C5 git 出事务 | fork 的 `CloneBare` 移到 `WithTx` 之前；**7 处**跨连接调用（5 处 `groupKeyOf` + 5 个 handler 里的 `h.repository`，去重后共 7 个语句点）提到事务外；git 兜底超时 10 min → **60 s**，clone 单独保留 10 min | fork 失败用例改为断言"不写行 + 清理目录"；`gitsvc` 测试通过 |
| C6 鉴权复用 | `authenticateWithUser` 合并原 `authenticate` + `resolveUser`，删除 `resolveUser` | general_log 实测：一次 `/api/repository/list` 中 `cc_sessions`/`cc_users` **各 1 次**（修复前各 2 次） |
| C7 删除双中间件链 | 删除 `adminBodyPaths` / `adminMultipartPreParse` / 二次 `s.Handler()`（根因已在 `contract.Parse` 修复） | admin 请求**每个 request_id 只有 1 条**访问日志；`POST /api/admin/userList`（multipart）返回正确数据；`checkPassword` 正确密码=成功、错误密码=0x0405 → 证明 body 字段确实被解析 |
| **B16**（审计外补做） | 站内信写入 2×N 条独立 INSERT → 每 100 人一批多行 INSERT | events 测试通过 |
| **B17**（审计外补做） | 拒收级联（`SetRefused`/`SetReceive`）包进单一事务，判重同事务内完成 | repo 测试通过 |

### 8.4 阶段 D —— 数据层（完成）

- **6 个索引迁移文件 / 共 12 项索引**（`004`–`009`，一表一文件以便部分失败后安全重跑）：
  `cc_users(u_tel)`、`cc_users(u_status)`、`cc_events(ev_status,ev_created)`、
  `cc_group_members(u_key,g_key)`、`cc_repository_members(u_key,r_key)`、
  `cc_repositories(g_key,r_status)`、`cc_repositories(u_key,r_status)`、
  `cc_notification_refuse(u_key,nr_type,g_key)`、`(u_key,nr_type,r_key)`、
  `cc_notification_users(u_key,nu_status,nu_is_read)`、
  `cc_activities(a_relative_r_key,a_relative_mr_key)`、
  `cc_proteced_branch_rules(r_key,pbr_status,pbr_updated)` —— 全部已应用。
- **保留期清理**（`repo/retention.go` + 每日任务）：事件（已处理 30 天 / 死信 60 天，
  **待处理与处理中永不删**）、已读站内信 90 天 + 孤儿内容行、webhook 日志 90 天；
  全部 `LIMIT` 分批、可 `CODEFEVER_RETENTION_*` 环境变量覆盖；
  `cc_activities` **默认关闭**（用户可见的产品数据，策略交给运维）。
- LIKE 通配符转义补齐（`repo/repository.go` 两处）；
- 排序稳定性：admin 三个列表在 `sort` 未命中白名单时回落到创建时间并追加主键兜底；
  `ListJoined` 的 `ORDER BY FIELD(...)` 改为 Go 侧等效排序（去掉 filesort 与重复占位符）。

> 实现中发现并修正了两处 SQL 方言问题：MySQL **不支持**多表 `DELETE` 上的 `LIMIT`，
> 孤儿内容清理改为单表 `DELETE ... WHERE NOT EXISTS (...)`；Prism 组件名应为
> `batch` / `coffeescript`（而非 `bat` / `coffee`）。

### 8.5 阶段 E —— 前端体积（完成）

| 项 | 改动 | 效果 |
|---|---|---|
| E1 图标 | `@pgyer/icons` 47 个文件、141 处 barrel import → **按路径默认导入**（该包无 `sideEffects`，154/154 图标全进包） | vendor chunk 大幅缩小（见下） |
| E2 Prism | 语言 56 → 36，且**补齐了映射表里原本静默回落 markdown 的 8 种**（batch/clojure/csharp/java/less/objectivec/pascal/r）；删除从未被使用的 `highlight.js` 主题 CSS | CSS chunk 3.6 KB → 1.0 KB |
| E3 Markdown | 新增 `helpers/MarkdownRenderer.js`，`React.lazy` 加载；`remarkPlugins` 提为模块常量；`github-markdown-css` 内置到异步 chunk；删除 render 里的第三方 CDN `<link>` | ~172 KB 移出首屏 |
| E4 路由分割 | 34 个路由组件全部 `React.lazy`，按业务域分 **7 个 chunk**（admin/settings/group/repository/repository-cfg/merge-request/misc）+ `Suspense` 占位 | 首屏只剩 2 个 JS |

**首屏 JS 实测**（`asset-manifest.json` 的 entrypoints）：

| | raw | gzip | brotli |
|---|---:|---:|---:|
| 修复前 | 1,699,347 | 454,176 | 411,736 |
| **修复后** | **871,055（-48.7%）** | **263,310（-42.0%）** | **242,363（-41.1%）** |

产物目录 7.9 MB → **1.8 MB**（同时因为 `GENERATE_SOURCEMAP=false` 不再随镜像分发 6.5 MB source map）。

### 8.6 阶段 F —— 渲染性能（完成）

- **F1/F3 reducer**：`DataStore`/`DrawerStates`/`NotificationStates` 全部改为不可变更新，
  与自身无关的 action **原样返回 state**；`notification.connection.ok` 这类每次 API 调用都发的
  action 不再导致根引用变化。顺带修掉两个真 bug：
  - `confirmation.remove` 分支原为死代码（`getObjectByProperty` 返回对象，`typeof === 'number'` 恒 false），**从未真正移除过队列项**；
  - `queueList` 上的 `delete` 会留空洞，`shift()` 可能返回 `undefined` 后 `newItem.data.title` 抛异常。
- **F4**：19 个文件、26 处 `JSON.stringify` 深比较改为**引用不等才序列化**（语义等价，零风险）；
- **F5**：`CodeViewer` 的 `shouldComponentUpdate` 由恒 `true` 改为比较关键 props；新增高亮结果缓存
  （按语言+全文，8 条上限）与 blame `Map` 索引（O(行数×段数) → O(行数)）；
- **F9**：`Main.js` 的副作用从 `shouldComponentUpdate` 移到 `componentDidUpdate`，
  并加"参数真变了才 dispatch"的判断（原先无 groupName 的路由**每次渲染**都 dispatch 两次）；
- **F12**：admin 三个列表的 200 ms 防抖定时器在 `componentWillUnmount` 里清理；
- **F17**：滚动监听加 `{ passive: true }`；**F18**：`DateFnsUtils` 提为模块级单例；
  **F19**：空态插图加 `loading="lazy"`（Header logo 不加 —— 首屏元素）。

### 8.7 明确未做的项（含理由）

| 项 | 结论 |
|---|---|
| F13 列表虚拟化 | 当前数据规模（3 用户 / 4 仓库）无收益；等 `pagesize` 上限生效后再按需评估 |
| F2 `React.memo` | 本项目 78 个 class 组件、44 处手写 `shouldComponentUpdate`，缺 memo 的根因是 F1/F4/F9，已修；再叠加 `React.memo` 属于风格叠加，收益需 Profiler 证明 |
| F14 `key={index}`（≥64 处） | 机械改动面大、回归风险高，且只有追加式列表（Notification / CommitList）受影响；建议单独一次改动 + 逐屏验证 |
| B10 前缀搜索 / FULLTEXT | 依赖真实数据量；当前 13 行时做 `FULLTEXT` 属于过度设计。已先修好通配符转义（安全 + 避免全表退化） |
| B14 `SELECT DISTINCT` | 去重的是 JOIN 扇出，去掉只对"结果进 map"安全；收益微小，随下次改动顺手处理 |
| I5 换基础镜像 | 影响分发体积而非运行时性能，独立议题 |
| I10 Service Worker | 与 A2 的 `immutable` 长缓存重叠，且有发布缓存中毒风险 |

### 8.8 回归证据汇总

| 层 | 命令 | 结果 |
|---|---|---|
| Go 单元/集成 | `go build ./... && go vet ./... && go test ./...` | 全绿 |
| DB 集成（需 MySQL） | `CODEFEVER_TEST_DSN=... go test ./internal/repo/ -run 'TestCountJoinedByUsersMatchesListJoined\|TestRetentionServiceRun\|TestGroupRepoWithTxBoth'` | 全绿 |
| PHP 渲染保真 | `go test ./internal/httpapi/`（快照比对 + **显式登记的偏离清单** `normalizePerfDeviations`） | 全绿 |
| 前端构建 | `GENERATE_SOURCEMAP=false NODE_OPTIONS=--openssl-legacy-provider yarn build` | 成功，ESLint 零告警 |
| 运行时 | `curl` 头/体积、general_log 计数、admin multipart、`cmd/stress` | 见 8.1–8.3 |

> 快照测试的处理方式值得记录：`php-*.html` 是 PHP→Go 迁移的保真基线，**没有**为了适配
> 本地改动去改快照；而是在 `pages_test.go` 里新增 `normalizePerfDeviations`，
> 把两处**有意偏离**（marked 按页加载、LCP 图改 `<picture>`）显式登记并从两侧抹平。
> 新增偏离会让测试失败 —— 这是刻意的，防止以后再"顺手改快照"绕过回归。
