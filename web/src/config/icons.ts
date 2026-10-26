/**
 * 图标集合（按需导入，避免把 154 个图标全部打进产物）。
 *
 * 数据来源：`@pgyer/icons@0.0.16`，由 `scripts/extract-icons.mjs` 本地化为
 * `src/assets/icons/*.ts`。这里只 re-export **当前批次用到的**图标；
 * 后续批次按需追加（业务代码请 `import { plXxx } from '@/config/icons'`，
 * 不要直接引 `assets/icons/*`，以便统一管理使用面）。
 */
export { default as plAddAlt } from '@/assets/icons/plAddAlt'
export { default as plBell } from '@/assets/icons/plBell'
export { default as plDraft } from '@/assets/icons/plDraft'
export { default as plExit } from '@/assets/icons/plExit'
export { default as plFork } from '@/assets/icons/plFork'
export { default as plHelp } from '@/assets/icons/plHelp'
export { default as plKey } from '@/assets/icons/plKey'
export { default as plLocalization } from '@/assets/icons/plLocalization'
export { default as plMember } from '@/assets/icons/plMember'
export { default as plMemberAlt } from '@/assets/icons/plMemberAlt'
export { default as plMerge } from '@/assets/icons/plMerge'
export { default as plRepository } from '@/assets/icons/plRepository'
export { default as plSearch } from '@/assets/icons/plSearch'
export { default as plSetting } from '@/assets/icons/plSetting'
export { default as psAddAlt } from '@/assets/icons/psAddAlt'
export { default as psBranch } from '@/assets/icons/psBranch'
export { default as psCaretDown } from '@/assets/icons/psCaretDown'
export { default as psCommit } from '@/assets/icons/psCommit'
export { default as psFile } from '@/assets/icons/psFile'
export { default as psFolder } from '@/assets/icons/psFolder'
export { default as psLog } from '@/assets/icons/psLog'
export { default as psMembers } from '@/assets/icons/psMembers'
export { default as plMenuCollapse } from '@/assets/icons/plMenuCollapse'
export { default as psMerge } from '@/assets/icons/psMerge'
export { default as psRepository } from '@/assets/icons/psRepository'
export { default as psRepositoryGroup } from '@/assets/icons/psRepositoryGroup'
export { default as psSetting } from '@/assets/icons/psSetting'
export { default as psTag } from '@/assets/icons/psTag'
export { default as psRepair } from '@/assets/icons/plRepair'
export { default as psBell } from '@/assets/icons/psBell'
export { default as psEmail } from '@/assets/icons/psEmail'
export { default as psKey } from '@/assets/icons/psKey'
export { default as psMemberAlt } from '@/assets/icons/psMemberAlt'
// 管理后台仪表盘用：资源计量（CPU/内存）与服务状态。
export { default as psMeter } from '@/assets/icons/psMeter'
export { default as psNewFeature } from '@/assets/icons/psNewFeature'
// 排序按钮（MR 列表 / 看板）。
export { default as plSortDesc } from '@/assets/icons/plSortDesc'
// diff 视图（折叠箭头 / 复制 / 展开）与评审状态角标。
export { default as plAngleDown } from '@/assets/icons/plAngleDown'
export { default as plAngleRight } from '@/assets/icons/plAngleRight'
export { default as plCopy } from '@/assets/icons/plCopy'
export { default as plExpand } from '@/assets/icons/plExpand'
export { default as psClock } from '@/assets/icons/psClock'
export { default as psConfirm } from '@/assets/icons/psConfirm'
