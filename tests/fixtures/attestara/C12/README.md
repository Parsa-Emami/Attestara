# C12 read-only Grype JSON subset

`DecodeGrypeReport` accepts existing JSON presenter output shaped by the C02-pinned
Grype 0.120.1 source. It does not run Grype, load a vulnerability database, fetch
URLs, or write accepted state. The output projects only scanner version/time,
reported DB schema/build time/validity, NVD capture time, and each match's
vulnerability ID/severity and affected package ID/name/version. Input order is
retained. Grype configuration, registry auth, DB path, match details and raw
report bytes are not returned.

`database_age_seconds` is calculated only when both Grype's scan timestamp and
DB build timestamp are present and ordered. The build timestamp is retained even
when age cannot be calculated. `reported_valid` repeats Grype's own status; it
does not independently verify the DB. `cve_feed_status` is `reported` only when
the report says the DB is valid and includes an NVD capture timestamp. Missing
DB/status/NVD metadata yields `unknown`, including an empty match list. A Grype
source build may put `[not provided]` in `descriptor.version`; that literal is
preserved but is not evidence of an exact scanner version.

All fixtures are repository-authored synthetic data under Apache-2.0. The
format source and checksum-verified upstream archive are recorded in
`provenance.json`. Tests check malformed and oversized input, secret omission,
inert source text, and deterministic decoding in a fresh process. This is
read-only repetition, not durable import recovery or producer trust.
