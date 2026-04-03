# Products

Generate files from workflows and deliver them to external systems via a `ProductSender`.

**What to notice:**
- `ProductSender` interface defines how files are delivered — implement `Send` to upload to S3, GCS, Kafka, email, or any external system
- `turbine.SendProduct` must be called from inside a step, not from the workflow body directly
- Products are deduplicated by `(workflow_id, step_id, file_name)` — safe on recovery
- Metadata lets you tag products for downstream routing or filtering
- If no sender is configured, products are still stored and accessible via the PocketBase API

<<< @/../examples/products/main.go
