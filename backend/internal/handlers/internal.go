// 本文件实现 /apiv1/internal/* 的四个内部网关回调端点。
//
// 这四个端点是 Go 网关（http-gateway / ssh-gateway）与 git 钩子
// （misc/hooks/main）访问后端的**唯一通道**，因此：
//
//  1. 请求方法与参数名必须与既有调用方发出的请求完全一致，
//     调用方不可修改；
//  2. 响应体是跨进程硬契约 —— 特别是 uid 字段的 "<u_key>|<r_key>" 格式，
//     改错会导致 push 后端找不到用户与仓库；
//  3. 状态码语义被 Go 网关二次改写（见 http-gateway/Request/main.go:144-150）：
//     除非后端返回 404，否则任何失败都会被改写成 401。
//     因此"仓库/分组不存在"必须用 404 / 0x0302 表达。
//
// 逐字对应 PHP 侧 application/controllers/apiv1/internal.php。
package handlers

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"codefever-next/backend/internal/auth"
	"codefever-next/backend/internal/config"
	"codefever-next/backend/internal/contract"
	"codefever-next/backend/internal/domain"
	"codefever-next/backend/internal/httpapi"
	"codefever-next/backend/internal/repo"
)

// gitStorageURL 是 git 裸仓库的对外访问地址。
//
// 逐字对应 application/controllers/apiv1/internal.php:120 的硬编码值。
const gitStorageURL = "http://127.0.0.1:27555"

// protectedBranchRemoteTemplate 是保护分支拒绝时返回给 git 客户端的提示。
//
// 逐字对应 application/controllers/apiv1/internal.php:172：
//
//	"\n[CodeFever Next]: You are not allowed to push code to protected branch <$head> !"
//
// 这段文本会被 misc/hooks/main 原样打印给客户端，因此一字都不能改。
const protectedBranchRemoteTemplate = "\n[CodeFever Next]: You are not allowed to push code to protected branch <%s> !"

// InternalHandler 实现 /apiv1/internal/* 端点。
type InternalHandler struct {
	// Guard 提供角色解析（RepositoryRole）。
	Guard *Guard
	// Repos 提供内部网关回调专用的查询（仓库组、仓库、保护分支规则、SSH 公钥）。
	Repos *repo.InternalRepo
	// Users 提供按邮箱查用户。
	Users *repo.UserRepo
	// Cfg 提供 git-storage 根目录。
	Cfg *config.Config
}

// NewInternalHandler 创建内部网关回调处理器。
func NewInternalHandler(cfg *config.Config, guard *Guard, repos *repo.InternalRepo, users *repo.UserRepo) *InternalHandler {
	return &InternalHandler{Guard: guard, Repos: repos, Users: users, Cfg: cfg}
}

// Register 把内部网关回调端点注册到服务器。
//
// 鉴权级别是 AuthInternalGateway —— Token 头的校验由 httpapi 分发器
// 在调用 handler **之前**完成（见 httpapi/dispatch.go:162），
// 因此 handler 内不再重复校验令牌。
func (h *InternalHandler) Register(s *httpapi.Server) {
	actions := map[string]map[string]httpapi.ActionHandler{
		// authHTTP / authSSH / searchPublicKey 用 GET + JSON body
		// （PHP 侧 Request::parse() 对 contentType=json 读 php://input）。
		"authHTTP":        {"GET": h.authHTTP},
		"authSSH":         {"GET": h.authSSH},
		"searchPublicKey": {"GET": h.searchPublicKey},
		// hooks 用 POST + JSON body。
		"hooks": {"POST": h.hooks},
	}

	s.Register(&httpapi.Controller{
		Name:    "internal",
		Auth:    httpapi.AuthInternalGateway,
		Actions: actions,
	})
}

// authHTTPResult 是 authHTTP 的 data 负载。
//
// 字段顺序必须与 PHP 侧 Response::output 的数组顺序一致 ——
// PHP 的 json_encode 按插入顺序输出键，Go 的 encoding/json 按结构体字段
// 顺序输出，两者必须对齐才能得到逐字相同的响应体。
type authHTTPResult struct {
	Storage string `json:"storage"`
	UID     string `json:"uid"`
	User    string `json:"user"`
	Addr    string `json:"addr"`
	Path    string `json:"path"`
}

// authSSHResult 是 authSSH 的 data 负载。
//
// 注意 PHP 侧只输出 uid 与 path 两个键，**没有** action 键
// （application/controllers/apiv1/internal.php:64-67）。
type authSSHResult struct {
	UID  string `json:"uid"`
	Path string `json:"path"`
}

// hookResult 是 hooks 的 data 负载。
type hookResult struct {
	Remote string `json:"remote"`
}

// publicKeyResult 是 searchPublicKey 的 data 数组元素。
type publicKeyResult struct {
	UserID    string `json:"userID"`
	KeyType   string `json:"keyType"`
	PublicKey string `json:"publicKey"`
}

// authHTTP 实现 GET /apiv1/internal/authHTTP。
//
// 逐字对应 application/controllers/apiv1/internal.php:81-140：
//
//	缺 user / pass / repo 或 repo 解析不出组名+仓库名 → 0x0201
//	邮箱或口令不匹配                              → 0x0101
//	仓库组或仓库不存在                            → 0x0302（HTTP 404）
//	用户不是仓库成员 / 动作无权限                  → 0x0106
//	成功 → {storage, uid, user, addr, path}
//
// 失败码的选取不是随意的：http-gateway 会把除 404 以外的所有失败
// 改写成 401，因此"仓库不存在"必须用 404 / 0x0302 才能被网关原样透出。
func (h *InternalHandler) authHTTP(ctx context.Context, req *httpapi.Request) (any, error) {
	p := req.Parsed
	user := p.JSONString("user")
	pass := p.JSONString("pass")
	repoRaw := p.JSONString("repo")
	action := p.JSONString("action")

	// PHP 侧判定的是 $data['user'] && $data['pass'] && $data['repo']
	// && $parsedRepo[1] && $parsedRepo[2]；action 不在缺参条件里
	//（缺 action 时 PHP 侧 $data['action'] 触发未定义下标，CI 错误处理器
	//  直接输出空白 500 页 —— 实测确认；这里按空 action 处理，
	//  等价于 PHP 的 REPO_ACTION_MAPPING[空串] = READ）。
	ref, refOK := domain.ParseRepositoryRef(repoRaw)
	if !domain.PHPTruthy(user) || !domain.PHPTruthy(pass) || !domain.PHPTruthy(repoRaw) || !refOK {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	// 按邮箱查用户并校验口令。
	//
	// PHP 侧用 md5(md5($pass)) == $result['u_password']，
	// 新后端用 auth.VerifyPassword 同时支持旧版双层 md5 与 argon2id。
	// 这里**不**写回新哈希 —— PHP 侧该端点是纯读，hooks 回调也不允许写库。
	userInfo, err := h.Users.GetByEmail(ctx, user)
	if err != nil {
		if !errors.Is(err, repo.ErrNotFound) {
			return nil, err
		}
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}
	if ok, _ := auth.VerifyPassword(userInfo.Password, pass); !ok {
		return nil, contract.NewError(contract.CodeInvalidCredential)
	}

	repoData, err := h.lookupRepository(ctx, ref)
	if err != nil {
		return nil, err
	}
	if repoData == nil {
		return nil, contract.NewError(contract.CodeRepositoryNotFound)
	}

	// 角色与动作权限。
	role, err := h.repositoryRole(ctx, repoData.Key, userInfo.Key)
	if err != nil {
		return nil, err
	}
	if role == domain.RoleNoPermission || !domain.CheckRepositoryAction(role, action) {
		return nil, contract.NewError(contract.CodeActionNotAllowed)
	}

	// 邮箱拆成 "用户名@域"：给 http-gateway 的 REMOTE_USER / REMOTE_ADDR。
	//
	// 逐字对应 internal.php:114-117：拆不出两段时回退到 [u_key, 'pgyer_user']。
	name, addr := splitEmail(userInfo.Email)
	if name == "" || addr == "" {
		name, addr = userInfo.Key, "pgyer_user"
	}

	return authHTTPResult{
		Storage: gitStorageURL,
		UID:     userInfo.Key + "|" + repoData.Key,
		User:    name,
		Addr:    addr,
		Path:    h.repositoryPath(repoData),
	}, nil
}

// authSSH 实现 GET /apiv1/internal/authSSH。
//
// 逐字对应 application/controllers/apiv1/internal.php:39-79：
//
//	缺 userID / repo / action 或 repo 解析不出组名+仓库名 → 0x0201
//	仓库组或仓库不存在                              → 0x0302（HTTP 404）
//	用户不是仓库成员 / 动作无权限                    → 0x0106
//	成功 → {uid, path}
//
// 与 authHTTP 的差别：uid 直接用请求里的 userID，不再按邮箱查用户；
// 且 action 是必填项。
func (h *InternalHandler) authSSH(ctx context.Context, req *httpapi.Request) (any, error) {
	p := req.Parsed
	userID := p.JSONString("userID")
	repoRaw := p.JSONString("repo")
	action := p.JSONString("action")

	ref, refOK := domain.ParseRepositoryRef(repoRaw)
	if !domain.PHPTruthy(userID) || !domain.PHPTruthy(repoRaw) || !domain.PHPTruthy(action) || !refOK {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	repoData, err := h.lookupRepository(ctx, ref)
	if err != nil {
		return nil, err
	}
	if repoData == nil {
		return nil, contract.NewError(contract.CodeRepositoryNotFound)
	}

	role, err := h.repositoryRole(ctx, repoData.Key, userID)
	if err != nil {
		return nil, err
	}
	if role == domain.RoleNoPermission || !domain.CheckRepositoryAction(role, action) {
		return nil, contract.NewError(contract.CodeActionNotAllowed)
	}

	return authSSHResult{
		UID:  userID + "|" + repoData.Key,
		Path: h.repositoryPath(repoData),
	}, nil
}

// hooks 实现 POST /apiv1/internal/hooks。
//
// 逐字对应 application/controllers/apiv1/internal.php:142-184：
//
//	args 为空或 uid 为空 / uid 拆不出两段 / 钩子名为空 → 0x0201 + data.remote = ""
//	post-receive → 逐行解析 stdin（本阶段不发事件，见下方 TODO）
//	pre-receive  → 命中保护分支 → 0x040F + data.remote = 提示文本
//	其他钩子名（含 update）→ 静默成功，data.remote = ""
//
// data.remote 会被 misc/hooks/main 原样打印给 git 客户端，文本必须逐字一致。
func (h *InternalHandler) hooks(ctx context.Context, req *httpapi.Request) (any, error) {
	p := req.Parsed
	args := jsonArray(p, "args")
	uid := p.JSONString("uid")

	// PHP 侧 count($data['args']) > 0 && $data['uid']。
	if len(args) == 0 || !domain.PHPTruthy(uid) {
		return nil, missingHookParameter()
	}

	uKey, rKey, ok := domain.ParseUID(uid)
	if !ok {
		return nil, missingHookParameter()
	}

	// 钩子名 = 脚本路径的 basename。
	hookName := domain.HookName(phpString(args[0]))
	if hookName == "" {
		return nil, missingHookParameter()
	}

	stdin := p.JSONString("stdin")
	switch hookName {
	case "post-receive":
		pushes := domain.ParseHookPushLines(stdin)
		// TODO: 发送 HOOK_POST_RECEIVE 事件（写 activities 表）。
		//
		// PHP 侧 application/controllers/apiv1/internal.php:150-162 对每一行
		// （恰好 3 段）调用一次：
		//
		//	$this->service->newEvent('HOOK_POST_RECEIVE', [
		//	    'rKey' => $rKey,
		//	    'name' => $pushInfo[2],   // 被更新的引用
		//	    'from' => $pushInfo[0],   // 推送前的 commit
		//	    'to'   => $pushInfo[1]    // 推送后的 commit
		//	], $uKey);
		//
		// 即：事件名 HOOK_POST_RECEIVE，载荷 {rKey, name, from, to}，
		// 触发者为 uKey，每条合法的 push 记录发一次。
		_ = pushes

	case "pre-receive":
		for _, push := range domain.ParseHookPushLines(stdin) {
			// 逐字对应 internal.php:168：去掉 refs/heads/ 前缀得到分支名。
			head := domain.BranchFromRef(push.Ref)
			if head == "" {
				continue
			}
			protected, err := h.isBranchProtected(ctx, rKey, uKey, head)
			if err != nil {
				return nil, err
			}
			if protected {
				return nil, contract.NewErrorWithData(contract.CodeBranchProtected, map[string]any{
					"remote": sprintfProtectedBranchRemote(head),
				})
			}
		}

	default:
		// PHP 侧只实现了 post-receive 与 pre-receive；
		// update 等其他钩子名走到 Response::output(['remote' => ''])，
		// 即**静默成功**。这里保持一致。
	}

	return hookResult{Remote: ""}, nil
}

// searchPublicKey 实现 GET /apiv1/internal/searchPublicKey。
//
// 逐字对应 application/controllers/apiv1/internal.php:21-37：
//
//	缺 fingerprint 或 publicKey → 0x0201
//	公钥未登记                  → 0x0101
//	成功 → 数组 [{"userID","keyType","publicKey"}]
//
// 注意 data 是**数组**（其余三个端点是对象），这是 PHP 侧
// Response::output([[...]]) 的直接结果。
func (h *InternalHandler) searchPublicKey(ctx context.Context, req *httpapi.Request) (any, error) {
	p := req.Parsed
	fingerprint := p.JSONString("fingerprint")
	publicKey := p.JSONString("publicKey")

	if !domain.PHPTruthy(fingerprint) || !domain.PHPTruthy(publicKey) {
		return nil, contract.NewError(contract.CodeMissingParameter)
	}

	// 指纹 = md5(base64_decode(公钥))，与登记时写入 sk_key_hash 的值一致。
	hash := auth.SSHKeyHash(publicKey)
	key, err := h.Repos.ActiveSSHKeyByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			return nil, contract.NewError(contract.CodeInvalidCredential)
		}
		return nil, err
	}

	return []publicKeyResult{{
		UserID:    key.UKey,
		KeyType:   auth.SSHKeyType(publicKey),
		PublicKey: publicKey,
	}}, nil
}

// lookupRepository 按组名 + 仓库名查仓库。
//
// 逐字对应 internal.php 的两级查找：先查仓库组（searchByName），
// 再在组内查仓库（searchByName）。任一环查不到都返回 nil，
// 对应 PHP 侧的 0x0302。
func (h *InternalHandler) lookupRepository(ctx context.Context, ref domain.RepositoryRef) (*domain.InternalRepository, error) {
	gKey, err := h.Repos.GroupKeyByName(ctx, ref.Group)
	if err != nil {
		return nil, err
	}
	if gKey == "" {
		return nil, nil
	}
	return h.Repos.RepositoryByName(ctx, ref.Name, gKey)
}

// repositoryRole 解析用户在仓库上的角色。
func (h *InternalHandler) repositoryRole(ctx context.Context, rKey, uKey string) (domain.Role, error) {
	if h.Guard == nil {
		return domain.RoleNoPermission, nil
	}
	return h.Guard.RepositoryRole(ctx, rKey, uKey)
}

// repositoryPath 返回仓库裸仓库的绝对路径。
//
// 逐字对应 internal.php 的 $this->_storagePath . $repositoryData['r_path']，
// 其中 _storagePath = dirname(APPPATH) . '/git-storage'。
// r_path 自带前导斜杠，因此这里直接拼接。
func (h *InternalHandler) repositoryPath(repoData *domain.InternalRepository) string {
	return h.Cfg.GitStoragePath() + repoData.Path
}

// isBranchProtected 判断目标分支是否受保护。
//
// 逐字对应 application/models/repository_model.php:2039-2079 的
// checkBranchProteced($rKey, $uKey, $ref, FALSE)（push 场景）。
func (h *InternalHandler) isBranchProtected(ctx context.Context, rKey, uKey, branch string) (bool, error) {
	role, err := h.repositoryRole(ctx, rKey, uKey)
	if err != nil {
		return false, err
	}
	// 角色低于 ROLE_DEVELOPER 一律受保护，PHP 侧在此处直接返回，
	// 不会去查保护分支规则表 —— 这里保持同样的短路。
	if role < domain.RoleDeveloper {
		return true, nil
	}
	rules, err := h.Repos.ProtectedBranchRules(ctx, rKey)
	if err != nil {
		return false, err
	}
	return domain.CheckBranchProtected(role, rules, branch, false), nil
}

// missingHookParameter 构造 hooks 的缺参错误。
//
// 逐字对应 internal.php:183 的 Response::reject(0x0201, NULL, [remote => 空字符串])：
// 错误码是 0x0201，data 恒为 {"remote":""}。
func missingHookParameter() error {
	return contract.NewErrorWithData(contract.CodeMissingParameter, map[string]any{"remote": ""})
}

// sprintfProtectedBranchRemote 生成保护分支的拒绝提示。
func sprintfProtectedBranchRemote(branch string) string {
	return fmt.Sprintf(protectedBranchRemoteTemplate, branch)
}

// splitEmail 按第一个 "@" 拆出用户名与域。
//
// 逐字对应 internal.php:114-115 的 explode('@', $result['u_email'])。
// PHP 取的是 [0] 与 [1] 两段，因此 "a@b@c" 会得到 "a" 与 "b"。
func splitEmail(email string) (name, addr string) {
	parts := strings.Split(email, "@")
	if len(parts) < 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

// jsonArray 从 JSON body 读取数组字段。
//
// contract 包只提供了 JSONString / JSONInt，没有数组读取入口，
// 而 hooks 的 args 必须是数组（PHP 侧 count($data['args'])），
// 因此这里直接读 Parsed.JSON。
func jsonArray(p *contract.Parsed, key string) []any {
	list, _ := p.JSON[key].([]any)
	return list
}

// phpString 把任意 JSON 标量转成字符串，等价于 PHP 的 (string) 转换。
//
// hooks 的 args 来自 JSON，PHP 侧会把它隐式转成字符串再做 explode('/')，
// 这里保持同样的行为。
func phpString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		if t {
			return "1"
		}
		return ""
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	default:
		return fmt.Sprintf("%v", t)
	}
}
