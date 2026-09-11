# chat skill evals

One case set, one rubric, three deterministic checkers, one driver.

| File | What it does |
|---|---|
| `evals.json` | 10 behaviour cases: prompt, expected behaviour, machine-decided `checks`, human-scored `expectations` |
| `rubric.md` | L1/L2 scoring and the regression baseline |
| `check_skill.py` | L1a: static checks on the skill files (command existence, aliases, paths, language, layout) |
| `check_quickstart.sh` | L1c: eight scenarios against `assets/quickstart.sh`, no model needed |
| `check_transcript.py` | L1b: deterministic checks on one case's reply (what ran, what didn't) |
| `run.sh` | Driver: isolated bus -> one `claude -p` per case -> transcripts -> L1 |

## Running

```bash
python3 evals/check_skill.py     # static only, seconds; run it after editing the docs
evals/check_quickstart.sh        # quick start scenarios, ~1 min; run it after editing the script
evals/run.sh                     # everything: L1a + L1c + the behaviour cases
evals/run.sh 1 4 7               # only these cases
```

`run.sh` uses `CHATTA_CHAT_PORT=6767` and a `CHATTA_CHAT_HOME` under
`evals/runs/<timestamp>/`; `check_quickstart.sh` uses port 6768 and a directory
under `$TMPDIR`. Neither touches the real `127.0.0.1:6667` bus.

## Notes

- `~/.claude/skills/chat` usually points at a *deployed copy* elsewhere. `run.sh`
  compares it against this directory and refuses to run when they differ,
  printing the sync command -- otherwise the run measures the older version.
- The scenarios in `check_quickstart.sh` come from an ablation study of
  `quickstart.sh`. S1-S6 were the baseline; S7 (ngircd not on PATH) and S8
  (another live session left a broken client here) are the two coverage gaps
  that study exposed -- S8 caught a real bug the first time it ran.
- L1 only answers "did it do that". L2 answers "was it any good". Passing L1 is
  not passing.
