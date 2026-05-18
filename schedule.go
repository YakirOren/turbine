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

var _ core.RecordProxy = (*schedule)(nil)

// schedule is a typed proxy for pt_schedules records.
type schedule struct {
	core.BaseRecordProxy
}

func newSchedule(record *core.Record) *schedule {
	s := &schedule{}
	s.SetProxyRecord(record)
	return s
}

func (s *schedule) WorkflowFQN() string            { return s.GetString("workflow_fqn") }
func (s *schedule) SetWorkflowFQN(fqn string)      { s.Set("workflow_fqn", fqn) }
func (s *schedule) Input() json.RawMessage         { return json.RawMessage(s.GetString("input")) }
func (s *schedule) SetInput(input json.RawMessage) { s.Set("input", string(input)) }
func (s *schedule) Type() string                   { return s.GetString("type") }
func (s *schedule) SetType(t string)               { s.Set("type", t) }
func (s *schedule) Enabled() bool                  { return s.GetBool("enabled") }
func (s *schedule) SetEnabled(v bool)              { s.Set("enabled", v) }
func (s *schedule) CronExpression() string         { return s.GetString("cron_expression") }
func (s *schedule) SetCronExpression(expr string)  { s.Set("cron_expression", expr) }
func (s *schedule) Jitter() time.Duration {
	jitter := s.GetString("jitter")
	if jitter == "" {
		return 0
	}
	d, _ := time.ParseDuration(jitter)
	return d
}
func (s *schedule) SetJitter(d time.Duration)  { s.Set("jitter", d.String()) }
func (s *schedule) ScheduledAt() time.Time     { return s.GetDateTime("scheduled_at").Time() }
func (s *schedule) SetScheduledAt(t time.Time) { s.Set("scheduled_at", t) }
func (s *schedule) Created() types.DateTime    { return s.GetDateTime("created") }
func (s *schedule) Updated() types.DateTime    { return s.GetDateTime("updated") }

func findAllSchedules(app core.App) ([]*schedule, error) {
	records, err := app.FindAllRecords(collectionSchedules)
	if err != nil {
		return nil, err
	}
	schedules := make([]*schedule, len(records))
	for i, r := range records {
		schedules[i] = newSchedule(r)
	}
	return schedules, nil
}

func findScheduleByFilter(app core.App, filter string, params map[string]any) (*schedule, error) {
	record, err := app.FindFirstRecordByFilter(collectionSchedules, filter, params)
	if err != nil {
		return nil, err
	}
	return newSchedule(record), nil
}

func findCompileScheduleByFQN(app core.App, fqn string) (*schedule, error) {
	return findScheduleByFilter(app,
		"workflow_fqn = {:fqn} && type = {:type}",
		map[string]any{"fqn": fqn, "type": scheduleTypeCompile},
	)
}

func createSchedule(app core.App, fqn string, input json.RawMessage, schedType string, cronExpr string, scheduledAt time.Time) (*schedule, error) {
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
