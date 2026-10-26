/**
 * 图标 `caretDownLine`（1024×1024，线框三角，指向下）。
 *
 * **手写图标，不由 `scripts/extract-icons.mjs` 生成**（该脚本只搬运
 * `@pgyer/icons@0.0.16` 里的图标，重新生成不会覆盖本文件，也不会覆盖同名项）。
 *
 * 为什么手写：
 *   - 图标集里只有 `psCaretDown`（**实心**三角），用在页头"仓库 / 仓库组"触发器上
 *     视觉重量明显压过 14px 的按钮文字；
 *   - 官方原版页头（旧 `www/view/src/components/unit/GroupRepositoryMenu.js`）用的是
 *     FontAwesome `faAngleDown`，即纯线条的下箭头 —— 线形才是这套页头的口径。
 *
 * 画法：同一个 path 里两个子路径（外三角 + 内三角），**绕向相反**。
 * 这样在 `fill="currentColor"` + 默认 nonzero 填充规则下，内三角被挖空成"线框"，
 * 不需要 `Icon` 组件支持 stroke（它只渲染填充路径，见 `components/unit/Icon`）。
 *
 * 尺寸口径：外三角 896×512（宽高比 1.75:1，与 `psCaretDown` 一致）；
 * 内三角按质心做 0.5 等比缩放，等效线宽约 1px（12px 渲染时）。
 */
const data = {
  name: 'caretDownLine',
  width: 1024,
  height: 1024,
  paths: [
    // 外三角（顺时针）：上左 → 上右 → 下尖
    'M64 256H960L512 768Z ' +
      // 内三角（逆时针，挖空）：上左 → 下尖 → 上右
      'M288 341L512 597L736 341Z',
    '',
  ],
} as const

export default data
