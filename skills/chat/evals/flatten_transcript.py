#!/usr/bin/env python3
"""evals/flatten_transcript.py -- turn a stream-json run into a checkable text.

`claude -p` prints only prose, so a reply that merely names a command is
indistinguishable from one that ran it. With --output-format stream-json the
events carry the tool calls themselves, and this flattens them into two clearly
separated parts:

    SAY: <a line the assistant said>
    RAN: <tool> <command or input summary>

in event order, so check_transcript.py can match "did it run X" against the
RAN lines only, "how did it report" against the SAY lines, and "did it ask
before acting" against the order of the two.

Usage: python3 evals/flatten_transcript.py <case.json>
"""
import json
import sys
from pathlib import Path


def tool_line(name: str, inp: dict) -> str:
    if name == "Bash":
        return f"RAN: Bash {inp.get('command', '')}"
    if name == "Monitor":
        return f"RAN: Monitor {inp.get('command', '')}"
    if name in ("Read", "Glob", "Grep"):
        target = inp.get("file_path") or inp.get("pattern") or ""
        return f"RAN: {name} {target}"
    if name == "Skill":
        return f"RAN: Skill {inp.get('skill', '')} {inp.get('args', '')}".rstrip()
    return f"RAN: {name} {json.dumps(inp, ensure_ascii=False)[:200]}"


def main() -> int:
    if len(sys.argv) != 2:
        print(__doc__)
        return 2
    out: list[str] = []
    saw_text = False
    for raw in Path(sys.argv[1]).read_text(errors="replace").split("\n"):
        raw = raw.strip()
        if not raw.startswith("{"):
            if raw:                      # a plain-text line: a crash or a refusal
                out.append(f"SAY: {raw}")
            continue
        try:
            event = json.loads(raw)
        except json.JSONDecodeError:
            continue
        if event.get("type") == "assistant":
            for block in event.get("message", {}).get("content", []):
                if block.get("type") == "tool_use":
                    out.append(tool_line(block.get("name", "?"), block.get("input", {})))
                elif block.get("type") == "text" and block.get("text", "").strip():
                    saw_text = True
                    for line in block["text"].strip().split("\n"):
                        out.append(f"SAY: {line}")
        elif event.get("type") == "result" and event.get("result") and not saw_text:
            for line in str(event["result"]).strip().split("\n"):
                out.append(f"SAY: {line}")
    print("\n".join(out))
    return 0


if __name__ == "__main__":
    sys.exit(main())
