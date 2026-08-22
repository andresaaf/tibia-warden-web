package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/andresaaf/tibia-warden-web/backend/internal/mapimg"
	"github.com/andresaaf/tibia-warden-web/backend/internal/store"
)

// normalizeMapCoord returns the coordinate triple only when all three parts are
// present and within the known Tibia map bounds; otherwise it returns nils, so a
// partial or out-of-range pick is stored as "no map" (the opt-in default).
func normalizeMapCoord(x, y, z *int) (*int, *int, *int) {
	if x == nil || y == nil || z == nil {
		return nil, nil, nil
	}
	if !mapimg.ValidCoord(*x, *y, *z) {
		return nil, nil, nil
	}
	return x, y, z
}

// handleAnnouncementMap serves a static PNG map crop for an announcement that has
// a marked spot. It is intentionally public (no auth): Discord's image proxy
// fetches it without our session cookie, and it exposes only a map image — the
// same location already shown in the linked Discord channel.
func (s *Server) handleAnnouncementMap(w http.ResponseWriter, r *http.Request) {
	announcementID, ok := parseID(w, r, "announcementID")
	if !ok {
		return
	}
	ann, err := s.stores.Announcements.GetByID(r.Context(), announcementID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load announcement")
		return
	}
	if ann.MapX == nil || ann.MapY == nil || ann.MapZ == nil {
		http.NotFound(w, r)
		return
	}

	png, err := mapimg.Render(r.Context(), *ann.MapX, *ann.MapY, *ann.MapZ)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to render map")
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(len(png)))
	// Immutable: an announcement's coordinate never changes once posted.
	w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(png)
}
