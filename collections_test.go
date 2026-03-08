package pbdbos

import (
	"testing"

	"github.com/pocketbase/pocketbase/tests"
)

func TestMigrationCreatesCollections(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	// Verify each collection exists (created by the init() migration)
	names := []string{
		collectionWorkflowStatus,
		collectionOperationOutputs,
		collectionNotifications,
		collectionWorkflowEvents,
		collectionWorkflowEventsHist,
	}
	for _, name := range names {
		col, err := app.FindCollectionByNameOrId(name)
		if err != nil {
			t.Fatalf("collection %s not found: %v", name, err)
		}
		if col.Name != name {
			t.Fatalf("expected collection name %s, got %s", name, col.Name)
		}
	}
}
