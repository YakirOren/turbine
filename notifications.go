package turbine

import (
	"fmt"
	"net/url"

	"github.com/nicholas-fedor/shoutrrr"
)

type cachedAlertChannel struct {
	url    string
	events []string
}

func (rt *Runtime) reloadAlertChannelCache() {
	records, err := rt.app.FindAllRecords(collectionAlertChannels)
	if err != nil {
		rt.app.Logger().Error("failed to load alert channels for cache", "error", err)
		return
	}

	var channels []cachedAlertChannel
	for _, r := range records {
		if !r.GetBool("enabled") {
			continue
		}
		channels = append(channels, cachedAlertChannel{
			url:    r.GetString("url"),
			events: r.GetStringSlice("events"),
		})
	}
	rt.alertChannelCache.Store(channels)
}

func (rt *Runtime) dispatchNotifications(workflowID, name string, status StatusType, errorMsg *string) {
	channels, _ := rt.alertChannelCache.Load().([]cachedAlertChannel)
	if len(channels) == 0 {
		return
	}

	eventName := "workflow." + string(status)
	message := formatNotificationMessage(workflowID, name, status, errorMsg)

	for _, ch := range channels {
		if !matchesEvent(ch.events, eventName) {
			continue
		}

		go func(rawURL string) {
			if err := shoutrrr.Send(rawURL, message); err != nil {
				scheme := extractScheme(rawURL)
				rt.app.Logger().Error("notification delivery failed",
					"service", scheme,
					"error", err,
					"source", "system",
				)
			}
		}(ch.url)
	}
}

func formatNotificationMessage(workflowID, name string, status StatusType, errorMsg *string) string {
	switch status {
	case StatusSuccess:
		return fmt.Sprintf("[Turbine] Workflow %q (%s) completed successfully", name, workflowID)
	case StatusError:
		msg := fmt.Sprintf("[Turbine] Workflow %q (%s) failed", name, workflowID)
		if errorMsg != nil && *errorMsg != "" {
			msg += ": " + *errorMsg
		}
		return msg
	case StatusCancelled:
		return fmt.Sprintf("[Turbine] Workflow %q (%s) cancelled", name, workflowID)
	case StatusWaitingForApproval:
		return fmt.Sprintf("[Turbine] Workflow %q (%s) is waiting for approval", name, workflowID)
	case StatusMaxRecoveryAttemptsExceeded:
		msg := fmt.Sprintf("[Turbine] Workflow %q (%s) exceeded max recovery attempts", name, workflowID)
		if errorMsg != nil && *errorMsg != "" {
			msg += ": " + *errorMsg
		}
		return msg
	default:
		return fmt.Sprintf("[Turbine] Workflow %q (%s) — %s", name, workflowID, status)
	}
}

func extractScheme(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "unknown"
	}
	return parsed.Scheme
}

// TestAlertChannel sends a test notification to verify the channel works.
func (rt *Runtime) TestAlertChannel(id string) error {
	record, err := rt.app.FindRecordById(collectionAlertChannels, id)
	if err != nil {
		return fmt.Errorf("alert channel not found: %w", err)
	}

	rawURL := record.GetString("url")
	return shoutrrr.Send(rawURL, "[Turbine] Test notification — this channel is working")
}
