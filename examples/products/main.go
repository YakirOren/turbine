package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"

	"github.com/YakirOren/turbine"
)

// ProductSender delivers product files to external systems.
// In production, this would upload to S3, GCS, Azure Blob Storage,
// or forward to a data pipeline (Snowflake, BigQuery, Kafka, etc.).
//
// The data parameter carries the full file bytes. The concrete value is
// *bytes.Reader, so senders that need to retry can type-assert to io.Seeker
// and rewind between attempts.
type ProductSender struct{}

func (s *ProductSender) Send(_ context.Context, product turbine.ProductRecord, data io.Reader) error {
	fmt.Printf("delivering %s (%d bytes), metadata: %v\n", product.FileName, product.Size, product.Metadata)
	return nil
}

type ReportInput struct {
	Month    string `json:"month"`
	Customer string `json:"customer"`
}

// ReportWorkflow generates a CSV report and delivers it as a product.
func ReportWorkflow(ctx turbine.Context, input ReportInput) (string, error) {
	fileName, err := turbine.Do(ctx, func(ctx context.Context) (string, error) {
		logger := turbine.LoggerFrom(ctx)

		var buf bytes.Buffer
		w := csv.NewWriter(&buf)
		_ = w.Write([]string{"date", "item", "amount"})
		_ = w.Write([]string{"2026-03-01", "Widget A", "49.00"})
		_ = w.Write([]string{"2026-03-15", "Widget B", "129.00"})
		w.Flush()

		name := fmt.Sprintf("%s-%s.csv", input.Customer, input.Month)

		err := turbine.SendProduct(ctx, name, &buf, map[string]any{
			"type":     "report",
			"month":    input.Month,
			"customer": input.Customer,
		})
		if err != nil {
			return "", err
		}

		logger.Info("product created", "file", name)
		return name, nil
	}, turbine.WithStepName("generate-report"))
	if err != nil {
		return "", err
	}

	return fileName, nil
}

func main() {
	rt := turbine.NewStandalone(turbine.Config{
		ProductSender: &ProductSender{},
	})
	defer rt.Shutdown()

	turbine.Register(rt, ReportWorkflow)

	if err := rt.Launch(); err != nil {
		log.Fatal(err)
	}

	handle, err := turbine.Run(rt, ReportWorkflow, ReportInput{
		Month:    "2026-03",
		Customer: "acme",
	})
	if err != nil {
		log.Fatal(err)
	}
	result, err := handle.GetResult()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("report:", result)
}
