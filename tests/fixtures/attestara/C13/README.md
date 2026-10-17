# C13 declared license expression recognition

`policy.AssessLicenseExpression` takes one expression string and returns its
exact input and a `recognized` or `review` status. Neither status is a compliance
or release decision. There is no allow/deny policy, HTTP/UI, SBOM license-field
projection, signature trust, network lookup, provider execution or persistence.

The parser uses the SPDX 3.0.1 License Expressions annex: case-insensitive
standard identifiers, uppercase or lowercase (not mixed-case) operators,
parentheses, `AND`, `OR`, `WITH`, immediate `+`, and custom license/addition
references. `WITH` requires a simple expression and an exception or addition
reference, not a parenthesized compound. Exception IDs are not license IDs.
Every operand is inspected, including every alternative of an `OR` expression.
`OR` alone does not require review. The recognizer does not calculate Boolean
policy decisions or rewrite expressions; operator precedence does not change
whether a term requires review.

The existing CycloneDX schema is moved unchanged to `internal/licensecatalog/`.
Its enum is the C02-pinned SPDX List 3.29.0 snapshot (740 licenses followed by 86
exceptions). Both sections were compared byte-for-byte as identifier sequences
against the official versioned license-list JSON files; URLs and hashes are in
`provenance.json`. Those JSON files are not shipped or read at runtime. Standard
IDs absent from this pinned version, unresolved custom references, `NONE`,
`NOASSERTION`, and blank declarations return `review`. A known alternative cannot
hide an unknown alternative. Deprecation and compatibility are not assessed.

Malformed syntax, invalid UTF-8, control characters, non-ASCII identifiers and
incorrect license/exception roles fail with a safe `contracts.Error` and an empty
assessment. This library accepts single-line expressions with spaces/tabs. The
C04 text boundary of 4096 Unicode code points bounds allocation and recursion;
longer input returns `limit_exceeded`. Source-like text remains inert. Missing
or modified embedded catalog bytes fail initialization rather than recognizing
identifiers against a placeholder list.

Fixtures and expected outputs are repository-authored synthetic Apache-2.0 data.
Tests check positive and negative expressions, all pinned identifier roles,
catalog byte ownership, C10 regression, maximum-length/nested input and identical
results in a new process. The restart case is read-only repetition, not durable
import recovery. Full release-rule evaluation remains C14 and later integration.
