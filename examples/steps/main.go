package main

import (
	"context"
	"fmt"
	"log"

	"github.com/YakirOren/pocketflow"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func ChargePayment(ctx context.Context) (string, error) {
	// call payment provider
	return "charge_ok", nil
}

func FulfillOrder(ctx context.Context) (bool, error) {
	// ship the order
	return true, nil
}

// OrderWorkflow demonstrates durable steps.
// Each step's result is saved — if the process crashes mid-workflow,
// it resumes from the last completed step without re-executing it.
func OrderWorkflow(ctx pocketflow.Context, orderID string) (string, error) {
	chargeID, err := pocketflow.RunAsStep(ctx, ChargePayment, pocketflow.WithStepName("charge"))
	if err != nil {
		return "", err
	}

	_, err = pocketflow.RunAsStep(ctx, FulfillOrder, pocketflow.WithStepName("fulfill"))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("order %s completed (charge: %s)", orderID, chargeID), nil
}

func main() {
	app := pocketbase.New()

	rt := pocketflow.Setup(app, pocketflow.Config{})

	pocketflow.Register(rt, OrderWorkflow)

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		e.Router.POST("/order/{id}", func(re *core.RequestEvent) error {
			id := re.Request.PathValue("id")

			handle, err := pocketflow.Run(rt, OrderWorkflow, id)
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
