package chat

import (
	"regexp"
	"strings"
)

var invalidChannel = regexp.MustCompile(`[^a-z0-9-]+`)

func NormalizeChannel(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.TrimPrefix(name, "#")
	name = invalidChannel.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		name = "lobby"
	}
	if len(name) > 47 {
		name = name[:47]
	}
	return "#" + name
}
func ValidateChannels(channels []ChannelMembership, home string) error {
	if len(channels) == 0 {
		return nil
	}
	expected := NormalizeChannel(home)
	seen := map[string]bool{}
	for i, c := range channels {
		n := NormalizeChannel(c.Name)
		if seen[n] {
			return &ChannelError{Message: "duplicate channel: " + n}
		}
		seen[n] = true
		if i == 0 && n != expected {
			return &ChannelError{Message: "home channel must be first"}
		}
	}
	return nil
}
func CanPart(channel, home string) error {
	if NormalizeChannel(channel) == NormalizeChannel(home) {
		return &ChannelError{Message: "cannot part home channel"}
	}
	return nil
}

type ChannelError struct{ Message string }

func (e *ChannelError) Error() string { return e.Message }
