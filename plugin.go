package turbine

import (
	"net/url"

	"github.com/YakirOren/turbine/internal/notifications"
	"github.com/YakirOren/turbine/internal/webhooks"
	"github.com/pocketbase/pocketbase/core"
)

var validDispatchEvents = map[string]bool{
	"workflow.SUCCESS":                        true,
	"workflow.ERROR":                          true,
	"workflow.CANCELLED":                      true,
	"workflow.WAITING_FOR_APPROVAL":           true,
	"workflow.MAX_RECOVERY_ATTEMPTS_EXCEEDED": true,
	"workflow.*":                              true,
}

// Setup wires turbine into an existing app's lifecycle.
// Returns the Runtime so you can register workflows before app.Start().
func Setup(app core.App, config Config) *Runtime {
	rt := NewRuntime(app, config)

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		if err := rt.Launch(); err != nil {
			return err
		}
		return e.Next()
	})

	app.OnTerminate().BindFunc(func(e *core.TerminateEvent) error {
		rt.Shutdown()
		return e.Next()
	})

	registerWebhookHooks(app, rt.config.AllowPrivateAddresses)
	registerAlertChannelHooks(app, rt.config.AllowPrivateAddresses)

	return rt
}

// validateWebhookRecord delegates to webhooks.ValidateRecord, kept as a thin
// adapter so the existing PocketBase hooks and tests don't need to thread the
// allowed-events map through every call site.
func validateWebhookRecord(r *core.Record, allowPrivate bool) error {
	return webhooks.ValidateRecord(r, allowPrivate, validDispatchEvents)
}

func registerWebhookHooks(app core.App, allowPrivate bool) {
	app.OnRecordCreate(collectionWebhooks).BindFunc(func(e *core.RecordEvent) error {
		if err := validateWebhookRecord(e.Record, allowPrivate); err != nil {
			return err
		}
		return e.Next()
	})

	app.OnRecordUpdate(collectionWebhooks).BindFunc(func(e *core.RecordEvent) error {
		if err := validateWebhookRecord(e.Record, allowPrivate); err != nil {
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

// validateAlertChannelRecord delegates to notifications.ValidateRecord.
func validateAlertChannelRecord(r *core.Record, allowPrivate bool) error {
	return notifications.ValidateRecord(r, allowPrivate, validDispatchEvents)
}

func registerAlertChannelHooks(app core.App, allowPrivate bool) {
	app.OnRecordCreate(collectionAlertChannels).BindFunc(func(e *core.RecordEvent) error {
		if err := validateAlertChannelRecord(e.Record, allowPrivate); err != nil {
			return err
		}
		return e.Next()
	})

	app.OnRecordUpdate(collectionAlertChannels).BindFunc(func(e *core.RecordEvent) error {
		if err := validateAlertChannelRecord(e.Record, allowPrivate); err != nil {
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
