package api

import (
	"net/http"
	"strings"

	"github.com/coder/websocket"
)

// handleWebSocket upgrades the connection and subscribes the user to a group's
// live announcement room. Membership is required.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	groupID, ok := parseID(w, r, "groupID")
	if !ok {
		return
	}
	if _, err := s.requireMembership(r, groupID); err != nil {
		writeMembershipError(w, err)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: originPatterns(s.cfg.AllowedOrigins),
	})
	if err != nil {
		return
	}

	s.hub.Serve(r.Context(), conn, []int64{groupID}, userID(r))
}

// handleFeedWebSocket subscribes the user to live updates from all of their
// groups at once (the home dashboard feed).
func (s *Server) handleFeedWebSocket(w http.ResponseWriter, r *http.Request) {
	groupIDs, err := s.stores.Groups.MemberGroupIDs(r.Context(), userID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load groups")
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: originPatterns(s.cfg.AllowedOrigins),
	})
	if err != nil {
		return
	}

	s.hub.Serve(r.Context(), conn, groupIDs, userID(r))
}

// originPatterns strips the scheme from configured origins to build the host
// patterns expected by the websocket Accept handshake check.
func originPatterns(origins []string) []string {
	patterns := make([]string, 0, len(origins))
	for _, o := range origins {
		host := o
		if i := strings.Index(host, "://"); i >= 0 {
			host = host[i+3:]
		}
		if host != "" {
			patterns = append(patterns, host)
		}
	}
	return patterns
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
