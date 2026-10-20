"""C39: synthetic historical proofs cannot authorize a real release."""
from pathlib import Path
import copy
import hashlib
import json
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]
FIXTURES = ROOT / 'tests/fixtures/attestara/C39'
sys.path.insert(0, str(ROOT / 'scripts'))
import verify_release_evidence as release


class ReleaseEvidenceTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='c39-proof-')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.git('init', '--quiet')
        self.write('README.md', b'Apache-2.0 synthetic source; no runtime certification.\n')
        self.commit()
        source = self.git('rev-parse', 'HEAD').strip()
        files = release.tree_files(self.root, source)
        self.proofs, records, states = {}, {}, []
        for task in release.TASKS:
            number = int(task[1:])
            names = {'README.md': files['README.md']['sha256']}
            if task == 'C07':
                names = {release.LEGACY_PREFIX + name: sha for name, sha in names.items()}
            proof = {
                'commit_id': task, 'passed': True, 'planning_template': False,
                'source_identity': release.identity(names if number <= 11 else
                    {name: item['sha256'] for name, item in files.items()}, release.method(task)),
                'commands': [{'command': 'inert synthetic description', 'exit_code': 0}],
            }
            if number <= 11:
                proof['source_files'] = names
            path = f'docs/evidence/{task}.json'
            self.write(path, json.dumps(proof).encode())
            self.proofs[task] = proof
            records[task] = {'source_commit': source, 'evidence_sha256': release.sha256((self.root / path).read_bytes())}
            states.append({'id': task, 'status': 'verified', 'evidence': [path], 'source_identity': proof['source_identity']})
        self.commit()
        self.gates = {'core_ready': True, 'verified_profile': 'local-cli-snapshots',
                      'release_prerequisites': release.TASKS,
                      'evidence_contract': {'schema_version': 1, 'historical_sources': records,
                                            'output_sha256': {}, 'external_outputs': {}}}
        self.progress = {'commits': states, 'current_core_ready': True}
        self.write('plans/progress.json', json.dumps(self.progress).encode())
        self.write('plans/project-state.json', b'{"core_ready":true,"verified_profile":"local-cli-snapshots"}')
        self.write('plans/release-gates.json', json.dumps(self.gates).encode())
        self.commit()

    def git(self, *args):
        return subprocess.check_output(['git', *args], cwd=self.root, stderr=subprocess.PIPE).decode()

    def commit(self):
        self.git('add', '--all')
        self.git('-c', 'user.name=C39 fixture', '-c', 'user.email=fixture@example.invalid', 'commit', '--quiet', '-m', 'Synthetic proof')

    def write(self, name, data):
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(data)

    def check(self):
        return release.verify_evidence(self.root)

    def change_proof(self, task, change):
        proof = copy.deepcopy(self.proofs[task])
        change(proof)
        self.write(f'docs/evidence/{task}.json', json.dumps(proof).encode())

    def test_complete_history_and_fixed_legacy_c07_path_are_verifiable(self):
        result = self.check()
        self.assertEqual(38, len(result['tasks']))
        self.assertFalse(result['runtime_tests_certified'])

    def test_missing_changed_template_and_false_success_are_rejected(self):
        cases = json.loads((FIXTURES / 'cases.json').read_text())
        mutations = [lambda p, case=case: p.update({case['field']: case['value']}) for case in cases['evidence_mutations']]
        mutations.append(lambda p: p['commands'][0].update(exit_code=False))
        for change in mutations:
            with self.subTest(change=change):
                self.change_proof('C01', change)
                with self.assertRaises(release.IntegrityError):
                    self.check()
        (self.root / 'docs/evidence/C01.json').unlink()
        with self.assertRaises(release.IntegrityError):
            self.check()

    def test_duplicate_states_and_missing_prerequisites_are_rejected(self):
        for value in [self.progress['commits'] + [self.progress['commits'][0]], self.progress['commits'][1:]]:
            self.write('plans/progress.json', json.dumps({**self.progress, 'commits': value}).encode())
            with self.assertRaises(release.IntegrityError):
                self.check()

    def test_false_core_readiness_and_unverified_state_are_rejected(self):
        self.progress['commits'][0]['status'] = 'planned'
        self.write('plans/progress.json', json.dumps(self.progress).encode())
        with self.assertRaises(release.IntegrityError):
            self.check()

    def test_wrong_historical_source_is_rejected_even_with_rehashed_evidence(self):
        self.write('README.md', b'Changed source that was never tested by these proofs.\n')
        self.commit()
        self.gates['evidence_contract']['historical_sources']['C01']['source_commit'] = self.git('rev-parse', 'HEAD').strip()
        self.write('plans/release-gates.json', json.dumps(self.gates).encode())
        with self.assertRaisesRegex(release.IntegrityError, 'historical source'):
            self.check()

    def test_unsafe_and_symlink_evidence_never_reads_outside_checkout(self):
        path = self.root / 'docs/evidence/C01.json'
        path.unlink()
        path.symlink_to(self.root / 'README.md')
        with self.assertRaises(release.IntegrityError):
            self.check()
        self.assertFalse(release.confined('../outside'))
        self.assertFalse(release.confined('/outside'))
        self.assertFalse(release.confined('a\\outside'))

    def test_corrupted_output_is_rejected_and_command_text_stays_inert(self):
        proof = self.proofs['C01']
        proof['commands'][0].update(command='touch must-not-exist', output_path='docs/evidence/output.log',
                                    output_sha256='sha256:' + hashlib.sha256(b'original').hexdigest())
        self.write('docs/evidence/C01.json', json.dumps(proof).encode())
        self.gates['evidence_contract']['historical_sources']['C01']['evidence_sha256'] = release.sha256((self.root / 'docs/evidence/C01.json').read_bytes())
        self.write('plans/release-gates.json', json.dumps(self.gates).encode())
        self.write('docs/evidence/output.log', b'corrupted')
        with self.assertRaisesRegex(release.IntegrityError, 'output'):
            self.check()
        self.assertFalse((self.root / 'must-not-exist').exists())

    def test_shallow_history_fails(self):
        shallow = self.root / 'shallow'
        subprocess.run(['git', 'clone', '--quiet', '--depth=1', '--no-local', str(self.root), str(shallow)], check=True)
        with self.assertRaisesRegex(release.IntegrityError, 'history'):
            release.verify_evidence(shallow)

    def test_runtime_source_mutation_and_mode_change_are_rejected(self):
        release.current_source(self.root)
        source = self.root / 'README.md'
        original = source.read_bytes()
        source.write_bytes(b'changed')
        with self.assertRaisesRegex(release.IntegrityError, 'Current source changed'):
            release.current_source(self.root)
        source.write_bytes(original)
        source.chmod(0o755)
        with self.assertRaisesRegex(release.IntegrityError, 'mode changed'):
            release.current_source(self.root)


class CurrentHistoryTest(unittest.TestCase):
    def test_actual_c01_c38_source_and_output_compatibility(self):
        result = release.verify_evidence(ROOT)
        self.assertEqual(release.TASKS, [task['id'] for task in result['tasks']])
        self.assertEqual('hex-lf', result['tasks'][31]['framing'])
        self.assertFalse(result['runtime_tests_certified'])

    def test_c39_fixture_provenance(self):
        provenance = json.loads((FIXTURES / 'provenance.json').read_text())
        self.assertEqual('Apache-2.0', provenance['license'])
        self.assertEqual({'README.md', 'cases.json'}, set(provenance['files']))
        for name, expected in provenance['files'].items():
            self.assertEqual(expected, release.sha256((FIXTURES / name).read_bytes()))


if __name__ == '__main__':
    unittest.main()
