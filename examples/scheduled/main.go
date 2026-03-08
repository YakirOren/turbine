package main

import (
	"context"
	"log"
	"time"

	"github.com/YakirOren/pbdbos"
	"github.com/pocketbase/pocketbase"
)

func DoCleanup(ctx context.Context) (int, error) {
	// do cleanup work
	return 42, nil
}

func Cleanup(ctx context.Context, rt *pbdbos.Runtime, scheduledAt time.Time) (string, error) {
	_, err := pbdbos.RunAsStep(ctx, rt, DoCleanup, pbdbos.WithStepName("cleanup"))
	if err != nil {
		return "", err
	}
	return "cleaned up", nil
}

func main() {
	app := pocketbase.New()

	rt := pbdbos.Register(app, pbdbos.Config{})

	// Run cleanup every hour
	pbdbos.RegisterWorkflow(rt, Cleanup, pbdbos.WithSchedule("0 * * * *"))

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
