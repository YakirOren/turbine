package main

import (
	"context"
	"log"
	"time"

	"github.com/YakirOren/pbdbos"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// LongRunningJob sleeps for a long time — can be cancelled and resumed.
func LongRunningJob(ctx context.Context, rt *pbdbos.Runtime, jobID string) (string, error) {
	if err := pbdbos.Sleep(ctx, rt, 1*time.Hour); err != nil {
		return "", err
	}

	_, err := pbdbos.RunAsStep(ctx, rt, func(ctx context.Context) (bool, error) {
		return true, nil
	}, pbdbos.WithStepName("process"))
	if err != nil {
		return "", err
	}

	return "job " + jobID + " completed", nil
}

func main() {
	app := pocketbase.New()

	rt := pbdbos.Register(app, pbdbos.Config{})

	pbdbos.RegisterWorkflow(rt, LongRunningJob)

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// Start a job with a deterministic ID
		e.Router.POST("/job/{id}", func(re *core.RequestEvent) error {
			id := re.Request.PathValue("id")

			handle, err := pbdbos.RunWorkflow(rt, LongRunningJob, id,
				pbdbos.WithWorkflowID("job-"+id),
			)
			if err != nil {
				return re.JSON(500, map[string]string{"error": err.Error()})
			}

			return re.JSON(202, map[string]string{
				"workflow_id": handle.GetWorkflowID(),
			})
		})

		// Get job status and steps
		e.Router.GET("/job/{id}", func(re *core.RequestEvent) error {
			id := re.Request.PathValue("id")
			wfID := "job-" + id

			handle := pbdbos.RetrieveWorkflow[string](rt, wfID)

			status, err := handle.GetStatus()
			if err != nil {
				return re.JSON(500, map[string]string{"error": err.Error()})
			}

			steps, err := pbdbos.GetWorkflowSteps(rt, wfID)
			if err != nil {
				return re.JSON(500, map[string]string{"error": err.Error()})
			}

			stepNames := make([]string, len(steps))
			for i, s := range steps {
				stepNames[i] = s.FunctionName
			}

			return re.JSON(200, map[string]any{
				"workflow_id": wfID,
				"status":      status.Status,
				"steps":       stepNames,
			})
		})

		// Cancel a job
		e.Router.POST("/job/{id}/cancel", func(re *core.RequestEvent) error {
			id := re.Request.PathValue("id")

			if err := pbdbos.CancelWorkflow(rt, "job-"+id); err != nil {
				return re.JSON(500, map[string]string{"error": err.Error()})
			}

			return re.JSON(200, map[string]string{"result": "cancelled"})
		})

		// Resume a cancelled job
		e.Router.POST("/job/{id}/resume", func(re *core.RequestEvent) error {
			id := re.Request.PathValue("id")

			if err := pbdbos.ResumeWorkflow(rt, "job-"+id); err != nil {
				return re.JSON(500, map[string]string{"error": err.Error()})
			}

			return re.JSON(200, map[string]string{"result": "resumed"})
		})

		return e.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
