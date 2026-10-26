import { postForm, type AjaxResult } from './client'

/**
 * 认证相关的表单接口（旧版服务端表单页的 POST 端点，全部 application/x-www-form-urlencoded）。
 *
 * 端点与字段逐字对应旧模板（`backend/internal/httpapi/templates/{login,register,verify,reset_password}.html`），
 * 后端实现见 `backend/internal/handlers/{user,web_user}.go`。
 */

export interface LoginFieldExtra {
  /** code 0：登录成功后的跳转地址（后端固定返回 /repositories）。 */
  redirect?: string
  /** code 100：2FA 临时键，用于 /user/verify?key=... */
  key?: string
}

export interface LoginResult extends AjaxResult {
  extra?: LoginFieldExtra
}

/** POST /user/login（code：0 成功、100 需 2FA、1/2 缺字段、3 账号不存在、4 口令错误）。 */
export function login(email: string, password: string): Promise<LoginResult> {
  return postForm('/user/login', { email, password }) as Promise<LoginResult>
}

/** POST /user/register（code 0 成功并返回 redirect，其余为错误文案）。 */
export function register(email: string, password: string, rePassword: string): Promise<LoginResult> {
  return postForm('/user/register', { email, password, rePassword }) as Promise<LoginResult>
}

/** POST /user/verify（2FA 二次验证；code 0 成功、100 失效、1 验证码错误）。 */
export function verifyTwoFA(key: string, code: string): Promise<LoginResult> {
  return postForm('/user/verify', { key, code }) as Promise<LoginResult>
}

/** POST /user/getResetPasswordCode —— 发送邮箱验证码（code 0 成功、3 邮箱格式错误、4 账号不存在）。 */
export function requestResetCode(email: string): Promise<AjaxResult> {
  return postForm('/user/getResetPasswordCode', { email })
}

/** POST /user/resetPassword（mode 缺省）—— 用邮箱验证码换取重置口令的临时 key。 */
export function requestResetTicket(email: string, emailCode: string): Promise<LoginResult> {
  return postForm('/user/resetPassword', { email, emailCode }) as Promise<LoginResult>
}

/** POST /user/resetPassword（mode=reset）—— 用临时 key 设置新口令。 */
export function applyNewPassword(key: string, password: string): Promise<LoginResult> {
  return postForm('/user/resetPassword', { mode: 'reset', key, password }) as Promise<LoginResult>
}

/** 登出：后端清会话并 302 到 /user/login，因此走整页跳转而不是 fetch。 */
export function logout(): void {
  window.location.href = '/user/logout'
}
