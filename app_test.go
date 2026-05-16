package turbine

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/tests"
)

func TestShutdownAfterLaunch(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	rt := NewRuntime(app, Config{ShutdownTimeout: 100 * time.Millisecond})
	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	rt.Shutdown()
}

func TestShutdownBeforeLaunchIsSafe(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	rt := NewRuntime(app, Config{})
	// No Launch. Shutdown must not panic.
	rt.Shutdown()
}

func TestLaunchRunsMigrationsForOwnedApp(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	rt := NewRuntime(app, Config{})
	rt.ownedApp = app

	if err := rt.Launch(); err != nil {
		t.Fatalf("first Launch failed: %v", err)
	}
	rt.Shutdown()
}

func TestLaunchMigrationsAreIdempotent(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	rt1 := NewRuntime(app, Config{})
	if err := rt1.Launch(); err != nil {
		t.Fatalf("first Launch failed: %v", err)
	}
	rt1.Shutdown()

	rt2 := NewRuntime(app, Config{})
	if err := rt2.Launch(); err != nil {
		t.Fatalf("second Launch failed (migration not idempotent?): %v", err)
	}
	rt2.Shutdown()
}

func TestNewStandaloneLifecycle(t *testing.T) {
	rt := NewStandalone(Config{})
	t.Cleanup(rt.Shutdown)

	if rt == nil {
		t.Fatal("NewStandalone returned nil")
	}
	if rt.ownedApp == nil {
		t.Fatal("NewStandalone should set ownedApp")
	}
	if rt.launched.Load() {
		t.Fatal("NewStandalone should return unlaunched runtime")
	}

	Register(rt, func(ctx Context, name string) (string, error) {
		return "hello, " + name, nil
	})

	if err := rt.Launch(); err != nil {
		t.Fatalf("Launch failed: %v", err)
	}
}

func TestNewStandaloneDefaultsLoggerToStdout(t *testing.T) {
	rt := NewStandalone(Config{})
	t.Cleanup(rt.Shutdown)

	if rt.logger == nil {
		t.Fatal("NewStandalone should default cfg.Logger when nil")
	}
}

func TestNewStandaloneRespectsCustomLogger(t *testing.T) {
	var buf bytes.Buffer
	custom := slog.New(slog.NewTextHandler(&buf, nil))

	rt := NewStandalone(Config{Logger: custom})
	t.Cleanup(rt.Shutdown)

	if rt.logger != custom {
		t.Fatal("NewStandalone should preserve caller-supplied Logger")
	}
}

func TestSetupStandaloneDoesNotDefaultLogger(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	rt := SetupStandalone(app, Config{})
	t.Cleanup(rt.Shutdown)

	if rt.ownedApp != nil {
		t.Fatal("SetupStandalone must not set ownedApp")
	}
	if rt.logger != nil {
		t.Fatal("SetupStandalone must not default Logger; baseLogger falls back to app.Logger")
	}
}

func TestLoggerMatrix(t *testing.T) {
	t.Run("NewStandalone defaults to stdout", func(t *testing.T) {
		rt := NewStandalone(Config{})
		t.Cleanup(rt.Shutdown)
		if rt.logger == nil {
			t.Fatal("expected stdout logger, got nil")
		}
	})

	t.Run("NewStandalone respects Config.Logger", func(t *testing.T) {
		var buf bytes.Buffer
		custom := slog.New(slog.NewTextHandler(&buf, nil))
		rt := NewStandalone(Config{Logger: custom})
		t.Cleanup(rt.Shutdown)

		rt.baseLogger().Info("from-runtime")
		if !strings.Contains(buf.String(), "from-runtime") {
			t.Fatalf("expected custom logger to capture 'from-runtime', got: %q", buf.String())
		}
	})

	t.Run("NewApp falls back to app.Logger when Config.Logger nil", func(t *testing.T) {
		app, rt := NewApp(Config{})
		t.Cleanup(rt.Shutdown)
		if rt.logger != nil {
			t.Fatal("NewApp must not default Config.Logger")
		}
		if got := rt.baseLogger(); got == nil || got != app.Logger() {
			t.Fatal("baseLogger should return app.Logger when rt.logger is nil")
		}
	})

	t.Run("NewApp respects Config.Logger", func(t *testing.T) {
		var buf bytes.Buffer
		custom := slog.New(slog.NewTextHandler(&buf, nil))
		_, rt := NewApp(Config{Logger: custom})
		t.Cleanup(rt.Shutdown)
		rt.baseLogger().Info("captured")
		if !strings.Contains(buf.String(), "captured") {
			t.Fatalf("expected custom logger output, got: %q", buf.String())
		}
	})
}
