package main

import (
	"context"
	"log"
	"time"

	"github.com/YakirOren/pbdbos"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func Send(ctx context.Context) (bool, error) {
	// send the email
	return true, nil
}

func SendEmail(ctx context.Context, rt *pbdbos.Runtime, to string) (string, error) {
	_, err := pbdbos.RunAsStep(ctx, rt, Send, pbdbos.WithStepName("send"))
	if err != nil {
		return "", err
	}
	return "sent to " + to, nil
}

func main() {
	app := pocketbase.New()

	rt := pbdbos.Register(app, pbdbos.Config{})

	pbdbos.RegisterWorkflow(rt, SendEmail)

	pbdbos.NewWorkflowQueue(rt, "emails",
		pbdbos.WithWorkerConcurrency(3),
		pbdbos.WithRateLimiter(pbdbos.RateLimiter{Limit: 10, Period: time.Minute}),
	)

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		e.Router.POST("/send-email/{to}", func(re *core.RequestEvent) error {
			to := re.Request.PathValue("to")

			handle, err := pbdbos.RunWorkflow(rt, SendEmail, to,
				pbdbos.WithQueue("emails"),
			)
			if err != nil {
				return re.JSON(500, map[string]string{"error": err.Error()})
			}

			return re.JSON(202, map[string]string{
				"workflow_id": handle.GetWorkflowID(),
				"status":      "enqueued",
			})
		})
		return e.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
