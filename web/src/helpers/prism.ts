/**
 * Prism 代码高亮（按需注册语言 + 结果缓存）。
 *
 * 与旧版的差异只在"怎么把语言装进来"：
 *   旧版用 webpack 的 `babel-plugin-prismjs` 在编译期注入 36 种语言，
 *   Vite 没有等价插件，改为在这里显式 `import`（同样是 36 种，来源见
 *   `config/languages.ts` 的注释与旧 `config/webpack.config.js:450-459`）。
 *   少引一种只会导致该语言回落到 markdown 高亮，不会报错 —— 所以这份清单
 *   必须与 `EXTENSION_LANGUAGES` 的取值集合**保持一致**。
 *
 * 主题：`prism-coy`（旧版 `babel-plugin-prismjs` 的 `theme: 'coy'`）。
 * 该主题的色值是第三方固定值，集中在本文件导入，页面不得自绘代码配色（AGENTS §5 例外条款）。
 */
import Prism from 'prismjs'

import 'prismjs/themes/prism-coy.css'

// 基础文法与依赖顺序：clike 是 javascript/java 等的前置，markup/css 相互独立。
import 'prismjs/components/prism-markup'
import 'prismjs/components/prism-css'
import 'prismjs/components/prism-clike'
import 'prismjs/components/prism-javascript'
import 'prismjs/components/prism-typescript'
import 'prismjs/components/prism-jsx'
import 'prismjs/components/prism-tsx' // 依赖 jsx + typescript，顺序不能提前
import 'prismjs/components/prism-json'
import 'prismjs/components/prism-yaml'
import 'prismjs/components/prism-bash'
import 'prismjs/components/prism-diff'
import 'prismjs/components/prism-c'
import 'prismjs/components/prism-cpp'
import 'prismjs/components/prism-csharp'
import 'prismjs/components/prism-go'
import 'prismjs/components/prism-graphql'
import 'prismjs/components/prism-ini'
import 'prismjs/components/prism-java'
import 'prismjs/components/prism-kotlin'
import 'prismjs/components/prism-less'
import 'prismjs/components/prism-lua'
import 'prismjs/components/prism-markdown'
import 'prismjs/components/prism-objectivec'
import 'prismjs/components/prism-pascal'
import 'prismjs/components/prism-perl'
import 'prismjs/components/prism-markup-templating' // php 的前置文法
import 'prismjs/components/prism-php'
import 'prismjs/components/prism-powershell'
import 'prismjs/components/prism-python'
import 'prismjs/components/prism-r'
import 'prismjs/components/prism-ruby'
import 'prismjs/components/prism-rust'
import 'prismjs/components/prism-scss'
import 'prismjs/components/prism-sql'
import 'prismjs/components/prism-swift'
import 'prismjs/components/prism-basic' // vbnet 的前置文法
import 'prismjs/components/prism-vbnet'
import 'prismjs/components/prism-batch'
import 'prismjs/components/prism-clojure'
import 'prismjs/components/prism-coffeescript'

import { languageForPath } from '@/config/languages'

/** 高亮结果缓存（与旧版同样的上界 8，避免长时间浏览后无限增长）。 */
const CACHE_MAX = 8
const cache = new Map<string, string[]>()

/**
 * 把整段代码高亮成**按行**的 HTML 数组。
 *
 * 旧版 `CodeViewer` 的做法是"整文件高亮后按 \n 切行"（性能整改 F5 的缓存也按
 * 「语言 + 全文」做 key）；这里保持一致，避免多行 token（如块注释、模板字符串）
 * 被逐行处理打断而产生错误的 HTML。
 */
export function highlightLines(raw: string, path: string | null | undefined, forcedLang?: string): string[] {
  const lang = forcedLang ?? languageForPath(path)
  const key = `${lang}|${raw}`
  const hit = cache.get(key)
  if (hit) {
    return hit
  }

  const grammar = Prism.languages[lang] ?? Prism.languages.markdown
  const html = Prism.highlight(raw, grammar, lang)
  const lines = html.split('\n')

  if (cache.size >= CACHE_MAX) {
    const oldest = cache.keys().next().value
    if (oldest !== undefined) {
      cache.delete(oldest)
    }
  }
  cache.set(key, lines)
  return lines
}
