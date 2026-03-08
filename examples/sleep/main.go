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

// ReminderWorkflow sends a reminder after a durable delay.
// If the process crashes during the sleep, it resumes with only the remaining time.
func ReminderWorkflow(ctx context.Context, rt *pbdbos.Runtime, userID string) (string, error) {
	// Wait 24 hours — survives crashes and restarts
	if err := pbdbos.Sleep(ctx, rt, 24*time.Hour); err != nil {
		return "", err
	}

	_, err := pbdbos.RunAsStep(ctx, rt, func(ctx context.Context) (bool, error) {
		fmt.Printf("sending reminder to %s\n", userID)
		return true, nil
	}, pbdbos.WithStepName("send-reminder"))
	if err != nil {
		return "", err
	}

	return "reminder sent to " + userID, nil
}

func main() {
	app := pocketbase.New()

	rt := pbdbos.Register(app, pbdbos.Config{})

	pbdbos.RegisterWorkflow(rt, ReminderWorkflow)

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		e.Router.POST("/remind/{userID}", func(re *core.RequestEvent) error {
			userID := re.Request.PathValue("userID")

			handle, err := pbdbos.RunWorkflow(rt, ReminderWorkflow, userID)
			if err != nil {
				return re.JSON(500, map[string]string{"error": err.Error()})
			}

			return re.JSON(202, map[string]string{
				"workflow_id": handle.GetWorkflowID(),
				"status":      "sleeping for 24h",
			})
		})
		return e.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
