# C37 acceptance audit cases

Repository-authored Apache-2.0 cases reuse existing C30/C33/C35/C36 inputs and regression checks. No new product payload format, runtime harness, trust authority or customer feedback is introduced. `cases.json` records the expected behavior; execution results and exact tested source belong to docs/evidence/C37.json, not this manifest.

The C03 failed-setup invalidation rule remains intentional. Optional OPA is exercised separately for the engine CI adapter regressions; the base CLI/viewer workflow requires no provider. Clone proof is local bubblewrap evidence, not a new independent hosted-runner claim.
