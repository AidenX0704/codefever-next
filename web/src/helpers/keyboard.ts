import type { KeyboardEvent } from 'react'

/**
 * 让「用 div 模拟按钮」的元素支持键盘激活。
 *
 * 背景（V3 §10 / AGENTS §4）：列表行、卡片这类可点击容器常用
 * `<div role="button" tabIndex={0} onClick={...}>`。这样写**只解决了一半**：
 * 能 Tab 到、读屏也会念成按钮，但按 Enter / Space 什么都不会发生 ——
 * 键盘用户实际上点不进去。这个坑在仓库行、团队卡、帮助页入口三处都出现过，
 * 所以抽成一处，别再各写一遍。
 *
 * 用法：
 * ```tsx
 * <div role="button" tabIndex={0} onClick={open} onKeyDown={activateOnKey(open)}>
 * ```
 */
export function activateOnKey(action: () => void) {
  return (event: KeyboardEvent<HTMLElement>) => {
    if (event.key !== 'Enter' && event.key !== ' ') return
    // Space 默认会滚动页面，Enter 在某些容器里会触发外层表单提交
    event.preventDefault()
    action()
  }
}
