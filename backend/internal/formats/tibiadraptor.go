package formats

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// tibiadraptorIDs is TibiaDraptor's Echo Warden table ({id, name}), keyed by
// their own database IDs rather than Tibia race IDs. It is only the fallback:
// the server replaces it with a live copy at startup (see RefreshTibiaDraptor).
// `go run ./cmd/draptorids` refreshes the fallback.
//
//go:embed tibiadraptor_ids.json
var tibiadraptorIDs []byte

// wikiToDraptor maps our TibiaWiki creature names to TibiaDraptor's where the
// two differ (TibiaWiki disambiguates some names). Everything else matches
// case-insensitively by name.
var wikiToDraptor = map[string]string{
	"Fish (Creature)":          "Fish",
	"Horse (Grey)":             "Horse (Gray)",
	"Monk (Creature)":          "Monk",
	"Nomad (Basic)":            "Nomad",
	"Northern Pike (Creature)": "Northern Pike",
	"Sabretooth (Creature)":    "Sabretooth",
}

type draptorTable struct {
	byName map[string]int    // lower-cased TibiaDraptor name -> id
	byID   map[int]string    // id -> our creature name (aliases applied)
	alias  map[string]string // lower-cased our name -> TibiaDraptor name
}

var embeddedDraptorEntries = sync.OnceValue(func() []DraptorEntry {
	var entries []DraptorEntry
	if err := json.Unmarshal(tibiadraptorIDs, &entries); err != nil {
		panic(fmt.Sprintf("formats: bad tibiadraptor_ids.json: %v", err))
	}
	return entries
})

var embeddedDraptorTable = sync.OnceValue(func() draptorTable {
	return buildDraptorTable(embeddedDraptorEntries())
})

// liveDraptorTable holds the most recent successfully refreshed table, or nil
// until a refresh succeeds.
var liveDraptorTable atomic.Pointer[draptorTable]

// loadDraptorTable returns the live table when one has been fetched, otherwise
// the one built into the binary.
func loadDraptorTable() draptorTable {
	if t := liveDraptorTable.Load(); t != nil {
		return *t
	}
	return embeddedDraptorTable()
}

func buildDraptorTable(entries []DraptorEntry) draptorTable {
	draptorToWiki := make(map[string]string, len(wikiToDraptor))
	alias := make(map[string]string, len(wikiToDraptor))
	for wiki, draptor := range wikiToDraptor {
		draptorToWiki[strings.ToLower(draptor)] = wiki
		alias[strings.ToLower(wiki)] = draptor
	}
	t := draptorTable{
		byName: make(map[string]int, len(entries)),
		byID:   make(map[int]string, len(entries)),
		alias:  alias,
	}
	for _, e := range entries {
		t.byName[strings.ToLower(e.Name)] = e.ID
		name := e.Name
		if wiki, ok := draptorToWiki[strings.ToLower(e.Name)]; ok {
			name = wiki
		}
		t.byID[e.ID] = name
	}
	return t
}

// draptorID returns TibiaDraptor's ID for one of our creature names.
func draptorID(name string) (int, bool) {
	t := loadDraptorTable()
	key := strings.ToLower(name)
	if draptor, ok := t.alias[key]; ok {
		key = strings.ToLower(draptor)
	}
	id, ok := t.byName[key]
	return id, ok
}

type draptorFileEntry struct {
	ID int `json:"id"`
}

type draptorFile struct {
	Version    *int   `json:"version"`
	ExportedAt string `json:"exported_at,omitempty"`
	Sections   struct {
		EchoWardens *[]draptorFileEntry `json:"echo_wardens"`
	} `json:"sections"`
}

var tibiaDraptor = Format{
	ID:     "tibiadraptor",
	Label:  "TibiaDraptor",
	Export: exportTibiaDraptor,
	Import: importTibiaDraptor,
	Covers: func(name string) bool {
		_, ok := draptorID(name)
		return ok
	},
}

func exportTibiaDraptor(names []string, character string, now time.Time) (Export, error) {
	ids := make([]int, 0, len(names))
	var unmapped []string
	for _, n := range names {
		if id, ok := draptorID(n); ok {
			ids = append(ids, id)
		} else {
			unmapped = append(unmapped, n)
		}
	}
	sort.Ints(ids)
	entries := make([]draptorFileEntry, len(ids))
	for i, id := range ids {
		entries[i] = draptorFileEntry{ID: id}
	}

	version := 1
	file := draptorFile{Version: &version, ExportedAt: now.Format(time.RFC3339)}
	file.Sections.EchoWardens = &entries
	body, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return Export{}, err
	}

	who := slug(character)
	if who == "" {
		who = "wardens"
	}
	sort.Strings(unmapped)
	return Export{
		Body:        append(body, '\n'),
		Filename:    fmt.Sprintf("tibiadraptor-echo-wardens-%s-%s.json", who, now.Format("2006-01-02")),
		ContentType: "application/json",
		Unmapped:    unmapped,
	}, nil
}

func importTibiaDraptor(data []byte) ([]string, int, error) {
	var file draptorFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, 0, &ImportError{"not a valid JSON file"}
	}
	if file.Sections.EchoWardens == nil {
		return nil, 0, &ImportError{"not a TibiaDraptor Echo Warden export (no sections.echo_wardens)"}
	}
	if file.Version != nil && *file.Version != 1 {
		return nil, 0, &ImportError{fmt.Sprintf("unsupported TibiaDraptor export version %d", *file.Version)}
	}

	t := loadDraptorTable()
	seen := map[int]bool{}
	var names []string
	unknown := 0
	for _, e := range *file.Sections.EchoWardens {
		if seen[e.ID] {
			continue
		}
		seen[e.ID] = true
		if name, ok := t.byID[e.ID]; ok {
			names = append(names, name)
		} else {
			unknown++
		}
	}
	return names, unknown, nil
}
