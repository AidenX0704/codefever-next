#!/usr/bin/env node
/**
 * 设计体系「真机」审计：对着**真实运行中的实例**逐页跑对比度 + 溢出检查。
 *
 * 与 `check-ui.mjs` 的分工：
 *   - `check-ui.mjs`（静态）看源码：硬编码色值、原始层外泄、主题键一致、
 *     **同一条规则内**同时声明前景与背景的对比度。快、无需环境，进必过门禁。
 *   - 本脚本（动态）看渲染结果：逐个**可见文本节点**向上找真实背景再算对比度。
 *     能覆盖静态扫描的盲区 —— 基类给 color、变体类给 background 这种跨规则组合。
 *     需要跑着的实例，所以是**可选**检查，不进必过门禁。
 *
 * 用法：
 *   node scripts/check-ui-live.mjs --url http://127.0.0.1:8099 \
 *        --login root@codefever.cn:123456 \
 *        --paths repositories,home,demo1/111222/files
 *
 * 注意：--paths 写成**不带前导斜杠**的形式。Git Bash / MSYS 会把 `/foo` 这种参数
 * 自动转换成 Windows 路径（`C:/Program Files/Git/foo`），脚本拿到就错了。
 *
 * 前置：实例已跑起来（`pwsh -File scripts/run-local.ps1 up`），且前端产物是最新的
 * （`npm run build`）。脚本自己会拉起 headless Chrome，不需要手动开调试端口。
 */

import { spawn } from 'node:child_process'
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'

/* ---------------- 参数 ---------------- */
const argv = process.argv.slice(2)
const arg = (name, fallback) => {
  const i = argv.indexOf('--' + name)
  return i >= 0 && argv[i + 1] ? argv[i + 1] : fallback
}
const BASE = arg('url', 'http://127.0.0.1:8099').replace(/\/$/, '')
const LOGIN = arg('login', '')
const PATHS = arg('paths', 'repositories,home,mergerequests,groups')
  .split(',')
  .filter(Boolean)
  .map((p) => (p.startsWith('/') ? p : '/' + p))
const PORT = Number(arg('port', '9500'))
const CHROME = arg('chrome', '')

/* ---------------- 找 Chrome ---------------- */
const CANDIDATES = [
  CHROME,
  process.env.CHROME_PATH,
  'C:/Program Files/Google/Chrome/Application/chrome.exe',
  'C:/Program Files (x86)/Google/Chrome/Application/chrome.exe',
  'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',
  '/usr/bin/google-chrome',
  '/usr/bin/chromium',
  '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
].filter(Boolean)
const chromePath = CANDIDATES.find((p) => fs.existsSync(p))
if (!chromePath) {
  console.error('✗ 找不到 Chrome。用 --chrome <路径> 或设置 CHROME_PATH。')
  process.exit(2)
}

/* ---------------- 拉起浏览器 ---------------- */
const profile = fs.mkdtempSync(path.join(os.tmpdir(), 'cf-ui-audit-'))
const child = spawn(
  chromePath,
  [
    '--headless=new',
    '--disable-gpu',
    '--hide-scrollbars',
    `--remote-debugging-port=${PORT}`,
    `--user-data-dir=${profile}`,
    '--window-size=1440,1000',
    'about:blank',
  ],
  { stdio: 'ignore' },
)

const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

async function findTarget() {
  for (let i = 0; i < 60; i++) {
    try {
      const list = await (await fetch(`http://127.0.0.1:${PORT}/json/list`)).json()
      const page = list.find((t) => t.type === 'page' && t.webSocketDebuggerUrl)
      if (page) return page
    } catch {
      /* 还没起来 */
    }
    await sleep(250)
  }
  throw new Error('浏览器调试端口未就绪')
}

/* ---------------- CDP ---------------- */
const target = await findTarget()
const ws = new WebSocket(target.webSocketDebuggerUrl)
let seq = 0
const pending = new Map()
const errors = []
ws.addEventListener('message', (event) => {
  const msg = JSON.parse(event.data)
  if (msg.id && pending.has(msg.id)) {
    pending.get(msg.id)(msg)
    pending.delete(msg.id)
    return
  }
  if (msg.method === 'Runtime.exceptionThrown') {
    errors.push((msg.params.exceptionDetails?.exception?.description || '').slice(0, 120))
  }
})
await new Promise((resolve) => ws.addEventListener('open', resolve))
const send = (method, params) => new Promise((resolve) => {
  const id = ++seq
  pending.set(id, resolve)
  ws.send(JSON.stringify({ id, method, params }))
})
const evaluate = async (expression) => {
  const r = await send('Runtime.evaluate', { expression, returnByValue: true, awaitPromise: true })
  return r.result?.result?.value ?? r.result?.exceptionDetails?.text
}
const navigate = async (p) => {
  await send('Page.navigate', { url: BASE + p })
  await sleep(2200)
}

await send('Runtime.enable')
await send('Page.enable')

/* ---------------- 审计表达式 ---------------- */
const AUDIT = `(() => {
  const parse = (s) => { const m = s.match(/[\\d.]+/g); return m ? m.map(Number) : null; };
  const lum = (rgb) => { const a = rgb.slice(0,3).map(c => { c/=255; return c <= 0.03928 ? c/12.92 : Math.pow((c+0.055)/1.055, 2.4); }); return 0.2126*a[0]+0.7152*a[1]+0.0722*a[2]; };
  const blend = (fg, bg) => [0,1,2].map(i => fg[i]*fg[3] + bg[i]*(1-fg[3]));
  const bgOf = (el) => {
    let n = el;
    while (n && n !== document.documentElement) {
      const c = parse(getComputedStyle(n).backgroundColor);
      if (c && (c.length < 4 || c[3] > 0.95)) return c.slice(0,3);
      n = n.parentElement;
    }
    const b = parse(getComputedStyle(document.body).backgroundColor);
    return b ? b.slice(0,3) : [255,255,255];
  };
  const bad = []; const seen = new Set();
  document.querySelectorAll('body *').forEach((el) => {
    if (el.children.length > 0) return;
    const text = (el.textContent || '').trim();
    if (!text || text.length > 40) return;
    const rect = el.getBoundingClientRect();
    if (rect.width < 2 || rect.height < 2) return;
    const cs = getComputedStyle(el);
    if (cs.visibility === 'hidden' || cs.opacity === '0') return;
    const fgRaw = parse(cs.color); if (!fgRaw) return;
    const bg = bgOf(el);
    const fg = fgRaw.length > 3 ? blend(fgRaw, bg) : fgRaw.slice(0,3);
    const l1 = lum(fg), l2 = lum(bg);
    const ratio = +(((Math.max(l1,l2)+0.05)/(Math.min(l1,l2)+0.05)).toFixed(2));
    const size = parseFloat(cs.fontSize), weight = Number(cs.fontWeight) || 400;
    // WCAG「大字号」= ≥24px 常规，或 ≥18.66px 粗体。别把 14px 粗体也当大字，
    // 那会把阈值从 4.5 放水到 3，真正的低对比就漏了。
    const large = size >= 24 || (size >= 18.66 && weight >= 700);
    const need = large ? 3 : 4.5;
    // v5 §3.1 已定取舍：主题蓝实心（rgb(22,119,255)）上的白字是既定按钮/角标规格
    // （≥14px/500），对比度 4.1:1。与 check-ui.mjs 的 BRAND_PAIR_ALLOW 同口径，
    // 仅豁免这一对固定组合；新增豁免必须先写入 DESIGN.md 再同步两处。
    const brandPair = Math.abs(l2 - 0.2055) < 0.02 && l1 > 0.85 && ratio >= 3.9;
    if (ratio < need && !brandPair) {
      const key = String(el.className) + '|' + Math.round(ratio*10) + '|' + text.slice(0,8);
      if (seen.has(key)) return;
      seen.add(key);
      bad.push({ cls: String(el.className).slice(0,36) || el.tagName.toLowerCase(), text: text.slice(0,16), ratio, need });
    }
  });
  return JSON.stringify({
    failures: bad.slice(0, 8),
    overflow: document.documentElement.scrollWidth > window.innerWidth + 1,
    path: location.pathname,
  });
})()`

/* ---------------- 登录 ---------------- */
if (LOGIN) {
  const [email, password] = LOGIN.split(':')
  await navigate('/user/login')
  const code = await evaluate(
    `fetch('/user/login',{method:'POST',headers:{'Content-Type':'application/x-www-form-urlencoded'},` +
      `body:'email='+encodeURIComponent(${JSON.stringify(email)})+'&password='+encodeURIComponent(${JSON.stringify(password)}),` +
      `credentials:'include'}).then(r=>r.json()).then(j=>j.code)`,
  )
  if (code !== 0) {
    console.error(`✗ 登录失败（code=${code}）。检查 --login 的邮箱与口令。`)
    child.kill()
    process.exit(2)
  }
  console.log('登录成功\n')
}

/* ---------------- 逐页 × 双主题 ---------------- */
let failed = 0
for (const dark of [false, true]) {
  await evaluate(dark ? `localStorage.setItem('cf-theme','dark');1` : `localStorage.removeItem('cf-theme');1`)
  for (const p of PATHS) {
    await navigate(p)
    const raw = await evaluate(AUDIT)
    let report
    try {
      report = JSON.parse(raw)
    } catch {
      console.error(`✗ ${p}: 审计脚本执行失败 — ${raw}`)
      failed += 1
      continue
    }
    const theme = dark ? '深色' : '浅色'
    if (report.failures.length === 0 && !report.overflow) {
      console.log(`✓ ${theme} ${p}`)
      continue
    }
    failed += 1
    console.log(`✗ ${theme} ${p}${report.overflow ? '  （横向溢出）' : ''}`)
    for (const f of report.failures) {
      console.log(`    ${String(f.ratio).padStart(5)}:1 (需 ${f.need})  ${f.cls}  «${f.text}»`)
    }
  }
}
await evaluate(`localStorage.removeItem('cf-theme');1`)

if (errors.length) {
  console.log('\n控制台异常：')
  for (const e of [...new Set(errors)].slice(0, 5)) console.log('  ' + e)
}

ws.close()
child.kill()
// Chrome 退出前可能仍占着 profile 目录；清理失败不影响检查结论。
await sleep(300)
try {
  fs.rmSync(profile, { recursive: true, force: true })
} catch {
  /* 留给系统临时目录回收 */
}

console.log(
  failed === 0
    ? `\n✓ 真机审计通过（${PATHS.length} 个页面 × 浅/深 = ${PATHS.length * 2} 组）`
    : `\n✗ 真机审计失败：${failed} 组有问题`,
)
process.exit(failed === 0 ? 0 : 1)
