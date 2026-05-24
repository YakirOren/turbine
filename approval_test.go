package turbine

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

func TestWaitForApproval(t *testing.T) {
	tests := []struct {
		name           string
		approvalOpts   []ApprovalOption
		decision       *ApprovalResult
		wantOutput     string
		wantApproved   bool
		wantComment    string
		checkAppStatus bool
	}{
		{
			name:           "Approved",
			decision:       &ApprovalResult{Approved: true, Comment: "looks good"},
			wantOutput:     "approved",
			wantApproved:   true,
			wantComment:    "looks good",
			checkAppStatus: true,
		},
		{
			name:       "Rejected",
			decision:   &ApprovalResult{Approved: false, Comment: "needs changes"},
			wantOutput: "rejected",
		},
		{
			name:         "WithTimeout",
			approvalOpts: []ApprovalOption{WithApprovalTimeout(200 * time.Millisecond)},
			wantOutput:   "timed out",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt, cleanup := setupRuntime(t)
			defer cleanup()

			var captured ApprovalResult

			wf := func(ctx Context, _ string) (string, error) {
				result, err := WaitForApproval(ctx, tt.approvalOpts...)
				if err != nil {
					return "timed out", nil
				}
				captured = result
				if !result.Approved {
					return "rejected", nil
				}
				return "approved", nil
			}

			Register(rt, wf, WithName("approval-"+tt.name))
			if err := rt.Launch(); err != nil {
				t.Fatal(err)
			}
			defer rt.Shutdown()

			handle, err := Run(rt, wf, "test-input")
			if err != nil {
				t.Fatal(err)
			}

			if tt.decision != nil {
				time.Sleep(200 * time.Millisecond)

				if tt.checkAppStatus {
					status, err := handle.GetStatus()
					if err != nil {
						t.Fatal(err)
					}
					if status.AppStatus != "waiting for approval" {
						t.Fatalf("expected app_status 'waiting for approval', got %q", status.AppStatus)
					}
				}

				ctx := rt.NewContext(context.Background())
				if err := Send(ctx, handle.GetWorkflowID(), *tt.decision, approvalTopic); err != nil {
					t.Fatal(err)
				}
			}

			output, err := handle.GetResult()
			if err != nil {
				t.Fatal(err)
			}
			if output != tt.wantOutput {
				t.Fatalf("expected %q, got %q", tt.wantOutput, output)
			}
			if tt.wantApproved && !captured.Approved {
				t.Fatal("expected ApprovalResult.Approved to be true")
			}
			if tt.wantComment != "" && captured.Comment != tt.wantComment {
				t.Fatalf("expected comment %q, got %q", tt.wantComment, captured.Comment)
			}
		})
	}
}

func TestWaitForApproval_Webhook(t *testing.T) {
	rt, cleanup := setupRuntime(t)
	defer cleanup()

	type webhookPayload struct {
		Event      string `json:"event"`
		WorkflowID string `json:"workflow_id"`
		Name       string `json:"name"`
		Status     string `json:"status"`
		Output     any    `json:"output,omitempty"`
		Error      string `json:"error,omitempty"`
		Timestamp  string `json:"timestamp"`
	}
	received := make(chan webhookPayload, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p webhookPayload
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			t.Errorf("failed to decode webhook payload: %v", err)
			return
		}
		received <- p
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	col, err := rt.app.FindCollectionByNameOrId(collectionWebhooks)
	if err != nil {
		t.Fatal(err)
	}
	rec := core.NewRecord(col)
	rec.Set("url", srv.URL)
	rec.Set("events", `["workflow.WAITING_FOR_APPROVAL"]`)
	rec.Set("enabled", true)
	if err := rt.app.SaveNoValidate(rec); err != nil {
		t.Fatal(err)
	}

	wf := func(ctx Context, _ string) (string, error) {
		result, err := WaitForApproval(ctx)
		if err != nil {
			return "", err
		}
		if !result.Approved {
			return "rejected", nil
		}
		return "approved", nil
	}

	Register(rt, wf, WithName("webhook-approval-test"))
	if err := rt.Launch(); err != nil {
		t.Fatal(err)
	}
	defer rt.Shutdown()

	handle, err := Run(rt, wf, "test-input")
	if err != nil {
		t.Fatal(err)
	}

	select {
	case p := <-received:
		if p.Event != "workflow.WAITING_FOR_APPROVAL" {
			t.Fatalf("expected event 'workflow.WAITING_FOR_APPROVAL', got %q", p.Event)
		}
		if p.WorkflowID != handle.GetWorkflowID() {
			t.Fatalf("expected workflow_id %q, got %q", handle.GetWorkflowID(), p.WorkflowID)
		}
		if p.Name != "webhook-approval-test" {
			t.Fatalf("expected name 'webhook-approval-test', got %q", p.Name)
		}
		if p.Status != "WAITING_FOR_APPROVAL" {
			t.Fatalf("expected status 'WAITING_FOR_APPROVAL', got %q", p.Status)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for webhook delivery")
	}

	ctx := rt.NewContext(context.Background())
	if err := Send(ctx, handle.GetWorkflowID(), ApprovalResult{Approved: true}, approvalTopic); err != nil {
		t.Fatal(err)
	}

	output, err := handle.GetResult()
	if err != nil {
		t.Fatal(err)
	}
	if output != "approved" {
		t.Fatalf("expected 'approved', got %q", output)
	}
}
