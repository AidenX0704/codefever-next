# 下线 PHP —— 依赖面清单、稳定性门禁与改造步骤

> **注意（2026-10-24）**：本文档中出现的 `www/view/`、`www/static/` 与 MUI v4 相关描述
> 属于**历史记录**——旧前端已在 2026-10 前端重写中删除。当前前端为 `web/`
> （React 19 + TypeScript + Vite + 自研令牌与组件库），见 `docs/FRONTEND_REWRITE_PLAN.md`。


> 目标：把 CodeFever Community 的运行时**彻底从 PHP 上摘下来**，
> 并在管理端移除 PHP-FPM 与相关服务的展示。
>
> 前置条件：**先证明 Go 后端是稳定的**。稳定性没过关之前不动 PHP，
> 否则一旦出问题就没有可回滚的目标。

> **文档状态（2026-10-23 晚）**：本文记录 PHP 下线过程。当前仓库已再次收敛：
> `http-gateway/` / `ssh-gateway/` 并入 `backend/cmd/`，`misc/` 迁移到
> `backend/deploy/`，`CODEFEVER_BACKEND` 已移除。文中的 `misc/*`、旧网关路径与
> 回滚命令为**历史快照**；当前结构见 `README.md` / `LOCAL_DEPLOY.md`。

---

## 1. 为什么现在可以下线 PHP

| 能力 | 原载体 | 现状 |
|---|---|---|
| 108 个 `/api/*` 端点 + 4 个 `/apiv1/internal` 回调 | PHP 控制器 | ✅ Go 后端已实现，契约回归 123/123 |
| 5 个服务端渲染页面 | PHP 视图 | ✅ Go 模板逐字节移植，43 条 web 路由与 PHP 逐字节一致 |
| 43 条 web 控制器路由 | PHP 控制器 | ✅ Go 侧 CI 三段判定复刻，43/43 对拍通过 |
| 仓库读写（clone/push/分支/标签/MR） | PHP + git 命令 | ✅ Go `gitsvc`（argv 调用，不经 shell） |
| 事件 / 活动流 / 站内信 / 邮件 | PHP 同步内联 | ✅ Go `cc_events` 队列表 + 独立 worker |
| webhook 投递 | PHP cron 控制器 | ✅ Go worker 内常驻投递器 |
| 前端静态产物 | 提交进仓库的 `www/view/build` | ✅ 未改动，21 张截图逐像素一致 |

**唯一还挂在 PHP 上的两件事**，就是本次要拆掉的：

1. `misc/hooks/main`（历史路径；当前为 `deploy/hooks/main` 的兼容软链） —— git 钩子（push 链路）
2. ~~`misc/modify_authorized_keys.php` —— SSH 公钥同步守护进程~~
   ✅ **已下线**（第 2 步完成）：改由 Go 后端（`backend/internal/authkeys`）
   直接写 `/home/<users.git>/.ssh/authorized_keys`，PHP 守护进程、FIFO +
   SIGUSR1 协议、supervisor 配置均已删除。
3. 以及 PHP 定时任务调度器（`crond` → `codefever_schedule`）——
   它的**唯一业务任务** `repository_webhook` 已由 Go worker 接管

---

## 2. PHP 依赖面清单（逐条带证据）

### 2.1 构建期

> 下表记录的是**改造前**（PHP 时代）构建配方的依赖面。构建配方当时叫 `Dockerfile.local`，
> 二次开源时已更名为 `Dockerfile`（上游的 `Dockerfile` 与 `codefever.build` 随之删除），
> 因此下表的"位置"列写作 `Dockerfile`，**行号仍对应改造前的版本**。

| 位置 | 内容 | 处置 |
|---|---|---|
| `Dockerfile:22,72` | 基础镜像 `webdevops/php-nginx:7.4` | 保留基底（nginx/supervisor/sshd 全套集成在上面），**在镜像里删除 PHP 运行时** |
| `Dockerfile:93` | apt `libyaml-dev`（只为 `pecl install yaml`） | 删 |
| `Dockerfile:96-100` | `pecl install yaml` + `docker-php-ext-enable` | 删 |
| `Dockerfile:109-110` | `COPY application/` / `COPY system/` | 删 |
| `Dockerfile:111` | `COPY www/` | 保留（静态产物），但删掉 `www/index.php` |
| `Dockerfile:186-191` | `composer install` | 删 |
| `Dockerfile:196-197` | `/usr/local/php/bin` 软链 | 删 |
| `Dockerfile:207-208,210` | `application/logs` 目录与授权 | 删 |
| `Dockerfile:219-220` | `docker-service enable cron` + `docker-cronjob` | 删（crond 只为 PHP 调度器而存在） |

### 2.2 运行期（基础镜像自带，需要在镜像里显式删除）

| 路径 | 内容 |
|---|---|
| `/usr/local/bin/php`、`php-config`、`php-fpm`、`phpize` | PHP 可执行文件 |
| `/usr/local/lib/php` | PHP 扩展（opcache/gd/pcntl/yaml/…） |
| `/usr/local/etc/php`、`/usr/local/etc/php-fpm*` | PHP 配置 |
| `/usr/local/sbin/php-fpm` | FPM 二进制 |
| `/opt/docker/etc/php` | FPM pool 配置 |
| `/opt/docker/bin/service.d/php-fpm.sh`、`php-fpm.d` | 启动脚本 |
| `/opt/docker/etc/supervisor.d/php-fpm.conf` | supervisor 里的 php-fpm program |
| `/opt/docker/provision/entrypoint.d/20-php.sh`、`20-php-fpm.sh` | 容器入口里的 PHP 初始化 |

### 2.3 仓库内 PHP 专用文件（历史清单）

> 下列 `misc/*` 是改造前的路径；当前 `backend/deploy/` 只保留
> `codefever-service-template` 与 `sql/`、`docker/` 脚本，旧路径不复存在。

`application/**`、`system/**`、`www/index.php`、`misc/hooks/main`（历史）、
`misc/php-fpm-service-template`、
`misc/php-fpm.conf-template`、`misc/www.conf-template`、`misc/php.ini-template`、
`misc/nginx.conf-template`（VM 部署路径的 PHP 版 nginx 主配置）、
`misc/codefever-service-template`、
`misc/install.sh`、`misc/update.sh`、`misc/initailize_container.sh`、`misc/system_init.sh`

> 第 2 步已删除：`misc/modify_authorized_keys.php`（PHP 守护进程）、
> `misc/docker/supervisor-codefever-modify-authorized-keys.conf`、
> `misc/codefever`（只服务于该守护进程的脚本）。
> `misc/codefever-service-template`（现为 `backend/deploy/codefever-service-template`）
> 保留，但已不再拉起 PHP 守护进程。

> **`application/**` 与 `system/**` 的源码已于二次开源时从仓库中彻底删除**：
> 仓库里保留它们原本只是为了"废弃与删除之间更稳的一步"，
> 待 Go 后端完成契约回归与稳定性门禁后（见 §3、§5）已不再需要回溯。
> 随之删除的还有 `tests/`（PHP 单测）、`www/index.php`、`phpunit.xml`、`run_test.sh`，
> 以及 `www/static/vendor/jquery-validation-1.19.1/demo/captcha/` 下的第三方 demo PHP 脚本。
> **当前仓库内已不存在任何 PHP 源码。**

---

## 3. 稳定性门禁（前置条件）

### 3.1 门禁标准

| 指标 | 阈值 |
|---|---|
| 错误率 | < 0.1% |
| 5xx | **0**（硬失败） |
| p99 延迟 | < 500 ms |
| p95 延迟 | < 200 ms |
| goroutine 数 | 长跑后不出现 > 2 倍增长 |
| 堆占用（inuse_space） | 长跑后不出现 > 2 倍增长 |

### 3.2 工具

`backend/cmd/stress` —— 模拟 SPA 真实读负载（8 个只读端点，按实际调用频次加权），
支持多档并发、p50/p95/p99 分位、状态码分布、pprof 资源采样，
并自带门禁判定（不达标以非 0 退出，可直接接 CI）。

```powershell
cd backend
go run ./cmd/stress -base http://127.0.0.1:9003 -pprof http://127.0.0.1:9003 `
    -duration 60s -concurrency 16 -sample-interval 5s -out report.json
```

> 资源采样需要后端以 `CODEFEVER_PPROF=1` 启动（仅挂载 `/debug/pprof/`，
> 与 `/api/*`、`/apiv1/*` 完全不重叠，不影响契约）。

### 3.3 实测结果

**并发阶梯**（每档 45 秒，24 并发档另见下）：

| 并发 | 总请求 | QPS | 错误率 | p95 | p99 | 判定 |
|---|---|---|---|---|---|---|
| 8 | 27,700 | 615 | **0** | 23.8 ms | 36.1 ms | ✅ |
| 16 | 39,013 | 866 | **0** | 33.6 ms | 40.4 ms | ✅ |
| 32 | 43,480 | 965 | **0** | 65.4 ms | 82.7 ms | ✅ |
| 64 | 44,086 | 978 | **0** | 141.6 ms | 177.7 ms | ✅ |

QPS 在 ~970 处饱和 —— 瓶颈是当次压测使用的 MySQL 5.7 容器（读写混合下的单库上限；
当前部署要求 MySQL 8.0.x），
不是 Go 后端本身：64 并发时 p99 仍只有 178 ms，**没有出现排队塌陷**。

**长稳浸泡**：20 分钟 × 24 并发，见 §3.4。

---

## 4. 改造步骤（每步都可独立验证与回滚）

| 步 | 内容 | 验证 | 状态 |
|---|---|---|---|
| 1 | **git 钩子 Go 化**：`misc/hooks/main`（PHP）→ `bin/codefever-hook`（Go） | 真实 git push 成功 + 保护分支被拒两条路径；**PHP↔Go 差分测试：stdout 逐字节、退出码、请求体均一致** | ✅ 已完成 |
| 2 | **SSH 公钥同步 Go 化**：去掉 PHP 守护进程 + FIFO + SIGUSR1 协议，Go 后端直接写 `authorized_keys` | 加/删公钥后文件内容正确 + 真实 `git clone` over SSH 成功 | ✅ 已完成 |
| 3 | **镜像去 PHP**：删 PHP 运行时 / php-fpm program / PHP 入口 / composer / crond；`application/`、`system/`、`www/index.php` 不再进镜像 | 镜像内 `find / -name 'php*'` 无结果；`ps` 无 php 进程 | ✅ 已完成 |
| 4 | **管理端移除 PHP 服务**：`systemStatus` 去掉 `php`/`crond` 两个键；前端仪表盘去掉 PHP-FPM / Crond 两张卡片 | 21 张截图逐像素比对：只有管理端仪表盘按预期变化 | ✅ 已完成 |
| 5 | **契约基线与最终回归** | 契约回归、页面路由对拍、冒烟回归、git push 端到端、SSH 端到端 | ✅ 已完成（历史验收；复跑见 `docs/IMPLEMENTATION_STATUS.md`） |

### 4.1 第 1 步的关键实现（`backend/internal/hook`）

Go 钩子复刻了 PHP 的全部可观察行为，并修掉一处缺陷：

| 项 | PHP | Go |
|---|---|---|
| stdout 三条分支 | `Operation Stopped: …` / `Contact site administrator…` / `data.remote` | **逐字节一致**（差分测试验证） |
| 退出码 | 0 放行 / 255 拒绝 | 一致 |
| 请求体 | `{"uid":…,"args":[…],"stdin":…}`，PHP `json_encode`（`\/` 转义、非 ASCII → `\uXXXX`） | 手工实现 PHP 风格编码，**逐字节一致** |
| stdin | `stream_set_blocking(0)` **非阻塞读** → 大 payload 可能截断 | `io.ReadAll` **读到 EOF**（修复） |
| 超时 | curl 无超时（后端卡死时 push 永久挂起） | 30 秒上限 |

### 4.2 第 2 步的关键实现（`backend/internal/authkeys`）

Go 后端**直接写** `authorized_keys`，不再走 FIFO + SIGUSR1：

- 行格式与 PHP **逐字节一致**（含字面量 `$PATH`、`$SSH_ORIGINAL_COMMAND`、四个 `no-*` 选项、字段单空格分隔）。
- 语义与 PHP 一致：**增量追加** + **按公钥正文子串删除**，不做全量重写（否则会抹掉历史遗留行）。
- 删除时**不补尾换行** —— PHP `implode("\n", …)` 的 quirk，已由单测锁定。
- 跨进程 `flock` 排他锁（PHP 时代靠 FIFO 串行消费）。
- 写失败**只记日志、不阻断加公钥接口**（与原 `Notify` 错误被忽略的取舍一致）。

### 4.3 第 4 步的行为变更（明确记录）

`GET /api/admin/systemStatus` 的 `service` 段从 5 个键变成 3 个：

```diff
  "service": {
    "nginx": true,
-   "php": false,
    "codefever": true,
-   "crond": true,
    "sendmail": false
  }
```

这是**有意的破坏性变更**（PHP 下线后这两个键没有意义），
与 `repository.fileContent`（PHP 恒 500 → Go 200）同类：
在"Go 实现 vs PHP 基线"的逐字节比对里会多出 1 条 DIFF，
但 Go 侧对 manifest 的符合度仍是 123/123。

---

## 5. 完成后的验证结果

### 5.1 镜像里没有 PHP

```
$ docker run --rm --entrypoint sh codefever-next:local -c '...'
no php ✓
no php-fpm ✓
残留目录: 全部不存在 ✓
supervisor.d: codefever-api / codefever-worker / codefever-http-gateway /
             dnsmasq / nginx / postfix / ssh / syslog   ← 无 php-fpm、无 cron
entrypoint.d: 05-permissions / 20-codefever / 20-nginx  ← 无 20-php / 20-php-fpm
应用目录: bin config.yaml env.yaml file-storage git-storage http-gateway misc ssh-gateway www
          ← 无 application/、无 system/、无 www/index.php
node_modules: 未打进镜像 ✓（.dockerignore 已排除，省约 170 MB）
```

容器运行态：`supervisord syslog-ng nginx main codefever-api codefever-worker sshd`
—— **无 php-fpm、无 crond**。

### 5.2 回归

| 项目 | 结果 |
|---|---|
| 契约回归（对 manifest） | **123 / 123 全部符合期望** |
| 契约回归（对 PHP 基线） | MATCH 121 / DIFF 2 —— 两处都是**有意的行为变更**：`admin.systemStatus`（少 `php`/`crond` 两个键）、`repository.fileContent`（PHP 恒 500 的缺陷，Go 修成 200） |
| 写操作成功路径 | **20 / 20 通过**（含 2 条回滚安全断言） |
| git push over SSH | ✅ `f67cfee..270b075`，钩子回调 3 次（pre-receive/update/post-receive）全部 200 |
| git push over HTTP | ✅ `270b075..a4e6b2b` |
| SSH clone | ✅ 克隆出 README.md / SSH.md |
| 响应式比对 | 21 张截图：**13 张完全一致**；8 张差异逐一定位（见 §5.3） |

### 5.3 响应式比对的差异说明

| 差异 | 数量 | 性质 |
|---|---|---|
| 仓库磁盘占用 `41 k Bytes` → `1 k Bytes` | 4 张 | **数据漂移**：本轮做 push 端到端验证后执行了 `git gc`，清掉了种子数据遗留的约 40 KB 松散对象。与后端实现无关 |
| README.md 大小 `27 Bytes` → `28 Bytes` | 4 张 | **PHP 侧 off-by-one 缺陷**（`exec()` 丢行尾换行），Go 侧是真实大小 |
| 排序图标 4 px、最大通道差 1 | 2 张 | **webfont 栅格化抖动**：同样这 4 px 在两次 PHP 采集之间也存在 |

> 管理端仪表盘这一页**没有 PHP 侧参照截图** —— 采集清单是在 PHP 参照集采完之后才加上 `/admin` 的。
> 该页的变更由三处证据确认：① 源码删除两张卡；② 重建产物已提交；③ 实际渲染为
> CodeFever / Nginx / SendMail 三张卡（PHP-FPM 与 Crond 已消失）。

### 5.4 顺带修掉的两个真缺陷

1. **旧 `ssh-gateway` 打不开日志文件就 panic** —— 历史实现
   `ssh-gateway/shell/main.go`（现由 `backend/cmd/codefever-ssh-gateway` 取代）的 `init()` 用
   `os.OpenFile("./message.txt", …)` 打开工作目录下的日志，失败即 `panic(err)`。
   forced command 以 `git` 用户运行、工作目录是 `/home/git`，一旦该文件属主不对
   （例如残留的 root 文件）或家目录不可写，**所有 SSH push / clone 全部失败**，
   而且客户端只看到 "Permission denied"，极难定位。已改为：打不开就退回 stderr，**绝不 panic**。
2. **`.dockerignore` 没排除 `www/view/node_modules/`** —— 本地装的前端依赖（数百 MB，
   里面还有一个恰好叫 `php` 的目录）被整个打进镜像。已排除。

### 5.5 已知副作用

| 项 | 说明 |
|---|---|
| **会话不跨后端** | PHP 会话在客户端 Cookie（CI 加密），Go 会话在服务端 `cc_sessions`，两者互不认得。切换后用户需重新登录一次（不影响数据） |
| **口令哈希仍锁在 legacy** | 存量账号的哈希是 PHP 时代的 `md5(md5(x))`，默认保持 legacy 才能让存量账号免重置口令登录。PHP 下线后可切 `argon2id`，但那时存量账号需重置口令 |
| **PHP 源码已从仓库删除** | `application/`、`system/`、`tests/`、`www/index.php` 等 PHP 源码在二次开源时已彻底删除（见 §2.3）。契约基线保留在 `backend/testdata/contract/` 与 `backend/testdata/contract-php-now/`（JSON / HTML 基线，不是 PHP 源码） |
| **`post-receive` 事件仍未发** | `backend/internal/handlers/internal.go` 的既有 TODO（与删 PHP 无关）：push 后活动流没有 push 记录。钩子本身返回 200，不影响 push |
| **PHP 回滚路径已关闭** | 第 3 步不可逆：镜像里已无 PHP，切回 PHP 后端不再可能。若要保留回滚能力，需要保留一份带 PHP 的镜像 tag |


## 6. 风险与回滚

| 风险 | 影响 | 缓解 |
|---|---|---|
| 钩子 Go 实现与 PHP 行为有偏差 | push 被误拒 / 误放 | 逐字节对齐 stdout 与退出码；单测 + **PHP↔Go 差分测试** + 真实 push 双向验证 |
| `authorized_keys` 写坏 | SSH 方式完全不可用 | 跨进程 flock；写失败只记日志不阻断加公钥接口；真实 `git clone` over SSH 验证 |
| 删 PHP 后仍有残留引用 | 容器启动失败 | 镜像构建期自检（`command -v php` 必须失败、PHP 目录必须不存在、supervisor 不得有 php-fpm/cron） |
| 删 PHP 后无法回滚 | 故障时没有退路 | **删 PHP 之前先完成稳定性门禁**（§3）；契约基线保留在 `backend/testdata/`；如需更强保证，保留一份带 PHP 的镜像 tag |

**回滚策略**：
- 步骤 1–2（钩子、公钥同步）：只删 PHP 守护进程，PHP 后端本身还在，
  历史上仍可临时把 `CODEFEVER_BACKEND` 切回 `php` 找原因（该开关现已被删除；
  PHP 后端也已不在仓库/镜像中）。
- 步骤 3（镜像去 PHP）：**不可回滚**。这是不可逆的一步，
  因此必须放在稳定性门禁之后、且在步骤 1–2 全部验证通过之后。

---

## 7. 完成后管理端的样子

服务状态面板从 5 张卡片变成 3 张：

```
CodeFever  Running
Nginx      Running
SendMail   Stopped
```

（PHP-FPM 与 Crond 两张卡片移除，接口里的 `php`/`crond` 两个键一并移除。）
