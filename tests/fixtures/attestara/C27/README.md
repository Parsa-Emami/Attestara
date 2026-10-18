# C27 local security fixtures

Version 1, repository-authored synthetic Apache-2.0 cases. `passport.json` reuses
C21's public fixture shape/digests and changes only free text to inert HTML-like
sentinels. Its report digest is deliberately not recomputed: C21 displays claims
and does not verify that digest. This is never a trusted/signed release fixture.

`cases.json` defines preview inputs, passport rejection messages and actual HTTP
method/path denial outcomes. Oversized bytes are generated in the test, not stored.
The browser tests consume every case; provenance pins both files and reused C21
bytes. The Authorization value is conspicuously synthetic, never a credential.

HTTP denials prove the absence of an enabled write/object/provider API, not
identity validation. Missing/expired/replayed identity, object ownership and
identity-provider failures remain unsupported, not passed or mocked. Local
single-user filesystem access relies on the operator's OS permissions. C32
network/private-ingress work cannot by itself establish user authorization.

No scanner, user source, private data or remote identity provider is executed.
