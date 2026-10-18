# C28 local CLI measurement fixtures

Repository-authored Apache-2.0 scenario metadata reuses the exact C14, C20
and C23 synthetic fixture bytes. The benchmark pads the valid C14 input with
JSON whitespace to the existing 100 MiB input ceiling, then adds one byte for
the rejected case. Whitespace does not alter the typed release fields. The
expected passport remains the C20 unverified passport; no scanner, signer,
provider or user source is executed.

`positive.json`, `negative.json` and `expected.json` declare the assertions;
`tests/scenarios/target-workload.json` supplies the bounded byte and process
counts. Performance numbers belong only in the observed C28 evidence.
