// 多视口截图采集器（Chrome DevTools Protocol 直驱）
//
// 用途：把"Go 后端切流态"与"PHP 后端回滚态"在**同一组视口**下逐页截图，
// 再做逐字节比对 —— 用于验收"前端 UI 风格保持一致"这条硬约束。
//
// 为什么不用 in-app Browser：内置浏览器的视口由宿主 FilePanel 决定，
// 无法精确指定窄屏尺寸；而响应式验收必须固定视口才能做像素级比对。
//
// 用法：
//   node scripts/shot.mjs --base http://localhost:9080 --out .tmp-shots/go \
//        --cookie "<会话 Cookie 值>" [--cookieName codefever_community]
//
// 退出码：0 全部成功；1 有页面采集失败。

import { spawn } from 'node:child_process';
import { mkdir, writeFile, rm } from 'node:fs/promises';
import { existsSync } from 'node:fs';
import { setTimeout as sleep } from 'node:timers/promises';
import os from 'node:os';
import path from 'node:path';

const CHROME_CANDIDATES = [
  'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
  'C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe',
  'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe',
];

// 视口：[名称, 宽, 高]
const VIEWPORTS = [
  ['mobile', 390, 844],
  ['tablet', 768, 1024],
  ['desktop', 1440, 900],
];

// 页面：相对路径 + 稳定 slug
const PAGES = [
  ['/home', 'home'],
  ['/repositories', 'repositories'],
  ['/rootteam/demorepo', 'repo-detail'],
  ['/rootteam/demorepo/files/master', 'repo-files'],
  ['/rootteam/demorepo/commits/master', 'repo-commits'],
  ['/rootteam/demorepo/branches', 'repo-branches'],
  ['/admin', 'admin-dashboard'],
  ['/admin/users', 'admin-users'],
  ['/user/login', 'login-page'],
];

function parseArgs(argv) {
  const out = {
    base: 'http://localhost:9080',
    out: '.tmp-shots/out',
    cookie: '',
    cookieName: 'codefever_community',
    port: 9333,
    settle: 3500,
    chrome: '',
    loginPath: '',
    loginEmail: '',
    loginPassword: '',
  };
  for (let i = 2; i < argv.length; i++) {
    const k = argv[i];
    if (!k.startsWith('--')) continue;
    const key = k.slice(2);
    const val = argv[i + 1];
    if (val === undefined || val.startsWith('--')) continue;
    i++;
    if (key in out) out[key] = /^\d+$/.test(val) ? Number(val) : val;
  }
  return out;
}

class CDP {
  constructor(url) {
    this.ws = new WebSocket(url);
    this.id = 0;
    this.pending = new Map();
    this.listeners = [];
    this.ready = new Promise((resolve, reject) => {
      this.ws.addEventListener('open', () => resolve());
      this.ws.addEventListener('error', (e) => reject(new Error('ws error: ' + e.message)));
    });
    this.ws.addEventListener('message', (ev) => {
      const msg = JSON.parse(ev.data);
      if (msg.id !== undefined) {
        const p = this.pending.get(msg.id);
        if (!p) return;
        this.pending.delete(msg.id);
        if (msg.error) p.reject(new Error(JSON.stringify(msg.error)));
        else p.resolve(msg.result);
        return;
      }
      for (const l of this.listeners) l(msg);
    });
  }

  send(method, params = {}, sessionId) {
    const id = ++this.id;
    const payload = { id, method, params };
    if (sessionId) payload.sessionId = sessionId;
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
      this.ws.send(JSON.stringify(payload));
      setTimeout(() => {
        if (this.pending.has(id)) {
          this.pending.delete(id);
          reject(new Error(`CDP timeout: ${method}`));
        }
      }, 60000);
    });
  }

  on(fn) {
    this.listeners.push(fn);
  }

  close() {
    try { this.ws.close(); } catch { /* ignore */ }
  }
}

async function waitForHttp(url, timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try {
      const r = await fetch(url);
      if (r.ok) return await r.json();
    } catch { /* retry */ }
    await sleep(200);
  }
  throw new Error('CDP endpoint not ready: ' + url);
}

async function main() {
  const a = parseArgs(process.argv);
  const chromePath = a.chrome || CHROME_CANDIDATES.find((p) => existsSync(p));
  if (!chromePath) throw new Error('未找到 Chrome/Edge 可执行文件');

  await rm(a.out, { recursive: true, force: true });
  await mkdir(a.out, { recursive: true });
  const userDataDir = path.join(os.tmpdir(), 'cf-shot-' + Date.now());

  const child = spawn(chromePath, [
    '--headless=new',
    '--disable-gpu',
    '--no-first-run',
    '--no-default-browser-check',
    '--disable-extensions',
    '--disable-background-networking',
    '--force-device-scale-factor=1',
    '--hide-scrollbars',
    `--remote-debugging-port=${a.port}`,
    `--user-data-dir=${userDataDir}`,
    'about:blank',
  ], { stdio: 'ignore' });

  const report = { base: a.base, chrome: chromePath, viewports: [], pages: [], consoleErrors: [], failedRequests: [] };

  try {
    const version = await waitForHttp(`http://127.0.0.1:${a.port}/json/version`, 30000);
    const cdp = new CDP(version.webSocketDebuggerUrl);
    await cdp.ready;

    const { targetId } = await cdp.send('Target.createTarget', { url: 'about:blank' });
    const { sessionId } = await cdp.send('Target.attachToTarget', { targetId, flatten: true });

    await cdp.send('Page.enable', {}, sessionId);
    await cdp.send('Network.enable', {}, sessionId);
    await cdp.send('Runtime.enable', {}, sessionId);

    let currentPage = '';
    cdp.on((msg) => {
      if (msg.sessionId !== sessionId) return;
      if (msg.method === 'Runtime.consoleAPICalled' && msg.params.type === 'error') {
        const t = (msg.params.args || []).map((x) => x.value ?? x.description ?? '').join(' ');
        report.consoleErrors.push({ page: currentPage, text: t.slice(0, 300) });
      }
      if (msg.method === 'Runtime.exceptionThrown') {
        const d = msg.params.exceptionDetails || {};
        report.consoleErrors.push({
          page: currentPage,
          text: (d.exception?.description || d.text || 'exception').slice(0, 300),
        });
      }
      if (msg.method === 'Network.loadingFailed') {
        report.failedRequests.push({
          page: currentPage,
          error: msg.params.errorText,
          type: msg.params.type,
        });
      }
    });

    if (a.cookie) {
      const host = new URL(a.base).hostname;
      const r = await cdp.send('Network.setCookie', {
        name: a.cookieName,
        value: a.cookie,
        domain: host,
        path: '/',
        httpOnly: true,
      }, sessionId);
      if (!r.success) throw new Error('setCookie 失败');
    }

    // 在**同一个浏览器实例内**走真实登录表单。
    //
    // 原因：CodeIgniter 的 sess_match_useragent 默认开启，会话 Cookie 里绑定了
    // 登录时的 User-Agent。外部注入 Cookie（跨 UA）会让 PHP 侧判定会话失效并
    // 跳回登录页 —— 那不是后端差异，而是会话策略差异。Go 侧会话存服务端，
    // 不受此限制。
    if (a.loginPath) {
      const nav = (url) => {
        const loaded = new Promise((resolve) => {
          const h = (msg) => {
            if (msg.sessionId === sessionId && msg.method === 'Page.loadEventFired') resolve(true);
          };
          cdp.on(h);
          setTimeout(() => resolve(false), 30000);
        });
        return cdp.send('Page.navigate', { url }, sessionId).then(() => loaded);
      };

      await nav(a.base + a.loginPath);
      await sleep(1500);

      const fill = await cdp.send('Runtime.evaluate', {
        expression: `(() => {
          const set = (sel, val) => {
            const el = document.querySelector(sel);
            if (!el) return 'missing:' + sel;
            const proto = el instanceof HTMLTextAreaElement
              ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
            Object.getOwnPropertyDescriptor(proto, 'value').set.call(el, val);
            el.dispatchEvent(new Event('input', { bubbles: true }));
            el.dispatchEvent(new Event('change', { bubbles: true }));
            return 'ok';
          };
          return JSON.stringify({
            email: set('input[name="email"]', ${JSON.stringify(a.loginEmail)}),
            password: set('input[name="password"]', ${JSON.stringify(a.loginPassword)}),
            button: !!document.querySelector('#submitButton'),
          });
        })()`,
        returnByValue: true,
      }, sessionId);
      process.stdout.write('  填表：' + fill.result.value + '\n');

      const afterLogin = new Promise((resolve) => {
        const h = (msg) => {
          if (msg.sessionId === sessionId && msg.method === 'Page.loadEventFired') resolve(true);
        };
        cdp.on(h);
        setTimeout(() => resolve(false), 30000);
      });
      await cdp.send('Runtime.evaluate', {
        expression: `document.querySelector('#submitButton').click()`,
      }, sessionId);
      const logged = await afterLogin;
      await sleep(2000);

      const probeLogin = async () => {
        const probe = await cdp.send('Runtime.evaluate', {
          expression: `fetch('/api/user/info', { headers: { 'Accept': 'application/json' } })
            .then(r => r.json()).then(j => 'code=' + j.code)`,
          awaitPromise: true,
          returnByValue: true,
        }, sessionId);
        return probe.result.value;
      };

      let probe = await probeLogin();
      process.stdout.write('  表单提交后 /api/user/info -> ' + probe + '\n');

      if (!/code=0/.test(probe || '')) {
        // 回退：直接在页面上下文发一次等价的 AJAX 登录请求
        // （与登录页 jQuery 提交的请求完全同形：urlencoded + Accept: application/json）。
        const direct = await cdp.send('Runtime.evaluate', {
          expression: `fetch('/user/login', {
            method: 'POST',
            credentials: 'same-origin',
            headers: { 'Accept': 'application/json', 'Content-Type': 'application/x-www-form-urlencoded' },
            body: 'email=' + encodeURIComponent(${JSON.stringify(a.loginEmail)}) +
                  '&password=' + encodeURIComponent(${JSON.stringify(a.loginPassword)})
          }).then(r => r.text()).then(t => t.slice(0, 200))`,
          awaitPromise: true,
          returnByValue: true,
        }, sessionId);
        report.loginDirect = direct.result.value;
        process.stdout.write('  直接登录响应 -> ' + direct.result.value + '\n');
        await sleep(1500);
        probe = await probeLogin();
        process.stdout.write('  直接登录后 /api/user/info -> ' + probe + '\n');
      }

      report.login = { navigated: logged, probe };
      if (!/code=0/.test(probe || '')) {
        throw new Error('登录未生效：' + probe);
      }
    }

    for (const [vpName, width, height] of VIEWPORTS) {
      report.viewports.push({ name: vpName, width, height });
      await cdp.send('Emulation.setDeviceMetricsOverride', {
        width, height, deviceScaleFactor: 1, mobile: false,
      }, sessionId);

      for (const [rel, slug] of PAGES) {
        const url = a.base + rel;
        currentPage = url;
        const loaded = new Promise((resolve) => {
          const h = (msg) => {
            if (msg.sessionId === sessionId && msg.method === 'Page.loadEventFired') resolve(true);
          };
          cdp.on(h);
          setTimeout(() => resolve(false), 30000);
        });
        await cdp.send('Page.navigate', { url }, sessionId);
        await loaded;
        await sleep(a.settle);

        const shot = await cdp.send('Page.captureScreenshot', {
          format: 'png',
          captureBeyondViewport: false,
          fromSurface: true,
        }, sessionId);
        const buf = Buffer.from(shot.data, 'base64');
        const file = path.join(a.out, `${slug}.${vpName}.png`);
        await writeFile(file, buf);
        report.pages.push({ page: rel, viewport: vpName, file, bytes: buf.length });
        process.stdout.write(`  ✓ ${rel} @ ${vpName} (${width}x${height}) -> ${buf.length} bytes\n`);
      }
    }

    cdp.close();
    await writeFile(path.join(a.out, 'report.json'), JSON.stringify(report, null, 2));
    process.stdout.write(`\n截图完成：${report.pages.length} 张 -> ${a.out}\n`);
    if (report.consoleErrors.length) {
      process.stdout.write(`控制台错误 ${report.consoleErrors.length} 条，详见 report.json\n`);
    }
  } finally {
    child.kill();
    await rm(userDataDir, { recursive: true, force: true }).catch(() => {});
  }
  // Chrome 子进程在 Windows 上可能仍持有 stdio 句柄，显式退出避免挂起。
  process.exit(0);
}

main().catch((e) => {
  process.stderr.write('shot.mjs 失败：' + (e?.stack || e) + '\n');
  process.exit(1);
});
