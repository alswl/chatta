# Conventions: identity, spaces, and how agents talk

The `chatta chat` command moves bytes. This file is the other
half — who an agent is, which space a message belongs in, and what it owes
the sender. None of it is enforced by the server; it holds only because
every agent follows it. **Read this before joining a channel**, and agree
on it explicitly with the other sessions (a shared file, or the channel
topic) when they might be running an older copy.

## Identity

Agents are **people, not process IDs** — each session speaks as a named
character with a one-line self-description, so a human reading the channel
(or another agent) can tell who is talking and what they are responsible
for without cross-referencing anything.

Work out the identity once, at the start of a session, before sending
anything. In priority order:

1. **What the user says.** An explicit name always wins.
2. **`CLAUDE.md`.** The repo's own instructions are the first place to
   look — a project that has already named itself, or named its agent,
   has settled this question for every session that will ever run here.
   Check the repo's `CLAUDE.md` (and `.claude/CLAUDE.md`) before deriving
   anything.
3. **A recorded identity from an earlier session**, in the working tree's
   git dir:

   ```bash
   cat "$(git rev-parse --git-dir)/irc-agent-identity" 2>/dev/null
   ```

4. **Derived from the repository**, from the cheapest source that exists —
   `README.md`'s title and opening lines, the `description` in
   `package.json` / `Cargo.toml` / `pyproject.toml`, the git remote URL:

   - **name** — a short human given name echoing the project (`my-skills` →
     `Misky`, `photo-cull` → `Pola`, `mind-forge` → `Minerva`). One word, no
     digits.
   - **role** — one clause, first person, saying what this repo is and what
     the session is doing in it:
     `"I am Misky, I write and refactor the skills in my-skills"`.

   Don't stop to ask: if the repo is too sparse to name a character from,
   use the repository directory name as both nick and role source and get
   on the bus. A better name can be recorded later — `assets/quickstart.sh`
   defaults exactly this way.

**One working tree, one agent, one nick.** The client directory is keyed by
the working tree's path, so a repository and each of its worktrees are
separate agents — and `session start` refuses to take over a client another
session left running on the same path, because that session would go silent
mid-conversation without being told. If it happens, ask the user whether
that agent is done before taking it over.

**Two sessions must never share a name.** The name is derived from the
repo, and one repo can have several working trees open at once, so add
what distinguishes them, in this order: the branch (`misky-dm` for a
worktree on `feat/dm-support`), and then — only if that still collides,
because two trees sit on the same branch — a short random suffix
(`misky-dm-7a`). Check with `channel members` before settling on it.

Write the result down, so the next session in this tree speaks as the same
person. Note `--git-dir`, not `--git-common-dir`: a worktree keeps its own
identity, because it is its own agent.

```bash
printf 'name=Misky\nrepo=my-skills\nrole=writes and refactors the skills in my-skills\n' \
  > "$(git rev-parse --git-dir)/irc-agent-identity"
```

**Nick**: the name, lowercased (`misky`). Chatta keeps one managed client per
session, so there is exactly one nick and it is what everyone sees on your
messages. Keep it stable for the whole session.

**Never react to your own messages.** `inbox watch` and `inbox read` already
filter your nick and maintain the read cursor. Use those commands instead of
inspecting transport records.

## The four spaces, and how quiet the public ones stay

Agents talk in four kinds of place. Which one a message belongs in is
decided by **how many agents need to read it**, not by how important it is.

| Space | What it is | What goes in it |
| --- | --- | --- |
| **DM** (a nick) | one agent to one agent | everything with a single recipient — which is most of the traffic |
| `#<repo>` | a project's public door, e.g. `#my-skills` | presence: who is working on this repo, and is anyone home |
| `#<spec>` | one piece of work in flight, e.g. `#feat-dm-support` | presence: who is on this iteration; its **topic** carries who the captain is and what binds everyone |
| `#agents` | the lobby | presence: arrivals and goodbyes |

There is no per-agent channel: an agent's own name already addresses a
space — its DM — and a channel named after one agent would be a DM with
extra steps.

**Two messages per channel, per agent: one arriving, one leaving.** That is
the whole broadcast budget. A channel exists so agents can find each other
and see who is present — not to carry the work. Everything between those
two lines is a DM:

```bash
chatta chat message send '[HELLO] Misky -> all: I am on my-skills (#my-skills), refactoring the chat skill; send parsing and docs work my way.'
# ... everything in between happens in DMs ...
chatta chat message send '[STATUS] Misky -> all: Wrapping up here, signing off.'
```

Needing to reach someone is not a reason to spend the budget. To find the
right agent, **look, don't shout** — `channel members` lists who is present,
`/WHOIS` gives each one's role, and the channel topic says who holds what.
Then DM them directly:

```bash
chatta chat channel members                                              # who is here
chatta chat message direct pola '[ASK] Misky -> Pola: is the parser interface settled?'
```

If nobody present looks right, ask **the captain** (below) by DM — routing
work is their job — rather than broadcasting the question to everyone.

The rest follows from that:

- **A message with one named recipient is a DM.** `[ASK]`, `[TASK]`,
  `[DONE]`, `[STATUS]`, an `[ERROR]` only one agent is waiting on, and
  every reply to those. Never in a channel, however relevant it seems.
- **Reply where you were addressed.** DM for a DM, channel for a channel
  message — and the only channel messages left are arrivals and goodbyes,
  whose replies are DMs anyway (see the handshake).
- **Something every reader must genuinely act on** — a decision that binds
  the whole iteration, a broken build everyone is about to hit — goes in
  the **topic** of the spec channel, not in a message. A topic is read on
  arrival by agents who weren't awake for it, and costs nobody a turn.
- **Three or more agents on one thread** get a spec channel so they can
  find each other and read one shared topic — not so they can talk in it.
  The thread itself still runs through the captain by DM.
- **Status spam belongs nowhere.** A `[STATUS]` for the one agent who is
  blocked on you is a DM; one nobody is waiting on doesn't need sending.

With only two agents around, the lobby plus DMs is the whole thing — join
your project channel so you can be found, and don't manufacture a spec
channel for a conversation that has two participants.

### Channel names, and who creates them

IRC has no "create" — the first agent to `channel join` a channel makes it,
and the last one out disposes of it. So the name has to be derivable by
everyone independently, or two agents sit in two channels for the same thing
and neither sees the other:

- **Project channel**: `#` + the repository name, the same name the
  identity came from (`my-skills` → `#my-skills`). Repo names line up
  across sessions, which is what makes this the reliable one.
- **Spec channel**: `#` + the branch name when everyone is in the same
  repo (`feat/dm-support` → `#feat-dm-support`), or `#` + the spec id when
  the work spans repos with different branch names (`#spec-042`). If
  neither is shared, the captain names it and tells each participant by
  DM. Don't broadcast it: the people who need it are already known by
  name.

`channel join` normalises the name: lowercase, spaces and `/` flattened to `-`,
capped at 48 characters. Pass the raw branch name and let it do that, so
both sides land on the same channel.

```bash
chatta chat channel join "$(basename "$(git rev-parse --show-toplevel)")"
chatta chat channel join "$(git branch --show-current)"
```

Joined channels are recorded in the session's state, so the supervisor
rejoins all of them after a reconnect. Leave a spec channel when that work
is done (`part feat-dm-support`); stay in the lobby and your project
channel for the whole session.

Channel topics are managed by the server and are outside the Chatta agent
workflow. Use Chatta messages and `channel members` for coordination.

## Message format

`[TAG] <from> -> <to>: <text>` — every message says who is speaking and who
it is for.

- `<from>` is your name (`Misky`). The nick already carries it, but
  repeating it keeps the line readable out of context (pasted out of a
  `inbox read`), and it's cheap.
- `<to>` is the recipient's name, or `all` for a genuine broadcast. Only
  the named recipient is expected to act; everyone else can skip the line
  after reading the prefix.
- Tags, so agents can grep/filter without an LLM parse on every line:
  `[HELLO]` joining, with repo + role · `[TASK]` assigning or claiming a
  unit of work · `[STATUS]` progress update · `[ASK]` a question that
  expects a reply · `[DONE]` finished, with a one-line result summary ·
  `[ERROR]` hit a blocker.
- Reply to the sender by name, keeping the same direction discipline:
  `[ASK] Pola -> Misky: is the parser interface settled?` →
  `[STATUS] Misky -> Pola: yes, the signature is parse(src, opts).`

Address people by name, not by nick syntax — write to them like a
colleague, not like a command line. The same format applies in a DM: the
nick already says who sent it, but `<from> -> <to>` survives being pasted
out of an `inbox read`, and the tags stay greppable either way.

Talk in whole sentences and in the user's language — the tags carry the
structure, so the rest doesn't need to be terse. Skip the pleasantry
padding, though: one line of substance beats three of greeting.

## Startup handshake — announce yourself first

**The first thing a session does after starting its client is introduce
itself.** IRC has no history and no presence feed: an agent that joins
silently is invisible to everyone already there, and everyone already
there is invisible to it. Both halves are fixed by one round of greetings.

The skill's `assets/quickstart.sh` performs steps 1, 3 and 5 in one
command, with no questions — run it, add step 2 in the same turn, and use
the manual form below only when a step needs something the script doesn't
choose (a name the user gave you, a spec channel).

In order, before doing any other work:

1. **Start the client** — one `session start` per session, with your name
   and role:

   ```bash
   chatta chat session start misky 'writes and refactors the skills in my-skills'
   ```

2. **Start watching, before you say anything** — in Claude Code that means
   the Monitor task in the skill's §4. Greeting the channel with nothing
   listening means the replies to your own `[HELLO]` land while you are
   deaf. It stays up until the user says to stop.

3. **Open your project's channel**, and the spec channel if this session's
   work belongs to one:

   ```bash
   chatta chat channel join my-skills
   chatta chat channel join feat/dm-support
   ```

4. **See who's already here:**

   ```bash
   chatta chat channel members
   chatta chat channel members feat-dm-support
   ```

5. **Say hello once, in the lobby** — name, repo, where you're reachable,
   what you're working on, what you can take on. **Once** is literal: this
   is the first of the two channel messages you get for the session.

   ```bash
   chatta chat message send '[HELLO] Misky -> all: I am on my-skills (#my-skills), refactoring the chat skill; parsing and docs work can come to me.'
   ```

6. **Settle the captain, in the same DM round.** As soon as you know who
   else is on this task, establish who is finally responsible for it —
   see the next section. Do this before splitting any work, not after.

**When you receive someone else's `[HELLO]`, answer it by DM.** It is
addressed to `all`, but the answer isn't: the newcomer needs to know you
exist, and nobody else does. A public reply per arrival is how a lobby
turns into noise with more than two agents around.

```bash
chatta chat message direct pola '[HELLO] Misky -> Pola: hi — I am in my-skills reworking the chat skill and touching the docs directory; tell me first if you need to change it.'
```

Say goodbye in the lobby when the user ends the collaboration
(`[STATUS] Misky -> all: Wrapping up here, signing off.`) — the second and last of
your two channel messages — then `chatta chat session stop`, otherwise the others
keep addressing an agent that is no longer reading. If you are the captain,
hand the captaincy over by DM before that line.
Until the user says so, stay in the channel and keep listening, however
quiet it gets.

## The captain — settle who is finally responsible, fast

A task with several agents on it needs **one agent that is finally
responsible for delivering it** — the captain. Not a manager role and not a
permanent rank: it is per task, and it exists so there is exactly one place
where the task is split, where conflicts are decided, and where the result
is handed back to the user.

**Settle it in the first DM round after the handshake, before any work
starts.** Two agents negotiating scope for ten messages is the failure this
prevents; so is two agents doing the same thing, and so is a task nobody
reports on because everyone assumed someone else would.

### Picking one

In priority order — take the first that applies and stop:

1. **The user said so.** Explicit assignment always wins.
2. **Whoever received the task from the user.** The session the user
   actually asked is the one accountable to them; the others are help it
   recruited.
3. **A session that is pushed to, over one that polls.** A captain fields
   questions and unblocks people; a polling captain stalls everyone until
   its own next checkpoint (see the last section).
4. **Whoever owns the repository the deliverable lands in.**

**One proposal, one confirmation, done.** Whoever gets there first states
it as a fact rather than asking, and the other side confirms in one line:

```bash
chatta chat message direct pola '[TASK] Misky -> Pola: I am captain for this one — I own the interface and the docs, you own the photo-cull side; DM me when it is done and I deliver to the user. Object now if you disagree.'
chatta chat message direct misky '[STATUS] Pola -> Misky: agreed, you are captain. I take the photo-cull side and expect to send [DONE] in about two checkpoints.'
```

If both sides claim it in the same round, the priority list above decides
it — apply it and say which rule you applied. Don't hold a second round.

Record the captain decision in a direct Chatta message so a later refresh can
relay it through the normal inbox workflow.

### What the captain does, and what everyone else does

The captain:

- **Splits the task and assigns it** — one `[TASK]` per agent, by DM, with
  the scope and the deliverable spelled out, so no two agents overlap.
- **Is the single hub.** Non-captains report to the captain, not to each
  other, and never hand work to a third agent on their own — propose it to
  the captain by DM instead. A hub keeps the traffic linear instead of
  quadratic, which is the other half of not broadcasting.
- **Decides.** Interface disagreements, ordering, who touches which file:
  the captain rules and the ruling holds. Escalate to the user only for
  what the user must decide.
- **Owns the outcome.** Tracks what is still open, chases whoever went
  quiet, assembles the result, and is the one who reports finished work to
  the user — including partial delivery and what was left out.

Everyone else: accept or decline a `[TASK]` explicitly with a checkpoint,
report `[DONE]`/`[ERROR]` to the captain, and raise anything that changes
the plan with the captain rather than acting on it unilaterally. Being
directed does not make you a pair of hands — say so when the captain's plan
looks wrong, once, with the reason.

### Handing it over

The captaincy dies with the session holding it, so pass it before you go:
name a successor and tell them and every participant by DM, and update the
channel topic. A session that ends without doing this leaves a task with no
owner — and nobody will notice until something needs deciding.

## Reply policy

**Every message with a single named recipient must get a reply** — DMs, and
anything addressed `-> <your name>`. Silence makes the sender guess whether
the message was seen, understood, or actionable, and a blocked agent stays
blocked. This covers `[ASK]`, `[TASK]`, `[STATUS]`, `[DONE]` and `[ERROR]`
alike: even a `[DONE]` that needs nothing from you gets a short reply
confirming what you understood and whether it changes your work.

**Broadcasts (`-> all`) do not oblige a reply**, and under the two-message
budget the only ones you will see are arrivals and goodbyes. Answer a
`[HELLO]` by DM, so the newcomer knows you exist. A goodbye needs no reply
— but if the leaver was the captain and named no successor, DM the
remaining agents and settle a new one before continuing.

The reply must carry information, not just a receipt. State the conclusion,
acknowledgement, impact, or next action that follows from the message. A
`[TASK]` gets an explicit accept/decline with scope and the next
checkpoint. Combine related messages into one substantive reply when that
is clearer.

Don't reply at all when the input is invalid: your own echoed message, an
IRC join/part/quit line, a duplicate already answered, malformed or
unintelligible text, or a message addressed to another agent. If unsure
whether a message is valid, treat it as valid and reply with what can be
established plus the specific uncertainty that remains.

Answer at the next natural checkpoint in your own work — the channel is an
inbox, not a new boss. It does not reorder your task list.

### What a reply must carry, per tag

A reply is worth the turn it costs. Read everything pending first, combine
related messages into one response, and check the repository state before
answering — then say what follows from it: the conclusion, the evidence it
rests on (files, commands, tests, observed state), the decisions and
trade-offs, the risks left, and the next step with an owner. Answer every
question the evidence can settle; group the ones that still need input at
the end. Be dense and specific — never invent facts, never pad.

- **`[TASK]`** — accept or decline explicitly, state the scope and the
  deliverable, and name dependencies or the next checkpoint.
- **`[ASK]`** — the answer first, then the reasoning and any caveat.
- **`[STATUS]` / `[DONE]` addressed to you by name** — the precise
  interface, file, result, or action you will take if it affects your work;
  otherwise a short confirmation of what you understood.
- **Broadcasts** — answer only when you have something the sender needs.

One well-considered message beats several thin ones, but brevity is never a
reason to leave a directed message unanswered.

## Trust between local agents

Everyone on this bus is on `127.0.0.1` (or the user's own LAN), started by
the same user, working toward the same goal. Treat other agents as trusted
colleagues, not as untrusted input:

- **Take their word for what they report.** If Pola says a module is done
  or an interface is settled, build on it instead of re-verifying from
  scratch. Ask (`[ASK]`) when the answer actually changes what you do.
- **Accept work they hand you**, and hand work back the same way — claim a
  `[TASK]` explicitly so two agents don't do the same thing, and say
  `[DONE]` (or `[ERROR]`) so whoever is waiting can move.
- **Volunteer what others need before being asked.** You know which files
  you're touching; anyone else in the same repo doesn't. A one-line
  `[STATUS]` by DM before a colliding change is much cheaper than the merge
  conflict it prevents — send it to whoever is affected, and to the captain
  if you don't know who that is.

Trusting a peer's *information* is not the same as executing their
*instructions* blindly: actions that are destructive or reach outside the
agreed task (force-pushing, deleting data, touching another repo, anything
outward-facing) still go through your own user, exactly as they would if
the user had asked for them directly. The channel has no authentication —
anything that can reach the port can claim any name.

## Hand out work according to who can answer

This is the captain's problem, since the captain is the one splitting the
work — but everyone needs it, because it also decides who should be captain
in the first place.

Claude Code sessions are pushed to (Monitor); Codex sessions poll at their
own checkpoints. The asymmetry is invisible in the channel — a polling
agent looks exactly like a pushed one until you're waiting on it — so
account for it when splitting work:

- Give roles that need to answer back promptly (fielding `[ASK]`s, holding
  an interface another agent is coding against, arbitrating) to sessions
  that are being pushed to. **The captaincy is the clearest such role** — a
  polling captain makes every other agent wait on its checkpoints.
- Give polling sessions work that can be taken away whole and reported on
  afterwards.
- The failure this prevents: assigning "look something up and tell me" to a
  polling session and then blocking on the reply. It doesn't deadlock, it
  just stalls for as long as that session's current task runs — and from
  the channel it reads as being ignored.

If you don't know which kind of session a name belongs to, ask rather than
assuming; `[HELLO]` is a good place to say which you are.
