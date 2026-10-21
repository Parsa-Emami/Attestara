# Local CLI snapshot workflow

Supported profile: one Linux x86_64 OS operator, existing CLI and loopback static viewer. Snapshot acceptance is local immutable storage, never latest-release authority or producer authenticity. No paid account, scanner execution, hosted payloads or user-source execution is required.

Use Node 24.19.0, npm 11.9.0 and Go 1.27.2 from the existing toolchain lock. Run `npm run setup`, then `npm start`. The default store is `.local-data/attestara`; `npm start -- --store /absolute/private/store --port 4301` selects another store. Its parent is prepared locally; the file store requires a private directory owned by the operator. CLI paths are explicit and never selected by uploaded data.

## Evaluate and view

```sh
dist/engine/attestara evaluate --input tests/fixtures/attestara/C30/manifest.json --files tests/fixtures/attestara/C30 --policy tests/fixtures/attestara/C30/rule.json --evidence tests/fixtures/attestara/C30/evidence.json --freshness fresh --store .local-data/attestara --operation-id release-1 > passport.json
dist/engine/attestara verify --store .local-data/attestara --operation-id release-1
```

Open `http://127.0.0.1:4173/#passport` and select `passport.json`. The viewer renders claims without uploading bytes. It does not independently authenticate them. CLI commands work while the supervised engine is running; the service holds no permanent storage writer lock. `--files` validates all referenced bytes plus at least one declared CycloneDX JSON 1.7 SBOM. A valid SBOM is not a vulnerability/compliance verdict. The supplied evidence facts describe evidence presence, not scanner findings. Other declared files remain inert.

`--store` and `--operation-id` must be supplied together and require `--files`. Repeating the same operation and exact input/result bytes is idempotent; changing content or freshness under that ID conflicts. Use a new operation ID for a different snapshot. Non-pass results may be stored but remain non-pass. Omitting `--evidence` in the original evaluation mode yields unknown. Explicit stale evidence also yields unknown. No implicit age threshold, expiry, global ordering or authority against replay of older releases is claimed.

## Independent file and signature verification

```sh
dist/engine/attestara verify --input tests/fixtures/attestara/C30/manifest.json --files tests/fixtures/attestara/C30 --passport passport.json --signature tests/fixtures/attestara/C30/signature.json --trust tests/fixtures/attestara/C30/trust.json
dist/engine/attestara export --input tests/fixtures/attestara/C30/manifest.json --files tests/fixtures/attestara/C30 --output release.zip
dist/engine/attestara verify --archive release.zip --passport passport.json
```

The optional `--signature` JSON has version `schema_version: "1"`, `signer_id`, `public_key_hex` (32-byte Ed25519 key) and `signature_hex` (64-byte signature over exact manifest bytes). `--trust` is independently configured version-1 JSON with `trusted_signers`, a map of exact identity to public-key hex. Uploaded keys never configure trust. C30 keys/signatures are synthetic examples; replace the configuration with independently established identities for real use. Signatures do not authenticate a policy decision or assert expiry/revision authority.

Full verification emits a version-1 assessment with `file_integrity`, `cryptographic_valid`, `signer_trusted` and `manifest_trusted`. With `--passport`, its unchanged passport goes to stdout and the independent assessment to stderr; without it the assessment goes to stdout. Invalid signatures reject with exit 2. Unsigned and valid-but-untrusted remain unverified. Passport digest-only mode is preserved. Matching checks include redacted metadata and manifest-bound evidence references; a policy outcome is not recomputed by `verify`.

## Backup, restart and restore

```sh
dist/engine/attestara export --store .local-data/attestara --operation-id release-1 --output ./release-backup
# Stop npm start, restart it, then repeat verify --store above.
dist/engine/attestara restore --backup ./release-backup --store ./restored-store --operation-id release-1
dist/engine/attestara verify --store ./restored-store --operation-id release-1
```

The portable backup directory uses the versioned local manifest/objects format and contains exactly the selected operation's input/result/files. Transfer the whole directory with private permissions preserved; never copy a live mutable store as a substitute. Backup and restore stage privately, commit, reopen/revalidate and atomically publish with Linux `renameat2(RENAME_NOREPLACE)` plus parent-directory sync. Every existing destination, including an empty directory or symlink, is rejected. No merge, replacement or rollback of newer accepted data occurs.

A missing/corrupt source, cancellation, quota/disk/lock failure cannot be reported as success. After an acknowledgement failure, inspect/reopen the destination before retrying: publication may already be durable. After a killed copy, an unpublished `.attestara-snapshot-*` directory may remain; preserve valid accepted data and inspect only task-owned leftovers before manual cleanup. The original store is not deleted.

Exit codes retain 0 for success/pass, 1 for policy fail/unknown, 2 for invalid input/digest/signature and 3 for storage/engine/output failure. Missing runtime prerequisites fail explicitly; unsupported provider options are rejected. OPA/Syft/Grype are optional existing tools and are not executed by this profile.

## Readiness and acceptance

Unified `/readyz` checks actual engine process health and a bounded Open/Close reconciliation of local storage. It reports `scope: local-cli-snapshots`, `storage: ready|locked|corrupt|unavailable`, builtin policy and an empty required-provider list. It is a point-in-time probe, not a persistent authorization/availability guarantee. Concurrent CLI writes can briefly yield 503/locked. `/healthz` remains web-process health; standalone web stays web-only. Domain HTTP APIs remain disabled. Diagnostics expose fixed states, never store paths or raw provider errors.

`npm run test:clone` needs outbound access for locked dependencies/browser preparation. Locally it requires the test-only system utility bubblewrap and runs in a Linux namespace with fresh HOME/npm/Go/browser caches and installed pinned prerequisites. On GitHub Actions it uses the runner's ephemeral VM and creates fresh HOME/npm/Go/browser caches inside the real Git clone; it does not create a nested user namespace, which hosted runners may deny. The host kernel/network are shared in both modes; this does not provide a separate physical machine. If fresh Playwright downloads are unavailable, an explicit `WEB_TEST_CHROMIUM=/opt/google/chrome/chrome` may select a system-installed test browser under `/usr` or `/opt`; the actual version/hash is recorded. Developer browser caches are rejected. It runs actual setup/start, CLI negatives, service restart, restore and browser parity. Generated test reports are retained in CI artifacts.
