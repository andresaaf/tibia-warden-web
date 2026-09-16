// Package creatures syncs the Tibia creature list from the TibiaWiki API.
package creatures

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/andresaaf/tibia-warden-web/backend/internal/models"
	"github.com/andresaaf/tibia-warden-web/backend/internal/store"
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

// occurrences maps lower-cased TibiaWiki occurrences to the canonical rarity.
// Only these occurrences are imported.
var occurrences = map[string]string{
	"common":   models.RarityCommon,
	"uncommon": models.RarityUncommon,
}

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

	type entry struct{ name, difficulty, rarity string }
	var entries []entry
	for _, c := range list {
		name := strings.TrimSpace(c.Name)
		difficulty, ok := difficulties[strings.ToLower(strings.TrimSpace(c.BestiaryLevel))]
		if name == "" || !ok {
			continue
		}
		rarity, ok := occurrences[strings.ToLower(strings.TrimSpace(c.Occurrence))]
		if !ok {
			continue
		}
		entries = append(entries, entry{name, difficulty, rarity})
	}

	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.name
	}
	images, err := resolveImages(ctx, client, wikiAPIURL, names)
	if err != nil {
		// Not fatal: fall back to computed URLs, which cover every creature
		// whose image file isn't a redirect to another creature's.
		slog.Warn("resolving creature images failed; using computed URLs", "error", err)
	}

	kept := make([]string, 0, len(entries))
	for _, e := range entries {
		img := images[e.name]
		if img == "" {
			img = imageURL(e.name)
		}
		if err := creatures.Upsert(ctx, e.name, e.difficulty, e.rarity, img); err != nil {
			return imported, 0, fmt.Errorf("upsert %q: %w", e.name, err)
		}
		kept = append(kept, e.name)
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

// imageURL computes the direct Fandom CDN URL of a creature's TibiaWiki image,
// the fallback when resolveImages has no answer. It misses creatures whose
// file page redirects to another creature's image (Hot Dog -> Dog).
// MediaWiki stores a file under /<h[0]>/<h[0:2]>/ where h is the hex MD5 of
// its name (spaces as underscores). We link the CDN directly rather than
// tibia.fandom.com's Special:FilePath redirect: that host sits behind a
// Cloudflare challenge which answers cross-site <img> loads with an HTML 403,
// so browsers (Firefox's OpaqueResponseBlocking in particular) refuse it.
func imageURL(name string) string {
	file := strings.ReplaceAll(name, " ", "_") + ".gif"
	h := fmt.Sprintf("%x", md5.Sum([]byte(file)))
	return "https://static.wikia.nocookie.net/tibia/images/" + h[:1] + "/" + h[:2] + "/" +
		url.PathEscape(file) + "/revision/latest?path-prefix=en"
}
