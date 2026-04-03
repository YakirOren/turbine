package turbine

import (
	"context"
	"fmt"
	"io"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

// WorkflowSender is a built-in ProductSender that forwards products to another workflow.
type WorkflowSender[R any] struct {
	rt       *Runtime
	workflow Workflow[ProductRecord, R]
}

// NewWorkflowSender creates a sender that starts the target workflow with ProductRecord as input.
func NewWorkflowSender[R any](rt *Runtime, workflow Workflow[ProductRecord, R]) *WorkflowSender[R] {
	return &WorkflowSender[R]{rt: rt, workflow: workflow}
}

func (ws *WorkflowSender[R]) Send(ctx context.Context, product ProductRecord) error {
	_, err := Run(ws.rt, ws.workflow, product)
	return err
}

// SendProduct stores a product file and optionally sends it via the registered ProductSender.
// Must be called from within a step (requires workflow context).
// Returns error if the sender fails — the product is still stored with "failed" status.
func SendProduct(ctx context.Context, fileName string, data io.Reader, metadata map[string]any) error {
	rt := runtimeFromContext(ctx)
	if rt == nil {
		return fmt.Errorf("turbine: SendProduct called outside of a turbine context")
	}
	wfState, ok := ctx.Value(workflowStateKey).(*workflowState)
	if !ok || wfState == nil {
		return fmt.Errorf("turbine: SendProduct called outside of a workflow")
	}
	if !wfState.isWithinStep {
		return fmt.Errorf("turbine: SendProduct must be called from within a step")
	}

	workflowID := wfState.workflowID
	functionID := int(wfState.stepID.Load())

	// Deduplication check via raw query (unique index will also enforce this)
	col, err := rt.app.FindCollectionByNameOrId(collectionProducts)
	if err != nil {
		return fmt.Errorf("turbine: products collection not found: %w", err)
	}

	var existingID string
	dupErr := rt.app.DB().NewQuery(
		"SELECT id FROM " + collectionProducts + " WHERE workflow_id = {:wid} AND function_id = {:fid} AND file_name = {:fn} LIMIT 1",
	).Bind(dbx.Params{"wid": workflowID, "fid": functionID, "fn": fileName}).Row(&existingID)
	if dupErr == nil && existingID != "" {
		return nil // Already stored — dedup
	}

	// Look up step name from operation outputs
	var functionName string
	_ = rt.app.DB().NewQuery(
		"SELECT function_name FROM " + collectionOperationOutputs + " WHERE workflow_id = {:wid} AND function_id = {:fid} LIMIT 1",
	).Bind(dbx.Params{"wid": workflowID, "fid": functionID}).Row(&functionName)

	// Create record
	record := core.NewRecord(col)
	record.Set("workflow_id", workflowID)
	record.Set("function_id", functionID)
	record.Set("function_name", functionName)
	record.Set("file_name", fileName)
	record.Set("metadata", metadata)
	record.Set("status", "stored")

	// Attach file
	fileBytes, err := io.ReadAll(data)
	if err != nil {
		return fmt.Errorf("turbine: failed to read product data: %w", err)
	}
	f, err := filesystem.NewFileFromBytes(fileBytes, fileName)
	if err != nil {
		return fmt.Errorf("turbine: failed to create file: %w", err)
	}
	record.Set("file", f)
	record.Set("size", len(fileBytes))

	if err := rt.app.Save(record); err != nil {
		return fmt.Errorf("turbine: failed to save product: %w", err)
	}

	// Send if sender configured
	if rt.productSender == nil {
		return nil
	}

	productRecord := ProductRecord{
		ID:       record.Id,
		FileName: fileName,
		Metadata: metadata,
		FileURL:  fmt.Sprintf("/api/files/%s/%s/%s", collectionProducts, record.Id, record.GetString("file")),
	}

	sendErr := rt.productSender.Send(ctx, productRecord)
	if sendErr != nil {
		record.Set("status", "failed")
		record.Set("error", sendErr.Error())
		if saveErr := rt.app.Save(record); saveErr != nil {
			rt.app.Logger().Error("failed to update product status to failed", "product_id", record.Id, "error", saveErr, "source", "system")
		}
		return sendErr
	}

	record.Set("status", "sent")
	if saveErr := rt.app.Save(record); saveErr != nil {
		rt.app.Logger().Error("failed to update product status to sent", "product_id", record.Id, "error", saveErr, "source", "system")
	}
	return nil
}
