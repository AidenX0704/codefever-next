# Webhooks

Webhooks let an external system (Jenkins, Jira, a chat bot, etc.) receive a payload when
something happens in a CodeFever repository.

Open the list from: **Repository home → Settings → Webhooks**.

## Create a webhook

1. Open the **Webhooks** list.
2. Click **Create Webhook** in the top-right corner.
3. Fill in the **URL**, **secret**, and **events**, then click **Create**.

## Options

- **URL**: the endpoint that receives the webhook.
- **Payload format**: `application/json`; the event is sent as the request body.
- **Secret**: used to sign the payload (see below).
- **Events**: subscribe to Push and/or other repository events.
- **Status**: enable or disable the webhook.

## Verify the payload

CodeFever sends an `X-CodeFever-Signature` header. Its value has the form:

```text
md5=<32 lowercase hex characters>
```

The digest is computed as:

```text
md5(payload + secret)
```

where `payload` is the raw request body and `secret` is the value configured for the
webhook.

## Request format

Headers:

```http
User-Agent: CodeFever-Webhook
X-CodeFever-Id: <request id>
X-CodeFever-Event: <event name>
X-CodeFever-Signature: md5=<content hash>
```

Body:

```json
{
  "event": "EventName",
  "data": {},
  "repository": {
    "id": "RepositoryID",
    "url": "RepositoryURL"
  },
  "sender": {
    "id": "UserID",
    "name": "UserName"
  }
}
```

- `X-CodeFever-Id` is unique per delivery and can be used for deduplication.
- `X-CodeFever-Event` matches `body.event`.
- `X-CodeFever-Signature` is described above.

## Common events

| EventName | Meaning |
| --- | --- |
| `hook:postReceive` | Push received |
| `repo:fork` | Repository forked |
| `repo:updateName` | Repository renamed |
| `repo:updateDescription` | Description changed |
| `repo:addMember` | Member invited |
| `repo:removeMember` | Member removed |
| `repo:remove` | Repository deleted |
| `branch:create` | Branch created |
| `branch:remove` | Branch deleted |
| `branch:changeDefaultBranch` | Default branch changed |
| `tag:create` | Tag created |
| `tag:remove` | Tag deleted |
| `mergeRequest:create` | Merge request created |
| `mergeRequest:close` | Merge request closed |
| `mergeRequest:merge` | Merge request merged |

> This page is a translation of the Chinese Webhook guide. The full event/data reference
> lives in [`doc/zh-cn/repo/webhooks.md`](../../zh-cn/repo/webhooks.md); translation
> contributions are welcome.
