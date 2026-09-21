package api

import "testing"

func TestParseAttendPricing(t *testing.T) {
	prices, mult, err := parseAttendPricing(map[string]string{
		"Easy": "5k", "Medium": "", "Hard": "30,000", "Challenging": "0.1kk",
	}, " x1.5 ")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int64{"Easy": 5000, "Hard": 30000, "Challenging": 100000}
	if len(prices) != len(want) {
		t.Fatalf("prices = %v; want %v", prices, want)
	}
	for k, v := range want {
		if prices[k] != v {
			t.Errorf("prices[%s] = %d; want %d", k, prices[k], v)
		}
	}
	if mult != 1.5 {
		t.Errorf("mult = %v; want 1.5", mult)
	}

	if _, mult, err := parseAttendPricing(map[string]string{"Hard": "30k"}, ""); err != nil || mult != 1 {
		t.Errorf("blank multiplier = %v, %v; want 1, nil", mult, err)
	}

	bad := []struct {
		prices map[string]string
		mult   string
	}{
		{map[string]string{}, "1"},                        // no prices
		{map[string]string{"Hard": "", "Easy": "0"}, "1"}, // all free
		{map[string]string{"Hard": "abc"}, "1"},           // bad price
		{map[string]string{"Epic": "5k"}, "1"},            // unknown difficulty
		{map[string]string{"Hard": "5k"}, "0"},            // zero multiplier
		{map[string]string{"Hard": "5k"}, "-2"},           // negative
		{map[string]string{"Hard": "5k"}, "101"},          // too big
		{map[string]string{"Hard": "5k"}, "0.001"},        // rounds to 0
		{map[string]string{"Hard": "5k"}, "two"},          // junk
	}
	for _, c := range bad {
		if _, _, err := parseAttendPricing(c.prices, c.mult); err == nil {
			t.Errorf("parseAttendPricing(%v, %q) succeeded; want error", c.prices, c.mult)
		}
	}
}
