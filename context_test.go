package turbine

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestFromContext(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	ptCtx := rt.NewContext(context.Background())
	wrapped, cancel := context.WithTimeout(ptCtx, 5*time.Second)
	defer cancel()

	cases := []struct {
		name   string
		ctx    context.Context
		wantOk bool
	}{
		{"with runtime", ptCtx, true},
		{"plain context", context.Background(), false},
		{"wrapped context", wrapped, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := FromContext(tc.ctx)
			if ok != tc.wantOk {
				t.Fatalf("FromContext ok = %v, want %v", ok, tc.wantOk)
			}
			if tc.wantOk && got.App() == nil {
				t.Fatal("expected App() to be non-nil")
			}
			if !tc.wantOk && got != nil {
				t.Fatal("expected nil context")
			}
		})
	}
}

func TestAppFrom(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	cases := []struct {
		name    string
		ctx     context.Context
		wantNil bool
	}{
		{"with runtime", rt.NewContext(context.Background()), false},
		{"plain context", context.Background(), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := AppFrom(tc.ctx)
			if (app == nil) != tc.wantNil {
				t.Fatalf("AppFrom returned nil=%v, want nil=%v", app == nil, tc.wantNil)
			}
		})
	}
}

func TestLoggerFrom(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	cases := []struct {
		name        string
		ctx         context.Context
		wantDefault bool
	}{
		{"plain context", context.Background(), true},
		{"with runtime", rt.NewContext(context.Background()), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logger := LoggerFrom(tc.ctx)
			isDefault := logger == slog.Default()
			if isDefault != tc.wantDefault {
				t.Fatalf("LoggerFrom returned default=%v, want default=%v", isDefault, tc.wantDefault)
			}
		})
	}
}

func TestContextWorkflowID(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	myWF := func(ctx Context, _ string) (string, error) {
		id, err := ctx.WorkflowID()
		if err != nil {
			return "", err
		}
		return id, nil
	}

	Register(rt, myWF)
	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(5 * time.Second)

	handle, err := Run(rt, myWF, "", WithID("wf-id-test"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := handle.GetResult()
	if err != nil {
		t.Fatal(err)
	}
	if result != "wf-id-test" {
		t.Fatalf("expected workflow ID 'wf-id-test', got %q", result)
	}
}

func TestContextWorkflowIDOutsideWorkflow(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	ptCtx := rt.NewContext(context.Background())
	_, err := ptCtx.WorkflowID()
	if err == nil {
		t.Fatal("expected error when calling WorkflowID outside a workflow")
	}
}

func TestSetAppStatus(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	myWF := func(ctx Context, _ string) (string, error) {
		ctx.SetAppStatus("deploying", "blue")
		return "done", nil
	}

	Register(rt, myWF)
	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(5 * time.Second)

	handle, err := Run(rt, myWF, "", WithID("status-test"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := handle.GetResult(); err != nil {
		t.Fatal(err)
	}

	status, err := handle.GetStatus()
	if err != nil {
		t.Fatal(err)
	}
	if status.AppStatus != "deploying" {
		t.Fatalf("expected app status 'deploying', got %q", status.AppStatus)
	}
	if status.AppStatusColor != "blue" {
		t.Fatalf("expected app status color 'blue', got %q", status.AppStatusColor)
	}
}

func TestSetAppStatusStandalone(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	cases := []struct {
		name    string
		ctx     context.Context
		label   string
		color   string
		wantErr bool
	}{
		{"empty label", rt.NewContext(context.Background()), "", "green", true},
		{"no turbine context", context.Background(), "test", "green", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := SetAppStatus(tc.ctx, tc.label, tc.color)
			if (err != nil) != tc.wantErr {
				t.Fatalf("SetAppStatus error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}
