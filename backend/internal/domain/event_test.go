package domain

import "testing"

// TestValidateEventData 锁定事件数据校验语义。
//
// 逐字复刻 application/libraries/Service.php:74-115 的 _validateEvent()。
// 这里的规则很微妙（类型前缀匹配、"?" 表示可缺省），一旦偏离就会导致
// 事件被静默丢弃或错误抛出，因此逐条断言。
func TestValidateEventData(t *testing.T) {
	cases := []struct {
		name    string
		desc    EventDataDesc
		data    map[string]any
		wantErr bool
		code    int
	}{
		{
			name: "全部必填字段齐备且类型正确",
			desc: "gKey:string|rKey:string|name:string",
			data: map[string]any{"gKey": "g1", "rKey": "r1", "name": "n"},
		},
		{
			name:    "缺少必填字段",
			desc:    "gKey:string|rKey:string",
			data:    map[string]any{"gKey": "g1"},
			wantErr: true,
			code:    2003,
		},
		{
			name:    "类型不符",
			desc:    "to:int",
			data:    map[string]any{"to": "not-an-int"},
			wantErr: true,
			code:    2003,
		},
		{
			name: "可缺省字段缺失",
			desc: "gKey:string|to:int?",
			data: map[string]any{"gKey": "g1"},
		},
		{
			name: "通配类型接受任意值",
			desc: "payload:*",
			data: map[string]any{"payload": map[string]any{"x": 1}},
		},
		{
			name: "空描述不校验",
			desc: "",
			data: map[string]any{},
		},
		{
			name:    "描述项格式非法",
			desc:    "gKeyString",
			data:    map[string]any{},
			wantErr: true,
			code:    2004,
		},
		{
			name: "描述中的空项被跳过",
			desc: "gKey:string||rKey:string",
			data: map[string]any{"gKey": "g", "rKey": "r"},
		},
	}

	for _, c := range cases {
		err := ValidateEventData(c.desc, c.data)
		if c.wantErr {
			if err == nil {
				t.Errorf("%s: 期望报错，实际通过", c.name)
				continue
			}
			ee, ok := err.(*EventError)
			if !ok {
				t.Errorf("%s: 期望 *EventError，实际 %T", c.name, err)
				continue
			}
			if ee.Code != c.code {
				t.Errorf("%s: 错误码 = %d, want %d", c.name, ee.Code, c.code)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: 期望通过，实际 %v", c.name, err)
		}
	}
}

// TestEventRegistryMatchesPHP 锁定事件注册表的规模与关键条目。
//
// 事件注册表由脚本从 PHP 源生成，这里断言关键条目与总数，
// 防止生成脚本或后续编辑悄悄改坏。
func TestEventRegistryMatchesPHP(t *testing.T) {
	cases := map[string]EventType{
		"GROUP_CREATE":                  "group:create",
		"GROUP_ADD_MEMBER":              "group:addMember",
		"GROUP_CHANGE_MEMBER_ROLE":      "group:changeMemberRole",
		"GROUP_CHANGE_OWNER":            "group:changeOwner",
		"GROUP_REMOVE_MEMBER":           "group:removeMember",
		"GROUP_UPDATE_AVATAR":           "group:updateAvator",
		"GROUP_UPDATE_NAME":             "group:updateName",
		"GROUP_UPDATE_DESCRIPTION":      "group:updateDescription",
		"REPO_CREATE":                   "repo:create",
		"REPO_FORK":                     "repo:fork",
		"REPO_REMOVE":                   "repo:remove",
		"REPO_ADD_MEMBER":               "repo:addMember",
		"REPO_CHANGE_MEMBER_ROLE":       "repo:changeMemberRole",
		"REPO_CHANGE_OWNER":             "repo:changeOwner",
		"REPO_UPDATE_AVATAR":            "repo:updateAvator",
		"REPO_UPDATE_NAME":              "repo:updateName",
		"REPO_UPDATE_DESCRIPTION":       "repo:updateDescription",
		"REPO_CHANGE_URL":               "repo:changeURL",
		"BRANCH_CREATE":                 "branch:create",
		"BRANCH_REMOVE":                 "branch:remove",
		"TAG_CREATE":                    "tag:create",
		"TAG_REMOVE":                    "tag:remove",
		"DEFAULT_BRANCH_CHANGE":         "branch:changeDefaultBranch",
		"PROTECTED_BRANCH_RULE_CREATE":  "branch:createProtectedBranchRule",
		"PROTECTED_BRANCH_RULE_CHANGE":  "branch:changeProtectedBranchRule",
		"PROTECTED_BRANCH_RULE_REMOVE":  "branch:removeProtectedBranchRule",
		"MERGE_REQUEST_CREATE":          "mergeRequest:create",
		"MERGE_REQUEST_CLOSE":           "mergeRequest:close",
		"MERGE_REQUEST_MERGE":           "mergeRequest:merge",
		"MERGE_REQUEST_REVIEWER_CREATE": "mergeRequestReviewer:create",
		"MERGE_REQUEST_REVIEWER_REVIEW": "mergeRequestReviewer:review",
		"MERGE_REQUEST_REVIEWER_DELETE": "mergeRequestReviewer:delete",
		"WEBHOOK_CREATE":                "webhook:create",
		"WEBHOOK_UPDATE":                "webhook:update",
		"WEBHOOK_DELETE":                "webhook:delete",
		"HOOK_POST_RECEIVE":             "hook:postReceive",
	}
	for name, want := range cases {
		var got EventType
		switch name {
		case "GROUP_CREATE":
			got = EventTypeGROUP_CREATE
		case "GROUP_ADD_MEMBER":
			got = EventTypeGROUP_ADD_MEMBER
		case "GROUP_CHANGE_MEMBER_ROLE":
			got = EventTypeGROUP_CHANGE_MEMBER_ROLE
		case "GROUP_CHANGE_OWNER":
			got = EventTypeGROUP_CHANGE_OWNER
		case "GROUP_REMOVE_MEMBER":
			got = EventTypeGROUP_REMOVE_MEMBER
		case "GROUP_UPDATE_AVATAR":
			got = EventTypeGROUP_UPDATE_AVATAR
		case "GROUP_UPDATE_NAME":
			got = EventTypeGROUP_UPDATE_NAME
		case "GROUP_UPDATE_DESCRIPTION":
			got = EventTypeGROUP_UPDATE_DESCRIPTION
		case "REPO_CREATE":
			got = EventTypeREPO_CREATE
		case "REPO_FORK":
			got = EventTypeREPO_FORK
		case "REPO_REMOVE":
			got = EventTypeREPO_REMOVE
		case "REPO_ADD_MEMBER":
			got = EventTypeREPO_ADD_MEMBER
		case "REPO_CHANGE_MEMBER_ROLE":
			got = EventTypeREPO_CHANGE_MEMBER_ROLE
		case "REPO_CHANGE_OWNER":
			got = EventTypeREPO_CHANGE_OWNER
		case "REPO_UPDATE_AVATAR":
			got = EventTypeREPO_UPDATE_AVATAR
		case "REPO_UPDATE_NAME":
			got = EventTypeREPO_UPDATE_NAME
		case "REPO_UPDATE_DESCRIPTION":
			got = EventTypeREPO_UPDATE_DESCRIPTION
		case "REPO_CHANGE_URL":
			got = EventTypeREPO_CHANGE_URL
		case "BRANCH_CREATE":
			got = EventTypeBRANCH_CREATE
		case "BRANCH_REMOVE":
			got = EventTypeBRANCH_REMOVE
		case "TAG_CREATE":
			got = EventTypeTAG_CREATE
		case "TAG_REMOVE":
			got = EventTypeTAG_REMOVE
		case "DEFAULT_BRANCH_CHANGE":
			got = EventTypeDEFAULT_BRANCH_CHANGE
		case "PROTECTED_BRANCH_RULE_CREATE":
			got = EventTypePROTECTED_BRANCH_RULE_CREATE
		case "PROTECTED_BRANCH_RULE_CHANGE":
			got = EventTypePROTECTED_BRANCH_RULE_CHANGE
		case "PROTECTED_BRANCH_RULE_REMOVE":
			got = EventTypePROTECTED_BRANCH_RULE_REMOVE
		case "MERGE_REQUEST_CREATE":
			got = EventTypeMERGE_REQUEST_CREATE
		case "MERGE_REQUEST_CLOSE":
			got = EventTypeMERGE_REQUEST_CLOSE
		case "MERGE_REQUEST_MERGE":
			got = EventTypeMERGE_REQUEST_MERGE
		case "MERGE_REQUEST_REVIEWER_CREATE":
			got = EventTypeMERGE_REQUEST_REVIEWER_CREATE
		case "MERGE_REQUEST_REVIEWER_REVIEW":
			got = EventTypeMERGE_REQUEST_REVIEWER_REVIEW
		case "MERGE_REQUEST_REVIEWER_DELETE":
			got = EventTypeMERGE_REQUEST_REVIEWER_DELETE
		case "WEBHOOK_CREATE":
			got = EventTypeWEBHOOK_CREATE
		case "WEBHOOK_UPDATE":
			got = EventTypeWEBHOOK_UPDATE
		case "WEBHOOK_DELETE":
			got = EventTypeWEBHOOK_DELETE
		case "HOOK_POST_RECEIVE":
			got = EventTypeHOOK_POST_RECEIVE
		}
		if got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

// TestEventDataDescriptions 锁定几个事件的数据描述（校验规则的输入）。
func TestEventDataDescriptions(t *testing.T) {
	cases := map[string]EventDataDesc{
		"GROUP_CREATE_D":                 "gKey:string|name:string",
		"GROUP_ADD_MEMBER_D":             "gKey:string|uid:string|name:string|email:string",
		"GROUP_CHANGE_MEMBER_ROLE_D":     "gKey:string|uid:string|name:string|email:string|to:int",
		"REPO_CREATE_D":                  "gKey:string|rKey:string|name:string",
		"BRANCH_CREATE_D":                "gKey:string|rKey:string|name:string",
		"DEFAULT_BRANCH_CHANGE_D":        "gKey:string|rKey:string|from:string|to:string",
		"PROTECTED_BRANCH_RULE_CREATE_D": "gKey:string|rKey:string|name:string",
	}
	for name, want := range cases {
		var got EventDataDesc
		switch name {
		case "GROUP_CREATE_D":
			got = EventDataDescGROUP_CREATE_D
		case "GROUP_ADD_MEMBER_D":
			got = EventDataDescGROUP_ADD_MEMBER_D
		case "GROUP_CHANGE_MEMBER_ROLE_D":
			got = EventDataDescGROUP_CHANGE_MEMBER_ROLE_D
		case "REPO_CREATE_D":
			got = EventDataDescREPO_CREATE_D
		case "BRANCH_CREATE_D":
			got = EventDataDescBRANCH_CREATE_D
		case "DEFAULT_BRANCH_CHANGE_D":
			got = EventDataDescDEFAULT_BRANCH_CHANGE_D
		case "PROTECTED_BRANCH_RULE_CREATE_D":
			got = EventDataDescPROTECTED_BRANCH_RULE_CREATE_D
		}
		if got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

// TestNormalizeEventData 确认数值类型被收敛为 JSON 友好的形态。
func TestNormalizeEventData(t *testing.T) {
	got := NormalizeEventData(map[string]any{
		"i":  3,
		"f":  float64(2),
		"fs": 1.5,
		"s":  "x",
		"b":  true,
	})
	if v, ok := got["i"].(int64); !ok || v != 3 {
		t.Errorf("int 应归一为 int64，实际 %#v", got["i"])
	}
	if v, ok := got["f"].(int64); !ok || v != 2 {
		t.Errorf("整数值 float 应归一为 int64，实际 %#v", got["f"])
	}
	if v, ok := got["fs"].(float64); !ok || v != 1.5 {
		t.Errorf("小数值 float 应保留 float64，实际 %#v", got["fs"])
	}
	if v, ok := got["s"].(string); !ok || v != "x" {
		t.Errorf("string 应保留，实际 %#v", got["s"])
	}
}
