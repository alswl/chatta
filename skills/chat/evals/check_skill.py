#!/usr/bin/env python3
"""evals/check_skill.py -- deterministic static checks on the chat skill (L1a).

Only mechanically decidable rules: every documented command exists, no falling
back to the flat aliases, referenced relative paths are present, the language
is consistent, and no inline code span is broken across lines. Whether the
prose reads well is left to evals/rubric.md.

Usage:
    python3 evals/check_skill.py            # run from skills/chat/
Exit status: 0 = no errors (warnings possible); 1 = at least one error.
"""
import json
import re
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
DOCS = [ROOT / "SKILL.md"] + sorted((ROOT / "references").glob("*.md"))
FLAT_ALIASES = "start health join part send dm poll watch who stop clients gc".split()

errors: list[str] = []
warnings: list[str] = []


def err(msg: str) -> None:
    errors.append(msg)


def warn(msg: str) -> None:
    warnings.append(msg)


def outside_fences(text: str):
    """Yield (line_no, line) for lines outside ``` fenced blocks and frontmatter."""
    lines = text.split("\n")
    start = 0
    if lines and lines[0] == "---":
        start = lines.index("---", 1) + 1
    fenced = False
    for n, line in enumerate(lines[start:], start + 1):
        if line.lstrip().startswith("```"):
            fenced = not fenced
            continue
        if not fenced:
            yield n, line


# ---- 1. Every documented command must exist ---------------------------------

have_chatta = subprocess.run(["which", "chatta"], capture_output=True).returncode == 0
seen_cmds: set[tuple[str, str]] = set()
for doc in DOCS:
    for m in re.finditer(r"chatta chat ([a-z]+) ([a-z]+)", doc.read_text()):
        seen_cmds.add(m.groups())
seen_flags: set[tuple[str, str, str]] = set()
for doc in DOCS:
    for m in re.finditer(r"chatta chat ([a-z]+) ([a-z]+)([^\n`]*)", doc.read_text()):
        # Stop at the first quote or substitution: past that the flags belong
        # to a nested command, not to this one.
        tail = re.split(r'["$]', m.group(3))[0]
        for flag in re.findall(r"--[a-z][a-z-]+", tail):
            seen_flags.add((m.group(1), m.group(2), flag))
if have_chatta:
    for group, sub in sorted(seen_cmds):
        r = subprocess.run(["chatta", "chat", group, sub, "--help"], capture_output=True)
        if r.returncode != 0:
            err(f"documented command does not exist: chatta chat {group} {sub}")
    # A flag the docs give a command but nobody registered is what --deep was.
    for group, sub, flag in sorted(seen_flags):
        r = subprocess.run(["chatta", "chat", group, sub, "--help"], capture_output=True)
        if r.returncode == 0 and flag not in r.stdout.decode(errors="replace"):
            err(f"documented flag does not exist: chatta chat {group} {sub} {flag}")
else:
    warn("chatta executable not found; skipped the command and flag existence checks")

# ---- 2. No falling back to the hidden flat aliases --------------------------

alias_re = re.compile(r"chatta chat (" + "|".join(FLAT_ALIASES) + r")\b")
bare_re = re.compile(r"`(" + "|".join(FLAT_ALIASES) + r")`")
for doc in DOCS:
    text = doc.read_text()
    for m in alias_re.finditer(text):
        err(f"{doc.name}: flat alias chatta chat {m.group(1)}; use the grouped command")
    for n, line in outside_fences(text):
        for m in bare_re.finditer(line):
            err(f"{doc.name}:{n}: bare command name `{m.group(1)}` in prose; write `<group> <sub>`")

# ---- 3. Referenced paths exist, and SKILL.md mentions every reference -------

skill_text = (ROOT / "SKILL.md").read_text()
for doc in DOCS:
    for m in re.finditer(r"`((?:references|assets)/[A-Za-z0-9._-]+)`", doc.read_text()):
        if not (ROOT / m.group(1)).exists():
            err(f"{doc.name}: references a path that does not exist: {m.group(1)}")
for ref in (ROOT / "references").glob("*.md"):
    if f"references/{ref.name}" not in skill_text:
        err(f"SKILL.md never mentions references/{ref.name}; an unlinked reference is never read")

# ---- 4. One language: docs, examples and eval files are English -------------

# ii is retired; chatta speaks IRC in-process now. These patterns stay as a
# regression guard against docs drifting back toward operating or explaining
# an external transport client's implementation details.
transport_patterns = [
    (r"(?im)^\s*(?:ii)(?:\s|$)", "direct ii command"),
    (r"(?i)ii[^\n]*(?:directory|manual|command|file format|layout|process)", "ii implementation detail"),
    (r"(?i)(?:cat|tail|head|echo|printf|pgrep|pkill)[^\n]*(?:/in\b|/out\b|FIFO|ii)", "direct transport file/process operation"),
    (r"(?i)references/ii-manual\.md", "retired ii manual reference"),
]
for doc in DOCS + [ROOT / "assets" / "quickstart.sh"]:
    text = doc.read_text(errors="replace")
    for pattern, label in transport_patterns:
        if re.search(pattern, text):
            err(f"{doc.name}: {label} is outside the Chatta CLI boundary")

cjk_re = re.compile("[\u3000-\u303f\u3400-\u4dbf\u4e00-\u9fff\uff00-\uffef]")
for doc in DOCS + [ROOT / "assets" / "quickstart.sh"] + sorted((ROOT / "evals").glob("*")):
    if doc.is_dir():
        continue
    for n, line in enumerate(doc.read_text(errors="replace").split("\n"), 1):
        if cjk_re.search(line):
            err(f"{doc.name}:{n}: non-English text; this skill is written in English throughout")

# ---- 5. Layout: inline code stays on one line, prose lines stay narrow ------

for doc in DOCS:
    for n, line in outside_fences(doc.read_text()):
        if line.count("`") % 2 == 1:
            err(f"{doc.name}:{n}: inline code span broken across lines -- {line.strip()[:50]}")
        if len(line) > 82 and not line.lstrip().startswith("|"):
            warn(f"{doc.name}:{n}: prose line is {len(line)} characters, over 82")

# ---- 6. Frontmatter and the quick start -------------------------------------

fm = re.match(r"---\n(.*?)\n---\n", skill_text, re.S)
if not fm:
    err("SKILL.md has no frontmatter")
else:
    for key in ("name:", "description:", "allowed-tools:"):
        if key not in fm.group(1):
            err(f"frontmatter is missing {key}")
    desc = fm.group(1).split("description:", 1)[-1]
    if len(desc) > 1400:
        warn(f"frontmatter description is {len(desc)} characters; trigger matching needs far less")

qs = ROOT / "assets" / "quickstart.sh"
if not qs.exists():
    err("assets/quickstart.sh is missing")
else:
    if not qs.stat().st_mode & 0o111:
        err("assets/quickstart.sh is not executable")
    if subprocess.run(["sh", "-n", str(qs)], capture_output=True).returncode != 0:
        err("assets/quickstart.sh fails its syntax check")
    if "quickstart.sh" not in skill_text:
        err("SKILL.md never mentions assets/quickstart.sh")

cq = Path(__file__).parent / "check_quickstart.sh"
if not cq.exists():
    err("evals/check_quickstart.sh is missing; the quick start has no scenario regression")
elif subprocess.run(["bash", "-n", str(cq)], capture_output=True).returncode != 0:
    err("evals/check_quickstart.sh fails its syntax check")

# ---- 7. evals.json itself ---------------------------------------------------

ev = Path(__file__).parent / "evals.json"
if ev.exists():
    data = json.loads(ev.read_text())
    ids = [c["id"] for c in data["evals"]]
    if len(ids) != len(set(ids)):
        err("evals.json has duplicate case ids")
    for case in data["evals"]:
        for key in ("prompt", "expected_output", "checks", "expectations"):
            if not case.get(key):
                err(f"evals.json case {case['id']} is missing {key}")

for m in errors:
    print(f"  [error] {m}")
for m in warnings:
    print(f"  [warn ] {m}")
print()
print(f"check_skill: error={len(errors)} warn={len(warnings)}")
sys.exit(1 if errors else 0)
