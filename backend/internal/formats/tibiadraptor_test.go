package formats

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestDraptorKnownIDs(t *testing.T) {
	for name, want := range map[string]int{"Rat": 381, "Cave Rat": 388, "cave rat": 388} {
		if got, ok := draptorID(name); !ok || got != want {
			t.Errorf("draptorID(%q) = %d, %v; want %d", name, got, ok, want)
		}
	}
}

func TestDraptorAliases(t *testing.T) {
	byName := loadDraptorTable().byName
	for wiki, draptor := range wikiToDraptor {
		id, ok := draptorID(wiki)
		if !ok {
			t.Errorf("alias %q -> %q does not resolve", wiki, draptor)
			continue
		}
		if byName[strings.ToLower(draptor)] != id {
			t.Errorf("alias %q resolved to %d, not %q", wiki, id, draptor)
		}
		if loadDraptorTable().byID[id] != wiki {
			t.Errorf("id %d imports as %q, want %q", id, loadDraptorTable().byID[id], wiki)
		}
	}
}

func TestDraptorTableUnique(t *testing.T) {
	var entries []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(tibiadraptorIDs, &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) < 700 {
		t.Fatalf("only %d entries in the embedded table", len(entries))
	}
	ids, names := map[int]bool{}, map[string]bool{}
	for _, e := range entries {
		key := strings.ToLower(e.Name)
		if ids[e.ID] || names[key] {
			t.Errorf("duplicate entry %d %q", e.ID, e.Name)
		}
		ids[e.ID], names[key] = true, true
	}
}

func TestDraptorRoundTrip(t *testing.T) {
	now := time.Date(2026, 9, 11, 4, 8, 40, 0, time.FixedZone("", -6*3600))
	in := []string{"Rat", "Cave Rat", "Fish (Creature)", "Not A Creature"}

	exp, err := exportTibiaDraptor(in, "Knight Of Rats", now)
	if err != nil {
		t.Fatal(err)
	}
	if want := "tibiadraptor-echo-wardens-knight-of-rats-2026-09-11.json"; exp.Filename != want {
		t.Errorf("filename = %q, want %q", exp.Filename, want)
	}
	if !slices.Equal(exp.Unmapped, []string{"Not A Creature"}) {
		t.Errorf("unmapped = %v", exp.Unmapped)
	}

	var file struct {
		Version    int    `json:"version"`
		ExportedAt string `json:"exported_at"`
		Sections   struct {
			EchoWardens []struct{ ID int } `json:"echo_wardens"`
		} `json:"sections"`
	}
	if err := json.Unmarshal(exp.Body, &file); err != nil {
		t.Fatal(err)
	}
	if file.Version != 1 || file.ExportedAt != "2026-09-11T04:08:40-06:00" {
		t.Errorf("header = %d %q", file.Version, file.ExportedAt)
	}
	if len(file.Sections.EchoWardens) != 3 || file.Sections.EchoWardens[1].ID != 381 {
		t.Errorf("echo_wardens = %+v", file.Sections.EchoWardens)
	}

	names, unknown, err := importTibiaDraptor(exp.Body)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(names)
	if want := []string{"Cave Rat", "Fish (Creature)", "Rat"}; !slices.Equal(names, want) || unknown != 0 {
		t.Errorf("import = %v (unknown %d), want %v", names, unknown, want)
	}
}

func TestDraptorExportEmpty(t *testing.T) {
	exp, err := exportTibiaDraptor(nil, "", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(exp.Body), `"echo_wardens": []`) {
		t.Errorf("empty export should carry an empty list:\n%s", exp.Body)
	}
	if !strings.HasPrefix(exp.Filename, "tibiadraptor-echo-wardens-wardens-") {
		t.Errorf("filename = %q", exp.Filename)
	}
}

func TestDraptorImport(t *testing.T) {
	names, unknown, err := importTibiaDraptor([]byte(`{
		"version": 1,
		"exported_at": "2026-09-11T04:08:40-06:00",
		"sections": {
			"achievements": [{"id": 5}],
			"echo_wardens": [{"id": 381}, {"id": 388}, {"id": 381}, {"id": 999999}]
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"Cave Rat", "Rat"}) || unknown != 1 {
		t.Errorf("import = %v (unknown %d)", names, unknown)
	}

	for _, bad := range []string{`nope`, `{"version":1,"sections":{}}`, `{"version":2,"sections":{"echo_wardens":[]}}`} {
		var ie *ImportError
		if _, _, err := importTibiaDraptor([]byte(bad)); !errors.As(err, &ie) {
			t.Errorf("import(%s) err = %v, want ImportError", bad, err)
		}
	}
}
