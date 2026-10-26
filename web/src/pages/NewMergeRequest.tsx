import MergeRequestCreate from '@/pages/repository/MergeRequestCreate'

/**
 * 全局新建合并请求（`/mergerequests/new`）。
 *
 * 旧版这里是独立的"第 1 步"页：先选源/目标仓库与分支，再进入详情页填标题。
 * 新版把表单统一收进 `MergeRequestCreate`，用 `global` 打开"源仓库/目标仓库"两个选择器，
 * 避免同一套表单写两遍（AGENTS §4）。
 */
export default function NewMergeRequest() {
  return <MergeRequestCreate global />
}
