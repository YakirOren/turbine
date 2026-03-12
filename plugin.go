package pocketflow

import (
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// Register hooks pocketflow into PocketBase's lifecycle as a plugin.
// Returns the Runtime so you can register workflows before app.Start().
func Register(app core.App, config Config) *Runtime {
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
