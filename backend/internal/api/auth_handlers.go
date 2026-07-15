package api

import (
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/andresaaf/tibia-warden-web/backend/internal/session"
)

const oauthStateCookie = "tww_oauth_state"

// handleDiscordLogin starts the OAuth2 flow by redirecting to Discord.
func (s *Server) handleDiscordLogin(w http.ResponseWriter, r *http.Request) {
	state, err := randomState()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to start login")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     oauthStateCookie,
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.Secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((10 * time.Minute).Seconds()),
	})
	http.Redirect(w, r, s.oauth.AuthCodeURL(state), http.StatusTemporaryRedirect)
}

// handleDiscordCallback completes the OAuth2 flow, creates a session, and
// redirects the user back into the SPA.
func (s *Server) handleDiscordCallback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie(oauthStateCookie)
	if err != nil || stateCookie.Value == "" {
		slog.Warn("discord callback: missing state cookie")
		writeError(w, http.StatusBadRequest, "missing oauth state")
		return
	}
	// Clear the state cookie regardless of outcome.
	http.SetCookie(w, &http.Cookie{Name: oauthStateCookie, Path: "/", MaxAge: -1})

	if r.URL.Query().Get("state") != stateCookie.Value {
		slog.Warn("discord callback: state mismatch")
		writeError(w, http.StatusBadRequest, "invalid oauth state")
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		slog.Warn("discord callback: missing code")
		writeError(w, http.StatusBadRequest, "missing authorization code")
		return
	}

	du, err := s.oauth.Exchange(r.Context(), code)
	if err != nil {
		slog.Error("discord callback: exchange failed", "error", err)
		writeError(w, http.StatusBadGateway, "failed to authenticate with Discord")
		return
	}

	user, err := s.stores.Users.UpsertByDiscord(r.Context(), du.ID, du.Username, du.AvatarURL())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create account")
		return
	}

	token, err := session.NewToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}
	if err := s.stores.Sessions.Create(r.Context(), token, user.ID, time.Now().Add(session.Duration)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to persist session")
		return
	}
	session.Set(w, token, s.cfg.Secure)

	// Send onboarding users to the character-name step.
	dest := strings.TrimRight(s.cfg.PublicBaseURL, "/")
	if user.CharacterName == "" {
		dest += "/onboarding"
	} else {
		dest += "/groups"
	}
	http.Redirect(w, r, dest, http.StatusTemporaryRedirect)
}

// handleMe returns the authenticated user's profile.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	user, err := s.stores.Users.GetByID(r.Context(), userID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load profile")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

// handleUpdateMe sets the user's Tibia character name (onboarding + edits).
func (s *Server) handleUpdateMe(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CharacterName string `json:"characterName"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	name := strings.TrimSpace(body.CharacterName)
	if name == "" || len(name) > 60 {
		writeError(w, http.StatusBadRequest, "character name must be between 1 and 60 characters")
		return
	}
	user, err := s.stores.Users.SetCharacterName(r.Context(), userID(r), name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update profile")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

// handleLogout deletes the current session.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if token, ok := session.Token(r); ok {
		_ = s.stores.Sessions.Delete(r.Context(), token)
	}
	session.Clear(w, s.cfg.Secure)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func randomState() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
