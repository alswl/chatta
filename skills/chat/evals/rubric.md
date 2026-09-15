# chat skill scoring

Two layers to scoring a run of this skill:

- **L1, deterministic** -- decided by `check_skill.py` (the skill files),
  `check_quickstart.sh` (the quick start's eight scenarios) and
  `check_transcript.py` (one case's reply). No person or model involved. Any
  error or fail is a fail.
- **L2, behaviour quality** -- the five dimensions below, scored by a person or
  a grader model against the `expectations` in `evals.json`.

Case score = L1 all green (otherwise 0) x the sum of the five L2 dimensions
(10 points).

## What L1 covers

`check_skill.py`: every `chatta chat` command in the docs exists, no falling
back to the hidden flat aliases, referenced `references/` and `assets/` paths
are present, SKILL.md links every reference, the whole skill is in English,
inline code never breaks across lines, the frontmatter is complete, and
`quickstart.sh` is executable and syntactically valid.

`check_quickstart.sh`: cold start; rerunning leaves a healthy client alone;
recovery after the supervisor dies; another session's *healthy* client is
reused rather than seized; a custom port takes effect; the identity file
decides the nick; a missing ngircd is refused and handed to the user; a broken
client another live session left behind is taken over.

`check_transcript.py`: which commands ran this turn, whether the user was asked
anything before connecting, whether `inbox watch` went under Monitor, whether
the agent installed anything or seized someone else's client.

## L2 dimensions

### 1. Connecting without interrupting (0-3)

The core dimension: **one command, on the bus, no questions.**

| Score | Criterion |
|---|---|
| 3 | The first action is `quickstart.sh`; nick, role, channels and server all take their defaults; not one question asked; Monitor is up in the same turn |
| 2 | Connected and listening, but with detours -- a `session status` to decide whether to connect, or asking the user for a name |
| 1 | Connected but no Monitor, or it took several turns |
| 0 | Asked the user something before connecting, or never connected |

### 2. Choosing the space (0-2)

| Score | Criterion |
|---|---|
| 2 | Anything with a single recipient goes by DM; the channel budget stays at arrival plus departure; people are found with `channel members` and `/WHOIS`, not by shouting |
| 1 | Something that belonged in a DM went to a channel, or an extra channel message was spent finding someone |
| 0 | The work itself was carried in the channel |

### 3. Listening discipline (0-2)

| Score | Criterion |
|---|---|
| 2 | Monitor stays up all session; `session stop` only on an explicit request; if Monitor exits it is restarted and the gap picked up with `inbox read` |
| 1 | The gap went unread after an interruption, or it hesitated when it should not have |
| 0 | Stopped listening, or stopped the session, because its own task finished |

### 4. Commands and boundaries (0-2)

| Score | Criterion |
|---|---|
| 2 | Grouped commands throughout (`message send`, `inbox read`, ...); no hand-rolled IRC, no reading or writing `out`/`in` directly; missing binaries go to the user; a healthy client is never seized |
| 1 | Mixed in a flat alias, or read an `out` file for convenience |
| 0 | Installed a dependency itself, spoke IRC by hand, or `--takeover`'d a connection that was still working |

### 5. Relay quality (0-1)

| Score | Criterion |
|---|---|
| 1 | Relays to the user lead with the emoji for the kind (📨 new message, ❓ waiting on an answer, 📋 work assigned, 🔧 progress, ✅ done, ⚠️ blocked, 👋 arrived or left) and say in one line who said what |
| 0 | Pasted raw IRC lines, or failed to surface a message the user had to decide on |

**A 0 here is not fatal on its own, but a 0 on dimension 4 fails the case
regardless of the other scores.**

## How to score

- Each `expectations` entry is pass or fail, with evidence (quote the
  transcript).
- The suite's pass rate is the share of passing assertions.
- Regression baseline: **after any change to `SKILL.md`, `references/` or
  `quickstart.sh`, the pass rate must not drop below the previous version.** A
  drop means the change regressed something.

## Known high-risk failure modes

Watch these first:

1. **One question before connecting** -- "which nick?", "which server?" (cases 1, 2).
2. **Connected but not listening** -- Monitor forgotten, so being addressed goes unnoticed (cases 1, 7).
3. **Leaving when its own task ends** -- reading "I'm done here" as a signal to sign off (case 4).
4. **Shouting what should be a DM** -- especially questions and assignments (cases 3, 9).
5. **Seizing a healthy client** -- reaching for `--takeover` on "already on this path" (case 7).
6. **Installing dependencies** -- `brew install` when ngircd is missing, or improvising another IRC path (case 6).
7. **Dumping the raw log** -- pasting `inbox read` output at the user (case 8).
8. **Blind restarts** -- repeating `session start` instead of reading what `session status` says broke (case 10).
