package domain

import "time"

// User 是 cc_users 表的一行。
//
// 可空列一律用 *string 表示 —— PHP 的 row_array() 对 NULL 列返回 NULL，
// 而 JSON 编码后是 null 而不是空字符串。契约要求逐字一致，
// 因此不能用 string 零值代替 NULL。
type User struct {
	Key                string
	Email              string
	Password           string
	Name               string
	Team               *string
	Role               *string
	Tel                *string
	CallingCode        *string
	Location           *string
	Avatar             *string
	RegisterIP         *string
	Admin              bool
	TwoFA              *string
	NotificationStatus int
	Status             int
	Created            time.Time
	Updated            time.Time
}

// HasTwoFA 判断用户是否启用了双因素认证。
func (u *User) HasTwoFA() bool {
	return u.TwoFA != nil && *u.TwoFA != ""
}

// CommitEmail 是 cc_commit_emails 表的一行。
type CommitEmail struct {
	Key     string
	UKey    string
	Email   string
	IsCheck bool
	Status  int
	Created time.Time
}

// SSHKey 是 cc_ssh_keys 表的一行。
type SSHKey struct {
	Key     string
	UKey    string
	Name    string
	KeyType string
	KeyHash string
	KeyData string
	Status  int
	Created time.Time
	Updated time.Time
}

// CommitEmailView 是提交邮箱的归一化输出。
//
// PHP 侧 normalizeCommitEmails() 有两种形态：
//   - isCheck=TRUE  → 直接返回邮箱字符串数组
//   - isCheck=FALSE → 返回 {id, email, isCheck} 对象数组
//
// /api/user/info 走的是 isCheck=FALSE（对象形态），
// 而 /api/user/listKey 之外的提交邮箱列表走 isCheck=TRUE。
type CommitEmailView struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	IsCheck bool   `json:"isCheck"`
}

// NormalizedUser 是用户对外的归一化形态。
//
// 逐字对应 application/models/user_model.php:23-40 的 normalize()。
// 字段名与顺序不影响 JSON 语义，但**字段集合必须一致** —— 契约快照会逐字段比对。
type NormalizedUser struct {
	ID                 string  `json:"id"`
	Icon               *string `json:"icon"`
	Name               string  `json:"name"`
	Email              string  `json:"email"`
	PhoneCode          *string `json:"phoneCode"`
	PhoneNumber        *string `json:"phoneNumber"`
	Team               *string `json:"team"`
	Role               *string `json:"role"`
	Notification       int     `json:"notification"`
	MFAEnabled         bool    `json:"mfaEnabled"`
	Admin              bool    `json:"admin"`
	Emails             []any   `json:"emails"`
	UnReadNotification int     `json:"unReadNotification"`
	Status             bool    `json:"status"`
	Host               string  `json:"host"`
	SSH                string  `json:"ssh"`
}

// NormalizeUser 把用户行转换为对外形态。
//
// extra 为 true 时会带上未读通知数（PHP 侧的 $extra 参数），
// 并且提交邮箱以对象形态返回；为 false 时提交邮箱是字符串数组。
func NormalizeUser(u *User, emails []CommitEmail, extra bool, unRead int, host, ssh string) NormalizedUser {
	view := NormalizedUser{
		ID:                 u.Key,
		Icon:               u.Avatar,
		Name:               u.Name,
		Email:              u.Email,
		PhoneCode:          u.CallingCode,
		PhoneNumber:        u.Tel,
		Team:               u.Team,
		Role:               u.Role,
		Notification:       u.NotificationStatus,
		MFAEnabled:         u.HasTwoFA(),
		Admin:              u.Admin,
		Emails:             []any{},
		UnReadNotification: 0,
		Status:             u.Status == CommonStatusNormal,
		Host:               host,
		SSH:                ssh,
	}
	if extra {
		view.UnReadNotification = unRead
	}

	// 主邮箱恒排在首位且视为已验证（PHP 侧 array_unshift 的行为）。
	list := make([]CommitEmail, 0, len(emails)+1)
	list = append(list, CommitEmail{Key: "", Email: u.Email, IsCheck: true})
	list = append(list, emails...)

	if extra {
		// isCheck=FALSE → 对象形态。
		out := make([]any, 0, len(list))
		for _, e := range list {
			out = append(out, CommitEmailView{ID: e.Key, Email: e.Email, IsCheck: e.IsCheck})
		}
		view.Emails = out
	} else {
		// isCheck=TRUE → 仅返回已验证的邮箱字符串数组。
		out := make([]any, 0, len(list))
		for _, e := range list {
			if e.IsCheck {
				out = append(out, e.Email)
			}
		}
		view.Emails = out
	}
	return view
}

// NormalizedSSHKey 是 SSH 公钥的归一化形态。
//
// 逐字对应 application/models/user_sshkey_model.php:19-27。
type NormalizedSSHKey struct {
	ID         string `json:"id"`
	Creator    string `json:"creator"`
	Name       string `json:"name"`
	Hash       string `json:"hash"`
	Type       string `json:"type"`
	Created    int64  `json:"created"`
	DeleteFlag bool   `json:"deleteFlag"`
}

// NormalizeSSHKey 把 SSH 公钥行转换为对外形态。
//
// 注意 deleteFlag：PHP 原版写作 `!!($item['rm_status'] == COMMON_STATUS_NORMAL)`，
// 其中 rm_status 是 repository_members 的列，在 ssh_keys 行上根本不存在，
// 因此该表达式恒为 false。��里**原样复刻**该行为，以免改变前端可见的响应。
func NormalizeSSHKey(k *SSHKey) NormalizedSSHKey {
	return NormalizedSSHKey{
		ID:         k.Key,
		Creator:    k.UKey,
		Name:       k.Name,
		Hash:       k.KeyHash,
		Type:       k.KeyType,
		Created:    k.Created.Unix(),
		DeleteFlag: false,
	}
}

// CommonStatusNormal 对应 PHP 常量 COMMON_STATUS_NORMAL。
const CommonStatusNormal = 1
