// Command draptorids regenerates the TibiaDraptor Echo Warden ID table used by
// the TibiaDraptor export/import format (internal/formats/tibiadraptor_ids.json).
//
// TibiaDraptor identifies creatures by its own database IDs (not Tibia race IDs).
// Their Echo Warden list is public but served by an internal API that expects a
// Laravel session cookie and CSRF token, so this fetches the homepage first to
// obtain both, then pages through the list.
//
// Usage (from backend/):
//
//	go run ./cmd/draptorids
//	go run ./cmd/draptorids -out internal/formats/tibiadraptor_ids.json
//
// Run it after a Tibia update adds creatures (an export reporting unmapped
// wardens is the signal) and commit the regenerated file.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

const baseURL = "https://tibiadraptor.com"

var csrfMeta = regexp.MustCompile(`<meta name="csrf-token" content="([^"]+)"`)

type entry struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type page struct {
	Data []entry `json:"data"`
	Meta struct {
		LastPage int `json:"last_page"`
		Total    int `json:"total"`
	} `json:"meta"`
}

func main() {
	out := flag.String("out", "internal/formats/tibiadraptor_ids.json", "output file")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 30 * time.Second}

	token, err := fetchCSRF(ctx, client)
	if err != nil {
		log.Fatalf("csrf: %v", err)
	}

	var all []entry
	for p, last := 1, 1; p <= last; p++ {
		pg, err := fetchPage(ctx, client, token, p)
		if err != nil {
			log.Fatalf("page %d: %v", p, err)
		}
		last = pg.Meta.LastPage
		all = append(all, pg.Data...)
		if p == last && len(all) != pg.Meta.Total {
			log.Fatalf("fetched %d wardens but the API reports %d", len(all), pg.Meta.Total)
		}
	}

	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	seenID := map[int]bool{}
	seenName := map[string]bool{}
	for _, e := range all {
		key := strings.ToLower(e.Name)
		if seenID[e.ID] || seenName[key] {
			log.Fatalf("duplicate warden in response: %d %q", e.ID, e.Name)
		}
		seenID[e.ID], seenName[key] = true, true
	}

	if err := os.WriteFile(*out, encode(all), 0o644); err != nil {
		log.Fatalf("write: %v", err)
	}
	fmt.Printf("wrote %d wardens to %s\n", len(all), *out)
}

func fetchCSRF(ctx context.Context, client *http.Client) (string, error) {
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

func fetchPage(ctx context.Context, client *http.Client, token string, n int) (*page, error) {
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
	var pg page
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&pg); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return &pg, nil
}

// encode writes one entry per line so diffs after a regeneration stay readable.
func encode(entries []entry) []byte {
	var b bytes.Buffer
	b.WriteString("[\n")
	for i, e := range entries {
		line, _ := json.Marshal(e)
		b.WriteString("  ")
		b.Write(line)
		if i < len(entries)-1 {
			b.WriteByte(',')
		}
		b.WriteByte('\n')
	}
	b.WriteString("]\n")
	return b.Bytes()
}
