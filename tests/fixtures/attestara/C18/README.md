# C18 decision reason fixtures

Repository-authored Apache-2.0 synthetic data. Version-1 case/expectation
documents reuse the actual C14 evaluator fixtures; grype.empty.json is inert
C12-compatible synthetic report data, not output from an executed scanner.
provenance.json lists every local fixture SHA-256.

Freshness is supplied independently by the caller. No database age threshold,
expiry timestamp or reported-valid claim establishes freshness here. Missing
evidence and stale evidence yield unknown; fresh policy failure yields fail.
Unknown policy facts with available references remain unknown without inventing
a more specific reason. Stale overrides both pass and fail. Missing plus stale
records both reasons, in that order. Existing warnings remain in order; generated
reason codes are appended only once. Failed evaluation envelopes remain failed.

The function is stateless: restart proof repeats the same fixtures in a fresh
process. It performs no persistence, scanner execution or revision acceptance.
