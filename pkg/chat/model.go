package chat

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
	InvokerKey string           `json:"invoker_key"`
	Offsets    map[string]int64 `json:"offsets"`
}
type ClientSurvey struct {
	ClientHome         string `json:"client_home"`
	SessionSummary     string `json:"session_summary"`
	ClientProcessState string `json:"client_process_state"`
	CleanupEligibility string `json:"cleanup_eligibility"`
}
