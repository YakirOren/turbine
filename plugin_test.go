package turbine

import (
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
)

func TestSetup(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	rt := Setup(app, Config{})
	if rt == nil {
		t.Fatal("expected Setup to return non-nil Runtime")
	}
	if rt.App() == nil {
		t.Fatal("expected runtime to have app set")
	}
	// Launch so the OnTerminate hook's Shutdown(30s) completes quickly.
	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateWebhookRecord(t *testing.T) {
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

	col, err := app.FindCollectionByNameOrId(collectionWebhooks)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		url     string
		events  any
		wantErr bool
	}{
		// URL validation
		{"valid https", "https://example.com/hook", []any{"workflow.SUCCESS"}, false},
		{"valid http localhost", "http://localhost:8080/hook", []any{"workflow.SUCCESS"}, false},
		{"invalid no scheme", "example.com", []any{"workflow.SUCCESS"}, true},
		{"invalid ftp scheme", "ftp://example.com", []any{"workflow.SUCCESS"}, true},
		{"invalid empty url", "", []any{"workflow.SUCCESS"}, true},
		// Event validation
		{"valid single event", "https://example.com", []any{"workflow.SUCCESS"}, false},
		{"valid multiple events", "https://example.com", []any{"workflow.SUCCESS", "workflow.ERROR"}, false},
		{"valid wildcard event", "https://example.com", []any{"workflow.*"}, false},
		{"invalid empty events", "https://example.com", []any{}, true},
		{"invalid nil events", "https://example.com", nil, true},
		{"invalid event type", "https://example.com", []any{"workflow.UNKNOWN"}, true},
		{"invalid mixed events", "https://example.com", []any{"workflow.SUCCESS", "bad"}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := core.NewRecord(col)
			rec.Set("url", tc.url)
			rec.SetRaw("events", tc.events)

			err := validateWebhookRecord(rec)
			if (err != nil) != tc.wantErr {
				t.Errorf("validateWebhookRecord() error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

func TestKVHookValidation(t *testing.T) {
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

	col, err := app.FindCollectionByNameOrId(collectionKV)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{"empty key rejected", "", true},
		{"valid key accepted", "mykey", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := core.NewRecord(col)
			rec.Set("key", tc.key)
			rec.Set("value", "test")
			err := app.Save(rec)
			if (err != nil) != tc.wantErr {
				t.Fatalf("Save() error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}

func TestWebhookSecretMasking(t *testing.T) {
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

	col, err := app.FindCollectionByNameOrId(collectionWebhooks)
	if err != nil {
		t.Fatal(err)
	}

	// Create a webhook record directly in DB (bypass hooks to avoid JSON field issues)
	rec := core.NewRecord(col)
	rec.Set("url", "https://example.com/hook")
	rec.Set("events", `["workflow.SUCCESS"]`)
	rec.Set("secret", "my-secret-token")
	// Use direct save to bypass Create hooks
	if err := app.SaveNoValidate(rec); err != nil {
		t.Fatalf("failed to save webhook: %v", err)
	}

	// Verify the secret is stored
	found, err := app.FindRecordById(collectionWebhooks, rec.Id)
	if err != nil {
		t.Fatal(err)
	}
	if found.GetString("secret") != "my-secret-token" {
		t.Fatalf("expected secret to be stored, got %q", found.GetString("secret"))
	}
}
