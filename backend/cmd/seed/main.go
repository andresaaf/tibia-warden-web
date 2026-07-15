// Command seed imports the Tibia creature list into the database.
//
// Usage:
//
//	seed -file ./data/creatures.json
//	seed -file ./data/creatures.csv
//
// JSON format: an array of objects with "name", "difficulty" and optional "imageUrl".
// CSV format:  a header row including "name" and "difficulty" (and optional "imageUrl"/"image").
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
	ImageURL   string `json:"imageUrl"`
}

var validDifficulties = map[string]string{
	"harmless":    "Harmless",
	"trivial":     "Trivial",
	"easy":        "Easy",
	"medium":      "Medium",
	"hard":        "Hard",
	"challenging": "Challenging",
}

func main() {
	filePath := flag.String("file", "", "path to the creatures data file (.json or .csv)")
	flag.Parse()

	if *filePath == "" {
		log.Fatal("the -file flag is required")
	}
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL must be set")
	}

	records, err := loadRecords(*filePath)
	if err != nil {
		log.Fatalf("failed to load records: %v", err)
	}
	if len(records) == 0 {
		log.Fatal("no records found in file")
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

	var imported int
	for _, rec := range records {
		name := strings.TrimSpace(rec.Name)
		diff, ok := validDifficulties[strings.ToLower(strings.TrimSpace(rec.Difficulty))]
		if name == "" || !ok {
			log.Printf("skipping invalid record: name=%q difficulty=%q", rec.Name, rec.Difficulty)
			continue
		}
		if err := stores.Creatures.Upsert(ctx, name, diff, strings.TrimSpace(rec.ImageURL)); err != nil {
			log.Fatalf("failed to upsert %q: %v", name, err)
		}
		imported++
	}

	fmt.Printf("imported/updated %d creatures\n", imported)
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

	var recs []creatureRecord
	for _, row := range rows[1:] {
		rec := creatureRecord{}
		if nameCol < len(row) {
			rec.Name = row[nameCol]
		}
		if diffCol < len(row) {
			rec.Difficulty = row[diffCol]
		}
		if imageCol >= 0 && imageCol < len(row) {
			rec.ImageURL = row[imageCol]
		}
		recs = append(recs, rec)
	}
	return recs, nil
}
