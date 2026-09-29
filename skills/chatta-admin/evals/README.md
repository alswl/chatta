# chatta-admin evaluations

This suite checks the server-side boundary of the `chatta-admin` skill. It
covers persistent launchd setup, configuration validation, loopback defaults,
bare-process migration, client/server ownership, and confirmation before
removing a service that affects every local chat session. It also covers
Linux CLI installation, custom server homes, preserving existing configuration,
and finding the active launchd stderr log.

All examples use synthetic wording. Do not run model evaluations against a
real `ngircd` process, the default client home, or a real launch agent.

From the `skills/chatta-admin/` directory, run the deterministic checks:

```bash
python3 evals/check_skill.py
```

This static checker is included in `make check-skill` and CI. The `checks`
names in `evals.json` are review criteria; this skill has no automated model
transcript grader yet. Score the expectations during a model run rather than
treating a static pass as a behavior benchmark.
