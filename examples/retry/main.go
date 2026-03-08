package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/YakirOren/pbdbos"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func CallUnreliableAPI(ctx context.Context) (string, error) {
	if rand.Intn(3) == 0 { // fails ~66% of the time
		return "", fmt.Errorf("service unavailable")
	}
	return "ok", nil
}

// FetchWorkflow calls an unreliable API with automatic retries and exponential backoff.
func FetchWorkflow(ctx context.Context, rt *pbdbos.Runtime, url string) (string, error) {
	result, err := pbdbos.RunAsStep(ctx, rt, CallUnreliableAPI,
		pbdbos.WithStepName("fetch"),
		pbdbos.WithStepMaxRetries(5),
		pbdbos.WithBackoffFactor(2.0),
		pbdbos.WithBaseInterval(500*time.Millisecond),
		pbdbos.WithMaxInterval(10*time.Second),
	)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("fetched %s: %s", url, result), nil
}

func main() {
	app := pocketbase.New()

	rt := pbdbos.Register(app, pbdbos.Config{})

	pbdbos.RegisterWorkflow(rt, FetchWorkflow)

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		e.Router.POST("/fetch/{url}", func(re *core.RequestEvent) error {
			url := re.Request.PathValue("url")

			handle, err := pbdbos.RunWorkflow(rt, FetchWorkflow, url)
			if err != nil {
				return re.JSON(500, map[string]string{"error": err.Error()})
			}

			result, err := handle.GetResult()
			if err != nil {
				return re.JSON(500, map[string]string{"error": err.Error()})
			}

			return re.JSON(200, map[string]string{"result": result})
		})
		return e.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
