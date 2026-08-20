// Command seed imports the Tibia creature list into the database.
//
// Usage:
//
//	seed -file ./data/creatures.json
//	seed -file ./data/creatures.csv
//	seed -file ./data/creatures.json -areas ./data/areas.json
//	seed -areas ./data/areas.json   (areas only, leaves creatures untouched)
//
// At least one of -file / -areas is required.
// JSON format: an array of objects with "name", "difficulty" and optional "rarity"/"imageUrl".
// CSV format:  a header row including "name" and "difficulty" (and optional "rarity", "imageUrl"/"image").
// Areas format (-areas, optional, .json): an array of objects with "name" and a
// "creatures" list of creature names; membership is replaced on each run.
package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/andresaaf/tibia-warden-web/backend/internal/database"
	"github.com/andresaaf/tibia-warden-web/backend/internal/store"
)

type creatureRecord struct {
	Name       string `json:"name"`
	Difficulty string `json:"difficulty"`
	Rarity     string `json:"rarity"`
	ImageURL   string `json:"imageUrl"`
}

type areaRecord struct {
	Name      string   `json:"name"`
	Creatures []string `json:"creatures"`
}

var validDifficulties = map[string]string{
	"harmless":    "Harmless",
	"trivial":     "Trivial",
	"easy":        "Easy",
	"medium":      "Medium",
	"hard":        "Hard",
	"challenging": "Challenging",
}

var validRarities = map[string]string{
	"common":   "Common",
	"uncommon": "Uncommon",
}

func main() {
	filePath := flag.String("file", "", "path to the creatures data file (.json or .csv)")
	areasPath := flag.String("areas", "", "path to an areas data file (.json)")
	flag.Parse()

	if *filePath == "" && *areasPath == "" {
		log.Fatal("provide -file (creatures) and/or -areas")
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL must be set")
	}

	// Load and validate the input files before touching the database, so a bad
	// file fails fast.
	var records []creatureRecord
	if *filePath != "" {
		var err error
		records, err = loadRecords(*filePath)
		if err != nil {
			log.Fatalf("failed to load records: %v", err)
		}
		if len(records) == 0 {
			log.Fatal("no records found in file")
		}
	}
	var areas []areaRecord
	if *areasPath != "" {
		var err error
		areas, err = loadAreas(*areasPath)
		if err != nil {
			log.Fatalf("failed to load areas: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := database.Connect(ctx, dbURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	if err := database.Migrate(ctx, pool); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	stores := store.New(pool)

	if records != nil {
		var imported int
		for _, rec := range records {
			name := strings.TrimSpace(rec.Name)
			diff, ok := validDifficulties[strings.ToLower(strings.TrimSpace(rec.Difficulty))]
			if name == "" || !ok {
				log.Printf("skipping invalid record: name=%q difficulty=%q", rec.Name, rec.Difficulty)
				continue
			}
			rarity := validRarities[strings.ToLower(strings.TrimSpace(rec.Rarity))]
			if err := stores.Creatures.Upsert(ctx, name, diff, rarity, strings.TrimSpace(rec.ImageURL)); err != nil {
				log.Fatalf("failed to upsert %q: %v", name, err)
			}
			imported++
		}
		fmt.Printf("imported/updated %d creatures\n", imported)
	}

	if areas != nil {
		seedAreas(ctx, stores, areas)
	}
}

// seedAreas upserts each area and replaces its membership, resolving creature
// names to ids. Unknown creature names are logged and skipped.
func seedAreas(ctx context.Context, stores *store.Stores, areas []areaRecord) {
	var imported int
	for _, area := range areas {
		name := strings.TrimSpace(area.Name)
		if name == "" {
			log.Printf("skipping area with empty name")
			continue
		}
		wanted := make([]string, 0, len(area.Creatures))
		for _, c := range area.Creatures {
			if c = strings.TrimSpace(c); c != "" {
				wanted = append(wanted, c)
			}
		}
		ids, err := stores.Areas.CreatureIDsByName(ctx, wanted)
		if err != nil {
			log.Fatalf("failed to resolve creatures for area %q: %v", name, err)
		}
		creatureIDs := make([]int64, 0, len(wanted))
		for _, c := range wanted {
			if id, ok := ids[c]; ok {
				creatureIDs = append(creatureIDs, id)
			} else {
				log.Printf("area %q: unknown creature %q, skipping", name, c)
			}
		}
		areaID, err := stores.Areas.UpsertArea(ctx, name)
		if err != nil {
			log.Fatalf("failed to upsert area %q: %v", name, err)
		}
		if err := stores.Areas.ReplaceAreaCreatures(ctx, areaID, creatureIDs); err != nil {
			log.Fatalf("failed to set creatures for area %q: %v", name, err)
		}
		imported++
	}
	fmt.Printf("imported/updated %d areas\n", imported)
}

func loadAreas(path string) ([]areaRecord, error) {
	if strings.ToLower(filepath.Ext(path)) != ".json" {
		return nil, fmt.Errorf("areas file must be .json: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var recs []areaRecord
	if err := json.Unmarshal(data, &recs); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}
	return recs, nil
}

func loadRecords(path string) ([]creatureRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		var recs []creatureRecord
		if err := json.Unmarshal(data, &recs); err != nil {
			return nil, fmt.Errorf("parse json: %w", err)
		}
		return recs, nil
	case ".csv":
		return parseCSV(data)
	default:
		return nil, fmt.Errorf("unsupported file extension: %s", filepath.Ext(path))
	}
}

func parseCSV(data []byte) ([]creatureRecord, error) {
	reader := csv.NewReader(strings.NewReader(string(data)))
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse csv: %w", err)
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("csv must contain a header and at least one row")
	}

	header := rows[0]
	idx := map[string]int{}
	for i, col := range header {
		idx[strings.ToLower(strings.TrimSpace(col))] = i
	}
	nameCol, ok1 := idx["name"]
	diffCol, ok2 := idx["difficulty"]
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("csv header must include 'name' and 'difficulty'")
	}
	imageCol := -1
	if c, ok := idx["imageurl"]; ok {
		imageCol = c
	} else if c, ok := idx["image"]; ok {
		imageCol = c
	}
	rarityCol := -1
	if c, ok := idx["rarity"]; ok {
		rarityCol = c
	} else if c, ok := idx["occurrence"]; ok {
		rarityCol = c
	}

	var recs []creatureRecord
	for _, row := range rows[1:] {
		rec := creatureRecord{}
		if nameCol < len(row) {
			rec.Name = row[nameCol]
		}
		if diffCol < len(row) {
			rec.Difficulty = row[diffCol]
		}
		if rarityCol >= 0 && rarityCol < len(row) {
			rec.Rarity = row[rarityCol]
		}
		if imageCol >= 0 && imageCol < len(row) {
			rec.ImageURL = row[imageCol]
		}
		recs = append(recs, rec)
	}
	return recs, nil
}
