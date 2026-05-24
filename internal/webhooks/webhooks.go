// Package webhooks owns turbine's outbound HTTP webhook delivery: cache
// management, SSRF guards, retryable HTTP client, payload formatting,
// signature signing, and the per-record validation used by collection hooks.
package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/YakirOren/turbine/internal/retry"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

// Config tunes the webhook sender.
type Config struct {
	Timeout               time.Duration
	MaxRetries            int
	AllowPrivateAddresses bool
}

// Sender delivers webhooks for workflow events. One Sender per turbine Runtime.
type Sender struct {
	app        core.App
	ctx        context.Context
	logger     *slog.Logger
	cfg        Config
	cache      atomic.Value // []cachedWebhook
	client     *retryablehttp.Client
	collection string
	validEvents map[string]bool
}

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

// NewSender builds a Sender. `collection` is the PocketBase collection holding
// webhook records (pt_webhooks). `validEvents` is the set of event names a
// record's events list is checked against during validation.
func NewSender(ctx context.Context, app core.App, logger *slog.Logger, cfg Config, collection string, validEvents map[string]bool) *Sender {
	s := &Sender{
		app:         app,
		ctx:         ctx,
		logger:      logger,
		cfg:         cfg,
		collection:  collection,
		validEvents: validEvents,
	}
	s.client = s.newHTTPClient()
	return s
}

// ReloadCache reloads the webhook cache from the database. Call this on startup
// and on collection-change hooks so a webhook add/edit takes effect without
// a restart.
func (s *Sender) ReloadCache() {
	records, err := s.app.FindAllRecords(s.collection)
	if err != nil {
		s.app.Logger().Error("failed to load webhooks for cache", "error", err)
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
	s.cache.Store(webhooks)
}

// Dispatch delivers the webhook payload for a single workflow event to every
// matching subscriber. Each delivery runs in its own goroutine; failures are
// logged and do not affect the workflow.
//
// EventMatcher decides which subscribed events match the synthesized event name.
type EventMatcher func(events []string, eventName string) bool

func (s *Sender) Dispatch(workflowID, name, status string, output *string, errorMsg *string, matcher EventMatcher) {
	webhooks, _ := s.cache.Load().([]cachedWebhook)
	if len(webhooks) == 0 {
		return
	}

	eventName := "workflow." + status

	payload := webhookPayload{
		Event:      eventName,
		WorkflowID: workflowID,
		Name:       name,
		Status:     status,
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
		s.app.Logger().Error("failed to marshal webhook payload", "error", err)
		return
	}

	for _, wh := range webhooks {
		if !matcher(wh.events, eventName) {
			continue
		}

		go func(targetURL, secret string) {
			defer retry.RecoverGoroutine(s.app.Logger(), "webhook goroutine panicked", "url", targetURL)
			if !s.cfg.AllowPrivateAddresses {
				if err := validateOutboundURL(targetURL); err != nil {
					s.app.Logger().Warn("webhook delivery refused", "url", targetURL, "error", err)
					return
				}
			}
			req, err := retryablehttp.NewRequestWithContext(s.ctx, http.MethodPost, targetURL, bytes.NewReader(body))
			if err != nil {
				s.app.Logger().Error("webhook request creation failed", "url", targetURL, "error", err)
				return
			}
			req.Header.Set("Content-Type", "application/json")
			if secret != "" {
				mac := hmac.New(sha256.New, []byte(secret))
				mac.Write(body)
				req.Header.Set("X-Turbine-Signature", hex.EncodeToString(mac.Sum(nil)))
			}

			resp, err := s.client.Do(req)
			if err != nil {
				s.app.Logger().Warn("webhook delivery failed after retries", "url", targetURL, "error", err)
				return
			}
			_ = resp.Body.Close()

			if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
				s.app.Logger().Warn("webhook delivery got unexpected status", "url", targetURL, "status", resp.StatusCode)
			}
		}(wh.url, wh.secret)
	}
}

// ValidateRecord validates a webhook record. When AllowPrivateAddresses is
// false, it also blocks loopback / link-local / RFC1918 / CGNAT hosts to
// prevent SSRF reach into internal services (cloud metadata, internal Redis,
// etc.).
func (s *Sender) ValidateRecord(r *core.Record) error {
	return ValidateRecord(r, s.cfg.AllowPrivateAddresses, s.validEvents)
}

// ValidateRecord is the package-level form for callers that don't yet have a
// Sender (e.g. PocketBase hook registration during runtime construction).
func ValidateRecord(r *core.Record, allowPrivate bool, validEvents map[string]bool) error {
	raw := r.GetString("url")
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return router.NewBadRequestError("invalid webhook URL: must be http or https", nil)
	}
	if !allowPrivate {
		if err := validateOutboundURL(raw); err != nil {
			return router.NewBadRequestError("invalid webhook URL: "+err.Error(), nil)
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

func (s *Sender) newHTTPClient() *retryablehttp.Client {
	c := retryablehttp.NewClient()
	c.RetryMax = s.cfg.MaxRetries - 1
	c.HTTPClient.Timeout = s.cfg.Timeout
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

	if !s.cfg.AllowPrivateAddresses {
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

// errPrivateAddress is returned when a webhook URL resolves to or redirects to
// a loopback, link-local, or RFC1918 address. Blocks SSRF reach into internal
// services (cloud metadata, internal Redis, etc.).
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

// RejectPrivateHost returns an error if host is an IP literal in a private/
// loopback range or a well-known DNS alias for loopback. Shared between
// http(s) webhook URLs and shoutrrr generic:// alert-channel URLs.
func RejectPrivateHost(host string) error {
	if ip := net.ParseIP(host); ip != nil && isPrivateOrLoopbackIP(ip) {
		return errPrivateAddress
	}
	switch strings.ToLower(host) {
	case "localhost", "ip6-localhost", "ip6-loopback":
		return errPrivateAddress
	}
	return nil
}

// validateOutboundURL parses a URL, requires http/https, and rejects hosts
// whose IP literal is private/loopback. DNS-resolved hostnames are validated
// at dial time by checkOutboundAddr below.
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
	return RejectPrivateHost(host)
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
