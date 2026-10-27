# 大厂代码托管平台「首页/工作台」调研

## 1. 小米：基本无公开资料
- 公开资料只到工具链构成：研效三阶段（开源→采购→自研效能平台），工具层为 **GitLab + Phabricator + Jenkins on K8s + Nexus**，自研平台目标是"标准化整合各工具、一键接入"，不自研单个工具（https://devops.phodal.com/case-study/xiaomi/）。
- 对外只有 github.com/micode（内核开源）、github.com/XiaoMi，是仓库集合而非内部平台。
- 内部平台（研效/效率工程）首页 IA：**无公开资料**，不可作为参考依据。

## 2. 阿里：Aone（内部）/ 云效 Codeup（对外）
- Aone 服务 2 万+工程师，端到端需求→发布（https://developer.aliyun.com/article/605149）；公开技术分享只讲**代码服务后端架构**（读写分离→分片→"伽利略"三副本多写），**不含首页设计**（https://www.cnblogs.com/yunqishequ/p/15146937.html）。
- Codeup 首页（有官方文档）：导航栏 = 代码库列表/代码组列表/合并请求列表/代码检测/回收站/全局设置。**首屏就是代码库列表**：顶部搜索框+导入+新建；Tab「我参与的/全部」；每行显示提交数、分支数、标签数、语言分布；「我参与的」按更新时间倒序、**收藏置顶**；外部成员无「全部」；MR 列表按开启/合并/关闭分类，可按评审人、发起人、WIP-评审中筛选。（https://help.aliyun.com/zh/yunxiao/user-guide/home-overview）
- 个人工作台：首次登录**先选角色**（研发者/研发负责人/产品经理/项目经理/测试）生成模板布局，之后可自选 **16 张卡片**（工作项、迭代进展、工作小结、合并请求、流水线、应用、测试用例、度量报表）。（https://help.aliyun.com/zh/yunxiao/user-guide/personal-workbench）

## 3. 腾讯：工蜂 / CODING
- 工蜂：主页导航「项目/项目组」，右侧 + 号或「+创建项目」，顶部有项目搜索框；项目内左侧导航 + 项目仪表板 + 里程碑；**「待办」集中所有与你相关的待办和消息**；项目「统计图表」可按分支看成员贡献与提交统计；关注/标星模型。特色是 UGit 客户端、TAPD 双向联动、OWNERS、划词评审。（https://code.tencent.com/help/productionDoc/project、https://code.tencent.com/production/、https://git.tencent.com/help/menu/instruction/product-docs/todo.html）
- CODING：**登录团队首页默认进入个人工作台**，按角色给三套模板——研发任务工作台（我的项目事项 / 我的合并请求「我发起的·我评审的」/ 我的构建计划 / 我的待确认发布）、需求管理工作台（待规划+迭代内已规划）、项目管理工作台（近 30 天概览仪表盘、项目整体进展、团队严重逾期事项）。（https://coding.net/help/docs/member/workbench.html）
- 首屏本质：**"未完成事项表 + 待我评审的 MR 表"**，不是卡片墙。

## 4. DeepSeek：无公开资料
未检索到任何内部代码托管/研效平台的公开资料，**明确无公开资料**。可参考对外线索：github.com/deepseek-ai（45 个仓，统一 Deep*/FlashMLA/TileKernels 命名、全英文 README、MIT/Apache-2.0、按 kernel-推理-通信分层、几乎无装饰）。据此推断其风格倾向：深蓝主色（deepseek.com 品牌蓝）+ 大留白；英文/术语优先，hash、分支、命令用等宽字体；文档化排版（层级标题+表格 > 插画）；入口少、无运营 banner；暗色跟随系统。**以上为推断，非内部事实**。

## 5. 标杆对照
- **GitHub**：旧 dashboard = Recent activity（近两周最多 4 条）+ Top repositories（自动生成、一年未交互掉出）+ Feed（关注者动态）；新版 Home（preview）= Copilot 提示框 / Agent sessions / Pull requests / Issues，**AI 入口占据首屏第一位**。（https://docs.github.com/en/account-and-profile/reference/personal-dashboard）
- **GitLab**：个人首页为默认落地页；右侧 Merge requests 卡片（我发起/指派给我/待我评审，含 Review requested tab）、「Items that need your attention」待处理清单（可按 Failed builds 过滤）、「Follow the latest updates」动态（Your activity/Starred projects/Followed users）、右侧 Quick access（Recently viewed / Projects）。（https://docs.gitlab.com/tutorials/personal_homepage/）
- **Gitea/Forgejo**：dashboard 为登录后首屏，支持**个人↔组织上下文切换**；左侧 Issues/Pulls/Milestones 聚合视图，中部活动 feed（Action 时间线），另有 Explore 面向发现。（https://deepwiki.com/go-gitea/gitea/7.1-user-dashboard-and-activity-feeds）
- **华为 CodeArts**：工作台页签 = 我的工作项/我的文件/我的 Wiki/我的测试；Repo 仓库列表按「我关注的/我参与的/我创建的」三维度。（https://support.huaweicloud.com/intl/zh-cn/usermanual-devcloud/devcloud_01_0024.html、https://support.huawei.com/enterprise/zh/doc/EDOC1100468527/ddf7bb8d）
- **百度 iCode**：产品页只有 CR 机制（Gerrit 风格）、代码搜索、操作审计、iScan/iPipe 打通、多人打分；**首页 IA 无公开资料**。（https://cloud.baidu.com/product/icode.html）
- **字节**：公开只有 MBox 本地研发链、Monorepo 流程、平台工程等演讲，**无代码平台首页资料**。（https://qcon.infoq.cn/2020/beijing/presentation/2988）

## 6. 跨平台共性结论（自建 Git 平台首页推荐排序）

| # | 模块 | 出现平台 | 定性 |
|---|---|---|---|
| 1 | 全局顶栏：搜索（仓库/人/提交）+「新建/导入仓库」+ 个人菜单 | 云效、工蜂、GitLab、CodeArts | 必须有 |
| 2 | 「待我处理」清单：待我评审 MR / 指派给我 / 流水线失败 | GitLab、CODING、GitHub、云效 | 必须有 |
| 3 | 我的仓库列表：我参与的/我创建的/收藏置顶/更新时间倒序，含提交数·分支数·语言条 | 云效、CodeArts、CODING | 必须有 |
| 4 | 组织/团队上下文切换与权限可见性表达（外部成员看不到「全部」） | Gitea、云效、CodeArts | 必须有 |
| 5 | 最近访问 / 常用仓库快捷区 | GitLab、GitHub、工蜂（关注/标星） | 必须有 |
| 6 | 快捷克隆地址（HTTPS/SSH 一键复制）+ 空仓初始化引导 | 云效、工蜂、CodeArts | 加分 |
| 7 | 团队动态 feed | GitLab、GitHub、Gitea | 加分（内部噪音大，默认折叠） |
| 8 | 角色化模板 / 可自定义卡片布局 | 云效（5 角色 16 卡片）、CODING（3 套） | 加分（价值高、成本高） |
| 9 | 贡献统计 / 个人工作小结 | CODING、工蜂 | 加分 |
| 10 | 公告 / 平台更新日志 | 云效、工蜂 | 加分 |
| 11 | 新人 first-run 引导（SSH key、clone、第一个 MR） | CODING、GitHub | 加分 |
| 12 | 暗色模式 | GitHub、GitLab（普遍做法） | 加分 |

**风格结论**：对内平台几乎全部**列表优先于卡片**，首屏密度优先（云效首屏直接是仓库表格）；仪表盘/卡片只在"项目管理"语境出现（CODING、云效 16 卡片）。色彩克制——单主色+语义色（评审中/失败/已合并），无插画与运营 banner。GitHub 新版把 AI 入口放首屏第一，是唯一显著卡片化的例外。留白处于中等偏紧，行高服务于表格可扫读性，而非营销页节奏。
