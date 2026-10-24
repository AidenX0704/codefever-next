// 性能整改 F1（2026-10）：本 reducer 原先有两处结构性问题。
//
// 1) **对每个 action 都返回新对象**
//
//    if (!event.type.match(/^data\./)) {
//        return { ...state }
//    }
//
//    combineReducers 只有在 slice 返回**同一个对象**时才保持根 state 引用不变。
//    而每次 API 响应都会 dispatch 一次 network.connection.ok
//    （helpers/NetworkHelper.js，全站 148 个调用点，且响应尚未判 ok 就发），
//    该 action 不匹配 ^data\. ⇒ 这里返回新对象 ⇒ 根引用变化 ⇒
//    react-redux 通知**全部 89 个 connected 组件**重跑 mapStateToProps。
//
// 2) **原地修改 state**
//
//    原实现直接 state.xxx = ... 再 return { ...state }，新旧 state 共享同一批
//    嵌套对象，任何持有旧引用的组件都会看到"被就地改掉"的数据，
//    引用比较（shouldComponentUpdate）因此失效。
//
// 现在的写法：只在 Redux 初始化（state === undefined）时建立默认结构，
// 之后对无关 action 原样返回 state，对相关 action 返回**不可变**的新对象。

// 建立初始状态。
function createInitialState () {
  let localLanguage = window.localStorage.getItem('lang')
  if (!localLanguage) {
    localLanguage = navigator.language.toLowerCase() || navigator.userLanguage.toLowerCase()

    if (['zh-cn', 'en-us'].indexOf(localLanguage) < 0) {
      localLanguage = 'zh-cn'
    }

    window.localStorage.setItem('lang', localLanguage)
  }

  return {
    repositoryList: [],
    repositoryListPending: true,
    groupList: [],
    groupListPending: true,
    currentRepositoryKey: '',
    currentRepositoryError: false,
    currentRepositoryConfig: {},
    currentGroupKey: '',
    currentGroupConfig: {},
    currentUserInfo: {},
    currentLanguage: localLanguage
  }
}

function DataStore (state, event) {
  if (state === undefined) {
    return createInitialState()
  }

  // 与本 reducer 无关的 action：**原样返回**（保持引用不变）。
  if (!event.type.match(/^data\./)) {
    return state
  }

  // 值未变化时不制造新引用。
  //
  // 这一条不是可选优化，而是 F9 修复的前提：components/Main.js 在没有
  // groupName 的路由（/settings、/admin、/repositories、/mergerequests）上
  // 会反复 dispatch data.currentGroupKey.update 与 ...Config.update。
  // 如果 reducer 每次都返回新对象，就会形成
  //   dispatch -> 根引用变化 -> 重渲染 -> 再次 dispatch
  // 的重渲染回环。值相同即原样返回，环就断了。
  const same = (a, b) => a === b

  switch (event.type) {
    case 'data.repositoryList.update':
      if (same(state.repositoryList, event.data) && state.repositoryListPending === false) {
        return state
      }
      return { ...state, repositoryList: event.data, repositoryListPending: false }
    case 'data.groupList.update':
      if (same(state.groupList, event.data) && state.groupListPending === false) {
        return state
      }
      return { ...state, groupList: event.data, groupListPending: false }
    case 'data.currentRepositoryKey.update':
      return same(state.currentRepositoryKey, event.data)
        ? state
        : { ...state, currentRepositoryKey: event.data }
    case 'data.currentRepositoryError.update':
      return same(state.currentRepositoryError, event.data)
        ? state
        : { ...state, currentRepositoryError: event.data }
    case 'data.currentRepositoryConfig.update':
      return same(state.currentRepositoryConfig, event.data)
        ? state
        : { ...state, currentRepositoryConfig: event.data }
    case 'data.currentGroupKey.update':
      return same(state.currentGroupKey, event.data)
        ? state
        : { ...state, currentGroupKey: event.data }
    case 'data.currentGroupConfig.update':
      return same(state.currentGroupConfig, event.data)
        ? state
        : { ...state, currentGroupConfig: event.data }
    case 'data.currentUserInfo.update':
      return same(state.currentUserInfo, event.data)
        ? state
        : { ...state, currentUserInfo: event.data }
    case 'data.currentLanguage.update':
      window.localStorage.setItem('lang', event.data)
      return same(state.currentLanguage, event.data)
        ? state
        : { ...state, currentLanguage: event.data }
    default:
      // 未知的 data.* 事件：同样原样返回，不要凭空造新引用。
      return state
  }
}

export default DataStore
