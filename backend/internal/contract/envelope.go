package contract

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// Envelope 是 API 的统一响应信封。
//
// 字段顺序与拼写必须与 PHP 侧 Response::_response() 逐字一致：
//
//	{"request-id":"...","code":0,"message":"Success","data":{...}}
//
// 前端 www/view/src/helpers/NetworkHelper.js 读取 code/message/data，
// www/view/src/helpers/ErrorCodeParser.js 按 code 数值分支处理。
//
// 注意 data 在为空时必须是空数组 []（PHP 的空 array 编码结果），
// 而不能是 null —— 这会影响前端的 truthy 判断。
type Envelope struct {
	RequestID string `json:"request-id"`
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      any    `json:"data"`
}

// Paging 是可选的分页信息。
//
// PHP 侧只有在 Response::setTotal() 被调用时才输出该字段，
// 而 setTotal() 在现有代码中没有任何调用点，因此实际响应里从不出现 paging。
// 本实现默认同样不输出；调用方显式设置 Paging 后才会出现。
type Paging struct {
	Page     int `json:"page"`
	PageSize int `json:"pagesize"`
	Total    int `json:"total"`
}

// NewSuccessEnvelope 构造成功信封。
//
// data 为 nil 时用空数组填充，以复刻 PHP 中 Response::output([]) 的输出。
func NewSuccessEnvelope(requestID string, data any) *Envelope {
	if data == nil {
		data = []any{}
	}
	return &Envelope{
		RequestID: requestID,
		Code:      int(CodeSuccess),
		Message:   CodeSuccess.Message(),
		Data:      data,
	}
}

// NewErrorEnvelope 构造错误信封。
func NewErrorEnvelope(requestID string, e *APIError) *Envelope {
	data := e.Data
	if data == nil {
		data = []any{}
	}
	return &Envelope{
		RequestID: requestID,
		Code:      int(e.Code),
		Message:   e.Message(),
		Data:      data,
	}
}

// WriteJSON 写出 JSON 响应，响应头与 PHP 侧保持一致。
//
// PHP 的 _response() 设置：
//
//	HTTP/1.1 <status> <reason>
//	Cache-Control: no-cache, must-revalidate
//	Content-Type: application/json
//	Content-Length: <n>
//	Request-ID: <uuid>
func WriteJSON(w http.ResponseWriter, requestID string, status int, body any) {
	payload, err := json.Marshal(body)
	if err != nil {
		// 序列化失败不应让整个服务崩溃；返回一个契约内的错误信封。
		payload = []byte(`{"request-id":"` + requestID + `","code":256,"message":"Request Error","data":[]}`)
		status = http.StatusInternalServerError
	}
	h := w.Header()
	h.Set("Cache-Control", "no-cache, must-revalidate")
	h.Set("Content-Type", "application/json")
	h.Set("Content-Length", strconv.Itoa(len(payload)))
	h.Set("Request-ID", requestID)
	w.WriteHeader(status)
	_, _ = w.Write(payload)
}

// WriteSuccess 写出成功响应。
func WriteSuccess(w http.ResponseWriter, requestID string, data any) {
	WriteJSON(w, requestID, http.StatusOK, NewSuccessEnvelope(requestID, data))
}

// WriteError 写出错误响应。
func WriteError(w http.ResponseWriter, requestID string, err error) {
	ae := AsAPIError(err)
	WriteJSON(w, requestID, ae.HTTPStatus(), NewErrorEnvelope(requestID, ae))
}
