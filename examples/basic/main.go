package main

import (
	"context"
	"log"

	"github.com/YakirOren/pbdbos"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func Greet(ctx context.Context, rt *pbdbos.Runtime, name string) (string, error) {
	return "hello " + name, nil
}

func main() {
	app := pocketbase.New()

	rt := pbdbos.Register(app, pbdbos.Config{})

	pbdbos.RegisterWorkflow(rt, Greet)

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		e.Router.POST("/greet/{name}", func(re *core.RequestEvent) error {
			name := re.Request.PathValue("name")

			handle, err := pbdbos.RunWorkflow(rt, Greet, name)
			if err != nil {
				return re.JSON(500, map[string]string{"error": err.Error()})
			}

			result, err := handle.GetResult()
			if err != nil {
				return re.JSON(500, map[string]string{"error": err.Error()})
			}

			return re.JSON(200, map[string]string{"greeting": result})
		})
		return e.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
