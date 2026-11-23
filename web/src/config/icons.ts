/**
 * 图标集合（按需导入，避免把整个图标库打进产物）。
 *
 * 来源：`@phosphor-icons/react`，regular 为默认字重，duotone 用于空态，
 * fill 仅用于选中态（由调用方传 `weight`）。
 * 业务代码一律从本模块导入，不得直接 import `@phosphor-icons/react`。
 */
import type { ComponentType } from 'react'
import type { IconProps as PhosphorIconProps } from '@phosphor-icons/react'
import {
  ArrowLeft,
  ArrowRight,
  ArrowsOutSimple,
  Bell,
  Book,
  CaretDown,
  CaretRight,
  Check,
  CheckCircle,
  Clock,
  Copy,
  DotsThree,
  EnvelopeSimple,
  Eye,
  EyeSlash,
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
  Key,
  List,
  Lock,
  MagnifyingGlass,
  Moon,
  Plus,
  Question,
  ShieldCheck,
  SignOut,
  SortDescending,
  Sparkle,
  SquaresFour,
  Star,
  Sun,
  Tag,
  User,
  Users,
  WarningCircle,
  Wrench,
  X,
} from '@phosphor-icons/react'

export type IconComponent = ComponentType<PhosphorIconProps>

export {
  /* —— 语义映射（canonical） —— */
  Book as repositoryIcon, // 仓库
  SquaresFour as groupIcon, // 仓库组
  GitPullRequest as mergeRequestIcon, // 合并请求
  Users as membersIcon, // 成员
  User as userIcon, // 用户（单人）
  GearSix as settingsIcon, // 设置
  Bell as notificationIcon, // 通知
  Copy as copyIcon, // 复制（成功后调用方切 Check）
  Check as checkIcon,
  Star as starIcon, // 星标（fill 表示已标星）
  Plus as plusIcon,
  X as closeIcon,
  CaretDown as caretDownIcon,
  DotsThree as moreIcon, // 行末 ⋯ 菜单
  Question as questionIcon, // 帮助
  Wrench as adminIcon, // 管理后台
  Gauge as meterIcon, // 资源计量
  CheckCircle as checkCircleIcon,
  WarningCircle as warningCircleIcon,
  GitFork as forkIcon, // 复刻
  Lock as lockIcon, // 私有
  ArrowLeft as arrowLeftIcon,
  ArrowRight as arrowRightIcon,
  EnvelopeSimple as emailIcon,
  Eye as eyeIcon,
  EyeSlash as eyeSlashIcon,
  Globe as globeIcon,
  Key as keyIcon,
  Moon as moonIcon,
  Sun as sunIcon,
  ShieldCheck as shieldCheckIcon,
  List as listViewIcon,
  SquaresFour as gridViewIcon,

  /* —— 兼容别名（旧 pl / ps 前缀） —— */
  SignOut as plExit,
  Users as plMember,
  Users as psMembers,
  GitMerge as plMerge,
  Book as plRepository,
  Book as psRepository,
  MagnifyingGlass as plSearch,
  GearSix as plSetting,
  GitBranch as psBranch,
  CaretDown as psCaretDown,
  GitCommit as psCommit,
  FileText as psFile,
  FolderSimple as psFolder,
  SquaresFour as psRepositoryGroup,
  Tag as psTag,
  Gauge as psMeter,
  Sparkle as psNewFeature,
  SortDescending as plSortDesc,
  CaretDown as plAngleDown,
  CaretRight as plAngleRight,
  ArrowsOutSimple as plExpand,
  Clock as psClock,
  CheckCircle as psConfirm,
  List as plMenuCollapse,
}
