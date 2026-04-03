package dashboard

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/YakirOren/pocketflow"
	"github.com/pocketbase/pocketbase/core"
)

//go:embed dist/*
var dashboardFS embed.FS

// Mount serves the embedded dashboard SPA at /_/pocketflow/ and registers
// custom API endpoints at /api/pf/. All routes require superuser auth.
func Mount(app core.App, rt *pocketflow.Runtime) {
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		subFS, err := fs.Sub(dashboardFS, "dist")
		if err != nil {
			return err
		}

		indexHTML, err := fs.ReadFile(subFS, "index.html")
		if err != nil {
			return err
		}

		fileServer := http.StripPrefix("/_/pocketflow/", http.FileServerFS(subFS))

		se.Router.GET("/_/pocketflow/{path...}", func(e *core.RequestEvent) error {
			if !e.HasSuperuserAuth() {
				return e.Redirect(http.StatusTemporaryRedirect, "/_/")
			}

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
