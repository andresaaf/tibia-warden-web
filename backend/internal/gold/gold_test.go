package gold

import "testing"

func TestParse(t *testing.T) {
	valid := map[string]int64{
		"30k":     30_000,
		"30K":     30_000,
		" 30 k ":  30_000,
		"1.5kk":   1_500_000,
		"1.5m":    1_500_000,
		"30000":   30_000,
		"30,000":  30_000,
		"2.5k":    2_500,
		"0":       0,
		"500gp":   500,
		"100kk":   100_000_000,
		"0.0015k": 2,
	}
	for in, want := range valid {
		got, err := Parse(in)
		if err != nil || got != want {
			t.Errorf("Parse(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", "abc", "k", "-5k", "+5k", "1e5", "101kk", "30kkk", "1.2.3k", "NaN", "Inf"} {
		if got, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) = %d; want error", in, got)
		}
	}
}

func TestFormat(t *testing.T) {
	cases := map[int64]string{
		0:         "0",
		999:       "999",
		1000:      "1k",
		1560:      "1.6k",
		12345:     "12k",
		30000:     "30k",
		1_000_000: "1kk",
		2_500_000: "2.5kk",
	}
	for in, want := range cases {
		if got := Format(in); got != want {
			t.Errorf("Format(%d) = %q; want %q", in, got, want)
		}
	}
}
