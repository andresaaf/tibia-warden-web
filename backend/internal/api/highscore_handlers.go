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

// handleListCreatureStats returns per-creature sighting statistics: how often
// each Warden has been announced, how many of those hunts ended in a kill, and
// how many players have it on their Warden List.
func (s *Server) handleListCreatureStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.stores.Creatures.SightingStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load creature statistics")
		return
	}
	if stats == nil {
		stats = []models.CreatureSighting{}
	}
	writeJSON(w, http.StatusOK, stats)
}
