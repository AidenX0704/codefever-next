# 前端重写版 vs 旧版：字段集合逐页核对报告

> 生成日期：2026-10-24
> 核对范围：设置族 → 仓库页 → 管理后台 → MR
> 结论定位：**功能回归清单**（不是视觉走查报告）

---

## 0. 结论速览

一句话：**新版把旧版「能用的页面」重写成了「能看的页面」——字段层面不是漏画，而是整块能力被裁掉；其中约 80% 的缺失后端接口都还在，前端只是没接。**

三个可量化的口径：

| 口径 | 数值 | 含义 |
|---|---|---|
| 后端接口缺口 | **旧版调用 109 个 → 新版调用 89 个**；逐个核对后端 `handlers` 注册表，**旧版用到的 109 个里只有 2 个后端没有**（`user/confirmTel`、`user/getCode`） | 差异**几乎全部出在前端**，不是后端砍能力 |
| 旧版 i18n 词条复用 | 旧版 553 个带命名空间词条，**新版只引用 57 个（10.3%）** | 旧版信息架构被整体丢弃而非迁移 |
| 路由断链 | 新版 `/repositories/new`、`/groups/new`、`/mergerequests/new` 等 **9 个旧版路由无对应**，而新版 Header / 列表页的按钮**仍指向它们** | 点按钮即 404，属"死链"而非"缺页" |

**最危险的不是"缺功能"，而是 5 个新版自带缺陷**（详见 §6），其中 2 个会让功能**必然失败**：
Webhook 新建/编辑必报 500（缺 `secret`）、保护分支"允许合并角色"下拉多给了后端不接受的选项。

---

## 1. 核对方法与基线

| 项 | 说明 |
|---|---|
| 旧版基线 | Git HEAD `891f090` 的 `www/view/src`（146 个文件，MUI v4 + Redux + react-intl）。工作区已删除，已导出到 `%TEMP%/cf_old_view` 供静态比对 |
| 新版 | 工作区未提交的 `web/src`（React 18 + TS + Vite + 自研组件库） |
| 后端真值 | `backend/internal/handlers/*.go`（控制器注册表）、`backend/internal/domain/*.go`（白名单与结构）、`backend/testdata/contract*`（契约基线） |
| 字段判定标准 | 只写**在文件里实际看到**的字段，位置标到 `文件:行号`；不按命名猜测 |
| 结论取值 | 保留 / 缺失（旧有新无）/ 新增（旧无新有）/ 改名或语义变化 |

> **口径提醒**：旧版 `RepositorySettingJob.js`（CI 任务页）本身就是**死代码**——无路由、无 API 封装、无常量、渲染即崩。新版没有它**不算回归**。

---

## 2. 设置族

设置族 = 个人设置（4 页）+ 仓库组设置（3 页）。

### 2.1 个人设置 · 资料 `/settings/profile`

OLD `components/view/UserSettingGeneral.js` → NEW `pages/settings/Profile.tsx`

| 字段/能力 | 旧版(位置) | 新版(位置) | 结论 |
|---|---|---|---|
| 用户名 name | UserSettingGeneral.js:426 | Profile.tsx:182 | 保留 |
| 邮箱 email | UserSettingGeneral.js:444 | Profile.tsx:185 | 保留 |
| 团队 team | UserSettingGeneral.js:462 | Profile.tsx:188 | 保留 |
| 角色 role | UserSettingGeneral.js:480（**可编辑**） | Profile.tsx:191（**只读 disabled**，且不提交） | 改名或语义变化 |
| 头像上传 | UserSettingGeneral.js:504 | Profile.tsx:175-179 | 保留（改为隐藏 file input + 按钮） |
| 旧口令 / 新口令 / 确认新口令 | UserSettingGeneral.js:527,547,565 | Profile.tsx:206,209,212 | 保留 |
| MFA 二维码图片 | UserSettingGeneral.js:603（`<img src={mfaQRCodeData}>`，150×150） | — | **缺失**（后端仍返回 `image`，见 user_profile.go:317） |
| MFA 引导文案 `message.mfaGuide` | UserSettingGeneral.js:598 | — | 缺失 |
| MFA secret 明文 | UserSettingGeneral.js:340-356（仅存 state，不展示） | Profile.tsx:240（`help` 明文展示） | 新增（但替代不了二维码） |
| MFA 验证码 1 / 2 | UserSettingGeneral.js:606,617 | Profile.tsx:244,247 | 保留 |
| MFA 状态标识（已启用/未启用） | —（旧版只显示按钮） | Profile.tsx:227（Tag） | 新增 |
| 「更换 MFA 设备」按钮 | UserSettingGeneral.js:592 | — | 缺失（新版仅"移除"） |
| 「移除 MFA 设备」 | UserSettingGeneral.js:594 | Profile.tsx:229 | 保留（加了二次确认） |
| 校验：name 2–50 字 | UserSettingGeneral.js:110 | Profile.tsx:69（仅非空） | 改名或语义变化 |
| 校验：email 格式 | UserSettingGeneral.js:126 | — | 缺失 |

**要点**：① **MFA 二维码彻底消失**——后端 `user/getMFAData` 仍返回 `image`（SVG data URI），新版 `MFAData` 类型（types.ts:245）只声明了 `secret`/`status`，用户只能手抄 secret（这在 `FRONTEND_REWRITE_PLAN.md:553` 登记为"下一轮补"，至今未补）。
② `role` 从可编辑变只读且不提交——**后端 `updateBasicInfo` 仍支持 role 字段**（user_profile.go:167），这是前端主动收窄。
③ 旧版 `SetUserInfo`（改单个字段）把 `company`/`job` 也作为可编辑项（UserInfo.js:57,59），新版 `/userInfo` 整页消失（见 §2.5）。

### 2.2 个人设置 · 邮箱 `/settings/email`

OLD `components/view/UserSettingEmail.js` → NEW `pages/settings/Email.tsx`

| 字段/能力 | 旧版(位置) | 新版(位置) | 结论 |
|---|---|---|---|
| 主邮箱展示 | UserSettingEmail.js:455 | Email.tsx:163 | 保留 |
| 主邮箱更换（email+password 发码 → 验证码 → 提交） | SetEmail.js:146,185 | Email.tsx:62,78 | 保留（旧版在 `/userInfo` 弹层内，新版内联到本页） |
| 提交邮箱列表 | UserSettingEmail.js:336-365 | Email.tsx:197-233 | 保留 |
| 提交邮箱状态（主邮箱/已验证/待验证） | UserSettingEmail.js:344-349 | Email.tsx:201（Tag 已验证/未验证） | 保留 |
| 新增提交邮箱 | UserSettingEmail.js:205 | Email.tsx:98 | 保留 |
| 重发验证码 | UserSettingEmail.js:233 | Email.tsx:111 | 保留 |
| 验证提交邮箱 | UserSettingEmail.js:275 | Email.tsx:128 | 保留 |
| 删除提交邮箱 | UserSettingEmail.js:313 | Email.tsx:149 | 保留 |
| 重发倒计时（120s） | UserSettingEmail.js:246-265 | — | 缺失 |
| 文档外链 `ShowHelper /common/multiple_email.md` | UserSettingEmail.js:467 | — | 缺失 |
| 邮箱格式校验 | UserSettingEmail.js:124 | Email.tsx:56（仅非空） | 改名或语义变化 |

### 2.3 个人设置 · SSH Key `/settings/sshkey`

OLD `components/view/UserSettingSSHKey.js` → NEW `pages/settings/SSHKey.tsx`

| 字段/能力 | 旧版(位置) | 新版(位置) | 结论 |
|---|---|---|---|
| 公钥名称 name | UserSettingSSHKey.js:211 | SSHKey.tsx:122 | 保留 |
| 公钥正文 key | UserSettingSSHKey.js:187 | SSHKey.tsx:126 | 保留 |
| 列表：名称 | SSHKeyItem.js:41 | SSHKey.tsx:95 | 保留 |
| 列表：**指纹 hash** | SSHKeyItem.js:44（`aa:bb:cc…`） | — | **缺失**（后端 `SSHKey.hash` 仍在，types.ts:238） |
| 列表：类型 type | — | SSHKey.tsx:96 | 新增 |
| 列表：创建时间 | — | SSHKey.tsx:98 | 新增 |
| 从 key 注释自动取名 | UserSettingSSHKey.js:200 | — | 缺失（改为手填 name） |
| 允许带注释的公钥（`ssh-rsa AAAA user@host`） | UserSettingSSHKey.js:69（正则接受，提交前剥注释） | SSHKey.tsx:52（拒绝任何空白） | **语义反转** |
| 删除公钥 | UserSettingSSHKey.js:143 | SSHKey.tsx:81 | 保留 |
| 文档外链 `ShowHelper /common/ssh_key.md` | UserSettingSSHKey.js:183 | — | 缺失 |
| name 5–30 字校验 | UserSettingSSHKey.js:85 | — | 缺失 |

**要点**：新版对公钥**空白一律拒绝**，而 `cat ~/.ssh/id_rsa.pub` 的标准输出**带尾随注释**，直接粘贴会被前端拦下。旧版会剥掉注释再提交。注意**后端同样拒绝含空格的 key**（user_profile.go:55），所以正确修法是前端剥注释，而不是让用户手改。

### 2.4 个人设置 · 通知 `/settings/notification`

OLD `components/view/UserSettingNotification.js` → NEW `pages/settings/Notification.tsx`

| 字段/能力 | 旧版(位置) | 新版(位置) | 结论 |
|---|---|---|---|
| 合并请求通知开关 | UserSettingNotification.js:368-376 | Notification.tsx:80 | 保留 |
| 邮件通知开关 | UserSettingNotification.js:380-388 | Notification.tsx:84 | 保留 |
| **全部仓库组 / 仓库的逐项通知开关** | UserSettingNotification.js:340-353, 395-407 | — | **缺失** |
| **关闭某组/仓库通知的操作** | UserSettingNotification.js:299（`open` 双向） | Notification.tsx:58（`open` 恒为 `true`） | **缺失**（只能"恢复"，无法"关闭"） |
| 组/仓库名 + 类型 | UserSettingNotification.js:322 | Notification.tsx:98-99 | 保留（但只列**已拒收**项） |
| 角色徽标（InlineMarker） | UserSettingNotification.js:323 | — | 缺失 |
| 只读成员禁用开关（role < DEVELOPER） | UserSettingNotification.js:319,325 | — | 缺失 |
| 组展开/折叠（显示其下仓库） | UserSettingNotification.js:334,404 | — | 缺失 |
| "不属于任何组"的独立仓库表 | UserSettingNotification.js:407 | — | 缺失 |
| 拒收列表（可恢复） | UserSettingNotification.js:239 | Notification.tsx:96-106 | 保留 |

**要点**：旧版是**全量矩阵**（每个组/仓库一个开关 + 层级 + 角色徽标），新版退化成**只显示已拒收项 + 只能恢复**。后端 `setGroupOrRepoNotification` 仍支持 `open=false`，接口是通的。

### 2.5 个人设置 · 用户信息弹层 `/userInfo`（旧版独立页）

OLD `components/view/UserInfo.js` → NEW **无此路由**

| 字段/能力 | 旧版(位置) | 新版(位置) | 结论 |
|---|---|---|---|
| 改用户名（`user/update` field=name） | UserInfo.js:51 | — | 缺失 |
| 改主邮箱（弹层内） | UserInfo.js:53 | — | 缺失（新版在 /settings/email） |
| 改公司 company | UserInfo.js:57 | — | 缺失（后端已无 `u_company` 列，**不可补**） |
| 改职位 job | UserInfo.js:59 | — | 缺失（后端已无 `u_job` 列，**不可补**） |
| 电话 tel / 地区 location | —（旧版 API 层有 `user/update` 白名单） | — | 缺失（后端 `updateProfileField` 仍支持 name/tel/location，user_profile.go:200-208） |

**要点**：新版 `api/user.ts:21` 封装了 `updateProfileField`（`user/update`），但**全仓库无任何页面调用**——是死封装。`tel`/`location` 两个后端仍支持的字段**完全没有 UI**。

### 2.6 仓库组设置

| 页面 | 旧版 | 新版 | 关键差异 |
|---|---|---|---|
| 常规 general | GroupSettingGeneral.js | pages/group/SettingsGeneral.tsx | 名称/描述/头像保留；**缺** 名称 5–30 字 + 字符集校验、描述 200 字计数与上限、"取消"按钮、slug 自动生成 |
| 成员 members | GroupSettingMembers.js | pages/group/SettingsMembers.tsx + MembersManager.tsx | 邀请/改角色/移除/转移所有者全保留（共用 `MembersManager`）；**缺** 邮箱格式校验、0x0407/0x0408 错误码文案、`creatorFlag`/`deleteFlag` 过滤 |
| 高级 advanced | GroupSettingAdvanced.js | pages/group/SettingsAdvanced.tsx + AdvancedSettings.tsx | 改名 + 删除保留；**缺** "更换所有者"下拉（已挪到成员页）、slug 的 `host/组/` 前缀装饰、删除的**两级**确认（新版只有一级） |

### 2.7 设置族小结

| 判定 | 条目 |
|---|---|
| **缺失** | MFA 二维码、通知的组/仓库级开关与"关闭"操作、SSH 指纹展示、公钥注释容忍、`/userInfo` 全页、`tel`/`location` 字段、旧版重发倒计时、各类文档外链 |
| **语义变化** | `role` 变只读；SSH 公钥空白校验反转；保护角色集合语义被压扁（见 §3） |
| **新增** | MFA 状态标识、SSH 列表的类型/创建时间列、组设置的三列展示 |

---

## 3. 仓库页

### 3.1 仓库列表 `/repositories`

OLD `components/view/RepositoryList.js` + `unit/RepositoryListItem.js` → NEW `pages/RepositoryList.tsx` + `components/view/unit/RepositoryRow.tsx`

| 字段/能力 | 旧版(位置) | 新版(位置) | 结论 |
|---|---|---|---|
| 图标/首字母头像 | RepositoryListItem.js:76 | RepositoryRow.tsx:36 | 保留 |
| 组名/仓库显示名 | RepositoryListItem.js:83 | RepositoryRow.tsx:44 | 保留 |
| 我的角色徽标 | RepositoryListItem.js:87 | RepositoryRow.tsx:48 | 保留 |
| 描述 | RepositoryListItem.js:90 | RepositoryRow.tsx:54 | 保留 |
| fork 数 / 未合并 MR 数 / 成员数 / 更新时间 | RepositoryListItem.js:99,106,112,119 | RepositoryRow.tsx:60,65,69,74 | 保留 |
| 页签：全部 / 我参与的 / 我创建的 | RepositoryList.js:35,36 | RepositoryList.tsx:65,66 | 保留（口径一致） |
| REPO_READ 过滤 / updated 倒序 | RepositoryList.js:33,32 | RepositoryList.tsx:40,41 | 保留 |
| 组上下文过滤 | RepositoryList.js:45 | RepositoryList.tsx:34 | 保留 |
| 空态插画 + 新建按钮 | RepositoryList.js:95 | RepositoryList.tsx:81 | 保留 |
| **fork 列表上下文（首 tab 变"fork 自 X"）** | RepositoryList.js:48 | — | 缺失（`/repositories/forklist/:id` 路由整体消失） |
| 顶部错误提示条 | — | RepositoryList.tsx:73 | 新增 |

### 3.2 新建仓库 `/repositories/new` — **新版无此页面/路由**

OLD `components/view/NewRepository.js` → NEW 无

| 字段 | 旧版(位置) | 新版 | 结论 |
|---|---|---|---|
| 仓库名称 displayName | NewRepository.js:263 | — | 缺失 |
| 仓库 URL（组 + slug） | NewRepository.js:282 | — | 缺失 |
| 组下拉 | NewRepository.js:284 | — | 缺失 |
| slug 输入 + 自动 makeSlug | NewRepository.js:305 | — | 缺失 |
| 描述（200 字上限 + 计数） | NewRepository.js:322,339 | — | 缺失 |
| 30/200 字校验、0x0401/0x0402 映射 | NewRepository.js:59,125 | — | 缺失 |
| "未创建个人组"引导 | NewRepository.js:155 | — | 缺失 |

**后端 `repository/create` 端点可用**（repository_write.go:182）。**新版列表页的"新建仓库"按钮指向不存在的路由**（RepositoryList.tsx:53）→ 点按钮即 404。

### 3.3 Fork 仓库 — **新版无任何入口**

OLD `components/view/NewRepositoryFork.js` → NEW 无

来源提示 / 名称·组·slug·描述预填 / forkID 提交 / fork 列表页 —— **全部缺失**。后端 `repository/fork` 可用（repository_write.go:183）。新版全站 grep `fork` 只命中 forkCount 展示、webhook 事件名、活动文案。

### 3.4 仓库设置 · 基本信息

| 字段/能力 | 旧版(位置) | 新版(位置) | 结论 |
|---|---|---|---|
| 头像上传 | RepositorySettingGeneral.js:219 | SettingsGeneral.tsx:85 | 保留 |
| 仓库名 / 描述 | RepositorySettingGeneral.js:234,253 | SettingsGeneral.tsx:96,99 | 保留 |
| 描述字数计数 n/200 | RepositorySettingGeneral.js:267 | — | 缺失 |
| 名称 30 字 / 描述 200 字校验 | RepositorySettingGeneral.js:56 | — | 缺失（仅非空） |
| 0x0402 重复名映射 | RepositorySettingGeneral.js:91 | — | 缺失 |
| "取消"回退 | RepositorySettingGeneral.js:278 | — | 缺失 |

### 3.5 仓库设置 · 成员

| 字段/能力 | 旧版(位置) | 新版(位置) | 结论 |
|---|---|---|---|
| 邀请成员（邮箱输入 + 按钮） | RepositorySettingMembers.js:207,223 | MembersManager.tsx:60,61 | 保留 |
| 头像 / 姓名 / 邮箱 / 角色徽标 | MemberItem.js:66,69,76 | MembersManager.tsx:70,72,73,76 | 保留 |
| 角色下拉 | MemberItem.js:82（**含 OWNER**） | MembersManager.tsx:36（**不含 OWNER**） | 语义变化（OWNER 改为只读徽标 + "转移所有权"） |
| **仓库成员 / 组成员 分两表** | MemberList.js:77 | — | 缺失（后端 `groupMember` 字段仍在） |
| **过滤已删除成员 deleteFlag** | MemberList.js:78 | — | 缺失 |
| **自身权限粗粒度判定**（role < MAINTAINER 整体禁用） | MemberList.js:96 | — | 缺失（新版统一用 `REPO_CHANGE_MEMBER`） |
| 移除成员 | MemberItem.js:88 | MembersManager.tsx:91 | 保留 |
| 0x0407 / 0x0408 错误码文案 | RepositorySettingMembers.js:70 | — | 缺失 |

### 3.6 仓库设置 · 分支（默认分支 + 保护分支）

| 字段/能力 | 旧版(位置) | 新版(位置) | 结论 |
|---|---|---|---|
| 默认分支下拉 | RepositorySettingBranch.js:473 | SettingsBranch.tsx:146 | 保留 |
| **"允许 Push 角色"= 角色集合语义** | RepositorySettingBranch.js:210（`Owner + Maintainer`） | SettingsBranch.tsx:166（**单角色**） | **语义被压扁** |
| **"允许 Merge 角色"= 角色集合语义** | RepositorySettingBranch.js:210 | SettingsBranch.tsx:167（**单角色**） | **语义被压扁** |
| Merge 角色可选集合 | 旧版仅 NOBODY/OWNER/MAINTAINER | SettingsBranch.tsx:75-80 **含 DEVELOPER** | **缺陷**（后端白名单只有 3 项） |
| 保护规则名 / 正则校验 / 重复校验 | RepositorySettingBranch.js:216,113,364 | SettingsBranch.tsx:207（仅非空） | 缺失 |
| 规则列表 / 新建 / 编辑 / 删除 | RepositorySettingBranch.js:196,508,331,196 | SettingsBranch.tsx:163,159,190,168 | 保留（编辑从行内改弹窗） |
| 空态插图 | RepositorySettingBranch.js:522 | SettingsBranch.tsx:186（通用空态） | 缺失 |

### 3.7 仓库设置 · 高级

| 字段/能力 | 旧版(位置) | 新版(位置) | 结论 |
|---|---|---|---|
| slug 输入 | RepositorySettingAdvanced.js:314 | AdvancedSettings.tsx:72 | 保留 |
| slug 校验 / 0x0401 映射 | RepositorySettingAdvanced.js:73,99 | — | 缺失 |
| **更新创建者 owner 下拉** | RepositorySettingAdvanced.js:292 | — | 缺失（已挪到成员页行内） |
| 删除仓库 + 两段提示 | RepositorySettingAdvanced.js:360,352 | AdvancedSettings.tsx:87,86 | 保留（提示合并为一句） |
| 删除两级确认 | RepositorySettingAdvanced.js:128,141 | AdvancedSettings.tsx:55（一级） | 语义变化 |

### 3.8 仓库设置 · Webhook

| 字段/能力 | 旧版(位置) | 新版(位置) | 结论 |
|---|---|---|---|
| URL 输入 | RepositorySettingWebhook.js:442 | SettingsWebhook.tsx:215 | 保留 |
| URL 校验（非空/http(s)/≤255） | RepositorySettingWebhook.js:101 | — | 缺失 |
| **Secret（签名密钥）输入** | RepositorySettingWebhook.js:466 | — | **缺失 → 功能不可用**（见 §6） |
| Content-Type 展示 | RepositorySettingWebhook.js:456 | — | 缺失 |
| 触发方式单选（推送/自定义） | RepositorySettingWebhook.js:480 | — | 缺失 |
| 事件清单勾选 | RepositorySettingWebhook.js:493 | SettingsWebhook.tsx:223 | 保留（25 事件逐项对应） |
| 启用/停用 | RepositorySettingWebhook.js:516 | SettingsWebhook.tsx:237 | 保留 |
| 列表：创建人 / 更新时间 | RepositorySettingWebhook.js:294,335 | — | 缺失（后端字段在 types.ts:255,261） |
| 列表：URL / Secret / 事件摘要 | RepositorySettingWebhook.js:296,305,316 | SettingsWebhook.tsx:163,164 | 保留（Secret 列消失；摘要 4 个改 2 个 +N） |
| URL / Secret / 投递 ID 复制 | RepositorySettingWebhook.js:300,310 | — | 缺失 |
| 投递日志（成功失败/投递 ID/时间/详情） | WebhookLog.js:111 | SettingsWebhook.tsx:260 | 保留（改弹窗） |
| 日志详情：请求/响应页签、HTTP 头、耗时、payload/body 分栏 | WebhookLog.js:127,139,132,156 | SettingsWebhook.tsx:284（平铺两段） | 缺失 |

### 3.9 分支列表 / 标签列表

| 字段/能力 | 旧版(位置) | 新版(位置) | 结论 |
|---|---|---|---|
| 分支名 / 短 SHA / 最近提交标题 / 更新时间 | RefList.js:161,169,170,175 | BranchList.tsx:79,96,89,94 | 保留（短 SHA 不再是链接） |
| 分支描述 detail | RefList.js:162 | — | 缺失 |
| 保护标记 | RefList.js:163 | BranchList.tsx:82 | 保留 |
| **默认分支标记** | RefList.js:164 | — | 缺失 |
| **已合并标记 isMerge** | RefList.js:165 | — | 缺失（后端仍返回，types.ts:157） |
| **更新者姓名** | RefList.js:172 | — | 缺失 |
| **删除分支** | RefList.js:187 | — | 缺失（后端 deleteBranch 可用） |
| **新建分支** | RefFilter.js:68 | — | 缺失（后端 createBranch 可用） |
| **关键字搜索 / 排序 / 活跃页签 / 每页 10"更多"** | RefFilter.js:57,67,55 / RefList.js:210 | — | **全部缺失** |
| 标签名 / 说明 / 短 SHA / 标题 / 时间 | RefList.js:161,162,169,170,175 | TagList.tsx:77,81,85,81,83 | 保留 |
| **新建 / 删除标签、搜索、排序、分页** | RefFilter.js:68 / RefList.js:196,210 | — | 缺失（后端 createTag/deleteTag 可用） |

### 3.10 仓库动态 Activity

| 字段/能力 | 旧版(位置) | 新版(位置) | 结论 |
|---|---|---|---|
| 操作者头像 / 名称 / 动作 / 细节 / 时间 | ActivityItem.js:55,58,59,65 | ActivityFeed.tsx:136,139,140,143 | 保留 |
| **分类页签（全部/提交/MR/成员）** | ActivityList.js:135 | — | 缺失（后端 category 支持，api/repository.ts:378 恒传 0） |
| **关键字搜索** | ActivityList.js:148 | — | 缺失（后端 keyword 支持） |
| **翻页（每页 20）** | ActivityList.js:181 | — | 缺失（新版只显示前 20 条） |

### 3.11 提交列表 / 提交详情

| 字段/能力 | 旧版(位置) | 新版(位置) | 结论 |
|---|---|---|---|
| 标题 / 作者头像 / 作者名 / 时间 / 短哈希 | CommitItem.js:75,88,95,111,119 | CommitRow.tsx:34,38,39,42,47 | 保留 |
| 分支/标签下拉 | CommitList.js:233 | CommitList.tsx:101 | 保留 |
| 关键字搜索 | CommitList.js:246 | CommitList.tsx:113 | 保留（触发方式：回车 → 输入即搜） |
| **按日期分组（yyyy-MM-dd + 当日计数）** | CommitList.js:126,268 | — | 缺失 |
| **子目录提交路径 `commits/:rev/*`** | CommitList.js:243 | — | 缺失（路由也无） |
| 每页条数 | CommitList.js:53（20） | CommitList.tsx:21（10） | 语义变化 |
| "更多"滚动 → 页码分页 | CommitList.js:307 | CommitList.tsx:152 | 语义变化（末页靠"返回<10"推断，不精确） |
| 父提交展开列表 | CommitDetail.js:200 | CommitDetail.tsx:114（一行文本） | 缺失 |
| "浏览文件"跳转 | CommitDetail.js:190 | — | 缺失 |
| SHA 复制 / "无文件变更"提示 | — | CommitDetail.tsx:112,128 | 新增 |

### 3.12 文件浏览 FileTree / 仓库头 Dashboard

| 字段/能力 | 旧版(位置) | 新版(位置) | 结论 |
|---|---|---|---|
| 仓库头：组名/显示名/描述 | RepositoryDashboard.js:111,123 | RepositoryLayout.tsx:30,33 | 保留 |
| 仓库头：**Fork 按钮 + fork 数** | RepositoryDashboard.js:132 | — | 缺失 |
| 仓库头：**Clone 地址（HTTPS/SSH + 复制）** | RepositoryDashboard.js:152 | — | 缺失 |
| 仓库头：**计数条（提交/分支/标签/体积/未合并 MR/成员）** | RepositoryDashboard.js:256 | — | 缺失（侧栏只有 commit/branch/tag/member；**体积与 MR 计数彻底没有**） |
| 仓库头：fork 来源链接 | RepositoryDashboard.js:113 | — | 缺失 |
| 文件列表：名称/最近提交标题/更新时间 | FileTree.js:216,247,254 | FileTree.tsx:165,166,168 | 保留 |
| **submodule 条目** | FileTree.js:239 | — | 缺失 |
| **文件名过滤** | FileTree.js:362 | — | 缺失 |
| **当前目录最后一条提交行** | FileTree.js:344 | — | 缺失 |
| **README 自动渲染** | FileTree.js:368 | — | 缺失 |
| **空仓库初始化引导（init/push + 复制）** | RepositoryEmpty.js:49 | — | 缺失 |
| **blame 模式** | FileTree.js:163 | — | 缺失（后端 blameInfo 可用） |
| 文件内容：复制路径 / 复制代码 / Markdown 渲染切换 / 历史入口 | ObjectViewer.js:185,192,207 | — | 缺失 |
| 行号 + Prism 高亮 | CodeViewer | CodeViewer.tsx:40 | 保留 |

### 3.13 仓库页小结

| 判定 | 条目 |
|---|---|
| **整页缺失** | 新建仓库、Fork 仓库、创建分支、创建标签（4 个后端端点全部可用） |
| **入口消失** | 删除分支、删除标签、分支/标签搜索、排序、活跃页签、分页 |
| **公共件瘦身** | 仓库头 Dashboard 丢 fork/clone/6 项计数；文件页丢搜索/README/submodule/空仓库引导/blame/复制 |
| **语义压扁** | 保护分支的角色集合语义 → 单角色；默认分支改"切换即提交"（旧版需显式点按钮） |
| **缺陷** | Webhook secret 缺失、保护分支 merge 角色越界 |

---

## 4. 管理后台

### 4.1 仪表盘 `/admin/dashboard`

| 字段/能力 | 旧版(位置) | 新版(位置) | 结论 |
|---|---|---|---|
| 用户/组/仓库总数 | Dashboard.js:68,71,74 | Dashboard.tsx:55,63,59 | 保留 |
| **统计卡可点击跳转** | Dashboard.js:68（NumbericDashboard.js:80） | Dashboard.tsx:53-64（无 onClick） | 缺失 |
| **卡片图标** | Dashboard.js:68 | — | 缺失 |
| **阈值配色（≥85红/≥70黄）** | NumbericDashboard.js:47-58 | Dashboard.tsx:72-84（纯文本） | 缺失 |
| **数值格式化（千分位/单位）** | NumbericDashboard.js:68-74 | Dashboard.tsx:73（手拼 %） | 缺失 |
| CPU / 内存 / 磁盘明细 | Dashboard.js:80,83,88-91 | Dashboard.tsx:72,76,79-84 | 保留 |
| "磁盘使用量"独立分组标题 | Dashboard.js:85-87 | — | 缺失 |
| 服务状态（codefever/nginx/sendmail + Running/Stopped） | Dashboard.js:97,100,105 | Dashboard.tsx:91-95 | 保留 |

> PHP-FPM / Crond 两张服务卡旧版已注释下线、后端也不再返回——**不是重构缺失**。

### 4.2 用户管理 `/admin/users`

| 字段/能力 | 旧版(位置) | 新版(位置) | 结论 |
|---|---|---|---|
| 用户名 / 邮箱 / 可见仓库数 / 创建时间 / 管理员 / 禁用状态 | Users.js:207,208,211,212,215,216 | Users.tsx:87,88,89,90,91,96 | 保留 |
| **头像** | Users.js:205 | — | 缺失（types.ts:355 有字段未渲染） |
| **2FA 已开启徽标** | Users.js:214 | — | 缺失 |
| **分类页签（全部/已开MFA/未开MFA/禁用）** | Users.js:392-401 | — | 缺失（后端 `category=0/1/2/3` 全支持，api/admin.ts:27 **硬编码 0**） |
| **排序下拉（name/email/created）** | Users.js:415-425 | — | 缺失（api/admin.ts:29 传空串） |
| 关键词搜索 | Users.js:408 | Users.tsx:79 | 保留（无 200ms 防抖，旧版有） |
| 分页 | Users.js:79,435 | Users.tsx:139 | 保留 |
| 启用/禁用、关闭 MFA、设/取消管理员 | Users.js:453,462,467 | Users.tsx:108,114,111 | 保留 |
| **设管理员需校验本人密码 `checkPassword`** | Users.js:331,514-541 | — | 缺失（后端仍在 admin.go:349）→ **权限模型降级** |
| **添加用户（name + email）** | Users.js:366,404 | — | 缺失（api/admin.ts:83 已封装 `addAdminUser`，无引用） |
| **新用户初始口令回显 + 复制** | Users.js:584-612 | — | 缺失 |
| **危险操作二次确认** | Users.js:241-249,279-284 | 仅启用/禁用有 | 缺失 |
| 重置口令 | Users.js:457,473-512 | Users.tsx:119,141（单输入框） | 语义变化（去掉二次确认） |

### 4.3 仓库组管理 `/admin/groups`

| 字段/能力 | 旧版(位置) | 新版(位置) | 结论 |
|---|---|---|---|
| 组显示名 | Groups.js:171 | Groups.tsx:58 | 保留 |
| **图标/首字母头像** | Groups.js:166-170 | — | 缺失 |
| **创建时间** | Groups.js:173 | — | 缺失 |
| **行操作（设置菜单）** | Groups.js:175 | — | 缺失 |
| **排序（name/created）** | Groups.js:353-362 | — | 缺失（api/admin.ts:38 传空串） |
| 关键词搜索 / 分页 | Groups.js:346,371 | Groups.tsx:50,69 | 保留 |
| **成员管理子系统（读+邀请+移除+转所有者）** | Groups.js:189-337 | — | **缺失**（写接口已封装但无引用；**读接口 `groupMembers` 连封装都没有**） |
| 成员表：头像/姓名/邮箱/角色徽标/创建者徽标/加入时间/已删除徽标 | Groups.js:217-230 | — | 缺失 |
| 组标识 name / 描述 / 所属者 | — | Groups.tsx:59,60,61 | 新增 |

### 4.4 仓库管理 `/admin/repositories`

| 字段/能力 | 旧版(位置) | 新版(位置) | 结论 |
|---|---|---|---|
| 仓库显示名 | Repositories.js:171 | Repositories.tsx:57 | 保留 |
| 所属组 | Repositories.js:171（同格） | Repositories.tsx:59（独立列） | 保留（列拆分） |
| **图标/首字母头像** | Repositories.js:167-170 | — | 缺失 |
| **创建时间** | Repositories.js:173 | — | 缺失 |
| **行操作（设置菜单）** | Repositories.js:175 | — | 缺失 |
| **排序（name/created）** | Repositories.js:316-325 | — | 缺失（api/admin.ts:48 传空串） |
| 关键词搜索 / 分页 | Repositories.js:309,334 | Repositories.tsx:49,68 | 保留 |
| **成员管理子系统（读+邀请+移除）** | Repositories.js:189-300 | — | 缺失（读接口未封装，写接口未封装） |
| 成员表：创建者徽标 / 组成员徽标 / 角色 / 加入时间 / 已删除 | Repositories.js:217-231 | — | 缺失 |
| 仓库标识 name / forkCount | — | Repositories.tsx:58,60 | 新增 |

> 旧版的 `creatorFlag` / `groupMember` 徽标是**唯一能区分「组继承权限 vs 仓库直授」**的 UI，新版无对应物。

### 4.5 系统设置 `/admin/settings`

| 字段/能力 | 旧版(位置) | 新版(位置) | 结论 |
|---|---|---|---|
| 站点 URL 前缀 host | Settings.js:182 | Settings.tsx:95-100（**裸 JSON 文本框**） | 语义变化 |
| SSH 前缀 ssh | Settings.js:196 | 同上 | 语义变化 |
| SMTP 发件人名称 email.name | Settings.js:218 | 同上 | 语义变化 |
| SMTP 发件人地址 email.from | Settings.js:232 | 同上 | 语义变化 |
| 开放注册开关 allowRegister | Settings.js:254 | 同上 | 语义变化 |
| 分组标题（主机/SMTP/注册） | Settings.js:174,210,246 | — | 缺失 |
| **字段级校验（URL 格式/SSH user@host/邮箱格式/必填，7 条正则）** | Settings.js:45-94 | 仅 `JSON.parse` | 缺失 |
| 保存 / 二次确认 / 重置按钮 | Settings.js:266 | Settings.tsx:103,66,106 | 保留 + 新增 2 项 |

**要点**：页面从**结构化表单退化为原始 JSON 编辑器**（`Settings.tsx:20-22` 已登记为有意偏离）。后端 `domain/admin.go:765-789` 的 `ValidateSiteDocument` 只校验 ssh"是不是字符串"、**不校验格式**——旧版能在前端拦下的错误，新版会写进 `config.yaml`。

### 4.6 管理后台小结

| 判定 | 条目 |
|---|---|
| **缺失** | 用户分类页签、三页排序、组成员管理、仓库成员管理、添加用户 + 初始口令回显、设管理员的密码校验、危险操作二次确认、列表头像、系统设置结构化表单、仪表盘跳转/配色 |
| **新增** | Groups 的 name/description/owner 三列、Repositories 的 name/forkCount 两列、Users 的姓名/邮箱拆列与独立状态列、分页总数、表格内置 loading/空态、保存前确认与重置按钮 |
| **权限** | UAC 矩阵逐字对齐无回退（config/uac.ts:9-80 ↔ config/UAC.js:1-51）；后台入口可见性保留（Header.tsx:119）。**但后台页族几乎没用 UAC**——旧版还做了角色→文案映射，新版连这个也没了 |

> 旧版后台**本来就没有**批量操作，grep `Checkbox|batch|selected` 全目录无命中——不存在批量操作缺失。

---

## 5. MR（合并请求）

### 5.1 MR 列表（仓库内）

OLD `components/view/MergeRequest.js` + `unit/MergeRequestItem.js` → NEW `pages/repository/MergeRequestList.tsx` + `MergeRequestRow.tsx`

| 字段/能力 | 旧版(位置) | 新版(位置) | 结论 |
|---|---|---|---|
| 状态 Tab：待合并/已合并/已关闭 | MergeRequest.js:178-180 | MergeRequestList.tsx:66-68 | 保留（"已打开"→"待合并"） |
| **状态 Tab："全部"** | MergeRequest.js:177 | — | 缺失（默认视图由"全部"变"只看待合并"） |
| **Tab 数量角标** | MergeRequest.js:191-226 | — | 缺失 |
| 关键字搜索 | MergeRequest.js:240-253 | MergeRequestList.tsx:72 | 语义变化（回车 → 即时，无防抖） |
| **排序 asc/desc** | MergeRequest.js:255 | — | 缺失（api 层支持，页面不传） |
| **分页（每页 10 + "查看更多"）** | MergeRequest.js:272-290 | — | 缺失（api 默认 pagesize=20，最多 20 条且无法翻页） |
| "创建合并请求"按钮 | MergeRequest.js:256 | MergeRequestList.tsx:73 | 保留 |
| 行：编号 + 标题 | MergeRequestItem.js:68 | MergeRequestRow.tsx:33 | 保留 |
| 行：状态徽标 | MergeRequestItem.js:102 | MergeRequestRow.tsx:41 | 保留 |
| 行：**目标仓库名** | MergeRequestItem.js:107（仓库名） | MergeRequestRow.tsx:39（**主键前 8 位十六进制**） | **语义变化** |
| 行：源分支 → 目标分支 | MergeRequestItem.js:110（可点跳 commits） | MergeRequestRow.tsx:38（纯文本） | 缺失（链接） |
| 行：最后提交人 updater | MergeRequestItem.js:82-90,116 | — | 缺失 |
| 行："编辑于" commit.time | MergeRequestItem.js:117 | — | 缺失 |
| 行：更新时间 update | MergeRequestItem.js:130 | MergeRequestRow.tsx:43 | 保留 |
| 行：评审人头像组 + 已/未评审角标 | MergeRequestItem.js:124 + MergeRequestReviewers.js:58 | — | 缺失 |

### 5.2 MR 详情

OLD `components/view/MergeRequestDetail.js` → NEW `pages/repository/MergeRequestDetail.tsx`

| 字段/能力 | 旧版(位置) | 新版(位置) | 结论 |
|---|---|---|---|
| 标题 / 编号 / 状态徽标 / 描述 | MergeRequestDetail.js:824,825,829 | MergeRequestDetail.tsx:216,218,235 | 保留（编号为新增） |
| 提交人头像 + 昵称 + 创建时间 | MergeRequestDetail.js:1036 | MergeRequestDetail.tsx:221-225 | 保留（但时间标签 key 写错，见 §6） |
| **处理人（合并者/关闭者）+ 合并/关闭时间** | MergeRequestDetail.js:1043-1049 | — | 缺失 |
| **源→目标分支全路径（组名/仓库名/分支）** | MergeRequestDetail.js:1034 | MergeRequestDetail.tsx:228（仅分支名） | 缺失（仓库名/组名） |
| 关闭 / 评审 / 合并 按钮 | MergeRequestDetail.js:833,836,840 | MergeRequestDetail.tsx:274,268,271 | 保留（"评审"改为所有人可见且与"合并"并存） |
| **合并提交信息输入框** | MergeRequestDetail.js:851-860 | —（恒取 title，MergeRequestDetail.tsx:70） | 缺失 |
| **"使用 Squash"勾选** | MergeRequestDetail.js:862-875 | MergeRequestDetail.tsx:197（**硬编码 true**） | 语义变化（用户无法选择） |
| **合并冲突 4 步 git 指引** | MergeRequestDetail.js:877-916 | — | 缺失 |
| **快进判断** | MergeRequestDetail.js:513 | MergeRequestDetail.tsx:192 | 缺失 |
| 评审人面板（增/删） | MergeRequestDetail.js:971-1011 | MergeRequestDetail.tsx:237-264 | 保留（交互形态变化） |
| **评审状态"已评审/未评审"徽标** | MergeRequestDetail.js:1019 | — | 缺失 |
| 评审人候选取自**目标**仓库成员且 role≥DEVELOPER | MergeRequestDetail.js:788,987 | MergeRequestDetail.tsx:254（取自 sourceRepoInfo，无角色过滤） | 语义变化 |
| **右侧活动栏（请求合并路径 / 创建·合并·关闭时间线）** | MergeRequestDetail.js:1027-1051 | — | 缺失 |
| **详情 Tab：提交动态 / 合并请求动态** | MergeRequestDetail.js:706,962 | — | 缺失（后端 `mergeRequestCommits`、`activities?mergeRequest=` 现成） |
| **版本对比下拉（最近修改/第N次修改）** | MergeRequestDetail.js:479 | — | 缺失（后端 `mergeRequestVersionList` 现成） |
| **diff 汇总条（所有文件 N / +add / -del）** | MergeRequestDiff.js:286-316 | — | 缺失 |
| **diff 布局切换（inline / side-by-side）** | MergeRequestDiff.js:317-322 | — | 缺失（DiffViewer 固定单列） |
| **MR 文件树侧栏 + 锚点跳转** | MergeRequestDiff.js:148,323 | MergeRequestDetail.tsx:231（链到仓库 /files） | 语义变化 |
| **逐文件折叠面板** | FileDiffExpansionPanel.js:170 | DiffViewer.tsx:100（静态） | 缺失 |
| **复制路径 / 展开完整文件 / 查看 @hash / 图片 diff 对照 / mode 变更提示** | ObjectDiffViewer.js:142,169 | — | 缺失 |

### 5.3 新建 MR

OLD `components/view/CreateMergeRequest.js`（第 1 步）+ `MergeRequestDetail.js` 创建态（第 2 步）→ NEW `pages/repository/MergeRequestCreate.tsx`

| 字段/能力 | 旧版(位置) | 新版(位置) | 结论 |
|---|---|---|---|
| **第 1 步：源仓库选择器** | CreateMergeRequest.js:340-372 | — | 缺失（固定当前仓库） |
| **第 1 步：目标仓库选择器（`targetRepository` + forkFlag）** | CreateMergeRequest.js:166-196 | — | 缺失（**跨仓库/fork MR 无法创建**） |
| 源分支 / 目标分支 | CreateMergeRequest.js:375,443 | MergeRequestCreate.tsx:86,93 | 保留 |
| 目标分支默认预选 defaultBranch | — | MergeRequestCreate.tsx:35 | 新增 |
| **源/目标分支末次提交预览** | CreateMergeRequest.js:401,461 | — | 缺失 |
| **"必须选择不同分支"校验** | CreateMergeRequest.js:290,479 | — | 缺失（只判非空） |
| **第 2 步：对比 diff 预览后提交** | MergeRequestDetail.js:929-945 | — | 缺失（两步流程压成一步） |
| 标题 | MergeRequestDetail.js:773（"标题"） | MergeRequestCreate.tsx:97（"名称"） | 语义变化 |
| 描述 | MergeRequestDetail.js:777 | MergeRequestCreate.tsx:100 | 保留 |
| 评审人选择 | MergeRequestDetail.js:785（单选 + role≥DEVELOPER + 目标仓库成员） | MergeRequestCreate.tsx:103（多选、无过滤、当前仓库） | 语义变化 |
| **"从 X 请求合并到 Y"摘要 / "修改分支"按钮** | MergeRequestDetail.js:758,766 | — | 缺失 |
| 取消 / 提交 | MergeRequestDetail.js:807,811 | MergeRequestCreate.tsx:116,119 | 保留 |
| 0x0406 / 0x040F / 0x0405 中文提示 | MergeRequestDetail.js:436-454 | —（client.ts 错误码表没有这三个） | 缺失（回落英文 "Operate Fail" / "New Target Exists" / "Branch Proteced"） |

> `assignee` / 标签 / 里程碑 **旧新皆无，后端也无字段**——不是漏做，是功能本身不存在。

### 5.4 MR 看板（全局 / 组内）

OLD `MergeRequest.js` 全局态 → NEW `pages/MergeRequestBoard.tsx`

| 字段/能力 | 旧版(位置) | 新版(位置) | 结论 |
|---|---|---|---|
| 全局数据源 `relatedMergeRequests` | MergeRequest.js:144 | MergeRequestBoard.tsx:59 | 保留（**但解析形状用错，见 §6**） |
| 组内数据源 `group/mergeRequests` | MergeRequest.js:138 | MergeRequestBoard.tsx:102 | 保留 |
| **relatedRepository（计数与归属）** | MergeRequest.js:159,198 | — | 缺失 |
| 状态 Tab / 搜索 / 排序 / 分页 | MergeRequest.js:177,240,255,272 | MergeRequestBoard.tsx:75,81（仅全局） | 多数缺失 |
| **"创建合并请求"入口** | MergeRequest.js:256 | — | 缺失（全局/组内看板都没有） |

### 5.5 MR 小结

| 判定 | 条目 |
|---|---|
| **缺失** | MR 详情三 Tab 结构、提交动态、MR 活动流、版本对比、diff 布局切换、MR 文件树、逐文件折叠、合并提交信息框、Squash 勾选、冲突指引、右侧活动栏、列表的最后提交人/编辑于/评审人角标、Tab 角标、排序、分页、新建 MR 的两步流程与源/目标仓库选择器 |
| **语义变化** | 评审人候选来源与过滤、目标仓库名显示为主键前缀、评审按钮对所有人可见、Squash 强制开启 |
| **缺陷** | 全局看板解析形状错误、评审人数据结构不符、合并冲突被当成功、错误码无中文 |

---

## 6. 新版自带缺陷（不是"缺旧版 UI"，是新版自己引入的）

| # | 缺陷 | 证据 | 影响 |
|---|---|---|---|
| 1 | **Webhook 新建/编辑必然失败** | 前端请求体无 `secret`（api/repository.ts:203-209）↔ 后端缺 secret 直接 `phpFatalError()`（repository_write.go:1868-1871） | **Webhook 功能整体不可用** |
| 2 | **保护分支"允许合并角色"多给 DEVELOPER** | 下拉共用 4 项含 DEVELOPER（SettingsBranch.tsx:75-80, 224）↔ 后端白名单只有 NOBODY/OWNER/MAINTAINER（domain/repository_write.go:120） | 选 DEVELOPER 必得 0x0201 |
| 3 | **全局 MR 看板取数形状错误 → 白屏** | `fetchRelatedMergeRequests` 把整个 `data` 当数组（api/repository.ts:390-395）↔ 后端返回 `{mergeRequests:[], relatedRepository:[]}`（repository.go:1025-1028） | 有数据时 `items.map` 抛 TypeError |
| 4 | **评审人数据结构不符** | 前端按 `{id,name,email,icon}` 用（types.ts:339）↔ 后端返回 `{id,user,isReview}`（repository.go:956-965） | 姓名空白；删除评审人传的是评审记录主键而非用户主键 → **静默无效** |
| 5 | **合并冲突被当成功** | 后端返回 `{conflict:true}` 且 code=0（repository_write.go:2594）↔ client.ts:171 仅在 code≠0 时抛错 | 冲突时提示"更新成功"并跳列表 |
| 6 | **i18n key 拼错** | `t('label.createAt')`（MergeRequestDetail.tsx:224）↔ 词表只有 `label.createdAt` | 页面直接显示字面量 "label.createAt" |
| 7 | **错误码表缺 0x0405/0x0406/0x040F** | client.ts:34-48 | 关键业务错误回落英文原文 |
| 8 | **Webhook 日志详情直接渲染对象** | 后端 `Request/Response` 是 `any`（repository_write.go:273）↔ 前端声明 `string` 并直接进 `<pre>`（SettingsWebhook.tsx:287） | React 抛 "Objects are not valid as a React child" |
| 9 | **保护分支角色语义被压扁** | 旧版是角色集合 `Owner + Maintainer`，新版单角色 | 用户误判权限范围 |
| 10 | **新建仓库/新建组/全局新建 MR 指向不存在的路由** | Header.tsx:59-61、RepositoryList.tsx:53、GroupList.tsx:43 ↔ router.tsx 无这些路径 | 点按钮即 404 |
| 11 | **仓库内 MR 列表必然报错** | 后端 `repository/mergeRequests` 把 `sort` 当必填（既非 `asc` 也非 `desc` 即 `0x0201`，repository.go:873-875）；前端 `fetchMergeRequests` 缺省传空串（api/repository.ts:287-292） | 列表页与"创建 MR"前的数据都拿不到 |
| 12 | **组内 MR 看板必然 500** | 后端 `group/mergeRequests` 要求 `keyword`/`page`/`pagesize` **显式存在**（缺失即 PHP TypeError → `phpFatalError()`，group.go:348-357），且 `sort` 同样必填；前端 `fetchGroupMergeRequests` 一个都没传（api/group.ts:117-121） | 组内 MR 看板整页报错 |
| 13 | **全局 MR 看板必然报错** | 后端 `repository/relatedMergeRequests` 同样把 `sort` 当必填（repository.go:986-988）；前端缺省传空串 | 全局 MR 看板整页报错 |

> #11–#13 是核对第二轮复查后端 handler 时补记的：它们不在"字段集合"维度，
> 但会让对应页面**完全不可用**，因此与 #1/#2/#3 同级处理。

---

## 7. 补齐优先级建议

> 优先级依据：**后端是否已支持**（支持 ⇒ 纯前端补 UI，成本低）+ **是否阻断主流程**。

| 级别 | 项 | 成本 | 后端就绪 |
|---|---|---|---|
| **P0** | 修 §6 的 10 个缺陷（其中 #1/#2/#3 是功能直接不可用） | 小 | — |
| **P1** | 补「新建仓库」页 + 「新建组」页 + 全局「新建 MR」入口（消除 404 死链） | 中 | ✅ create / group.create |
| **P1** | 补 MR 详情的三 Tab（提交动态 / MR 活动流 / 版本对比）与右侧活动栏 | 中 | ✅ mergeRequestCommits / activities?mergeRequest= / mergeRequestVersionList |
| **P2** | 补 MR 的源/目标仓库选择器 + diff 预览两步流程（fork MR 的前提） | 中 | ✅ targetRepository |
| **P2** | 补分支/标签的「新建 + 删除」与分支列表三项筛选 | 小 | ✅ createBranch/deleteBranch/createTag/deleteTag |
| **P2** | 补仓库头的 fork 按钮 + Clone 地址 + 6 项计数（含仓库体积、未合并 MR） | 小 | ✅ 数据已在 repository/config |
| **P3** | 补后台的用户分类页签 + 三页排序（**改 api/admin.ts 两个参数即恢复**） | 极小 | ✅ category / sort |
| **P3** | 补后台的组/仓库成员管理（后端与 i18n 都已就绪，缺读接口封装 + UI） | 中 | ✅ groupMembers / repositoryMembers |
| **P3** | 补后台的「添加用户 + 初始口令回显」与「设管理员密码校验」（权限相关，建议提前） | 小 | ✅ addUser / checkPassword |
| **P4** | 补通知设置的组/仓库级开关（含关闭方向、角色徽标、层级） | 中 | ✅ setGroupOrRepoNotification(open=false) |
| **P4** | 补 MFA 二维码渲染（后端已返回 image，只差前端一个 `<img>`） | 极小 | ✅ user/getMFAData.image |
| **P4** | 补 SSH 指纹列 + 公钥注释容忍（前端剥注释，与后端"拒绝空格"配合） | 极小 | ✅ SSHKey.hash |
| **P4** | 补系统设置的结构化表单 + 7 条字段校验 | 小 | ✅ admin/config 五字段 |
| **P4** | 补活动流的分类页签 + 关键字搜索 + 翻页 | 小 | ✅ category/keyword/page |
| **P4** | 补 Webhook 的 secret / 创建人 / 更新时间 / 复制能力 + 日志详情分栏 | 小 | ✅ 字段都在 |
| **P5** | 补文件页的搜索 / README 渲染 / submodule / 空仓库引导 / blame / 复制 | 中 | ✅ blameInfo |
| **P5** | 补提交列表的日期分组、提交详情的父提交展开与"浏览文件" | 小 | ✅ |
| **不建议** | 仓库 Job(CI) 设置页 | — | ❌ 旧版即死代码、后端无端点；若要 CI 属**新需求**而非补旧版 |

---

## 8. 附录：交叉核对数据

### 8.1 接口层：旧版调用但新版未调用的 22 个后端端点

```
admin/checkPassword        admin/groupMembers      admin/repositoryAddMember
admin/repositoryMembers    admin/repositoryRemoveMember
group/create               repository/blameInfo     repository/create
repository/createBranch    repository/createTag     repository/deleteBranch
repository/deleteTag       repository/fileContent   repository/fork
repository/lastCommitLog   repository/mergeRequestCommits
repository/mergeRequestVersionList                  repository/refListContainSHA
repository/targetRepository
user/confirmTel  user/getCode  user/getCountriesCode
```

其中 **仅 `user/confirmTel`、`user/getCode` 后端确实没有**（电话验证码链路整体下线）；其余 20 个端点后端全部在册，属于前端漏接。

### 8.2 旧版词条覆盖率

| 命名空间 | 新版未引用的旧版词条数 |
|---|---|
| message | 218 |
| label | 123 |
| phrase | 108 |
| term | 45 |
| menu | 2 |

> 旧版共 553 个带命名空间的词条，新版仅复用 57 个（**10.3%**）；未引用合计 496。典型被丢弃的词条即已丢失的能力：
> `label.changeMFADevice`、`label.createWebhook`、`label.deleteBranch`、`label.deleteTag`、
> `label.forkFrom_S`、`label.groupSlug`、`label.groupURL`、`label.branchRule`、
> `label.allowPush`、`label.allowMerge`、`label.changeMemberRole`、`label.inviteMember`、
> `label.memberLimitation`、`label.updateCreator`、`label.updateRepositoryURL`、
> `label.webhookSecret`、`label.senderName`、`label.senderAddress`、`label.serviceStatus`、
> `label.getPhoneCode`、`label.codeDiff`、`label.compareBranch`、`label.showDiffOnly` …

### 8.3 路由级差异

| 旧版路由 | 新版 |
|---|---|
| `/repositories/new` | **无**（但 Header/列表页按钮仍指向它） |
| `/repositories/fork/:id`、`/repositories/forklist/:id` | **无** |
| `/groups/new`、`/groups/:g/repositories/new` | **无**（按钮仍指向） |
| `/groups/:g/mergerequests/new` | **无** |
| `/mergerequests/new`（全局） | **无**（按钮仍指向） |
| `/:g/:r/branches/new`、`/:g/:r/tags/new` | **无** |
| `/:g/:r/mergerequests/detail/:srcR/:srcB/:tgtR/:tgtB` | **无** |
| `/:g/:r/commits/:rev/*`（子目录） | **无** |
| `/userInfo` | **无** |
| `/:g/:r/settings/job` | 旧版即死代码 |
| — | 新增：`/__design`、`/doc/:lang/*`、`/admin/dashboard`（旧版 `/admin` 即仪表盘） |

---

## 9. 未核实项（诚实记录）

1. 全部分析为**静态读码**，未运行前端或后端验证实际渲染与请求行为；`web/build` 产物与运行时表现未核对。
2. `fileChanges` 的 `original/modified` 方向：旧版为「源→目标」，新版为「目标SHA→源SHA」，后端 `Git.Diff(ctx, repoPath, original, modified)` 哪一侧为"正确"需实跑判定。
3. 新版 `MergeRequestDetail.tsx` 把 `targetObject`/`sourceObject` 声明为非空 `string`，后端类型是可空指针，未在真实数据上验证。
4. 各子页面的**视觉**差异（间距/字号/密度/暗色模式）不在本报告范围——本报告只核字段与能力。

---

## 10. 本轮对齐的处理状态（2026-10-24）

核对结论已全部进入实现：按 §7 的优先级 P0→P5 推进，**已落地**的条目如下。
未落地的条目连同原因登记在 `docs/FRONTEND_REWRITE_PLAN.md` §10 批 8 的偏离表。

| 报告条目 | 状态 | 落点 |
|---|---|---|
| §6 #1 Webhook 缺 secret → 必 500 | ✅ | `api/repository.ts#saveWebhook` + `pages/repository/SettingsWebhook.tsx`（输入框、列表列、复制） |
| §6 #2 保护分支 merge 角色越界 | ✅ | `pages/repository/SettingsBranch.tsx`（merge 只给 NOBODY/OWNER/MAINTAINER） |
| §6 #3 全局 MR 看板取数形状错 → 白屏 | ✅ | `api/repository.ts#fetchRelatedMergeRequests` 返回 `{mergeRequests, relatedRepository}` |
| §6 #4 评审人结构不符 + 删除主键错 | ✅ | `api/types.ts#MergeRequestReviewer` + `MergeRequestDetail.tsx`（传 `user` 主键、已评审禁删） |
| §6 #5 合并冲突被当成功 | ✅ | `api/repository.ts#mergeMergeRequest` 返回 `{conflict}`；详情页补 4 步 git 指引 |
| §6 #6 i18n key 拼错 | ✅ | `label.createdAt`（并补进 `i18n/supplement.ts`） |
| §6 #7 错误码表缺 3 个 | ✅ | 改为**按页面**处理（旧版即各页 `codeValidator`），不再扩全局表 |
| §6 #8 webhook 日志渲染对象 | ✅ | `RepositoryWebhookLogData.request/response` 改 `unknown` + 安全序列化 |
| §6 #9 保护分支角色语义压扁 | ✅ | `SettingsBranch.tsx` 恢复角色集合语义（最低角色及以上全部可操作） |
| §6 #10 404 死链 | ✅ | `router.tsx` 补 9 条路由 + 6 个新页面 |
| §6 #11 MR 列表 `sort` 必填未传 | ✅ | `api/repository.ts#fetchMergeRequests` 默认 `'desc'` |
| §6 #12 组内 MR 缺 keyword/page/pagesize → 500 | ✅ | `api/group.ts#fetchGroupMergeRequests` 三个参数显式带上 |
| §6 #13 全局 MR 看板 `sort` 必填未传 | ✅ | `api/repository.ts#fetchRelatedMergeRequests` 默认 `'desc'` |
| §2.1 资料页 MFA 二维码 / `role` 可编辑 | ✅ | `pages/settings/Profile.tsx` |
| §2.3 SSH 指纹 / 公钥注释容忍 | ✅ | `pages/settings/SSHKey.tsx` |
| §2.4 通知组/仓库级开关 | ✅ | `pages/settings/Notification.tsx` + `SettingsNotificationMatrix.tsx` |
| §3.2 / §3.3 新建仓库 / Fork 页 | ✅ | `pages/NewRepository.tsx`、`pages/NewRepositoryFork.tsx` |
| §3.1 fork 列表上下文 | ✅ | `pages/RepositoryList.tsx` + `/repositories/forklist/{id}` |
| §3.4 仓库设置名称/描述校验与计数 | ✅ | `pages/repository/SettingsGeneral.tsx` |
| §3.5 成员分表 / deleteFlag / 粗粒度权限 | ✅ | `components/view/unit/MembersManager.tsx` + `SettingsMembers.tsx` |
| §3.6 保护分支集合语义 + 角色白名单 | ✅ | `pages/repository/SettingsBranch.tsx` |
| §3.7 slug 校验 / 创建者下拉 / 两级确认 | ✅ | `components/view/unit/AdvancedSettings.tsx` + `SettingsAdvanced.tsx` |
| §3.8 Webhook secret / 创建人 / 更新时间 / 复制 | ✅ | `pages/repository/SettingsWebhook.tsx` |
| §3.9 分支标签增删与筛选 | ✅ | `components/view/unit/RefListView.tsx` + `BranchList.tsx` / `TagList.tsx` |
| §3.10 活动流分类/搜索/翻页 | ✅ | `pages/repository/ActivityList.tsx` |
| §3.11 提交日期分组 / 父提交 / 浏览文件 / 子目录 | ✅ | `pages/repository/CommitList.tsx`、`CommitDetail.tsx`、`router.tsx` |
| §3.12 仓库头 fork/clone/6 项计数；文件页 README/submodule/blame/空仓库/复制 | ✅ | `components/view/unit/RepositoryHeader.tsx`、`ObjectViewer.tsx`、`RepositoryEmpty.tsx`、`FileTree.tsx` |
| §4.1 仪表盘可点击 + 阈值配色 + 数值格式 | ✅ | `pages/admin/Dashboard.tsx` |
| §4.2 用户分类页签 / 排序 / 添加用户 / 口令校验 | ✅ | `pages/admin/Users.tsx`、`api/admin.ts` |
| §4.3 / §4.4 组与仓库成员管理 | ✅ | `components/view/unit/AdminMembersDialog.tsx` |
| §4.5 系统设置结构化表单 | ✅ | `pages/admin/Settings.tsx` |
| §5.1 MR 列表全部页签/角标/排序/分页/行字段 | ✅ | `pages/repository/MergeRequestList.tsx` + `MergeRequestRow.tsx` + `MergeRequestToolbar.tsx` |
| §5.2 MR 详情三 Tab / 活动栏 / 版本对比 / 合并信息 / Squash / 评审状态 | ✅ | `pages/repository/MergeRequestDetail.tsx` |
| §5.3 新建 MR 源/目标仓库选择器 + 分支提交预览 + 同分支校验 | ✅ | `pages/repository/MergeRequestCreate.tsx`（`global` 态） |
| §5.4 看板 relatedRepository / 搜索 / 排序 / 分页 / 创建入口 | ✅ | `pages/MergeRequestBoard.tsx` |
| §5.2 diff 汇总条 / 布局切换 / 逐文件折叠 | ✅ | `components/unit/DiffViewer.tsx` |

**门禁**（AGENTS §8）：后端 `pwsh -File scripts/verify.ps1` 六项全绿；
前端 `npm run typecheck` / `npm run lint` / `npm run build` 全绿，`web/build` 已重建。