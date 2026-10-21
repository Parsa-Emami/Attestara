# Local operation and optional service templates

Use the pinned Linux x86_64 prerequisites in CLONE_TO_RUN.md. From source:

```sh
npm run setup
npm start -- --store /absolute/private/attestara-store --port 4173
```

For complete C31 artifacts and source rebuilds follow `../release/ARTIFACTS.md`.
The binary distribution starts with `node scripts/start-local.mjs`, requires only
Node 24.19.0 at runtime and includes the actual Go executable. The existing
`npm run package:web` remains a standalone web-only convenience artifact under
`dist/release`; it never certifies the complete product or storage readiness.
It can still run with `node scripts/serve-web.mjs --root web --port 4304 --config
ops/web/runtime-config.json` from that web-only directory.

## Readiness and data

`/healthz` is web-process health. Under the unified launcher `/readyz` checks real
engine process and C05 snapshot storage readiness. A concurrent writer can yield
503/locked. Standalone web readiness covers only web assets/configuration.
`/api/engine/health` is process health; other domain API routes remain disabled.
The browser-selected passport stays local and unverified. The runtime has no
multi-user/private authorization contract. Never enable it by a public config flag.

Keep a private store outside immutable release directories. LOCAL_WORKFLOW.md
covers exact CLI workflow, portable backup, non-overwriting restore and recovery
on uncertain acknowledgement. Never copy a live store or replace newer data with
an older backup. Stop/restart the unified launcher with SIGTERM/Ctrl+C; it supervises
the Go child. Diagnostics use fixed redacted states; inspect no raw payload logs.

## Optional Linux service

No host configuration is applied by packaging. An operator can review
`ops/web/service.systemd` for an unprivileged `attestara-web` account and releases
under `/srv/attestara/releases/<commit>`, with `/srv/attestara/current` selecting one.
Verify archive checksums before extracting into a fresh directory. Match the Node
binary path to the pinned runtime. `StateDirectory=attestara` provides the private
writable store at `/var/lib/attestara/store`; the release tree remains read-only.
The service binds only loopback on port 4304. Copy the template to a temporary file named `attestara-web.service` and run
`systemd-analyze verify /path/to/attestara-web.service` before an operator installs/enables it.

The Nginx template denies all external traffic by default. Only an operator-owned
domain, actual TLS paths and explicitly reviewed private network ranges may be
substituted. IP allowlists do not provide user/object authorization. Keep domain
API routes unavailable and validate a rendered template using `nginx -t` before
an operator reloads it. C32 owns further network/private-ingress verification.
No domain, certificate, account or service installation is created here.

## Test alongside Apache on port 80

`npm start` uses `127.0.0.1:4173`; Apache can keep port 80. Select another
application port with `npm start -- --port 4301` if needed. The TLS site template
uses 443, so an Nginx error about `0.0.0.0:80` usually comes from another
loaded/default site. `nginx -t` checks configuration, not availability of a
listening port; actual startup and HTTP checks are separate.

For same-host testing, the complete isolated
[`nginx.local.conf`](../../ops/web/nginx.local.conf) binds `127.0.0.1:8080` and
proxies to the default application on 4173 without loading system sites. With
Nginx installed and `npm start` running in another terminal, run from this
repository root:

```sh
mkdir -p .local-data/nginx/logs .local-data/nginx/temp
nginx -p "$PWD/.local-data/nginx/" -c "$PWD/ops/web/nginx.local.conf" -t
nginx -p "$PWD/.local-data/nginx/" -c "$PWD/ops/web/nginx.local.conf"
curl --fail http://127.0.0.1:8080/healthz
curl --fail http://127.0.0.1:8080/readyz
```

Open `http://127.0.0.1:8080`. `/healthz` reports web health; unified `/readyz`
must report actual engine/storage readiness. An unavailable backend gives an
error such as 502. Stop only this isolated instance using its matching paths:

```sh
nginx -p "$PWD/.local-data/nginx/" -c "$PWD/ops/web/nginx.local.conf" -s quit
```

If 8080 is occupied, copy the config to a private temporary file and change its
`listen` port. If the application uses 4301, change `proxy_pass` to 4301 too.
Use the copy's absolute path in every `-c` command. Inspect `ss -ltn` for
listeners; do not stop another service or overwrite its configuration to free a
port. Do not use `systemctl reload nginx` for this isolated instance.

HTTP is restricted to same-host loopback testing. For remote access retain the
reviewed TLS/private-network template. The proxy preserves the browser port in
`Host`, replaces forwarded claims from the real connection, and does not grant
identity or object authorization. Domain API writes remain disabled.

Run the real proxy regression with `node --test tests/ops/nginx-local.test.mjs`
using an installed Nginx, or `NGINX_BIN=/absolute/path/to/nginx`. The test uses
disposable prefixes/free ports and never reloads a host service.

## Update and recovery

Retain the previous immutable artifact and the independent store. After stopping
the service, switch only the release symlink, restart, and verify readiness plus
the real CLI workflow. Reverting an executable never authorizes reverting data.
C35 owns verified install/upgrade/rollback compatibility. A corrupt or unavailable
store must produce failure; retain the latest accepted bytes and use a new-store
restore/forward repair. A successful static web probe does not prove storage recovery.
