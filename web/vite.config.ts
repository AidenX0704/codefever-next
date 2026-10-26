import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

/**
 * CodeFever Next 前端构建配置。
 *
 * 关键约定（与 nginx `backend/deploy/docker/vhost-go.conf-template` 对齐）：
 *   1. 产物目录固定为 `web/build`，镜像只 COPY、不在容器内构建；
 *   2. 带内容哈希的产物全部落在 `static/js/` 下 —— nginx 对 `/static/js/` 下发
 *      `Cache-Control: immutable`（一年），JS/CSS/字体/图片都在这一档里，因此这里
 *      故意把 assetsDir 设成 `static/js` 而不是默认的 `assets`；
 *   3. 生产不产出 source map（旧版把 6.3 MB 的 .map 一起入镜，属历史缺陷）；
 *   4. 现代浏览器目标（es2022）—— 不再为 IE11 付 polyfill 成本。
 */
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  build: {
    outDir: 'build',
    emptyOutDir: true,
    assetsDir: 'static/js',
    sourcemap: false,
    target: 'es2022',
    cssCodeSplit: true,
    // 体积预算以 gzip 为准（见 docs/FRONTEND_REWRITE_PLAN.md §7：首屏 gzip ≤ 200 KB）。
    // vendor 块原始体积 ~313 KB 但 gzip 仅 ~99 KB，因此把告警阈值抬到 400 KB，
    // 避免每次构建都刷一条没有行动价值的 warning。
    chunkSizeWarningLimit: 400,
    rollupOptions: {
      output: {
        // 只把"框架层"固定成一个块，业务依赖（如 prismjs、date-fns）跟随使用它的
        // 懒加载页面走，避免它们被 vendor 块带回首屏。
        // 注意 Vite 8 / Rolldown 起 `manualChunks` 只接受函数形式（对象形式已移除）。
        manualChunks: (id: string) =>
          /node_modules[\\/](react|react-dom|react-router|scheduler)[\\/]/.test(id) ? 'vendor' : undefined,
      },
    },
  },
  server: {
    port: 3000,
    // 开发期把 Go 后端的应用层路径原样代理过去（与 nginx 的生产规则保持一致）。
    proxy: Object.fromEntries(
      ['/api', '/user', '/file', '/doc', '/lang', '/captcha', '/feature'].map((p) => [
        p,
        { target: 'http://127.0.0.1:9001', changeOrigin: false },
      ]),
    ),
  },
})
