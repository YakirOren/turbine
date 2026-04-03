package turbine

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

	ctx, cancel := context.WithCancel(rt.drainCtx)
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
	cronJobID := fmt.Sprintf("pt_ui_sched_%s", recordID)
	return rt.app.Cron().Add(cronJobID, cronExpr, func() {
		if !rt.launched.Load() {
			return
		}
		if jitter > 0 {
			delay := time.Duration(rand.N(jitter))
			select {
			case <-time.After(delay):
			case <-rt.drainCtx.Done():
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
	cronJobID := fmt.Sprintf("pt_ui_sched_%s", recordID)
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

func (sm *scheduleManager) activate(rt *Runtime, s *Schedule) error {
	switch s.Type() {
	case scheduleTypeCron:
		return sm.registerCron(rt, s.Id, s.WorkflowFQN(), s.Input(), s.CronExpression(), s.Jitter())
	case scheduleTypeOnce:
		sm.registerOnce(rt, s.Id, s.WorkflowFQN(), s.Input(), s.ScheduledAt())
	}
	return nil
}

func (sm *scheduleManager) deactivate(rt *Runtime, s *Schedule) {
	switch s.Type() {
	case scheduleTypeCron:
		sm.removeCron(rt, s.Id)
	case scheduleTypeOnce:
		sm.cancelOnce(s.Id)
	}
}

func (sm *scheduleManager) registerHooks(rt *Runtime) {
	rt.app.OnRecordAfterCreateSuccess(collectionSchedules).BindFunc(func(e *core.RecordEvent) error {
		s := newSchedule(e.Record)
		if !s.Enabled() {
			return e.Next()
		}
		if err := sm.activate(rt, s); err != nil {
			rt.app.Logger().Error("failed to register schedule from hook", "error", err)
		}
		return e.Next()
	})

	rt.app.OnRecordAfterUpdateSuccess(collectionSchedules).BindFunc(func(e *core.RecordEvent) error {
		s := newSchedule(e.Record)

		// Compile-time schedules: just update the in-memory disabled state.
		if s.Type() == scheduleTypeCompile {
			if s.Enabled() {
				rt.disabledSchedules.Delete(s.WorkflowFQN())
			} else {
				rt.disabledSchedules.Store(s.WorkflowFQN(), struct{}{})
			}
			return e.Next()
		}

		sm.deactivate(rt, s)
		if s.Enabled() {
			if err := sm.activate(rt, s); err != nil {
				rt.app.Logger().Error("failed to re-register schedule from hook", "error", err)
			}
		}
		return e.Next()
	})

	rt.app.OnRecordAfterDeleteSuccess(collectionSchedules).BindFunc(func(e *core.RecordEvent) error {
		sm.deactivate(rt, newSchedule(e.Record))
		return e.Next()
	})
}

func (sm *scheduleManager) loadExisting(rt *Runtime) error {
	schedules, err := findAllSchedules(rt.app)
	if err != nil {
		return fmt.Errorf("failed to load schedules: %w", err)
	}

	for _, s := range schedules {
		if s.Type() == scheduleTypeCompile || !s.Enabled() {
			continue
		}
		if err := sm.activate(rt, s); err != nil {
			rt.app.Logger().Error("failed to load schedule", "record_id", s.Id, "error", err)
		}
	}
	return nil
}
