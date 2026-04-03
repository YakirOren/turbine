package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"math/rand/v2"
	"time"

	"github.com/YakirOren/turbine"
	"github.com/YakirOren/turbine/dashboard"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// --- Step functions ---

func ValidateOrder(ctx context.Context) (string, error) {
	logger := turbine.LoggerFrom(ctx)
	logger.Info("validating order")
	time.Sleep(2 * time.Second)
	return "valid", nil
}

func ChargePayment(ctx context.Context) (string, error) {
	logger := turbine.LoggerFrom(ctx)
	time.Sleep(10 * time.Second)
	chargeID := fmt.Sprintf("charge_%d", rand.IntN(10000))
	logger.Info("payment charged", "charge_id", chargeID)
	return chargeID, nil
}

func CheckInventory(ctx context.Context) (int, error) {
	logger := turbine.LoggerFrom(ctx)
	time.Sleep(2 * time.Second)
	stock := rand.IntN(100)
	logger.Info("inventory checked", "stock", stock)
	return stock, nil
}

func ReserveStock(ctx context.Context) (bool, error) {
	logger := turbine.LoggerFrom(ctx)
	logger.Info("stock reserved")
	time.Sleep(2 * time.Second)
	return true, nil
}

func GenerateInvoice(ctx context.Context) (string, error) {
	logger := turbine.LoggerFrom(ctx)
	logger.Info("generating invoice PDF")
	time.Sleep(1 * time.Second)

	invoiceID := fmt.Sprintf("INV-%d", rand.IntN(100000))
	content := fmt.Sprintf("Invoice %s\nDate: %s\nAmount: $%.2f", invoiceID, time.Now().Format("2006-01-02"), float64(rand.IntN(10000))/100)

	err := turbine.SendProduct(ctx, invoiceID+".txt", bytes.NewReader([]byte(content)), map[string]any{
		"type":       "invoice",
		"invoice_id": invoiceID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to send invoice: %w", err)
	}

	logger.Info("invoice sent as product", "invoice_id", invoiceID)
	return invoiceID, nil
}

func SendConfirmation(ctx context.Context) (string, error) {
	logger := turbine.LoggerFrom(ctx)
	logger.Info("confirmation email sent")
	time.Sleep(1 * time.Second)
	return "email_sent", nil
}

func ShipOrder(ctx context.Context) (string, error) {
	logger := turbine.LoggerFrom(ctx)
	time.Sleep(4 * time.Second)
	tracking := fmt.Sprintf("tracking_%d", rand.IntN(100000))
	logger.Info("order shipped", "tracking", tracking)
	return tracking, nil
}

// OrderWorkflow demonstrates a multi-step workflow with both
// sequential and parallel steps, visible in the dashboard.
func OrderWorkflow(ctx turbine.Context, orderID string) (string, error) {
	ctx.SetAppStatus("validating", "yellow")

	// Step 1: Validate
	_, err := turbine.Do(ctx, ValidateOrder, turbine.WithStepName("validate"))
	if err != nil {
		return "", err
	}

	ctx.SetAppStatus("processing-payment", "blue")

	// Steps 2-3: Charge payment and check inventory in parallel
	chargeCh, err := turbine.DoAsync(ctx, ChargePayment, turbine.WithStepName("charge"))
	if err != nil {
		return "", err
	}
	inventoryCh, err := turbine.DoAsync(ctx, CheckInventory, turbine.WithStepName("inventory"))
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

	ctx.SetAppStatus("reserving-stock", "orange")

	// Step 4: Reserve stock
	_, err = turbine.Do(ctx, ReserveStock, turbine.WithStepName("reserve"))
	if err != nil {
		return "", err
	}

	ctx.SetAppStatus("invoicing", "cyan")

	// Step 5: Generate invoice product
	_, err = turbine.Do(ctx, GenerateInvoice, turbine.WithStepName("invoice"))
	if err != nil {
		return "", err
	}

	ctx.SetAppStatus("shipping", "purple")

	// Steps 6-7: Ship and send confirmation in parallel
	shipCh, err := turbine.DoAsync(ctx, ShipOrder, turbine.WithStepName("ship"))
	if err != nil {
		return "", err
	}
	emailCh, err := turbine.DoAsync(ctx, SendConfirmation, turbine.WithStepName("confirm"))
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

	ctx.SetAppStatus("fulfilled", "green")

	return fmt.Sprintf("order %s: charge=%s, stock=%d, tracking=%s, %s",
		orderID, charge.Result, inventory.Result, ship.Result, email.Result), nil
}

type LogSender struct{}

func (s *LogSender) Send(_ context.Context, product turbine.ProductRecord) error {
	fmt.Printf("[ProductSender] file=%s url=%s metadata=%v\n", product.FileName, product.FileURL, product.Metadata)
	return nil
}

func main() {
	app := pocketbase.New()

	rt := turbine.Setup(app, turbine.Config{
		ProductSender: &LogSender{},
	})

	turbine.Register(rt, OrderWorkflow, turbine.WithDashboardTrigger())

	// Mount the dashboard at /_/turbine/
	dashboard.Mount(app, rt)

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		// POST /order/:id — start an order workflow
		e.Router.POST("/order/{id}", func(re *core.RequestEvent) error {
			id := re.Request.PathValue("id")

			handle, err := turbine.Run(rt, OrderWorkflow, id)
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
