// Package notifications owns turbine's outbound shoutrrr-based alert
// channels: cache management, SSRF guards on generic:// URLs, message
// formatting, and per-record validation used by collection hooks.
package notifications

import (
	"fmt"
	"log/slog"
	"net/url"
	"sync/atomic"

	"github.com/YakirOren/turbine/internal/retry"
	"github.com/YakirOren/turbine/internal/sysdb"
	"github.com/YakirOren/turbine/internal/webhooks"
	"github.com/nicholas-fedor/shoutrrr"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

// Config tunes the notification sender.
type Config struct {
	AllowPrivateAddresses bool
}

type Sender struct {
	app         core.App
	logger      *slog.Logger
	cfg         Config
	cache       atomic.Value // []cachedChannel
	collection  string
	validEvents map[string]bool
}

type cachedChannel struct {
	url    string
	events []string
}

func NewSender(app core.App, logger *slog.Logger, cfg Config, collection string, validEvents map[string]bool) *Sender {
	return &Sender{
		app:         app,
		logger:      logger,
		cfg:         cfg,
		collection:  collection,
		validEvents: validEvents,
	}
}

func (s *Sender) ReloadCache() {
	records, err := s.app.FindAllRecords(s.collection)
	if err != nil {
		s.app.Logger().Error("failed to load alert channels for cache", "error", err)
		return
	}

	var channels []cachedChannel
	for _, r := range records {
		if !r.GetBool("enabled") {
			continue
		}
		channels = append(channels, cachedChannel{
			url:    r.GetString("url"),
			events: r.GetStringSlice("events"),
		})
	}
	s.cache.Store(channels)
}

// EventMatcher decides which subscribed events match the synthesized event name.
type EventMatcher func(events []string, eventName string) bool

func (s *Sender) Dispatch(workflowID, name string, status sysdb.StatusType, errorMsg *string, matcher EventMatcher) {
	channels, _ := s.cache.Load().([]cachedChannel)
	if len(channels) == 0 {
		return
	}

	eventName := "workflow." + string(status)
	message := FormatMessage(workflowID, name, status, errorMsg)

	for _, ch := range channels {
		if !matcher(ch.events, eventName) {
			continue
		}

		go func(rawURL string) {
			scheme := ExtractScheme(rawURL)
			defer retry.RecoverGoroutine(s.app.Logger(), "notification goroutine panicked", "service", scheme)
			if err := shoutrrr.Send(rawURL, message); err != nil {
				s.app.Logger().Error("notification delivery failed",
					"service", scheme,
					"error", err,
					"source", "system",
				)
			}
		}(ch.url)
	}
}

// SendCustom delivers a custom message to the channel matching name. Disabled
// channels are a silent no-op (returns nil), matching the event-driven dispatch
// behavior, toggling a channel off mutes both. If multiple channels share the
// same name, the first match wins.
func (s *Sender) SendCustom(name, message string) error {
	record, err := s.app.FindFirstRecordByFilter(
		s.collection,
		"name = {:name}",
		dbx.Params{"name": name},
	)
	if err != nil {
		return fmt.Errorf("alert channel %q not found: %w", name, err)
	}
	if !record.GetBool("enabled") {
		return nil
	}
	return shoutrrr.Send(record.GetString("url"), message)
}

// Test sends a test notification to verify the channel works.
func (s *Sender) Test(id string) error {
	record, err := s.app.FindRecordById(s.collection, id)
	if err != nil {
		return fmt.Errorf("alert channel not found: %w", err)
	}
	rawURL := record.GetString("url")
	return shoutrrr.Send(rawURL, "[Turbine] Test notification: this channel is working")
}

// ValidateRecord validates an alert channel record. When allowPrivate is false,
// it also blocks generic:// URLs whose host is in a private/loopback range.
func ValidateRecord(r *core.Record, allowPrivate bool, validEvents map[string]bool) error {
	rawURL := r.GetString("url")
	if rawURL == "" {
		return router.NewBadRequestError("URL is required", nil)
	}

	if _, err := shoutrrr.CreateSender(rawURL); err != nil {
		return router.NewBadRequestError("invalid notification URL: "+err.Error(), nil)
	}

	if !allowPrivate {
		if err := validateShoutrrrSSRF(rawURL); err != nil {
			return router.NewBadRequestError("invalid notification URL: "+err.Error(), nil)
		}
	}

	events := r.GetStringSlice("events")
	if len(events) == 0 {
		return router.NewBadRequestError("at least one event is required", nil)
	}
	for _, ev := range events {
		if !validEvents[ev] {
			return router.NewBadRequestError("invalid event type: "+ev, nil)
		}
	}

	return nil
}

// validateShoutrrrSSRF rejects shoutrrr URLs whose generic:// host points at
// a private/loopback/link-local address. Well-known service schemes (slack,
// discord, telegram, ...) hit fixed public endpoints and are accepted as-is.
func validateShoutrrrSSRF(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if parsed.Scheme != "generic" {
		return nil
	}
	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("generic:// URL must have a host")
	}
	return webhooks.RejectPrivateHost(host)
}

// FormatMessage produces the human-readable line shown in each alert channel.
func FormatMessage(workflowID, name string, status sysdb.StatusType, errorMsg *string) string {
	switch status {
	case sysdb.StatusSuccess:
		return fmt.Sprintf("[Turbine] Workflow %q (%s) completed successfully", name, workflowID)
	case sysdb.StatusError:
		msg := fmt.Sprintf("[Turbine] Workflow %q (%s) failed", name, workflowID)
		if errorMsg != nil && *errorMsg != "" {
			msg += ": " + *errorMsg
		}
		return msg
	case sysdb.StatusCancelled:
		return fmt.Sprintf("[Turbine] Workflow %q (%s) cancelled", name, workflowID)
	case sysdb.StatusWaitingForApproval:
		return fmt.Sprintf("[Turbine] Workflow %q (%s) is waiting for approval", name, workflowID)
	case sysdb.StatusMaxRecoveryAttemptsExceeded:
		msg := fmt.Sprintf("[Turbine] Workflow %q (%s) exceeded max recovery attempts", name, workflowID)
		if errorMsg != nil && *errorMsg != "" {
			msg += ": " + *errorMsg
		}
		return msg
	default:
		return fmt.Sprintf("[Turbine] Workflow %q (%s): %s", name, workflowID, status)
	}
}

// ExtractScheme returns the URL scheme (slack, discord, smtp, ...). Used by
// the dispatcher for log fields and by tests.
func ExtractScheme(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "unknown"
	}
	return parsed.Scheme
}
