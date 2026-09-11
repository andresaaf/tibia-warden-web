package formats

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

// fakeDraptor serves a TibiaDraptor-shaped homepage and paged Echo Warden API
// over entries, enforcing the session cookie + CSRF token handshake.
func fakeDraptor(t *testing.T, entries []DraptorEntry) *httptest.Server {
	t.Helper()
	const token = "tok123"
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "s1", Path: "/"})
		w.Write([]byte(`<html><head><meta name="csrf-token" content="` + token + `"></head></html>`))
	})
	mux.HandleFunc("POST /api/v1/echo-wardens", func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie("session"); err != nil || c.Value != "s1" || r.Header.Get("X-CSRF-TOKEN") != token {
			http.Error(w, "csrf mismatch", 419)
			return
		}
		var req struct{ Page int }
		json.NewDecoder(r.Body).Decode(&req)
		const per = 60
		last := (len(entries) + per - 1) / per
		lo, hi := min((req.Page-1)*per, len(entries)), min(req.Page*per, len(entries))
		var resp draptorPage
		resp.Data = entries[lo:hi]
		resp.Meta.LastPage = last
		resp.Meta.Total = len(entries)
		json.NewEncoder(w).Encode(resp)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestRefreshTibiaDraptorSwapsTable(t *testing.T) {
	t.Cleanup(func() { liveDraptorTable.Store(nil) })

	live := slices.Clone(embeddedDraptorEntries())
	for i := range live {
		if live[i].Name == "Rat" {
			live[i].ID = 99999 // renumbered upstream
		}
	}
	srv := fakeDraptor(t, live)

	n, err := RefreshTibiaDraptor(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(live) {
		t.Errorf("refreshed %d, want %d", n, len(live))
	}
	if id, _ := draptorID("Rat"); id != 99999 {
		t.Errorf("Rat = %d after refresh, want 99999", id)
	}
	if id, _ := draptorID("Fish (Creature)"); id == 0 {
		t.Error("aliases lost after refresh")
	}
}

func TestRefreshTibiaDraptorKeepsTableOnBadData(t *testing.T) {
	t.Cleanup(func() { liveDraptorTable.Store(nil) })

	embedded := embeddedDraptorEntries()
	dup := slices.Clone(embedded)
	dup[1].Name = dup[0].Name

	for name, entries := range map[string][]DraptorEntry{
		"partial":   embedded[:10],
		"duplicate": dup,
	} {
		srv := fakeDraptor(t, entries)
		if _, err := RefreshTibiaDraptor(context.Background(), srv.URL); err == nil {
			t.Errorf("%s: refresh accepted bad data", name)
		}
		if id, _ := draptorID("Rat"); id != 381 {
			t.Errorf("%s: Rat = %d, want built-in 381", name, id)
		}
	}

	down := httptest.NewServer(http.NotFoundHandler())
	down.Close()
	if _, err := RefreshTibiaDraptor(context.Background(), down.URL); err == nil {
		t.Error("refresh against an unreachable server succeeded")
	}
}
