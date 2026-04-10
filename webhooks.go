package turbine

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/hashicorp/go-retryablehttp"
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
			events: r.GetStringSlice("events"),
		})
	}
	rt.webhookCache.Store(webhooks)
}

func (rt *Runtime) newWebhookClient() *retryablehttp.Client {
	c := retryablehttp.NewClient()
	c.RetryMax = rt.config.WebhookMaxRetries - 1
	c.HTTPClient.Timeout = rt.config.WebhookTimeout
	c.Logger = nil // suppress default log output; we log on final failure
	c.CheckRetry = func(ctx context.Context, resp *http.Response, err error) (bool, error) {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		if err != nil {
			return true, nil
		}
		if resp.StatusCode >= http.StatusInternalServerError {
			return true, nil
		}
		return false, nil
	}
	return c
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

	client := rt.newWebhookClient()

	for _, wh := range webhooks {
		if !matchesEvent(wh.events, eventName) {
			continue
		}

		go func(url, secret string) {
			req, err := retryablehttp.NewRequestWithContext(rt.ctx, http.MethodPost, url, bytes.NewReader(body))
			if err != nil {
				rt.app.Logger().Error("webhook request creation failed", "url", url, "error", err)
				return
			}
			req.Header.Set("Content-Type", "application/json")
			if secret != "" {
				mac := hmac.New(sha256.New, []byte(secret))
				mac.Write(body)
				req.Header.Set("X-Turbine-Signature", hex.EncodeToString(mac.Sum(nil)))
			}

			resp, err := client.Do(req)
			if err != nil {
				rt.app.Logger().Warn("webhook delivery failed after retries", "url", url, "error", err)
				return
			}
			_ = resp.Body.Close()

			if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
				rt.app.Logger().Warn("webhook delivery got unexpected status", "url", url, "status", resp.StatusCode)
			}
		}(wh.url, wh.secret)
	}
}
