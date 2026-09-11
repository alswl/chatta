# chat-refresh evaluations

This suite evaluates the Codex-specific manual inbox checkpoint without touching a real chat bus.

## Coverage

- one delta read per ordinary refresh;
- no Monitor, `inbox watch`, or polling loop in Codex;
- no calls to the removed Python wrapper;
- stale local logs after a fresh session are ignored;
- directed tasks and questions receive substantive DMs;
- messages for other agents and goodbye broadcasts are skipped;
- startup failures are reported as unreadable inboxes, not empty inboxes;
- explicit history requests use exactly one `inbox read --all`;
- the chat session remains running after refresh.

## Privacy

All prompts, transcripts, names, paths, dates, and message content are synthetic. `check_privacy.py`
scans the complete skill tree and rejects CJK text, common personal paths, email addresses, session
URLs, and locally supplied private terms. Do not copy real IRC output into fixtures; replace it with a
minimal fictional equivalent.

Supply machine- or organization-specific identifiers at runtime so they never enter the repository:

```bash
CHAT_REFRESH_PRIVACY_DENYLIST='private-user,private-repository' python3 evals/check_privacy.py
```

## Run deterministic checks

```bash
./evals/test_checks.sh
```

`check_transcript.py` consumes flattened transcripts containing `RAN:` and `SAY:` lines. The included
fixtures prove that every declared rule is executable and that key negative cases are rejected. For a
model evaluation, run each prompt on an isolated bus, flatten the tool calls and assistant response into
that format, then invoke:

```bash
python3 evals/check_transcript.py <case-id> <transcript.txt>
```

Use `rubric.md` for the qualitative score. Never point model evaluations at the default client home or
the user's live IRC server.
