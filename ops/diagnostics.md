# Local readiness and diagnostics (C29)

The supported launcher is `npm start` after `npm run setup`. C29 reports the
current local process profile. It does not initialize authoritative storage,
select a provider, admit domain work or certify the C30 workflow.

## Operator questions

| Question | Check | Meaning |
| --- | --- | --- |
| Is the web process responding? | `GET /healthz` | HTTP 200, `scope: web-shell`; process health only. |
| Are the web assets available? | `GET /readyz` | HTTP 200 for the shell, HTTP 503 if its index is absent, not a regular file or escapes the web root; startup configuration was already validated. `engineReady: false` is the existing domain-integration boundary. |
| Is the Go process responding correctly now? | `GET /api/engine/health` | The real loopback Go process must return the exact supported health contract within 1500 ms and 4096 bytes. Missing, malformed, redirected, oversized or unavailable upstream health returns HTTP 503 with `engine_unavailable`. |
| Are authoritative storage and required providers available? | Launcher JSON `readiness` | Both are `not_integrated`. The current launch profile has no required providers (`requiredProviders: []`). This is not a successful storage or provider probe. |

For example, with the default port, use `curl --fail http://127.0.0.1:4173/healthz`,
`curl --fail http://127.0.0.1:4173/readyz`, and
`curl --fail http://127.0.0.1:4173/api/engine/health`. A web-only launch with
`node scripts/serve-web.mjs` reports engine `not_configured` and returns 503 for
engine health. Optional OPA and operator-run Syft/Grype reports do not become
required through a public configuration flag. `engineEnabled` remains false in
the existing mirrored public runtime configuration.

## Local lifecycle output

Both direct Node entry points write version-1 JSON lines with fixed event names,
`entryPoint` (`local` or `web`), a fresh per-launch UUID `runId`, `coreReady: false`
and separate `readiness` states. A successful local `started` event follows the
real Go health probe and web listener startup. It reports `web: ready` and
`engine: process_healthy`; it is a startup snapshot, not continuous monitoring.
The web-only entry point reports `engine: not_configured`.

There are at most two application diagnostic events per launch, each below
1024 UTF-8 bytes: `started` and one terminal event, or just a startup failure.
`started` includes validated numeric-loopback host/port and, for the local
launcher, the OS-assigned engine PID. HTTP requests produce no access log and
there is no diagnostic history, file retention, polling loop or diagnostics API.
Normal lifecycle events go to stdout; failures go to stderr. npm's own banner
is outside this application JSON stream; use the Node entry point directly
when a JSON-only stream is needed.

- `startup_failed` (exit 1): check the pinned prerequisites with `npm run doctor`,
  rebuild with `npm run setup`, verify public configuration and built assets,
  then check whether the requested web port is occupied. The launcher cleans
  up any child it started; it never stops an unrelated listener.
- `runtime_failed` (exit 1): the engine exited unexpectedly, the web listener
  failed, or shutdown required forced termination. Restart the local launcher
  after checking prerequisites. No accepted domain state is changed by this
  launch profile.
- `stopped` (exit 0): requested shutdown/cancellation completed. A fresh launch
  gets a new `runId` and repeats the real health check.

Terminal events report `web: not_ready`; the local engine is `unavailable`.
Storage/providers remain `not_integrated` throughout. Check the health routes
for current state while the server is running. A crash closes the unified web
listener; neither a previous `started` event nor a healthy standalone web
listener proves the Go process is still healthy.

## Data minimization and remaining work

Diagnostics allowlist fixed fields instead of filtering arbitrary secret text.
Raw errors/stacks, filesystem paths, input/configuration bytes, request URLs,
headers/bodies, environment values, and child stdout/stderr are not forwarded.
Only the bounded initial engine listening event is parsed privately; remaining
stdout is drained/discarded and stderr is sent to the null device. Synthetic
hostile-process tests verify this boundary separately from the real-Go tests.
This deliberately limits error detail; it is not a general secret detector or
an audit trail. Programmatic consumers of `startLocal` still receive errors and
must apply their own output policy.

The implementation follows Node's documented [stdio handling](https://nodejs.org/docs/latest-v24.x/api/child_process.html#optionsstdio)
and [per-launch UUID generation](https://nodejs.org/docs/latest-v24.x/api/crypto.html#cryptorandomuuidoptions).
No submitted source is executed. There are no new runtime API routes or public
config fields. Before C30 can claim full local usability, it must integrate and
probe the actual authoritative store and any selected required providers, then
prove the reference workflow, restart and restore. Private/multi-user admission
and release publication remain separate gates.

## C30 approved local snapshot composition

The current CLI snapshot workflow, full file/signature verification, real storage readiness and safe backup/restore are described in [C30 local workflow](../docs/engineering/C30_LOCAL_WORKFLOW.md). The earlier C03/C20/C29 sections above retain their historical capability boundaries. Full C30 status is determined by source-bound evidence, not these instructions.
