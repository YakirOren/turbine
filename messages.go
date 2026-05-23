package turbine

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// messages pairs the durable message tables with the in-memory event bus.
// The two halves are inseparable: every method either combines a DB read
// with a subscription, or a DB write with a notification.
type messages struct {
	app    core.App
	eb     *eventBus
	logger *slog.Logger
}

func newMessages(app core.App, eb *eventBus, logger *slog.Logger) *messages {
	return &messages{
		app:    app,
		eb:     eb,
		logger: logger.With("service", "messages"),
	}
}

func (m *messages) awaitWorkflowResult(ctx context.Context, workflowID string, _ time.Duration) (*string, error) {
	key := "workflow::" + workflowID
	ch := m.eb.Wait(key)
	defer m.eb.Remove(key, ch)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		var status StatusType
		var outputString, errorStr sql.NullString
		var attempts int

		err := m.app.DB().Select("status", "output", "error", "recovery_attempts").
			From("pt_workflow_status").
			Where(dbx.HashExp{"id": workflowID}).
			Row(&status, &outputString, &errorStr, &attempts)

		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				select {
				case <-ch:
					ch = m.eb.Swap(key, ch)
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				continue
			}
			return nil, fmt.Errorf("failed to query workflow status: %w", err)
		}

		var output *string
		if outputString.Valid {
			output = &outputString.String
		}

		switch status {
		case StatusSuccess, StatusError:
			if !errorStr.Valid || errorStr.String == "" {
				return output, nil
			}
			return output, errors.New(errorStr.String)
		case StatusCancelled:
			return output, newErrAwaitCancelled(workflowID)
		case StatusMaxRecoveryAttemptsExceeded:
			return output, newErrDeadLetter(workflowID, attempts-2)
		default:
			select {
			case <-ch:
				ch = m.eb.Swap(key, ch)
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}
}

func (m *messages) send(ctx context.Context, input sendInput) error {
	// When called as a workflow step, derive a deterministic row id from
	// (producer_workflow, producer_step) so step replay after crash does not
	// re-insert the message. ON CONFLICT DO NOTHING makes the insert idempotent.
	var msgID string
	if input.ProducerWorkflow != "" {
		msgID = fmt.Sprintf("snd_%s_%d", input.ProducerWorkflow, input.ProducerStepID)
	} else {
		msgID = core.GenerateDefaultRandomId()
	}
	_, err := m.app.DB().NewQuery(`INSERT INTO pt_notifications
		(id, destination_id, topic, message, created_at_epoch_ms, consumed)
		VALUES ({:id}, {:dest}, {:topic}, {:msg}, {:ts}, FALSE)
		ON CONFLICT (id) DO NOTHING`).Bind(dbx.Params{
		"id":    msgID,
		"dest":  input.DestinationUUID,
		"topic": input.Topic,
		"msg":   derefStr(input.Message),
		"ts":    time.Now().UnixMilli(),
	}).Execute()

	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}

	// Signal event bus
	payload := fmt.Sprintf("%s::%s", input.DestinationUUID, input.Topic)
	m.eb.Notify(payload)

	return nil
}

func (m *messages) recv(ctx context.Context, input recvInput) (*string, error) {
	// Try to consume a message directly
	var message sql.NullString
	err := m.app.DB().NewQuery(`
		WITH oldest AS (
			SELECT id, message
			FROM pt_notifications
			WHERE destination_id = {:dest} AND topic = {:topic} AND consumed = FALSE
			ORDER BY created_at_epoch_ms ASC
			LIMIT 1
		)
		UPDATE pt_notifications SET consumed = TRUE
		WHERE id = (SELECT id FROM oldest)
		RETURNING message`).Bind(dbx.Params{
		"dest":  input.workflowUUID,
		"topic": input.topic,
	}).Row(&message)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to consume notification: %w", err)
	}

	if message.Valid {
		return &message.String, nil
	}

	// No message found, wait with event bus
	if input.timeout <= 0 {
		return nil, nil
	}

	payload := fmt.Sprintf("%s::%s", input.workflowUUID, input.topic)
	ch := m.eb.Wait(payload)
	defer m.eb.Remove(payload, ch)

	timer := time.NewTimer(input.timeout)
	defer timer.Stop()

	for {
		select {
		case <-ch:
			ch = m.eb.Swap(payload, ch)

			// Try again
			err = m.app.DB().NewQuery(`
				WITH oldest AS (
					SELECT id, message
					FROM pt_notifications
					WHERE destination_id = {:dest} AND topic = {:topic} AND consumed = FALSE
					ORDER BY created_at_epoch_ms ASC
					LIMIT 1
				)
				UPDATE pt_notifications SET consumed = TRUE
				WHERE id = (SELECT id FROM oldest)
				RETURNING message`).Bind(dbx.Params{
				"dest":  input.workflowUUID,
				"topic": input.topic,
			}).Row(&message)

			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return nil, fmt.Errorf("failed to consume notification: %w", err)
			}
			if message.Valid {
				return &message.String, nil
			}

		case <-timer.C:
			return nil, nil

		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (m *messages) setEvent(ctx context.Context, input setValueInput) error {
	_, err := m.app.DB().NewQuery(`INSERT INTO pt_workflow_events (id, workflow_id, key, value)
		VALUES ({:id}, {:wf_id}, {:key}, {:value})
		ON CONFLICT (workflow_id, key)
		DO UPDATE SET value = excluded.value`).Bind(dbx.Params{
		"id":    fmt.Sprintf("%s_%s", input.WorkflowUUID, input.Key),
		"wf_id": input.WorkflowUUID,
		"key":   input.Key,
		"value": derefStr(input.Value),
	}).Execute()

	if err != nil {
		return fmt.Errorf("failed to set event: %w", err)
	}

	// Signal event bus
	payload := fmt.Sprintf("%s::%s", input.WorkflowUUID, input.Key)
	m.eb.Notify(payload)

	return nil
}

func (m *messages) getEvent(ctx context.Context, input getEventInput) (*string, error) {
	var value sql.NullString
	err := m.app.DB().Select("value").
		From("pt_workflow_events").
		Where(dbx.HashExp{"workflow_id": input.targetWorkflowUUID, "key": input.key}).
		Row(&value)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to get event: %w", err)
	}

	if value.Valid {
		return &value.String, nil
	}

	// Event not found, wait with event bus
	if input.timeout <= 0 {
		return nil, nil
	}

	payload := fmt.Sprintf("%s::%s", input.targetWorkflowUUID, input.key)
	ch := m.eb.Wait(payload)
	defer m.eb.Remove(payload, ch)

	timer := time.NewTimer(input.timeout)
	defer timer.Stop()

	for {
		select {
		case <-ch:
			ch = m.eb.Swap(payload, ch)

			err = m.app.DB().Select("value").
				From("pt_workflow_events").
				Where(dbx.HashExp{"workflow_id": input.targetWorkflowUUID, "key": input.key}).
				Row(&value)

			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return nil, fmt.Errorf("failed to get event: %w", err)
			}
			if value.Valid {
				return &value.String, nil
			}

		case <-timer.C:
			return nil, nil

		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// subscribeQueue returns a channel that fires when work is enqueued to queueName.
// The caller must pass the channel back to unsubscribeQueue when done.
func (m *messages) subscribeQueue(queueName string) chan struct{} {
	return m.eb.Wait("queue::" + queueName)
}

// unsubscribeQueue removes a previously subscribed channel for queueName.
func (m *messages) unsubscribeQueue(queueName string, ch chan struct{}) {
	m.eb.Remove("queue::"+queueName, ch)
}
