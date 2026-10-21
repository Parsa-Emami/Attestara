# Local readiness and diagnostics

Run `npm start` after `npm run setup`. The launcher probes the Go process and
opens/checks the bounded local snapshot store before starting the web listener.
It supervises both processes and closes the listener if the engine fails.

| Check | Meaning |
| --- | --- |
| `GET /healthz` | Web process health only. |
| `GET /readyz` with the full local launcher | Actual engine and snapshot-store readiness; reports `ready`, `locked`, `corrupt` or `unavailable`. |
| `GET /api/engine/health` | Bounded real Go process-health response; invalid/unavailable responses return 503. |
| Standalone `serve-web.mjs` readiness | Web assets/configuration only; engine work is unavailable. |

Readiness is a point-in-time probe. Concurrent CLI writes may briefly return
503/locked; a prior healthy response is not an authorization or availability
promise. No scanner/provider is required for the core workflow. Domain HTTP
APIs remain disabled.

The direct Node launchers emit bounded version-1 JSON lifecycle events with
`entryPoint`, per-launch `runId`, event and separate readiness states. Local
`started` reports `web: ready`, `engine: process_healthy`, `storage: ready` and
`providers: not_required`. A standalone viewer reports unintegrated storage and
an unconfigured engine. The legacy `coreReady: false` diagnostic field is not
a certification of the current domain workflow.

Events are `started`, `stopped`, `startup_failed` and `runtime_failed`.
Failures return nonzero; requested clean shutdown returns zero. Check actual
routes after startup. The launcher never stops unrelated listeners.

Diagnostics expose fixed allowlisted fields, not raw errors, stack traces,
store paths, request bodies, configuration or environment values. Requests have
no access log, polling history or persistent diagnostic store. npm banners are
separate from the application JSON stream; run the Node entry point directly
when JSON-only output is needed.

Use `npm run doctor` to probe built assets and the real executable, and
[the CLI reference](../docs/engineering/LOCAL_WORKFLOW.md) for immutable operations,
full file/signature verification and safe backup/restore.
