package main

import (
	"context"
	"fmt"
	"log"

	"github.com/YakirOren/turbine"
	"github.com/pocketbase/pocketbase/core"
)

// CreateUserNote demonstrates accessing the underlying app from within a workflow.
// The workflow creates a record in a "notes" collection as a durable step.
func CreateUserNote(ctx turbine.Context, input NoteInput) (string, error) {
	noteID, err := turbine.Do(ctx, func(stepCtx context.Context) (string, error) {
		app := turbine.AppFrom(stepCtx)
		collection, err := app.FindCollectionByNameOrId("notes")
		if err != nil {
			return "", fmt.Errorf("collection not found: %w", err)
		}

		record := core.NewRecord(collection)
		record.Set("user", input.UserID)
		record.Set("title", input.Title)
		record.Set("body", input.Body)

		if err := app.Save(record); err != nil {
			return "", fmt.Errorf("failed to save note: %w", err)
		}

		return record.Id, nil
	}, turbine.WithStepName("create-note"))
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
	app, rt := turbine.NewApp(turbine.Config{})

	turbine.Register(rt, CreateUserNote)

	app.OnServe().BindFunc(func(e *turbine.ServeEvent) error {
		e.Router.POST("/notes", func(re *turbine.RequestEvent) error {
			input := NoteInput{
				UserID: re.Request.FormValue("user"),
				Title:  re.Request.FormValue("title"),
				Body:   re.Request.FormValue("body"),
			}

			handle, err := turbine.Run(rt, CreateUserNote, input)
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
