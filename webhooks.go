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

type webhookPayload struct {
	Event      string `json:"event"`
	WorkflowID string `json:"workflow_id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Output     any    `json:"output,omitempty"`
	Error      string `json:"error,omitempty"`
	Timestamp  string `json:"timestamp"`
}

func (rt *Runtime) dispatchWebhooks(workflowID, name string, status StatusType, output *string, errorMsg *string) {
	records, err := rt.app.FindAllRecords(collectionWebhooks)
	if err != nil || len(records) == 0 {
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

		url := record.GetString("url")
		secret := record.GetString("secret")

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
		}(url, secret)
	}
}
