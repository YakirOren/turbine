// Package dispatch holds the small shared helpers used by both webhook and
// notification delivery: event-name matching, etc.
//
// The per-event orchestration (deciding when to call webhook+notification
// senders for a given workflow status) lives on Runtime at root, since that's
// where the senders are wired together.
package dispatch

// MatchesEvent checks if eventName matches any entry in the events list,
// including the wildcard "workflow.*".
func MatchesEvent(events []string, eventName string) bool {
	for _, ev := range events {
		if ev == eventName || ev == "workflow.*" {
			return true
		}
	}
	return false
}
