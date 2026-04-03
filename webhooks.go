package turbine

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"
)

type cachedWebhook struct {
	url    string
	secret string
	events []string
}

type webhookPayload struct {
	Event      string `json:"event"`
	WorkflowID string `json:"workflow_id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Output     any    `json:"output,omitempty"`
	Error      string `json:"error,omitempty"`
	Timestamp  string `json:"timestamp"`
}

func (rt *Runtime) reloadWebhookCache() {
	records, err := rt.app.FindAllRecords(collectionWebhooks)
	if err != nil {
		rt.app.Logger().Error("failed to load webhooks for cache", "error", err)
		return
	}

	var webhooks []cachedWebhook
	for _, r := range records {
		if !r.GetBool("enabled") {
			continue
		}
		webhooks = append(webhooks, cachedWebhook{
			url:    r.GetString("url"),
			secret: r.GetString("secret"),
			events: parseEvents(r.Get("events")),
		})
	}
	rt.webhookCache.Store(webhooks)
}

func (rt *Runtime) dispatchWebhooks(workflowID, name string, status StatusType, output *string, errorMsg *string) {
	webhooks, _ := rt.webhookCache.Load().([]cachedWebhook)
	if len(webhooks) == 0 {
		return
	}

	eventName := "workflow." + string(status)

	payload := webhookPayload{
		Event:      eventName,
		WorkflowID: workflowID,
		Name:       name,
		Status:     string(status),
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}
	if output != nil {
		var parsed any
		if err := json.Unmarshal([]byte(*output), &parsed); err == nil {
			payload.Output = parsed
		} else {
			payload.Output = *output
		}
	}
	if errorMsg != nil {
		payload.Error = *errorMsg
	}

	body, err := json.Marshal(payload)
	if err != nil {
		rt.app.Logger().Error("failed to marshal webhook payload", "error", err)
		return
	}

	for _, wh := range webhooks {
		if !matchesEvent(wh.events, eventName) {
			continue
		}

		go func(url, secret string) {
			req, err := http.NewRequest("POST", url, bytes.NewReader(body))
			if err != nil {
				rt.app.Logger().Error("webhook request failed", "url", url, "error", err)
				return
			}
			req.Header.Set("Content-Type", "application/json")

			if secret != "" {
				mac := hmac.New(sha256.New, []byte(secret))
				mac.Write(body)
				sig := hex.EncodeToString(mac.Sum(nil))
				req.Header.Set("X-Turbine-Signature", sig)
			}

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				rt.app.Logger().Error("webhook delivery failed", "url", url, "error", err)
				return
			}
			resp.Body.Close()
		}(wh.url, wh.secret)
	}
}
