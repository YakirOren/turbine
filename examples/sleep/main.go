package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/YakirOren/turbine"
)

// ReminderWorkflow sends a reminder after a durable delay.
// If the process crashes during the pause, it resumes with only the remaining time.
func ReminderWorkflow(ctx turbine.Context, userID string) (string, error) {
	if err := turbine.Pause(ctx, 2*time.Second); err != nil {
		return "", err
	}

	_, err := turbine.Do(ctx, func(ctx context.Context) (bool, error) {
		fmt.Printf("sending reminder to %s\n", userID)
		return true, nil
	}, turbine.WithStepName("send-reminder"))
	if err != nil {
		return "", err
	}

	return "reminder sent to " + userID, nil
}

func main() {
	rt := turbine.NewStandalone(turbine.Config{})
	defer func() { _ = rt.Shutdown() }()

	turbine.Register(rt, ReminderWorkflow)

	if err := rt.Launch(); err != nil {
		log.Fatal(err)
	}

	handle, err := turbine.Run(rt, ReminderWorkflow, "user-42")
	if err != nil {
		log.Fatal(err)
	}
	result, err := handle.GetResult()
	if err != nil {
		log.Fatal(err)
	}
	log.Println(result)
}
