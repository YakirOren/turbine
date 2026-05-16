package main

import (
	"context"
	"log"
	"time"

	"github.com/YakirOren/turbine"
)

// LongRunningJob sleeps for a long time. Can be cancelled and resumed.
func LongRunningJob(ctx turbine.Context, jobID string) (string, error) {
	if err := turbine.Pause(ctx, 1*time.Hour); err != nil {
		return "", err
	}

	_, err := turbine.Do(ctx, func(ctx context.Context) (bool, error) {
		return true, nil
	}, turbine.WithStepName("process"))
	if err != nil {
		return "", err
	}

	return "job " + jobID + " completed", nil
}

func main() {
	app, rt := turbine.NewApp(turbine.Config{})

	turbine.Register(rt, LongRunningJob)

	app.OnServe().BindFunc(func(e *turbine.ServeEvent) error {
		e.Router.POST("/job/{id}", func(re *turbine.RequestEvent) error {
			id := re.Request.PathValue("id")

			handle, err := turbine.Run(rt, LongRunningJob, id,
				turbine.WithID("job-"+id),
			)
			if err != nil {
				return re.JSON(500, map[string]string{"error": err.Error()})
			}

			return re.JSON(202, map[string]string{
				"workflow_id": handle.GetWorkflowID(),
			})
		})

		e.Router.GET("/job/{id}", func(re *turbine.RequestEvent) error {
			id := re.Request.PathValue("id")
			wfID := "job-" + id

			handle := turbine.Retrieve[string](rt, wfID)

			status, err := handle.GetStatus()
			if err != nil {
				return re.JSON(500, map[string]string{"error": err.Error()})
			}

			steps, err := rt.Steps(wfID)
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

		e.Router.POST("/job/{id}/cancel", func(re *turbine.RequestEvent) error {
			id := re.Request.PathValue("id")

			if err := rt.Cancel("job-" + id); err != nil {
				return re.JSON(500, map[string]string{"error": err.Error()})
			}

			return re.JSON(200, map[string]string{"result": "cancelled"})
		})

		e.Router.POST("/job/{id}/resume", func(re *turbine.RequestEvent) error {
			id := re.Request.PathValue("id")

			if err := rt.Resume("job-" + id); err != nil {
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
