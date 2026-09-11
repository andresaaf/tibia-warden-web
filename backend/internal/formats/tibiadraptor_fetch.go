package formats

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"regexp"
	"sort"
	"strings"
	"time"
)

// DraptorEntry is one row of TibiaDraptor's Echo Warden table.
type DraptorEntry struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

var csrfMeta = regexp.MustCompile(`<meta name="csrf-token" content="([^"]+)"`)

// FetchTibiaDraptorIDs downloads TibiaDraptor's full Echo Warden table from
// baseURL (e.g. https://tibiadraptor.com), sorted by ID. Their list is public
// but served by an internal API that expects a Laravel session cookie and CSRF
// token, so the homepage is fetched first to obtain both. The API pages at a
// fixed 60 rows, so this is one homepage request plus one per page.
func FetchTibiaDraptorIDs(ctx context.Context, baseURL string) ([]DraptorEntry, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 30 * time.Second}

	token, err := fetchCSRF(ctx, client, baseURL)
	if err != nil {
		return nil, fmt.Errorf("csrf: %w", err)
	}

	var all []DraptorEntry
	for p, last := 1, 1; p <= last; p++ {
		pg, err := fetchDraptorPage(ctx, client, baseURL, token, p)
		if err != nil {
			return nil, fmt.Errorf("page %d: %w", p, err)
		}
		last = pg.Meta.LastPage
		all = append(all, pg.Data...)
		if p == last && len(all) != pg.Meta.Total {
			return nil, fmt.Errorf("fetched %d wardens but the API reports %d", len(all), pg.Meta.Total)
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	return all, nil
}

// ValidateDraptorEntries rejects a table that would be worse than the one
// built in: duplicate or blank rows, or far fewer rows (a partial response).
func ValidateDraptorEntries(entries []DraptorEntry) error {
	if min := len(embeddedDraptorEntries()) * 9 / 10; len(entries) < min {
		return fmt.Errorf("only %d wardens, expected at least %d", len(entries), min)
	}
	ids, names := map[int]bool{}, map[string]bool{}
	for _, e := range entries {
		key := strings.ToLower(strings.TrimSpace(e.Name))
		if e.ID <= 0 || key == "" {
			return fmt.Errorf("invalid warden %d %q", e.ID, e.Name)
		}
		if ids[e.ID] || names[key] {
			return fmt.Errorf("duplicate warden %d %q", e.ID, e.Name)
		}
		ids[e.ID], names[key] = true, true
	}
	return nil
}

// RefreshTibiaDraptor fetches the live table and, if it validates, makes it
// the one exports and imports use. On error the current table is kept. The
// server calls this once at startup, like the TibiaWiki creature sync.
func RefreshTibiaDraptor(ctx context.Context, baseURL string) (int, error) {
	entries, err := FetchTibiaDraptorIDs(ctx, baseURL)
	if err != nil {
		return 0, err
	}
	if err := ValidateDraptorEntries(entries); err != nil {
		return 0, err
	}
	t := buildDraptorTable(entries)
	liveDraptorTable.Store(&t)
	return len(entries), nil
}

type draptorPage struct {
	Data []DraptorEntry `json:"data"`
	Meta struct {
		LastPage int `json:"last_page"`
		Total    int `json:"total"`
	} `json:"meta"`
}

func fetchCSRF(ctx context.Context, client *http.Client, baseURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "tibia-warden-web/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	m := csrfMeta.FindSubmatch(body)
	if m == nil {
		return "", fmt.Errorf("no csrf-token meta tag on %s", baseURL)
	}
	return string(m[1]), nil
}

func fetchDraptorPage(ctx context.Context, client *http.Client, baseURL, token string, n int) (*draptorPage, error) {
	payload, _ := json.Marshal(map[string]int{"page": n})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/v1/echo-wardens", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("X-CSRF-TOKEN", token)
	req.Header.Set("User-Agent", "tibia-warden-web/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	var pg draptorPage
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&pg); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return &pg, nil
}
