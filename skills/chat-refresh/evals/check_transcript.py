#!/usr/bin/env python3
"""Deterministic checks for flattened chat-refresh evaluation transcripts.

The input format contains one event per line:
  RAN: <tool or shell invocation>
  SAY: <assistant text shown to the user>

Usage: check_transcript.py <case-id> <transcript.txt>
"""

import json
import re
import sys
from pathlib import Path


EVALS = json.loads((Path(__file__).parent / "evals.json").read_text())


def ran(text: str) -> str:
    return "\n".join(line for line in text.splitlines() if line.startswith("RAN:"))


def said(text: str) -> str:
    return "\n".join(line for line in text.splitlines() if line.startswith("SAY:"))


def rule_ran_quickstart(text):
    return bool(re.search(r"quickstart\.sh\b", ran(text))), "quick start was not run"


def rule_single_delta_read(text):
    calls = re.findall(r"chatta chat inbox read(?:\s+--all)?", ran(text))
    return calls == ["chatta chat inbox read"], f"expected one delta read, got {calls}"


def rule_single_full_read(text):
    calls = re.findall(r"chatta chat inbox read(?:\s+--all)?", ran(text))
    return calls == ["chatta chat inbox read --all"], f"expected one full read, got {calls}"


def rule_no_inbox_read(text):
    return "chatta chat inbox read" not in ran(text), "read the inbox after startup failed"


def rule_no_monitor_or_watch(text):
    hit = re.search(r"\bMonitor\b|chatta chat inbox watch", ran(text))
    return not hit, "started Monitor or inbox watch in a Codex refresh"


def rule_no_legacy_wrapper(text):
    hit = re.search(r"agent_chat\.py|python3\s+[^\n]*(?:\shealth|\spoll|\sdm|\ssend)\b", ran(text))
    return not hit, "used the removed Python wrapper"


def rule_no_session_stop(text):
    return "chatta chat session stop" not in ran(text), "stopped the chat session during refresh"


def direct_to(text: str, nick: str) -> bool:
    return bool(re.search(rf"chatta chat message direct\s+{re.escape(nick)}\b", ran(text), re.I))


def rule_replied_to_atlas(text):
    return direct_to(text, "atlas"), "did not reply to Atlas by DM"


def rule_replied_to_birch(text):
    return direct_to(text, "birch"), "did not reply to Birch by DM"


def rule_did_not_reply_to_cedar(text):
    return not direct_to(text, "cedar"), "replied to a stale Cedar message"


def rule_no_direct_message(text):
    return "chatta chat message direct" not in ran(text), "sent a DM when no reply was required"


def rule_no_channel_work_message(text):
    return "chatta chat message send" not in ran(text), "sent one-recipient work traffic to a channel"


def rule_emoji_summary(text):
    return bool(re.search(r"[📨❓📋🔧✅⚠️👋]", said(text))), "summary has no message-kind emoji"


def rule_reported_refresh_failure(text):
    prose = said(text).lower()
    ok = ("failed" in prose or "could not" in prose or "unable" in prose) and re.search(r"transport|client|dependency", prose)
    return bool(ok), "did not clearly report the Chatta client failure and unreadable inbox"


def rule_no_install_attempt(text):
    return not re.search(r"(?:brew|apt|dnf|yum)\s+install", ran(text)), "installed a dependency"


def rule_no_raw_log_dump(text):
    raw_lines = re.findall(r"^SAY: \d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} ", text, re.M)
    return len(raw_lines) == 0, "pasted timestamped IRC log lines to the user"


RULES = {
    name.removeprefix("rule_"): fn
    for name, fn in list(globals().items())
    if name.startswith("rule_")
}


def main() -> int:
    if len(sys.argv) != 3:
        print(__doc__)
        return 2

    case_id = int(sys.argv[1])
    transcript = Path(sys.argv[2]).read_text(errors="replace")
    case = next((item for item in EVALS["evals"] if item["id"] == case_id), None)
    if case is None:
        print(f"[error] unknown case id: {case_id}")
        return 2

    failures = 0
    for check in case["checks"]:
        rule = RULES.get(check)
        if rule is None:
            print(f"[error] unknown rule: {check}")
            failures += 1
            continue
        passed, reason = rule(transcript)
        print(f"[{'pass' if passed else 'fail'}] {check}" + ("" if passed else f": {reason}"))
        failures += int(not passed)

    print(f"case {case_id}: checks={len(case['checks'])} failures={failures}")
    return int(failures > 0)


if __name__ == "__main__":
    sys.exit(main())
