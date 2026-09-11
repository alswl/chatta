#!/usr/bin/env python3
"""Reject common forms of personal or environment-specific data in eval assets."""

import os
import re
import sys
from pathlib import Path


ROOT = Path(__file__).parent.parent
PATTERNS = {
    "macOS home directory": re.compile(r"/Users/[^/\s]+/"),
    "Linux home directory": re.compile(r"/home/[^/\s]+/"),
    "email address": re.compile(r"\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b", re.I),
    "Claude session URL": re.compile(r"https://claude\.ai/code/session_[A-Za-z0-9]+"),
    "Codex session identifier": re.compile(r"\b(?:session|thread)[_-][A-Za-z0-9]{12,}\b", re.I),
    "non-English CJK text": re.compile(r"[\u3400-\u9fff]"),
}

private_terms = [
    term.strip()
    for term in os.environ.get("CHAT_REFRESH_PRIVACY_DENYLIST", "").split(",")
    if term.strip()
]
if private_terms:
    PATTERNS["private denylist term"] = re.compile(
        "|".join(re.escape(term) for term in private_terms), re.I
    )


def main() -> int:
    failures = 0
    for path in sorted(ROOT.rglob("*")):
        if not path.is_file() or path.resolve() == Path(__file__).resolve():
            continue
        text = path.read_text(errors="replace")
        for label, pattern in PATTERNS.items():
            match = pattern.search(text)
            if match:
                print(f"[fail] {path.relative_to(ROOT)}: {label}: {match.group(0)!r}")
                failures += 1
    if failures == 0:
        print("[pass] skill tree is English and contains no recognized personal or local identifiers")
    return int(failures > 0)


if __name__ == "__main__":
    sys.exit(main())
