package api

import (
	"net/http"

	"github.com/andresaaf/tibia-warden-web/backend/internal/models"
)

// handleListHighscores returns the statistics leaderboard across all users.
func (s *Server) handleListHighscores(w http.ResponseWriter, r *http.Request) {
	entries, err := s.stores.Creatures.Highscores(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load highscores")
		return
	}
	if entries == nil {
		entries = []models.HighscoreEntry{}
	}
	writeJSON(w, http.StatusOK, entries)
}
