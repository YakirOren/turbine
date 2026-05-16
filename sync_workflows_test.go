package turbine

import (
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

func TestSyncRegisteredWorkflows_AllFields(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	schema := map[string]any{
		"fields": []map[string]any{
			{"name": "url", "type": "string", "required": true},
		},
	}

	wf := func(ctx Context, input time.Time) (string, error) { return input.String(), nil }
	Register(rt, wf,
		WithName("full-wf"),
		WithDashboardTrigger(),
		WithSchedule("*/5 * * * *"),
		WithInputSchema(schema),
		WithTags("deploy", "critical"),
	)

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rt.Shutdown() }()

	records, err := rt.app.FindAllRecords(collectionWorkflows)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	r := records[0]
	fqn := resolveWorkflowFunctionName(wf)

	if r.GetString("name") != "full-wf" {
		t.Errorf("name: got %q, want %q", r.GetString("name"), "full-wf")
	}
	if r.GetString("fqn") != fqn {
		t.Errorf("fqn: got %q, want %q", r.GetString("fqn"), fqn)
	}
	if !r.GetBool("triggerable") {
		t.Error("expected triggerable to be true")
	}
	if r.GetString("cron_schedule") != "*/5 * * * *" {
		t.Errorf("cron_schedule: got %q, want %q", r.GetString("cron_schedule"), "*/5 * * * *")
	}

	var gotSchema map[string]any
	if err := r.UnmarshalJSONField("input_schema", &gotSchema); err != nil {
		t.Fatalf("failed to unmarshal input_schema: %v", err)
	}
	if gotSchema == nil {
		t.Fatal("expected input_schema to be set")
	}
	if _, ok := gotSchema["fields"]; !ok {
		t.Fatal("expected 'fields' key in input_schema")
	}

	var tags []string
	if err := r.UnmarshalJSONField("tags", &tags); err != nil {
		t.Fatalf("failed to unmarshal tags: %v", err)
	}
	if len(tags) != 2 || tags[0] != "deploy" || tags[1] != "critical" {
		t.Errorf("tags: got %v, want [deploy critical]", tags)
	}
}

func TestSyncRegisteredWorkflows_NameFallbackToFQN(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	wf := func(ctx Context, input string) (string, error) { return input, nil }
	Register(rt, wf) // no WithName

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rt.Shutdown() }()

	records, err := rt.app.FindAllRecords(collectionWorkflows)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	fqn := resolveWorkflowFunctionName(wf)
	if records[0].GetString("name") != fqn {
		t.Errorf("expected name to fall back to FQN %q, got %q", fqn, records[0].GetString("name"))
	}
}

func TestSyncRegisteredWorkflows_Upsert(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	wf := func(ctx Context, input string) (string, error) { return input, nil }
	Register(rt, wf, WithName("upsert-wf"), WithTags("v1"))

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rt.Shutdown() }()

	records, _ := rt.app.FindAllRecords(collectionWorkflows)
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	originalID := records[0].Id

	// Modify the registry entry to simulate a code change.
	fqn := resolveWorkflowFunctionName(wf)
	rt.workflowRegistry.Store(fqn, workflowRegistryEntry{
		FQN:         fqn,
		Name:        "upsert-wf",
		Triggerable: true,
		Tags:        []string{"v2", "updated"},
	})

	if err := rt.syncRegisteredWorkflows(); err != nil {
		t.Fatal(err)
	}

	records, _ = rt.app.FindAllRecords(collectionWorkflows)
	if len(records) != 1 {
		t.Fatalf("expected 1 record after upsert, got %d (duplicate created?)", len(records))
	}

	r := records[0]
	if r.Id != originalID {
		t.Error("expected same record ID after upsert")
	}
	if !r.GetBool("triggerable") {
		t.Error("expected triggerable to be updated to true")
	}
	var tags []string
	if err := r.UnmarshalJSONField("tags", &tags); err != nil {
		t.Fatalf("failed to unmarshal tags: %v", err)
	}
	if len(tags) != 2 || tags[0] != "v2" {
		t.Errorf("expected updated tags [v2 updated], got %v", tags)
	}
}

func TestSyncRegisteredWorkflows_Idempotent(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	wf := func(ctx Context, input string) (string, error) { return input, nil }
	Register(rt, wf, WithName("idem-wf"))

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rt.Shutdown() }()

	records, _ := rt.app.FindAllRecords(collectionWorkflows)
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	originalID := records[0].Id

	// Sync again with no changes.
	if err := rt.syncRegisteredWorkflows(); err != nil {
		t.Fatal(err)
	}

	records, _ = rt.app.FindAllRecords(collectionWorkflows)
	if len(records) != 1 {
		t.Fatalf("expected 1 record after re-sync, got %d", len(records))
	}
	if records[0].Id != originalID {
		t.Error("record ID changed after idempotent sync")
	}
}

func TestSyncRegisteredWorkflows_NilOptionalFields(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	wf := func(ctx Context, input string) (string, error) { return input, nil }
	Register(rt, wf, WithName("bare-wf")) // no tags, schema, schedule, or trigger

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rt.Shutdown() }()

	records, err := rt.app.FindAllRecords(collectionWorkflows)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	r := records[0]
	if r.GetBool("triggerable") {
		t.Error("expected triggerable false")
	}
	if r.GetString("cron_schedule") != "" {
		t.Errorf("expected empty cron_schedule, got %q", r.GetString("cron_schedule"))
	}

	var schema map[string]any
	if err := r.UnmarshalJSONField("input_schema", &schema); err != nil {
		t.Fatalf("failed to unmarshal input_schema: %v", err)
	}
	if schema != nil {
		t.Errorf("expected nil input_schema, got %v", schema)
	}

	var tags []string
	if err := r.UnmarshalJSONField("tags", &tags); err != nil {
		t.Fatalf("failed to unmarshal tags: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("expected empty tags, got %v", tags)
	}
}

func TestSyncRegisteredWorkflows_RemovesStale(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	wf := func(ctx Context, input string) (string, error) { return input, nil }
	Register(rt, wf, WithName("keep-me"))

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rt.Shutdown() }()

	// Insert a stale record manually.
	col, err := rt.app.FindCollectionByNameOrId(collectionWorkflows)
	if err != nil {
		t.Fatal(err)
	}
	stale := core.NewRecord(col)
	stale.Set("name", "stale-wf")
	stale.Set("fqn", "github.com/fake/stale-wf")
	stale.Set("triggerable", false)
	if err := rt.app.Save(stale); err != nil {
		t.Fatal(err)
	}

	// Re-sync should remove the stale record.
	if err := rt.syncRegisteredWorkflows(); err != nil {
		t.Fatal(err)
	}

	records, err := rt.app.FindAllRecords(collectionWorkflows)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record after stale removal, got %d", len(records))
	}
	if records[0].GetString("name") != "keep-me" {
		t.Fatalf("expected keep-me, got %s", records[0].GetString("name"))
	}
}
