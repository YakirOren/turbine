package pbdbos

import (
	"testing"

	"github.com/pocketbase/pocketbase/tests"
)

func TestEnsureCollections(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	err = ensureCollections(app)
	if err != nil {
		t.Fatal(err)
	}

	// Verify each collection exists
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

func TestEnsureCollectionsIdempotent(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	// Run twice — should not error on second run
	if err := ensureCollections(app); err != nil {
		t.Fatal(err)
	}
	if err := ensureCollections(app); err != nil {
		t.Fatal(err)
	}
}
