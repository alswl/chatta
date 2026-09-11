#!/usr/bin/env python3
"""Run deterministic static checks for the chatta-admin skill."""

import json
import re
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent
SKILL = ROOT / "SKILL.md"
PLIST = ROOT / "assets" / "homebrew.ngircd.plist"
EVALS = ROOT / "evals" / "evals.json"
errors: list[str] = []


def fail(message: str) -> None:
    errors.append(message)


if not SKILL.exists():
    fail("SKILL.md is missing")
else:
    text = SKILL.read_text()
    if not text.startswith("---\n") or "name: chatta-admin" not in text:
        fail("SKILL.md has invalid frontmatter")
    if re.search(r"[\u3400-\u9fff]", text):
        fail("SKILL.md contains non-English CJK text")
    for path in re.findall(r"`(assets/[A-Za-z0-9._-]+)`", text):
        if not (ROOT / path).exists():
            fail(f"missing referenced asset: {path}")
    for term in ("misky", "pola", "/Users/", "/home/", "local.agent-chat"):
        if term.lower() in text.lower():
            fail(f"SKILL.md contains forbidden local identifier: {term}")

if not PLIST.exists():
    fail("homebrew.ngircd.plist is missing")
else:
    text = PLIST.read_text()
    for placeholder in ("__NGIRCD__", "__HOME__"):
        if placeholder not in text:
            fail(f"plist is missing placeholder: {placeholder}")
    if "127.0.0.1" in text or "/Users/" in text or "/home/" in text:
        fail("plist contains a machine-specific path or address")
    if "RunAtLoad" not in text or "KeepAlive" not in text:
        fail("plist is not persistent")

try:
    data = json.loads(EVALS.read_text())
    cases = data["evals"]
    if len({case["id"] for case in cases}) != len(cases):
        fail("evals.json contains duplicate ids")
    for case in cases:
        for key in ("prompt", "expected_output", "checks", "expectations"):
            if not case.get(key):
                fail(f"eval {case.get('id')} is missing {key}")
except (OSError, ValueError, KeyError) as exc:
    fail(f"invalid evals.json: {exc}")

if errors:
    for error in errors:
        print(f"[fail] {error}")
    sys.exit(1)

print("[pass] chatta-admin skill is English, self-contained, and sanitized")
