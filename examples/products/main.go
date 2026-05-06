package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"

	"github.com/YakirOren/turbine"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
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
	fmt.Printf("delivering %s (%d bytes) — metadata: %v\n", product.FileName, product.Size, product.Metadata)

	// Examples of what you'd do here:
	// - Upload to S3:        s3Client.PutObject(ctx, &s3.PutObjectInput{Bucket: ..., Key: &product.FileName, Body: data})
	// - Send to GCS:         w := gcsClient.Bucket(b).Object(product.FileName).NewWriter(ctx); io.Copy(w, data); w.Close()
	// - Push to Kafka:       buf, _ := io.ReadAll(data); producer.Send(&kafka.Message{Key: []byte(product.FileName), Value: buf})
	// - Email attachment:    smtpClient.SendWithAttachment(to, product.FileName, data)
	// - POST to a webhook:   http.Post(webhookURL, "application/octet-stream", data)

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
	app := pocketbase.New()

	rt := turbine.Setup(app, turbine.Config{
		ProductSender: &ProductSender{},
	})

	turbine.Register(rt, ReportWorkflow)

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		e.Router.POST("/report", func(re *core.RequestEvent) error {
			input := ReportInput{
				Month:    re.Request.URL.Query().Get("month"),
				Customer: re.Request.URL.Query().Get("customer"),
			}

			handle, err := turbine.Run(rt, ReportWorkflow, input)
			if err != nil {
				return re.JSON(500, map[string]string{"error": err.Error()})
			}

			result, err := handle.GetResult()
			if err != nil {
				return re.JSON(500, map[string]string{"error": err.Error()})
			}

			return re.JSON(200, map[string]string{"file": result})
		})
		return e.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
