package dashboard

import (
	"github.com/YakirOren/pocketflow"
	"github.com/pocketbase/pocketbase/core"
)

func registerRoutes(se *core.ServeEvent, rt *pocketflow.Runtime) {
	h := &handlers{rt: rt, app: rt.App()}

	se.Router.POST("/api/pf/workflows/{id}/cancel", func(e *core.RequestEvent) error {
		if !e.HasSuperuserAuth() {
			return e.ForbiddenError("", nil)
		}
		return h.cancelWorkflow(e)
	})

	se.Router.POST("/api/pf/workflows/{id}/resume", func(e *core.RequestEvent) error {
		if !e.HasSuperuserAuth() {
			return e.ForbiddenError("", nil)
		}
		return h.resumeWorkflow(e)
	})

	se.Router.GET("/api/pf/workflows/{id}/steps-tree", func(e *core.RequestEvent) error {
		if !e.HasSuperuserAuth() {
			return e.ForbiddenError("", nil)
		}
		return h.stepsTree(e)
	})

	se.Router.GET("/api/pf/queues", func(e *core.RequestEvent) error {
		if !e.HasSuperuserAuth() {
			return e.ForbiddenError("", nil)
		}
		return h.listQueues(e)
	})

	se.Router.GET("/api/pf/queues/{name}/stats", func(e *core.RequestEvent) error {
		if !e.HasSuperuserAuth() {
			return e.ForbiddenError("", nil)
		}
		return h.queueStats(e)
	})
}
