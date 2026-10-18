# Local engine lifecycle (C03)

Use the Linux x86_64 toolchain locked by C02: Node 24.19.0, npm 11.9.0 and Go 1.27.2 on PATH. Go is a build prerequisite; setup does not install global runtimes. The checksum-pinned official Go source and tested bootstrap recipe are in [RESOURCE_LOCK.md](RESOURCE_LOCK.md). Optional OPA, Syft and Grype are not required for process health.

```sh
npm run setup
npm start
```

Setup retains npm ci with install scripts disabled, builds the web assets and builds the standard-library-only Go module using `GOTOOLCHAIN=local`, `CGO_ENABLED=0`, `-mod=readonly` and `-trimpath`. It probes the built executable's actual version before moving it to `dist/engine/attestara`. Setup invalidates the previous engine executable first and removes pending output on failure. Re-run setup after source changes or a partial failure. No third-party Go modules are needed, so there is no go.sum. `--skip-install` reuses installed npm dependencies; `--check` prints the preparation plan without executing it and does not assert readiness. `npm run doctor` reports built assets and executable preparation, independently of full product readiness.

The Node supervisor owns the existing web server and one directly spawned Go child. The engine binds `127.0.0.1:0`, reports its allocated address through a bounded startup message and must pass an actual health probe before the web listener starts. The default web URL is http://127.0.0.1:4173; existing `--host`, `--port`, `--root` and `--config` web options remain supported. Both listeners are loopback-only. No shell or uploaded source is executed.

`GET /api/engine/health` is the only engine route exposed through the web server. It returns the validated actual Go response:

```json
{"schemaVersion":1,"status":"ok","scope":"engine-process","version":"0.0.1","goVersion":"go1.27.2"}
```

The proxy accepts only a fixed loopback HTTP health URL, follows no redirects, permits at most 4096 response bytes and has a 1500 ms deadline. Missing, invalid or unavailable engine health returns HTTP 503 with `{"error":"engine_unavailable"}`. GET and HEAD are supported; mutations receive 405. Other `/api` routes remain unavailable. Public runtime configuration still cannot enable engine admission. The browser displays a process-health result or an explicit unavailable state; reload retries. It continues to keep preview bytes within tab memory.

Web `/healthz` and `/readyz` retain their web-only meaning and `engineReady:false`; C03 process health does not certify domain functionality, storage, provider readiness, identity, trust or security results. The standalone web artifact can run without the engine and truthfully displays engine unavailable. Full engine packaging remains C31.

SIGINT/SIGTERM close the web listener and terminate the Go child. Go allows up to three seconds for HTTP shutdown; the supervisor force-stops a child still present after four seconds and reports failure. Unexpected engine exit or a web-listener failure closes the whole runtime with a nonzero exit. An occupied web port also terminates the engine before returning failure. Restart repeats the same process-health contract; C03 owns no durable product data to restore.

## Local evidence CLI (C20)

The built engine also accepts `evaluate`, `verify` and `export` subcommands; the
existing process-health options remain unchanged.

```sh
dist/engine/attestara evaluate --input input.json --policy rule.json --evidence evidence.json --freshness fresh > passport.json
dist/engine/attestara verify --passport passport.json
dist/engine/attestara export --passport passport.json > portable-passport.json
```

`--evidence` may be omitted; the decision then remains unknown. `--freshness`
must be explicitly set to `fresh` or `stale`; the CLI does not infer age or
expiry. Exit codes are 0 for pass/success, 1 for policy fail or unknown, 2 for
invalid input/version/digest, and 3 for engine or output failure. Non-pass
results from all three commands still write the typed C19 passport to stdout.
`verify` checks only
the passport version and canonical digest. It does not check referenced file
bytes, signatures, signer trust or revision/replay authority; successful output
continues to say `verification: unverified`.

The existing web CI now prepares Go and the engine before its engine-dependent web/browser/clone checks. `actions/setup-go@924ae3a1cded613372ab5595356fb5720e22ba16` (official v6 tag, MIT) supplies the C02 Go 1.26.2 bootstrap compiler; the final compiler is built from the C02 source archive after SHA-256 verification. This repairs the existing job's new prerequisite; C06 still owns broader domain integration CI. No remote CI run is implied by a local test result.

Implementation references: [Node 24.19 child processes](https://nodejs.org/download/release/v24.19.0/docs/api/child_process.html#child_processspawncommand-args-options), [Go graceful HTTP shutdown](https://pkg.go.dev/net/http@go1.27.2#Server.Shutdown), [Go signal context](https://pkg.go.dev/os/signal@go1.27.2#NotifyContext). Source-bound commands, versions and fixture hashes are recorded in `docs/evidence/C03.json`.
