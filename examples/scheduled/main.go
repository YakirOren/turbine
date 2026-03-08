package main

import (
	"context"
	"log"
	"time"

	"github.com/YakirOren/pocketflow"
	"github.com/pocketbase/pocketbase"
)

func DoCleanup(ctx context.Context) (int, error) {
	// do cleanup work
	return 42, nil
}

func Cleanup(ctx context.Context, rt *pocketflow.Runtime, scheduledAt time.Time) (string, error) {
	_, err := pocketflow.RunAsStep(ctx, rt, DoCleanup, pocketflow.WithStepName("cleanup"))
	if err != nil {
		return "", err
	}
	return "cleaned up", nil
}

func main() {
	app := pocketbase.New()

	rt := pocketflow.Register(app, pocketflow.Config{})

	// Run cleanup every hour
	pocketflow.RegisterWorkflow(rt, Cleanup, pocketflow.WithSchedule("0 * * * *"))

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
