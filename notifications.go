package turbine

// SendNotification sends a custom message to the alert channel matching name.
// Disabled channels are a silent no-op (returns nil), matching the event-driven
// dispatch behavior, toggling a channel off mutes both. If multiple channels
// share the same name, the first match wins. Returns an error only if no
// channel with that name exists or if delivery fails.
func (rt *Runtime) SendNotification(name, message string) error {
	return rt.notifications.SendCustom(name, message)
}

// TestAlertChannel sends a test notification to verify the channel works.
func (rt *Runtime) TestAlertChannel(id string) error {
	return rt.notifications.Test(id)
}
