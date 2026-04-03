package main

import (
	"context"
	"fmt"
	"log"
	"math/rand/v2"
	"time"

	"github.com/YakirOren/turbine"
	"github.com/YakirOren/turbine/dashboard"
	"github.com/pocketbase/pocketbase"
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

func ShipOrder(ctx context.Context) (string, error) {
	logger := turbine.LoggerFrom(ctx)
	time.Sleep(3 * time.Second)
	tracking := fmt.Sprintf("TRACK-%d", rand.IntN(100000))
	logger.Info("order shipped", "tracking", tracking)
	return tracking, nil
}

func SendConfirmation(ctx context.Context) (string, error) {
	logger := turbine.LoggerFrom(ctx)
	logger.Info("confirmation email sent")
	time.Sleep(1 * time.Second)
	return "email_sent", nil
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

// OrderWorkflow processes an order end-to-end. Triggerable from the dashboard.
func OrderWorkflow(ctx turbine.Context, orderID string) (string, error) {
	ctx.SetAppStatus("validating", "yellow")

	_, err := turbine.Do(ctx, ValidateOrder, turbine.WithStepName("validate"))
	if err != nil {
		return "", err
	}

	ctx.SetAppStatus("processing", "blue")

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

	ctx.SetAppStatus("shipping", "purple")

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

	return fmt.Sprintf("order=%s charge=%s stock=%d tracking=%s %s",
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
		ctx.SetAppStatus("awaiting approval", "yellow")
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
		time.Sleep(1 * time.Second)
		return fmt.Sprintf("sent to #%s: %s", input.Channel, input.Message), nil
	}, turbine.WithStepName("send"))
	if err != nil {
		return "", err
	}

	ctx.SetAppStatus("sent", "green")
	return result, nil
}

func main() {
	app := pocketbase.New()

	rt := turbine.Setup(app, turbine.Config{})

	// Triggerable from dashboard (no schema — raw JSON input)
	turbine.Register(rt, OrderWorkflow, turbine.WithDashboardTrigger(), turbine.WithTags("orders", "e2e"))
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

	// Scheduled
	turbine.Register(rt, MetricsSync, turbine.WithSchedule("*/5 * * * *"), turbine.WithTags("metrics", "scheduled"))
	turbine.Register(rt, DailyCleanup, turbine.WithSchedule("0 3 * * *"), turbine.WithTags("maintenance", "scheduled"))

	dashboard.Mount(app, rt)

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
