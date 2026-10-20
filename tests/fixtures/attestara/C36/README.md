# C36 independent onboarding fixtures

Repository-authored Apache-2.0 acceptance cases for replaying the public local guide in a clean clone. The existing C01 ten-step specification remains historical; C36 binds its fresh-run checks to the existing C30 CLI, store and browser workflow. These fixtures contain no customer data, credentials or executable source.

The hosted runner records its actual commit, source identity, tool versions, fixture hashes, elapsed phases, expected CLI exits and observed failures in `dist/onboarding/C36-run.json`. A local bubblewrap run validates the harness but is not independent-runner evidence.
