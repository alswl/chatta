# chatta-admin evaluations

This suite checks the server-side boundary of the `chatta-admin` skill. It
covers persistent launchd setup, configuration validation, loopback defaults,
bare-process migration, client/server ownership, and confirmation before
removing a service that affects every local chat session.

All examples use synthetic wording. Do not run model evaluations against a
real `ngircd` process, the default client home, or a real launch agent.

From the `skills/chatta-admin/` directory, run the deterministic checks:

```bash
python3 evals/check_skill.py
```
