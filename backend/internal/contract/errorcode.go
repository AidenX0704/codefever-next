// Package contract 实现 CodeFever API 的外部契约兼容层。
//
// 本包是"后端重写但前端零改动"的技术前提：所有对外部可见的行为
// （响应信封、错误码数值与 HTTP 状态映射、请求头校验、参数来源）都收敛在此，
// 其余业务代码可以自由重构。
//
// 契约来源：application/libraries/service/ErrorCode/ErrorCode.php 与
// application/libraries/service/Network/{Request,Response}.php。
package contract

import (
	"errors"
	"fmt"
	"net/http"
)

// ErrorCode 是 API 对外暴露的错误码。
//
// 数值与 message 字符串必须与 PHP 侧逐字一致 —— 前端
// www/view/src/helpers/ErrorCodeParser.js 按数值分支处理（0x0101 触发跳转登录、
// code < 0x0400 视为业务错误、code > 0x0400 视为系统错误），任何偏移都会导致前端行为错误。
type ErrorCode int

const (
	// 0x0000 成功。
	CodeSuccess ErrorCode = 0x0000

	// 0x01xx 请求与认证错误。
	CodeRequestError      ErrorCode = 0x0100
	CodeInvalidCredential ErrorCode = 0x0101
	CodeMethodNotAllowed  ErrorCode = 0x0102
	CodeHeaderRequired    ErrorCode = 0x0103
	CodeContentLength     ErrorCode = 0x0104
	CodeEndpointNotFound  ErrorCode = 0x0105
	CodeActionNotAllowed  ErrorCode = 0x0106

	// 0x02xx 参数错误。
	CodeEmpty            ErrorCode = 0x0200
	CodeMissingParameter ErrorCode = 0x0201
	CodeAmountOutOfRange ErrorCode = 0x0202

	// 0x03xx 通用业务错误。
	CodeGroupNotFound      ErrorCode = 0x0301
	CodeRepositoryNotFound ErrorCode = 0x0302

	// 0x04xx 具体业务错误。
	CodeDuplicateRepoSlug        ErrorCode = 0x0401
	CodeDuplicateRepoDisplayName ErrorCode = 0x0402
	CodeDuplicateGroupSlug       ErrorCode = 0x0403
	CodeDuplicateGroupDisplay    ErrorCode = 0x0404
	CodeOperateFail              ErrorCode = 0x0405
	CodeTargetExists             ErrorCode = 0x0406
	CodeUserNotFound             ErrorCode = 0x0407
	CodeCannotAddCreator         ErrorCode = 0x0408
	CodeCannotDeleteUserGroup    ErrorCode = 0x0409
	CodeCannotDeleteNonEmpty     ErrorCode = 0x040A
	CodeCannotChangeOwnerOfGroup ErrorCode = 0x040B
	CodeOldTargetNotExists       ErrorCode = 0x040C
	CodeDuplicateSSHKey          ErrorCode = 0x040D
	CodeFileNotFound             ErrorCode = 0x040E
	CodeBranchProtected          ErrorCode = 0x040F
	CodePasswordNotCorrect       ErrorCode = 0x0410
	CodeMFAInvalid               ErrorCode = 0x0411
	CodeMFAInvalidSecond         ErrorCode = 0x0412
)

// codeMeta 逐字搬运自 PHP 侧 ErrorCode::$errorCode，格式为 "HTTP状态码|message"。
type codeMeta struct {
	status  int
	message string
}

var codeTable = map[ErrorCode]codeMeta{
	CodeSuccess:                  {http.StatusOK, "Success"},
	CodeRequestError:             {http.StatusForbidden, "Request Error"},
	CodeInvalidCredential:        {http.StatusForbidden, "Invalid Login Credential"},
	CodeMethodNotAllowed:         {http.StatusBadRequest, "Method Not Allowed"},
	CodeHeaderRequired:           {http.StatusBadRequest, "Accept and Content-Type Is Required In Header"},
	CodeContentLength:            {http.StatusBadRequest, "Content-Length Is Not Match The Content Size"},
	CodeEndpointNotFound:         {http.StatusNotFound, "Endpoint Not Found Or Not Allowed Current Request Method"},
	CodeActionNotAllowed:         {http.StatusForbidden, "Action Not Allowed"},
	CodeEmpty:                    {http.StatusBadRequest, "empty"},
	CodeMissingParameter:         {http.StatusBadRequest, "Missing Parameters"},
	CodeAmountOutOfRange:         {http.StatusBadRequest, "Missing Amount Or Amount Out Of Range"},
	CodeGroupNotFound:            {http.StatusNotFound, "Group Not Found"},
	CodeRepositoryNotFound:       {http.StatusNotFound, "Repository Not Found"},
	CodeDuplicateRepoSlug:        {http.StatusBadRequest, "Duplicate Repository URL Slug"},
	CodeDuplicateRepoDisplayName: {http.StatusBadRequest, "Duplicate Repository Display Name"},
	CodeDuplicateGroupSlug:       {http.StatusBadRequest, "Duplicate Group URL Slug"},
	CodeDuplicateGroupDisplay:    {http.StatusBadRequest, "Duplicate Group Display Name"},
	CodeOperateFail:              {http.StatusBadRequest, "Operate Fail"},
	CodeTargetExists:             {http.StatusBadRequest, "New Target Exists"},
	CodeUserNotFound:             {http.StatusNotFound, "User Not Found"},
	CodeCannotAddCreator:         {http.StatusNotFound, "Can Not Add Creator As Member"},
	CodeCannotDeleteUserGroup:    {http.StatusBadRequest, "Can Not Delete User Group"},
	CodeCannotDeleteNonEmpty:     {http.StatusBadRequest, "Can Not Non-empty Group"},
	CodeCannotChangeOwnerOfGroup: {http.StatusBadRequest, "Can Not Change Owner Of User Group"},
	CodeOldTargetNotExists:       {http.StatusBadRequest, "Old Target Not Exists"},
	CodeDuplicateSSHKey:          {http.StatusBadRequest, "Duplicate SSH Key"},
	CodeFileNotFound:             {http.StatusNotFound, "File Not Found"},
	CodeBranchProtected:          {http.StatusForbidden, "Branch Proteced"},
	CodePasswordNotCorrect:       {http.StatusForbidden, "Password Not Correct"},
	CodeMFAInvalid:               {http.StatusForbidden, "MFA Code Invalid"},
	CodeMFAInvalidSecond:         {http.StatusForbidden, "MFA Code Invalid (2)"},
}

// HTTPStatus 返回错误码对应的 HTTP 状态码。
func (c ErrorCode) HTTPStatus() int {
	if m, ok := codeTable[c]; ok {
		return m.status
	}
	return http.StatusInternalServerError
}

// Message 返回错误码对应的英文消息（与 PHP 侧逐字一致）。
func (c ErrorCode) Message() string {
	if m, ok := codeTable[c]; ok {
		return m.message
	}
	return "Internal Server Error"
}

// Error 让 ErrorCode 自身实现 error，便于 handler 直接返回。
func (c ErrorCode) Error() string {
	return fmt.Sprintf("contract error 0x%04X: %s", int(c), c.Message())
}

// APIError 是携带额外数据与附加说明的业务错误。
//
// ExtraMessage 对应 PHP 的 Response::reject($code, $extraMessage, $data)：
// 传入时 message 变为 "<原始message> >>> Extra: <extra>"。
type APIError struct {
	Code        ErrorCode
	ExtraMsg    string
	Data        any
	HasExtraMsg bool
}

// Error 实现 error 接口。
func (e *APIError) Error() string { return e.Code.Error() }

// NewError 构造一个不带附加说明的业务错误。
func NewError(code ErrorCode) *APIError {
	return &APIError{Code: code}
}

// NewErrorWithData 构造一个携带 data 负载的业务错误。
//
// 用于 internal.hooks 的保护分支拒绝：0x040F + data.remote 会被 git 钩子
// 原样打印给客户端。
func NewErrorWithData(code ErrorCode, data any) *APIError {
	return &APIError{Code: code, Data: data}
}

// NewErrorWithExtra 构造一个带附加说明的业务错误。
func NewErrorWithExtra(code ErrorCode, extra string) *APIError {
	return &APIError{Code: code, ExtraMsg: extra, HasExtraMsg: true}
}

// HTTPStatus 返回该错误对应的 HTTP 状态码。
func (e *APIError) HTTPStatus() int { return e.Code.HTTPStatus() }

// Message 返回最终写入响应体的 message 字段。
func (e *APIError) Message() string {
	if e.HasExtraMsg {
		return e.Code.Message() + " >>> Extra: " + e.ExtraMsg
	}
	return e.Code.Message()
}

// AsAPIError 把任意 error 归一化为 APIError。
//
// 非 APIError 一律视为内部错误（0x0400 段），避免把实现细节泄漏给前端。
func AsAPIError(err error) *APIError {
	if err == nil {
		return nil
	}
	var ae *APIError
	if errors.As(err, &ae) {
		return ae
	}
	return &APIError{Code: CodeRequestError, ExtraMsg: err.Error(), HasExtraMsg: true}
}
