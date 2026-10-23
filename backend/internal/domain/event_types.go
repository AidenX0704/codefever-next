package domain

// 事件类型注册表 —— 逐字搬运自 application/libraries/service/Constant/EventType.php。
//
// 命名规则：
//
//	事件字符串        主事件:子事件
//	事件常量名        主事件_子事件
//	事件数据描述常量  主事件_子事件_D
//	事件数据描述内容  变量名:基础类型[|变量名:基础类型...]
//
// 基础类型：int / float / bool / string / array / *；类型后缀 "?" 表示该字段可缺省。
// 空描述（""）表示不校验该事件的数据。
//
// 事件数据描述在 Emit 时被逐字段校验 —— 描述错了会导致事件被静默丢弃，
// 因此这份注册表必须与 PHP 侧保持逐字一致。

// EventType 是事件名常量。
type EventType string

// EventDataDesc 是事件数据描述。
type EventDataDesc string

const EventTypeBRANCH_CREATE EventType = "branch:create"
const EventDataDescBRANCH_CREATE_D EventDataDesc = "gKey:string|rKey:string|name:string"
const EventTypeBRANCH_REMOVE EventType = "branch:remove"
const EventDataDescBRANCH_REMOVE_D EventDataDesc = "gKey:string|rKey:string|name:string"
const EventTypeDEFAULT_BRANCH_CHANGE EventType = "branch:changeDefaultBranch"
const EventDataDescDEFAULT_BRANCH_CHANGE_D EventDataDesc = "gKey:string|rKey:string|from:string|to:string"
const EventTypeGROUP EventType = "group"
const EventTypeGROUP_ADD_MEMBER EventType = "group:addMember"
const EventDataDescGROUP_ADD_MEMBER_D EventDataDesc = "gKey:string|uid:string|name:string|email:string"
const EventTypeGROUP_CHANGE_MEMBER_ROLE EventType = "group:changeMemberRole"
const EventDataDescGROUP_CHANGE_MEMBER_ROLE_D EventDataDesc = "gKey:string|uid:string|name:string|email:string|to:int"
const EventTypeGROUP_CHANGE_OWNER EventType = "group:changeOwner"
const EventDataDescGROUP_CHANGE_OWNER_D EventDataDesc = "gKey:string|uid:string|name:string|email:string"
const EventTypeGROUP_CHANGE_URL EventType = "group:changeURL"
const EventDataDescGROUP_CHANGE_URL_D EventDataDesc = "gKey:string|from:string|to:string"
const EventTypeGROUP_CREATE EventType = "group:create"
const EventDataDescGROUP_CREATE_D EventDataDesc = "gKey:string|name:string"
const EventDataDescGROUP_D EventDataDesc = ""
const EventTypeGROUP_REMOVE EventType = "group:remove"
const EventDataDescGROUP_REMOVE_D EventDataDesc = "gKey:string|name:string"
const EventTypeGROUP_REMOVE_MEMBER EventType = "group:removeMember"
const EventDataDescGROUP_REMOVE_MEMBER_D EventDataDesc = "gKey:string|uid:string|name:string|email:string"
const EventTypeGROUP_UPDATE_AVATAR EventType = "group:updateAvator"
const EventDataDescGROUP_UPDATE_AVATAR_D EventDataDesc = "gKey:string|from:string|to:string"
const EventTypeGROUP_UPDATE_DESCRIPTION EventType = "group:updateDescription"
const EventDataDescGROUP_UPDATE_DESCRIPTION_D EventDataDesc = "gKey:string"
const EventTypeGROUP_UPDATE_NAME EventType = "group:updateName"
const EventDataDescGROUP_UPDATE_NAME_D EventDataDesc = "gKey:string|from:string|to:string"
const EventTypeHOOK EventType = "hook"
const EventDataDescHOOK_D EventDataDesc = ""
const EventTypeHOOK_POST_RECEIVE EventType = "hook:postReceive"
const EventDataDescHOOK_POST_RECEIVE_D EventDataDesc = "rKey:string|name:string|from:string|to:string"
const EventTypeMERGE_REQUEST_CLOSE EventType = "mergeRequest:close"
const EventDataDescMERGE_REQUEST_CLOSE_D EventDataDesc = "gKey:string|rKey:string|id:int|mrKey:string|sourceBranch:string|targetBranch:string|sourceRKey:string|sourceGKey:string"
const EventTypeMERGE_REQUEST_CREATE EventType = "mergeRequest:create"
const EventDataDescMERGE_REQUEST_CREATE_D EventDataDesc = "gKey:string|rKey:string|id:int|mrKey:string|sourceBranch:string|targetBranch:string|sourceRKey:string|sourceGKey:string"
const EventTypeMERGE_REQUEST_MERGE EventType = "mergeRequest:merge"
const EventDataDescMERGE_REQUEST_MERGE_D EventDataDesc = "gKey:string|rKey:string|id:int|mrKey:string|sourceBranch:string|targetBranch:string|sourceRKey:string|sourceGKey:string"
const EventTypeMERGE_REQUEST_REVIEWER_CREATE EventType = "mergeRequestReviewer:create"
const EventDataDescMERGE_REQUEST_REVIEWER_CREATE_D EventDataDesc = "gKey:string|rKey:string|id:int|mrKey:string|reviewer:string|mrrKey:string"
const EventTypeMERGE_REQUEST_REVIEWER_DELETE EventType = "mergeRequestReviewer:delete"
const EventDataDescMERGE_REQUEST_REVIEWER_DELETE_D EventDataDesc = "gKey:string|rKey:string|id:int|mrKey:string|reviewer:string"
const EventTypeMERGE_REQUEST_REVIEWER_REVIEW EventType = "mergeRequestReviewer:review"
const EventDataDescMERGE_REQUEST_REVIEWER_REVIEW_D EventDataDesc = "gKey:string|rKey:string|id:int|mrKey:string|reviewer:string"
const EventTypePROTECTED_BRANCH_RULE_CHANGE EventType = "branch:changeProtectedBranchRule"
const EventDataDescPROTECTED_BRANCH_RULE_CHANGE_D EventDataDesc = "gKey:string|rKey:string|name:string"
const EventTypePROTECTED_BRANCH_RULE_CREATE EventType = "branch:createProtectedBranchRule"
const EventDataDescPROTECTED_BRANCH_RULE_CREATE_D EventDataDesc = "gKey:string|rKey:string|name:string"
const EventTypePROTECTED_BRANCH_RULE_REMOVE EventType = "branch:removeProtectedBranchRule"
const EventDataDescPROTECTED_BRANCH_RULE_REMOVE_D EventDataDesc = "gKey:string|rKey:string|name:string"
const EventTypeREPO EventType = "repo"
const EventTypeREPO_ADD_MEMBER EventType = "repo:addMember"
const EventDataDescREPO_ADD_MEMBER_D EventDataDesc = "gKey:string|rKey:string|uid:string|name:string|email:string"
const EventTypeREPO_CHANGE_MEMBER_ROLE EventType = "repo:changeMemberRole"
const EventDataDescREPO_CHANGE_MEMBER_ROLE_D EventDataDesc = "gKey:string|rKey:string|uid:string|name:string|email:string|to:int"
const EventTypeREPO_CHANGE_OWNER EventType = "repo:changeOwner"
const EventDataDescREPO_CHANGE_OWNER_D EventDataDesc = "gKey:string|rKey:string|uid:string|name:string|email:string"
const EventTypeREPO_CHANGE_URL EventType = "repo:changeURL"
const EventDataDescREPO_CHANGE_URL_D EventDataDesc = "gKey:string|rKey:string|from:string|to:string"
const EventTypeREPO_CREATE EventType = "repo:create"
const EventDataDescREPO_CREATE_D EventDataDesc = "gKey:string|rKey:string|name:string"
const EventDataDescREPO_D EventDataDesc = ""
const EventTypeREPO_FORK EventType = "repo:fork"
const EventDataDescREPO_FORK_D EventDataDesc = "gKey:string|rKey:string|forkFrom:string|name:string|sourceName:string"
const EventTypeREPO_REMOVE EventType = "repo:remove"
const EventDataDescREPO_REMOVE_D EventDataDesc = "gKey:string|rKey:string|name:string"
const EventTypeREPO_REMOVE_MEMBER EventType = "repo:removeMember"
const EventDataDescREPO_REMOVE_MEMBER_D EventDataDesc = "gKey:string|rKey:string|uid:string|name:string|email:string"
const EventTypeREPO_UPDATE_AVATAR EventType = "repo:updateAvator"
const EventDataDescREPO_UPDATE_AVATAR_D EventDataDesc = "gKey:string|rKey:string|from:string|to:string"
const EventTypeREPO_UPDATE_DESCRIPTION EventType = "repo:updateDescription"
const EventDataDescREPO_UPDATE_DESCRIPTION_D EventDataDesc = "gKey:string|rKey:string"
const EventTypeREPO_UPDATE_NAME EventType = "repo:updateName"
const EventDataDescREPO_UPDATE_NAME_D EventDataDesc = "gKey:string|rKey:string|from:string|to:string"
const EventTypeTAG_CREATE EventType = "tag:create"
const EventDataDescTAG_CREATE_D EventDataDesc = "gKey:string|rKey:string|name:string"
const EventTypeTAG_REMOVE EventType = "tag:remove"
const EventDataDescTAG_REMOVE_D EventDataDesc = "gKey:string|rKey:string|name:string"
const EventTypeWEBHOOK_CREATE EventType = "webhook:create"
const EventDataDescWEBHOOK_CREATE_D EventDataDesc = "gKey:string|rKey:string|rwKey:string"
const EventTypeWEBHOOK_DELETE EventType = "webhook:delete"
const EventDataDescWEBHOOK_DELETE_D EventDataDesc = "gKey:string|rKey:string|rwKey:string"
const EventTypeWEBHOOK_UPDATE EventType = "webhook:update"
const EventDataDescWEBHOOK_UPDATE_D EventDataDesc = "gKey:string|rKey:string|rwKey:string"
