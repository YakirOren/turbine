package turbine

import (
	"context"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestValidateAlertChannelRecord(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	rt := New(app, Config{})
	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(0)

	col, err := app.FindCollectionByNameOrId(collectionAlertChannels)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		url     string
		events  any
		wantErr bool
	}{
		// URL validation via Shoutrrr
		{"valid slack url", "logger://", []any{"workflow.SUCCESS"}, false},
		{"invalid empty url", "", []any{"workflow.SUCCESS"}, true},
		{"invalid garbage url", "not-a-valid-service://", []any{"workflow.SUCCESS"}, true},
		// Event validation
		{"valid single event", "logger://", []any{"workflow.SUCCESS"}, false},
		{"valid multiple events", "logger://", []any{"workflow.SUCCESS", "workflow.ERROR"}, false},
		{"valid wildcard event", "logger://", []any{"workflow.*"}, false},
		{"invalid empty events", "logger://", []any{}, true},
		{"invalid nil events", "logger://", nil, true},
		{"invalid event type", "logger://", []any{"workflow.UNKNOWN"}, true},
		{"invalid mixed events", "logger://", []any{"workflow.SUCCESS", "bad"}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := core.NewRecord(col)
			rec.Set("url", tc.url)
			rec.SetRaw("events", tc.events)

			err := validateAlertChannelRecord(rec)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateAlertChannelRecord() error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

func TestAlertChannelURLMasking(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	rt := New(app, Config{})
	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(0)

	col, err := app.FindCollectionByNameOrId(collectionAlertChannels)
	if err != nil {
		t.Fatal(err)
	}

	rec := core.NewRecord(col)
	rec.Set("name", "test channel")
	rec.Set("url", "slack://xoxb:token-a-token-b@C12345")
	rec.Set("events", `["workflow.SUCCESS"]`)
	rec.Set("enabled", true)
	if err := app.SaveNoValidate(rec); err != nil {
		t.Fatalf("failed to save alert channel: %v", err)
	}

	// FindRecordById should NOT mask (used by TestAlertChannel)
	found, err := app.FindRecordById(collectionAlertChannels, rec.Id)
	if err != nil {
		t.Fatal(err)
	}
	if found.GetString("url") != "slack://xoxb:token-a-token-b@C12345" {
		t.Fatalf("expected raw URL from FindRecordById, got %q", found.GetString("url"))
	}
}

func TestFormatNotificationMessage(t *testing.T) {
	errMsg := "connection refused"
	cases := []struct {
		name       string
		workflowID string
		wfName     string
		status     StatusType
		errorMsg   *string
		want       string
	}{
		{"success", "abc123", "OrderWorkflow", StatusSuccess, nil,
			`[Turbine] Workflow "OrderWorkflow" (abc123) completed successfully`},
		{"error with message", "abc123", "OrderWorkflow", StatusError, &errMsg,
			`[Turbine] Workflow "OrderWorkflow" (abc123) failed: connection refused`},
		{"error without message", "abc123", "OrderWorkflow", StatusError, nil,
			`[Turbine] Workflow "OrderWorkflow" (abc123) failed`},
		{"cancelled", "abc123", "OrderWorkflow", StatusCancelled, nil,
			`[Turbine] Workflow "OrderWorkflow" (abc123) cancelled`},
		{"waiting for approval", "abc123", "OrderWorkflow", StatusWaitingForApproval, nil,
			`[Turbine] Workflow "OrderWorkflow" (abc123) is waiting for approval`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := formatNotificationMessage(tc.workflowID, tc.wfName, tc.status, tc.errorMsg)
			if got != tc.want {
				t.Errorf("formatNotificationMessage() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExtractScheme(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"slack://xoxb:token@channel", "slack"},
		{"discord://token@webhookid", "discord"},
		{"smtp://user:pass@host:587", "smtp"},
		{"logger://", "logger"},
		{"not-a-url", ""},
		{"", ""},
	}

	for _, tc := range cases {
		t.Run(tc.url, func(t *testing.T) {
			got := extractScheme(tc.url)
			if got != tc.want {
				t.Errorf("extractScheme(%q) = %q, want %q", tc.url, got, tc.want)
			}
		})
	}
}

func TestSendNotification(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	rt := New(app, Config{})
	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(0)

	col, err := app.FindCollectionByNameOrId(collectionAlertChannels)
	if err != nil {
		t.Fatal(err)
	}

	enabled := core.NewRecord(col)
	enabled.Set("name", "ops-alerts")
	enabled.Set("url", "logger://")
	enabled.Set("events", `["workflow.SUCCESS"]`)
	enabled.Set("enabled", true)
	if err := app.SaveNoValidate(enabled); err != nil {
		t.Fatalf("save enabled channel: %v", err)
	}

	disabled := core.NewRecord(col)
	disabled.Set("name", "muted-channel")
	disabled.Set("url", "logger://")
	disabled.Set("events", `["workflow.SUCCESS"]`)
	disabled.Set("enabled", false)
	if err := app.SaveNoValidate(disabled); err != nil {
		t.Fatalf("save disabled channel: %v", err)
	}

	t.Run("by name via runtime method", func(t *testing.T) {
		if err := rt.SendNotification("ops-alerts", "hello"); err != nil {
			t.Errorf("SendNotification() error = %v", err)
		}
	})

	t.Run("via context helper", func(t *testing.T) {
		ctx := rt.NewContext(context.Background())
		if err := SendNotification(ctx, "ops-alerts", "hello via ctx"); err != nil {
			t.Errorf("SendNotification(ctx) error = %v", err)
		}
	})

	t.Run("context helper outside turbine context", func(t *testing.T) {
		if err := SendNotification(context.Background(), "ops-alerts", "x"); err == nil {
			t.Error("expected error when ctx has no runtime")
		}
	})

	t.Run("not found", func(t *testing.T) {
		if err := rt.SendNotification("does-not-exist", "x"); err == nil {
			t.Error("expected error for missing channel")
		}
	})

	t.Run("disabled channel is a silent no-op", func(t *testing.T) {
		if err := rt.SendNotification("muted-channel", "x"); err != nil {
			t.Errorf("expected nil for disabled channel, got %v", err)
		}
	})
}

func TestCancelWorkflowTransitionFlag(t *testing.T) {
	sysDB, cleanup := setupSysDB(t)
	defer cleanup()

	// Cancel a PENDING workflow — should transition
	wfID := "wf-cancel-flag-1"
	input := insertStatusDBInput{status: makeStatus(wfID)}
	if _, err := sysDB.insertStatus(context.Background(), input); err != nil {
		t.Fatal(err)
	}

	changed, err := sysDB.cancelWorkflow(context.Background(), cancelWorkflowDBInput{workflowID: wfID})
	if err != nil {
		t.Fatalf("cancelWorkflow failed: %v", err)
	}
	if !changed {
		t.Fatal("expected cancel of PENDING workflow to transition state")
	}

	// Cancel a non-existent workflow — should return error, changed=false
	changed, err = sysDB.cancelWorkflow(context.Background(), cancelWorkflowDBInput{workflowID: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for non-existent workflow")
	}
	if changed {
		t.Fatal("expected changed=false for non-existent workflow")
	}
}
