package api

import (
	"net/http"

	"github.com/andresaaf/tibia-warden-web/backend/internal/models"
)

// handleListAreas returns every area with its creatures, each carrying the
// current user's killed state so the client can compute area completion. Areas
// always show full membership — the warden-list filters do not apply here.
func (s *Server) handleListAreas(w http.ResponseWriter, r *http.Request) {
	areas, err := s.stores.Areas.List(r.Context(), userID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load areas")
		return
	}
	if areas == nil {
		areas = []models.Area{}
	}
	writeJSON(w, http.StatusOK, areas)
}
