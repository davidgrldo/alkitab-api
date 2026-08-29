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

func TestFromOSIS(t *testing.T) {
	raw := `<?xml version="1.0"?>
<osis xmlns="http://www.bibletechnologies.net/2003/OSIS/namespace">
  <osisText>
    <div type="book" osisID="3John">
      <chapter osisID="3John.1">
        <title>Greeting</title>
        <verse osisID="3John.1.1">The elder unto Gaius</verse>
        <verse osisID="3John.1.2">Beloved I wish</verse>
      </chapter>
    </div>
  </osisText>
</osis>`
	out, err := FromOSIS([]byte(raw), Options{ID: "kjv", Name: "KJV", Lang: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Books[0].ID != "3john" || out.VerseCount != 2 {
		t.Fatalf("%+v", out.Books[0])
	}
	vs := out.Books[0].ChapterData[0].Verses
	if len(vs) != 3 || vs[0].Type != "title" || vs[1].Verse != 1 {
		t.Fatalf("%+v", vs)
	}
}

func TestDetectOSIS(t *testing.T) {
	if !DetectOSIS([]byte("<osis><osisText/>")) {
		t.Fatal("want osis")
	}
}

func TestFromAutoOSIS(t *testing.T) {
	raw := `<osis><osisText><div type="book" osisID="Phlm"><chapter osisID="Phlm.1"><verse osisID="Phlm.1.1">Paul a prisoner</verse></chapter></div></osisText></osis>`
	out, err := From([]byte(raw), "auto", Options{ID: "kjv", Name: "KJV", Lang: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Books[0].ID != "phlm" || out.VerseCount != 1 {
		t.Fatalf("%+v", out)
	}
}

func TestFromUSFMFootnote(t *testing.T) {
	raw := `\id 3JN
\c 1
\v 1 The elder\f + \fr 1:1 \ft A note about Gaius.\f* unto Gaius
`
	out, err := FromUSFM([]byte(raw), Options{ID: "kjv", Name: "KJV", Lang: "en"})
	if err != nil {
		t.Fatal(err)
	}
	vs := out.Books[0].ChapterData[0].Verses
	if len(vs) != 2 || vs[0].Type != "content" || vs[1].Type != "note" {
		t.Fatalf("%+v", vs)
	}
	if !strings.Contains(vs[0].Content, "The elder") || strings.Contains(vs[0].Content, `\f`) {
		t.Errorf("content %q", vs[0].Content)
	}
	if !strings.Contains(vs[1].Content, "A note about Gaius") {
		t.Errorf("note %q", vs[1].Content)
	}
	if out.VerseCount != 1 {
		t.Errorf("verse count %d", out.VerseCount)
	}
}

func TestFromOSISNote(t *testing.T) {
	raw := `<osis><osisText><div type="book" osisID="3John"><chapter osisID="3John.1"><verse osisID="3John.1.1">The elder<note>A note</note> unto Gaius</verse></chapter></div></osisText></osis>`
	out, err := FromOSIS([]byte(raw), Options{ID: "kjv", Name: "KJV", Lang: "en"})
	if err != nil {
		t.Fatal(err)
	}
	vs := out.Books[0].ChapterData[0].Verses
	if len(vs) != 2 || vs[0].Type != "content" || vs[1].Type != "note" {
		t.Fatalf("%+v", vs)
	}
	if strings.Contains(vs[0].Content, "A note") {
		t.Errorf("note leaked into content: %q", vs[0].Content)
	}
	if vs[1].Content != "A note" {
		t.Errorf("note %q", vs[1].Content)
	}
}

func TestToCSVAndFromCSV(t *testing.T) {
	in := `[{"name":"3 John","chapters":[["a","b"]]}]`
	out, err := FromJSON([]byte(in), Options{ID: "kjv", Name: "KJV", Lang: "en"})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := ToCSV(out)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "book,chapter,verse,type,content") || !strings.Contains(s, "3john,1,1,content,a") {
		t.Fatalf("csv: %s", s)
	}
	back, err := FromCSV(raw, Options{ID: "kjv", Name: "KJV", Lang: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if back.VerseCount != 2 || back.Books[0].ID != "3john" {
		t.Fatalf("%+v", back)
	}
}
