package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/andresaaf/tibia-warden-web/backend/internal/models"
)

var validDifficulties = map[string]struct{}{
	models.DifficultyHarmless:    {},
	models.DifficultyTrivial:     {},
	models.DifficultyEasy:        {},
	models.DifficultyMedium:      {},
	models.DifficultyHard:        {},
	models.DifficultyChallenging: {},
}

// handleListCreatures returns the warden list for the current user, with search
// and difficulty filters applied.
func (s *Server) handleListCreatures(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("search")

	var difficulties []string
	if raw := r.URL.Query().Get("difficulty"); raw != "" {
		for _, d := range strings.Split(raw, ",") {
			d = strings.TrimSpace(d)
			if _, ok := validDifficulties[d]; ok {
				difficulties = append(difficulties, d)
			}
		}
	}

	creatures, err := s.stores.Creatures.List(r.Context(), userID(r), search, difficulties)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load creatures")
		return
	}
	if creatures == nil {
		creatures = []models.Creature{}
	}
	writeJSON(w, http.StatusOK, creatures)
}

// handleMarkKilled marks a creature as killed for the current user.
func (s *Server) handleMarkKilled(w http.ResponseWriter, r *http.Request) {
	creatureID, err := strconv.ParseInt(chiURLParam(r, "creatureID"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid creature id")
		return
	}
	exists, err := s.stores.Creatures.Exists(r.Context(), creatureID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to verify creature")
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "creature not found")
		return
	}
	if err := s.stores.Creatures.SetKilled(r.Context(), userID(r), creatureID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update warden list")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"killed": true})
}

// handleUnmarkKilled clears a creature's killed mark for the current user.
func (s *Server) handleUnmarkKilled(w http.ResponseWriter, r *http.Request) {
	creatureID, err := strconv.ParseInt(chiURLParam(r, "creatureID"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid creature id")
		return
	}
	if err := s.stores.Creatures.UnsetKilled(r.Context(), userID(r), creatureID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update warden list")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"killed": false})
}
