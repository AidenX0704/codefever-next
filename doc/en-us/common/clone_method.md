# Choosing between HTTP and SSH

When cloning a repository you can use either `HTTP` or `SSH`. CodeFever supports both, and
the clone panel generates the URLs from the configured site address.

### Comparison

`SSH` is convenient after you add a public key in your personal settings: no password is
needed for clone/pull/push. The downside is that you must generate and configure a key.
HTTP is easier for beginners, but each connection asks for your e-mail and password.

### Ports and URL forms

- Compose defaults: HTTP `9080`, SSH `2222` (override with `CODEFEVER_HTTP_PORT` /
  `CODEFEVER_SSH_PORT`).
- HTTP: `http://<email-or-user>@<host>:9080/<group>/<repo>.git`
- SSH: `ssh://git@<host>:2222/<group>/<repo>.git`

> SSH must use the `ssh://git@host:port/...` form to carry a non-22 port; the scp-like
> `git@host:<path>` form treats `2222` as part of the path. For a source install that still
> uses port 22, `git@host:<group>/<repo>.git` is also valid.

Test the SSH connection:

```shell
ssh -p 2222 -T git@<host>
```

> See [Generate and configure an SSH key](ssh_key.md) for key setup.
