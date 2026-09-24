package api

import "testing"

func TestParseAttendPricing(t *testing.T) {
	got, err := parseAttendPricing(map[string]map[string]string{
		"Common":   {"Easy": "5k", "Medium": "", "Hard": "30,000"},
		"Uncommon": {"Hard": "0.1kk"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]map[string]int64{
		"Common":   {"Easy": 5000, "Hard": 30000},
		"Uncommon": {"Hard": 100000},
	}
	for rarity, prices := range want {
		if len(got[rarity]) != len(prices) {
			t.Fatalf("%s = %v; want %v", rarity, got[rarity], prices)
		}
		for d, v := range prices {
			if got[rarity][d] != v {
				t.Errorf("%s %s = %d; want %d", rarity, d, got[rarity][d], v)
			}
		}
	}

	// Uncommon-only pricing is fine, and Common is still present but empty.
	only, err := parseAttendPricing(map[string]map[string]string{"Uncommon": {"Hard": "30k"}})
	if err != nil || len(only["Common"]) != 0 || only["Uncommon"]["Hard"] != 30000 {
		t.Errorf("uncommon-only = %v, %v", only, err)
	}

	bad := []map[string]map[string]string{
		{},                                          // nothing set
		{"Common": {"Hard": "", "Easy": "0"}},       // all free
		{"Common": {"Hard": "abc"}},                 // bad price
		{"Common": {"Epic": "5k"}},                  // unknown difficulty
		{"Rare": {"Hard": "5k"}},                    // unknown rarity
		{"Common": {"Hard": "5k"}, "Rare": {}},      // unknown rarity, empty
		{"Uncommon": {"Hard": "5k", "Easy": "-1k"}}, // negative
	}
	for _, c := range bad {
		if _, err := parseAttendPricing(c); err == nil {
			t.Errorf("parseAttendPricing(%v) succeeded; want error", c)
		}
	}
}
