package session

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"time"
)

// CookieName is the name of the session cookie.
const CookieName = "tww_session"

// Duration is how long a session remains valid.
const Duration = 30 * 24 * time.Hour

// NewToken generates a cryptographically random, URL-safe session token.
func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Set writes the session cookie on the response.
func Set(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(Duration),
		MaxAge:   int(Duration.Seconds()),
	})
}

// Clear removes the session cookie.
func Clear(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// Token extracts the session token from the request, if present.
func Token(r *http.Request) (string, bool) {
	c, err := r.Cookie(CookieName)
	if err != nil || c.Value == "" {
		return "", false
	}
	return c.Value, true
}
