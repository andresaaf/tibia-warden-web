package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/andresaaf/tibia-warden-web/backend/internal/models"
	"github.com/andresaaf/tibia-warden-web/backend/internal/store"
	"github.com/andresaaf/tibia-warden-web/backend/internal/ws"
)

// checkerUpdate is the checker.updated payload: one user's check of one
// subarea, or Check == nil when it was undone away entirely.
type checkerUpdate struct {
	SubareaID int64                `json:"subareaId"`
	UserID    int64                `json:"userId"`
	Check     *models.SubareaCheck `json:"check"`
}

// handleGetChecker returns everything the checker page needs besides the area
// tree: the server's clock (so clients agree on countdowns), the user's
// favorites, the checks visible to them, and their per-group share settings.
func (s *Server) handleGetChecker(w http.ResponseWriter, r *http.Request) {
	ctx, uid := r.Context(), userID(r)
	favorites, err := s.stores.Checker.Favorites(ctx, uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load favorites")
		return
	}
	checks, err := s.stores.Checker.Visible(ctx, uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load checks")
		return
	}
	sharing, err := s.stores.Checker.ShareSettings(ctx, uid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load share settings")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"now":       time.Now(),
		"favorites": favorites,
		"checks":    checks,
		"sharing":   sharing,
	})
}

// handleCheckSubarea records (or restarts) the user's check of a subarea and
// pushes it live to every group they share checks with.
func (s *Server) handleCheckSubarea(w http.ResponseWriter, r *http.Request) {
	subareaID, ok := s.parseSubareaID(w, r)
	if !ok {
		return
	}
	uid := userID(r)
	check, err := s.stores.Checker.Check(r.Context(), uid, subareaID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record check")
		return
	}
	s.broadcastCheck(r.Context(), uid, checkerUpdate{SubareaID: subareaID, UserID: uid, Check: check})
	writeJSON(w, http.StatusOK, map[string]any{"check": check})
}

// handleUndoCheck reverts the user's latest check of a subarea to their
// previous one (or removes it) and pushes the result live.
func (s *Server) handleUndoCheck(w http.ResponseWriter, r *http.Request) {
	subareaID, ok := s.parseSubareaID(w, r)
	if !ok {
		return
	}
	uid := userID(r)
	check, err := s.stores.Checker.Undo(r.Context(), uid, subareaID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to undo check")
		return
	}
	s.broadcastCheck(r.Context(), uid, checkerUpdate{SubareaID: subareaID, UserID: uid, Check: check})
	writeJSON(w, http.StatusOK, map[string]any{"check": check})
}

func (s *Server) handleAddCheckerFavorite(w http.ResponseWriter, r *http.Request) {
	subareaID, ok := s.parseSubareaID(w, r)
	if !ok {
		return
	}
	if err := s.stores.Checker.AddFavorite(r.Context(), userID(r), subareaID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add favorite")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"favorite": true})
}

func (s *Server) handleRemoveCheckerFavorite(w http.ResponseWriter, r *http.Request) {
	subareaID, ok := parseID(w, r, "subareaID")
	if !ok {
		return
	}
	if err := s.stores.Checker.RemoveFavorite(r.Context(), userID(r), subareaID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to remove favorite")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"favorite": false})
}

// handleSetCheckerShare toggles whether the user's checks are shared with a
// group. That group's clients are told to resync, since the set of checks they
// may see just changed.
func (s *Server) handleSetCheckerShare(w http.ResponseWriter, r *http.Request) {
	groupID, ok := parseID(w, r, "groupID")
	if !ok {
		return
	}
	var body struct {
		Share bool `json:"share"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := s.stores.Checker.SetShare(r.Context(), groupID, userID(r), body.Share); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusForbidden, "you are not a member of this group")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update share setting")
		return
	}
	s.hub.Broadcast(groupID, ws.EventCheckerResync, struct{}{})
	writeJSON(w, http.StatusOK, map[string]bool{"share": body.Share})
}

// parseSubareaID reads {subareaID} and verifies the subarea exists.
func (s *Server) parseSubareaID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	subareaID, ok := parseID(w, r, "subareaID")
	if !ok {
		return 0, false
	}
	exists, err := s.stores.Areas.SubareaExists(r.Context(), subareaID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to look up subarea")
		return 0, false
	}
	if !exists {
		writeError(w, http.StatusNotFound, "subarea not found")
		return 0, false
	}
	return subareaID, true
}

// broadcastCheck pushes a check change to every group uid shares checks with.
// A viewer in several of those groups receives it more than once; clients
// upsert by (subareaId, userId).
func (s *Server) broadcastCheck(ctx context.Context, uid int64, update checkerUpdate) {
	groupIDs, err := s.stores.Checker.SharingGroupIDs(ctx, uid)
	if err != nil {
		slog.Warn("checker: failed to resolve share groups", "user", uid, "error", err)
		return
	}
	for _, gid := range groupIDs {
		s.hub.Broadcast(gid, ws.EventCheckerUpdated, update)
	}
}
