// Markdown 渲染器（按需加载）
//
// 性能整改 E3（2026-10）。
//
// 背景：react-markdown + remark-gfm + 它们背后的 micromark / mdast / hast /
// unist 全家桶合计约 172 KB（minified），而整个应用里**只有一个地方**用到它：
// ObjectViewer 渲染 README / Markdown 文件时。整改前它是静态 import，
// 连登录页的访客也要为它付下载与解析成本。
//
// 做法：把 react-markdown 与 remark-gfm 收进这个独立模块，由调用方用
// React.lazy(() => import('APPSRC/helpers/MarkdownRenderer')) 引入，
// webpack 会自动把它切成独立的异步 chunk —— 只有真正查看 Markdown 时才下载。
//
// 顺带修掉两个渲染期开销：
//   1. remarkPlugins 提升为**模块常量**。整改前写在 JSX 里
//      （remarkPlugins={[remarkGfm]}），每次渲染都新建一个数组，
//      react-markdown 会因此重新初始化插件处理链。
//   2. github-markdown-css 改为本地内置（App.styl 引入），
//      不再在 render body 里插第三方 CDN 的 <link>。
import React from 'react'
import PropTypes from 'prop-types'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'

// github-markdown-css 的本地副本。
//
// 放在**本模块内**导入（而不是 index.js / App.styl），webpack 会把它并进
// MarkdownRenderer 的异步 chunk 的 CSS 里：只有真正查看 Markdown 时才加载，
// 不会进入首屏样式。
import 'APPSRC/assets/styles/markdown-body.css'

// 模块常量：整个应用生命周期内只创建一次。
const REMARK_PLUGINS = [remarkGfm]

/**
 * 渲染一段 Markdown。
 *
 * @param {Object} props
 * @param {string} props.raw  Markdown 原文
 */
function MarkdownRenderer ({ raw }) {
  return <ReactMarkdown remarkPlugins={REMARK_PLUGINS} children={raw} />
}

MarkdownRenderer.propTypes = {
  // Markdown 原文（来自对象查看器的 object.object.raw）。
  raw: PropTypes.string
}

export default MarkdownRenderer
