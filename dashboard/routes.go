package dashboard

import (
	"github.com/YakirOren/turbine"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func registerRoutes(se *core.ServeEvent, rt *turbine.Runtime) {
	h := &handlers{rt: rt, app: rt.App()}

	apiGroup := se.Router.Group("/api/pt")
	apiGroup.Bind(apis.RequireSuperuserAuth())

	apiGroup.POST("/workflows/{id}/cancel", h.cancelWorkflow)
	apiGroup.POST("/workflows/{id}/resume", h.resumeWorkflow)
	apiGroup.GET("/workflows/{id}/steps-tree", h.stepsTree)
	apiGroup.GET("/queues", h.listQueues)
	apiGroup.GET("/queues/{name}/stats", h.queueStats)
	apiGroup.POST("/trigger", h.triggerWorkflow)
	apiGroup.POST("/workflows/{id}/approve", h.approveWorkflow)
	apiGroup.POST("/alert-channels/{id}/test", h.testAlertChannel)
	apiGroup.GET("/calendar", h.calendarStats)
}
