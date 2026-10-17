package ws

// Event type names broadcast over the group WebSocket. Shared by the HTTP API
// and the Discord bot so both surfaces emit identical live updates.
const (
	EventAnnouncementCreated = "announcement.created"
	EventAnnouncementUpdated = "announcement.updated"

	// Respawn checker: one user's check of a subarea changed (payload carries
	// the check, or null when undone away), or the set of checks a group may
	// see changed (a member toggled sharing) and clients should refetch.
	EventCheckerUpdated = "checker.updated"
	EventCheckerResync  = "checker.resync"
)
