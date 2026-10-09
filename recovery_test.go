package turbine

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/pocketbase/pocketbase/tests"
	"github.com/stretchr/testify/suite"
)

// RecoverySuite runs a workflow on one runtime, stops it mid-wait, and checks
// how a second runtime on the same app recovers it. Each test runs in a
// synctest bubble, so sleeps and timeouts use a fake clock and finish at once.
type RecoverySuite struct {
	suite.Suite
	app             *tests.TestApp
	rt              *Runtime
	shutdownTimeout time.Duration
}

func TestRecoverySuite(t *testing.T) {
	suite.Run(t, new(RecoverySuite))
}

func (s *RecoverySuite) SetupTest() {
	s.shutdownTimeout = 100 * time.Millisecond
}

// bubble runs fn in a synctest bubble with a fresh app. The app and runtimes
// must be created inside the bubble so their timers use the fake clock.
func (s *RecoverySuite) bubble(fn func()) {
	outer := s.T()
	synctest.Test(outer, func(t *testing.T) {
		s.SetT(t)
		defer s.SetT(outer)

		app, err := tests.NewTestApp()
		s.Require().NoError(err)
		s.app = app
		s.rt = nil
		defer app.Cleanup()
		defer func() {
			if s.rt != nil {
				s.rt.Shutdown()
			}
		}()

		fn()
	})
}

// launch starts a new runtime on the suite's app, replacing the previous one.
func (s *RecoverySuite) launch(wf Workflow[string, string]) {
	s.rt = NewRuntime(s.app, Config{ShutdownTimeout: s.shutdownTimeout})
	Register(s.rt, wf)
	s.Require().NoError(s.rt.Launch())
}

func (s *RecoverySuite) start(wf Workflow[string, string], id string) {
	_, err := Run(s.rt, wf, "", WithID(id))
	s.Require().NoError(err)
}

func (s *RecoverySuite) result(id string) string {
	result, err := Retrieve[string](s.rt, id).GetResult()
	s.Require().NoError(err)
	return result
}

func (s *RecoverySuite) status(id string) Status {
	status, err := Retrieve[string](s.rt, id).GetStatus()
	s.Require().NoError(err)
	return status
}

// parkedAt waits until every goroutine is blocked, which means the workflow
// is parked in its wait, and checks that the given step was recorded.
func (s *RecoverySuite) parkedAt(id, stepName string) {
	synctest.Wait()
	steps, err := s.rt.Steps(id)
	s.Require().NoError(err)
	for _, step := range steps {
		if step.FunctionName == stepName {
			return
		}
	}
	s.Require().Failf("workflow not parked", "step %s was never recorded", stepName)
}

func (s *RecoverySuite) TestSleepSurvivesForcedShutdown() {
	const sleepFor = time.Hour
	wf := func(ctx Context, _ string) (string, error) {
		if err := Sleep(ctx, sleepFor); err != nil {
			return "", err
		}
		return "done", nil
	}

	s.bubble(func() {
		begin := time.Now()
		s.launch(wf)
		s.start(wf, "sleep-shutdown")
		s.parkedAt("sleep-shutdown", "pt.sleep")
		time.Sleep(20 * time.Minute)
		s.rt.Shutdown()
		s.Equal(StatusPending, s.status("sleep-shutdown").Status)

		s.launch(wf)
		s.Equal("done", s.result("sleep-shutdown"))
		s.Equal(sleepFor, time.Since(begin), "recovery should sleep only the remaining time")
	})
}

func (s *RecoverySuite) TestRecvReplaysAfterForcedShutdown() {
	wf := func(ctx Context, _ string) (string, error) {
		msg, err := Recv[string](ctx, "greet", time.Hour)
		if err != nil {
			return "", err
		}
		if err := Sleep(ctx, time.Minute); err != nil {
			return "", err
		}
		return msg, nil
	}

	s.bubble(func() {
		s.launch(wf)
		s.start(wf, "recv-replay")
		s.Require().NoError(s.rt.SendToWorkflow("recv-replay", "hello", "greet"))
		s.parkedAt("recv-replay", "pt.sleep")
		s.rt.Shutdown()

		s.launch(wf)
		begin := time.Now()
		s.Equal("hello", s.result("recv-replay"), "the received message should be replayed")
		s.LessOrEqual(time.Since(begin), time.Minute, "Recv should replay without waiting")
	})
}

func (s *RecoverySuite) TestRecvTimeoutSurvivesForcedShutdown() {
	const timeout = time.Hour
	wf := func(ctx Context, _ string) (string, error) {
		msg, err := Recv[string](ctx, "never", timeout)
		if err != nil {
			return "", err
		}
		return "timed out:" + msg, nil
	}

	s.bubble(func() {
		begin := time.Now()
		s.launch(wf)
		s.start(wf, "recv-timeout")
		s.parkedAt("recv-timeout", "pt.recv")
		time.Sleep(20 * time.Minute)
		s.rt.Shutdown()

		s.launch(wf)
		s.Equal("timed out:", s.result("recv-timeout"))
		s.Equal(timeout, time.Since(begin), "recovery should wait only the remaining timeout")
	})
}

func (s *RecoverySuite) TestShutdownInterruptsSleepWithoutCountingAttempt() {
	s.shutdownTimeout = 10 * time.Second
	wf := func(ctx Context, _ string) (string, error) {
		if err := Sleep(ctx, time.Hour); err != nil {
			return "", err
		}
		return "done", nil
	}

	s.bubble(func() {
		s.launch(wf)
		s.start(wf, "sleep-drain")
		s.parkedAt("sleep-drain", "pt.sleep")
		begin := time.Now()
		s.rt.Shutdown()
		s.Zero(time.Since(begin), "shutdown should interrupt Sleep instead of waiting out the timeout")
		before := s.status("sleep-drain")
		s.Equal(StatusPending, before.Status)

		for range 3 {
			s.launch(wf)
			s.parkedAt("sleep-drain", "pt.sleep")
			s.rt.Shutdown()
		}

		after := s.status("sleep-drain")
		s.Equal(StatusPending, after.Status)
		s.Equal(before.Attempts, after.Attempts, "interrupted recoveries should not count as attempts")
	})
}

func (s *RecoverySuite) TestRecvClearsRecoveringWhenLive() {
	wf := func(ctx Context, _ string) (string, error) {
		if _, err := Do(ctx, func(context.Context) (bool, error) { return true, nil }, WithStepName("before")); err != nil {
			return "", err
		}
		msg, err := Recv[string](ctx, "go", time.Hour)
		if err != nil {
			return "", err
		}
		if err := SetAppStatus(ctx, "got-"+msg, "green"); err != nil {
			return "", err
		}
		return msg, nil
	}

	s.bubble(func() {
		s.launch(wf)
		s.start(wf, "recv-recovering")
		s.parkedAt("recv-recovering", "pt.recv")
		s.rt.Shutdown()

		s.launch(wf)
		s.Require().NoError(s.rt.SendToWorkflow("recv-recovering", "hi", "go"))
		s.Equal("hi", s.result("recv-recovering"))
		s.Equal("got-hi", s.status("recv-recovering").AppStatus, "app status should be set after a live Recv")
	})
}
