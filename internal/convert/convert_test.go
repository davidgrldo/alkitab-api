package convert

import (
	"strings"
	"testing"
)

func TestFromJSON(t *testing.T) {
	in := `[{"name":"3 John","chapters":[["a","b"]]}]`
	out, err := FromJSON([]byte(in), Options{ID: "kjv", Name: "KJV", Lang: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Books) != 1 || out.Books[0].ID != "3john" || out.VerseCount != 2 {
		t.Fatalf("%+v", out)
	}
	if out.Books[0].Name != "3 John" {
		t.Errorf("english name: %s", out.Books[0].Name)
	}
}

func TestFromJSONLocaleID(t *testing.T) {
	in := `[{"name":"3 John","chapters":[["a"]]}]`
	out, err := FromJSON([]byte(in), Options{ID: "kjv", Name: "KJV", Lang: "id", Locale: "id"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Books[0].Name != "3 Yohanes" {
		t.Errorf("got %q", out.Books[0].Name)
	}
}

func TestValidateRejectsShortBook(t *testing.T) {
	in := `[{"name":"Genesis","chapters":[["a"]]}]`
	_, err := FromJSON([]byte(in), Options{ID: "kjv", Name: "KJV", Lang: "en", Validate: true})
	if err == nil || !strings.Contains(err.Error(), "validate") {
		t.Fatalf("want validate error, got %v", err)
	}
}

func TestFromUSFM(t *testing.T) {
	raw := `\id 3JN
\c 1
\s Greeting
\v 1 The elder unto Gaius
\v 2 Beloved I wish
`
	out, err := FromUSFM([]byte(raw), Options{ID: "kjv", Name: "KJV", Lang: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Books[0].ID != "3john" || out.Books[0].Chapters != 1 {
		t.Fatalf("%+v", out.Books[0])
	}
	vs := out.Books[0].ChapterData[0].Verses
	if len(vs) != 3 || vs[0].Type != "title" || vs[1].Type != "content" || vs[1].Verse != 1 {
		t.Fatalf("%+v", vs)
	}
}

func TestDetectUSFM(t *testing.T) {
	if !DetectUSFM([]byte("\\id GEN\n")) {
		t.Fatal("want usfm")
	}
	if DetectUSFM([]byte(`[{"name":"x"}]`)) {
		t.Fatal("want json")
	}
}
