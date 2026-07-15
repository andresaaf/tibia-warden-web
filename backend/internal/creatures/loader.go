// Package creatures syncs the Tibia creature list from the TibiaWiki API.
package creatures

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/baz/tibia-warden-web/backend/internal/store"
)

// difficulties maps lower-cased bestiary levels to the canonical difficulty.
var difficulties = map[string]string{
	"harmless":    "Harmless",
	"trivial":     "Trivial",
	"easy":        "Easy",
	"medium":      "Medium",
	"hard":        "Hard",
	"challenging": "Challenging",
}

// allowedOccurrence is the set of occurrences we import.
var allowedOccurrence = map[string]bool{"common": true, "uncommon": true}

// apiCreature captures only the fields we need from the (expanded) API response.
type apiCreature struct {
	Name          string `json:"name"`
	BestiaryLevel string `json:"bestiarylevel"`
	Occurrence    string `json:"occurrence"`
}

// Sync fetches creatures from the TibiaWiki API and upserts the ones that have a
// bestiary difficulty and a Common/Uncommon occurrence. It never deletes
// existing rows. Returns the number of creatures imported/updated.
func Sync(ctx context.Context, creatures *store.CreatureStore, apiURL string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "tibia-warden-web/1.0")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("fetch creatures: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("creatures api returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20)) // 64 MB cap
	if err != nil {
		return 0, fmt.Errorf("read creatures response: %w", err)
	}

	var list []apiCreature
	if err := json.Unmarshal(body, &list); err != nil {
		return 0, fmt.Errorf("parse creatures: %w", err)
	}

	var imported int
	for _, c := range list {
		name := strings.TrimSpace(c.Name)
		difficulty, ok := difficulties[strings.ToLower(strings.TrimSpace(c.BestiaryLevel))]
		if name == "" || !ok {
			continue
		}
		if !allowedOccurrence[strings.ToLower(strings.TrimSpace(c.Occurrence))] {
			continue
		}
		if err := creatures.Upsert(ctx, name, difficulty, ""); err != nil {
			return imported, fmt.Errorf("upsert %q: %w", name, err)
		}
		imported++
	}
	return imported, nil
}
