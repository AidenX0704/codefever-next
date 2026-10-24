// 性能整改 F1（2026-10）。
//
// 原实现的问题（比 DataStore / DrawerStates 更严重，因为它还会**改坏数据**）：
//
//   1. 与 notification.* 无关的 action 也 `return { ...state }`，而且结尾那一行
//      是**无条件**重建：
//
//    return { ...state, confirmation: { ...state.confirmation, queueList: [...state.confirmation.queueList] } }
//
//      于是每次 API 调用（network.connection.ok，全站 148 个调用点）都会新建
//      confirmation 对象与 queueList 数组，把引用相等彻底破坏。
//
//   2. 直接原地修改 state（state.notificationCount++、queueList.push(...)）。
//
//   3. confirm 队列的 remove 分支是一段**死代码**：
//
//    const id = getObjectByProperty(queueList, 'key', event.data.key)  // 返回对象，不是下标
//    typeof id === 'number' && delete queueList[id]                     // 永远为 false
//
//      因此"从队列中移除某个确认框"从来没有生效过。这里按 key 过滤修正。
//
//   4. 若把上面那行改成按下标 delete，会在数组上留下**空洞**，随后 consume 的
//      shift() 可能返回 undefined，newItem.data.title 会直接抛异常。
//      这里改用不可变的头尾拆分，彻底消除空洞。
//
// 事件语义（与被保留的行为逐条对齐）：
//   eventCount 只在 add / close / remove / consume(取到条目时) 递增 ——
//   components/Comformation.js 用它的变化作为"继续消费队列"的信号，
//   因此这几个分支必须继续递增。

const NOOP = function () { return true }

// 建立一个空队列的初始确认框状态。
function createConfirmation () {
  return {
    open: false,
    title: '',
    description: '',
    accept: NOOP,
    reject: NOOP,
    queueList: [],
    eventCount: 0
  }
}

function createInitialState () {
  return {
    notificationCount: 0,
    notificationBody: {},
    askHandleStatus: false,
    confirmation: createConfirmation(),
    notificationOpenStatus: false
  }
}

function NotificationReducer (state, event) {
  if (state === undefined) {
    return createInitialState()
  }

  // 与本 reducer 无关的 action：原样返回，保持引用不变。
  if (!event.type.match(/^notification\./)) {
    return state
  }

  const confirmation = state.confirmation

  switch (event.type) {
    case 'notification.message.add':
      return {
        ...state,
        notificationCount: state.notificationCount + 1,
        notificationBody: event.data
      }

    case 'notification.message.change':
      // 原实现写作 `state.notificationBody = event.data = {}`（顺手改了 action
      // 对象本身）。这里只清空 store 里的值，语义一致且不再污染 action。
      return { ...state, notificationBody: {} }

    case 'notification.askhandle.status':
      return { ...state, askHandleStatus: event.data }

    case 'notification.confirmation.popup':
      return {
        ...state,
        confirmation: {
          ...confirmation,
          open: true,
          title: event.data.title,
          description: event.data.description,
          accept: event.data.accept || NOOP,
          reject: event.data.reject || NOOP
        }
      }

    case 'notification.confirmation.close':
      return {
        ...state,
        confirmation: {
          ...confirmation,
          eventCount: confirmation.eventCount + 1,
          open: false,
          accept: NOOP,
          reject: NOOP
        }
      }

    case 'notification.confirmation.add':
      return {
        ...state,
        confirmation: {
          ...confirmation,
          eventCount: confirmation.eventCount + 1,
          queueList: [...confirmation.queueList, event.data]
        }
      }

    case 'notification.confirmation.remove': {
      // 修正：按 key 过滤（原先 typeof id === 'number' 恒为 false，从未删除成功）。
      const key = event.data && event.data.key
      return {
        ...state,
        confirmation: {
          ...confirmation,
          eventCount: confirmation.eventCount + 1,
          queueList: confirmation.queueList.filter(item => !item || item.key !== key)
        }
      }
    }

    case 'notification.confirmation.consume': {
      // 队列为空或已有确认框在显示：无需改动（原实现会白白制造新引用）。
      if (!confirmation.queueList.length || confirmation.open) {
        return state
      }
      const [next, ...rest] = confirmation.queueList
      if (!next || !next.data) {
        // 理论上不会发生（add 只会推入带 data 的条目），
        // 但防御一下：丢弃坏条目，避免渲染期抛异常。
        return {
          ...state,
          confirmation: { ...confirmation, eventCount: confirmation.eventCount + 1, queueList: rest }
        }
      }
      return {
        ...state,
        confirmation: {
          ...confirmation,
          eventCount: confirmation.eventCount + 1,
          queueList: rest,
          open: true,
          title: next.data.title,
          description: next.data.description,
          accept: next.data.accept || NOOP,
          reject: next.data.reject || NOOP
        }
      }
    }

    case 'notification.notificationOpenStatus.toggle':
      return { ...state, notificationOpenStatus: !state.notificationOpenStatus }

    case 'notification.notificationOpenStatus.open':
      return state.notificationOpenStatus === true ? state : { ...state, notificationOpenStatus: true }

    case 'notification.notificationOpenStatus.close':
      return state.notificationOpenStatus === false ? state : { ...state, notificationOpenStatus: false }

    default:
      return state
  }
}

export default NotificationReducer
