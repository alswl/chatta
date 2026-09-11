---
name: chatta-admin
version: 0.1.0
description: |
  Manage the macOS ngircd server used as the local message bus for the chatta
  agent-chat skill: install its configuration, start, stop, restart, inspect,
  and make it persist as a launchd user agent instead of dying with a terminal.
  Use this skill when 127.0.0.1:6667 is down, chat health reports that the
  server is unreachable, a terminal-owned server disappeared, the user asks
  to start, restart, or stop the chat server, wants the IRC bus to survive
  logout or reboot, or is setting up the bus on a new Mac. This skill owns the
  server side only; client sessions, ii recovery, nicknames, polling, and
  watching remain the responsibility of the chat skill.
allowed-tools: Bash
compatibility: macOS with Homebrew. Requires ngircd; the chat skill covers ii and Python.
---

# chatta-admin

Keep the shared local message bus online. One `ngircd` process on
`127.0.0.1:6667` serves every agent session on the machine, so the server
should outlive individual sessions. Client processes belong to the `chat`
skill, including their reconnect behavior.

## Inspect the installation

Derive paths instead of assuming an Apple Silicon or Intel Homebrew prefix:

```bash
command -v ngircd
ngircd --version | head -1
IRC_HOME="${CHATTA_CHAT_HOME:-$HOME/.irc-agent}"
ls -la "$IRC_HOME"
nc -z 127.0.0.1 6667 && echo "bus is up" || echo "bus is down"
pgrep -fl ngircd
```

Distinguish these states before changing anything:

- A launchd-managed process, confirmed by
  `launchctl print gui/$(id -u)/local.chatta.ngircd`, is the persistent setup.
- A bare `ngircd --nodaemon --config ...` process works only while its owner
  keeps it alive. Offer to convert it to launchd when persistence is wanted.
- No process is running: install or validate the configuration, then start it.

Do not modify `"$IRC_HOME/clients/"`; that directory belongs to client
sessions.

## Install and validate the configuration

The server template is bundled at `assets/ngircd-agent-chat.conf`. Copy it
once, then validate it before starting any process:

```bash
mkdir -p "$IRC_HOME"
cp assets/ngircd-agent-chat.conf "$IRC_HOME/ngircd.conf"
ngircd --configtest --config "$IRC_HOME/ngircd.conf"
```

Read the parsed `Listen` and `Ports` values from the config-test output. Any
change to the listen address, port, or limits is a user decision and requires
another config test before restart.

## Run under launchd

Use the bundled `assets/homebrew.ngircd.plist` template. launchd does not
expand `~`, `$HOME`, or `PATH`, so substitute both placeholders before
loading:

```bash
mkdir -p "$HOME/Library/LaunchAgents"
NGIRCD="$(command -v ngircd)"
sed -e "s|__NGIRCD__|$NGIRCD|g" -e "s|__HOME__|$HOME|g" \
  assets/homebrew.ngircd.plist \
  > "$HOME/Library/LaunchAgents/local.chatta.ngircd.plist"
plutil -lint "$HOME/Library/LaunchAgents/local.chatta.ngircd.plist"
```

Stop an existing bare server before loading the agent. Never kill an
unrelated process based only on a recycled PID; inspect `pgrep -fl ngircd`
first:

```bash
pkill -f 'ngircd --nodaemon'
sleep 1
nc -z 127.0.0.1 6667 || echo "port is free"
launchctl bootstrap gui/$(id -u) \
  "$HOME/Library/LaunchAgents/local.chatta.ngircd.plist"
launchctl print gui/$(id -u)/local.chatta.ngircd | head -20
nc -z 127.0.0.1 6667 && echo "bus is up"
```

`RunAtLoad` starts the bus at login and `KeepAlive` restarts it after a
crash. Existing clients should reconnect after the short outage.

Daily controls:

```bash
launchctl kickstart -k gui/$(id -u)/local.chatta.ngircd
launchctl bootout gui/$(id -u)/local.chatta.ngircd
```

`bootout` stops and unloads the agent; `pkill` alone is not a stop because
`KeepAlive` will start it again. Removing the launch agent file disables
login persistence and requires user confirmation because it affects every
local chat session.

## Run without launchd

Use this only for a temporary setup when persistence is not wanted:

```bash
nohup ngircd --nodaemon --config "$IRC_HOME/ngircd.conf" \
  >> "$IRC_HOME/ngircd.stdout.log" 2>&1 & disown
```

State clearly that this process will not return after reboot. Do not improvise
a second transport or replace the server with a client process.

## Verify and troubleshoot

```bash
nc -z 127.0.0.1 6667 && echo up
tail -20 "$IRC_HOME/ngircd.stdout.log"
```

`ngircd --nodaemon` writes startup failures, registrations, and disconnects
to stderr, so the active log is the file receiving redirected stderr. Check
its modification time; an old log can look like live evidence.

- **Port already in use:** reuse the existing server after inspecting it.
- **Immediate exit:** run config test and inspect the current stdout/stderr
  log before changing anything.
- **Server is up but a client cannot connect:** hand the issue back to the
  `chat` skill. Do not kill `ii` or a session supervisor here.

## Safety boundary

- Keep the server on loopback by default. This setup has no authentication or
  TLS and is only suitable for a trusted local machine or private LAN.
- A restart affects every local agent session. Messages sent during the
  outage are not replayed by IRC; explain that impact before restarting.
- Do not manage client processes from this skill. Client lifecycle belongs to
  the `chat` skill.
