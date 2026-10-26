# Authentication and access tokens

### A token is a user

The public API authenticates with **access tokens**, and each token maps to exactly one user:

- Permissions are **inherited entirely** from the owning user: the API reuses the same role
  resolution as the web UI, so `/api/v1` introduces no separate permission model;
- Tokens carry **no scopes**. The `at_scope` column exists as a placeholder but is not
  implemented in this first version, so every token has the same authority as its user;
- A user may hold several tokens; revoking one does not affect the others.

### Three ways to send a token

```bash
export API=https://git.example.com
export TOKEN=<your-access-token>

# 1. GitHub style: Authorization: token <plaintext>
curl -s -H "Authorization: token $TOKEN" "$API/api/v1/user"

# 2. Bearer style
curl -s -H "Authorization: Bearer $TOKEN" "$API/api/v1/user"

# 3. Query parameter (for CI systems that cannot set headers)
curl -s "$API/api/v1/user?access_token=$TOKEN"
```

Details:

- The scheme prefix is case-insensitive (`token`, `TOKEN`, `Bearer`), the header must be
  **exactly two whitespace-separated fields**, so the prefix and the plaintext take exactly
  one space;
- When both the header and the query parameter are present, the **header wins**;
- Anything else — `Authorization: Basic ...`, or a prefix with no plaintext — is treated as
  **no token at all**;
- `?access_token=` leaks the token into access logs, proxy logs and browser history. Prefer
  the header whenever the client supports it.

### Lifecycle

- The plaintext is returned **once, at creation time**. The server stores only its SHA-256
  digest, so a lost token cannot be recovered — issue a new one instead;
- Disabling or soft-deleting the owning account **invalidates its tokens immediately**;
- The public API currently offers no expiry, revocation or rotation endpoints.

### Current limitation: no UI yet

Neither a **management UI** nor an **internal creation endpoint** for access tokens exists in
this first version. Tokens must currently be issued by operations through a script or directly
in the database: generate a high-entropy plaintext (the implementation uses 32 random bytes,
base64url without padding), store its SHA-256 hex digest in `cc_access_tokens` (owning user key
`u_key`, token name `at_name`, digest `at_sha256`, active status) and hand the plaintext to its
owner. A later version will add the UI and creation endpoint, and this page will be updated then.

> Never paste a real token into documentation, an issue or a log.

### Troubleshooting `401`

| Symptom | Likely cause |
|---|---|
| `401` with message `缺少访问令牌` ("missing access token") | No header and no `?access_token`; or the header uses `Basic`, or has the prefix without a plaintext |
| `401` with message `访问令牌无效` ("invalid access token") | Typo, stray spaces or quotes around the plaintext; the token no longer exists; the owning account was disabled or soft-deleted |
| Valid token, still `401` | You are sending the browser's session cookie. The public API accepts access tokens only and ignores session cookies |
| An anonymous endpoint returns `404` instead of `401` | Expected: the token is **optional** on `/repos/{owner}/{repo}`, an invalid token is treated as anonymous, and private repositories are invisible to anonymous callers |

Endpoints that require a token (`/api/v1/user`, `/api/v1/user/repos`) answer a missing or
invalid token with `401` and the same `{message,url}` error body:

```json
{"message":"缺少访问令牌","url":"https://git.example.com/doc/cn/api/overview.md"}
```
