package turbine

import (
	"log/slog"
	"os"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// NewApp creates a new PocketBase app and a Runtime wired into its OnServe and
// OnTerminate hooks. The caller registers workflows, then calls app.Start();
// Launch and Shutdown are driven by the app lifecycle.
func NewApp(config Config) (App, *Runtime) {
	app := pocketbase.New()
	return app, Setup(app, config)
}

// NewStandalone creates a Runtime that owns an embedded PocketBase app. Use for
// scripts and background processes that run workflows without serving HTTP.
//
// The returned runtime is constructed but not launched. The caller must:
//
//	rt := turbine.NewStandalone(cfg)
//	defer rt.Shutdown()
//	turbine.Register(rt, MyWorkflow)
//	if err := rt.Launch(); err != nil { log.Fatal(err) }
//
// Workflow and step logs go to stdout by default. Set Config.Logger to override.
func NewStandalone(config Config) *Runtime {
	if config.Logger == nil {
		config.Logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}
	app := pocketbase.New()
	rt := NewRuntime(app, config)
	rt.ownedApp = app
	return rt
}

// SetupStandalone wires turbine into a caller-supplied PocketBase app for
// non-HTTP use. The caller is responsible for Bootstrap (if needed) and must
// invoke rt.Launch and rt.Shutdown themselves. Unlike NewStandalone, this does
// NOT default Config.Logger, the supplied app's logger is used as a fallback
// (via baseLogger) so caller-configured logging is preserved.
func SetupStandalone(app core.App, config Config) *Runtime {
	return NewRuntime(app, config)
}

type (
	App          = *pocketbase.PocketBase
	ServeEvent   = core.ServeEvent
	RequestEvent = core.RequestEvent
)
