# Dashboard

A full demo with the Turbine dashboard mounted, showing triggerable workflows with parallel steps, app status updates, and product outputs.

**What to notice:**
- `dashboard.Mount(app, rt)` adds the dashboard UI at `/_/turbine/`
- `WithDashboardTrigger()` enables starting workflows from the dashboard
- `ctx.SetAppStatus("phase", "color")` updates the status shown in the dashboard
- `turbine.SendProduct` stores a file artifact (invoice) linked to the workflow
- `ProductSender` interface (`LogSender`) receives products after they're stored

<<< @/../examples/dashboard/main.go
