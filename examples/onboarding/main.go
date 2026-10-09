package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/YakirOren/turbine"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

const (
	topicFirstTodo = "first-todo"
	stageKey       = "stage"
	stageAwaitTodo = "await_todo"
)

type Todo struct {
	Text string `json:"text"`
}

func onboardingID(userID string) string {
	return "onboarding-" + userID
}

// OnboardingWorkflow walks a new user through their first session.
// It starts on login, suspends until the user creates their first todo,
// pauses for a minute, and then finalizes. One workflow covers the whole
// flow, and a crash at any point resumes where it left off.
func OnboardingWorkflow(ctx turbine.Context, userID string) (string, error) {
	_, err := turbine.Do(ctx, func(ctx context.Context) (bool, error) {
		turbine.LoggerFrom(ctx).Info("welcome", "user", userID)
		return true, nil
	}, turbine.WithStepName("greet"))
	if err != nil {
		return "", err
	}

	if err := turbine.SetValue(ctx, stageKey, stageAwaitTodo); err != nil {
		return "", err
	}

	// Suspend until the create-todo handler sends the first todo.
	todo, err := turbine.Recv[Todo](ctx, topicFirstTodo, 24*time.Hour)
	if err != nil {
		return "", err
	}
	if todo.Text == "" {
		_ = turbine.SetValue(ctx, stageKey, "expired")
		return "onboarding expired for " + userID, nil
	}

	if err := turbine.SetValue(ctx, stageKey, "todo_captured"); err != nil {
		return "", err
	}

	// Durable pause. A restart during this minute sleeps only the time left.
	if err := turbine.Sleep(ctx, time.Minute); err != nil {
		return "", err
	}

	_, err = turbine.Do(ctx, func(ctx context.Context) (bool, error) {
		turbine.LoggerFrom(ctx).Info("onboarding complete", "user", userID, "first_todo", todo.Text)
		return true, nil
	}, turbine.WithStepName("finalize"))
	if err != nil {
		return "", err
	}

	if err := turbine.SetValue(ctx, stageKey, "done"); err != nil {
		return "", err
	}
	return "onboarded " + userID, nil
}

func main() {
	app, rt := turbine.NewApp(turbine.Config{})

	turbine.Register(rt, OnboardingWorkflow)

	// Start onboarding on every successful login. The workflow ID is derived
	// from the user, so later logins attach to the same run instead of
	// starting a new one.
	app.OnRecordAuthRequest("users").BindFunc(func(e *core.RecordAuthRequestEvent) error {
		if err := e.Next(); err != nil {
			return err
		}
		if _, err := turbine.Run(rt, OnboardingWorkflow, e.Record.Id,
			turbine.WithID(onboardingID(e.Record.Id)),
		); err != nil {
			e.App.Logger().Error("start onboarding", "user", e.Record.Id, "error", err)
		}
		return nil
	})

	app.OnServe().BindFunc(func(e *turbine.ServeEvent) error {
		e.Router.POST("/todos", func(re *turbine.RequestEvent) error {
			var todo Todo
			if err := re.BindBody(&todo); err != nil || todo.Text == "" {
				return re.BadRequestError("text is required", err)
			}

			// Save the todo here.

			// Messages wait until Recv consumes them, so a todo created before
			// the workflow reaches Recv is not lost. Only the first one is
			// read, later ones are ignored.
			id := onboardingID(re.Auth.Id)
			status, err := turbine.Retrieve[string](rt, id).GetStatus()
			if err != nil {
				re.App.Logger().Warn("no onboarding to notify", "user", re.Auth.Id, "error", err)
				return re.JSON(http.StatusCreated, todo)
			}
			if status.Status == turbine.StatusPending {
				tCtx := rt.NewContext(re.Request.Context())
				if err := turbine.Send(tCtx, id, todo, topicFirstTodo); err != nil {
					return err
				}
			}

			return re.JSON(http.StatusCreated, todo)
		}).Bind(apis.RequireAuth("users"))

		e.Router.GET("/onboarding", func(re *turbine.RequestEvent) error {
			tCtx := rt.NewContext(re.Request.Context())
			stage, err := turbine.GetValue[string](tCtx, onboardingID(re.Auth.Id), stageKey, 0)
			if err != nil {
				return err
			}
			return re.JSON(http.StatusOK, map[string]string{"stage": stage})
		}).Bind(apis.RequireAuth("users"))

		return e.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
