/**
 * 图标集合（按需导入，避免把整个图标库打进产物）。
 *
 * 来源（DESIGN.md §5）：`@phosphor-icons/react`，regular 为默认字重，
 * duotone 用于空态 / 功能介绍，fill 仅用于选中态（由调用方传 `weight`）。
 * 业务代码一律从本模块导入，不得直接 import `@phosphor-icons/react` ——
 * 便于统一管理使用面与语义映射（§5 语义映射表）。
 *
 * 历史说明：v3 及以前用的是本地化的 `@pgyer/icons`（pl 与 ps 前缀）。
 * v4 切换 Phosphor 后，下面保留这批旧导出名作为别名，既有调用零改动；
 * 新代码请直接用 Phosphor 原名（Book / GitBranch / …）。
 * `extract-icons.mjs` 与 `assets/icons/` 已随 v4 移除。
 */
import type { ComponentType } from 'react'
import type { IconProps as PhosphorIconProps } from '@phosphor-icons/react'
import {
  ArrowClockwise,
  ArrowRight,
  ArrowsOutSimple,
  Article,
  Bell,
  Book,
  CaretDown,
  CaretLeft,
  CaretRight,
  CaretUp,
  Check,
  CheckCircle,
  CircleNotch,
  Clock,
  Code,
  Copy,
  DownloadSimple,
  DotsThree,
  EnvelopeSimple,
  Eye,
  FileText,
  FolderSimple,
  Gauge,
  GearSix,
  GitBranch,
  GitCommit,
  GitFork,
  GitMerge,
  GitPullRequest,
  Globe,
  House,
  Info,
  Key,
  List,
  Lock,
  MagnifyingGlass,
  Password,
  PencilSimple,
  Plus,
  Question,
  Rows,
  SignOut,
  SortDescending,
  Sparkle,
  SquaresFour,
  Star,
  Tag,
  Trash,
  User,
  Users,
  Warning,
  WarningCircle,
  WebhooksLogo,
  Wrench,
  X,
} from '@phosphor-icons/react'

export type IconComponent = ComponentType<PhosphorIconProps>

export {
  /* —— DESIGN.md §5 语义映射（canonical） —— */
  Book as repositoryIcon, // 仓库
  SquaresFour as groupIcon, // 仓库组
  GitPullRequest as mergeRequestIcon, // 合并请求
  GitBranch as branchIcon, // 分支
  GitCommit as commitIcon, // 提交
  Tag as tagIcon, // 标签
  GitMerge as mergeIcon, // 合并
  Code as codeIcon, // 代码
  WebhooksLogo as webhookIcon, // Webhook
  Key as keyIcon, // SSH 密钥
  Password as tokenIcon, // 访问令牌
  Users as membersIcon, // 成员
  User as userIcon, // 用户（单人）
  GearSix as settingsIcon, // 设置
  MagnifyingGlass as searchIcon, // 搜索
  Bell as notificationIcon, // 通知
  Sparkle as sparkleIcon, // AI / 新特性
  Copy as copyIcon, // 复制（成功后调用方切 Check）
  Check as checkIcon,
  Trash as trashIcon, // 删除
  Star as starIcon, // 星标（fill 表示已标星）
  DownloadSimple as downloadIcon, // 下载
  Plus as plusIcon,
  X as closeIcon,
  CaretDown as caretDownIcon,
  CaretUp as caretUpIcon,
  CaretLeft as caretLeftIcon,
  CaretRight as caretRightIcon,
  DotsThree as moreIcon, // 行末 ⋯ 菜单
  EnvelopeSimple as mailIcon,
  SignOut as signOutIcon,
  Globe as globeIcon, // 语言
  Question as questionIcon, // 帮助
  Wrench as adminIcon, // 管理后台
  Gauge as meterIcon, // 资源计量
  List as listIcon,
  ArrowsOutSimple as expandIcon,
  Rows as rowsIcon, // 列表视图
  SortDescending as sortDescIcon,
  Clock as clockIcon,
  CheckCircle as checkCircleIcon,
  Warning as warningIcon,
  WarningCircle as warningCircleIcon,
  Info as infoIcon,
  FileText as fileIcon,
  FolderSimple as folderIcon,
  Article as logIcon, // 日志
  GitFork as forkIcon, // 复刻
  PencilSimple as editIcon,
  Eye as eyeIcon,
  Lock as lockIcon, // 私有
  House as homeIcon,
  ArrowRight as arrowRightIcon,
  ArrowClockwise as refreshIcon, // 重新生成 / 重发
  CircleNotch as spinnerIcon,

  /* —— 旧导出名别名（v3 时代 pl 与 ps 前缀，保持既有调用零改动） —— */
  Plus as plAddAlt,
  Plus as psAddAlt,
  Plus as psAdd,
  Bell as plBell,
  Bell as psBell,
  EnvelopeSimple as plDraft,
  EnvelopeSimple as psEmail,
  SignOut as plExit,
  GitFork as plFork,
  Question as plHelp,
  Key as plKey,
  Key as psKey,
  Globe as plLocalization,
  Users as plMember,
  Users as psMembers,
  User as plMemberAlt,
  User as psMemberAlt,
  GitMerge as plMerge,
  GitMerge as psMerge,
  Book as plRepository,
  Book as psRepository,
  MagnifyingGlass as plSearch,
  GearSix as plSetting,
  GearSix as psSetting,
  GitBranch as psBranch,
  CaretDown as psCaretDown,
  GitCommit as psCommit,
  FileText as psFile,
  FolderSimple as psFolder,
  Article as psLog,
  SquaresFour as psRepositoryGroup,
  Tag as psTag,
  Wrench as psRepair,
  Wrench as plRepair,
  Gauge as psMeter,
  Sparkle as psNewFeature,
  SortDescending as plSortDesc,
  CaretDown as plAngleDown,
  CaretRight as plAngleRight,
  Copy as plCopy,
  ArrowsOutSimple as plExpand,
  Clock as psClock,
  CheckCircle as psConfirm,
  CaretDown as plCaretDownLine,
  Check as plCheck,
  List as plMenuCollapse,
}
