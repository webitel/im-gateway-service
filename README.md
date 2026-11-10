# im-gateway-service

## Secrets

Secrets do not have to be stored in plain text in `/etc/default/webitel-im-gateway`.
The service resolves them through
[webitel-go-kit appconfig](https://github.com/webitel/webitel-go-kit/blob/main/appconfig/secret/README.md), so each of these keys can also come
from a systemd credential (encrypted at rest) or a file:

| Config key | Service gets | systemd credential | File env var |
|---|---|---|---|
| `postgres.dsn` | contents | `postgres.dsn` | `POSTGRES_DSN_FILE` |
| `redis.password` | contents | `redis.password` | `REDIS_PASSWORD_FILE` |
| `pubsub.url` | contents | `pubsub.url` | `PUBSUB_URL_FILE` |
| `service.conn.key` | path of the key file | `service.conn.key` | `SERVICE_CONN_KEY_FILE` |
| `service.conn.client.key` | path of the key file | `service.conn.client.key` | `SERVICE_CONN_CLIENT_KEY_FILE` |
| `service.http.tls.key` | path of the key file | `service.http.tls.key` | `SERVICE_HTTP_TLS_KEY_FILE` |

A key set in plain text **and** present as a credential or `_FILE` makes the
service refuse to start, so remove the old value when you move a secret.

Example: move `postgres.dsn` and `pubsub.url` into encrypted credentials (systemd ≥ 250).

```sh
install -d -m 0700 /etc/credstore.encrypted
for name in postgres.dsn pubsub.url; do
  systemd-ask-password -n "$name:" \
    | systemd-creds encrypt --name="$name" - "/etc/credstore.encrypted/webitel-im-gateway.$name"
done
```

```ini
# systemctl edit webitel-im-gateway
[Service]
LoadCredentialEncrypted=postgres.dsn:/etc/credstore.encrypted/webitel-im-gateway.postgres.dsn
LoadCredentialEncrypted=pubsub.url:/etc/credstore.encrypted/webitel-im-gateway.pubsub.url
```

Then delete the matching lines from `/etc/default/webitel-im-gateway` and run
`systemctl restart webitel-im-gateway`. The unit's sandboxing needs no changes.

In Docker or Kubernetes mount the secret as a file and point the `_FILE` env
var at it, e.g. `POSTGRES_DSN_FILE=/run/secrets/postgres.dsn`. Rules, older systemd
and security limits: [secret/README.md](https://github.com/webitel/webitel-go-kit/blob/main/appconfig/secret/README.md).
