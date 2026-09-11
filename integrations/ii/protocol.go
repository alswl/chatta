//go:build darwin || linux

package ii

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	chatLineRE   = regexp.MustCompile(`^(\d+) <([^>]*)> (.*)$`)
	chatAnyRE    = regexp.MustCompile(`^(\d+) (.*)$`)
	chatNamesRE  = regexp.MustCompile(`^\d+ = (\S+) (.*)$`)
	rawNamesRE   = regexp.MustCompile(`^\d+ :\S+ 353 \S+ = (\S+) :(.*)$`)
	rawMessageRE = regexp.MustCompile(`^(\d+) :([^! ]+)!\S+ PRIVMSG \S+ :(.*)$`)
)

func ParseNames(line string) (string, []string, bool) {
	m := chatNamesRE.FindStringSubmatch(line)
	if m != nil {
		return m[1], strings.Fields(m[2]), true
	}
	m = rawNamesRE.FindStringSubmatch(line)
	if m != nil {
		return m[1], strings.Fields(m[2]), true
	}
	return "", nil, false
}

func ParseLine(line string) (timestamp int64, nick, text string, ok bool) {
	if m := chatLineRE.FindStringSubmatch(line); m != nil {
		ts, err := strconv.ParseInt(m[1], 10, 64)
		return ts, m[2], m[3], err == nil
	}
	if m := chatAnyRE.FindStringSubmatch(line); m != nil {
		if raw := rawMessageRE.FindStringSubmatch(line); raw != nil {
			ts, err := strconv.ParseInt(raw[1], 10, 64)
			return ts, raw[2], raw[3], err == nil
		}
		ts, err := strconv.ParseInt(m[1], 10, 64)
		return ts, "", m[2], err == nil
	}
	return 0, "", line, false
}
