# C23 offline export fixtures

The v1 manifest and three inert files are repository-authored Apache-2.0
synthetic C08 inputs copied unchanged for export interoperability. `manifest.json`
is the positive verifier input. The negative cases describe mutations applied by
`tests/export_test.go`; no user source, provider or scanner is executed.

The export remains unverified. `bundle.ReadArchive` checks bytes and paths, not
signer trust, expiry, accepted revision or replay authority.
