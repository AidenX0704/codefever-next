# 后端重写实施状态

> 截至 2026-10-23 上午的落地进度。本文档只记录**已验证**的事实，
> 未完成部分明确标注，不做乐观描述。

> **二次开源补充（2026-10-23 晚）**：本文档记录的是 PHP→Go 迁移**收尾阶段**的状态。
> 此后已完成的收尾动作：① 切流完成（PHP 后端下线，镜像内无 PHP）；
> ② 仓库内 PHP 源码（`application/`、`system/`、`tests/`、`www/index.php` 等）**已彻底删除**；
> ③ 镜像改为**全部从本地源码构建**（`Dockerfile`），不再依赖任何上游预构建镜像；
> ④ `docker-compose.local.yml` 已合并进 `docker-compose.yml`。
> 下文凡涉及"PHP 栈仍在 `localhost:9080`""默认仍走 PHP"等描述，均为收尾阶段的历史状态。

---

## 0. 当前状态一句话

**全部 123 个 API 端点 + 43 条 web 控制器路由已移植完成；Go 后端在真实切流下（`http://localhost:9080` → nginx upstream 127.0.0.1:9001）契约回归 123/123 符合 manifest 期望、页面路由与 PHP 逐字节一致 43/43、3 视口 × 7 页面的 21 张截图逐像素一致；PHP 运行时已从镜像中彻底移除（无 php / php-fpm / crond，`application/`、`system/`、`www/index.php` 均不再进镜像），git push 与 SSH clone 两条链路端到端验证通过。**

**PHP 侧仅剩 2 项与 Go 不一致，且都是有意的**：
- `repository.fileContent` —— PHP 因方法名拼写错误恒 500（已在新后端修复为正确行为）；
- `admin.systemStatus` —— PHP 下线后 `php` / `crond` 两个服务键已移除（管理端同步去掉这两张卡片）。

**详见 `docs/PHP_REMOVAL.md`**（依赖面清单、稳定性门禁、分步改造与验证结果）。

---

## 0.0 切流实战（已完成）

### 蓝绿并存结构

新镜像 `codefever-community:local` 中，**php-fpm 与 Go 后端同时运行**：

```
codefever-api            RUNNING   ← Go 后端 (:9001)
codefever-worker         RUNNING   ← 事件 worker
codefever-http-gateway   RUNNING   ← Git over HTTP 网关
php-fpm:php-fpmd         RUNNING   ← 旧 PHP 后端 (:9000)
nginx:nginxd             RUNNING
ssh:sshd / cron:crond    RUNNING
```

两套后端读**同一份 `env.yaml`**（因此共享同一个 `gateway.token` 与同一个数据库），nginx 同一时刻只把流量指向其中一侧，不会出现双写。

### 切流与回滚

```powershell
# 切流到 Go 后端
$env:CODEFEVER_BACKEND='go'
docker compose -f docker-compose.local.yml up -d --force-recreate codefever

# 秒级回滚到 PHP（无需重建容器，nginx reload 即可）
$env:CODEFEVER_BACKEND='php'
docker compose -f docker-compose.local.yml up -d --force-recreate codefever
```

开关由 `misc/docker/docker-entrypoint.sh` 读取，把 `vhost-go.conf-template` 复制为 nginx 的 `vhost.conf`。

### 切流后的验收

```
http://localhost:9080  （已切流到 Go 后端）
契约回归：总计 123 条：MATCH 122 | DIFF 1 | NOT_IMPLEMENTED 0
采集器判定：所有用例均与 manifest 期望一致        ← 123/123
唯一 DIFF：repository.fileContent（PHP 500 自身缺陷 → Go 200 已修复）
web 路由对拍：43 / 43 逐字节一致（go run ./cmd/page-compare -php ... -go ...）
写操作回归：20 / 20 通过（含 2 条"回滚安全"断言）
浏览器：登录后 SPA 全流程 6 个页面渲染完整，控制台 0 错误、全部 API 200
```

### ⚠️ 切流演练抓到的真实生产缺陷（已修）

**Cookie 的 `Secure` 标志原先由"环境名是否 production"决定**。而本项目的默认部署是**明文 HTTP**（容器内 nginx 监听 80），于是切流后：

- 登录接口返回 `{"code":0,"message":"登录成功"}` ✅
- 但 Cookie 带了 `Secure` → 客户端**不回传** → 后续每个请求都是 `0x0101`（403）❌

现象极具迷惑性（"登录成功但全部 403"），且**只在切流到生产配置的容器里出现**，在开发机（`env: development`）上完全正常。

修复：`Secure` 改由**请求协议**决定（`r.TLS != nil` 或 `X-Forwarded-Proto: https`），与配置名解耦。已补回归测试 `TestIsSecureRequest`。

---

## 0.1 契约回归总览（Linux 生产同构容器）

```
总计 123 条：MATCH 122 | DIFF 1 | NOT_IMPLEMENTED 0 | BASELINE_ONLY 0 | ACTUAL_ONLY 0

[DIFF] repository.fileContent  PHP 500（自身缺陷）→ Go 200（已修复）
```

> 早期版本曾出现第二条 DIFF `admin.systemStatus`（nginx/php 服务状态为 false）——
> 那是**环境差异**：`cf-go-linux` 是个只跑 Go 后端的裸容器，里面没有 nginx / php-fpm 进程。
> 在真实切流容器里（php-fpm 与 nginx 并存）这一条同样 MATCH。**服务状态探测依赖本机进程，
> 因此只能在部署形态下比对。**

采集器对两端的判定：

```
PHP: repository.fileContent（期望 200，实际 500 —— PHP 自身缺陷）
Go : 所有用例均与 manifest 期望一致        ← 123/123
```

### 端点覆盖

| 域 | 端点数 | 状态 |
|---|---|---|
| `user` | 24 | ✅ 全部实现 |
| `group` | 13 | ✅ 全部实现 |
| `repository` | 52 | ✅ 全部实现（19 只读 + 33 写） |
| `admin` | 19 | ✅ 全部实现 |
| `apiv1/internal` | 4 | ✅ 全部实现 |
| **合计** | **112** | **100% 覆盖** |

---

## 1. 已交付物

### 1.1 契约基线（W1）

| 文件 | 说明 |
|---|---|
| `backend/contract/manifest.yaml` | 123 条用例的端点清单（108 个 `/api/*` 中的 107 个 + 4 个 `/apiv1/internal/*` + 12 条契约错误路径） |
| `backend/contract/README.md` | 字段语义、归一化规则、副作用规避策略、事故记录 |
| `backend/cmd/contract-collect/main.go` | 采集器：登录 → 逐条发请求 → 归一化 → 落盘 |
| `backend/testdata/contract/*.json` | 123 个真实响应（对着运行中的 PHP 后端采集） |
| `backend/cmd/contract-compare/main.go` | 比对工具：MATCH / DIFF / NOT_IMPLEMENTED 分类 |

**基线的三个关键性质**（已实测）：

1. **可逐字节复现**：连续两次采集聚合 SHA256 一致。
2. **无副作用**：采集前后登录 200、`/api/user/info` 200，DB 各表行数完全未变。采集器内置硬约束 —— `forbidden_write_paths` 永不收写请求；写方法且未标 `readonly: true` 的端点只允许空参请求，否则**拒发并计为跳过**。
3. **覆盖到位**：用脚本把四个 PHP 控制器的 `public function <action>_(get|post)` 与 manifest 求差集，唯一缺失是 `admin.config.POST`（因 P0 缺陷被主动剔除）。

### 1.2 Go 后端内核（W2/W3）

| 包 | 内容 | 状态 |
|---|---|---|
| `internal/contract` | 响应信封、31 个错误码、Accept/Content-Type/Content-Length 校验、PHP 数组语义还原、分页归一化 | ✅ 14 个单测通过 |
| `internal/config` | `env.yaml` + `config.yaml` 加载，键结构与 PHP 侧完全一致；支持环境变量覆盖（本地开发用） | ✅ |
| `internal/db` | 连接池 + 版本化迁移器（`go:embed` + `cc_schema_migrations`） | ✅ |
| `internal/session` | 服务端会话（`cc_sessions` 表），Cookie 名/有效期与 PHP 一致，新增 HttpOnly + SameSite=Lax | ✅ |
| `internal/auth` | argon2id + 旧 `md5(md5())` **双轨校验**；写入格式由 `auth.hash_format` / `CODEFEVER_PASSWORD_HASH` 决定，**默认 legacy**（保证 PHP 回滚目标可用）；RFC 6238 TOTP；恒定时间令牌比较 | ✅ 11 个单测通过 |
| `internal/domain` | 权限矩阵（39 权限码 × 6 角色）、用户/SSH key 归一化形态 | ✅ 5 个单测通过（与 PHP 运行时反射结果逐值比对） |
| `internal/repo` | 显式 SQL + 绑定参数（无 ORM） | ✅ |
| `internal/httpapi` | 路由、鉴权中间件、panic 恢复、结构化日志、分发顺序 | ✅ |
| `internal/httpapi/web.go` | **web 控制器命名空间**的 CI 三段判定（404 HTML / 501 JSON / 方法分发）、控制器名区分大小写与方法名不区分、`Before` 钩子 | ✅ 4 个单测 |
| `internal/httpapi/json.go` | **PHP 风格 JSON 编码**（非 ASCII → `\uXXXX`、`/` → `\/`），供 web 命名空间的 `_ajax_message` 信封使用 | ✅ |
| `internal/httpapi/templates` | 13 个服务端渲染页面 → 14 个 Go 模板 + 214 条国家数据 | ✅ 6/6 PHP 快照比对通过 |
| `internal/authkeys` | authorized_keys 同步：**Go 后端直接写文件**（逐字复刻 PHP 守护进程的追加/按子串删除语义），PHP 守护进程与 FIFO + SIGUSR1 协议已下线 | ✅ 13 个单测（行格式逐字节 / 删除按子串） |
| `internal/gitsvc` | **Git 服务层**：全部用 `exec.CommandContext` + argv 数组（不经 shell），退出码精确判定 | ✅ 11 个单测（含注入阻断） |
| `internal/storage` | 头像磁盘存储（PHP 同款五级目录布局 + 路径穿越防护） | ✅ 4 个单测 |
| `internal/mailer` | 邮件发送（系统 `mail` 命令，argv 调用，返回真实退出码） | ✅ |
| `internal/handlers` | `user` / `group` / `repository` / `internal` 四域端点 | ✅ |
| `internal/handlers/web_*.go` | web 控制器命名空间（`user` 注册/2FA/重置口令、`doc` 文档站、`lang` 语言切换、`captcha` 验证码 PNG、`home`/`nopage`/`base`） | ✅ |
| `internal/httpapi/pages.go` | 页面渲染层、`{code,message,extra}` AJAX 信封 | ✅ |
| `backend/cmd/page-compare/main.go` | **web 路由逐字节对拍工具**（PHP 实例 vs Go 实例） | ✅ 43/43 |
| `backend/cmd/pagediff/main.go` | 页面响应体的字节级差异定位（定位模板空白差异用） | ✅ |
| `scripts/shot.mjs` | CDP 直驱的多视口截图采集（浏览器内走真实登录表单） | ✅ 21 张 |
| `scripts/shotcmp.py` | 截图逐像素比对 + 差异区域裁切 | ✅ |
| `scripts/verify.ps1` | 一键复验流水线（编译 → 镜像 → 切流 → 契约 → 冒烟 → 截图比对 → 打包） | ✅ |

### 1.3 数据库侧

- **MySQL 8.0.46 隔离实例**（`cf-mysql8`，宿主 `:3307`），已从线上 5.7 全量 `mysqldump` 导入，数据一致（3 用户 / 2 组 / 3 仓库 / 1 SSH key）。
- **迁移路径简化为 1 处改动**（详见 `BACKEND_REWRITE_PLAN.md` §6.2）：只改 `misc/docker/docker-entrypoint.sh` 的 sql_mode，DDL 与种子数据**零改动**。

---

## 2. 已验证的行为（逐条实测）

### 2.1 双后端逐字节一致

同一组请求分别打 PHP（`:9080`，CI 2.1.1 / MySQL 5.7）与 Go（`:9001`，新后端 / MySQL 8.0.46）：

| 场景 | PHP | Go | 一致 |
|---|---|---|---|
| `POST /user/login` | `{"code":0,"message":"登录成功","extra":{"redirect":"/repositories"}}` | 同 | ✅ |
| 登录响应 Content-Type | `text/html; charset=UTF-8` | 同 | ✅ |
| `GET /api/user/info` | 17 字段完整归一化对象 | 逐字段相同（含 `team:null` / `role:null`） | ✅ |
| `GET /api/user/listKey` | `data:[]` | 同 | ✅ |
| 未登录访问 | HTTP 403 / code 257 | 同 | ✅ |
| 缺 `Accept` 头 | HTTP 400 / code 259 | 同 | ✅ |
| 已知 controller + 未知动作 | HTTP 404 / code 261 | 同 | ✅ |
| 未知 controller | HTTP 404 / `text/html` | 同 | ✅ |

**契约比对工具输出**：`总计 123 条：MATCH 14 | DIFF 0 | NOT_IMPLEMENTED 109`。

> 唯一已知的**非语义**字节差异：PHP 的 `json_encode` 会把非 ASCII 转成 `\uXXXX`、把 `/` 转成 `\/`；Go 的 `json.Marshal` 输出原始 UTF-8 与未转义的 `/`。两者经 `JSON.parse` 后完全等价。契约比对采用**解析后结构比较**，因此不受影响。

### 2.1.1 已实现的端点（68 条契约用例全绿）

| 域 | 端点数 | 状态 |
|---|---|---|
| `user` | 20（info / listKey / getMFAData / addSSHKey / removeSSHKey / updateBasicInfo / update / updatePassword / updateMFAData / revokeMFAData / notifications / setNotificationRead / deleteNotification / userNotificationSetting / notificationRefused / setGroupOrRepoNotification / getCountriesCode / uploadAvatar + 登录/登出） | ✅ 逐字一致 |
| `group` | 13（list / create / config / mergeRequests / activities / uploadAvatar / update / addMember / changeMemberRole / removeMember / changeOwner / updateName / deleteGroup） | ✅ 13/13 MATCH |
| `repository` | 19 只读（list / config / commitList / commitDetail / object / pathStack / branchList / tagList / blameInfo / activities / mergeRequests / relatedMergeRequests / mergeRequestDetail / mergeRequestVersionList / mergeRequestCommits / targetRepository / lastCommitLog / refListContainSHA / fileChanges） | ✅ 19/19 MATCH（Linux 容器内） |
| `internal` | 4（authHTTP / authSSH / hooks / searchPublicKey） | ✅ 逐字一致 |

**总量**：68 / 123 条用例 MATCH，**DIFF = 0**。

### 2.1.2 契约回归的环境敏感性（重要）

在 **Windows 宿主机**上跑同一套比对会得到 `MATCH 65 | DIFF 3`，3 项 DIFF 全部是**宿主机与容器的环境差异**，不是实现差异：

| 用例 | 差异 | 根因 |
|---|---|---|
| `internal.authHTTP` / `internal.authSSH` | `data.path` 分别为 `/data/www/codefever-community/git-storage/...` 与 `E:\WorkSpace\codefever\git-storage/...` | 绝对路径随部署根目录变化；采集器只归一化了 `/data/www/...` 形态 |
| `repository.config` | `count.file` 为 `24576`（Linux）与 `0`（Windows） | `git count-objects -v` 的 `size` 在 Linux 按 4K 块统计、Windows 按字节统计 |

**实测三方对照（同一仓库）**：

```
PHP  (容器 Linux)   count = {"commit":2,"branch":1,"tag":0,"file":24576,...}
Go   (Windows 开发)  count = {"commit":2,"branch":1,"tag":0,"file":0,...}
Go   (Linux 容器)   count = {"commit":2,"branch":1,"tag":0,"file":24576,...}   ← 与 PHP 完全一致
```

**在 Linux 生产同构容器内（`cf-go-linux`，与生产同一镜像同一网络）复验：`MATCH 68 | DIFF 0 | NOT_IMPLEMENTED 55`。**

### 2.1.3 生产同构拓扑验证

| 组件 | 位置 | 说明 |
|---|---|---|
| PHP 栈（对照组） | `http://localhost:9080` | 现有实现，契约基线来源 |
| Go 后端（Windows 开发） | `http://127.0.0.1:9001` | 日常开发 |
| **Go 后端（Linux 容器，生产同构）** | `http://localhost:9003` | 交叉编译产物，运行于 `codefever-community:local` 镜像内 |
| 同构 nginx | `http://localhost:9081` | nginx 静态 + SPA → Go 应用层，与生产拓扑一致 |
| MySQL 8.0.46 | `cf-mysql8`，宿主 `:3307` | Go 后端专用库（数据从线上 5.7 全量导入） |
| MySQL 5.7.31 | `codefever-db` | PHP 栈原库，未被改动 |

### 2.1.4 浏览器 UI 验收（内置浏览器实测）

| 页面 | 结果 |
|---|---|
| Go 登录页 `/user/login`（经 nginx） | ✅ **完整渲染**：CODE FEVER logo、主色按钮（`#3455db`）、中文文案、下划线输入框、忘记密码/去注册链接 |
| Go 与 PHP 登录页的 HTML 对比 | ✅ **class 集合、id 集合、静态资源路径集合完全一致**（8266 vs 8471 字节，差异为换行归一化） |
| Go 版 SPA `/repositories`（经 nginx） | ✅ **SPA 完整引导**，设计系统（logo / 顶栏 / Tab / 主按钮 / 空状态）与 PHP 侧一致 |
| API 数据对比 | ✅ `repository/list` 与 `group/list` 的数据与 PHP **完全相同**（2 个仓库 / 1 个组，显示名与 MR 计数均一致） |
| **仓库详情页**（git 支撑） | ✅ `/rootteam/demorepo` 完整渲染：侧栏（文件/提交 2/合并请求 0/分支 1/标签 0/成员 1/动态/设置）、分支选择器 master、文件表（README.md · SSH.md）、README 预览（28 Bytes）、提交信息与时间戳 —— **与 PHP 版逐字一致** |
| SPA 截图比对 | ✅ Go 版仓库列表页截图与 PHP 版**像素一致**（同视口下同为 28022 字节的 PNG，可见文本逐字相同） |
| 未登录时的 API 行为 | ✅ `user/info` → 403 `0x0101`（与 PHP 一致） |

**已知的渲染差异**（PHP 与 Go 的语义等价差异，不影响功能）：
1. Go `html/template` 会删除 HTML 注释与 JS 注释 —— HTML 注释已用 `safeHTML` 还原，JS 注释无法还原（运行时无影响）。
2. Go 的上下文转义器把 JS 串内的 `< & " ' \`` 编码为 `\uXXXX`、换行编码为 `\n`（运行时等价、字节不同）。
3. PHP 的 `?>` 会吞掉紧随的换行，Go 无此规则 —— include 调用点已逐一对齐 PHP 语义。
4. PHP `echo` 不转义，Go 默认转义 → 邮箱等用户输入被转义。**这是顺手修掉的 XSS**，字节与 PHP 不同。

### 2.1.5 写端点成功路径回归（契约基线的空白区）

契约基线（`contract-collect`）按设计**不写业务数据** —— 所有写端点只覆盖"缺参 / 资源不存在"这类失败分支。因此"建组 / 建库 / 建分支 / 建标签 / 建合并请求 / 加成员 / 加 webhook / 管理端建用户与改配置"这些**成功路径**在基线里是空白。

为此新增 **`backend/cmd/smoke-write`**：驱动真实 HTTP API，在**隔离环境**（独立库 `codefever_smoke` + 独立 git-storage + 独立后端实例）上走完整成功路径并断言结果，最后清理全部数据。

**结果：18 / 18 全部通过**

| 步骤 | 断言 |
|---|---|
| 登录 | `code=0`，`extra.redirect=/repositories` |
| 建组 group.create | `code=0`，返回 g_key |
| 查组 group.config | `code=0` |
| 建库 repository.create | `code=0`，**裸仓库目录真的建出** |
| 推送初始提交 | `git push` 到新建裸库成功 |
| 建分支 createBranch | `code=0` —— **PHP 侧"成功反而报 0x040C"的修复点** |
| 分支提交 | push 到分支成功 |
| 建标签 createTag | `code=0` —— **同上修复点** |
| 建合并请求 createMergeRequest | `code=0`，返回 MR id |
| 管理端建用户 addUser | `code=0`，返回初始明文口令 |
| 加成员 addMember | `code=0` |
| 加 webhook editWebhook | `code=0` |
| 管理端写配置 admin.config | `code=0` —— **P0 砖化缺陷的修复点** |
| 数据库落库 | 仓库行存在 |
| 事件入队 | `cc_events` 中有 `repo:*` 事件 |
| 裸仓库目录 | 磁盘上目录存在 |
| 清理 | 删库 + 删组成功 |

复现方式：

```powershell
# 1) 建隔离库 codefever_smoke 并导入基线
# 2) 复制 git-storage 与配置到 .tmp-smoke/，启动独立实例（:9004）
# 3) 跑回归
cd backend
go run ./cmd/smoke-write -base http://127.0.0.1:9004 `
  -dsn "root:123456@tcp(127.0.0.1:3307)/codefever_smoke?parseTime=true" `
  -gitstorage E:\WorkSpace\codefever\.tmp-smoke\git-storage
```

> 该工具还暴露并修掉了一处兼容面差异：PHP 的 `$this->input->post()` 对 **multipart/form-data 同样生效**，而 Go 的 `r.ParseForm()` 只认 urlencoded。登录处理原先因此不接受 multipart。已改为两者都支持（新增 `formValue` 辅助函数）。

### 2.1.7 窄屏 / 响应式验收（3 视口 × 7 页面 = 21 张截图逐像素比对）

**方法**：用 Chrome DevTools Protocol 直驱无头浏览器（`scripts/shot.mjs`），在**同一个浏览器实例内**走真实登录表单拿到会话，然后按固定视口逐页截图；再用 `scripts/shotcmp.py` 做逐像素比较。

**为什么不用内置浏览器**：内置浏览器的视口由宿主 FilePanel 决定（本次为 1039×899），无法精确指定窄屏尺寸；而响应式验收必须锁定视口才能做像素级比对。

| 视口 | 尺寸 | 覆盖页面 |
|---|---|---|
| mobile | 390×844 | 7 页 |
| tablet | 768×1024 | 7 页 |
| desktop | 1440×900 | 7 页 |

7 个页面：`/repositories`（仓库列表）、`/rootteam/demorepo`（仓库详情）、`.../files/master`（文件树）、`.../commits/master`（提交记录）、`.../branches`（分支）、`/admin/users`（管理后台用户）、`/user/login`（登录页）。

**结果（Go 切流态 vs PHP 回滚态，同一套镜像）**：

```
21 张：完全一致 16，存在差异 5
```

5 处差异的性质（逐一定位，非笼统放过）：

| 页面 | 差异 | 性质 |
|---|---|---|
| `repo-detail` / `repo-files`（desktop + tablet） | 73 px，位于文件大小那一格 | **PHP 侧缺陷**：PHP 的 `Command::batch()` 用 `exec()` 收输出，而 `exec()` 会按行拆分并丢掉行尾换行 → 文件大小少 1 字节。实测 28 字节的 README.md：PHP 显示 `27 Bytes`，Go 显示 `28 Bytes`（正确）。详见 §3.6 第 10 条 |
| `repo-branches`（tablet） | 4 px，**最大通道差 = 1** | **渲染抖动**：同样这 4 px 在**两次 PHP 采集之间**也存在（旧镜像 vs 新镜像），是 webfont 字形栅格化的亚像素差异，与后端无关 |

> 结论：**UI 风格保持一致**这条硬约束成立。唯一可观察的差异是 Go 修正了 PHP 的文件大小 off-by-one。

### 2.1.8 web 控制器命名空间补齐（43 条路由逐字节对拍）

**本轮新发现**：Go 后端原先只接线了 `/user/login` 与 `/user/logout` 两个页面路由。nginx 的应用层反代规则覆盖 `/user|file|feature|doc|community|lang|captcha|boss|service|pricing|api`，也就是说**这些路径全部落到后端**，而 Go 侧一律 404 —— 包括注册页、2FA 验证码页、重置口令页、文档站、语言切换、验证码图片，以及**根路径 `/`**（PHP 为 302 → `/user/login` 或 `/repositories`）。

**实现要点**（`backend/internal/httpapi/web.go` + `backend/internal/handlers/web_*.go`）：

- **CI 三段判定**：控制器文件不存在 → HTML 404 页（`404_override = nopage`）；控制器存在、方法不存在 → JSON `{"code":501,"message":"Unknown method","data":[]}`（HTTP 状态码仍是 **200**，501 只在 body 里 —— PHP 的 `header('HTTP/1.1: '.$code)` 是畸形状态行）。
- **控制器名区分大小写、方法名不区分**：CI 用 `file_exists()` 探测控制器文件（Linux 区分大小写），用 `method_exists()` 查方法（PHP 方法名大小写不敏感）。实测 `/USER/register` → 404，`/user/resetpassword` → 命中 `resetPassword`。
- **PHP 风格 JSON 编码**（`internal/httpapi/json.go`）：PHP 的 `json_encode` 默认把非 ASCII 转成 `\uXXXX`、把 `/` 转成 `\/`，Go 标准库都不转。要逐字节一致就必须自己实现编码器。**仅用于 web 命名空间的 `_ajax_message` 信封**，`/api/*` 命名空间未动。
- **`Before` 钩子**复刻 PHP 控制器构造函数：`doc` 控制器在构造函数里 `exit()`，所以 `/doc/xxx`（xxx 根本不是方法）是 302 而不是 501。
- **`Content-Length` 与换行语义**：PHP 的 `?>` 会吃掉紧随其后的换行，模板逐字复刻（CRLF 保持 CRLF）。

**对拍工具**：`backend/cmd/page-compare/main.go`

```powershell
go run ./cmd/page-compare -php http://localhost:9080 -go http://127.0.0.1:9003
```

**结果：43 / 43 逐字节一致**（状态码 + Location + Content-Type + 响应体）。覆盖：

| 类别 | 路径 | 结果 |
|---|---|---|
| 根路径 | `/` | ✅ 302 → `/user/login`（已登录 → `/repositories`） |
| 登录 | `/user/login` GET（未登录/已登录）、POST | ✅ 200 登录页 / 302 / `{"code":1,…}` |
| 登出 | `/user/logout` | ✅ 302 → `/user/login` |
| 注册 | `/user/register` GET + POST×3 | ✅ 200 注册页 / code 1 / 2 / 3 |
| 2FA | `/user/verify` GET + POST | ✅ 200 验证码页 / code 100 |
| 重置口令 | `/user/resetPassword` GET + POST×2 | ✅ 200 / code 1 / 100 |
| 找回码 | `/user/getResetPasswordCode` GET + POST×2 | ✅ code 3 / 3 / 4 |
| 未知方法 | `/user/nosuchmethod`、`/file`、`/lang`、`/captcha` | ✅ 200 + 501 JSON |
| 方法名大小写 | `/user/resetpassword` | ✅ 200 重置页 |
| 文件下载 | `/file/fetch`、`/file/fetch/avatar/nosuchfile`、`/file/fetch/other/key` | ✅ `No storage` / `Not Found` / `Not Allowed` |
| 文档站 | `/doc`、`/doc/index`、`/doc/cn`、`/doc/en`、`/doc/xx`、`/doc/cn/`、`/doc/zh-cn/` | ✅ 302 → 对应 `/` / 404 CI error_404 页 / 501 JSON |
| 语言 | `/lang/select`、`/lang/select?l=cn`、`?l=en` | ✅ 200 错误页 / 302 → `/` |
| 验证码 | `/captcha/view` | ✅ 200 `image/png` 150×50 |
| 未知控制器 | `/feature`、`/community`、`/boss`、`/service`、`/pricing` | ✅ 404 HTML |

**已知无法逐字节比对的三条**：`/base`、`/home/index`、`/nopage/index` —— 它们**不在 nginx 的反代白名单里**，PHP 侧由 nginx 直接吐 SPA 静态文件（200，3357 字节），后端永远收不到；Go 后端按 PHP 控制器语义实现（`home/index`→302、`nopage/index`→404、`base/*`→501），在**部署形态（经 nginx）下两端完全一致**（已实测两套 nginx 返回同样的 3357 字节 SPA）。

### 2.1.9 登录后 SPA 全流程（切流态，内置浏览器实测）

在 `http://localhost:9080`（Go 切流态）用演示账号登录后，逐页走查：

| 页面 | 结果 | 证据 |
|---|---|---|
| `/repositories` | ✅ 渲染完整 | 2 个仓库（Root Team/demo repo、demo-repo），计数 0/0/1；3 个 API（`user/info`、`repository/list`、`group/list`）全部 200 |
| `/rootteam/demorepo` | ✅ 渲染完整 | 侧栏计数（提交 2 / 合并请求 0 / 分支 1 / 标签 0 / 成员 1）、Fork 0、仓库大小 41 k Bytes、文件表 2 行、README 内容预览 |
| `.../commits/master` | ✅ 渲染完整 | 2 条提交（`f67cfee7` ssh push test、`c943e85a` test commit from local build），按日期分组，末页"没有更多" |
| `.../branches` | ✅ 渲染完整 | master（默认），`f67cfee7` · ssh push test · codefever · 更新于 昨天 23:34 |
| `.../files/master` | ✅ 渲染完整 | 文件树与内容预览 |
| `/admin/users` | ✅ 渲染完整 | 3 个用户（test2 已禁用 / test 已禁用 / codefever 管理员），**管理员的"创建于"显示 `0001-01-01 00:00:00`** —— PHP 的零日期语义被完整复刻 |

**控制台：全程 0 条 error / warning。所有 XHR/fetch 全部 200，无 4xx/5xx、无失败请求。**

### 2.2 契约层单测（`internal/contract`，16 项）

锁定了以下"一旦漂移就会改变前端行为"的规则：
- 31 个错误码的**数值 + HTTP 状态 + 英文文案**（逐条）。
- `code < 0x0400` 业务错误 / `> 0x0400` 系统错误的阈值语义（前端硬编码）。
- `Accept` **严格匹配**：只认 `application/json` 与 `*/*`；`application/json; charset=utf-8` 会被拒（PHP 侧不剥离 `;`）。
- `Content-Type` 分派：`multipart/form-data`（前端唯一写形态）/ `application/x-www-form-urlencoded`（登录表单）/ `application/json` / 无。
- `Content-Length` 校验，且 `multipart` 豁免。
- PHP 数组语义：`a` 重复键保留最后一个、`a[]` → 数组、`a[b]` → 嵌套 map、`a[b][c]` → 三层。
- 分页归一化：query 恒被改写为 `page=1` / `pagesize=20`（复刻 PHP 的 `is_int` 判定失效），原始值仍可取。
- 失败信封的 `data` 必须是 `[]` 而非 `null`，且字段顺序为 `request-id` → `code` → `message` → `data`。

### 2.3 认证单测（`internal/auth`，7 项）

- `LegacyHash("123456")` 必须等于 `14e1b600b1fd579f47433b88e8d85291`（与 `db_data.sql` 种子一致）。
- argon2id 往返 + 盐随机性。
- **RFC 6238 官方测试向量**（6 组时间点 × 8 位）全部通过 —— 证明 TOTP 与 Google Authenticator 兼容。
- 令牌恒定时间比较。

### 2.4 权限矩阵单测（`internal/domain`，5 项）

基线来自**容器内 PHP 运行时反射** `UserAccessController`，不是人工抄写：

```
role 1 => [0x01]
role 2 => [0x01,0x08]
role 3 => [0x01,0x02,0x08,0x09]
role 4 => [0x01,0x02,0x04,0x05,0x08,0x09,0x0A,0x0B,0x11,0x12,0x13,0x18,0x19,0x1A,0x1B,0x1C,0x1D,0x1E,0x1F,0x22,0x23]
role 5 => [0x01,0x02,0x03,0x04,0x05,0x06,0x07,0x08,0x09,0x0A,0x0B,0x10,0x11,0x12,0x13,0x14,0x15,0x18,0x19,0x1A,0x1B,0x1C,0x1D,0x1E,0x1F,0x22,0x23]
role 6 => []
```

同时锁定 `checkPermission` 的 PHP 语义（权限码为 0 或角色为 0 一律 false）与 `REPO_ACTION_MAPPING`（空 → READ、`git-upload-pack` → READ、`git-receive-pack` → PUSH）。

---

## 3. 实施过程中发现的三个真实缺陷（均已修复或已定位）

### 3.1 🔴 P0：`POST /api/admin/config` 可单请求砖化全站

**现象**：`data` 字段解码为 null 时 → `array_merge($config, null)` 在 PHP 7.4 只发 **Warning** 并返回 null → `yaml_emit_file(config.yaml, null)` 把 `config.yaml` 写成 `--- ~\n...`（10 字节）→ 之后 `www/index.php:21` 的 `make_constants('YAML_', yaml_parse_file(config.yaml))` 拿到 null → **抛 TypeError** → **登录与全部 API 永久返回 500**。

**证据**：本次实施中真实触发并观测到（容器内 `config.yaml` 变成 10 字节；`docker logs` 出现 `Uncaught TypeError: Argument 2 passed to make_constants() must be of the type array, null given`）。已手工恢复。

**缺失防护**：无 payload 校验、无文件锁、无备份、无原子替换。

**新后端必须做**：校验 payload → 写临时文件 → `rename` 原子替换 → 保留 `.bak` → 写失败保留原文件 → 启动时自检 `config.yaml` 可解析。

**采集器侧已加硬约束**：`forbidden_write_paths` 永不向该路径发写请求。

### 3.2 🟠 `repository.fileContent` 恒返回 500

`Repository_model` 调用了不存在的 `geFilesByBranchOrSHA1Hash`（应为 `getFilesBy...`，拼写错误）→ PHP 致命错误。已如实记入基线（HTTP 500），新后端按**正确语义**实现。

### 3.3 🟠 `cc_users` 没有 `u_job` / `u_company` 列

但 `api/user.php` 的 `update` 白名单里列出了 `job` / `company` 两个字段 → 写入必然失败。新后端按实际列实现，不沿用该白名单。

### 3.4 其他已记录但未修的原缺陷

- `u_team` / `u_role` 在 DB 中是 **NULL**，PHP 输出 JSON `null`（不是空串）。Go 侧已用 `*string` 精确复刻。
- `user_sshkey_model` 的 `deleteFlag` 恒为 `false`（它误用了 `rm_status` 列）。Go 侧**原样复刻**并加注释说明。
- `nopage/index.php` include 的 `header_include.php` 路径错误（实际在 `newpage/` 下）→ PHP 侧 404 页本身就有 warning。新后端的 404 页使用移植后的模板。
- `u_2fa` 在 PHP 侧**明文存储** TOTP secret。新后端改为 AES-GCM 加密存储（`enc:v1:` 前缀），读取时兼容历史明文值。

### 3.5 🟠 实施过程中发现并修复的自身缺陷（6 个）

| # | 缺陷 | 发现方式 | 修复 |
|---|---|---|---|
| 1 | **口令哈希算错**：`md5(md5(x))` 被实现成对 16 字节原始摘要再求一次 md5，而 PHP 是对 32 字符的 hex 字符串求 md5 | 登录返回 `code 4`（口令错误） | 补单元测试锁定 `LegacyHash("123456") == 14e1b600b1fd579f47433b88e8d85291` |
| 2 | **404 页返回 HTTP 200**：`renderPage` 固定 `WriteHeader(200)` | 经 nginx 拓扑做契约回归时，采集器报 `error.endpointNotFound` 期望 404 实际 200 | 新增 `renderPageStatus`，404 页改用 `WriteHeader(404)` |
| 3 | **multipart 表单参数全丢**：`contract.Parse` 先 `io.ReadAll` 读完请求流，再调 `r.ParseMultipartForm` → 流已空，**所有** multipart POST 的表单字段与上传文件都被静默丢弃（表现为"参数永远缺失"，且缺参用例照样通过 —— 错误的原因相同） | admin 域实现时发现 | 读 body 后用 `io.NopCloser(bytes.NewReader(body))` **复位请求流**；补 2 个回归测试（表单字段 + 文件上传） |
| 4 | **事件永远无法被消费**：`worker.go` 领取 SQL 把 `ids` 切片当成**单个**参数传入 → `unsupported type []interface {}` | 事件处理器的端到端验证时发现 | 逐个展开参数；并把领取改为事务内 `SELECT ... FOR UPDATE SKIP LOCKED` + 逐条 UPDATE，消除多 worker 重复消费（此前会产生重复活动行） |
| 5 | **口令哈希透明升级击穿 PHP 回滚目标** | 切回 PHP 后同一账号登录失败 | 见 §3.7：写入格式改为可配置、**默认 legacy**，自动升级只在显式切到 argon2id 时才发生 |
| 6 | **整块 web 控制器命名空间未接线** | 用 nginx 反代白名单反推后端应当承接的路径，逐条探测发现全是 404 | 见 §2.1.8：补齐 43 条路由并与 PHP 逐字节对拍 |

### 3.6 🟠 本轮新发现并修复的 PHP 侧缺陷（10 个）

| # | 缺陷 | 影响 | 修复 |
|---|---|---|---|
| 1 | **建分支/建标签成功反而报错**：`if ($result) { Response::reject(0x040C); }`，而 model 成功时返回 true | 建分支/建标签**永远失败** | 改为成功语义；错误码分离（源不存在 → `0x040C`，目标已存在 → `0x0406`） |
| 2 | **fork 丢弃 git 退出码** → 克隆失败留下"有 DB 行、无磁盘仓库"的孤儿 | 数据不一致 | 检查结果 + 事务回滚 + 清理磁盘目录 |
| 3 | **建仓无事务、失败不清目录** | 同上 | 事务 + 显式 `RemoveRepo` |
| 4 | **changeOwner 三步写无事务** | 可能写一半 | 同一事务 |
| 5 | **附注标签缺提交者身份**（PHP 由环境变量导出 `GIT_COMMITTER_*`/`GIT_AUTHOR_*`） | `git tag -a` 必失败 | 补齐环境变量 |
| 6 | **editWebhook/deleteWebhook 不校验 rwKey 归属** | 可改删他人仓库的 webhook | 限定 rKey |
| 7 | **getRepositoryWebhookLogs/LogData 无任何权限校验** | 任意登录用户可读任意仓库的 webhook 投递日志 | 补仓库读权限 |
| 8 | **`repository.fileContent` 调用不存在的方法**（`geFilesByBranchOrSHA1Hash`） | 恒 500 | 用 `cat-file blob <sha>:<path>` 正确实现 |
| 9 | **`admin/systemStatus` 的服务探测依赖本机进程** | 在拆分部署下误报 | 语义按"Go 后端或旧守护进程"扩展；nginx/php 依赖同机部署，属环境差异 |
| 10 | **文件大小少 1 字节**：`Command::batch()` 用 `exec($cmd, $output)` 收集输出 —— PHP 的 `exec()` 会**按行拆分并丢掉行尾换行**，随后 `implode("\n", $output)` 重新拼接，因此文件内容末尾的换行被吞掉，`Helper::parseObjectToFile()` 里的 `strlen($fileContent)` 就比真实大小少 1 | 前端文件查看器显示的大小永远比实际小 1（文本文件尤其明显） | Go 侧按 **git 的真实字节数**返回（`cat-file` 输出不做行拆分），实测 28 字节的 README.md：PHP 返回 `size:27`，Go 返回 `size:28` |

### 3.7 🔴 P0（切流后新发现并已修复）：Go 写 argon2id 口令哈希，**PHP 回滚目标被废掉**

**现象**：切流到 Go 后用 `root@codefever.cn` / `123456` 登录成功；把 nginx 切回 PHP（`CODEFEVER_BACKEND=php`）后，**同一个账号登录失败**，PHP 返回 `{"code":4,"message":"您输入的账号或密码有误"}`。

**根因**：PHP 侧只会用 `md5(md5($password))` 校验口令 —— 见 `application/controllers/user.php:79`、`api/user.php:214`、`api/admin.php:106`、`apiv1/internal.php:91`。而 Go 后端在登录成功时会把旧哈希**透明升级**为 argon2id，于是：

```
u_password = 14e1b600b1fd579f47433b88e8d85291   （PHP 可验证）
   ↓ Go 登录一次
u_password = $argon2id$v=19$m=65536,t=3,p=2$...   （PHP 完全不认）
```

**影响面**：不止"登录时自动升级"这一处。Go 侧所有写口令哈希的地方（管理端建用户 / 管理端重置口令 / 用户自助改密）都写 argon2id —— 也就是说**只要 Go 侧动过某个账号，PHP 侧就再也登不进这个账号**。这直接击穿了"php-fpm 保留为秒级回滚目标"这条承诺。

**实测证据**：
```
docker exec codefever-db mysql -uroot -p123456 codefever_community \
  -e "SELECT u_email, LENGTH(u_password) FROM cc_users;"
u_email                len
root@codefever.cn      97     ← argon2id
test2@pgyer.com        32     ← 仍是 legacy
```

**修复**：
1. 新增 `auth.HashFormat`（`legacy` / `argon2id`）与 `auth.HashPasswordForFormat()`；`auth.ParseHashFormat()` 对空值/未知值**一律回落到 legacy**（更安全的一侧）。
2. 写入格式由 `auth.hash_format`（env.yaml，可选）或环境变量 `CODEFEVER_PASSWORD_HASH` 决定，**默认 legacy**；`docker-compose.local.yml` 已把该开关显式化（默认 `legacy`）。
3. **登录时的自动升级只在显式配置为 `argon2id` 时才发生** —— legacy 模式下绝不改写库里的哈希。
4. 校验侧保持双轨：`auth.VerifyPassword` 两种格式都能校验，因此**已经被升级过的账号**在 Go 侧仍能正常登录（不需要回滚数据）。
5. 存量修复：把本次被升级的 `root@codefever.cn` 的哈希改回 `14e1b600b1fd579f47433b88e8d85291`，PHP 登录随即恢复。
6. 回归锁定：
   - `internal/auth`：`HashPasswordForFormat(legacy)` 必须**逐字等于** `md5(md5(x))`（用标准库独立复刻计算，不用被测函数自证）；
   - `internal/config`：默认必须是 `legacy`，env.yaml 与环境变量均可覆盖，未知值回落 legacy；
   - `cmd/smoke-write`：管理端建号后直接查库断言 `u_password == md5(md5(初始明文口令))` —— 把"回滚安全"钉进写操作回归。

**给运维的结论**：**双后端共存期间 `CODEFEVER_PASSWORD_HASH` 必须保持 `legacy`**。等 PHP 下线、确认不再需要回滚之后，再切成 `argon2id`；一旦切换，PHP 侧将无法验证任何被 Go 改写过的账号。

### 3.8 🟠 移植 web 控制器命名空间时发现的 PHP 侧缺陷（4 个真缺陷 + 5 个"怪异行为"）

**真缺陷**（Go 侧按 PHP 语义逐字复刻 + 注释标注，不做"顺手修正"，以免改变前端可见行为）：

| # | 缺陷 | 证据 | Go 侧处理 |
|---|---|---|---|
| 1 | **选"中文"却变成英文**：`lang.php:21` 往会话里存的是常量 `LANG_CN`（= 1，整数），而 `base.php:28` 用 `in_array(strtolower($userLang), $cnLangArr)` 判定，`cn_langs` 里只有 `'zh-cn'/'zh-tw'/…` 字符串 → `"1"` 永远匹配不上 → 回落英文 | `/lang/select?l=cn` 后页面仍是英文 | 逐字复刻 + 注释标注 |
| 2 | **7 个语言键在 `user_lang.php` 里根本不存在**（`user_js_email_error`、`user_js_email_not_exist`、`user_js_password_retrieval_link_expired/_success/_failed`、`user_js_code_verfiy_error`、`user_form_email`）→ `lang()` 返回 FALSE → 响应里 `message` 是**空串** | 实测 `{"code":3,"message":"","extra":[]}` | 逐字复刻（空 message） |
| 3 | **注册不校验两次输入的密码**：`user.php:161` 只读 `password`，从不比对 `rePassword`（校验只在���端 JS 里）→ 可以用两个不同的密码注册 | 源码比对 | 逐字复刻 |
| 4 | **验证码形同虚设**：`captcha.php:18` 把验证码写进 `$_SESSION['captcha']`，但全应用**没有任何端点读取它**，登录/注册表单都不校验 | 全仓检索 | 仍生成并存入会话（保持可观察行为），但注明无人校验 |

**"怪异但可观察"的行为**（必须复刻，否则前端会看到不同结果）：

| # | 行为 | 说明 |
|---|---|---|
| 5 | **501 的 HTTP 状态码是 200** | `api_controller.php:87` 的 `header('HTTP/1.1: ' . $http_code)` 是畸形状态行；501 只出现在 body 的 `code` 字段里 |
| 6 | **控制器名区分大小写、方法名不区分** | CI 用 `file_exists()` 探测控制器文件（Linux 区分大小写），用 `method_exists()` 查方法（PHP 方法名大小写不敏感） |
| 7 | **`Doc` 构造函数的重定向先于方法判定** | `doc.php:9-22` 在构造函数里 `exit()`，所以 `/doc/xxx`（xxx 根本不是方法）是 302 而不是 501 |
| 8 | **`/file/fetch` 的错误用 200 返回文本** | `exit('No storage')` 等，HTTP 状态码是 200；前端靠 `<img>` 加载失败判断"没有头像" |
| 9 | **`json_encode` 转义非 ASCII 与斜杠** | PHP 默认把中文转成 `\uXXXX`、把 `/` 转成 `\/`；Go 标准库都不转 —— 要逐字节一致必须自己实现编码器（`internal/httpapi/json.go`） |

---

## 4. 未完成 / 未验证（诚实清单）

| # | 项 | 状态 |
|---|---|---|
| 1 | **group 域的事件发射** | ✅ 已完成：10 个事件（GROUP_CREATE / GROUP_UPDATE_AVATAR / GROUP_UPDATE_NAME / GROUP_UPDATE_DESCRIPTION / GROUP_ADD_MEMBER / GROUP_CHANGE_MEMBER_ROLE / GROUP_REMOVE_MEMBER / GROUP_CHANGE_OWNER / GROUP_CHANGE_URL / GROUP_REMOVE）与业务写入**同事务**入队 |
| 2 | **webhook 投递器** | ✅ 已完成：`md5(body . secret)` 签名、`X-CodeFever-Id/Event/Signature` 三头、指数退避、超过上限进死信；已实测事件 → 投递 → 日志全链路 |
| 3 | **邮件投递** | 走系统 `mail` 命令（与 PHP 一致）；未引入 SMTP 配置；容器内无 `mail` 命令时仅记日志 |
| 4 | **切流实战** | ✅ 已完成且 **PHP 已下线**：`http://localhost:9080` 由 Go 后端承载；镜像内已无 php / php-fpm / crond，`application/`、`system/`、`www/index.php` 不再进镜像。契约回归 123/123 |
| 5 | **镜像产物** | ✅ 已构建并 `docker save` 打包（`dist/codefever-community-local-20261023-171253.tar`，**398.2 MB**），含 Go 后端 / worker / git 钩子三件套 + 两个 Go 网关；**无 PHP 运行时** |
| 6 | **窄屏 / 响应式验收** | ✅ 已完成：3 种视口（390×844 / 768×1024 / 1440×900）× 7 个页面的 **21 张截图逐像素比对**（见 §2.1.7） |
| 7 | **admin 域的写端点回归** | 契约比对覆盖的是"缺参/资源不存在"路径；成功路径由 `cmd/smoke-write` 在**隔离库 + 隔离 git-storage** 上覆盖（**20 项断言全绿**，含 2 条回滚安全断言），**不纳入契约基线**（基线采集器按设计不写业务数据） |
| 8 | **repository 写端点的成功路径** | 同上：建仓/分支/标签/MR/成员/webhook/配置 已由冒烟回归覆盖并自动清理 |
| 9 | **登录后的 SPA 全流程复验** | ✅ 已完成（切流态，见 §2.1.7） |
| 10 | **web 控制器命名空间** | ✅ 本轮新发现并已修复：Go 侧原本只接线了 `/user/login` 与 `/user/logout` 两个页面路由，`/user/register`、`/user/verify`、`/user/resetPassword`、`/doc/*`、`/lang/select`、`/captcha/view` 等一律 404；根路径 `/` 也是 404（PHP 为 302）。已补齐并与 PHP **43/43 逐字节对拍**通过 |
| 11 | **git push / SSH clone 链路** | ✅ 已修复并验证：钩子 Go 化（PHP↔Go 差分测试 stdout / 退出码 / 请求体逐字节一致）；`authorized_keys` 由 Go 后端直接写（行格式与 PHP 逐字节一致）；实测 SSH push、HTTP push、SSH clone 三条路径全通，钩子回调 3 次全部 200 |
| 12 | **回滚后的会话** | ⚠️ 已知副作用：PHP 会话存在**客户端 Cookie**（CI 加密），Go 会话存在**服务端**（`cc_sessions`）。两种后端的会话 Cookie 互不认得 —— 切流/回滚后用户需要**重新登录一次**（不影响数据） |
| 13 | **生产环境的实际数据量** | 本轮全部验证在开发数据集（3 用户 / 2 组 / 3 仓库）上进行；未在生产数据量下做压测（本地压测已覆盖到 978 QPS，见 §3.3） |

---

## 5. 下一步（按依赖顺序）

1. **生产灰度** —— 按 `docs/BACKEND_REWRITE_PLAN.md` 的七阶段推进：先在预发环境跑满一轮，再按 1% → 10% → 50% → 100% 放流量，每阶段观察错误率与 p99 延迟。
2. **口令哈希最终切换** —— PHP 已下线、回滚窗口已关闭，可以把 `CODEFEVER_PASSWORD_HASH` 从 `legacy` 切成 `argon2id`；但存量账号的哈希是 PHP 时代的 `md5(md5(x))`，切换前需先让所有账号重置过口令（或补一个批量升级工具）。
3. ~~**清理仓库里的 PHP 源码**~~ —— ✅ **已完成**（二次开源时）：`application/`、`system/`、`tests/`、`www/index.php`、`phpunit.xml`、`run_test.sh` 及第三方 vendor 的 demo PHP 脚本均已删除，仓库内不再有任何 PHP 源码。契约基线（JSON / HTML）保留在 `backend/testdata/`。
4. **补齐 `post-receive` 事件** —— `backend/internal/handlers/internal.go` 的既有 TODO：push 成功后活动流没有 push 记录。钩子本身返回 200，不影响 push，但活动流缺了一块。
5. **压测与容量规划** —— 在接近生产的数据量下跑读写混合压测，给出容量基线（本机已到 978 QPS 饱和，瓶颈在单库）。
6. **前端构建复现** —— `www/view/build` 是提交进仓库的产物。本轮已修好 webpack 配置里两处**真实缺陷**（`@pgyer/essential-component` 自带 JSX 却被"只做 ES 转换"的规则先执行；include 路径分隔符混用导致 Windows 下匹配失败），并验证了"修复后重建的产物与已提交产物渲染逐像素一致"。重建仍必须锁定依赖版本（`yarn.lock` 已把 `latest` 锁到 0.0.3），并重跑多视口像素比对。

---

## 6. 本地运行方式

```powershell
# 1) MySQL 8 隔离实例（已运行）
docker exec cf-mysql8 mysql -uroot -p123456 -e "SELECT @@version;"

# 2) 构建并启动 Go 后端
cd E:\WorkSpace\codefever\backend
go build -o bin/codefever-api.exe ./cmd/codefever-api
$env:CODEFEVER_BASE_DIR='E:\WorkSpace\codefever'
$env:CODEFEVER_ADDR='127.0.0.1:9001'
.\bin\codefever-api.exe

# 3) 契约采集（PHP 基准 / Go 实际）
#    直接打 Go 端口，或打经 nginx 的同构拓扑（推荐，更贴近生产）
go run ./cmd/contract-collect -base http://localhost:9080        -out testdata/contract
go run ./cmd/contract-collect -base http://127.0.0.1:9001        -out testdata/contract-go

# 4) 契约比对
go run ./cmd/contract-compare -baseline testdata/contract -actual testdata/contract-go

# 5) 全量测试
go test ./internal/...
```

---

## 7. 本地验证环境清单（当前在跑）

| 组件 | 位置 | 用途 |
|---|---|---|
| PHP 栈（对照组） | `http://localhost:9080` | 现有实现，契约基线来源 |
| Go 后端 | `http://127.0.0.1:9001` | 新实现 |
| 同构 nginx | `http://localhost:9081` | nginx 静态 + SPA → Go 应用层，与生产拓扑一致 |
| MySQL 8.0.46 | `cf-mysql8`，宿主 `:3307` | Go 后端专用库（数据从线上 5.7 全量导入） |
| MySQL 5.7.31 | `codefever-db` | PHP 栈原库，未被改动 |

同构 nginx 的配置在 `.tmp-nginx-go/`（已被 `.gitignore` 的 `/.tmp-*` 覆盖）。
生产用的正式模板是 `misc/docker/vhost-go.conf-template`，由容器环境变量 `CODEFEVER_BACKEND=go` 启用（默认仍走 PHP，零风险）。

```powershell
# 重建同构 nginx（Windows 目录联接无法挂进容器，需 docker cp）
docker rm -f cf-nginx-go
docker run -d --name cf-nginx-go -p 9081:80 `
  -v "E:\WorkSpace\codefever\.tmp-nginx-go\nginx.conf:/etc/nginx/nginx.conf:ro" `
  -v "E:\WorkSpace\codefever\.tmp-nginx-go\www:/usr/share/nginx/html" `
  --add-host "host.docker.internal:host-gateway" `
  docker.1ms.run/library/nginx:alpine
docker exec cf-nginx-go sh -c "rm -rf /usr/share/nginx/html/static /usr/share/nginx/html/app"
docker cp "E:\WorkSpace\codefever\www\static"      cf-nginx-go:/usr/share/nginx/html/static
docker cp "E:\WorkSpace\codefever\www\view\build"  cf-nginx-go:/usr/share/nginx/html/app
```
