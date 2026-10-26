/**
 * 把 `@pgyer/icons` 的图标数据本地化到 `web/src/assets/icons/`（一次性工具）。
 *
 * 为什么这么做（AGENTS §9 的图标坑 + 旧版体积问题）：
 *   - 上游 `icons/*.js` 的每个模块只导出原始数组 `[name, [w,h], path0, path1]`；
 *   - 上游 barrel（`index.js`）一次性 import 全部 154 个图标并逐个包装成
 *     FontAwesome 结构，导致 154/154 全部打包（实际只用到几十个），
 *     而且必须连带引入 `@fortawesome/*` 两个包；
 *   - 新前端不需要 FontAwesome：直接把这些路径数据渲染成 `<svg><path/></svg>`
 *     （见 `components/unit/Icon.tsx`），按需 import，永不打包未使用的图标。
 *
 * 用法：node scripts/extract-icons.mjs
 */
import { readFileSync, writeFileSync, mkdirSync, readdirSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const repoRoot = resolve(here, '../..')
const srcDir = resolve(repoRoot, 'www/view/node_modules/@pgyer/icons/icons')
const outDir = resolve(here, '../src/assets/icons')
const pkg = JSON.parse(readFileSync(resolve(repoRoot, 'www/view/node_modules/@pgyer/icons/package.json'), 'utf8'))

mkdirSync(outDir, { recursive: true })

let count = 0
const skipped = []

for (const file of readdirSync(srcDir).sort()) {
  if (!file.endsWith('.js')) continue
  const text = readFileSync(resolve(srcDir, file), 'utf8')

  const name = text.match(/const name = '([^']+)'/)?.[1]
  const size = text.match(/const size = \[(\d+),\s*(\d+)\]/)
  const path0 = text.match(/const path0 = '((?:[^'\\]|\\.)*)'/)?.[1] ?? ''
  const path1 = text.match(/const path1 = '((?:[^'\\]|\\.)*)'/)?.[1] ?? ''

  if (!name || !size) {
    skipped.push(file)
    continue
  }

  const base = file.replace(/\.js$/, '')
  const body = `/**
 * 图标 \`${name}\`（${size[1]}×${size[2]}）。
 *
 * 自动生成，请勿手改：来源 @pgyer/icons@${pkg.version} 的 icons/${file}。
 * 重新生成：cd web && node scripts/extract-icons.mjs
 */
const data = {
  name: ${JSON.stringify(name)},
  width: ${size[1]},
  height: ${size[2]},
  paths: [${JSON.stringify(path0)}, ${JSON.stringify(path1)}],
} as const

export default data
`
  writeFileSync(resolve(outDir, `${base}.ts`), body, 'utf8')
  count += 1
}

console.log(`已生成 ${count} 个图标模块${skipped.length ? `，跳过 ${skipped.length} 个（${skipped.join(', ')}）` : ''}`)
