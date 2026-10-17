# C11 read-only SPDX subset

`DecodeSPDX` admits the compact JSON-LD spelling in the C02-pinned SPDX 3.0
release example: `https://spdx.org/rdf/3.0.0/spdx-context.jsonld`, `@graph`,
`software_Package`, and CreationInfo `specVersion: 3.0.0`. Context aliases,
scoped contexts, expanded RDF, other serialization formats/versions and other
node classes are unsupported. The embedded CycloneDX `spdx.schema.json` is a
license-list schema, not an SPDX document schema; C11 does not reuse it for SPDX.
No new schema or dependency is shipped. This adapter validates its subset,
not full SPDX schema/OWL conformance or producer/signature trust.

The graph contains exactly one SpdxDocument, one or more packages, CreationInfo
and Person/Organization/Tool creator nodes. IDs are unique absolute URIs;
CreationInfo may use blank-node IDs. CreationInfo references and document package
roots must resolve locally. Metadata remains inert and is retained privately for
export. Only package-to-package `Relationship` / `dependsOn` is projected;
qualifiers (`completeness`, `startTime`, `endTime`) and other relationships fail
with typed errors. External relationship targets are unsupported.

Package `spdxId` maps unchanged to `bom-ref`, `name` to `name`, optional
`software_packageVersion` to `version` and `software_packageUrl` to `purl`.
`software_primaryPurpose` is required by this subset. Exact type mappings are:

| SPDX purpose | CycloneDX component type |
| --- | --- |
| application, framework, library, container, device, firmware, file, data, platform | same spelling |
| operatingSystem | operating-system |
| deviceDriver | device-driver |
| model | machine-learning-model |

Missing/other purposes fail without guessing. Components, dependency sources and
targets are sorted by exact identifier; repeated dependency edges are merged.
Cycles and shared targets remain valid. A package without a dependency assertion
has no dependency entry: its dependencies remain unknown. No empty dependency
assertion is invented. SPDX NoneElement/NoAssertionElement targets are outside
this package-only subset.

`EncodeSPDX` uses the normalized projection for package facts and dependency
edges, revalidates output, and retains the decoded document metadata. It supports
round-trip of the decoded graph, not structural graph editing or creating a new
SPDX document from arbitrary data. Multiple unqualified dependency assertions
from the same source are grouped using the first relationship's ID/metadata;
individual assertion grouping/IDs are not part of this normalized graph contract.
No full canonical RDF serialization claim is made. Caller data is not mutated.

Positive/negative/expected fixtures are repository-authored Apache-2.0 synthetic
data. `provenance.json` identifies the checksum-verified C02 specification source.
Tests use the actual C09 reader for hostile ZIP paths/symlinks, SIGKILL an active
read-only decoding process and repeat decoding unchanged bytes. This proves
read-only interruption, not durable import transactions. C05/C09 regressions
continue to cover their own real storage/archive boundaries.
