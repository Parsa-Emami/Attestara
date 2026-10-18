# Local Syft and Grype reports

Syft and Grype are optional local tools. The operator runs them; Attestara does
not start scanners, fetch vulnerability databases, accept a remote scan target,
or execute files from the scanned source. C24 documents a manual report workflow
and does not add a runtime adapter or API.

The project resource lock pins Syft `1.54.1` and Grype `0.120.1`; see
[`toolchain-lock.json`](../engineering/toolchain-lock.json) and
[`RESOURCE_LOCK.md`](../engineering/RESOURCE_LOCK.md). Check the installed
executables before scanning and keep the output directory outside the scan root:

```sh
set -eu
: "${SCAN_ROOT:?set SCAN_ROOT to an authorized local directory}"
: "${REPORT_DIR:?set REPORT_DIR to a private output directory outside SCAN_ROOT}"
umask 077
mkdir -p "$REPORT_DIR"

syft version --output json > "$REPORT_DIR/syft-version.json"
grype version --output json > "$REPORT_DIR/grype-version.json"
grype db status --output json > "$REPORT_DIR/grype-db-status.json"

syft "$SCAN_ROOT" --output "syft-json=$REPORT_DIR/syft.json"
grype "sbom:$REPORT_DIR/syft.json" --output json > "$REPORT_DIR/grype.json"
```

Set `SCAN_ROOT` to the local directory the operator is authorized to
inspect and `REPORT_DIR` to a private directory outside that input. Use the
locked tool versions. Grype uses a local vulnerability database and may need an
available database before producing a useful report. If the local database is
missing and network access is permitted, update it explicitly with
`grype db update`; otherwise stop with an explicit error. Inspect `grype db status` and do
not treat an empty findings list as proof that scanning succeeded.

Syft's JSON descriptor records its scanner version. Grype's JSON descriptor
records its scanner version and, when available, database schema, build time,
and reported validity. Preserve those report bytes with the separate version
and database-status outputs. Compare the report's scanner version with the
binary output and the project lock. A source-built binary that reports
`[not provided]` does not satisfy the exact-version record; do not fill that
value from an assumption. A Grype database schema/build time is reported
metadata, not a digest or independent proof of the database's contents or
freshness.

`internal/adapters.DecodeGrypeReport` reads an existing Grype JSON report. Its
typed projection retains the reported scanner version, database schema/build
time/status, NVD capture metadata and vulnerability/package matches. Missing
database or feed metadata remains `unknown`; the adapter does not run a scan,
change the input, or verify a provider's claims. The C10 CycloneDX reader
supports CycloneDX JSON 1.7; Syft's native JSON report is only used here as
Grype's local input and is not a C10-validated SBOM.

Keep raw reports and scanner diagnostics local unless their contents have been
reviewed for disclosure. Reports may contain local paths or source/package
details. Keep credentials out of reports, command lines and published artifacts;
protect any scanner configuration that contains credentials. Do not publish raw
reports as passports. C25 owns public-passport redaction; C26 owns bundle trust
outcomes. A report, version string, or successful database-status message is
not a signature, provenance proof, freshness decision, or trusted result.

The companion [`adapter-manifest.json`](../../tests/fixtures/adapter-manifest.json)
is repository-authored synthetic metadata for this documented shape. Its
versions and database timestamps illustrate fields only; no scanner or live
database was used to create it. Upstream references: [Syft CLI](https://oss.anchore.com/docs/reference/syft/cli/), [Syft output formats](https://oss.anchore.com/docs/guides/sbom/formats/), [Grype CLI](https://oss.anchore.com/docs/reference/grype/cli/), and [Grype result interpretation](https://oss.anchore.com/docs/guides/vulnerability/interpreting-results/).
