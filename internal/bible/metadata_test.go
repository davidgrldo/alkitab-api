package bible

import "testing"

func TestResolveBookID(t *testing.T) {
	cases := []struct{ in, want string }{
		{"gen", "gen"},
		{"Genesis", "gen"},
		{"GENESIS", "gen"},
		{"Kejadian", "gen"},
		{"kej", "gen"},
		{"Mazmur", "ps"},
		{"Psalms", "ps"},
		{"3 john", "3john"},
		{"3Yoh", "3john"},
		{"phlm", "phlm"},
		{"Filemon", "phlm"},
	}
	for _, c := range cases {
		got, err := ResolveBookID(c.in)
		if err != nil || got != c.want {
			t.Errorf("ResolveBookID(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
	if _, err := ResolveBookID("nope"); !errorIs(err, ErrNotFound) {
		t.Errorf("unknown book want ErrNotFound, got %v", err)
	}
}

func TestCanonCounts(t *testing.T) {
	books := CanonicalBooks()
	if len(books) != 66 {
		t.Fatalf("canon has %d books, want 66", len(books))
	}
	ot, nt := 0, 0
	for _, b := range books {
		switch b.Testament {
		case "OT":
			ot++
		case "NT":
			nt++
		}
	}
	if ot != 39 || nt != 27 {
		t.Errorf("testaments OT=%d NT=%d; want 39/27", ot, nt)
	}
}

func TestIndonesianBookName(t *testing.T) {
	if got := IndonesianBookName("ps"); got != "Mazmur" {
		t.Errorf("IndonesianBookName(ps)=%q want Mazmur", got)
	}
	if got := IndonesianBookName("gen"); got != "Kejadian" {
		t.Errorf("IndonesianBookName(gen)=%q want Kejadian", got)
	}
}

func errorIs(err, target error) bool {
	return err == target
}
