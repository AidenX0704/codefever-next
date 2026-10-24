// core
import React from 'react'
import PropTypes from 'prop-types'
import { connect } from 'react-redux'
import { withRouter } from 'react-router-dom'
import { withStyles } from '@material-ui/core/styles'

// components
import MainRoutes from 'APPSRC/routes/MainRoutes'
import UserData from 'APPSRC/data_providers/UserData'
import GroupData from 'APPSRC/data_providers/GroupData'
import RepositoryData from 'APPSRC/data_providers/RepositoryData'

// helpers
import FilterGenerator from 'APPSRC/helpers/FilterGenerator'
import NetworkHelper from 'APPSRC/helpers/NetworkHelper'

const styles = theme => ({
  container: {
    backgroundColor: theme.palette.background.light
  },
  content: {
    padding: 0
  },
  toolbar: theme.mixins.toolbar,
  mainContainer: {
    padding: theme.spacing(3) + 'px ' + theme.spacing(6) + 'px'
  }
})

class Main extends React.Component {
  constructor (props) {
    super(props)

    this.state = {
      groupListLoaded: false,
      repositoryListLoaded: false
    }
  }

  componentDidMount () {
    UserData.getUserInfo()
      .then(NetworkHelper.withEventdispatcher(this.props.dispatchEvent)(NetworkHelper.getJSONData))
      .then(data => {
        if (!data.code) {
          this.props.dispatchEvent({ type: 'data.currentUserInfo.update', data: data.data })
        }
      })

    RepositoryData.list()
      .then(NetworkHelper.withEventdispatcher(this.props.dispatchEvent)(NetworkHelper.getJSONData))
      .then(data => {
        if (!data.code) {
          this.props.dispatchEvent({ type: 'data.repositoryList.update', data: data.data })
          this.setState({ repositoryListLoaded: true })
        }
      })

    GroupData.list()
      .then(NetworkHelper.withEventdispatcher(this.props.dispatchEvent)(NetworkHelper.getJSONData))
      .then(data => {
        if (!data.code) {
          this.props.dispatchEvent({ type: 'data.groupList.update', data: data.data })
          this.setState({ groupListLoaded: true })
        }
      })
  }

  componentDidUpdate (prevProps, prevState) {
    // 性能整改 F9（2026-10）：本方法的前身是 shouldComponentUpdate 里的一整段副作用。
    //
    // 整改前的写法有三个问题：
    //   1. 在 shouldComponentUpdate 里 dispatch（以及 setState）—— 渲染前改变
    //      store，React 随后又因为 props 变化重跑 shouldComponentUpdate，
    //      反复触发；
    //   2. `else if (!nextProps.match.params.groupName)` 这一支**没有任何变化判断**，
    //      于是 /settings、/admin、/repositories、/mergerequests 这类没有 groupName
    //      的路由**每次渲染**都会 dispatch 两次；
    //   3. 用 `{}` 重置 currentGroupConfig / currentRepositoryConfig —— 每次都是
    //      新对象，订阅者先收到空对象、再收到真实配置，白渲染一轮。
    //
    // 现在：只在"路由参数真的变了"或"列表刚刚加载完"时触发；值未变化时不 dispatch
    // （reducer 也有对应的引用兜底，见 reducers/DataStore.js）。
    const groupNameChanged = prevProps.match.params.groupName !== this.props.match.params.groupName
    const repoNameChanged = prevProps.match.params.repositoryName !== this.props.match.params.repositoryName
    const groupListJustLoaded = !prevState.groupListLoaded && this.state.groupListLoaded
    const repoListJustLoaded = !prevState.repositoryListLoaded && this.state.repositoryListLoaded

    // load group config
    if (groupListJustLoaded || groupNameChanged) {
      const currentGroup = this.props.groupList.filter(FilterGenerator.name(this.props.match.params.groupName))[0]

      if (currentGroup && currentGroup.id) {
        this.props.dispatchEvent({ type: 'data.currentGroupKey.update', data: currentGroup.id })
        if (this.props.history.location.pathname.match(/^\/groups/i)) {
          GroupData.config({ gKey: currentGroup.id })
            .then(NetworkHelper.withEventdispatcher(this.props.dispatchEvent)(NetworkHelper.getJSONData))
            .then((data) => {
              this.props.dispatchEvent({
                type: 'data.currentGroupConfig.update',
                data: !data.code ? data.data : {}
              })
            })
        }
      } else {
        this.props.dispatchEvent({ type: 'data.currentGroupKey.update', data: '' })
        this.props.dispatchEvent({ type: 'data.currentGroupConfig.update', data: {} })
      }
    }

    // load repository config
    if (repoListJustLoaded || groupNameChanged || repoNameChanged) {
      const currentRepository = this.props.repositoryList.filter(
        item =>
          item.group.name === this.props.match.params.groupName &&
          item.name === this.props.match.params.repositoryName
      )[0]

      if (currentRepository && currentRepository.id) {
        this.props.dispatchEvent({ type: 'data.currentRepositoryKey.update', data: currentRepository.id })
        this.props.dispatchEvent({ type: 'data.currentRepositoryError.update', data: false })
        RepositoryData.config({ rKey: currentRepository.id })
          .then(NetworkHelper.withEventdispatcher(this.props.dispatchEvent)(NetworkHelper.getJSONData))
          .then((data) => {
            this.props.dispatchEvent({
              type: 'data.currentRepositoryConfig.update',
              data: !data.code ? data.data : {}
            })
          })
      } else {
        this.props.dispatchEvent({ type: 'data.currentRepositoryKey.update', data: '' })
        this.props.dispatchEvent({ type: 'data.currentRepositoryConfig.update', data: {} })
        this.props.dispatchEvent({ type: 'data.currentRepositoryError.update', data: true })
      }
    }
  }

  render () {
    const { classes } = this.props
    return <div className={classes.container}>
      <main className={classes.content}>
        <div className={classes.toolbar} />
        <div className={classes.mainContainer}>
          <MainRoutes />
        </div>
      </main>
    </div>
  }
}

Main.propTypes = {
  history: PropTypes.object.isRequired,
  classes: PropTypes.object.isRequired,
  match: PropTypes.object.isRequired,
  repositoryList: PropTypes.array.isRequired,
  groupList: PropTypes.array.isRequired,
  dispatchEvent: PropTypes.func.isRequired
}

const mapStateToProps = (state, ownProps) => {
  return {
    repositoryList: state.DataStore.repositoryList,
    groupList: state.DataStore.groupList
  }
}

const mapDispatchToProps = (dispatch, ownProps) => {
  return {
    dispatchEvent: (event) => { dispatch(event) }
  }
}

export default withStyles(styles)(
  withRouter(
    connect(mapStateToProps, mapDispatchToProps)(Main)
  )
)
