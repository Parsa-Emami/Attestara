# C15 synthetic bounded OPA fixtures

Repository-authored Apache-2.0 fixtures. Policy bytes are unchanged from C04;
facts and subject are unchanged from C14. Input manifests bind their actual
bytes, including the Rego policy digest. No scanner-specific fact is introduced.

The fixed query is `data.attestara.release.allow`: true maps to pass and false
to fail. The existing Rego default makes an absent/null fact false, unlike C14's
first-party predicate which retains unknown. Missing evidence bytes are an
explicit error. All successful outcomes retain unverified authenticity.

Only the exact approved C04 policy is supported, not arbitrary Rego modules.
Negative facts are re-bound to the manifest in tests so parsing, not an unrelated
hash mismatch, is exercised. Provider error/timeout/output-limit tests use small
synthetic executable fixtures; they do not certify real OPA behavior. Real tests
use OPA 1.21.2 and are separately required for C15 evidence. They explicitly skip
when neither ATTESTARA_TEST_OPA nor opa on PATH is available; such a run alone
cannot verify C15. The application itself takes an explicit absolute executable
path and never installs a provider or invokes a shell.

Reopen uses the real C05 file port; a fresh Go process repeats real OPA evaluation.
This is not new durability, SIGKILL, power-loss, C30 restore or signer-trust proof.
