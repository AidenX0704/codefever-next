/**
 * 生成前端 i18n 词表（一次性工具，批 7 删除旧前端后退役）。
 *
 * 两个来源合并，**逐字搬运**，不做任何改写：
 *   1. Go 侧 SSR 页面文案：`backend/internal/httpapi/pages.go` 的 `langZhCN` / `langEnglish`；
 *   2. 旧 SPA 词表：`www/view/src/lang/{zh-cn,en-us}/*.js`。
 *
 * 第 2 类是**运行时拼词**的实现（例如 `newRepository: [Phrase.new, Term.repository].join('')`），
 * 静态正则解析会漏掉这些计算值，所以这里把 lang 目录复制到临时目录、把 `APPSRC/...` 导入
 * 改写成相对路径，然后真正 `import()` 执行一遍，再按旧版 `composeNamespacedData` 的规则
 * 加前缀（`label.` / `message.` / `menu.` …）。
 *
 * 冲突时 **SPA 词表优先**（业务页面是它的原生语境）。
 *
 * 用法：node scripts/extract-lang.mjs
 */
import { readFileSync, writeFileSync, mkdirSync, readdirSync, rmSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath, pathToFileURL } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))
const repoRoot = resolve(here, '../..')
const outDir = resolve(here, '../src/i18n')
const tmpRoot = resolve(here, '../.tmp-lang')
const SPA_LANG_DIR = resolve(repoRoot, 'www/view/src/lang')

/* ------------------------------ 1. Go 侧 ------------------------------ */

const goSource = readFileSync(resolve(repoRoot, 'backend/internal/httpapi/pages.go'), 'utf8')

function goBlock(name) {
  const start = goSource.indexOf(`var ${name} = Translations{`)
  if (start < 0) throw new Error(`找不到变量 ${name}`)
  return goSource.slice(start, goSource.indexOf('\n}', start))
}

function unescapeGo(raw) {
  return raw.replace(/\\(u[0-9a-fA-F]{4}|.)/g, (_, seq) => {
    if (seq[0] === 'u') return String.fromCharCode(parseInt(seq.slice(1), 16))
    if (seq === 'n') return '\n'
    if (seq === 't') return '\t'
    if (seq === 'r') return '\r'
    return seq
  })
}

function parseGo(text) {
  const entries = new Map()
  const lineRe = /^\s*"((?:[^"\\]|\\.)*)":\s*"((?:[^"\\]|\\.)*)",?\s*$/gm
  let m
  while ((m = lineRe.exec(text)) !== null) entries.set(unescapeGo(m[1]), unescapeGo(m[2]))
  return entries
}

const goZh = parseGo(goBlock('langZhCN'))
const goEn = parseGo(goBlock('langEnglish'))

/* ------------------------------ 2. 旧 SPA（真正执行模块） ------------------------------ */

/** 把 lang 目录复制到临时目录，并把 APPSRC 导入改写为相对路径。 */
function stageLocale(locale) {
  const srcDir = resolve(SPA_LANG_DIR, locale)
  const dstDir = resolve(tmpRoot, locale)
  mkdirSync(dstDir, { recursive: true })

  // Lang.js 只提供语言名，作为命名空间 'lang' 参与合成。
  const langFile = resolve(SPA_LANG_DIR, 'Lang.js')
  writeFileSync(
    resolve(dstDir, 'Lang.js'),
    readFileSync(langFile, 'utf8').replace(/from 'APPSRC\/lang\/([A-Za-z]+)'/g, "from './$1.js'"),
    'utf8',
  )

  for (const name of readdirSync(srcDir)) {
    if (!name.endsWith('.js')) continue
    const text = readFileSync(resolve(srcDir, name), 'utf8')
    const rewritten = text.replace(/from 'APPSRC\/lang\/[a-z-]+\/([A-Za-z]+)'/g, "from './$1.js'")
    writeFileSync(resolve(dstDir, name), rewritten, 'utf8')
  }
  return dstDir
}

/** 复刻旧版 composeNamespacedData：带 `__namespace__` 的对象展开成 `ns.key`。 */
function composeNamespaced(modules) {
  const out = {}
  for (const mod of modules) {
    if (!mod) continue
    if (mod.__namespace__) {
      for (const [key, value] of Object.entries(mod)) {
        if (key === '__namespace__') continue
        out[`${mod.__namespace__}.${key}`] = value
      }
    } else {
      Object.assign(out, mod)
    }
  }
  return out
}

const SPA_MODULE_ORDER = [
  'Lang',
  'Message',
  'ActivityMessage',
  'ErrorMessage',
  'Term',
  'Phrase',
  'Menu',
  'Label',
  'Notification',
]

async function loadSpaLocale(locale) {
  const dir = stageLocale(locale)
  const modules = []
  for (const name of SPA_MODULE_ORDER) {
    const file = resolve(dir, `${name}.js`)
    try {
      const imported = await import(pathToFileURL(file).href)
      modules.push(imported.default)
    } catch (error) {
      console.warn(`跳过 ${locale}/${name}.js：${error.message}`)
    }
  }
  const composed = composeNamespaced(modules)
  const map = new Map()
  for (const [key, value] of Object.entries(composed)) {
    if (typeof value === 'string') map.set(key, value)
  }
  return map
}

const spaZh = await loadSpaLocale('zh-cn')
const spaEn = await loadSpaLocale('en-us')
rmSync(tmpRoot, { recursive: true, force: true })

/* ------------------------------ 3. 合并输出 ------------------------------ */

function merge(...maps) {
  const out = new Map()
  for (const map of maps) for (const [k, v] of map) out.set(k, v)
  return out
}

const zh = merge(goZh, spaZh)
const en = merge(goEn, spaEn)
const keys = [...new Set([...zh.keys(), ...en.keys()])].sort()

function emit(entries, title, goVar, locale) {
  const lines = keys.map((key) => `  ${JSON.stringify(key)}: ${JSON.stringify(entries.get(key) ?? '')},`)
  return `/**
 * ${title} 词表。
 *
 * ⚠️ 自动生成，请勿手改：来源
 *   - backend/internal/httpapi/pages.go 的 ${goVar}（SSR 页面文案）
 *   - www/view/src/lang/${locale}/*.js（业务页面文案，含 {s}/{n} 占位符）
 * 重新生成：cd web && node scripts/extract-lang.mjs
 */
export const messages = {
${lines.join('\n')}
} as const
`
}

mkdirSync(outDir, { recursive: true })
writeFileSync(resolve(outDir, 'zh-cn.ts'), emit(zh, '简体中文', 'langZhCN', 'zh-cn'), 'utf8')
writeFileSync(resolve(outDir, 'en-us.ts'), emit(en, 'English', 'langEnglish', 'en-us'), 'utf8')

const onlyZh = keys.filter((k) => zh.has(k) && !en.has(k))
console.log(`合并后键总数 ${keys.length}（go ${goZh.size}/${goEn.size}，spa ${spaZh.size}/${spaEn.size}）`)
if (onlyZh.length) console.log(`仅 zh 有值：${onlyZh.length} 个 → ${onlyZh.slice(0, 10).join(', ')}`)
