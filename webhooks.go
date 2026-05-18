package turbine

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"
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

// errPrivateAddress is returned when a webhook URL resolves to or redirects to
// a loopback, link-local, or RFC1918 address. Blocks SSRF reach into internal services
// (cloud metadata, internal Redis, etc.).
var errPrivateAddress = errors.New("turbine: refusing to dispatch to private/loopback address")

// isPrivateOrLoopbackIP reports whether the IP belongs to a range that the
// webhook dispatcher must refuse to talk to.
func isPrivateOrLoopbackIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsUnspecified() || ip.IsPrivate() {
		return true
	}
	// 100.64.0.0/10 (CGNAT, RFC6598). net.IP.IsPrivate does not cover this.
	if v4 := ip.To4(); v4 != nil && v4[0] == 100 && v4[1]&0xC0 == 64 {
		return true
	}
	return false
}

// validateOutboundURL parses a URL, requires http/https, and rejects hosts
// whose IP literal is private/loopback. DNS-resolved hostnames are validated
// at dial time by checkOutboundAddr below; this catches the obvious "http://127.0.0.1/" case
// at configuration time.
func validateOutboundURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("invalid URL: scheme must be http or https")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("invalid URL: host is empty")
	}
	if ip := net.ParseIP(host); ip != nil && isPrivateOrLoopbackIP(ip) {
		return errPrivateAddress
	}
	// Reject obvious DNS aliases for loopback. DNS-level rebinding is caught at dial time.
	switch strings.ToLower(host) {
	case "localhost", "ip6-localhost", "ip6-loopback":
		return errPrivateAddress
	}
	return nil
}

// checkOutboundAddr is plugged into http.Transport.DialContext to reject
// connections to private IPs even after DNS resolution (defeats DNS-rebinding
// and CNAME tricks that point public names at private ranges).
func checkOutboundAddr(network, addr string) error {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return nil
	}
	if isPrivateOrLoopbackIP(ip) {
		return errPrivateAddress
	}
	return nil
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

	if !rt.config.AllowPrivateAddresses {
		dialer := &net.Dialer{Timeout: 10 * time.Second}
		transport := &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				if err := checkOutboundAddr(network, addr); err != nil {
					return nil, err
				}
				return dialer.DialContext(ctx, network, addr)
			},
		}
		c.HTTPClient.Transport = transport

		// Reject redirects to private/loopback addresses (defeats public->private redirect SSRF).
		c.HTTPClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			return validateOutboundURL(req.URL.String())
		}
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
			defer func() {
				if r := recover(); r != nil {
					rt.app.Logger().Error("webhook goroutine panicked",
						"url", url,
						"panic", r,
						"stack", string(debug.Stack()),
						"source", "system")
				}
			}()
			if !rt.config.AllowPrivateAddresses {
				if err := validateOutboundURL(url); err != nil {
					rt.app.Logger().Warn("webhook delivery refused", "url", url, "error", err)
					return
				}
			}
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
