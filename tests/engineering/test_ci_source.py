from pathlib import Path
import hashlib
import importlib.util
import subprocess
import tempfile
import unittest

SCRIPT = Path(__file__).resolve().parents[2] / 'scripts/verify_ci_source.py'
SPEC = importlib.util.spec_from_file_location('ci_source', SCRIPT)
ci_source = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(ci_source)


class CiSourceTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='ci-source-regression-')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name) / 'source'
        self.root.mkdir()
        self.git('init', '--quiet')
        self.write('.gitignore', 'dist/\n__pycache__/\n')
        self.write('.github/workflows/verify.yml', 'uses: actions/example@' + '1' * 40 + '\n')
        self.write('README.md', 'Synthetic licensed source fixture.\n')
        self.commit('Synthetic baseline')

    def git(self, *args):
        result = subprocess.run(['git', *args], cwd=self.root, text=True, capture_output=True)
        if result.returncode:
            self.fail(result.stderr)
        return result.stdout

    def write(self, name, content):
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content)

    def commit(self, message):
        self.git('add', '--all')
        self.git('-c', 'user.name=CI fixture', '-c', 'user.email=fixture@example.invalid', 'commit', '--quiet', '-m', message)

    def test_every_committed_file_is_bound_to_the_current_commit_and_tree(self):
        before = ci_source.verify(self.root)
        content = 'uses: actions/example@' + '2' * 40 + '\n'
        self.write('.github/workflows/verify.yml', content)
        self.write('data/record.json', '{"synthetic":true}\n')
        self.commit('Synthetic source update')
        proof = ci_source.verify(self.root)
        self.assertEqual(self.git('rev-parse', 'HEAD').strip(), proof['commit'])
        self.assertEqual(self.git('rev-parse', 'HEAD^{tree}').strip(), proof['git_tree'])
        self.assertNotEqual(before['current_source_identity'], proof['current_source_identity'])
        self.assertEqual(hashlib.sha256(content.encode()).hexdigest(), proof['current_files']['.github/workflows/verify.yml']['sha256'])
        self.assertIn('data/record.json', proof['current_files'])
        self.assertFalse(proof['runtime_tests_certified'])

    def test_dirty_workflow_is_rejected(self):
        self.write('.github/workflows/verify.yml', 'uncommitted altered workflow\n')
        with self.assertRaisesRegex(ci_source.IntegrityError, 'Checkout differs'):
            ci_source.verify(self.root)

    def test_assume_unchanged_does_not_hide_altered_bytes(self):
        self.git('update-index', '--assume-unchanged', 'README.md')
        self.write('README.md', 'Hidden working change\n')
        with self.assertRaisesRegex(ci_source.IntegrityError, 'Checkout differs'):
            ci_source.verify(self.root)

    def test_missing_files_and_symlink_substitution_are_rejected(self):
        path = self.root / 'README.md'
        original = path.read_text()
        path.unlink()
        with self.assertRaisesRegex(ci_source.IntegrityError, 'Checkout differs'):
            ci_source.verify(self.root)
        outside = Path(self.temp.name) / 'outside'
        outside.write_text(original)
        path.symlink_to(outside)
        with self.assertRaisesRegex(ci_source.IntegrityError, 'Checkout differs'):
            ci_source.verify(self.root)

    def test_executable_mode_changes_are_rejected(self):
        (self.root / 'README.md').chmod(0o755)
        with self.assertRaisesRegex(ci_source.IntegrityError, 'Checkout mode differs'):
            ci_source.verify(self.root)

    def test_staged_source_is_not_silently_omitted(self):
        self.write('additional.txt', 'Uncommitted staged source\n')
        self.git('add', 'additional.txt')
        with self.assertRaisesRegex(ci_source.IntegrityError, 'Index differs'):
            ci_source.verify(self.root)

    def test_untracked_source_is_not_silently_omitted(self):
        self.write('unexpected-source.txt', 'Unexpected untracked source.\n')
        with self.assertRaisesRegex(ci_source.IntegrityError, 'Untracked source'):
            ci_source.verify(self.root)

    def test_a_shallow_clone_can_verify_its_complete_current_tree(self):
        shallow = Path(self.temp.name) / 'shallow'
        subprocess.run(['git', 'clone', '--quiet', '--depth=1', '--no-local', str(self.root), str(shallow)], check=True)
        proof = ci_source.verify(shallow)
        self.assertTrue(proof['passed'])
        self.assertEqual(ci_source.verify(self.root)['current_source_identity'], proof['current_source_identity'])


if __name__ == '__main__':
    unittest.main()
