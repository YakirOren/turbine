package turbine

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/nicholas-fedor/shoutrrr"
)

func (rt *Runtime) dispatchNotifications(workflowID, name string, status StatusType, errorMsg *string) {
	records, err := rt.app.FindAllRecords(collectionAlertChannels)
	if err != nil || len(records) == 0 {
		return
	}

	eventName := "workflow." + string(status)

	message := formatNotificationMessage(workflowID, name, status, errorMsg)

	for _, record := range records {
		if !record.GetBool("enabled") {
			continue
		}

		var events []string
		eventsRaw := record.Get("events")
		if b, err := json.Marshal(eventsRaw); err == nil {
			_ = json.Unmarshal(b, &events)
		}

		matched := false
		for _, ev := range events {
			if ev == eventName || ev == "workflow.*" {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}

		rawURL := record.GetString("url")
		go func(rawURL string) {
			if err := shoutrrr.Send(rawURL, message); err != nil {
				scheme := extractScheme(rawURL)
				rt.app.Logger().Error("notification delivery failed",
					"service", scheme,
					"error", err,
					"source", "system",
				)
			}
		}(rawURL)
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
