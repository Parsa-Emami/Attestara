# C29 local lifecycle diagnostics

Apache-2.0 repository-authored synthetic fixtures. `expected.json` defines
version-1 states, synthetic credential/path sentinels and diagnostic bounds.
`untrusted-engine.mjs` is copied as an executable into a temporary test checkout
only; it tests malformed/oversized startup output, invalid health and noisy
stdout/stderr. It is not evidence of Go/provider/storage behavior.

The independent real-Go subprocess test checks actual health, restart, crash,
shutdown and HTTP rejection without executing submitted source. Storage and
providers remain explicitly not integrated; no provider is required by the
current launch profile. Missing configured engine health fails with HTTP 503.
