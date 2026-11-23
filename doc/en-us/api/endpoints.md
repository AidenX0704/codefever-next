# Endpoints

Seven endpoints exist today, all read-only. In the examples `$API` is your deployment URL
(for instance `https://git.example.com`) and `$TOKEN` is an access token plaintext (see
[Authentication and access tokens](auth.md)).

### General conventions

- **`GET` and `HEAD` only**; any other method returns `405` with `Allow: GET, HEAD`
  (`HEAD` is accepted so health checks keep working);
- Success bodies are **plain JSON**: a single object or an array, never wrapped in an envelope;
- Responses carry `Content-Type: application/json; charset=utf-8` and
  `Cache-Control: no-cache, must-revalidate`;
- Errors use a non-`200` status and the body `{"message": "...", "url": "..."}`
  (see [Errors and limits](errors.md));
- List endpoints return the full array by default for backward compatibility; pass `?page=&per_page=` (max 100) for pagination and `?q=` for keyword filtering.

### GET /api/v1/user

Requires a token; returns the user that owns it.

```bash
curl -s -H "Authorization: token $TOKEN" "$API/api/v1/user"
```

```json
{
  "id": 7341916824063314,
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
    "id": 3227920133129147,
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
| `id` | number | Derived from the first 8 bytes of the SHA-256 digest of the 32-char hex primary key, shifted right by 11 bits; stable across requests |
| `login` | string | Username |
| `full_name` | string | Currently identical to `login` |
| `email` | string | Account email |
| `avatar_url` | string | Avatar URL, **may be an empty string** |

> `id` is an **opaque** identifier: always a non-negative integer no larger than the JavaScript
> safe-integer limit (`9007199254740991`), so a JS `Number` represents it exactly and it can be
> compared directly. It is **not** an auto-increment sequence and is not guaranteed to be
> contiguous — treat it as an opaque handle and do not infer ordering or counts from it. The ID
> of an object is constant while it is visible; should the space ever need to grow, IDs become
> strings and the change is recorded in `CHANGELOG.md`.

### Object fields: repository

| Field | Type | Notes |
|---|---|---|
| `id` | number | As above, derived from the repository primary key; stable across requests |
| `owner` | string | Repository group name; **falls back to the repository name** when no group is resolvable |
| `name` | string | Repository slug |
| `display_name` | string | Repository display name |
| `full_name` | string | `owner/name` |
| `description` | string | Description, may be empty |
| `private` | boolean | Currently **always `true`**: the table has no visibility column yet, so every repository is treated as private |
| `fork` | boolean | Whether the repository was created by forking |
| `empty` | boolean | Whether the repository is empty (derived from `default_branch`) |
| `default_branch` | string | Default branch name, **may be empty** (for example before the first commit) |
| `fork_count` | number | Number of forks |
| `open_mr_count` | number | Number of open merge requests |
| `updated_at` | number | Last updated Unix timestamp (seconds) |
| `html_url` | string | Web URL `<site>/<owner>/<name>` |
| `clone_url` | string | Git clone URL `<site>/<owner>/<name>.git` |

### GET /api/v1/groups

Requires a token; lists repository groups visible to the user (supports `?q=&page=&per_page=`).

### GET /api/v1/groups/{group}

Token optional; returns a repository group by slug. Returns `404` if nonexistent or unreadable.

### GET /api/v1/groups/{group}/repos

Token optional; lists repositories in the group (supports `?q=&sort=&page=&per_page=`). Returns `404` if nonexistent or unreadable.

### Object fields: group

| Field | Type | Notes |
|---|---|---|
| `id` | number | Stable numeric ID derived from the group primary key |
| `name` | string | Group slug |
| `display_name` | string | Group display name |
| `description` | string | Group description |
| `type` | number | `1` = team group, `2` = personal default group |
| `repository_count` | number | Number of repositories in the group |
| `member_count` | number | Number of active members in the group |
| `html_url` | string | Web URL `<site>/groups/<name>` |
| `created_at` | number | Created Unix timestamp (seconds) |
| `updated_at` | number | Updated Unix timestamp (seconds) |
