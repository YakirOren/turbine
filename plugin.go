package turbine

import (
	"net/url"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

var validWebhookEvents = map[string]bool{
	"workflow.SUCCESS":   true,
	"workflow.ERROR":     true,
	"workflow.CANCELLED": true,
	"workflow.*":         true,
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
		if !validWebhookEvents[ev] {
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
