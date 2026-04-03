package turbine

import (
	"encoding/json"
	"time"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

const (
	scheduleTypeCron    = "cron"
	scheduleTypeOnce    = "once"
	scheduleTypeCompile = "compile"
)

var _ core.RecordProxy = (*Schedule)(nil)

// Schedule is a typed proxy for pt_schedules records.
type Schedule struct {
	core.BaseRecordProxy
}

func newSchedule(record *core.Record) *Schedule {
	s := &Schedule{}
	s.SetProxyRecord(record)
	return s
}

func (s *Schedule) WorkflowFQN() string            { return s.GetString("workflow_fqn") }
func (s *Schedule) SetWorkflowFQN(fqn string)      { s.Set("workflow_fqn", fqn) }
func (s *Schedule) Input() json.RawMessage         { return json.RawMessage(s.GetString("input")) }
func (s *Schedule) SetInput(input json.RawMessage) { s.Set("input", string(input)) }
func (s *Schedule) Type() string                   { return s.GetString("type") }
func (s *Schedule) SetType(t string)               { s.Set("type", t) }
func (s *Schedule) Enabled() bool                  { return s.GetBool("enabled") }
func (s *Schedule) SetEnabled(v bool)              { s.Set("enabled", v) }
func (s *Schedule) CronExpression() string         { return s.GetString("cron_expression") }
func (s *Schedule) SetCronExpression(expr string)  { s.Set("cron_expression", expr) }
func (s *Schedule) Jitter() time.Duration {
	jitter := s.GetString("jitter")
	if jitter == "" {
		return 0
	}
	d, _ := time.ParseDuration(jitter)
	return d
}
func (s *Schedule) SetJitter(d time.Duration)  { s.Set("jitter", d.String()) }
func (s *Schedule) ScheduledAt() time.Time     { return s.GetDateTime("scheduled_at").Time() }
func (s *Schedule) SetScheduledAt(t time.Time) { s.Set("scheduled_at", t) }
func (s *Schedule) Created() types.DateTime    { return s.GetDateTime("created") }
func (s *Schedule) Updated() types.DateTime    { return s.GetDateTime("updated") }

func findAllSchedules(app core.App) ([]*Schedule, error) {
	records, err := app.FindAllRecords(collectionSchedules)
	if err != nil {
		return nil, err
	}
	schedules := make([]*Schedule, len(records))
	for i, r := range records {
		schedules[i] = newSchedule(r)
	}
	return schedules, nil
}

func findScheduleByFilter(app core.App, filter string, params map[string]any) (*Schedule, error) {
	record, err := app.FindFirstRecordByFilter(collectionSchedules, filter, params)
	if err != nil {
		return nil, err
	}
	return newSchedule(record), nil
}

func findCompileScheduleByFQN(app core.App, fqn string) (*Schedule, error) {
	return findScheduleByFilter(app,
		"workflow_fqn = {:fqn} && type = {:type}",
		map[string]any{"fqn": fqn, "type": scheduleTypeCompile},
	)
}

func createSchedule(app core.App, fqn string, input json.RawMessage, schedType string, cronExpr string, scheduledAt time.Time) (*Schedule, error) {
	col, err := app.FindCollectionByNameOrId(collectionSchedules)
	if err != nil {
		return nil, err
	}
	record := core.NewRecord(col)
	s := newSchedule(record)
	s.SetWorkflowFQN(fqn)
	s.SetInput(input)
	s.SetType(schedType)
	s.SetEnabled(true)
	if cronExpr != "" {
		s.SetCronExpression(cronExpr)
	}
	if !scheduledAt.IsZero() {
		s.SetScheduledAt(scheduledAt)
	}
	if err := app.Save(s); err != nil {
		return nil, err
	}
	return s, nil
}
