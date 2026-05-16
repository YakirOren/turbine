package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/YakirOren/turbine"
)

func CallUnreliableAPI(ctx context.Context) (string, error) {
	if rand.Intn(3) == 0 {
		return "", fmt.Errorf("service unavailable")
	}
	return "ok", nil
}

// FetchWorkflow calls an unreliable API with automatic retries and exponential backoff.
func FetchWorkflow(ctx turbine.Context, url string) (string, error) {
	result, err := turbine.Do(ctx, CallUnreliableAPI,
		turbine.WithStepName("fetch"),
		turbine.WithStepMaxRetries(5),
		turbine.WithBackoffFactor(2.0),
		turbine.WithBaseInterval(500*time.Millisecond),
		turbine.WithMaxInterval(10*time.Second),
	)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("fetched %s: %s", url, result), nil
}

func main() {
	rt := turbine.NewStandalone(turbine.Config{})
	defer func() { _ = rt.Shutdown() }()

	turbine.Register(rt, FetchWorkflow)

	if err := rt.Launch(); err != nil {
		log.Fatal(err)
	}

	handle, err := turbine.Run(rt, FetchWorkflow, "https://example.com")
	if err != nil {
		log.Fatal(err)
	}
	result, err := handle.GetResult()
	if err != nil {
		log.Fatal(err)
	}
	log.Println(result)
}
