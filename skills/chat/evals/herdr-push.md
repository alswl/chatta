# Codex push conversation acceptance

## Reproduce

Start two fresh Herdr tabs with native remote Codex sessions on a compatible
Chatta build. Give each a separate client home and a distinct nick. Select
this repository's `skills/chat/SKILL.md`, not a globally installed copy.
Give only a natural-language task, with the skill path and participant names:

> First read <repo>/skills/chat/SKILL.md completely, then use $chat to take
> turns telling jokes in #agents for three rounds. You are Riva; your partner
> is Ribo; you go first.

The other participant receives the same task with the names reversed and
"wait for Riva to go first". The actual run used these prompts in Chinese;
their original wording is preserved in the raw captures. Do not supply Chatta
commands or follow-up coaching.

Inspect both terminal histories, not only message storage or RPC ACKs. Require
three incoming jokes on each side, six alternating contributions in total,
with the actual order A1, B1, A2, B2, A3, B3 in both terminal histories.
Greetings and readiness notices cannot trigger B1. Require one joke per CLI
message and per model turn, followed by turn completion before the next peer
contribution; six jokes alone do not prove correct turn-taking. Require
successful channel subscriptions and current-thread bindings, no inbox reads
after push, and no closing acknowledgement loop. Ending the exchange means
ending turns; it does not require disconnecting IRC.

## Observed run: 2026-10-16

The 1.1.3 evidence below is historical and is not a strict-alternation pass.
Independent acceptance subsequently observed B1, A1, A2, B2, A3, B3, including
two consecutive jokes in one turn. The revised 1.1.4 workflow must pass the
stricter ordering and turn-boundary checks above in fresh native sessions.

- Model: GPT-6-Luna, medium reasoning, native Codex CLI 0.162.1.
- Herdr tabs: `w4F:t11` / `w4F:t12`; panes: `w4F:p16` / `w4F:p17`.
- Agents: `chat-verify-a` (Riva), `chat-verify-b` (Ribo).
- Candidate binary built from this workspace, including its existing staged
  owner-lifecycle fix. This acceptance does not validate an older installed CLI.
- Raw terminal captures: `runs/2026-10-16-push/verify-a.txt` and
  `runs/2026-10-16-push/verify-b.txt` (local, gitignored).

Each item below appeared as `Chatta incoming message`, `kind=channel`,
`target=#agents` in the peer's actual terminal:

| Order | Sender | Receiving terminal event ID | Joke |
| --- | --- | --- | --- |
| A1 | Riva | `8befcd8e5589e80783d5bf66efc608ad` | Bugs feel safer hiding in dark mode. |
| B1 | Ribo | `60e9c653725d6260e4551d7b5cfacd71` | Java needs no sugar: code has sweet bugs. |
| A2 | Riva | `4a1ef8b28e83d16db5d65498cd3de875` | Open a window before the computer tells a joke. |
| B2 | Ribo | `b5eff875004adde9449c9a277f11bfc0` | Desktop files go into a procrastination folder. |
| A3 | Riva | `df2a2acd6a01a28ba30e03d2e117b8b9` | Escape keys help a keyboard handle pressure. |
| B3 | Ribo | `2630b384eaf4b93a075b329f21333fbd` | A database administrator always finds an index. |

Both terminal histories contain no executed `inbox read`/`inbox watch`
commands. Each peer sends its next joke through `message send --channel`,
then ends its turn. After B3 both report completion locally without another
outgoing completion message.

Startup was not flawless: Ribo initially omitted the supplied nick, recovered
from a nick collision, and missed Riva's first attempt. They recovered through
ordinary Chatta messages and restarted round one without operator coaching.
The six contributions above describe the completed exchange, not a claim that
only six messages or joke attempts were sent. Existing local hooks printed
`Hook failed`; they did not prevent the observed incoming messages.

Earlier trials exposed manual quickstart selection, inbox reads, three total
jokes instead of three per participant, and mixed installed/repository paths.
Those trials are not counted as passes. This run demonstrates the revised
workflow on Luna medium; it does not establish repeatability on Luna low.

## Strict ordering regression: skill 1.1.4

On 2026-10-16, two fresh native Codex Luna / medium tabs received only the
natural-language task above, explicitly selecting the repository skill rather
than any installed namesake. Names were Xira (starter) and Xibo. No Chatta
commands or follow-up coaching were supplied.

- A: `chat-strict-a`, tab `w4F:t1J`, pane `w4F:p1S`.
- B: `chat-strict-b`, tab `w4F:t1K`, pane `w4F:p1T`.
- Raw terminal histories: `runs/2026-10-16-order/strict-a.txt` and
  `runs/2026-10-16-order/strict-b.txt` (local, gitignored).

Actual terminal order, combining each side's outgoing CLI invocation and
incoming peer event, was A1, B1, A2, B2, A3, B3 on both sides:

| Contribution | Receiving terminal event ID | Joke subject |
| --- | --- | --- |
| A1 | `0f662c8a2aea0df25e5b9ab6ca651d85` | Bugs outside the window |
| B1 | `786b62fa31127766f43c7fad15015785` | All paths reach exit |
| A2 | `7e09cd53c8c17ca7a3cc6f26a3353b74` | AI spends ten minutes saving time |
| B2 | `c1441ca110956edcb29ed865bbe47001` | AI defines short before shortening |
| A3 | `5daf577503c2769f4330a38c605a3447` | Tomatoes blush at merge conflicts |
| B3 | `6bfa81e9b742a12c8ba6388bb588361c` | Programmers merge tree branches |

Before A1, B received A's HELLO, confirmed readiness without a joke, and
ended its turn. Each of A's three sends and B's three sends is followed by a
local final response and a native `Worked for` boundary before the next peer
contribution. No tool call contains two jokes. Neither history contains an
executed inbox command. Both finish locally after B3 without sending a closing
confirmation. Existing unrelated local hook errors remain visible in the raw
captures; they did not prevent this observed pass.

Earlier regression trials remain failures or invalid runs: Vira/Vibo achieved
the right message order but A sent three contributions during one native turn;
Wira/Wibo was invalid because B used the installed skill and manual inbox
workflow despite the requested repository path. These runs do not count as
passes. The strict Xira/Xibo run is one observed pass, not a reliability rate.
