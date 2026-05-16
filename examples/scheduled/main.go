package main

import (
	"context"
	"log"
	"time"

	"github.com/YakirOren/turbine"
)

func DoCleanup(ctx context.Context) (int, error) {
	// do cleanup work
	return 42, nil
}

func Cleanup(ctx turbine.Context, scheduledAt time.Time) (string, error) {
	_, err := turbine.Do(ctx, DoCleanup, turbine.WithStepName("cleanup"))
	if err != nil {
		return "", err
	}
	return "cleaned up", nil
}

func main() {
	app, rt := turbine.NewApp(turbine.Config{})

	// Run cleanup every hour
	turbine.Register(rt, Cleanup, turbine.WithSchedule("0 * * * *"))

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
