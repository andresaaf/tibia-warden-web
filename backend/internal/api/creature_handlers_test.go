package api

import (
	"slices"
	"testing"

	"github.com/andresaaf/tibia-warden-web/backend/internal/models"
)

func TestPlanImport(t *testing.T) {
	creatures := []models.Creature{
		{ID: 1, Name: "Rat", Killed: true},       // in file, already marked
		{ID: 2, Name: "Cave Rat"},                // in file, new
		{ID: 3, Name: "Dragon", Killed: true},    // marked, missing from file
		{ID: 4, Name: "Brand New", Killed: true}, // marked, format can't list it
		{ID: 5, Name: "Troll"},                   // untouched
	}
	names := []string{"rat", "Cave Rat", "Not Ours"}
	covers := func(name string) bool { return name != "Brand New" }

	add := planImport(creatures, names, models.ImportModeAdd, covers)
	if !slices.Equal(add.add, []int64{2}) || len(add.remove) != 0 {
		t.Errorf("add mode: add=%v remove=%v", add.add, add.remove)
	}
	if r := add.result; r.AlreadyMarked != 1 || r.Unknown != 1 || r.KeptUnsupported != 0 || r.RemovedNames == nil {
		t.Errorf("add mode result = %+v", r)
	}

	rep := planImport(creatures, names, models.ImportModeReplace, covers)
	if !slices.Equal(rep.add, []int64{2}) || !slices.Equal(rep.remove, []int64{3}) {
		t.Errorf("replace mode: add=%v remove=%v", rep.add, rep.remove)
	}
	r := rep.result
	if r.Added != 1 || r.Removed != 1 || r.KeptUnsupported != 1 || r.AlreadyMarked != 1 {
		t.Errorf("replace mode result = %+v", r)
	}
	if !slices.Equal(r.AddedNames, []string{"Cave Rat"}) || !slices.Equal(r.RemovedNames, []string{"Dragon"}) {
		t.Errorf("replace mode names = %v / %v", r.AddedNames, r.RemovedNames)
	}
}
