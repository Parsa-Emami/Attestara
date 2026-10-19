# C34 support and proxy verification

Repository-authored synthetic metadata, Apache-2.0. Tests create a temporary
static page and isolated Nginx prefix; real web CSP/cache/API boundaries are
preserved. No uploaded source is executed, external listener exposed, host
service reloaded or vulnerability report submitted.

`tests/web/security-reporting.test.mjs` tests gate failure semantics only.
The separate live `npm run security:check` is required for actual reporting
availability. `tests/ops/nginx-local.test.mjs` uses a real Nginx binary and real
web server; it tests headers, disabled APIs and unavailable upstream. It does
not certify Apache deployment, TLS or all C34 criteria. Historical C30/C33
evidence retains its original source identity.
