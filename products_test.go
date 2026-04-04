package turbine

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/tests"
)

type mockProductSender struct {
	called  bool
	product ProductRecord
	err     error
}

func (m *mockProductSender) Send(_ context.Context, product ProductRecord) error {
	m.called = true
	m.product = product
	return m.err
}

func setupRuntimeWithSender(t *testing.T, sender ProductSender) (*Runtime, func()) {
	t.Helper()
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{}
	if sender != nil {
		cfg.ProductSender = sender
	}
	rt := New(app, cfg)
	return rt, app.Cleanup
}

func TestSendProduct_Store(t *testing.T) {
	rt, cleanup := setupRuntimeWithSender(t, nil)
	defer cleanup()

	myWF := func(ctx Context, input string) (string, error) {
		_, err := Do(ctx, func(ctx context.Context) (any, error) {
			return nil, SendProduct(ctx, "report.pdf", strings.NewReader("fake pdf data"), map[string]any{"key": "value"})
		}, WithStepName("send-product"))
		if err != nil {
			return "", err
		}
		return "done", nil
	}

	Register(rt, myWF)
	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(5 * time.Second)

	handle, err := Run(rt, myWF, "test")
	if err != nil {
		t.Fatal(err)
	}
	result, err := handle.GetResult()
	if err != nil {
		t.Fatalf("GetResult failed: %v", err)
	}
	if result != "done" {
		t.Fatalf("expected 'done', got %q", result)
	}

	// Verify product record exists
	records, err := rt.app.FindRecordsByFilter(collectionProducts, "file_name = 'report.pdf'", "", 1, 0)
	if err != nil {
		t.Fatalf("failed to find product: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 product record, got %d", len(records))
	}

	rec := records[0]
	if rec.GetString("workflow_id") != handle.GetWorkflowID() {
		t.Fatalf("expected workflow_id %q, got %q", handle.GetWorkflowID(), rec.GetString("workflow_id"))
	}
	if rec.GetString("status") != "stored" {
		t.Fatalf("expected status 'stored', got %q", rec.GetString("status"))
	}
	if rec.GetString("file_name") != "report.pdf" {
		t.Fatalf("expected file_name 'report.pdf', got %q", rec.GetString("file_name"))
	}
}

func TestSendProduct_WithSender(t *testing.T) {
	sender := &mockProductSender{}
	rt, cleanup := setupRuntimeWithSender(t, sender)
	defer cleanup()

	myWF := func(ctx Context, input string) (string, error) {
		_, err := Do(ctx, func(ctx context.Context) (any, error) {
			return nil, SendProduct(ctx, "output.csv", strings.NewReader("col1,col2\na,b"), map[string]any{"type": "csv"})
		}, WithStepName("send-product"))
		if err != nil {
			return "", err
		}
		return "done", nil
	}

	Register(rt, myWF)
	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(5 * time.Second)

	handle, err := Run(rt, myWF, "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := handle.GetResult(); err != nil {
		t.Fatalf("GetResult failed: %v", err)
	}

	// Verify sender was called
	if !sender.called {
		t.Fatal("expected sender to be called")
	}
	if sender.product.FileName != "output.csv" {
		t.Fatalf("expected file_name 'output.csv', got %q", sender.product.FileName)
	}

	// Verify status is "sent"
	records, err := rt.app.FindRecordsByFilter(collectionProducts, "file_name = 'output.csv'", "", 1, 0)
	if err != nil {
		t.Fatalf("failed to find product: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 product record, got %d", len(records))
	}
	if records[0].GetString("status") != "sent" {
		t.Fatalf("expected status 'sent', got %q", records[0].GetString("status"))
	}
}

func TestSendProduct_SenderFails(t *testing.T) {
	sender := &mockProductSender{err: fmt.Errorf("upload failed")}
	rt, cleanup := setupRuntimeWithSender(t, sender)
	defer cleanup()

	var sendErr error
	myWF := func(ctx Context, input string) (string, error) {
		_, err := Do(ctx, func(ctx context.Context) (any, error) {
			sendErr = SendProduct(ctx, "fail.txt", strings.NewReader("data"), nil)
			return nil, sendErr
		}, WithStepName("send-product"))
		if err != nil {
			return "", err
		}
		return "done", nil
	}

	Register(rt, myWF)
	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(5 * time.Second)

	handle, err := Run(rt, myWF, "test")
	if err != nil {
		t.Fatal(err)
	}
	_, err = handle.GetResult()
	if err == nil {
		t.Fatal("expected error from sender failure")
	}

	// Verify product status is "failed" with error message
	records, err := rt.app.FindRecordsByFilter(collectionProducts, "file_name = 'fail.txt'", "", 1, 0)
	if err != nil {
		t.Fatalf("failed to find product: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 product record, got %d", len(records))
	}
	rec := records[0]
	if rec.GetString("status") != "failed" {
		t.Fatalf("expected status 'failed', got %q", rec.GetString("status"))
	}
	if rec.GetString("error") != "upload failed" {
		t.Fatalf("expected error 'upload failed', got %q", rec.GetString("error"))
	}
}

func TestWorkflowSender(t *testing.T) {
	app, err := tests.NewTestApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Cleanup()

	rt := New(app, Config{})

	targetWF := func(ctx Context, input ProductRecord) (string, error) {
		return "received:" + input.FileName, nil
	}

	Register(rt, targetWF)

	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(5 * time.Second)

	sender := NewWorkflowSender(rt, targetWF)

	product := ProductRecord{
		ID:       "test-id",
		FileName: "report.pdf",
		Metadata: map[string]any{"key": "value"},
		FileURL:  "/api/files/test/test-id/report.pdf",
	}

	if err := sender.Send(context.Background(), product); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify the target workflow was started by checking pt_workflow_status
	records, err := rt.app.FindRecordsByFilter(collectionStatus, "name != ''", "", 10, 0)
	if err != nil {
		t.Fatalf("failed to query workflow status: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected at least one workflow status record from WorkflowSender")
	}
}

func TestSendProduct_Dedup(t *testing.T) {
	rt, cleanup := setupRuntimeWithSender(t, nil)
	defer cleanup()

	myWF := func(ctx Context, input string) (string, error) {
		_, err := Do(ctx, func(ctx context.Context) (any, error) {
			err1 := SendProduct(ctx, "dup.txt", strings.NewReader("data1"), nil)
			if err1 != nil {
				return nil, err1
			}
			// Second call with same file_name — should be a no-op
			err2 := SendProduct(ctx, "dup.txt", strings.NewReader("data2"), nil)
			if err2 != nil {
				return nil, err2
			}
			return nil, nil
		}, WithStepName("send-product"))
		if err != nil {
			return "", err
		}
		return "done", nil
	}

	Register(rt, myWF)
	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown(5 * time.Second)

	handle, err := Run(rt, myWF, "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := handle.GetResult(); err != nil {
		t.Fatalf("GetResult failed: %v", err)
	}

	// Verify only one record exists
	records, err := rt.app.FindRecordsByFilter(collectionProducts, "workflow_id = '"+handle.GetWorkflowID()+"'", "", 10, 0)
	if err != nil {
		t.Fatalf("failed to find products: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 product record (dedup), got %d", len(records))
	}
}
