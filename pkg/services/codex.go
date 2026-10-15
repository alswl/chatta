//go:build darwin || linux

package services

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/alswl/chatta/pkg/common"
	"github.com/alswl/chatta/pkg/dal"
	"golang.org/x/sys/unix"
)

var errCodexBindingChanged = errors.New("codex binding changed or was disabled before dispatch")

func newEventID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (m *ChatService) runCodexWorker(ctx context.Context, h *controlHandler, log *slog.Logger, wake <-chan struct{}) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	backoff := time.Second
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-wake:
			// Debounce wake notifications; persisted messages are still delivered individually.
			select {
			case <-time.After(250 * time.Millisecond):
			case <-ctx.Done():
				return
			}
		}
		h.bindingMu.Lock()
		binding, err := dal.LoadCodexBinding(m.Paths.CodexBinding)
		h.bindingMu.Unlock()
		if err != nil || !binding.Enabled {
			backoff = time.Second
			continue
		}
		if binding.SessionID != m.State.SessionID {
			log.Error("Codex binding belongs to another session")
			continue
		}
		records, next, err := dal.ReadCompleteMessageRecords(m.Paths.Messages, binding.Offset)
		if err != nil {
			var invalid *dal.InvalidMessageRecordError
			if !errors.As(err, &invalid) {
				log.Error("read Codex delivery queue", "error", err)
				continue
			}
			log.Error("Codex delivery queue contains an invalid complete record; cursor retained at record", "offset", invalid.Offset, "error", err)
		}
		if len(records) == 0 {
			if next > binding.Offset {
				if err := h.saveCodexOffset(binding, next); err != nil {
					log.Error("advance Codex cursor", "error", err)
				}
			}
			continue
		}
		for i := 0; i < len(records); {
			if ctx.Err() != nil {
				return
			}
			record := records[i]
			msg := record.Message
			if !addressedTo(msg, m.State.Nick, binding) {
				if err := h.saveCodexOffset(binding, record.Offset); err != nil {
					log.Error("advance filtered Codex event", "error", err)
					break
				}
				binding.Offset = record.Offset
				i++
				continue
			}
			newOffset := record.Offset
			if msg.EventID == "" {
				log.Error("message lacks stable event ID; retaining Codex cursor")
				break
			}
			unlock, err := lockThreadWith(binding, m.codexLockContended)
			if err != nil {
				log.Error("serialize Codex thread delivery", "error", err)
				break
			}
			body := formatCodexEvent(msg, m.State.Nick)
			if !h.codexBindingCurrent(binding) {
				unlock()
				log.Info("Codex binding changed or disabled while waiting for thread lock; pending event retained", "event_id", msg.EventID, "binding_id", binding.BindingID)
				break
			}
			deliveryAck, deliveryErr := (codexAppClient{
				Endpoint: binding.Endpoint, ThreadID: binding.ThreadID, TracePath: os.Getenv("CHATTA_CODEX_RPC_TRACE"),
				NoStartAfterSteerFailure: binding.NoStartAfterSteerFailure,
				AuthorizeDispatch:        func(send func() error) error { return h.authorizeCodexDispatch(binding, send) },
			}).Deliver(ctx, body)
			unlock()
			if errors.Is(deliveryErr, errCodexBindingChanged) {
				log.Info("Codex binding changed or was disabled before dispatch; pending event retained", "event_id", msg.EventID, "binding_id", binding.BindingID)
				break
			}
			if deliveryErr != nil {
				log.Warn("Codex event delivery retained", "event_id", msg.EventID, "error", deliveryErr)
				if err := h.saveCodexDeliveryFailure(binding, deliveryErr); err != nil {
					log.Error("persist Codex delivery failure", "event_id", msg.EventID, "error", err)
				}
				break
			}
			log.Info("Codex delivery RPC acknowledged", "event_id", msg.EventID, "source", msg.Source, "method", deliveryAck.Method, "thread_id", deliveryAck.ThreadID, "turn_id", deliveryAck.TurnID)
			if err := h.saveCodexProgress(binding, newOffset, true); err != nil {
				log.Error("persist Codex delivery ACK", "event_id", msg.EventID, "error", err)
				break
			}
			binding.Offset = newOffset
			backoff = time.Second
			if i+1 < len(records) {
				select {
				case <-time.After(200 * time.Millisecond):
				case <-ctx.Done():
					return
				}
			}
			i++
		}
		if binding.Offset < next {
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			}
			if backoff < 30*time.Second {
				backoff *= 2
				if backoff > 30*time.Second {
					backoff = 30 * time.Second
				}
			}
		}
	}
}

func (h *controlHandler) codexBindingCurrent(expected common.CodexBinding) bool {
	h.bindingMu.Lock()
	defer h.bindingMu.Unlock()
	current, err := dal.LoadCodexBinding(h.m.Paths.CodexBinding)
	return err == nil && current.Enabled && codexBindingMatches(current, expected) && current.SessionID == h.m.State.SessionID
}

func (h *controlHandler) authorizeCodexDispatch(expected common.CodexBinding, send func() error) error {
	h.bindingMu.Lock()
	defer h.bindingMu.Unlock()
	current, err := dal.LoadCodexBinding(h.m.Paths.CodexBinding)
	if err != nil {
		return err
	}
	if !codexBindingMatches(current, expected) || !current.Enabled || current.SessionID != h.m.State.SessionID {
		return errCodexBindingChanged
	}
	return send()
}

func codexBindingMatches(current, expected common.CodexBinding) bool {
	// Rebinding changes the generation; an idempotent startup reuse keeps it.
	return current.BindingID == expected.BindingID && current.SessionID == expected.SessionID && current.Endpoint == expected.Endpoint && current.ThreadID == expected.ThreadID
}

func formatCodexEvent(msg common.StoredMessage, receiver string) string {
	target := msg.Target
	if msg.Kind == "direct" {
		target = receiver
	}
	return fmt.Sprintf("Chatta incoming message\nevent_id=%s\nsource=%s\nfrom=%s\ntarget=%s\nkind=%s\n\n%s", msg.EventID, msg.Source, msg.Nick, target, msg.Kind, msg.Text)
}

func (h *controlHandler) saveCodexOffset(expected common.CodexBinding, offset int64) error {
	return h.saveCodexProgress(expected, offset, false)
}

func (h *controlHandler) saveCodexProgress(expected common.CodexBinding, offset int64, acknowledged bool) error {
	h.bindingMu.Lock()
	defer h.bindingMu.Unlock()
	current, err := dal.LoadCodexBinding(h.m.Paths.CodexBinding)
	if err != nil {
		return err
	}
	if !codexBindingMatches(current, expected) || !current.Enabled {
		return fmt.Errorf("codex binding changed while processing event; cursor retained")
	}
	current.Offset = offset
	if acknowledged {
		current.DeliveryBlocked = false
		current.DeliveryState = "accepted"
		current.DeliveryError = ""
		current.NoStartAfterSteerFailure = false
	}
	current.UpdatedAt = time.Now().Unix()
	return dal.SaveCodexBinding(h.m.Paths.CodexBinding, current)
}

func (h *controlHandler) saveCodexDeliveryFailure(expected common.CodexBinding, deliveryErr error) error {
	h.bindingMu.Lock()
	defer h.bindingMu.Unlock()
	current, err := dal.LoadCodexBinding(h.m.Paths.CodexBinding)
	if err != nil {
		return err
	}
	if !codexBindingMatches(current, expected) || !current.Enabled {
		return fmt.Errorf("codex binding changed while recording delivery failure")
	}
	current.DeliveryBlocked = true
	current.DeliveryState = deliveryFailureState(deliveryErr)
	var unavailable *threadUnavailableError
	if errors.As(deliveryErr, &unavailable) {
		if unavailable.State != "" {
			current.DeliveryState = unavailable.State
		}
	}
	var rpcErr *rpcError
	if errors.As(deliveryErr, &rpcErr) && rpcErr.Method == "turn/steer" && !isIdleSteerRace(rpcErr) {
		current.NoStartAfterSteerFailure = true
	}
	current.DeliveryError = deliveryErr.Error()
	current.UpdatedAt = time.Now().Unix()
	return dal.SaveCodexBinding(h.m.Paths.CodexBinding, current)
}

func deliveryFailureState(err error) string {
	var unavailable *threadUnavailableError
	if errors.As(err, &unavailable) {
		if unavailable.State != "" {
			return unavailable.State
		}
		return "unavailable"
	}
	var rpcErr *rpcError
	if errors.As(err, &rpcErr) {
		message := strings.ToLower(rpcErr.Message)
		if rpcErr.Method == "turn/steer" {
			return "steer_error_blocked"
		}
		switch {
		case strings.Contains(message, "not loaded"):
			return "not_loaded"
		case strings.Contains(message, "closed"):
			return "closed"
		default:
			return "rpc_error"
		}
	}
	if strings.Contains(strings.ToLower(err.Error()), "idle after an unclassified turn/steer error") {
		return "steer_error_blocked"
	}
	return "unavailable"
}

func addressedTo(msg common.StoredMessage, nick string, binding common.CodexBinding) bool {
	if nick == "" || strings.EqualFold(msg.Nick, nick) {
		return false
	}
	if msg.Kind == "direct" {
		// The IRC parser marks the event as direct only after the server
		// delivered it to this connection; Target records the sender nick.
		return true
	}
	if msg.Kind != "channel" {
		return false
	}
	if channelSubscribed(msg.Target, binding.SubscribedChannels) {
		return true
	}
	if !binding.MentionCurrentIdentity {
		return false
	}
	mention := regexp.MustCompile(`(?i)(^|[^a-z0-9_])` + regexp.QuoteMeta(nick) + `([^a-z0-9_]|$)`)
	return mention.MatchString(msg.Text)
}

func channelSubscribed(target string, subscriptions []string) bool {
	target = strings.ToLower(strings.TrimSpace(target))
	for _, channel := range subscriptions {
		if target == strings.ToLower(strings.TrimSpace(channel)) {
			return true
		}
	}
	return false
}

func normalizeSubscribedChannels(channels []string) ([]string, error) {
	seen := make(map[string]struct{}, len(channels))
	normalized := make([]string, 0, len(channels))
	for _, channel := range channels {
		channel = strings.ToLower(strings.TrimSpace(channel))
		if channel == "" || !strings.HasPrefix(channel, "#") {
			return nil, fmt.Errorf("subscribed channel must start with #: %q", channel)
		}
		if _, ok := seen[channel]; ok {
			continue
		}
		seen[channel] = struct{}{}
		normalized = append(normalized, channel)
	}
	return normalized, nil
}

func lockThread(binding common.CodexBinding) (func(), error) {
	return lockThreadWith(binding, nil)
}

func lockThreadWith(binding common.CodexBinding, onContended func()) (func(), error) {
	key := binding.Endpoint + "\x00" + binding.ThreadID
	root, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	// A deterministic lock path serializes all Chatta supervisors in this
	// user's local machine that target the same endpoint and thread.
	sum := sha256.Sum256([]byte(key))
	lockPath := filepath.Join(root, ".irc-agent", "codex-locks", hex.EncodeToString(sum[:])+".lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EAGAIN) {
			_ = f.Close()
			return nil, err
		}
		if onContended != nil {
			onContended()
		}
		if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
			_ = f.Close()
			return nil, err
		}
	}
	return func() { _ = unix.Flock(int(f.Fd()), unix.LOCK_UN); _ = f.Close() }, nil
}
