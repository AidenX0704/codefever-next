/**
 * 统一 API 客户端。
 *
 * 逐项复刻旧版 `www/view/src/data_providers/main.js` + `helpers/NetworkHelper.js`
 * 的**线上契约**（后端 `backend/internal/contract` 依赖这些细节，改动会导致请求被拒）：
 *   1. POST 使用 multipart/form-data；嵌套对象编码为 `key[sub]`、数组编码为 `key[]`；
 *      键名后缀 `_SAFE` / `_BINARY` 会被剥掉（旧版用它标记二进制字段）；
 *   2. GET 追加 `requestTime=<毫秒>`；`undefined` / `null` 的查询参数不发送；
 *   3. 请求头固定带 `codefever-end-env: codefever-app` 与 `codefever-end-lang`；
 *   4. 响应是 `{ code, message, data, extra }` 信封，`code !== 0` 视为业务错误；
 *   5. `code === 0x0101`（登录失效）→ 3 秒后跳登录页，与旧版行为一致。
 */

export interface Envelope<T> {
  code: number
  message?: string
  data?: T
  extra?: Record<string, unknown>
}

export class ApiError extends Error {
  readonly code: number
  readonly extra?: Record<string, unknown>

  constructor(code: number, message: string, extra?: Record<string, unknown>) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.extra = extra
  }
}

/** 与旧版 `helpers/ErrorCodeParser.js` 完全一致的错误码文案表。 */
const ERROR_MESSAGES: Record<number, { zh: string; en: string }> = {
  0x0100: { zh: '请求错误', en: 'Request Error' },
  0x0101: { zh: '登录信息失效, 需要重新登陆', en: 'Invalid Login' },
  0x0105: { zh: '请求的地址不存在', en: 'Resource Not Found' },
  0x0106: { zh: '没有操作权限', en: 'Insufficient Permission' },
  0x0200: { zh: '请求资源为空', en: 'Empty Resource' },
  0x0201: { zh: '输入错误', en: 'Invalid Input' },
  0x0202: { zh: '输入值超过允许范围', en: 'Input Out of Range' },
  0x0301: { zh: '没有此仓库组', en: 'No Such Group' },
  0x0302: { zh: '没有此仓库', en: 'No Such Repository' },
  0xff00: { zh: '不能识别的响应数据，请稍后再试', en: 'Response Data Can Not Be Parsed, Try Later' },
  0xff01: { zh: '网络链接错误，请稍后再试', en: 'Network Error, Try Later' },
  0xff02: { zh: '请求过于频繁，请稍后再试', en: 'Too Many Request, Try Later' },
  0xff03: { zh: '服务暂时不可用，请稍后再试', en: 'Service Unavailable, Try Later' },
}

const LANG_STORAGE_KEY = 'lang'
const DEFAULT_LANG = 'zh-cn'

export function getLang(): string {
  try {
    return window.localStorage.getItem(LANG_STORAGE_KEY) ?? DEFAULT_LANG
  } catch {
    return DEFAULT_LANG
  }
}

export function setLang(lang: string): void {
  try {
    window.localStorage.setItem(LANG_STORAGE_KEY, lang)
  } catch {
    /* localStorage 不可用时静默降级：语言只影响展示，不影响功能 */
  }
}

/** 错误码 → 本地化文案；未知码回退到后端给的 message。 */
export function errorMessage(code: number, fallback?: string): string {
  const entry = ERROR_MESSAGES[code]
  if (!entry) {
    return fallback ?? '操作失败，请稍后再试'
  }
  return getLang() === 'zh-cn' ? entry.zh : entry.en
}

type Primitive = string | number | boolean | null | undefined | Blob | File
export type PostBody = Record<string, Primitive | ReadonlyArray<Primitive> | Record<string, Primitive>>
export type QueryParams = Record<string, string | number | boolean | null | undefined>

const SAFE_SUFFIX = /_(SAFE|BINARY)$/

/** 复刻旧版 makeFormData：嵌套对象 → `key[sub]`，数组 → `key[]`，剥掉 _SAFE/_BINARY 后缀。 */
export function encodeFormData(body: PostBody): FormData {
  const form = new FormData()
  for (const [rawKey, value] of Object.entries(body)) {
    if (value === undefined || value === null) {
      continue
    }
    if (typeof value === 'object' && !(value instanceof Blob) && !Array.isArray(value)) {
      for (const [subKey, subValue] of Object.entries(value)) {
        if (subValue === undefined || subValue === null) {
          continue
        }
        form.append(`${rawKey}[${subKey}]`, String(subValue))
      }
      continue
    }
    if (Array.isArray(value)) {
      for (const item of value) {
        form.append(`${rawKey}[]`, item instanceof Blob ? item : String(item))
      }
      continue
    }
    form.append(rawKey.replace(SAFE_SUFFIX, ''), value instanceof Blob ? value : String(value))
  }
  return form
}

/** 复刻旧版 makeQueryString：跳过 undefined/null，值与键都做 encodeURIComponent。 */
export function encodeQuery(query: QueryParams): string {
  const parts: string[] = []
  for (const [key, value] of Object.entries(query)) {
    if (value === undefined || value === null) {
      continue
    }
    parts.push(`${window.encodeURIComponent(key)}=${window.encodeURIComponent(String(value))}`)
  }
  return parts.join('&')
}

function baseHeaders(): Record<string, string> {
  return {
    // 后端用这两个头区分"前端调用"与语言（见 contract/parse.go 的既有约定）。
    'codefever-end-env': 'codefever-app',
    'codefever-end-lang': getLang(),
    Accept: 'application/json',
  }
}

let onUnauthorized: (() => void) | null = null

/** 注册登录失效回调（由应用入口注入路由跳转，避免 api 层依赖路由库）。 */
export function setUnauthorizedHandler(handler: (() => void) | null): void {
  onUnauthorized = handler
}

async function send<T>(method: 'GET' | 'POST', endpoint: string, init: { body?: PostBody; query?: QueryParams }): Promise<Envelope<T>> {
  const url = init.query ? `${endpoint}?${encodeQuery(init.query)}` : endpoint

  let response: Response
  try {
    response = await window.fetch(url, {
      method,
      body: init.body ? encodeFormData(init.body) : undefined,
      cache: 'no-cache',
      credentials: 'same-origin',
      headers: baseHeaders(),
      mode: 'same-origin',
      redirect: 'follow',
    })
  } catch {
    throw new ApiError(0xff01, errorMessage(0xff01))
  }

  if (response.status === 429) {
    throw new ApiError(0xff02, errorMessage(0xff02))
  }
  if (response.status >= 500) {
    throw new ApiError(0xff03, errorMessage(0xff03))
  }

  let envelope: Envelope<T>
  try {
    envelope = (await response.json()) as Envelope<T>
  } catch {
    throw new ApiError(0xff00, errorMessage(0xff00))
  }

  if (envelope.code !== 0) {
    if (envelope.code === 0x0101) {
      // 旧版行为：提示后 3 秒跳登录页（等用户看清提示）。
      window.setTimeout(() => {
        if (onUnauthorized) {
          onUnauthorized()
        } else {
          window.location.href = '/user/login'
        }
      }, 3000)
    }
    throw new ApiError(envelope.code, errorMessage(envelope.code, envelope.message), envelope.extra)
  }

  return envelope
}

/** GET：自动追加 requestTime（旧版用于绕开中间层缓存）。 */
export function get<T>(endpoint: string, query: QueryParams = {}): Promise<Envelope<T>> {
  return send<T>('GET', endpoint, { query: { ...query, requestTime: Date.now() } })
}

/** POST：multipart/form-data。 */
export function post<T>(endpoint: string, body: PostBody = {}, query: QueryParams = {}): Promise<Envelope<T>> {
  return send<T>('POST', endpoint, { body, query })
}

/**
 * 认证类表单信封：`{ code, message, extra }`（**没有** `data` 字段）。
 *
 * 这是旧版服务端表单页（/user/login 等）的响应形状，与 `/api/*` 的信封不同：
 *   - `extra` 可能是对象（`{redirect}` / `{key}`）也可能是空数组 `[]`；
 *   - `code` 是**业务码**（1..8、100），不是 `0x0xxx` 错误码，因此这里**不抛异常**，
 *     由调用方按码分支（登录的 100 代表"需要 2FA"，不是错误）。
 */
export interface AjaxResult {
  code: number
  message?: string
  extra?: unknown
}

/** 从 `extra` 中安全取字段（旧版 `extra` 可能是 `[]`）。 */
export function extraField(result: AjaxResult, key: string): string | undefined {
  const extra = result.extra
  if (!extra || typeof extra !== 'object' || Array.isArray(extra)) {
    return undefined
  }
  const value = (extra as Record<string, unknown>)[key]
  return typeof value === 'string' ? value : undefined
}

/**
 * POST `application/x-www-form-urlencoded`。
 *
 * 为什么认证页不用 multipart：旧版这四个页面是 jQuery `$('#form').serialize()`，
 * 走的正是 urlencoded；后端 `web_user.go` 用 `r.FormValue` 读取，两种都能解析，
 * 但保持与旧请求完全一致可以继续复用 `cmd/contract-collect` 的基线。
 */
export async function postForm(endpoint: string, body: Record<string, string>): Promise<AjaxResult> {
  let response: Response
  try {
    response = await window.fetch(endpoint, {
      method: 'POST',
      body: new URLSearchParams(body).toString(),
      cache: 'no-cache',
      credentials: 'same-origin',
      headers: { ...baseHeaders(), 'Content-Type': 'application/x-www-form-urlencoded; charset=UTF-8' },
      mode: 'same-origin',
      redirect: 'follow',
    })
  } catch {
    throw new ApiError(0xff01, errorMessage(0xff01))
  }

  if (response.status === 429) {
    throw new ApiError(0xff02, errorMessage(0xff02))
  }
  if (response.status >= 500) {
    throw new ApiError(0xff03, errorMessage(0xff03))
  }

  try {
    return (await response.json()) as AjaxResult
  } catch {
    throw new ApiError(0xff00, errorMessage(0xff00))
  }
}
