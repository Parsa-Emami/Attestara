# C17 subject binding fixtures

Repository-authored Apache-2.0 synthetic data, using the unchanged C08 v1
manifest and its actual subject/evidence/policy files. `positive.json` is the
existing C07 v1 envelope shape with C08 subject and artifact metadata.
`expected.json` and `negative.json` declare outcomes consumed by Go tests.

SubjectBound means only that two validated subject digest claims agree.
Builder/source assertions and issuer authenticity remain unverified. This is
not SLSA/in-toto conformance, signature verification, or file-byte validation.
C08 and C16 independently supply those existing file/signature checks.
No execution, I/O, persistence or revision authority is added by C17.
