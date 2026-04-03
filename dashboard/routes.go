package dashboard

import (
	"github.com/YakirOren/turbine"
	"github.com/pocketbase/pocketbase/core"
)

func registerRoutes(se *core.ServeEvent, rt *turbine.Runtime) {
	h := &handlers{rt: rt, app: rt.App()}

	se.Router.POST("/api/pt/workflows/{id}/cancel", func(e *core.RequestEvent) error {
		if !e.HasSuperuserAuth() {
			return e.ForbiddenError("", nil)
		}
		return h.cancelWorkflow(e)
	})

	se.Router.POST("/api/pt/workflows/{id}/resume", func(e *core.RequestEvent) error {
		if !e.HasSuperuserAuth() {
			return e.ForbiddenError("", nil)
		}
		return h.resumeWorkflow(e)
	})

	se.Router.GET("/api/pt/workflows/{id}/steps-tree", func(e *core.RequestEvent) error {
		if !e.HasSuperuserAuth() {
			return e.ForbiddenError("", nil)
		}
		return h.stepsTree(e)
	})

	se.Router.GET("/api/pt/queues", func(e *core.RequestEvent) error {
		if !e.HasSuperuserAuth() {
			return e.ForbiddenError("", nil)
		}
		return h.listQueues(e)
	})

	se.Router.GET("/api/pt/queues/{name}/stats", func(e *core.RequestEvent) error {
		if !e.HasSuperuserAuth() {
			return e.ForbiddenError("", nil)
		}
		return h.queueStats(e)
	})

	se.Router.GET("/api/pt/scheduled", func(e *core.RequestEvent) error {
		if !e.HasSuperuserAuth() {
			return e.ForbiddenError("", nil)
		}
		return h.listScheduled(e)
	})

	se.Router.GET("/api/pt/registered", func(e *core.RequestEvent) error {
		if !e.HasSuperuserAuth() {
			return e.ForbiddenError("", nil)
		}
		return h.listRegistered(e)
	})

	se.Router.POST("/api/pt/trigger", func(e *core.RequestEvent) error {
		if !e.HasSuperuserAuth() {
			return e.ForbiddenError("", nil)
		}
		return h.triggerWorkflow(e)
	})

	se.Router.GET("/api/pt/schedules", func(e *core.RequestEvent) error {
		if !e.HasSuperuserAuth() {
			return e.ForbiddenError("", nil)
		}
		return h.listSchedules(e)
	})

	se.Router.POST("/api/pt/schedules", func(e *core.RequestEvent) error {
		if !e.HasSuperuserAuth() {
			return e.ForbiddenError("", nil)
		}
		return h.createSchedule(e)
	})

	se.Router.DELETE("/api/pt/schedules/{id}", func(e *core.RequestEvent) error {
		if !e.HasSuperuserAuth() {
			return e.ForbiddenError("", nil)
		}
		return h.deleteSchedule(e)
	})
}
