package main

import (
	"context"
	"fmt"
	"log"

	"github.com/YakirOren/turbine"
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
// Each step is durable. On recovery, completed steps replay from the DB.
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
	rt := turbine.NewStandalone(turbine.Config{})
	defer rt.Shutdown()

	turbine.Register(rt, ProductWorkflow)

	if err := rt.Launch(); err != nil {
		log.Fatal(err)
	}

	handle, err := turbine.Run(rt, ProductWorkflow, "prod-123")
	if err != nil {
		log.Fatal(err)
	}
	result, err := handle.GetResult()
	if err != nil {
		log.Fatal(err)
	}
	log.Println(result)
}
