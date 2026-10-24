import React from 'react'

export default function (options) {
  return function (WrappedComponent) {
    return class extends React.Component {
      constructor (props) {
        super(props)
        this.target = null
        this.cancel = null
        this.onScroll = this.onScroll.bind(this)
        this.state = {
          scrollTrigger: false
        }
      }

      componentDidMount () {
        if (options.watching) {
          this.cancel && this.cancel()
          this.target = document.querySelector(options.watching)
          this.cancel = function () { this.target.removeEventListener('scroll', this.onScroll) }
          // 性能整改 F17：滚动监听显式声明 passive。
          // 未声明时浏览器必须假定监听器可能调用 preventDefault，
          // 合成器线程无法立即滚动，滚动会卡手。
          this.target.addEventListener('scroll', this.onScroll, { passive: true })
          this.onScroll({ target: this.target })
        }
      }

      componentWillUnmount () {
        this.cancel && this.cancel()
        this.cancel = null
        this.target = null
      }

      onScroll (ev) {
        const scrollAmount = ev.target.pageYOffset || ev.target.scrollTop
        if (scrollAmount > options.threshold && !this.state.scrollTrigger) {
          this.setState({ scrollTrigger: true })
        } else if (scrollAmount <= options.threshold && this.state.scrollTrigger) {
          this.setState({ scrollTrigger: false })
        }
      }

      render () {
        return <WrappedComponent ref={this.mountContainer} scrollTrigger={this.state.scrollTrigger} {...this.props} />
      }
    }
  }
}
