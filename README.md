# turbine

[![Go Reference](https://pkg.go.dev/badge/github.com/YakirOren/turbine.svg)](https://pkg.go.dev/github.com/YakirOren/turbine)
[![Go Report Card](https://goreportcard.com/badge/github.com/YakirOren/turbine)](https://goreportcard.com/report/github.com/YakirOren/turbine)
[![Test](https://github.com/YakirOren/turbine/actions/workflows/test.yml/badge.svg)](https://github.com/YakirOren/turbine/actions/workflows/test.yml)

SQLite-based durable workflow engine for Go. No external dependencies, single binary.

[![Dashboard screenshot](docs/screenshots/steps-page-logs.png)](https://turbine.yakir.io/)

## Install

```bash
go get github.com/YakirOren/turbine
```

## Features

**Durability**
- **Durable workflows**, survive crashes, restart where they left off
- **Steps**, record intermediate results, replay on recovery
- **Step retries**, automatic retries with exponential backoff
- **Durable pause**, survives restarts

**Coordination**
- **Queues**, concurrency control, priority, rate limiting, partitioning
- **Scheduled workflows**, cron expressions via the built-in scheduler
- **Send/Receive**, inter-workflow messaging
- **Events**, key-value signaling between workflows

**Lifecycle**
- **Timeout/Deadline**, cancel workflows after a duration or at a specific time
- **Garbage collection**, automatic cleanup of completed workflows

## [Documentation](https://turbine.yakir.io/)

- Concepts: [workflows](https://turbine.yakir.io/concepts/workflows), [steps](https://turbine.yakir.io/concepts/steps), [checkpoints](https://turbine.yakir.io/concepts/checkpoints), [queues](https://turbine.yakir.io/concepts/queues), [scheduling](https://turbine.yakir.io/concepts/scheduling), [communication](https://turbine.yakir.io/concepts/communication), [kv-store](https://turbine.yakir.io/concepts/kv-store), [lifecycle](https://turbine.yakir.io/concepts/lifecycle), [errors](https://turbine.yakir.io/concepts/errors)
- API reference: [workflows](https://turbine.yakir.io/api/workflows), [steps](https://turbine.yakir.io/api/steps), [queues](https://turbine.yakir.io/api/queues), [configuration](https://turbine.yakir.io/api/configuration)

## Examples

Every feature has a runnable example under [`examples/`](examples/). Start with [`basic`](examples/basic/) for a minimal workflow.

| Example | What it shows |
|---|---|
| [`basic`](examples/basic/) | Minimal workflow |
| [`steps`](examples/steps/) | Multi-step order processing |
| [`concurrent`](examples/concurrent/) | Parallel steps with `DoAsync` |
| [`retry`](examples/retry/) | Step retries with exponential backoff |
| [`sleep`](examples/sleep/) | Durable pause |
| [`queue`](examples/queue/) | Queue with concurrency and rate limiting |
| [`scheduled`](examples/scheduled/) | Cron-scheduled workflows |
| [`events`](examples/events/) | Key-value events and `Send`/`Receive` |
| [`lifecycle`](examples/lifecycle/) | Cancel, resume, and inspect workflows |
| [`app-access`](examples/app-access/) | Serve HTTP routes alongside workflows |
| [`products`](examples/products/) | Generate files and deliver them via a `ProductSender` |
| [`connector`](examples/connector/) | Live connections with `WithoutCheckpoint` |
| [`dashboard`](examples/dashboard/) | Full demo with the dashboard UI mounted |
