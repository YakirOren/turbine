package main

import (
	"context"
	"fmt"
	"log"

	"github.com/YakirOren/turbine"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func FetchPricing(ctx context.Context) (string, error) {
	return "price: $99", nil
}

func FetchInventory(ctx context.Context) (int, error) {
	return 42, nil
}

func FetchReviews(ctx context.Context) (string, error) {
	return "4.5 stars", nil
}

// ProductWorkflow fetches pricing, inventory, and reviews concurrently.
// Each step is durable — on recovery, completed steps replay from the DB.
func ProductWorkflow(ctx turbine.Context, productID string) (string, error) {
	priceCh, err := turbine.DoAsync(ctx, FetchPricing, turbine.WithStepName("pricing"))
	if err != nil {
		return "", err
	}

	inventoryCh, err := turbine.DoAsync(ctx, FetchInventory, turbine.WithStepName("inventory"))
	if err != nil {
		return "", err
	}

	reviewsCh, err := turbine.DoAsync(ctx, FetchReviews, turbine.WithStepName("reviews"))
	if err != nil {
		return "", err
	}

	price := <-priceCh
	inventory := <-inventoryCh
	reviews := <-reviewsCh

	if price.Err != nil || inventory.Err != nil || reviews.Err != nil {
		return "", fmt.Errorf("one or more steps failed")
	}

	return fmt.Sprintf("product %s: %s, stock=%d, %s", productID, price.Result, inventory.Result, reviews.Result), nil
}

func main() {
	app := pocketbase.New()

	rt := turbine.Setup(app, turbine.Config{})

	turbine.Register(rt, ProductWorkflow)

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		e.Router.GET("/product/{id}", func(re *core.RequestEvent) error {
			id := re.Request.PathValue("id")

			handle, err := turbine.Run(rt, ProductWorkflow, id)
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
