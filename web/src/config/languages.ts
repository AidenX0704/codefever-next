/**
 * 代码高亮语言映射。
 *
 * 逐字移植旧版 `www/view/src/config/Constants.js` 的 `codeFileExtentsions`
 * 与 `helpers/VaribleHelper.js` 的 `getCodeLanguageType`：
 *   - 扩展名未命中 → 回落到 `markdown`（旧版行为，保持不变）；
 *   - 旧表里的值有 5 个不是 Prism 的真实语言 id（旧版靠 webpack 的
 *     `babel-plugin-prismjs` 做映射），这里直接写 Prism id 并注明对应关系：
 *       html/xml → markup、mysql/pgsql → sql、vb → vbnet、
 *       objective-c → objectivec、coffee → coffeescript。
 */
export const EXTENSION_LANGUAGES: Readonly<Record<string, string>> = {
  bat: 'batch',
  clj: 'clojure',
  cljs: 'clojure',
  cljn: 'clojure',
  cljc: 'clojure',
  cljx: 'clojure',
  coffee: 'coffeescript',
  litcoffee: 'coffeescript',
  c: 'cpp',
  h: 'cpp',
  cpp: 'cpp',
  cs: 'csharp',
  css: 'css',
  go: 'go',
  graphql: 'graphql',
  htm: 'markup',
  xhtm: 'markup',
  html: 'markup',
  xhtml: 'markup',
  ini: 'ini',
  java: 'java',
  js: 'javascript',
  jsx: 'javascript',
  es: 'javascript',
  jsm: 'javascript',
  esm: 'javascript',
  json: 'json',
  kt: 'kotlin',
  less: 'less',
  lua: 'lua',
  md: 'markdown',
  mysql: 'sql',
  m: 'objectivec',
  pas: 'pascal',
  perl: 'perl',
  pgsql: 'sql',
  php: 'php',
  xhp: 'php',
  php3: 'php',
  php4: 'php',
  ps: 'powershell',
  py: 'python',
  r: 'r',
  rb: 'ruby',
  rust: 'rust',
  scss: 'scss',
  sh: 'bash',
  sql: 'sql',
  swift: 'swift',
  ts: 'typescript',
  tsx: 'typescript',
  tsm: 'typescript',
  vb: 'vbnet',
  xml: 'markup',
  yml: 'yaml',
}

/** 文件路径 → Prism 语言 id（未命中回落 markdown，与旧版一致）。 */
export function languageForPath(path: string | undefined | null): string {
  const ext = path?.split('.').pop()?.toLowerCase()
  return (ext && EXTENSION_LANGUAGES[ext]) || 'markdown'
}
