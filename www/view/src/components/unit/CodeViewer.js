// core
import React from 'react'
import PropTypes from 'prop-types'
import { connect } from 'react-redux'
import { withRouter } from 'react-router-dom'
import { withStyles } from '@material-ui/core/styles'
import { injectIntl } from 'react-intl'

// component
// import hljs from 'highlight.js'
import Prism from 'prismjs'
// 性能整改 E2：删掉 highlight.js 的主题样式表。
// 这里的渲染全程使用 Prism（hljs 的 import 早已被注释掉），
// 那份 CSS 只会白白进入 3.6 KB 的样式 chunk。
import { getCodeLanguageType, makeLink } from 'APPSRC/helpers/VaribleHelper'
import CodeLine from 'APPSRC/components/unit/CodeLine'
import CommitItem from 'APPSRC/components/unit/CommitItem'
import CircularProgress from '@material-ui/core/CircularProgress'

// ---------------------------------------------------------------------------
// 性能整改 F5（2026-10）：高亮结果缓存 + blame 索引
// ---------------------------------------------------------------------------
//
// 整改前 render() 每次都会：
//   1. Prism.highlight(整个文件) 再 split 成行；
//   2. 对每一行做 blame.filter(...) —— O(行数 × blame 段数)。
//
// 而 shouldComponentUpdate 无条件 return true，于是父组件任何一次重渲染
// （哪怕只是搜索框敲了一个字符）都会把这两步全跑一遍：
// 5000 行的文件是数百毫秒级的主线程长任务。
//
// 这里做两件事：
//   * 高亮结果按「语言 + 全文」缓存，文件内容不变就不重算；
//   * blame 按数组引用建一次索引（Map: 起始行 -> blame 段），把每行的
//     线性查找换成 O(1)。
//
// 两个缓存都有上界，避免长时间浏览大量文件时无限增长。
const HIGHLIGHT_CACHE_MAX = 8
const highlightCache = new Map()

function highlightCached (text, lang) {
  const key = lang + '|' + text
  const hit = highlightCache.get(key)
  if (hit !== undefined) {
    return hit
  }
  const grammar = Prism.languages[lang] || Prism.languages.markdown
  const parsed = Prism.highlight(text, grammar, lang)
  if (highlightCache.size >= HIGHLIGHT_CACHE_MAX) {
    // 简单淘汰：删掉最早插入的一条（Map 保持插入顺序）。
    highlightCache.delete(highlightCache.keys().next().value)
  }
  highlightCache.set(key, parsed)
  return parsed
}

const blameIndexCache = new WeakMap()

// blameIndex 把 blame 数组建成「起始行 -> 段落」的 Map，按数组引用缓存。
function blameIndex (blame) {
  if (!blame || !blame.length) {
    return new Map()
  }
  let index = blameIndexCache.get(blame)
  if (index) {
    return index
  }
  index = new Map()
  for (let i = 0; i < blame.length; i++) {
    index.set(blame[i].start, blame[i])
  }
  blameIndexCache.set(blame, index)
  return index
}

// style
const styles = theme => ({
  table: {
    width: '100%',
    tableLayout: 'fixed',
    borderCollapse: 'collapse',
    '& tr:first-child': {
      border: 'none'
    }
  },
  topBorder: {
    borderTop: '1px solid ' + theme.palette.border
  },
  blame: {
    width: theme.spacing(40),
    verticalAlign: 'middle'
  },
  blameLoading: {
    width: theme.spacing(40),
    verticalAlign: 'top',
    textAlign: 'center',
    paddingTop: theme.spacing(10)
  },
  lineNumber: {
    lineHeight: theme.spacing(2) + 'px',
    width: theme.spacing(4),
    padding: '0 ' + theme.spacing(1) + 'px',
    textAlign: 'right',
    verticalAlign: 'middle',
    userSelect: 'none',
    color: theme.palette.text.lighter,
    backgroundColor: theme.palette.background.main
  },
  code: {
    verticalAlign: 'middle',
    overflowWrap: 'break-word'
  }
})

class CodeViewer extends React.Component {
  constructor (props) {
    super(props)
    const lineNumber = window.location.href.match(/^.*#(\d+)$/)
    this.state = {
      lineNumber: lineNumber ? parseInt(lineNumber[1]) : -1
    }
    this.ref = React.createRef()
  }

  componentDidMount () {
    if (this.ref.current) {
      let parent = this.ref.current
      let scroll = 0
      while (parent) {
        scroll += parent.offsetTop
        parent = parent.offsetParent
      }

      document.querySelector('.app-content').scrollTop = scroll - 64 - 21 * 5
    }
    return true
  }

  shouldComponentUpdate (nextProps, nextState) {
    // 性能整改 F5：原实现无条件 return true，父组件任何重渲染都会触发
    // 全文件重新高亮 + 全量 blame 查找。这里只比较真正影响输出的 props。
    //
    // 之所以可以用引用比较：数据来自 API 响应，reducer 整体替换对象
    // （见 reducers/DataStore.js 的不可变改造），内容变化时引用必然变化。
    return nextProps.object !== this.props.object ||
      nextProps.blame !== this.props.blame ||
      nextProps.fragment !== this.props.fragment ||
      nextProps.currentRepositoryConfig !== this.props.currentRepositoryConfig ||
      nextProps.match !== this.props.match ||
      nextState.lineNumber !== this.state.lineNumber
  }

  componentDidUpdate (prevProps, prevState) {
    return true
  }

  render () {
    const { object, blame, currentRepositoryConfig, match, classes, fragment } = this.props
    const { lineNumber } = this.state
    const lang = getCodeLanguageType(object.path)
    const codes = fragment ? object.object : object.object.raw.split('\n')
    // let parsed = hljs.highlight(lang, object.object.raw, true)
    // let parsedHtml = parsed.value.split('\n')
    // 性能整改 F5：按「语言 + 全文」缓存，内容不变就不重新高亮。
    const parsedHtml = highlightCached(codes.join('\n'), lang).split('\n')
    let codeLines = []

    if (blame && blame.length) {
      let count = 0
      const blameByStart = blameIndex(blame)
      codeLines = codes.map((item, key) => {
        let blameCell = null
        // 性能整改 F5：blame 预先建成 Map（起始行 → 段落），
        // 把每行的 O(blame 段数) 查找降为 O(1)（原先 1000 行 × 400 段 = 40 万次比较）。
        const blameData = blameByStart.get(key + 1)
        if (blameData) {
          count = blameData.length
          blameCell = <td className={classes.blame} rowSpan={blameData.length}>
            <CommitItem key={key}
              data={blameData.commit}
              linkPathBase={makeLink(
                match.params.groupName,
                match.params.repositoryName,
                'commit'
              )}
              currentRepositoryConfig={currentRepositoryConfig}
              blameMode
            />
          </td>
        }

        return <tr key={key} className={blameCell && classes.topBorder}>
          {(key === (codes.length - 1) && codes[key].length === 0) || count-- < 1 ? <td className={classes.blame} /> : blameCell}
          <td className={classes.lineNumber}><code>{key + 1}</code></td>
          <td className={classes.code}><CodeLine code={item} htmlCode={parsedHtml[key]} /></td>
        </tr>
      })
    } else if (match.params.type === 'blame') {
      codeLines = codes.map((item, key) => {
        return <tr key={key}>
          { key === 0 && <td className={classes.blameLoading} rowSpan={codes.length}><CircularProgress size={24} /></td> }
          <td className={classes.lineNumber}><code>{key + 1}</code></td>
          <td className={classes.code}><CodeLine code={item} htmlCode={parsedHtml[key]} /></td>
        </tr>
      })
    } else {
      codeLines = codes.map((item, key) => {
        let style = {}
        if (fragment && fragment[key]) {
          style = {
            color: '#fff',
            background: fragment[key]
          }
        }

        return <tr key={key} ref={key + 1 === lineNumber ? this.ref : null}>
          <td className={classes.lineNumber} style={style}><code>{key + 1}</code></td>
          <td className={classes.code}><CodeLine code={item} htmlCode={parsedHtml[key]} /></td>
        </tr>
      })
    }

    return (<table className={classes.table}>
      <tbody>{ codeLines }</tbody>
    </table>)
  }
}

CodeViewer.propTypes = {
  classes: PropTypes.object.isRequired,
  object: PropTypes.object.isRequired,
  match: PropTypes.object.isRequired,
  currentRepositoryConfig: PropTypes.object.isRequired,
  blame: PropTypes.array,
  fragment: PropTypes.array
}

const mapStateToProps = (state, ownProps) => {
  return {
    currentRepositoryConfig: state.DataStore.currentRepositoryConfig
  }
}

const mapDispatchToProps = (dispatch, ownProps) => {
  return {
    dispatchEvent: (event) => { dispatch(event) }
  }
}

export default injectIntl(
  withStyles(styles)(
    withRouter(
      connect(mapStateToProps, mapDispatchToProps)(CodeViewer)
    )
  )
)
