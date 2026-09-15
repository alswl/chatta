// Package common holds types shared across the chat layers: session state,
// health reports, and other data exchanged between dal, managers, and services.
package common

import "time"

const StateSchemaVersion = 1

type OwnerBinding struct {
	PID              int    `json:"pid"`
	StartFingerprint string `json:"start_fingerprint"`
	Runtime          string `json:"runtime"`
}
type ChannelMembership struct {
	Name      string    `json:"name"`
	Kind      string    `json:"kind"`
	JoinedAt  time.Time `json:"joined_at"`
	Confirmed bool      `json:"confirmed"`
}
type ChatSession struct {
	SchemaVersion              int                 `json:"schema_version"`
	Nick                       string              `json:"nick"`
	Role                       string              `json:"role"`
	Host                       string              `json:"host"`
	Port                       int                 `json:"port"`
	HomeChannel                ChannelMembership   `json:"home_channel"`
	Channels                   []ChannelMembership `json:"channels"`
	Owner                      OwnerBinding        `json:"owner"`
	SessionID                  string              `json:"session_id"`
	SupervisorPID              int                 `json:"supervisor_pid"`
	SupervisorStartFingerprint string              `json:"supervisor_start_fingerprint"`
	StartedAt                  time.Time           `json:"started_at"`
}
type HealthReport struct {
	Owner          bool   `json:"owner"`
	Supervisor     bool   `json:"supervisor"`
	JoinedChannels bool   `json:"joined_channels"`
	ClientReader   bool   `json:"client_reader"`
	ServerLink     bool   `json:"server_link"`
	Membership     bool   `json:"membership"`
	Failure        string `json:"failure,omitempty"`
}
type Conversation struct {
	Name           string `json:"name"`
	SourceKind     string `json:"source_kind"`
	TranscriptPath string `json:"transcript_path"`
}
type MessageCursor struct {
	InvokerKey string `json:"invoker_key"`
	Offset     int64  `json:"offset"`
}
type ClientSurvey struct {
	ClientHome         string `json:"client_home"`
	SessionSummary     string `json:"session_summary"`
	OwnerState         string `json:"owner_state"`
	SupervisorState    string `json:"supervisor_state"`
	ClientProcessState string `json:"client_process_state"`
	IIProcessCount     int    `json:"ii_process_count"`
	CleanupEligibility string `json:"cleanup_eligibility"`
}

// StoredMessage is one line of messages.jsonl: a received PRIVMSG the
// supervisor has recorded.
type StoredMessage struct {
	TS     int64  `json:"ts"`
	Kind   string `json:"kind"`
	Target string `json:"target"`
	Nick   string `json:"nick"`
	Text   string `json:"text"`
}

// TransportStatus reports the supervisor's live IRC connection state, as
// returned by the control socket's "status" op.
type TransportStatus struct {
	Connected  bool     `json:"connected"`
	Registered bool     `json:"registered"`
	Nick       string   `json:"nick"`
	Channels   []string `json:"channels"`
	LastPong   int64    `json:"last_pong"`
	Failure    string   `json:"failure,omitempty"`
}
