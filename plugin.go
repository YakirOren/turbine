package turbine

import (
	"net/url"
	"time"

	"github.com/nicholas-fedor/shoutrrr"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

var validDispatchEvents = map[string]bool{
	"workflow.SUCCESS":                        true,
	"workflow.ERROR":                          true,
	"workflow.CANCELLED":                      true,
	"workflow.WAITING_FOR_APPROVAL":           true,
	"workflow.MAX_RECOVERY_ATTEMPTS_EXCEEDED": true,
	"workflow.*":                              true,
}

// Setup hooks turbine into PocketBase's lifecycle as a plugin.
// Returns the Runtime so you can register workflows before app.Start().
func Setup(app core.App, config Config) *Runtime {
	rt := New(app, config)

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		if err := rt.Launch(); err != nil {
			return err
		}
		return e.Next()
	})

	app.OnTerminate().BindFunc(func(e *core.TerminateEvent) error {
		rt.Shutdown(30 * time.Second)
		return e.Next()
	})

	registerWebhookHooks(app)
	registerAlertChannelHooks(app)
	registerKVHooks(app)

	return rt
}

func validateWebhookRecord(r *core.Record) error {
	// Validate URL
	raw := r.GetString("url")
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return router.NewBadRequestError("invalid webhook URL — must be http or https", nil)
	}

	// Validate events
	var events []string
	raw2 := r.Get("events")
	switch v := raw2.(type) {
	case []string:
		events = v
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				events = append(events, s)
			}
		}
	}

	if len(events) == 0 {
		return router.NewBadRequestError("at least one event is required", nil)
	}

	for _, ev := range events {
		if !validDispatchEvents[ev] {
			return router.NewBadRequestError("invalid event type: "+ev, nil)
		}
	}

	return nil
}

func registerKVHooks(app core.App) {
	app.OnRecordCreate(collectionKV).BindFunc(func(e *core.RecordEvent) error {
		if e.Record.GetString("key") == "" {
			return router.NewBadRequestError("key must not be empty", nil)
		}
		return e.Next()
	})

	app.OnRecordUpdate(collectionKV).BindFunc(func(e *core.RecordEvent) error {
		if e.Record.GetString("key") == "" {
			return router.NewBadRequestError("key must not be empty", nil)
		}
		return e.Next()
	})
}

func registerWebhookHooks(app core.App) {
	app.OnRecordCreate(collectionWebhooks).BindFunc(func(e *core.RecordEvent) error {
		if err := validateWebhookRecord(e.Record); err != nil {
			return err
		}
		return e.Next()
	})

	app.OnRecordUpdate(collectionWebhooks).BindFunc(func(e *core.RecordEvent) error {
		if err := validateWebhookRecord(e.Record); err != nil {
			return err
		}
		return e.Next()
	})

	app.OnRecordEnrich(collectionWebhooks).BindFunc(func(e *core.RecordEnrichEvent) error {
		if e.Record.GetString("secret") != "" {
			e.Record.Set("secret", "***")
		}
		return e.Next()
	})
}

func validateAlertChannelRecord(r *core.Record) error {
	rawURL := r.GetString("url")
	if rawURL == "" {
		return router.NewBadRequestError("URL is required", nil)
	}

	// Validate URL by attempting to create a Shoutrrr sender
	_, err := shoutrrr.CreateSender(rawURL)
	if err != nil {
		return router.NewBadRequestError("invalid notification URL: "+err.Error(), nil)
	}

	// Validate events
	var events []string
	eventsRaw := r.Get("events")
	switch v := eventsRaw.(type) {
	case []string:
		events = v
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				events = append(events, s)
			}
		}
	}

	if len(events) == 0 {
		return router.NewBadRequestError("at least one event is required", nil)
	}

	for _, ev := range events {
		if !validDispatchEvents[ev] {
			return router.NewBadRequestError("invalid event type: "+ev, nil)
		}
	}

	return nil
}

func registerAlertChannelHooks(app core.App) {
	app.OnRecordCreate(collectionAlertChannels).BindFunc(func(e *core.RecordEvent) error {
		if err := validateAlertChannelRecord(e.Record); err != nil {
			return err
		}
		return e.Next()
	})

	app.OnRecordUpdate(collectionAlertChannels).BindFunc(func(e *core.RecordEvent) error {
		if err := validateAlertChannelRecord(e.Record); err != nil {
			return err
		}
		return e.Next()
	})

	app.OnRecordEnrich(collectionAlertChannels).BindFunc(func(e *core.RecordEnrichEvent) error {
		rawURL := e.Record.GetString("url")
		if rawURL != "" {
			parsed, err := url.Parse(rawURL)
			if err == nil {
				e.Record.Set("url", parsed.Scheme+"://***")
			} else {
				e.Record.Set("url", "***")
			}
		}
		return e.Next()
	})
}
