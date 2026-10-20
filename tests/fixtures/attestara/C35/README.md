# C35 synthetic release verification fixtures

Repository-authored Apache-2.0 scenarios. The baseline is the actual C34 source commit, not a published release. Unit tests create bounded synthetic C31 outputs and ephemeral Ed25519 keys; the package harness builds and installs actual baseline/candidate packages. Private keys remain in memory and are never serialized. The trust map is explicitly enrolled independently of the signed artifact. Public verification results are retained under dist/release-verification/.

Freshness and the expected source commit are operator inputs. Reverification is stateless; no automatic latest-version, clock expiry or persistent anti-replay authority is claimed. Rollback changes software directories while retaining the same external store.
