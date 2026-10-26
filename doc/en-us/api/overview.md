# Public API overview

`/api/v1` is the public REST API of `CodeFever Next`, built for external scripts, CI/CD
pipelines and third-party integrations. It is read-only, returns plain JSON and authenticates
with access tokens. Four read-only endpoints are implemented today, and this documentation
describes **only what is actually implemented**.

> Examples use `https://git.example.com` throughout. Replace it with your own deployment URL.

### How it differs from the internal API

`/api/v1` and the internal `/api/{controller}/{action}` API used by the web UI are two
**fully isolated** contracts:

| Aspect | Public API `/api/v1` | Internal API `/api/*` |
|---|---|---|
| Path shape | `/api/v1/repos/{owner}/{repo}`, variable segment count | `/api/{controller}/{action}` |
| Success body | Plain JSON (object or array) | `{request-id,code,message,data}` envelope |
| Error body | `{message,url}` | `{request-id,code,message,data}` |
| Request encoding | Standard query parameters | `multipart` |
| Authentication | Access token | Cookie session only |

The internal contract is locked by snapshot baselines, so any field change would break the
regression suite. The public API therefore has its own parameter parsing, error codes and
response shapes; the two share only the underlying repository queries and permission checks.

Note that `/apiv1/internal/...` is an internal gateway callback prefix and has **nothing to do**
with `/api/v1`. It is not part of the public API and the two must not be mixed.

### Quick start in 30 seconds

```bash
# 1. Deployment URL and an access token (tokens are currently issued out-of-band,
#    see "Authentication and access tokens")
export API=https://git.example.com
export TOKEN=<your-access-token>

# 2. The user that owns the token
curl -s -H "Authorization: token $TOKEN" "$API/api/v1/user"

# 3. Repositories visible to that user (a JSON array, not paginated)
curl -s -H "Authorization: token $TOKEN" "$API/api/v1/user/repos"

# 4. A single repository (owner is resolved as a repository group first, then as a username)
curl -s -H "Authorization: token $TOKEN" "$API/api/v1/repos/dev/hello-world"
```

### Endpoints at a glance

| Method and path | Auth | Description |
|---|---|---|
| `GET /api/v1/user` | Token required | The user that owns the token |
| `GET /api/v1/user/repos` | Token required | Repositories the user has joined, unpaginated, as a JSON array |
| `GET /api/v1/users/{username}` | Anonymous | Public profile of a user by username; `404` if unknown |
| `GET /api/v1/repos/{owner}/{repo}` | Token optional | A single repository; a private repository you cannot read returns `404`, not `403` |

Successful responses always carry `Content-Type: application/json; charset=utf-8` and
`Cache-Control: no-cache, must-revalidate`.

### Permissions and visibility

The public API adds no permission model of its own: a token represents one user, and what that
token can read is exactly what the user can read in the web UI. Private repositories answer
`404` for anyone without read access, so never use the status code to probe whether a
repository exists — for anonymous callers almost every repository answers `404`.

### Next steps

- [Authentication and access tokens](auth.md): the three ways to send a token, its lifecycle, `401` troubleshooting
- [Endpoints](endpoints.md): per-endpoint requests, sample success payloads and object fields
- [Errors and limits](errors.md): error body, status codes, current limits and the change policy
