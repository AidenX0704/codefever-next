// dynamic path settings
import './env'

// core
import React from 'react'
import ReactDOM from 'react-dom'
import { BrowserRouter } from 'react-router-dom'
import { createStore } from 'redux'
import { Provider } from 'react-redux'

// import assets
import 'APPSRC/assets/styles/App.styl'

// import pollyfile
import '@formatjs/intl-pluralrules/polyfill'
import '@formatjs/intl-relativetimeformat/polyfill'

// import Components
import BaseRoutes from 'APPSRC/routes/BaseRoutes'
import Reducer from 'APPSRC/reducers/main'

// 2026-10 性能整改 B1：接入 web-vitals 真实用户监控。
//
// 背景：package.json 早就声明了 web-vitals 依赖，但全仓库没有任何接线代码，
// 导致上一轮性能审查的所有前端结论都无法持续追踪、只能靠人工复测。
//
// 采集面（与 Core Web Vitals 对齐）：
//   CLS  累积布局偏移（稳定性）
//   FID  首次输入延迟（交互性，web-vitals@1 的 INP 前身）
//   FCP  首次内容绘制
//   LCP  最大内容绘制（加载性能，登录页的关键指标）
//   TTFB 首字节时间（区分前后端问题）
//
// 上报方式：navigator.sendBeacon —— 不阻塞卸载、不污染关键路径；
// 不支持时退化为 keepalive fetch。后端 /api/web-vitals 只写日志不入库。
const report = (metric) => {
  let body
  try {
    body = JSON.stringify({
      name: metric.name,
      value: metric.value,
      rating: metric.rating,
      id: metric.id,
      path: window.location.pathname
    })
  } catch (e) {
    return
  }
  const url = '/api/web-vitals'
  if (navigator.sendBeacon) {
    // text/plain 属于 CORS 简单请求，避免 sendBeacon 触发的预检。
    navigator.sendBeacon(url, new Blob([body], { type: 'text/plain' }))
    return
  }
  if (window.fetch) {
    window.fetch(url, {
      method: 'POST',
      body,
      headers: { 'Content-Type': 'text/plain' },
      keepalive: true
    }).catch(() => {})
  }
}

// 动态引入：web-vitals 只在浏览器上报时用到，不应该进入首屏关键路径。
// 失败时静默忽略 —— 监控永远不该影响业务。
import('web-vitals')
  .then(({ getCLS, getFID, getFCP, getLCP, getTTFB }) => {
    getCLS(report)
    getFID(report)
    getFCP(report)
    getLCP(report)
    getTTFB(report)
  })
  .catch(() => {})

const Store = createStore(Reducer)
const routeBaseName = '/'

// render app with router
ReactDOM.render((
  <Provider store={Store}>
    <BrowserRouter basename={routeBaseName}>
      <BaseRoutes />
    </BrowserRouter>
  </Provider>
), document.getElementById('root'))
