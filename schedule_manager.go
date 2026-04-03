package pocketflow

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

type scheduleManager struct {
	mu     sync.Mutex
	timers map[string]context.CancelFunc
}

func newScheduleManager() *scheduleManager {
	return &scheduleManager{
		timers: make(map[string]context.CancelFunc),
	}
}

func (sm *scheduleManager) registerOnce(rt *Runtime, recordID string, fqn string, rawInput json.RawMessage, scheduledAt time.Time) {
	delay := time.Until(scheduledAt)
	if delay < 0 {
		delay = 0
	}

	ctx, cancel := context.WithCancel(rt.ctx)
	sm.mu.Lock()
	sm.timers[recordID] = cancel
	sm.mu.Unlock()

	go func() {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return
		}

		if !rt.launched.Load() {
			return
		}

		sm.mu.Lock()
		delete(sm.timers, recordID)
		sm.mu.Unlock()

		dedupID := fmt.Sprintf("sched-once-%s", recordID)
		_, err := rt.triggerByFQNWithOpts(fqn, rawInput, WithDeduplicationID(dedupID))
		if err != nil {
			rt.app.Logger().Error("failed to execute one-time schedule", "record_id", recordID, "error", err)
		}

		record, err := rt.app.FindRecordById(collectionSchedules, recordID)
		if err == nil {
			_ = rt.app.Delete(record)
		}
	}()
}

func (sm *scheduleManager) registerCron(rt *Runtime, recordID string, fqn string, rawInput json.RawMessage, cronExpr string, jitter time.Duration) error {
	cronJobID := fmt.Sprintf("pf_ui_sched_%s", recordID)
	return rt.app.Cron().Add(cronJobID, cronExpr, func() {
		if !rt.launched.Load() {
			return
		}
		if jitter > 0 {
			delay := time.Duration(rand.N(jitter))
			select {
			case <-time.After(delay):
			case <-rt.ctx.Done():
				return
			}
		}
		_, err := rt.TriggerByFQN(fqn, rawInput)
		if err != nil {
			rt.app.Logger().Error("failed to execute cron schedule", "record_id", recordID, "error", err)
		}
	})
}

func (sm *scheduleManager) removeCron(rt *Runtime, recordID string) {
	cronJobID := fmt.Sprintf("pf_ui_sched_%s", recordID)
	rt.app.Cron().Remove(cronJobID)
}

func (sm *scheduleManager) cancelOnce(recordID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if cancel, ok := sm.timers[recordID]; ok {
		cancel()
		delete(sm.timers, recordID)
	}
}

func (sm *scheduleManager) registerHooks(rt *Runtime) {
	rt.app.OnRecordAfterCreateSuccess(collectionSchedules).BindFunc(func(e *core.RecordEvent) error {
		record := e.Record
		fqn := record.GetString("workflow_fqn")
		rawInput := json.RawMessage(record.GetString("input"))
		schedType := record.GetString("type")

		switch schedType {
		case "cron":
			cronExpr := record.GetString("cron_expression")
			jitter := parseDuration(record.GetString("jitter"))
			if err := sm.registerCron(rt, record.Id, fqn, rawInput, cronExpr, jitter); err != nil {
				rt.app.Logger().Error("failed to register cron from hook", "error", err)
			}
		case "once":
			scheduledAt := record.GetDateTime("scheduled_at").Time()
			sm.registerOnce(rt, record.Id, fqn, rawInput, scheduledAt)
		}
		return e.Next()
	})

	rt.app.OnRecordAfterDeleteSuccess(collectionSchedules).BindFunc(func(e *core.RecordEvent) error {
		record := e.Record
		schedType := record.GetString("type")

		switch schedType {
		case "cron":
			sm.removeCron(rt, record.Id)
		case "once":
			sm.cancelOnce(record.Id)
		}
		return e.Next()
	})
}

func (sm *scheduleManager) loadExisting(rt *Runtime) error {
	records, err := rt.app.FindAllRecords(collectionSchedules)
	if err != nil {
		return fmt.Errorf("failed to load schedules: %w", err)
	}

	for _, record := range records {
		fqn := record.GetString("workflow_fqn")
		rawInput := json.RawMessage(record.GetString("input"))
		schedType := record.GetString("type")

		switch schedType {
		case "cron":
			cronExpr := record.GetString("cron_expression")
			jitter := parseDuration(record.GetString("jitter"))
			if err := sm.registerCron(rt, record.Id, fqn, rawInput, cronExpr, jitter); err != nil {
				rt.app.Logger().Error("failed to load cron schedule", "record_id", record.Id, "error", err)
			}
		case "once":
			scheduledAt := record.GetDateTime("scheduled_at").Time()
			sm.registerOnce(rt, record.Id, fqn, rawInput, scheduledAt)
		}
	}
	return nil
}

func parseDuration(s string) time.Duration {
	if s == "" {
		return 0
	}
	d, _ := time.ParseDuration(s)
	return d
}

func createScheduleRecord(app core.App, fqn string, input json.RawMessage, schedType string, cronExpr string, scheduledAt time.Time) (*core.Record, error) {
	col, err := app.FindCollectionByNameOrId(collectionSchedules)
	if err != nil {
		return nil, err
	}
	record := core.NewRecord(col)
	record.Set("workflow_fqn", fqn)
	record.Set("input", string(input))
	record.Set("type", schedType)
	if cronExpr != "" {
		record.Set("cron_expression", cronExpr)
	}
	if !scheduledAt.IsZero() {
		record.Set("scheduled_at", scheduledAt)
	}
	if err := app.Save(record); err != nil {
		return nil, err
	}
	return record, nil
}
