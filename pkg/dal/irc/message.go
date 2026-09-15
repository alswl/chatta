//go:build darwin || linux

package irc

import "strings"

// Message is one parsed IRC protocol line: an optional prefix, a command
// (numeric or word), and its parameters. The last parameter is "trailing"
// when the line carried a " :" argument — everything after it, colons
// included, is one parameter.
type Message struct {
	Prefix   string
	Command  string
	Params   []string
	Trailing bool
}

// ParseLine parses one IRC protocol line (without its trailing CRLF/LF).
// A line with no command is malformed and returns ok=false.
func ParseLine(line string) (Message, bool) {
	line = strings.TrimRight(line, "\r\n")
	if line == "" {
		return Message{}, false
	}
	var msg Message
	if strings.HasPrefix(line, ":") {
		sp := strings.IndexByte(line, ' ')
		if sp < 0 {
			return Message{}, false
		}
		msg.Prefix = line[1:sp]
		line = line[sp+1:]
	}
	// Split off the trailing parameter (" :...") first, if present.
	rest := line
	if idx := strings.Index(rest, " :"); idx >= 0 {
		trailing := rest[idx+2:]
		rest = rest[:idx]
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			return Message{}, false
		}
		msg.Command = strings.ToUpper(fields[0])
		msg.Params = append(fields[1:], trailing)
		msg.Trailing = true
		return msg, true
	}
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return Message{}, false
	}
	msg.Command = strings.ToUpper(fields[0])
	msg.Params = fields[1:]
	return msg, true
}

// FormatLine formats a command and parameters into one IRC protocol line
// (without a trailing CRLF). The last parameter is sent as a trailing
// parameter (" :...") whenever it contains a space, starts with ':', or is
// empty, since those cannot survive as an ordinary middle parameter.
func FormatLine(command string, params ...string) string {
	var b strings.Builder
	b.WriteString(command)
	for i, p := range params {
		b.WriteByte(' ')
		last := i == len(params)-1
		if last && (p == "" || strings.ContainsAny(p, " ") || strings.HasPrefix(p, ":")) {
			b.WriteByte(':')
		}
		b.WriteString(p)
	}
	return b.String()
}

// Nick extracts the nick from a "nick!user@host" prefix, or returns the
// prefix unchanged if it carries no user/host part (server prefixes).
func Nick(prefix string) string {
	nick, _, found := strings.Cut(prefix, "!")
	if !found {
		return prefix
	}
	return nick
}
