package domain

// AccessToken 是 cc_access_tokens 表的一行。
//
// 对应公开 API（/api/v1）的访问令牌。**明文令牌不入库** ——
// 表里只存 SHA-256（at_sha256），明文仅在创建时返回一次。
//
// 权限模型：令牌 ≡ 用户。令牌本身不带 scope，
// 权限完全继承所属用户（at_scope 预留给未来的细粒度控制）。
type AccessToken struct {
	// Key 是令牌主键（at_key）。
	Key string
	// UKey 是令牌所属用户的主键（u_key）。
	UKey string
	// Name 是用户给令牌起的名字（at_name）。
	Name string
	// SHA256 是令牌明文的 SHA-256 十六进制摘要（at_sha256）。
	SHA256 string
	// Scope 是预留的权限范围（at_scope），当前恒为空串。
	Scope string
	// LastUsed 是最后一次成功使用时间（at_last_used），可空。
	LastUsed *string
	// Created / Updated 是创建与更新时间。
	Created string
	Updated string
	// Status 是软删除状态（at_status）。
	Status int
}
