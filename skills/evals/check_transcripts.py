#!/usr/bin/env python3
"""Fixtures measure grader regressions, not model performance.

Transcript commands are never executed.
"""

import importlib.util
import json
import sys
from pathlib import Path

# Grader bytecode embeds local paths rejected by the privacy scan.
sys.dont_write_bytecode = True

ROOT = Path(__file__).resolve().parent.parent


def main():
    modules = {}
    count = 0
    for skill in ('chat', 'chat-refresh'):
        directory = ROOT / skill / 'evals'
        spec = importlib.util.spec_from_file_location(skill, directory / 'check_transcript.py')
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        modules[skill] = module
        cases = module.EVALS['evals']
        assert len({case['id'] for case in cases}) == len(cases), f'{skill}: duplicate ids'
        for case in cases:
            for key in ('prompt', 'expected_output', 'checks', 'expectations'):
                assert case.get(key), f"{skill}/{case['id']}: missing {key}"
            for check in case['checks']:
                assert check in module.RULES, f'{skill}: unknown check {check}'
            fixture = directory / 'fixtures' / f"case-{case['id']}.txt"
            if fixture.exists():
                transcript = fixture.read_text()
                for check in case['checks']:
                    passed, reason = module.RULES[check](transcript)
                    assert passed, f'{fixture}: {check}: {reason}'
                    count += 1

    negatives = json.loads((ROOT / 'evals' / 'negative-transcripts.json').read_text())
    for case in negatives:
        skill, rule = case['skill'], case['rule']
        module = modules[skill]
        if 'case_id' in case:
            fixture = ROOT / skill / 'evals' / 'fixtures' / f"case-{case['case_id']}.txt"
            transcript = fixture.read_text()
            assert module.RULES[rule](transcript)[0], f'{rule}: baseline must pass'
            if 'replace' in case:
                old, new = case['replace']
                assert old in transcript, f'{rule}: replacement did not match'
                transcript = transcript.replace(old, new)
            if 'append' in case:
                transcript += '\n' + case['append'] + '\n'
        else:
            transcript = case['transcript']
        assert not module.RULES[rule](transcript)[0], f'{skill}/{rule}: accepted regression'

    print(f'[pass] {count} fixture assertions; {len(negatives)} rejected regressions')


if __name__ == '__main__':
    main()
