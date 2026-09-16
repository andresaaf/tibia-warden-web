package creatures

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// wikiAPIURL is TibiaWiki's MediaWiki API. Unlike the wiki pages themselves it
// isn't behind Fandom's Cloudflare challenge, so the server can query it.
const wikiAPIURL = "https://tibia.fandom.com/api.php"

// imageBatch is the most titles an anonymous MediaWiki query accepts at once.
const imageBatch = 50

// resolveImages asks the wiki for the CDN URL of each creature's image file,
// following file redirects: several variants have no image of their own and
// redirect to the base creature's (Ravenous Lava Lurker -> Lava Lurker), which
// a URL computed from the name alone can't know. Returns name -> URL; names
// the wiki has no image for are absent. On error, the batches resolved so far
// are still returned.
func resolveImages(ctx context.Context, client *http.Client, apiURL string, names []string) (map[string]string, error) {
	out := make(map[string]string, len(names))
	for start := 0; start < len(names); start += imageBatch {
		batch := names[start:min(start+imageBatch, len(names))]
		if err := resolveImageBatch(ctx, client, apiURL, batch, out); err != nil {
			return out, err
		}
	}
	return out, nil
}

type imageQueryResponse struct {
	Query struct {
		Normalized []titleMapping `json:"normalized"`
		Redirects  []titleMapping `json:"redirects"`
		Pages      map[string]struct {
			Title     string `json:"title"`
			ImageInfo []struct {
				URL string `json:"url"`
			} `json:"imageinfo"`
		} `json:"pages"`
	} `json:"query"`
}

type titleMapping struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func resolveImageBatch(ctx context.Context, client *http.Client, apiURL string, names []string, out map[string]string) error {
	titles := make([]string, len(names))
	for i, n := range names {
		titles[i] = "File:" + n + ".gif"
	}
	q := url.Values{
		"action":    {"query"},
		"format":    {"json"},
		"prop":      {"imageinfo"},
		"iiprop":    {"url"},
		"redirects": {"1"},
		"titles":    {strings.Join(titles, "|")},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL+"?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "tibia-warden-web/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("query wiki images: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("wiki api returned status %d", resp.StatusCode)
	}
	var r imageQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return fmt.Errorf("parse wiki images: %w", err)
	}

	urls := make(map[string]string, len(r.Query.Pages))
	for _, p := range r.Query.Pages {
		if len(p.ImageInfo) > 0 && p.ImageInfo[0].URL != "" {
			urls[p.Title] = p.ImageInfo[0].URL
		}
	}
	normalized := toMap(r.Query.Normalized)
	redirects := toMap(r.Query.Redirects)
	for i, title := range titles {
		if t, ok := normalized[title]; ok {
			title = t
		}
		// Follow redirect chains, bounded in case of a cycle.
		for hops := 0; hops < 5; hops++ {
			t, ok := redirects[title]
			if !ok {
				break
			}
			title = t
		}
		if u := urls[title]; u != "" {
			out[names[i]] = u
		}
	}
	return nil
}

func toMap(ms []titleMapping) map[string]string {
	m := make(map[string]string, len(ms))
	for _, x := range ms {
		m[x.From] = x.To
	}
	return m
}
