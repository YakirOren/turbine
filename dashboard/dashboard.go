package dashboard

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/YakirOren/turbine"
	"github.com/pocketbase/pocketbase/core"
)

//go:embed dist/*
var dashboardFS embed.FS

// Mount serves the embedded dashboard SPA at /_/turbine/ and registers
// custom API endpoints at /api/pt/. All routes require superuser auth.
func Mount(app core.App, rt *turbine.Runtime) {
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		subFS, err := fs.Sub(dashboardFS, "dist")
		if err != nil {
			return err
		}

		indexHTML, err := fs.ReadFile(subFS, "index.html")
		if err != nil {
			return err
		}

		fileServer := http.StripPrefix("/_/turbine/", http.FileServerFS(subFS))

		se.Router.GET("/_/turbine/{path...}", func(e *core.RequestEvent) error {
			path := e.Request.PathValue("path")

			if path != "" {
				if _, err := fs.Stat(subFS, path); err == nil {
					if strings.HasPrefix(path, "assets/") {
						e.Response.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
					}
					fileServer.ServeHTTP(e.Response, e.Request)
					return nil
				}
			}

			// SPA fallback
			e.Response.Header().Set("Content-Type", "text/html; charset=utf-8")
			e.Response.WriteHeader(http.StatusOK)
			_, writeErr := e.Response.Write(indexHTML)
			return writeErr
		})

		registerRoutes(se, rt)

		return se.Next()
	})
}
