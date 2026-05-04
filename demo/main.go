package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"time"

	"github.com/YakirOren/turbine"
	"github.com/YakirOren/turbine/dashboard"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// --- Step functions ---

func SendEmail(ctx context.Context) (string, error) {
	logger := turbine.LoggerFrom(ctx)
	logger.Info("sending email")
	time.Sleep(time.Duration(1+rand.IntN(3)) * time.Second)
	return fmt.Sprintf("msg_%d", rand.IntN(10000)), nil
}

func ValidateOrder(ctx context.Context) (string, error) {
	logger := turbine.LoggerFrom(ctx)
	logger.Info("validating order")
	time.Sleep(2 * time.Second)
	return "valid", nil
}

func ChargePayment(ctx context.Context) (string, error) {
	logger := turbine.LoggerFrom(ctx)
	time.Sleep(5 * time.Second)
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

func ShipOrder(ctx context.Context) (string, error) {
	logger := turbine.LoggerFrom(ctx)
	time.Sleep(4 * time.Second)
	tracking := fmt.Sprintf("tracking_%d", rand.IntN(100000))
	logger.Info("order shipped", "tracking", tracking)
	return tracking, nil
}

func SendConfirmation(ctx context.Context) (string, error) {
	logger := turbine.LoggerFrom(ctx)
	logger.Info("confirmation email sent")
	time.Sleep(1 * time.Second)
	return "email_sent", nil
}

type OrderInput struct {
	OrderID  string `json:"order_id"`
	Customer string `json:"customer"`
}

// --- Input types ---

type DeployInput struct {
	Service     string   `json:"service"`
	Environment string   `json:"environment"`
	Regions     []string `json:"regions"`
	Version     string   `json:"version"`
	DryRun      bool     `json:"dry_run"`
}

type NotifyInput struct {
	Channel string `json:"channel"`
	Message string `json:"message"`
	Urgent  bool   `json:"urgent"`
}

// --- Workflows ---

// EmailWorkflow sends an email to a recipient. Enqueued via the "emails" queue.
func EmailWorkflow(ctx turbine.Context, to string) (string, error) {
	ctx.SetAppStatus("sending", "blue")

	msgID, err := turbine.Do(ctx, SendEmail, turbine.WithStepName("send"))
	if err != nil {
		return "", err
	}

	ctx.SetAppStatus("sent", "green")
	return fmt.Sprintf("to=%s msg_id=%s", to, msgID), nil
}

// OrderWorkflow processes an order end-to-end. Triggerable from the dashboard.
func OrderWorkflow(ctx turbine.Context, input OrderInput) (string, error) {
	orderID := input.OrderID
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

func FetchMetrics(ctx context.Context) (int, error) {
	logger := turbine.LoggerFrom(ctx)
	count := rand.IntN(500)
	logger.Info("fetched metrics", "count", count)
	time.Sleep(1 * time.Second)
	return count, nil
}

func AggregateMetrics(ctx context.Context) (string, error) {
	logger := turbine.LoggerFrom(ctx)
	logger.Info("aggregating metrics")
	time.Sleep(2 * time.Second)
	return "aggregated", nil
}

// MetricsSync runs on a schedule to fetch and aggregate metrics.
func MetricsSync(ctx turbine.Context, scheduledAt time.Time) (string, error) {
	count, err := turbine.Do(ctx, FetchMetrics, turbine.WithStepName("fetch"))
	if err != nil {
		return "", err
	}

	result, err := turbine.Do(ctx, AggregateMetrics, turbine.WithStepName("aggregate"))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("synced %d records: %s", count, result), nil
}

func PruneExpired(ctx context.Context) (int, error) {
	logger := turbine.LoggerFrom(ctx)
	pruned := rand.IntN(50)
	logger.Info("pruned expired records", "count", pruned)
	time.Sleep(1 * time.Second)
	return pruned, nil
}

func CompactStorage(ctx context.Context) (string, error) {
	logger := turbine.LoggerFrom(ctx)
	logger.Info("compacting storage")
	time.Sleep(3 * time.Second)
	return "compacted", nil
}

// DailyCleanup runs daily to prune expired records and compact storage.
func DailyCleanup(ctx turbine.Context, scheduledAt time.Time) (string, error) {
	pruned, err := turbine.Do(ctx, PruneExpired, turbine.WithStepName("prune"))
	if err != nil {
		return "", err
	}

	result, err := turbine.Do(ctx, CompactStorage, turbine.WithStepName("compact"))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("pruned %d, %s", pruned, result), nil
}

func GenerateReport(ctx context.Context) (string, error) {
	logger := turbine.LoggerFrom(ctx)
	logger.Info("generating report")
	time.Sleep(4 * time.Second)
	return fmt.Sprintf("report_%d", rand.IntN(10000)), nil
}

// ReportWorkflow generates a report on demand. Triggerable from the dashboard.
func ReportWorkflow(ctx turbine.Context, reportType string) (string, error) {
	ctx.SetAppStatus("generating", "blue")

	reportID, err := turbine.Do(ctx, GenerateReport, turbine.WithStepName("generate"))
	if err != nil {
		return "", err
	}

	ctx.SetAppStatus("done", "green")

	return fmt.Sprintf("report_type=%s id=%s", reportType, reportID), nil
}

// DeployWorkflow simulates a service deployment with approval gate.
func DeployWorkflow(ctx turbine.Context, input DeployInput) (string, error) {
	logger := ctx.Logger()

	ctx.SetAppStatus("building", "blue")
	buildID, err := turbine.Do(ctx, func(ctx context.Context) (string, error) {
		logger.Info("building service", "service", input.Service, "version", input.Version)
		time.Sleep(3 * time.Second)
		return fmt.Sprintf("build-%d", rand.IntN(10000)), nil
	}, turbine.WithStepName("build"))
	if err != nil {
		return "", err
	}

	if !input.DryRun {
		logger.Info("requesting deployment approval", "environment", input.Environment)

		approval, err := turbine.WaitForApproval(ctx)
		if err != nil {
			return "", fmt.Errorf("approval failed: %w", err)
		}
		if !approval.Approved {
			ctx.SetAppStatus("rejected", "red")
			return fmt.Sprintf("deployment rejected: %s", approval.Comment), nil
		}
	}

	ctx.SetAppStatus("deploying", "purple")
	result, err := turbine.Do(ctx, func(ctx context.Context) (string, error) {
		logger.Info("deploying", "service", input.Service, "env", input.Environment, "regions", input.Regions, "build", buildID)
		time.Sleep(4 * time.Second)
		regions := "all"
		if len(input.Regions) > 0 {
			regions = fmt.Sprintf("%v", input.Regions)
		}
		return fmt.Sprintf("deployed %s@%s to %s %s (build %s)", input.Service, input.Version, input.Environment, regions, buildID), nil
	}, turbine.WithStepName("deploy"))
	if err != nil {
		return "", err
	}

	ctx.SetAppStatus("deployed", "green")
	return result, nil
}

// NotifyWorkflow sends a notification to a channel.
func NotifyWorkflow(ctx turbine.Context, input NotifyInput) (string, error) {
	logger := ctx.Logger()

	if input.Urgent {
		ctx.SetAppStatus("urgent", "red")
	} else {
		ctx.SetAppStatus("sending", "blue")
	}

	result, err := turbine.Do(ctx, func(ctx context.Context) (string, error) {
		logger.Info("sending notification", "channel", input.Channel, "urgent", input.Urgent)
		for i := 0; i < 150; i++ {
			logger.Info("fetching metric", "metric", fmt.Sprintf("metric_%d", i))
		}
		time.Sleep(1 * time.Second)
		return fmt.Sprintf("sent to #%s: %s", input.Channel, input.Message), nil
	}, turbine.WithStepName("send"))
	if err != nil {
		return "", err
	}

	ctx.SetAppStatus("sent", "green")
	return result, nil
}

type LogSender struct{}

func (s *LogSender) Send(_ context.Context, product turbine.ProductRecord, _ io.Reader) error {
	fmt.Printf("[ProductSender] file=%s size=%d metadata=%v\n", product.FileName, product.Size, product.Metadata)
	return nil
}

func main() {
	app := pocketbase.New()

	rt := turbine.Setup(app, turbine.Config{
		ProductSender: &LogSender{},
	})

	// Triggerable from dashboard (no schema — raw JSON input)
	turbine.Register(rt, OrderWorkflow,
		turbine.WithDashboardTrigger(),
		turbine.WithTags("orders", "e2e"),
		turbine.WithSummaryFunc(func(in OrderInput) string {
			return fmt.Sprintf("Order %s for %s", in.OrderID, in.Customer)
		}),
		turbine.WithInputSchema(map[string]any{
			"fields": []map[string]any{
				{"name": "order_id", "type": "string", "label": "Order ID", "required": true, "placeholder": "ORD-123"},
				{"name": "customer", "type": "string", "label": "Customer", "required": true, "placeholder": "Alice"},
			},
		}),
	)
	turbine.Register(rt, ReportWorkflow, turbine.WithDashboardTrigger(), turbine.WithTags("reports"))

	// Triggerable with input schema — typed form in dashboard
	turbine.Register(rt, DeployWorkflow,
		turbine.WithName("deploy"),
		turbine.WithDashboardTrigger(),
		turbine.WithTags("deploy", "infra"),
		turbine.WithInputSchema(map[string]any{
			"fields": []map[string]any{
				{"name": "service", "type": "string", "label": "Service Name", "required": true, "placeholder": "api-gateway"},
				{"name": "environment", "type": "select", "label": "Environment", "required": true, "options": []string{"staging", "production"}},
				{"name": "regions", "type": "multiselect", "label": "Regions", "options": []string{"us-east-1", "us-west-2", "eu-west-1", "ap-southeast-1"}},
				{"name": "version", "type": "string", "label": "Version", "required": true, "placeholder": "v1.2.3"},
				{"name": "dry_run", "type": "boolean", "label": "Dry Run"},
			},
		}),
	)

	turbine.Register(rt, NotifyWorkflow,
		turbine.WithName("notify"),
		turbine.WithDashboardTrigger(),
		turbine.WithTags("notifications"),
		turbine.WithInputSchema(map[string]any{
			"fields": []map[string]any{
				{"name": "channel", "type": "select", "label": "Channel", "required": true, "options": []string{"general", "alerts", "deployments", "incidents"}},
				{"name": "message", "type": "textarea", "label": "Message", "required": true, "placeholder": "Your notification message..."},
				{"name": "urgent", "type": "boolean", "label": "Urgent"},
			},
		}),
	)

	// Queue-based
	turbine.Register(rt, EmailWorkflow, turbine.WithTags("email", "queue"))

	rt.Queue("emails",
		turbine.WithWorkerConcurrency(3),
		turbine.WithRateLimiter(turbine.RateLimiter{Limit: 10, Period: time.Minute}),
	)

	// Scheduled
	turbine.Register(rt, MetricsSync, turbine.WithSchedule("*/5 * * * *"), turbine.WithTags("metrics", "scheduled"))
	turbine.Register(rt, DailyCleanup, turbine.WithSchedule("0 3 * * *"), turbine.WithTags("maintenance", "scheduled"))

	dashboard.Mount(app, rt)

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		e.Router.POST("/order/{id}", func(re *core.RequestEvent) error {
			id := re.Request.PathValue("id")

			handle, err := turbine.Run(rt, OrderWorkflow, OrderInput{
				OrderID:  id,
				Customer: re.Request.URL.Query().Get("customer"),
			})
			if err != nil {
				return re.JSON(500, map[string]string{"error": err.Error()})
			}

			result, err := handle.GetResult()
			if err != nil {
				return re.JSON(500, map[string]string{"error": err.Error()})
			}

			return re.JSON(200, map[string]string{"result": result})
		})

		e.Router.POST("/send-email/{to}", func(re *core.RequestEvent) error {
			to := re.Request.PathValue("to")

			handle, err := turbine.Run(rt, EmailWorkflow, to,
				turbine.WithQueue("emails"),
			)
			if err != nil {
				return re.JSON(500, map[string]string{"error": err.Error()})
			}

			return re.JSON(202, map[string]string{
				"workflow_id": handle.GetWorkflowID(),
				"status":      "enqueued",
			})
		})
		return e.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
