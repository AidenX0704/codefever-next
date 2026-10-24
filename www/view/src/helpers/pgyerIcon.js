// @pgyer/icons 图标的本地包装。
//
// 为什么需要这个文件（2026-10 性能整改 E1 的回归修复）：
//
// `@pgyer/icons` 的 `icons/*.js` 每个模块只导出**原始数据数组**
// `[name, size, path0, path1]`。真正的图标对象是 barrel（`index.js`）里
// 用一个内部 helper 造出来的：
//
//   const warpper = function (name, size, path0, path1) {
//     return {
//       prefix: 'pg',
//       iconName: name,
//       icon: [size[0], size[1], [], 'e000', [path1, path0]]
//     }
//   }
//   const plSearch = warpper(...plSearchData)
//
// 该 barrel 没有 `sideEffects` 字段，`index.js` 一次性 import 了全部 154 个
// 图标并逐个包装，导致 154/154 全部进入产物（实际只用 59 个）。
// 为了减小体积，源码改为按路径导入 —— 但**按路径拿到的是原始数组**，
// 直接传给 `<FontAwesomeIcon icon={...}>` 不会渲染任何东西（图标一片空白）。
//
// 因此这里复刻 barrel 的包装逻辑：体积上仍然只引入用到的图标模块，
// 语义上与 barrel 完全一致。
//
// 该实现必须与 `node_modules/@pgyer/icons/index.js` 的 `warpper` 保持一致；
// 若依赖升级后图标仍不显示，**首先怀疑这里与新版实现已不同步**。
export default function pgyerIcon (data) {
  const [name, size, path0, path1] = data
  return {
    prefix: 'pg',
    iconName: name,
    icon: [size[0], size[1], [], 'e000', [path1, path0]]
  }
}
