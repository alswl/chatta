package daemon

import "github.com/alswl/chatta/pkg/common"

// Failure codes, per contracts/control-socket.md.
const (
	CodeNotConnected  = "not_connected"
	CodeNotRegistered = "not_registered"
	CodeNickInUse     = "nick_in_use"
	CodeNickInvalid   = "nick_invalid"
	CodeNoSuchNick    = "no_such_nick"
	CodeNotOnChannel  = "not_on_channel"
	CodeJoinFailed    = "join_failed"
	CodeTimeout       = "timeout"
	CodeBadRequest    = "bad_request"
)

// ControlRequest is one newline-delimited JSON request sent to the
// supervisor's control socket.
type ControlRequest struct {
	Op     string `json:"op"`
	Target string `json:"target,omitempty"`
	Text   string `json:"text,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// ControlResponse is the supervisor's reply to one ControlRequest.
type ControlResponse struct {
	OK      bool                    `json:"ok"`
	Error   string                  `json:"error,omitempty"`
	Code    string                  `json:"code,omitempty"`
	Members []string                `json:"members,omitempty"`
	Status  *common.TransportStatus `json:"status,omitempty"`
}
