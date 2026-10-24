# HTTP 和 SSH 的选择

在克隆仓库到本地时，一般需要选择 `HTTP` 或 `SSH` 方式连接远端仓库。CodeFever 两种都支持，
clone 面板会按当前站点配置生成对应地址。

### 两种方式的比较

`SSH` 方式的优点在于：在个人设置中添加 `SSH Key` 后，每次连接远端仓库都不需要再输入
账号密码；缺点是需要自己生成并配置密钥。HTTP 方式更适合新手，但每次连接需要输入
邮箱和密码。

### 端口与 URL 形式

- Compose 默认 HTTP 端口是 `9080`，SSH 端口是 `2222`（可在 `CODEFEVER_HTTP_PORT` /
  `CODEFEVER_SSH_PORT` 修改）。
- HTTP：`http://<邮箱或用户名>@<host>:9080/<group>/<repo>.git`
- SSH：`ssh://git@<host>:2222/<group>/<repo>.git`

> SSH 必须使用 `ssh://git@host:port/...` 形式才能指定非 22 端口；scp 风格的
> `git@host:<path>` 会把 `2222` 当成路径。源码安装且 SSH 仍在 22 端口时，可以省略
> `:22`，即 `git@host:<group>/<repo>.git`。

测试 SSH 连接：

```shell
ssh -p 2222 -T git@<host>
```

> 关于如何生成并设置 `SSH Key`，参见：[获取并设置 SSH Key](ssh_key.md)。
