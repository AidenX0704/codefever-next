// 性能整改 F1（2026-10）。
//
// 原实现有两个结构性问题（与 reducers/DataStore.js 同源）：
//   1. 与 drawer.* 无关的 action 也 `return { ...state }` —— 每次 API 调用都会
//      dispatch network.connection.ok，于是根 state 引用每次变化，
//      react-redux 通知全部 connected 组件重跑 mapStateToProps；
//   2. 直接改 state.expandStatus 再返回新对象，新旧 state 共享同一份字段，
//      引用比较（shouldComponentUpdate）失效。
//
// 现在：无关 action 原样返回 state；相关 action 返回不可变的新对象，
// 且值未变化时不制造新引用（例如对已关闭的抽屉再发 close）。

function DrawerReducer (state, event) {
  if (state === undefined) {
    state = { expandStatus: true }
  }

  if (!event.type.match(/^drawer\./)) {
    return state
  }

  switch (event.type) {
    case 'drawer.expandStatus.toggle':
      return { ...state, expandStatus: !state.expandStatus }
    case 'drawer.expandStatus.close':
      return state.expandStatus === false ? state : { ...state, expandStatus: false }
    case 'drawer.expandStatus.open':
      return state.expandStatus === true ? state : { ...state, expandStatus: true }
    default:
      return state
  }
}

export default DrawerReducer
