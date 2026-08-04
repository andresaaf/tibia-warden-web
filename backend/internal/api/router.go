package api

import (
	"net/http"
	"time"

	"github.com/andresaaf/tibia-warden-web/backend/internal/auth"
	"github.com/andresaaf/tibia-warden-web/backend/internal/config"
	"github.com/andresaaf/tibia-warden-web/backend/internal/discord"
	"github.com/andresaaf/tibia-warden-web/backend/internal/store"
	"github.com/andresaaf/tibia-warden-web/backend/internal/ws"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// Server holds shared dependencies for HTTP handlers.
type Server struct {
	cfg    *config.Config
	stores *store.Stores
	oauth  *auth.DiscordProvider
	hub    *ws.Hub
	bot    *discord.Bot
}

// NewRouter wires up all routes and middleware and returns the root handler.
func NewRouter(cfg *config.Config, stores *store.Stores, oauth *auth.DiscordProvider, hub *ws.Hub, bot *discord.Bot) http.Handler {
	s := &Server{cfg: cfg, stores: stores, oauth: oauth, hub: hub, bot: bot}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Route("/api", func(r chi.Router) {
		r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		})

		// Auth (public).
		r.Get("/auth/discord/login", s.handleDiscordLogin)
		r.Get("/auth/discord/callback", s.handleDiscordCallback)

		// Authenticated routes.
		r.Group(func(r chi.Router) {
			r.Use(s.requireAuth)

			r.Get("/me", s.handleMe)
			r.Patch("/me", s.handleUpdateMe)
			r.Post("/auth/logout", s.handleLogout)

			// Home feed (aggregated across the user's groups).
			r.Get("/feed", s.handleListFeed)
			r.Get("/feed/ws", s.handleFeedWebSocket)
			r.Post("/announcements/broadcast", s.handleBroadcastAnnouncement)

			// Warden list.
			r.Get("/creatures", s.handleListCreatures)
			r.Get("/wardens", s.handleListKilled)
			r.Put("/wardens/{creatureID}", s.handleMarkKilled)
			r.Delete("/wardens/{creatureID}", s.handleUnmarkKilled)

			// Statistics leaderboard (global across all users).
			r.Get("/highscores", s.handleListHighscores)

			// Groups.
			r.Get("/groups", s.handleListGroups)
			r.Post("/groups", s.handleCreateGroup)
			r.Post("/groups/join", s.handleRedeemInvite)
			r.Get("/groups/{groupID}", s.handleGetGroup)
			r.Post("/groups/{groupID}/join", s.handleJoinPublic)
			r.Post("/groups/{groupID}/leave", s.handleLeaveGroup)
			r.Delete("/groups/{groupID}", s.handleDeleteGroup)
			r.Get("/groups/{groupID}/members", s.handleListMembers)
			r.Patch("/groups/{groupID}/members/{userID}", s.handleSetMemberRole)
			r.Delete("/groups/{groupID}/members/{userID}", s.handleRemoveMember)
			r.Get("/groups/{groupID}/invites", s.handleListInvites)
			r.Post("/groups/{groupID}/invites", s.handleCreateInvite)
			r.Delete("/groups/{groupID}/invites/{inviteID}", s.handleDeleteInvite)
			r.Post("/groups/{groupID}/discord/link-code", s.handleCreateDiscordLinkCode)
			r.Delete("/groups/{groupID}/discord", s.handleUnlinkDiscord)
			r.Get("/groups/{groupID}/discord/roles", s.handleListDiscordRoles)
			r.Put("/groups/{groupID}/discord/role", s.handleSetDiscordRole)
			r.Delete("/groups/{groupID}/discord/role", s.handleClearDiscordRole)
			r.Put("/groups/{groupID}/discord/autodelete", s.handleSetDiscordAutodelete)

			// Announcements.
			r.Get("/groups/{groupID}/announcements", s.handleListAnnouncements)
			r.Post("/groups/{groupID}/announcements", s.handleCreateAnnouncement)
			r.Post("/announcements/{announcementID}/response", s.handleSetResponse)
			r.Delete("/announcements/{announcementID}/response", s.handleClearResponse)
			r.Post("/announcements/{announcementID}/killed", s.handleMarkAnnouncementKilled)
			r.Post("/announcements/{announcementID}/claim", s.handleClaimAnnouncement)

			// WebSocket (auth via cookie).
			r.Get("/groups/{groupID}/ws", s.handleWebSocket)
		})
	})

	// Serve the built SPA when a static directory is configured.
	if cfg.StaticDir != "" {
		s.mountStatic(r)
	}

	return r
}
