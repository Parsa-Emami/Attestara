# Local network boundary and optional private ingress (C32)

For testing while Apache keeps port 80, use the isolated loopback-only
[`nginx.local.conf`](../ops/web/nginx.local.conf) on 8080, forwarding to the
default application on 4173. [The runbook](../docs/engineering/WEB_RUNBOOK.md#test-alongside-apache-on-port-80)
uses explicit prefix/config paths to avoid loading default/system Nginx sites.
Check syntax, actual startup and HTTP behavior separately. This same-host HTTP
test does not authorize remote exposure; no existing service must be stopped.

A clone runs on `http://127.0.0.1:4173` with no owned domain, certificate, cloud account or ingress. `scripts/start-local.mjs` and `scripts/serve-web.mjs` accept loopback bind addresses only. The browser runtime uses `/api` on its own origin and has no remote engine endpoint. Dynamic web/configuration/health responses use `Cache-Control: no-store`; filename-fingerprinted `/assets/` responses may use immutable caching. The web response sets `connect-src 'self'` and does not grant cross-origin requests with `Access-Control-Allow-Origin`. Other domain API routes remain unavailable, and write methods are rejected.

For an optional private Linux service, use [WEB_RUNBOOK.md](../docs/engineering/WEB_RUNBOOK.md) and review [service.systemd](../ops/web/service.systemd) plus [nginx.conf.template](../ops/web/nginx.conf.template) on the target host. The service binds loopback; the proxy template requires an operator-owned name and real TLS paths. It denies external clients until the operator explicitly reviews and adds a trusted private CIDR. Verify the rendered service with `systemd-analyze verify` and the rendered Nginx configuration with `nginx -t` before enabling either. Keep TLS termination and CIDR admission at the operator-controlled ingress. Do not expose the loopback listener directly or enable domain work through a public flag.

These headers and CIDR rules do not authenticate users or authorize objects. CORS restricts browser access to responses; it does not block non-browser clients or defend against DNS rebinding. The current server has no Host allowlist. An optional ingress therefore requires operator review and private-network controls; the existing runtime is not a multi-user remote domain service. C32 tests local HTTP behavior, not a deployed TLS proxy.
