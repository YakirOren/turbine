package main

import (
	"context"
	"fmt"
	"log"

	"github.com/YakirOren/pocketflow"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// CreateUserNote demonstrates accessing the PocketBase app from within a workflow.
// The workflow creates a record in a "notes" collection as a durable step.
func CreateUserNote(ctx pocketflow.Context, input NoteInput) (string, error) {
	noteID, err := pocketflow.Do(ctx, func(stepCtx context.Context) (string, error) {
		collection, err := ctx.App().FindCollectionByNameOrId("notes")
		if err != nil {
			return "", fmt.Errorf("collection not found: %w", err)
		}

		record := core.NewRecord(collection)
		record.Set("user", input.UserID)
		record.Set("title", input.Title)
		record.Set("body", input.Body)

		if err := ctx.App().Save(record); err != nil {
			return "", fmt.Errorf("failed to save note: %w", err)
		}

		return record.Id, nil
	}, pocketflow.WithStepName("create-note"))
	if err != nil {
		return "", err
	}

	return noteID, nil
}

type NoteInput struct {
	UserID string
	Title  string
	Body   string
}

func main() {
	app := pocketbase.New()

	rt := pocketflow.Setup(app, pocketflow.Config{})

	pocketflow.Register(rt, CreateUserNote)

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		e.Router.POST("/notes", func(re *core.RequestEvent) error {
			input := NoteInput{
				UserID: re.Request.FormValue("user"),
				Title:  re.Request.FormValue("title"),
				Body:   re.Request.FormValue("body"),
			}

			handle, err := pocketflow.Run(rt, CreateUserNote, input)
			if err != nil {
				return re.JSON(500, map[string]string{"error": err.Error()})
			}

			noteID, err := handle.GetResult()
			if err != nil {
				return re.JSON(500, map[string]string{"error": err.Error()})
			}

			return re.JSON(201, map[string]string{"note_id": noteID})
		})
		return e.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
