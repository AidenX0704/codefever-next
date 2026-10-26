/**
 * 帮助中心 / 关于页用到的**站外**地址（唯一事实源）。
 *
 * 为什么要下沉到 config/（AGENTS §4）：这批地址原先写死在
 * `components/layout/Header.tsx` 的 `HELP_LINKS` 里，帮助中心页是第二处使用点 ——
 * 按"第二次出现即抽取"的约定收口到这里，页头菜单与帮助页共用一份。
 *
 * 取值口径：**指向 CodeFever Next 本仓库**，不是上游 PGYER/codefever ——
 * 本项目是上游的二次开发衍生版、由独立维护者维护（见 README「项目声明」），
 * 把反馈/贡献入口指到上游会让问题落到无权处理的人手里。
 * 上游仓库与官网仍然保留：它们是"关于"页要交代的来源信息。
 */
export const HELP_LINKS = {
  /** 技术支持 / 问题求助：本仓库议题区。 */
  support: 'https://github.com/AidenX0704/codefever-next/issues',
  /** 提交反馈：新建议题。 */
  feedback: 'https://github.com/AidenX0704/codefever-next/issues/new',
  /** 为 CodeFever Next 贡献代码：本仓库首页（Fork / PR 入口）。 */
  contribute: 'https://github.com/AidenX0704/codefever-next',
  /** 贡献指南（开发环境、提交规范、PR 流程）。 */
  contributingGuide: 'https://github.com/AidenX0704/codefever-next/blob/main/CONTRIBUTING.md',
  /** 安全漏洞的私下报告流程。 */
  securityPolicy: 'https://github.com/AidenX0704/codefever-next/blob/main/SECURITY.md',
  /** 项目维护者。 */
  maintainer: 'https://github.com/AidenX0704',
  /** 上游项目（CodeFever Community）。 */
  upstream: 'https://github.com/PGYER/codefever',
  /** 上游官网。 */
  upstreamSite: 'https://codefever.cn',
} as const
