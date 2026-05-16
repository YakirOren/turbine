package main

import (
	"context"
	"fmt"
	"log"

	"github.com/YakirOren/turbine"
)

func ChargePayment(ctx context.Context) (string, error) {
	return "charge_ok", nil
}

func FulfillOrder(ctx context.Context) (bool, error) {
	return true, nil
}

// OrderWorkflow demonstrates durable steps.
// Each step's result is saved. If the process crashes mid-workflow,
// it resumes from the last completed step without re-executing it.
func OrderWorkflow(ctx turbine.Context, orderID string) (string, error) {
	chargeID, err := turbine.Do(ctx, ChargePayment, turbine.WithStepName("charge"))
	if err != nil {
		return "", err
	}

	_, err = turbine.Do(ctx, FulfillOrder, turbine.WithStepName("fulfill"))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("order %s completed (charge: %s)", orderID, chargeID), nil
}

func main() {
	rt := turbine.NewStandalone(turbine.Config{})
	defer func() { _ = rt.Shutdown() }()

	turbine.Register(rt, OrderWorkflow)

	if err := rt.Launch(); err != nil {
		log.Fatal(err)
	}

	handle, err := turbine.Run(rt, OrderWorkflow, "ord-42")
	if err != nil {
		log.Fatal(err)
	}
	result, err := handle.GetResult()
	if err != nil {
		log.Fatal(err)
	}
	log.Println(result)
}
