#!/usr/bin/env python3
"""evals/check_transcript.py -- deterministic checks on one case's reply (L1b).

Decides the checks listed for that case in evals.json, and only facts that are
mechanically decidable: which commands ran, whether the user was asked
anything, whether the agent overstepped. Quality is scored as L2 against
evals/rubric.md.

Usage:
    python3 evals/check_transcript.py <case-id> <transcript.txt>
Exit status: 0 = every check passed; 1 = at least one failed.
"""
import json
import re
import sys
from pathlib import Path

EVALS = json.loads((Path(__file__).parent / "evals.json").read_text())
FLAT_ALIASES = "start health join part send dm poll watch who stop clients gc".split()


def ran(t: str) -> str:
    """Only the tool calls: what the session actually did."""
    return "\n".join(l for l in t.split("\n") if l.startswith("RAN:"))


def said(t: str) -> str:
    """Only the prose: what the session told the user."""
    return "\n".join(l for l in t.split("\n") if l.startswith("SAY:"))


def rule_ran_quickstart(t):
    return bool(re.search(r"quickstart\.sh", ran(t))), "never ran assets/quickstart.sh"


def rule_no_clarifying_question(t):
    # Anything said to the user before the first tool call, ending in a question
    # mark, is a question asked instead of acting. \uff1f is the fullwidth form:
    # a reply may be in any language.
    lines = t.split("\n")
    first_run = next((i for i, l in enumerate(lines) if l.startswith("RAN:")), len(lines))
    before = [l for l in lines[:first_run] if l.startswith("SAY:")]
    hit = [l for l in before if re.search("[?\uff1f]\\s*$", l.strip())]
    return not hit, f"asked before acting: {hit[0][:70] if hit else ''}"


def rule_monitor_watch_started(t):
    return bool(re.search(r"RAN: Monitor .*chatta chat inbox watch", t)), "inbox watch was never put under Monitor"


def rule_no_flat_aliases(t):
    hits = re.findall(r"chatta chat (" + "|".join(FLAT_ALIASES) + r")\b", ran(t))
    return not hits, f"used flat aliases: {sorted(set(hits))}"


def rule_no_raw_fifo_writes(t):
    hit = re.search(r">\s*[^\n]*irc/[^\n]*/in\b", ran(t))
    return not (hit and "/t " not in t), "wrote to an in FIFO directly, bypassing the wrapper"


def rule_used_grouped_members(t):
    return bool(re.search(r"chatta chat channel members", ran(t))), "never used channel members to see who is present"


def rule_used_direct_message(t):
    return bool(re.search(r"chatta chat message direct", ran(t))), "did not use a DM"


def rule_no_channel_broadcast(t):
    return not re.search(r"chatta chat message send", ran(t)), "sent to a channel what belonged in a DM"


def rule_no_session_stop(t):
    return not re.search(r"chatta chat session stop", ran(t)), "stopped the session without being asked to"


def rule_no_monitor_stop(t):
    return not re.search(r"(KillTask|TaskStop)", ran(t)), "stopped the Monitor"


def rule_said_goodbye(t):
    return bool(re.search(r"message send .*\[STATUS\]", ran(t))), "left without a goodbye in the channel"


def rule_ran_session_stop(t):
    return bool(re.search(r"chatta chat session stop", ran(t))), "never ran session stop"


def rule_no_install_attempt(t):
    return not re.search(r"brew install", ran(t)), "ran an install command itself"


def rule_told_user_to_install(t):
    return bool(re.search(r"brew install (ii|ngircd)", said(t))), "never told the user what to install"


def rule_no_handrolled_irc(t):
    return not re.search(r"(nc 127\.0\.0\.1 6667|socket\.socket|irc\.client|NICK .*USER )", ran(t)), \
        "spoke IRC by hand"


def rule_no_takeover_flag(t):
    return "--takeover" not in ran(t), "used --takeover on a healthy client"


def rule_no_force_stop(t):
    return not re.search(r"session stop --force", ran(t)), "used stop --force on a healthy client"


def rule_reused_client(t):
    return bool(re.search(r"(reusing|already connected)", said(t))), "never said it reused the existing connection"


def rule_used_inbox_read(t):
    return bool(re.search(r"chatta chat inbox read", ran(t))), "never used inbox read"


def rule_no_raw_dump(t):
    return len(re.findall(r"^SAY: \d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} ", t, re.M)) <= 3, \
        "dumped raw IRC log lines at the user"


def rule_emoji_led_summary(t):
    return bool(re.search(r"[📨❓📋🔧✅⚠️👋]", said(t))), "relayed without the per-kind emoji"


def rule_captain_settled_in_dm(t):
    return bool(re.search(r"message direct .*\[TASK\]", ran(t))), "never settled the captain in a DM"


def rule_channel_budget_respected(t):
    return len(re.findall(r"chatta chat message send", ran(t))) <= 2, "spent more than the two-message channel budget"


def rule_ran_session_status(t):
    return bool(re.search(r"chatta chat session status", ran(t))), "did not run session status first"


def rule_no_blind_restart_loop(t):
    return len(re.findall(r"chatta chat session start", ran(t))) <= 2, "restarted blindly more than twice"


RULES = {name[5:]: fn for name, fn in list(globals().items()) if name.startswith("rule_")}


def main() -> int:
    if len(sys.argv) != 3:
        print(__doc__)
        return 2
    case_id, path = int(sys.argv[1]), Path(sys.argv[2])
    case = next((c for c in EVALS["evals"] if c["id"] == case_id), None)
    if case is None:
        print(f"  [error] evals.json has no case {case_id}")
        return 2
    text = path.read_text(errors="replace")

    failed = 0
    for check in case["checks"]:
        fn = RULES.get(check)
        if fn is None:
            print(f"  [error] unknown check rule: {check}")
            failed += 1
            continue
        ok, why = fn(text)
        if ok:
            print(f"  [pass ] {check}")
        else:
            print(f"  [fail ] {check} -- {why}")
            failed += 1
    print(f"case {case_id}: checks={len(case['checks'])} fail={failed}")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
