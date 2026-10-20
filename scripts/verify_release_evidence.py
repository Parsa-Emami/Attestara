#!/usr/bin/env python3
"""C39 historical evidence audit; never executes commands recorded in evidence."""
from pathlib import Path, PurePosixPath
import argparse
import hashlib
import io
import json
import re
import sys
import zipfile

from verify_ci_source import IntegrityError, git, tree_files

TASKS = [f'C{i:02}' for i in range(1, 39)]
EXCLUDED = {'plans/progress.json', 'plans/requirements.json', 'plans/project-state.json',
            'docs/agents/SESSION_HANDOFF.json', 'apps/web/src/generated/project.json', 'SHA256SUMS.txt'}
LEGACY_PREFIX = '/home/amirreza/Desktop/Projects/Attestara/'


def require(condition, message):
    if not condition:
        raise IntegrityError(message)


def sha256(data):
    return 'sha256:' + hashlib.sha256(data).hexdigest()


def canonical_digest(value):
    require(isinstance(value, str) and re.fullmatch(r'(sha256:)?[a-f0-9]{64}', value), 'Invalid output digest')
    return 'sha256:' + value.removeprefix('sha256:')


def confined(name):
    return (isinstance(name, str) and bool(name) and not PurePosixPath(name).is_absolute()
            and not any(part in {'', '.', '..'} for part in name.split('/'))
            and not any(char in name for char in '\\\0\r\n'))


def read(root, name):
    require(confined(name), 'Unsafe evidence/output path')
    path = root / name
    require(not any(parent.is_symlink() for parent in [path, *path.parents] if parent != root.parent),
            'Symlink evidence/output path')
    require(path.is_file() and path.stat().st_size <= 32 * 1024 * 1024, 'Missing/oversized evidence/output: ' + name)
    return path.read_bytes()


def unique(pairs):
    result = {}
    for key, value in pairs:
        require(key not in result, 'Duplicate JSON key: ' + key)
        result[key] = value
    return result


def document(root, name):
    return json.loads(read(root, name), object_pairs_hook=unique)


def method(task):
    number = int(task[1:])
    # C32's recorded description says raw; its actual historical digest uses hex/LF.
    return 'hex-lf' if 5 <= number <= 19 or task == 'C32' else ('raw' if number <= 25 else 'raw-lf')


def identity(files, framing):
    digest = hashlib.sha256()
    for name, value in sorted(files.items()):
        require(isinstance(value, str) and re.fullmatch(r'(sha256:)?[a-f0-9]{64}', value), 'Invalid source digest')
        value = value.removeprefix('sha256:')
        digest.update(name.encode() + b'\0')
        digest.update(value.encode() if framing == 'hex-lf' else bytes.fromhex(value))
        if framing != 'raw':
            digest.update(b'\n')
    return 'sha256:' + digest.hexdigest()


def source_inputs(files):
    return {name: item for name, item in files.items() if name not in EXCLUDED and not name.startswith('docs/evidence/')}


def current_source(root):
    commit = git(root, 'rev-parse', 'HEAD').decode().strip()
    files = source_inputs(tree_files(root, commit))
    for name, item in files.items():
        require(item['mode'] in {'100644', '100755'} and read(root, name) == item['bytes'],
                'Current source changed: ' + name)
        require(bool((root / name).stat().st_mode & 0o111) == (item['mode'] == '100755'), 'Current source mode changed: ' + name)
    return {'commit': commit, 'tree': git(root, 'rev-parse', 'HEAD^{tree}').decode().strip(),
            'identity': identity({name: item['sha256'] for name, item in files.items()}, 'raw-lf'),
            'files': {name: {'sha256': item['sha256'], 'mode': item['mode']} for name, item in sorted(files.items())}}


def references(value):
    if isinstance(value, dict):
        if 'output_path' in value:
            yield value['output_path'], value.get('output_sha256')
        for key in ['record', 'upgrade_record']:
            if key in value and key + '_sha256' in value:
                yield value[key], value[key + '_sha256']
        for child in value.values():
            yield from references(child)
    elif isinstance(value, list):
        for child in value:
            yield from references(child)


def verify_evidence(root):
    root = root.resolve()
    require(git(root, 'rev-parse', '--is-shallow-repository').strip() == b'false', 'Full Git history required (fetch-depth: 0)')
    gates = document(root, 'plans/release-gates.json')
    progress = document(root, 'plans/progress.json')
    project = document(root, 'plans/project-state.json')
    require(gates.get('release_prerequisites') == TASKS, 'Exact C01-C38 prerequisites required')
    require(gates.get('core_ready') is True and progress.get('current_core_ready') is True
            and project.get('core_ready') is True, 'Core readiness is incomplete')
    require(gates.get('verified_profile') == project.get('verified_profile') == 'local-cli-snapshots', 'Unsupported profile')
    contract = gates.get('evidence_contract', {})
    require(type(contract.get('schema_version')) is int and contract['schema_version'] == 1, 'Unsupported evidence contract')
    records = contract.get('historical_sources', {})
    require(set(records) == set(TASKS), 'Exact historical source coverage required')
    entries = progress.get('commits', [])
    require(all(isinstance(entry, dict) and isinstance(entry.get('id'), str) for entry in entries), 'Invalid task states')
    states = {entry['id']: entry for entry in entries}
    require(len(states) == len(entries) and all(task in states for task in TASKS), 'Missing/duplicate task states')
    trees, checked_outputs, verified = {}, {}, []

    def snapshot(commit):
        require(isinstance(commit, str) and re.fullmatch(r'[a-f0-9]{40}', commit), 'Invalid historical commit')
        if commit not in trees:
            git(root, 'merge-base', '--is-ancestor', commit, 'HEAD')
            trees[commit] = tree_files(root, commit)
        return trees[commit]

    for task in TASKS:
        path = f'docs/evidence/{task}.json'
        state, record = states[task], records[task]
        require(state.get('status') == 'verified' and state.get('evidence') == [path], task + ': unverified/missing evidence')
        raw = read(root, path)
        require(sha256(raw) == record.get('evidence_sha256'), task + ': evidence checksum mismatch')
        proof = json.loads(raw, object_pairs_hook=unique)
        require(proof.get('passed') is True and proof.get('planning_template') is False
                and proof.get('commit_id') == task and proof.get('source_identity') == state.get('source_identity'), task + ': invalid proof')
        commands = proof.get('commands')
        require(isinstance(commands, list) and bool(commands), task + ': missing commands')
        require(all(isinstance(command, dict) and type(command.get('exit_code')) is int and command['exit_code'] == 0
                    and isinstance(command.get('command'), (str, list)) and bool(command['command']) for command in commands),
                task + ': failed/invalid command')
        if proof.get('blockers'):
            note = contract.get('historical_scope_notes', {}).get(task, {})
            require(note.get('original_blockers') == proof['blockers'] and bool(note.get('disposition'))
                    and bool(note.get('verified_by')) and all(ref in TASKS and states[ref].get('status') == 'verified'
                                                             for ref in note['verified_by']), task + ': unresolved blockers')
        files = dict(snapshot(record.get('source_commit')))
        omissions, overrides = record.get('source_omissions', []), record.get('source_overrides', {})
        if omissions or overrides:
            require(task == 'C37' and omissions == ['docs/product/BETA_FIXES.md']
                    and overrides == {'docs/ROADMAP_FA.md': proof.get('base_commit')}, 'Unapproved source overlay')
            files.pop(omissions[0])
            for name, commit in overrides.items():
                files[name] = snapshot(commit)[name]
        names = proof.get('source_files')
        if names is not None:
            require(isinstance(names, dict) and bool(names), task + ': empty source manifest')
            for name, digest in names.items():
                lookup = name.removeprefix(LEGACY_PREFIX) if task == 'C07' else name
                require(confined(lookup) and lookup in files and files[lookup]['mode'] in {'100644', '100755'}
                        and isinstance(digest, str) and files[lookup]['sha256'] == digest.removeprefix('sha256:'),
                        task + ': historical source bytes differ')
        else:
            names = {name: item['sha256'] for name, item in source_inputs(files).items()}
        require(identity(names, method(task)) == proof.get('source_identity'), task + ': historical source identity differs')
        for output, digest in references(proof):
            external = contract.get('external_outputs', {}).get(output)
            if external:
                archive = read(root, external['archive'])
                require(sha256(archive) == digest, task + ': external output archive checksum mismatch')
                with zipfile.ZipFile(io.BytesIO(archive)) as zipped:
                    require(zipped.namelist() == [external['member']] and zipped.getinfo(external['member']).file_size <= 8 * 1024 * 1024,
                            'Invalid external output archive')
                    data = zipped.read(external['member'])
                require(data == read(root, external['path']) and sha256(data) == external['sha256'], 'External output member differs')
                output, digest = external['path'], external['sha256']
            else:
                require(confined(output), task + ': unsafe output reference')
                data = read(root, output)
                digest = canonical_digest(digest or contract.get('output_sha256', {}).get(output))
                if sha256(data) != digest:
                    recovery = contract.get('output_recoveries', {}).get(output, {})
                    require(bool(recovery), task + ': output checksum mismatch: ' + output)
                    recovered = read(root, recovery['path'])
                    attestation = document(root, recovery['attestation'])
                    require(recovered == data + b'\n' and sha256(recovered) == digest
                            and attestation.get('passed') is True and attestation.get('planning_template') is False
                            and attestation.get('source_commit') == record['source_commit']
                            and attestation.get('source_identity') == proof['source_identity']
                            and len(attestation.get('commands', [])) == 1
                            and attestation['commands'][0].get('exit_code') == 0
                            and attestation['commands'][0].get('output_path') == recovery['path']
                            and attestation['commands'][0].get('output_sha256') == digest,
                            task + ': invalid source-bound output recovery')
                    output = recovery['path']
            checked_outputs[output] = digest
        verified.append({'id': task, 'evidence_sha256': sha256(raw), 'source_commit': record['source_commit'],
                         'source_identity': proof['source_identity'], 'framing': method(task)})
    return {'schema_version': 1, 'passed': True, 'tasks': verified, 'outputs': checked_outputs,
            'historical_scope_notes': contract.get('historical_scope_notes', {}),
            'runtime_tests_certified': False}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--current-source', action='store_true')
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[1]
    try:
        print(json.dumps(current_source(root) if args.current_source else verify_evidence(root), sort_keys=True))
        return 0
    except (IntegrityError, OSError, ValueError, KeyError, TypeError, zipfile.BadZipFile) as error:
        print('RELEASE EVIDENCE FAILED: ' + str(error), file=sys.stderr)
        return 2


if __name__ == '__main__':
    sys.exit(main())
