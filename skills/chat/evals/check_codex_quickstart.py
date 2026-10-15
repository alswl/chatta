#!/usr/bin/env python3
"""Exercise startup decisions without a shared IRC bus or agent process."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SCRIPT = Path(__file__).resolve().parents[1] / "assets/quickstart.sh"

class QuickstartTests(unittest.TestCase):
    def run_case(self, args=(), **overrides):
        with tempfile.TemporaryDirectory(prefix="chatta-quickstart-") as folder:
            root = Path(folder)
            tools = root / "tools"
            tools.mkdir()
            log = root / "calls"
            candidate = root / "candidate with spaces"
            candidate.write_text(r"""#!/usr/bin/env python3
import json, os, sys
with open(os.environ['QS_LOG'], 'a') as out:
    out.write(json.dumps(sys.argv[1:]) + '\n')
a = sys.argv[1:]
if a == ['chat', 'codex', 'bind', '--help']:
    print('legacy' if os.environ.get('QS_OLD') else '--current --if-needed')
elif a[:3] == ['chat', 'codex', 'bind']:
    if os.environ.get('QS_FAIL_BIND'):
        print('thread unavailable; binding unchanged', file=sys.stderr)
        sys.exit(3)
    print('codex=enabled target_state=ready')
elif a == ['chat', 'codex', 'status']:
    print('codex=enabled subscribed_channels=#review')
""")
            candidate.chmod(0o700)
            for name, script in {
                'nc': '#!/bin/sh\nexit 0\n',
                'chatta': '#!/bin/sh\necho WRONG_BINARY >&2\nexit 97\n',
            }.items():
                path = tools / name
                path.write_text(script)
                path.chmod(0o700)
            env = dict(os.environ, PATH=str(tools)+os.pathsep+os.environ['PATH'],
                       CHATTA_BIN=str(candidate), CODEX_THREAD_ID='thread-current',
                       QS_LOG=str(log))
            env.update(overrides)
            result = subprocess.run(['sh', str(SCRIPT), *args], env=env,
                                    cwd=root, capture_output=True, text=True)
            calls = [json.loads(line) for line in log.read_text().splitlines()] if log.exists() else []
            return result, calls

    def test_codex_checkpoints_do_not_claim_monitor(self):
        result, _ = self.run_case()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertNotIn('Monitor:', result.stdout)
        self.assertIn('cannot wake', result.stdout)

    def test_explicit_binary_and_channel_bind(self):
        result, calls = self.run_case(('--codex', 'receiver', 'review', '--subscribe-channel', '#review'))
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn(['chat', 'codex', 'bind', '--current', '--if-needed', '--subscribe-channel', '#review'], calls)
        self.assertIn(['chat', 'channel', 'join', '#review'], calls)
        self.assertIn('subscribed_channels=#review', result.stdout)
        self.assertNotIn('WRONG_BINARY', result.stderr)
        self.assertNotIn('inbox watch', result.stdout)

    def test_bind_failure_is_not_success(self):
        result, _ = self.run_case(('--codex',), QS_FAIL_BIND='1')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('thread unavailable', result.stderr)
        self.assertNotIn('wake-up ready', result.stdout)

    def test_old_binary_rejected_before_session_mutation(self):
        result, calls = self.run_case(('--codex',), QS_OLD='1')
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse(any(call[:2] == ['chat', 'session'] for call in calls))
        self.assertIn('Codex', result.stderr)

    def test_current_identity_required_before_mutation(self):
        result, calls = self.run_case(('--codex',), CODEX_THREAD_ID='')
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(calls, [])
        self.assertIn('CODEX_THREAD_ID', result.stderr)

    def test_channel_option_requires_codex_mode(self):
        result, calls = self.run_case(('--subscribe-channel', '#review'))
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(calls, [])

    def test_non_codex_guidance_preserved(self):
        result, _ = self.run_case(CODEX_THREAD_ID='')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn('Monitor:', result.stdout)

if __name__ == '__main__':
    unittest.main(verbosity=2)
