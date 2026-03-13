package main

import (
	"context"
	"log"
	"time"

	"github.com/YakirOren/pocketflow"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func Send(ctx context.Context) (bool, error) {
	// send the email
	return true, nil
}

func SendEmail(ctx pocketflow.Context, to string) (string, error) {
	_, err := pocketflow.RunAsStep(ctx, Send, pocketflow.WithStepName("send"))
	if err != nil {
		return "", err
	}
	return "sent to " + to, nil
}

func main() {
	app := pocketbase.New()

	rt := pocketflow.Setup(app, pocketflow.Config{})

	pocketflow.Register(rt, SendEmail)

	pocketflow.NewWorkflowQueue(rt, "emails",
		pocketflow.WithWorkerConcurrency(3),
		pocketflow.WithRateLimiter(pocketflow.RateLimiter{Limit: 10, Period: time.Minute}),
	)

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		e.Router.POST("/send-email/{to}", func(re *core.RequestEvent) error {
			to := re.Request.PathValue("to")

			handle, err := pocketflow.Run(rt, SendEmail, to,
				pocketflow.WithQueue("emails"),
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
