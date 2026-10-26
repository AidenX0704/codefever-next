# Endpoints

Four endpoints exist today, all read-only. In the examples `$API` is your deployment URL
(for instance `https://git.example.com`) and `$TOKEN` is an access token plaintext (see
[Authentication and access tokens](auth.md)).

### General conventions

- Success bodies are **plain JSON**: a single object or an array, never wrapped in an envelope;
- Responses carry `Content-Type: application/json; charset=utf-8` and
  `Cache-Control: no-cache, must-revalidate`;
- Errors use a non-`200` status and the body `{"message": "...", "url": "..."}`
  (see [Errors and limits](errors.md));
- No endpoint is paginated, and no `Link` header is emitted.

### GET /api/v1/user

Requires a token; returns the user that owns it.

```bash
curl -s -H "Authorization: token $TOKEN" "$API/api/v1/user"
```

```json
{
  "id": 7341916824063314155,
  "login": "giserxiao",
  "full_name": "giserxiao",
  "email": "dev@example.com",
  "avatar_url": "https://git.example.com/static/avatar/2f1c.png"
}
```

Possible errors: `401` (missing or invalid token), `500` (internal error).

### GET /api/v1/user/repos

Requires a token; returns the repositories the user has **joined**, as a JSON array at the top
level and **without pagination**. An empty result is `[]`, never `null`.

```bash
curl -s -H "Authorization: token $TOKEN" "$API/api/v1/user/repos"
```

```json
[
  {
    "id": 3227920133129147716,
    "owner": "dev",
    "name": "hello-world",
    "full_name": "dev/hello-world",
    "description": "Sample repository",
    "private": true,
    "fork": false,
    "empty": false,
    "default_branch": "main",
    "html_url": "https://git.example.com/dev/hello-world",
    "clone_url": "https://git.example.com/dev/hello-world.git"
  }
]
```

Possible errors: `401` (missing or invalid token), `500` (failed to read the repository list).

### GET /api/v1/users/{username}

Anonymous access; returns a user's public profile. The success body is the same user object as
in the first endpoint.

```bash
curl -s "$API/api/v1/users/giserxiao"
```

Possible errors:

- `404`: no such user (a soft-deleted account is treated as nonexistent);
- `422`: `{username}` is blank, for example `/api/v1/users/%20`;
- `500`: internal error.

### GET /api/v1/repos/{owner}/{repo}

The token is **optional**. `{owner}` is resolved as a **repository group name** first, and as a
**username** only if no group matches: `CodeFever` keeps repositories under groups, so the
public path is "group/repository", mirroring the familiar "owner/repository" shape.

```bash
# With a token: works for your own private repositories
curl -s -H "Authorization: token $TOKEN" "$API/api/v1/repos/dev/hello-world"

# Anonymous: anything unreadable is a 404
curl -s "$API/api/v1/repos/dev/hello-world"
```

The success body is a single repository object (fields below).

Possible errors:

- `404`: the repository does not exist, **or** the caller cannot read it — a private repository
  returns `404` rather than `403` so that its existence is not disclosed; anonymous access
  always ends here today;
- `422`: `{owner}` or `{repo}` is blank, for example `/api/v1/repos/%20/%20`;
- `500`: internal error.

### Object fields: user

| Field | Type | Notes |
|---|---|---|
| `id` | number | Derived from the first 8 bytes of the SHA-256 digest of the 32-char hex primary key; stable across requests |
| `login` | string | Username |
| `full_name` | string | Currently identical to `login` |
| `email` | string | Account email |
| `avatar_url` | string | Avatar URL, **may be an empty string** |

> `id` is a signed 64-bit integer and may be negative. Treat it as an opaque integer: it is not
> an auto-increment sequence, and it is not guaranteed to be positive.

### Object fields: repository

| Field | Type | Notes |
|---|---|---|
| `id` | number | As above, derived from the repository primary key; stable across requests |
| `owner` | string | Repository group name; **falls back to the repository name** when no group is resolvable |
| `name` | string | Repository name |
| `full_name` | string | `owner/name` |
| `description` | string | Description, may be empty |
| `private` | boolean | Currently **always `true`**: the table has no visibility column yet, so every repository is treated as private |
| `fork` | boolean | Whether the repository was created by forking |
| `empty` | boolean | Currently **always `false`** |
| `default_branch` | string | Default branch name, **may be empty** (for example before the first commit) |
| `html_url` | string | Web URL `<site>/<owner>/<name>` |
| `clone_url` | string | Git clone URL `<site>/<owner>/<name>.git` |
