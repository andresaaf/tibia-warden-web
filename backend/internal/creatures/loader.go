// Package creatures syncs the Tibia creature list from the TibiaWiki API.
package creatures

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
// bestiary difficulty and a Common/Uncommon occurrence. It then safely prunes
// creatures that no longer qualify, but only those with no kill history and no
// announcements. Returns the number imported/updated and the number pruned.
func Sync(ctx context.Context, creatures *store.CreatureStore, apiURL string) (imported, pruned int, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "tibia-warden-web/1.0")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("fetch creatures: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("creatures api returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20)) // 64 MB cap
	if err != nil {
		return 0, 0, fmt.Errorf("read creatures response: %w", err)
	}

	var list []apiCreature
	if err := json.Unmarshal(body, &list); err != nil {
		return 0, 0, fmt.Errorf("parse creatures: %w", err)
	}

	kept := make([]string, 0, len(list))
	for _, c := range list {
		name := strings.TrimSpace(c.Name)
		difficulty, ok := difficulties[strings.ToLower(strings.TrimSpace(c.BestiaryLevel))]
		if name == "" || !ok {
			continue
		}
		if !allowedOccurrence[strings.ToLower(strings.TrimSpace(c.Occurrence))] {
			continue
		}
		if err := creatures.Upsert(ctx, name, difficulty, imageURL(name)); err != nil {
			return imported, 0, fmt.Errorf("upsert %q: %w", name, err)
		}
		kept = append(kept, name)
		imported++
	}

	// Guard against a partial/empty response deleting the whole table.
	if imported >= 100 {
		pruned, err = creatures.PruneExcept(ctx, kept)
		if err != nil {
			return imported, 0, fmt.Errorf("prune creatures: %w", err)
		}
	}
	return imported, pruned, nil
}

// imageURL builds a TibiaWiki image URL for a creature via Fandom's Special:FilePath,
// which resolves to the creature's default image.
func imageURL(name string) string {
	return "https://tibia.fandom.com/wiki/Special:FilePath/" + url.PathEscape(name) + ".gif"
}
