# Errors and limits

### Error body

Public API errors are always `{message,url}` — no envelope, no numeric error code:

```json
{
  "message": "仓库不存在",
  "url": "https://git.example.com/doc/cn/api/overview.md"
}
```

- `message`: human-readable text, safe to surface to end users;
- `url`: a link to the public API page in the documentation centre (the Chinese page
  `/doc/cn/api/overview.md`; the English page `/doc/en/api/overview.md` mirrors it one-to-one),
  derived from the site URL at deployment time.

Success payloads are plain JSON as well, served with
`Content-Type: application/json; charset=utf-8` and `Cache-Control: no-cache, must-revalidate`.

A few real bodies:

```json
{"message":"缺少访问令牌","url":"https://git.example.com/doc/cn/api/overview.md"}
```

```json
{"message":"用户不存在","url":"https://git.example.com/doc/cn/api/overview.md"}
```

```json
{"message":"读取仓库列表失败","url":"https://git.example.com/doc/cn/api/overview.md"}
```

### Status codes

| Status | Meaning | When it happens |
|---|---|---|
| `200` | OK | Plain JSON object or array |
| `401` | Unauthenticated | Missing token, invalid token, disabled or soft-deleted owning account. Only on token-required endpoints: `/api/v1/user`, `/api/v1/user/repos` |
| `404` | Not found | No such user; no such repository; a private repository the caller cannot read (**`404` rather than `403`**, so existence is not disclosed) |
| `422` | Unprocessable | `{username}`, `{owner}` or `{repo}` is blank, for example `/api/v1/users/%20` |
| `500` | Internal error | For example failing to read the repository list |

### Limits

- Only four **read-only** endpoints. Branches, tags, commits, file contents, releases and
  access-token management are not implemented yet;
- **No pagination** and no `Link` header: list endpoints return everything in one response, and
  callers are responsible for handling the size themselves;
- **No rate limiting** and **no scope granularity**: a token is exactly as powerful as its user;
- **No webhooks or event subscriptions**. Webhooks configured inside a repository are a UI
  feature and are not managed by the public API;
- Access tokens have no management UI yet and must currently be issued by operations
  (see [Authentication and access tokens](auth.md)).

### Change policy

`/api/v1` and the internal `/api/*` API evolve independently: the internal contract is protected
by snapshot baselines, while public API additions aim to stay **backward compatible** — existing
paths, fields and status-code semantics are kept stable, and new capabilities arrive as new
fields or new endpoints. If an incompatible change ever becomes unavoidable, it is recorded in
the repository's `CHANGELOG.md`.

### Documentation and code

The source of truth for this documentation is `backend/internal/httpapi/publicapi/`:

- `router.go`: the route table and the `/api/v1` namespace isolation;
- `server.go`: per-endpoint authentication mode, response shapes and status codes;
- `publicapi.go`: the error body, token extraction rules and response headers.

When the endpoint table changes, update the pages in this directory as well.
