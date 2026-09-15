//go:build darwin || linux

package irc

// Numeric replies this feature cares about. See RFC 1459/2812 for the full set.
const (
	RPL_WELCOME          = "001"
	RPL_NAMREPLY         = "353"
	RPL_ENDOFNAMES       = "366"
	RPL_TIME             = "391"
	ERR_NOSUCHNICK       = "401"
	ERR_NOSUCHCHANNEL    = "403"
	ERR_ERRONEUSNICKNAME = "432"
	ERR_NICKNAMEINUSE    = "433"
	ERR_NOTREGISTERED    = "451"
	ERR_INVITEONLYCHAN   = "473"
	ERR_BANNEDFROMCHAN   = "474"
	ERR_BADCHANNELKEY    = "475"
)
