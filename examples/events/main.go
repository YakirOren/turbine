package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/YakirOren/pbdbos"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func ProcessRequest(ctx context.Context) (bool, error) {
	// process the approved request
	return true, nil
}

// ApprovalWorkflow waits for an external approval signal via events.
func ApprovalWorkflow(ctx context.Context, rt *pbdbos.Runtime, requestID string) (string, error) {
	if err := pbdbos.SetEvent(ctx, rt, "status", "waiting_approval"); err != nil {
		return "", err
	}

	// Wait up to 1 hour for approval
	approved, err := pbdbos.Recv[bool](ctx, rt, "approval", 1*time.Hour)
	if err != nil {
		return "", err
	}

	if !approved {
		pbdbos.SetEvent(ctx, rt, "status", "rejected")
		return fmt.Sprintf("request %s rejected", requestID), nil
	}

	_, err = pbdbos.RunAsStep(ctx, rt, ProcessRequest, pbdbos.WithStepName("process"))
	if err != nil {
		return "", err
	}

	pbdbos.SetEvent(ctx, rt, "status", "completed")
	return fmt.Sprintf("request %s approved and processed", requestID), nil
}

func main() {
	app := pocketbase.New()

	rt := pbdbos.Register(app, pbdbos.Config{})

	pbdbos.RegisterWorkflow(rt, ApprovalWorkflow)

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		e.Router.POST("/request/{id}", func(re *core.RequestEvent) error {
			id := re.Request.PathValue("id")

			handle, err := pbdbos.RunWorkflow(rt, ApprovalWorkflow, id,
				pbdbos.WithWorkflowID("approval-"+id),
			)
			if err != nil {
				return re.JSON(500, map[string]string{"error": err.Error()})
			}

			return re.JSON(202, map[string]string{
				"workflow_id": handle.GetWorkflowID(),
			})
		})

		e.Router.GET("/request/{id}/status", func(re *core.RequestEvent) error {
			id := re.Request.PathValue("id")

			status, err := pbdbos.GetEvent[string](context.Background(), rt, "approval-"+id, "status", 5*time.Second)
			if err != nil {
				return re.JSON(500, map[string]string{"error": err.Error()})
			}

			return re.JSON(200, map[string]string{"status": status})
		})

		e.Router.POST("/request/{id}/approve", func(re *core.RequestEvent) error {
			id := re.Request.PathValue("id")

			if err := pbdbos.Send(context.Background(), rt, "approval-"+id, true, "approval"); err != nil {
				return re.JSON(500, map[string]string{"error": err.Error()})
			}

			return re.JSON(200, map[string]string{"result": "approved"})
		})

		return e.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
