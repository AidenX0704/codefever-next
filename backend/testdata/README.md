# backend/testdata —— 契约基线与页面快照

本目录保存后端重写（PHP → Go）过程中沉淀的**可执行基线**，是回归比对的数据来源，
不是运行时数据。

## 目录说明

| 目录 | 内容 | 用途 |
| --- | --- | --- |
| `contract/` | 123 条 API 契约基线（每端点一个 JSON） | **生效基线**。`contract-compare -baseline testdata/contract` 的比对对象，也是 `contract-collect` 的默认输出目录 |
| `contract-go/` | 同一套端点在 Go 后端上的采集快照 | `contract-compare -actual` 的默认目录；与 `contract/` 逐条比对得出 MATCH / DIFF |
| `contract-php-now/` | PHP 后端下线前的最后一次采集 | **上游史料**。用于证明「Go 与 PHP 逐字节一致」的历史结论；`admin.systemStatus`、`repository.fileContent` 两条有意变更即以此为参照 |
| `contract-go-linux/` | Go 后端在 Linux 容器内的采集快照 | **跨平台史料**。证明 Linux 与 Windows 上的响应一致 |
| `contract-cutover/` | 切流过程中（PHP 与 Go 并存）的过渡快照 | **切流史料**，用于回溯切流窗口的行为 |
| `pages/` | 6 个服务端渲染页面的 PHP 产物快照 | `TestPagesMatchPHPSnapshots` 的比对对象（标签序列 / class / id / 资源路径 / HTML 逐字符 / JS 内容） |

## 怎么用

```bash
cd backend

# 重新采集当前实现
go run ./cmd/contract-collect -base http://localhost:9080 -out testdata/contract-verify

# 与生效基线比对（存在 DIFF 时以非 0 退出）
go run ./cmd/contract-compare -baseline testdata/contract -actual testdata/contract-verify
```

## 有意偏离 PHP 基线的地方

二次开源时，站点品牌名由 `CodeFever Community` 改为 `CodeFever Next`，因此下列
位置**有意**与 PHP 基线不同：

| 位置 | PHP 基线 | 现在的期望值 |
| --- | --- | --- |
| `contract/admin.config.json` 的 `data.email.name` | `CodeFever Community` | `CodeFever Next` |
| `pages/php-*.html` 的 `<meta name="description">` | `CodeFever Community` | `CodeFever Next` |

除此之外，Go 实现与 PHP 基线保持逐字节一致。比对结果里的每一条 DIFF 都应能对应到
上表之一，或对应 `docs/PHP_REMOVAL.md` §4.3 记录的既有行为变更。

## ⚠️ 在全新安装上跑契约回归会得到 32 条 DIFF —— 这是环境问题，不是回归

契约基线是在**上游的演示库**上采集的。那个库与 `misc/db_data.sql` 的种子数据**不是同一份**：

| | 基线采集时的演示库 | `misc/db_data.sql` 种子库 |
| --- | --- | --- |
| 仓库组 | `rootteam`（id `29efb28ef9dd3018353eb5a02c018b28`） | `codefever`（id `e6508c1a66e86f697c96bdffb30c1ae3`） |
| 仓库 | `demorepo`（id `1031c9342d6539fd36ff79f75ca4f8b7`） | `codefever`（id `2f941aef39bc8a048da55dd28c678655`） |
| 提交 | `f67cfee73d75172d4e7afd77c3efa310b460fa1b` | 无 |

`contract/manifest.yaml` 的 `demo:` 段写死了上面这些 key。因此在一份**全新安装**的库上
采集并比对，会得到约 32 条 DIFF，来源是三类：

1. **数据差异**（占多数）：所有需要 `demo.repository_key` / `demo.group_key` / `demo.commit`
   的端点，在新库上返回 403 / 404 / 不同的列表；
2. **头像载荷差异**：`admin.userList` / `admin.groupList` / `admin.repositoryList` 等
   返回的 `avatar`（Go 侧按邮箱生成的 PNG）与 PHP 基线里存下来的字节不同；
3. **已知的有意变更**：`admin.systemStatus`（少 `php`/`crond` 两个键）、
   `repository.fileContent`（PHP 恒 500 → Go 200）。

**要让契约回归跑出 123/123，必须使用与基线同一份的演示库**（即带 `demorepo` /
`rootteam` / 指定 commit 的那一份）。全新安装的环境下，这 32 条 DIFF 属于预期噪音，
不要据此判断实现是否回归。

判断自己的改动是否引入回归的正确姿势：**先在同一环境下采集一次基线，再改代码，
再采一次，两次比对**（`contract-compare` 的 `-actual` 指向两次采集结果），
而不是拿新装环境去比对历史基线。

## 不要手改

这些文件是**比对基准**，不是普通数据：

* 改动基线 = 放弃回归保护。除非是有意的行为变更（并在此说明），否则不要动。
* `contract-php-now/`、`contract-go-linux/`、`contract-cutover/` 是史料，只读不改。
