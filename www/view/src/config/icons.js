// 项目内的图标注册表 —— 唯一允许的图标引入入口。
//
// 为什么需要它（2026-10 性能整改 E1）：
//
// `@pgyer/icons` 的 barrel（index.js）一次性 import 了全部 154 个图标并逐个包装，
// 而该包没有 `sideEffects` 字段，于是 154/154 全部进入产物 —— 实际只用到 59 个。
// 这里改用**按路径导入**（webpack 因此只打包用到的模块），并用 helpers/pgyerIcon
// 复刻 barrel 的包装逻辑，保证导出的仍是 FontAwesome 认识的图标定义对象。
//
// 使用约定（见 AGENTS.md §4）：
//   ✅ import { plSearch } from 'APPSRC/config/icons'
//   ❌ import plSearch from '@pgyer/icons/icons/plSearch'  （拿到原始数组 → 图标空白）
//   ❌ import { plSearch } from '@pgyer/icons'             （重新引入全部 154 个图标）
//
// 新增图标：在此追加两行（import + 数组项）即可。
import pgyerIcon from 'APPSRC/helpers/pgyerIcon'

import plAddAltData from '@pgyer/icons/icons/plAddAlt'
import plAngleDownData from '@pgyer/icons/icons/plAngleDown'
import plBellData from '@pgyer/icons/icons/plBell'
import plBranchData from '@pgyer/icons/icons/plBranch'
import plCheckData from '@pgyer/icons/icons/plCheck'
import plClockData from '@pgyer/icons/icons/plClock'
import plCloseData from '@pgyer/icons/icons/plClose'
import plCodeFileData from '@pgyer/icons/icons/plCodeFile'
import plCommitData from '@pgyer/icons/icons/plCommit'
import plConfirmData from '@pgyer/icons/icons/plConfirm'
import plCopyData from '@pgyer/icons/icons/plCopy'
import plDraftData from '@pgyer/icons/icons/plDraft'
import plEditData from '@pgyer/icons/icons/plEdit'
import plExitData from '@pgyer/icons/icons/plExit'
import plExpandData from '@pgyer/icons/icons/plExpand'
import plFileData from '@pgyer/icons/icons/plFile'
import plForbidData from '@pgyer/icons/icons/plForbid'
import plForkData from '@pgyer/icons/icons/plFork'
import plHelpData from '@pgyer/icons/icons/plHelp'
import plKeyData from '@pgyer/icons/icons/plKey'
import plLocalizationData from '@pgyer/icons/icons/plLocalization'
import plMemberData from '@pgyer/icons/icons/plMember'
import plMemberAltData from '@pgyer/icons/icons/plMemberAlt'
import plMenuCollapseData from '@pgyer/icons/icons/plMenuCollapse'
import plMergeData from '@pgyer/icons/icons/plMerge'
import plMergedData from '@pgyer/icons/icons/plMerged'
import plRepairData from '@pgyer/icons/icons/plRepair'
import plRepositoryData from '@pgyer/icons/icons/plRepository'
import plSafeData from '@pgyer/icons/icons/plSafe'
import plSearchData from '@pgyer/icons/icons/plSearch'
import plSettingData from '@pgyer/icons/icons/plSetting'
import plSortDescData from '@pgyer/icons/icons/plSortDesc'
import plTagData from '@pgyer/icons/icons/plTag'
import plTrashData from '@pgyer/icons/icons/plTrash'
import psAddAltData from '@pgyer/icons/icons/psAddAlt'
import psBellData from '@pgyer/icons/icons/psBell'
import psBranchData from '@pgyer/icons/icons/psBranch'
import psCaretDownData from '@pgyer/icons/icons/psCaretDown'
import psClockData from '@pgyer/icons/icons/psClock'
import psCommitData from '@pgyer/icons/icons/psCommit'
import psConfirmData from '@pgyer/icons/icons/psConfirm'
import psEmailData from '@pgyer/icons/icons/psEmail'
import psErrorData from '@pgyer/icons/icons/psError'
import psFileData from '@pgyer/icons/icons/psFile'
import psFolderData from '@pgyer/icons/icons/psFolder'
import psHomeData from '@pgyer/icons/icons/psHome'
import psKeyData from '@pgyer/icons/icons/psKey'
import psLogData from '@pgyer/icons/icons/psLog'
import psMemberAltData from '@pgyer/icons/icons/psMemberAlt'
import psMembersData from '@pgyer/icons/icons/psMembers'
import psMergeData from '@pgyer/icons/icons/psMerge'
import psMeterData from '@pgyer/icons/icons/psMeter'
import psMoreData from '@pgyer/icons/icons/psMore'
import psNewFeatureData from '@pgyer/icons/icons/psNewFeature'
import psRepositoryData from '@pgyer/icons/icons/psRepository'
import psRepositoryGroupData from '@pgyer/icons/icons/psRepositoryGroup'
import psSettingData from '@pgyer/icons/icons/psSetting'
import psTagData from '@pgyer/icons/icons/psTag'
import psTrashData from '@pgyer/icons/icons/psTrash'

export const {
  plAddAlt, plAngleDown, plBell, plBranch, plCheck, plClock, plClose, plCodeFile, plCommit, plConfirm, plCopy, plDraft, plEdit, plExit, plExpand, plFile, plForbid, plFork, plHelp, plKey, plLocalization, plMember, plMemberAlt, plMenuCollapse, plMerge, plMerged, plRepair, plRepository, plSafe, plSearch, plSetting, plSortDesc, plTag, plTrash, psAddAlt, psBell, psBranch, psCaretDown, psClock, psCommit, psConfirm, psEmail, psError, psFile, psFolder, psHome, psKey, psLog, psMemberAlt, psMembers, psMerge, psMeter, psMore, psNewFeature, psRepository, psRepositoryGroup, psSetting, psTag, psTrash
} = {
  plAddAlt: pgyerIcon(plAddAltData),
  plAngleDown: pgyerIcon(plAngleDownData),
  plBell: pgyerIcon(plBellData),
  plBranch: pgyerIcon(plBranchData),
  plCheck: pgyerIcon(plCheckData),
  plClock: pgyerIcon(plClockData),
  plClose: pgyerIcon(plCloseData),
  plCodeFile: pgyerIcon(plCodeFileData),
  plCommit: pgyerIcon(plCommitData),
  plConfirm: pgyerIcon(plConfirmData),
  plCopy: pgyerIcon(plCopyData),
  plDraft: pgyerIcon(plDraftData),
  plEdit: pgyerIcon(plEditData),
  plExit: pgyerIcon(plExitData),
  plExpand: pgyerIcon(plExpandData),
  plFile: pgyerIcon(plFileData),
  plForbid: pgyerIcon(plForbidData),
  plFork: pgyerIcon(plForkData),
  plHelp: pgyerIcon(plHelpData),
  plKey: pgyerIcon(plKeyData),
  plLocalization: pgyerIcon(plLocalizationData),
  plMember: pgyerIcon(plMemberData),
  plMemberAlt: pgyerIcon(plMemberAltData),
  plMenuCollapse: pgyerIcon(plMenuCollapseData),
  plMerge: pgyerIcon(plMergeData),
  plMerged: pgyerIcon(plMergedData),
  plRepair: pgyerIcon(plRepairData),
  plRepository: pgyerIcon(plRepositoryData),
  plSafe: pgyerIcon(plSafeData),
  plSearch: pgyerIcon(plSearchData),
  plSetting: pgyerIcon(plSettingData),
  plSortDesc: pgyerIcon(plSortDescData),
  plTag: pgyerIcon(plTagData),
  plTrash: pgyerIcon(plTrashData),
  psAddAlt: pgyerIcon(psAddAltData),
  psBell: pgyerIcon(psBellData),
  psBranch: pgyerIcon(psBranchData),
  psCaretDown: pgyerIcon(psCaretDownData),
  psClock: pgyerIcon(psClockData),
  psCommit: pgyerIcon(psCommitData),
  psConfirm: pgyerIcon(psConfirmData),
  psEmail: pgyerIcon(psEmailData),
  psError: pgyerIcon(psErrorData),
  psFile: pgyerIcon(psFileData),
  psFolder: pgyerIcon(psFolderData),
  psHome: pgyerIcon(psHomeData),
  psKey: pgyerIcon(psKeyData),
  psLog: pgyerIcon(psLogData),
  psMemberAlt: pgyerIcon(psMemberAltData),
  psMembers: pgyerIcon(psMembersData),
  psMerge: pgyerIcon(psMergeData),
  psMeter: pgyerIcon(psMeterData),
  psMore: pgyerIcon(psMoreData),
  psNewFeature: pgyerIcon(psNewFeatureData),
  psRepository: pgyerIcon(psRepositoryData),
  psRepositoryGroup: pgyerIcon(psRepositoryGroupData),
  psSetting: pgyerIcon(psSettingData),
  psTag: pgyerIcon(psTagData),
  psTrash: pgyerIcon(psTrashData)
}
