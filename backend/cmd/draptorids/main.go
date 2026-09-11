// Command draptorids regenerates the fallback TibiaDraptor Echo Warden ID table
// built into the server (internal/formats/tibiadraptor_ids.json).
//
// This is optional upkeep: the server fetches the live table itself at startup
// and only falls back to the built-in copy when TibiaDraptor can't be reached.
// Refreshing the fallback now and then just keeps that copy recent.
//
// Usage (from backend/):
//
//	go run ./cmd/draptorids
//	go run ./cmd/draptorids -out internal/formats/tibiadraptor_ids.json
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/andresaaf/tibia-warden-web/backend/internal/formats"
)

func main() {
	out := flag.String("out", "internal/formats/tibiadraptor_ids.json", "output file")
	base := flag.String("url", "https://tibiadraptor.com", "TibiaDraptor base URL")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	entries, err := formats.FetchTibiaDraptorIDs(ctx, *base)
	if err != nil {
		log.Fatalf("fetch: %v", err)
	}
	if err := formats.ValidateDraptorEntries(entries); err != nil {
		log.Fatalf("validate: %v", err)
	}
	if err := os.WriteFile(*out, encode(entries), 0o644); err != nil {
		log.Fatalf("write: %v", err)
	}
	fmt.Printf("wrote %d wardens to %s\n", len(entries), *out)
}

// encode writes one entry per line so diffs after a regeneration stay readable.
func encode(entries []formats.DraptorEntry) []byte {
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
