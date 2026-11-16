#!/usr/bin/env node
/**
 * 设计体系静态检查（AGENTS §5 / §8）。
 *
 * 跑法：`npm run check:ui`（在 `web/` 下）。CI 与本地门禁同一条命令。
 *
 * 四条检查，按拦截收益排序：
 *   1. 硬编码色值   —— CSS 里出现 #hex / rgb() / rgba()，或 TSX 里出现 6 位 hex
 *   2. 原始层外泄   —— 非 theme.css 的文件引用 var(--p-*)
 *   3. 主题键一致   —— theme.css 浅色 / 深色语义块的键集合必须完全相同
 *   4. 文字对比度   —— 同一条规则内同时声明前景与背景时，对比度必须 ≥ 4.5:1
 *
 * 白名单（文件级）：
 *   - theme/theme.css            令牌定义本身
 *   - styles/markdown-body.css   第三方 vendored 副本（GitHub README 样式）
 *   - styles/markdown-body.dark.css  上面那份的深色覆盖层（需写具体值）
 *
 * 为什么要有第 4 条：2026-10 的令牌改名把「浅琥珀」错映射成「实心琥珀」，
 * 让 .cf-tag--warning 变成 1.41:1 —— 深琥珀字压琥珀底，肉眼几乎读不出来，
 * 但 typecheck / lint / build 全绿。这类问题只能靠算对比度抓。
 */

import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const SRC = path.resolve(HERE, '..', 'src');

const ALLOW_COLOR_LITERALS = new Set([
  'theme/theme.css',
  'styles/markdown-body.css',
  'styles/markdown-body.dark.css',
]);
const ALLOW_PRIMITIVE_REFS = new Set(['theme/theme.css']);

const rel = (p) => path.relative(SRC, p).split(path.sep).join('/');

function walk(dir, out = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, entry.name);
    if (entry.isDirectory()) walk(p, out);
    else if (/\.(css|tsx?)$/.test(entry.name)) out.push(p);
  }
  return out;
}

const files = walk(SRC);
const problems = [];

/* ---------------- 令牌解析（浅色为默认作用域） ---------------- */
const themeCss = fs.readFileSync(path.join(SRC, 'theme/theme.css'), 'utf8').replace(/\r\n/g, '\n');
const TOKENS = new Map();
{
  const darkAt = themeCss.indexOf('3. 语义层 · 深色');
  const head = themeCss.slice(0, darkAt);
  for (const m of head.matchAll(/(--[a-z0-9-]+)\s*:\s*([^;]+);/g)) TOKENS.set(m[1], m[2].trim());
}
function resolveVar(value, depth = 0) {
  if (depth > 8) return value;
  const m = /var\(\s*(--[a-z0-9-]+)\s*\)/.exec(value);
  if (!m) return value;
  const next = TOKENS.get(m[1]);
  return next === undefined ? value : resolveVar(value.replace(m[0], next), depth + 1);
}
function toRgb(str) {
  const s = resolveVar(str).trim();
  if (s.startsWith('#')) {
    let h = s.slice(1);
    if (h.length === 3) h = h.split('').map((c) => c + c).join('');
    if (h.length !== 6) return null;
    return [parseInt(h.slice(0, 2), 16), parseInt(h.slice(2, 4), 16), parseInt(h.slice(4, 6), 16), 1];
  }
  const m = s.match(/rgba?\(([^)]+)\)/);
  if (!m) return null;
  const parts = m[1].split(/[,\s/]+/).filter(Boolean).map(Number);
  if (parts.length < 3 || parts.some((n) => Number.isNaN(n))) return null;
  return [parts[0], parts[1], parts[2], parts.length > 3 ? parts[3] : 1];
}
const over = (fg, bg) => [0, 1, 2].map((i) => Math.round(fg[i] * fg[3] + bg[i] * (1 - fg[3])));
function luminance(rgb) {
  const a = rgb.slice(0, 3).map((c) => {
    const v = c / 255;
    return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4);
  });
  return 0.2126 * a[0] + 0.7152 * a[1] + 0.0722 * a[2];
}
function contrastOf(fgStr, bgStr) {
  const fg = toRgb(fgStr);
  let bg = toRgb(bgStr);
  if (!fg || !bg) return null;
  if (bg[3] < 1) {
    const base = toRgb('var(--bg-surface)') || [255, 255, 255, 1];
    bg = [...over(bg, base), 1];
  }
  if (fg[3] < 1) {
    const flat = over(fg, bg);
    const l1 = luminance(flat);
    const l2 = luminance(bg);
    return +(((Math.max(l1, l2) + 0.05) / (Math.min(l1, l2) + 0.05)).toFixed(2));
  }
  const l1 = luminance(fg);
  const l2 = luminance(bg);
  return +(((Math.max(l1, l2) + 0.05) / (Math.min(l1, l2) + 0.05)).toFixed(2));
}

/* v5 §3.1 已定取舍：主题蓝实心（#1677ff）上的白字/白图标是既定按钮与角标规格
   （按钮文字 ≥14px/500），对比度 4.1:1。仅豁免这一对固定组合（两种主题同值），
   其余组合仍按 4.5:1 执行；新增豁免必须先写入 DESIGN.md 再改这里。 */
const BRAND_PAIR_ALLOW = new Set(['#ffffff/#1677ff']);
const toHexKey = (rgb) =>
  '#' + rgb.slice(0, 3).map((c) => c.toString(16).padStart(2, '0')).join('');

/* ---------------- 检查 1 / 2：色值字面值与原始层引用 ---------------- */
const HEX_OR_RGB = /#[0-9a-fA-F]{3,8}\b|\brgba?\s*\(/g;
const HEX6 = /#[0-9a-fA-F]{6}\b/g;

for (const file of files) {
  const r = rel(file);
  const raw = fs.readFileSync(file, 'utf8').replace(/\r\n/g, '\n');
  // 去掉注释后再判断，避免文档性说明误报
  const code = r.endsWith('.css') ? raw.replace(/\/\*[\s\S]*?\*\//g, '') : raw.replace(/\/\*[\s\S]*?\*\/|\/\/[^\n]*/g, '');

  if (!ALLOW_COLOR_LITERALS.has(r)) {
    const re = r.endsWith('.css') ? HEX_OR_RGB : HEX6;
    re.lastIndex = 0;
    let m;
    while ((m = re.exec(code))) {
      const line = code.slice(0, m.index).split('\n').length;
      problems.push(`${r}:${line}  硬编码色值 \`${m[0]}\` —— 改用语义令牌（AGENTS §5.4）`);
    }
  }

  if (!ALLOW_PRIMITIVE_REFS.has(r)) {
    const re = /var\(\s*(--p-[a-z0-9-]+)/g;
    let m;
    while ((m = re.exec(code))) {
      const line = code.slice(0, m.index).split('\n').length;
      problems.push(`${r}:${line}  引用了原始层 \`${m[1]}\` —— 组件与页面只能引用语义层 / 尺寸层（AGENTS §5.3）`);
    }
  }
}

/* ---------------- 检查 3：浅色 / 深色语义键一致 ---------------- */
{
  const keySet = (start, end) => {
    const i = themeCss.indexOf(start);
    const j = end ? themeCss.indexOf(end, i + 1) : themeCss.length;
    return new Set([...themeCss.slice(i, j).matchAll(/(--[a-z0-9-]+)\s*:/g)].map((m) => m[1]));
  };
  const light = keySet('2. 语义层 · 浅色', '3. 语义层 · 深色');
  const dark = keySet('3. 语义层 · 深色', '4. 尺寸层');
  for (const k of light) if (!dark.has(k)) problems.push(`theme/theme.css  深色缺键 \`${k}\` —— 切主题时这一处会静默回退`);
  for (const k of dark) if (!light.has(k)) problems.push(`theme/theme.css  浅色缺键 \`${k}\` —— 浅色下该令牌未定义`);
}

/* ---------------- 检查 4：文字 / 背景对比度 ---------------- */
for (const file of files) {
  const r = rel(file);
  if (!r.endsWith('.css') || r.startsWith('styles/markdown-body')) continue;
  const raw = fs.readFileSync(file, 'utf8').replace(/\r\n/g, '\n');
  const code = raw.replace(/\/\*[\s\S]*?\*\//g, (m) => m.replace(/[^\n]/g, ' '));
  const ruleRe = /([^{}]+)\{([^{}]*)\}/g;
  let m;
  while ((m = ruleRe.exec(code))) {
    const selector = m[1].trim().replace(/\s+/g, ' ');
    if (selector.startsWith('@')) continue;
    const decls = m[2];
    const bgM = /(?:^|;)\s*background(?:-color)?\s*:\s*([^;]+)/.exec(decls);
    const fgM = /(?:^|;)\s*color\s*:\s*([^;]+)/.exec(decls);
    if (!bgM || !fgM) continue;
    const bg = bgM[1].trim();
    const fg = fgM[1].trim();
    if (/gradient|transparent|none|inherit|currentColor/.test(bg + fg)) continue;
    const fgRgb = toRgb(fg);
    const bgRgb = toRgb(bg);
    if (
      fgRgb && bgRgb && fgRgb[3] === 1 && bgRgb[3] === 1 &&
      BRAND_PAIR_ALLOW.has(`${toHexKey(fgRgb)}/${toHexKey(bgRgb)}`)
    ) continue;
    const ratio = contrastOf(fg, bg);
    if (ratio === null || ratio >= 4.5) continue;
    const line = code.slice(0, m.index).split('\n').length;
    problems.push(`${r}:${line}  \`${selector.slice(0, 46)}\` 文字/背景 ${ratio}:1 < 4.5:1`);
  }
}

/* ---------------- 汇总 ---------------- */
if (problems.length === 0) {
  console.log('✓ 设计体系静态检查通过（色值字面值 / 原始层引用 / 主题键一致 / 对比度）');
  process.exit(0);
}
console.error(`✗ 设计体系静态检查发现 ${problems.length} 处问题：\n`);
for (const p of problems) console.error('  ' + p);
console.error('\n规则见 AGENTS.md §5；令牌基线见 docs/design-v5/DESIGN.md');
process.exit(1);
