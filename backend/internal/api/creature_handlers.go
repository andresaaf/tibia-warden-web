package api

import (
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/andresaaf/tibia-warden-web/backend/internal/formats"
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

var validRarities = map[string]struct{}{
	models.RarityCommon:   {},
	models.RarityUncommon: {},
}

// handleListCreatures returns the warden list for the current user, with search,
// difficulty and rarity filters applied.
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

	var rarities []string
	if raw := r.URL.Query().Get("rarity"); raw != "" {
		for _, rr := range strings.Split(raw, ",") {
			rr = strings.TrimSpace(rr)
			if _, ok := validRarities[rr]; ok {
				rarities = append(rarities, rr)
			}
		}
	}

	creatures, err := s.stores.Creatures.List(r.Context(), userID(r), search, difficulties, rarities)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load creatures")
		return
	}
	if creatures == nil {
		creatures = []models.Creature{}
	}
	writeJSON(w, http.StatusOK, creatures)
}

// handleListKilled returns the creature IDs the current user has marked killed.
func (s *Server) handleListKilled(w http.ResponseWriter, r *http.Request) {
	ids, err := s.stores.Creatures.KilledIDs(r.Context(), userID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load warden list")
		return
	}
	if ids == nil {
		ids = []int64{}
	}
	writeJSON(w, http.StatusOK, ids)
}

// handleExportWardens downloads the current user's killed wardens in another
// tracker's file format (?format=, see internal/formats). Killed creatures the
// format has no identifier for are left out and listed, path-escaped and
// comma-separated, in the X-Export-Unmapped header.
func (s *Server) handleExportWardens(w http.ResponseWriter, r *http.Request) {
	format, ok := formats.Get(r.URL.Query().Get("format"))
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown export format")
		return
	}
	user, err := s.stores.Users.GetByID(r.Context(), userID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load profile")
		return
	}
	creatures, err := s.stores.Creatures.List(r.Context(), user.ID, "", nil, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load warden list")
		return
	}
	var names []string
	for _, c := range creatures {
		if c.Killed {
			names = append(names, c.Name)
		}
	}

	exp, err := format.Export(names, user.CharacterName, time.Now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to build export")
		return
	}
	if len(exp.Unmapped) > 0 {
		slog.Warn("warden export: creatures without an identifier in format",
			"format", format.ID, "creatures", exp.Unmapped)
		escaped := make([]string, len(exp.Unmapped))
		for i, n := range exp.Unmapped {
			escaped[i] = url.PathEscape(n)
		}
		w.Header().Set("X-Export-Unmapped", strings.Join(escaped, ","))
	}
	w.Header().Set("Content-Type", exp.ContentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": exp.Filename}))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(exp.Body)
}

// handleImportWardens reads another tracker's export file (the raw request
// body, ?format= as for export) and marks the wardens it lists as killed. It
// only adds marks, never removes them.
func (s *Server) handleImportWardens(w http.ResponseWriter, r *http.Request) {
	format, ok := formats.Get(r.URL.Query().Get("format"))
	if !ok {
		writeError(w, http.StatusBadRequest, "unknown import format")
		return
	}
	// Generous cap: a tracker's full backup can carry many other sections.
	data, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read file")
		return
	}
	names, unknown, err := format.Import(data)
	if err != nil {
		var ie *formats.ImportError
		if errors.As(err, &ie) {
			writeError(w, http.StatusBadRequest, ie.Msg)
		} else {
			writeError(w, http.StatusInternalServerError, "failed to read file")
		}
		return
	}

	creatures, err := s.stores.Creatures.List(r.Context(), userID(r), "", nil, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load warden list")
		return
	}
	byName := make(map[string]int64, len(creatures))
	for _, c := range creatures {
		byName[strings.ToLower(c.Name)] = c.ID
	}
	ids := make([]int64, 0, len(names))
	for _, n := range names {
		if id, ok := byName[strings.ToLower(n)]; ok {
			ids = append(ids, id)
		} else {
			unknown++
		}
	}

	added, err := s.stores.Creatures.SetKilledMany(r.Context(), userID(r), ids)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update warden list")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{
		"added":         added,
		"alreadyMarked": int64(len(ids)) - added,
		"unknown":       int64(unknown),
	})
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
