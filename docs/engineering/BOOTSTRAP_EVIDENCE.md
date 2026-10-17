# C03 bootstrap verification boundary

C03 verifies the Go process lifecycle, unified setup/start, real health/version and the browser's process-status display. Synthetic Apache-2.0 positive, negative and expected-result fixtures live under `tests/fixtures/attestara/C03/`. No fixture is executed as source code.

Run setup with the pinned C02 toolchain on PATH, then `npm run check`, `go test ./...` and `npm run test:browser`. The Node tests use actual local Go processes for health, shutdown, restart, occupied-port cleanup, cancellation and crash behavior. Setup failure tests use isolated temporary outputs and include a real missing-Go process lookup. Invalid/oversized/redirected health responses exercise the actual proxy boundary. Go tests exercise a real listener and release its port after shutdown. Browser tests use the real unified launcher for the success case; response interception is used only for unavailable/malformed UI failures.

`docs/evidence/C03.json` records observed results and exact source/tool/fixture identities. A passing C03 result does not certify the domain reference workflow, clean-host minimum hardware, durable recovery, private admission or release readiness. C30/C40 gates remain closed. See [ENGINE_LAUNCH.md](ENGINE_LAUNCH.md) for launch and failure recovery.
