package main

import (
	"context"
	"fmt"
	"log"
	"math/rand/v2"
	"time"

	"github.com/YakirOren/pocketflow"
	"github.com/YakirOren/pocketflow/dashboard"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// --- Step functions ---

func ValidateOrder(ctx context.Context) (string, error) {
	logger := pocketflow.LoggerFrom(ctx)
	logger.Info("validating order")
	time.Sleep(100 * time.Millisecond)
	return "valid", nil
}

func ChargePayment(ctx context.Context) (string, error) {
	logger := pocketflow.LoggerFrom(ctx)
	time.Sleep(200 * time.Millisecond)
	chargeID := fmt.Sprintf("charge_%d", rand.IntN(10000))
	logger.Info("payment charged", "charge_id", chargeID)
	return chargeID, nil
}

func CheckInventory(ctx context.Context) (int, error) {
	logger := pocketflow.LoggerFrom(ctx)
	time.Sleep(150 * time.Millisecond)
	stock := rand.IntN(100)
	logger.Info("inventory checked", "stock", stock)
	return stock, nil
}

func ReserveStock(ctx context.Context) (bool, error) {
	logger := pocketflow.LoggerFrom(ctx)
	logger.Info("stock reserved")
	time.Sleep(100 * time.Millisecond)
	return true, nil
}

func SendConfirmation(ctx context.Context) (string, error) {
	logger := pocketflow.LoggerFrom(ctx)
	logger.Info("confirmation email sent")
	time.Sleep(50 * time.Millisecond)
	return "email_sent", nil
}

func ShipOrder(ctx context.Context) (string, error) {
	logger := pocketflow.LoggerFrom(ctx)
	time.Sleep(300 * time.Millisecond)
	tracking := fmt.Sprintf("tracking_%d", rand.IntN(100000))
	logger.Info("order shipped", "tracking", tracking)
	return tracking, nil
}

// OrderWorkflow demonstrates a multi-step workflow with both
// sequential and parallel steps, visible in the dashboard.
func OrderWorkflow(ctx pocketflow.Context, orderID string) (string, error) {
	// Step 1: Validate
	_, err := pocketflow.Do(ctx, ValidateOrder, pocketflow.WithStepName("validate"))
	if err != nil {
		return "", err
	}

	// Steps 2-3: Charge payment and check inventory in parallel
	chargeCh, err := pocketflow.DoAsync(ctx, ChargePayment, pocketflow.WithStepName("charge"))
	if err != nil {
		return "", err
	}
	inventoryCh, err := pocketflow.DoAsync(ctx, CheckInventory, pocketflow.WithStepName("inventory"))
	if err != nil {
		return "", err
	}

	charge := <-chargeCh
	inventory := <-inventoryCh
	if charge.Err != nil {
		return "", charge.Err
	}
	if inventory.Err != nil {
		return "", inventory.Err
	}

	// Step 4: Reserve stock
	_, err = pocketflow.Do(ctx, ReserveStock, pocketflow.WithStepName("reserve"))
	if err != nil {
		return "", err
	}

	// Steps 5-6: Ship and send confirmation in parallel
	shipCh, err := pocketflow.DoAsync(ctx, ShipOrder, pocketflow.WithStepName("ship"))
	if err != nil {
		return "", err
	}
	emailCh, err := pocketflow.DoAsync(ctx, SendConfirmation, pocketflow.WithStepName("confirm"))
	if err != nil {
		return "", err
	}

	ship := <-shipCh
	email := <-emailCh
	if ship.Err != nil {
		return "", ship.Err
	}
	if email.Err != nil {
		return "", email.Err
	}

	return fmt.Sprintf("order %s: charge=%s, stock=%d, tracking=%s, %s",
		orderID, charge.Result, inventory.Result, ship.Result, email.Result), nil
}

func main() {
	app := pocketbase.New()

	rt := pocketflow.Setup(app, pocketflow.Config{})

	pocketflow.Register(rt, OrderWorkflow, pocketflow.WithDashboardTrigger())

	// Mount the dashboard at /_/pocketflow/
	dashboard.Mount(app, rt)

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// POST /order/:id — start an order workflow
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
