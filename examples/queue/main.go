package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/YakirOren/turbine"
)

func Send(ctx context.Context) (bool, error) {
	return true, nil
}

func SendEmail(ctx turbine.Context, to string) (string, error) {
	_, err := turbine.Do(ctx, Send, turbine.WithStepName("send"))
	if err != nil {
		return "", err
	}
	return "sent to " + to, nil
}

func main() {
	rt := turbine.NewStandalone(turbine.Config{})
	defer rt.Shutdown()

	turbine.Register(rt, SendEmail)

	rt.Queue("emails",
		turbine.WithWorkerConcurrency(3),
		turbine.WithRateLimiter(turbine.RateLimiter{Limit: 10, Period: time.Minute}),
	)

	if err := rt.Launch(); err != nil {
		log.Fatal(err)
	}

	recipients := []string{"alice@example.com", "bob@example.com", "carol@example.com"}
	handles := make([]turbine.Handle[string], 0, len(recipients))

	for _, to := range recipients {
		handle, err := turbine.Run(rt, SendEmail, to, turbine.WithQueue("emails"))
		if err != nil {
			log.Fatal(err)
		}
		handles = append(handles, handle)
	}

	for _, h := range handles {
		result, err := h.GetResult()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(result)
	}
}
