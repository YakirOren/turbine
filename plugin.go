package turbine

import (
	"time"

	"github.com/pocketbase/pocketbase/core"
)

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

	return rt
}
