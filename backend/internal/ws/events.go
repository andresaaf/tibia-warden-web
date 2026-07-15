package ws

// Event type names broadcast over the group WebSocket. Shared by the HTTP API
// and the Discord bot so both surfaces emit identical live updates.
const (
	EventAnnouncementCreated = "announcement.created"
	EventAnnouncementUpdated = "announcement.updated"
)
