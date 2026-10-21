#!/usr/bin/env python3
from pathlib import Path
import json
import re
import sys

root = Path(__file__).resolve().parents[1]
errors = []
required = ['README.md', 'LICENSE', 'NOTICE', 'THIRD_PARTY_NOTICES.md', 'SECURITY.md',
            'CONTRIBUTING.md', '.env.example', 'package.json', 'package-lock.json',
            'go.mod', 'go.sum', 'cmd/attestara/main.go', 'apps/web/src/main.tsx',
            'contracts/input.schema.json', 'contracts/result.schema.json', 'contracts/result.v2.schema.json',
            'docs/release/USER_GUIDE.md', 'docs/engineering/toolchain-lock.json',
            '.github/workflows/contracts.yml', '.github/workflows/runtime.yml',
            '.github/workflows/web.yml', '.github/workflows/release-gate.yml', '.github/workflows/pages.yml']
for name in required:
    if not (root / name).is_file():
        errors.append('Missing required application file: ' + name)
ignored = {'.git', 'node_modules', 'dist', 'test-results', 'playwright-report',
           'vendor', 'build', '.cache', '.local-data', '__pycache__'}
count = 0
for path in root.rglob('*.json'):
    if ignored.intersection(path.relative_to(root).parts):
        continue
    try:
        json.loads(path.read_text(encoding='utf-8'))
        count += 1
    except (ValueError, OSError) as error:
        errors.append('Invalid JSON ' + str(path.relative_to(root)) + ': ' + str(error))
for path in (root / '.github/workflows').glob('*.yml'):
    for action in re.findall(r'uses:\s*([^\s]+)', path.read_text()):
        if not re.fullmatch(r'[A-Za-z0-9_.-]+/[A-Za-z0-9_./-]+@[0-9a-f]{40}', action):
            errors.append('CI action must be pinned to a full SHA: ' + action)
if errors:
    print('VALIDATION FAILED\n' + '\n'.join(' - ' + error for error in errors))
    sys.exit(1)
print(f'Validated application files, {count} JSON documents and pinned CI actions.')
