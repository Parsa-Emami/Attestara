# CI source integrity

`python3 scripts/verify_ci_source.py` verifies every tracked file against the
actual current Git commit, including bytes and executable modes. It rejects
missing/altered files, symlink substitution, staged changes and untracked source.
It reads Git objects directly, so `assume-unchanged` cannot conceal modified bytes.
A shallow clone can verify its complete current tree without older planning data.

The generated inventory is `dist/ci/source.json`, uploaded by the contracts
workflow. It contains commit/tree IDs, each file's mode/SHA-256 and a deterministic
source identity: sorted path, NUL, raw SHA-256 bytes, LF for every tracked file.
No source path is excluded as planning metadata. The `--snapshot` option prints
the checked commit/tree/identity and file hashes for the release gate.

```sh
python3 -m unittest discover -s tests/engineering -p 'test_*.py' -v
python3 scripts/validate_repo.py
python3 scripts/verify_ci_source.py
```

Run commit-bound inventory verification on a clean committed checkout. Behavioral
checks run separately; a matching inventory does not establish application safety
or certify previous executions. Distribution checksums/signatures are produced by
packaging and release tools; the source inventory does not replace them.
