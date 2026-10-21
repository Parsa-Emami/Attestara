# Current-source release checks

```sh
npm run release:check
```

Run from a clean committed Linux x86_64 checkout with pinned Node/npm/Go on PATH,
Python 3, Git, GNU archive tools, Nginx and Chromium. Local clean-clone checks also
need `bwrap`; CI uses its ephemeral VM. The tool downloads and checksum-verifies
the optional OPA test binary unless `ATTESTARA_TEST_OPA` selects the exact pinned
artifact. Public dependency/provider downloads are test prerequisites.

Each invocation creates a new `dist/release-gate/run-*/report.json` and command
logs. The gate validates the current Git source before and after execution,
repository contracts, source-integrity regressions, setup, resources, required
Go test discovery, Go build/vet/tests, web checks/types, literal user-guide and
Nginx behavior, real browser tests, fresh clone workflow, reproducible installed
packages, dependency audit and the live private-reporting route.

The gate fails on a nonzero exit, timeout, interruption, incomplete required test
stream, source change or missing result. It never reuses an earlier green report.
Reports bind the actual commit/tree/source identity and tools to this run.
Required command IDs are checked in order before signing/publication.

Exit 0 means this invocation passed; 1 indicates operational prerequisites or
execution were unavailable, and 2 indicates an executed readiness check failed.
This gate neither signs nor publishes a release. See
[RELEASING.md](../release/RELEASING.md) for separate maintainer approval and public
installed-artifact acceptance. CI retains run artifacts rather than adding them
to the repository.
