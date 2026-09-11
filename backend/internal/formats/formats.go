// Package formats converts a user's Warden List to and from other trackers'
// file formats. Formats work purely in terms of our canonical creature names
// (TibiaWiki spelling, as stored in the creatures table); resolving those names
// to creature rows is the caller's job.
package formats

import (
	"regexp"
	"strings"
	"time"
)

// Format is one supported export/import file format.
type Format struct {
	ID    string
	Label string
	// Export renders the killed creatures (by name) as a file. character is the
	// user's Tibia character name, used in the filename (may be empty).
	Export func(names []string, character string, now time.Time) (Export, error)
	// Import parses a file into our creature names. unknown counts entries in
	// the file that could not be mapped to a creature name.
	Import func(data []byte) (names []string, unknown int, err error)
	// Covers reports whether the format has an identifier for a creature, i.e.
	// whether a file in this format could list it at all. A replacing import
	// must not unmark creatures the format can't represent.
	Covers func(name string) bool
}

// Export is a rendered file ready to download.
type Export struct {
	Body        []byte
	Filename    string
	ContentType string
	// Unmapped lists killed creatures the format has no identifier for; they are
	// left out of Body.
	Unmapped []string
}

// ImportError is a user-facing problem with an uploaded file.
type ImportError struct{ Msg string }

func (e *ImportError) Error() string { return e.Msg }

var registry = map[string]Format{
	tibiaDraptor.ID: tibiaDraptor,
}

// Get returns the format with the given ID.
func Get(id string) (Format, bool) {
	f, ok := registry[id]
	return f, ok
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// slug lower-cases s and collapses anything that is not a letter or digit into
// single dashes, for use in filenames.
func slug(s string) string {
	return strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(s), "-"), "-")
}
