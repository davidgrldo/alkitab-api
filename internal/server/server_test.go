package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/davidgrldo/alkitab-api/bible"
	"github.com/davidgrldo/alkitab-api/local"
)

func newServer(t *testing.T) *Server {
	t.Helper()
	l, err := local.New("")
	if err != nil {
		t.Fatal(err)
	}
	return New(bible.New(l))
}

func getJSON(t *testing.T, h http.Handler, path string, code int) map[string]any {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	h.ServeHTTP(rr, req)
	if rr.Code != code {
		t.Fatalf("GET %s: status %d, want %d; body=%s", path, rr.Code, code, rr.Body.String())
	}
	var m map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &m)
	return m
}

func TestTranslations(t *testing.T) {
	h := newServer(t).Handler()
	m := getJSON(t, h, "/v1/translations", 200)
	if _, ok := m["translations"]; !ok {
		t.Errorf("missing translations key: %v", m)
	}
}

func TestBooks(t *testing.T) {
	h := newServer(t).Handler()
	m := getJSON(t, h, "/v1/kjv/books", 200)
	books, _ := m["books"].([]any)
	if len(books) != 2 {
		t.Errorf("want 2 sample books, got %v", books)
	}
}

func TestChapterByName(t *testing.T) {
	h := newServer(t).Handler()
	m := getJSON(t, h, "/v1/kjv/3John/1", 200)
	verses, _ := m["verses"].([]any)
	if len(verses) != 14 {
		t.Errorf("want 14 verses, got %d", len(verses))
	}
}

func TestChapterNotFound(t *testing.T) {
	h := newServer(t).Handler()
	getJSON(t, h, "/v1/kjv/3john/9", 404)
}

func TestBadChapter(t *testing.T) {
	h := newServer(t).Handler()
	getJSON(t, h, "/v1/kjv/3john/abc", 400)
}

func TestBadVerse(t *testing.T) {
	h := newServer(t).Handler()
	getJSON(t, h, "/v1/kjv/3john/1/abc", 400)
}

func TestSingleVerse(t *testing.T) {
	h := newServer(t).Handler()
	m := getJSON(t, h, "/v1/kjv/3john/1/4", 200)
	verses, _ := m["verses"].([]any)
	if len(verses) != 1 {
		t.Errorf("want 1 verse filtered, got %d", len(verses))
	}
}

func TestVerseRange(t *testing.T) {
	h := newServer(t).Handler()
	m := getJSON(t, h, "/v1/kjv/3john/1/4-6", 200)
	verses, _ := m["verses"].([]any)
	if len(verses) != 3 {
		t.Errorf("want 3 verses, got %d", len(verses))
	}
}

func TestVerseRangeReversed(t *testing.T) {
	h := newServer(t).Handler()
	getJSON(t, h, "/v1/kjv/3john/1/6-4", 400)
}

func TestVerseRangeNotFound(t *testing.T) {
	h := newServer(t).Handler()
	getJSON(t, h, "/v1/kjv/3john/1/20-22", 404)
}

func TestPassageQuery(t *testing.T) {
	h := newServer(t).Handler()
	m := getJSON(t, h, "/v1/passage?version=kjv&q=3john+1:4-6", 200)
	verses, _ := m["verses"].([]any)
	if len(verses) != 3 {
		t.Errorf("want 3 verses, got %d; body=%v", len(verses), m)
	}
	if m["book"] != "3john" || m["chapter"] != float64(1) {
		t.Errorf("book/chapter: %v", m)
	}
}

func TestPassageQueryMissing(t *testing.T) {
	h := newServer(t).Handler()
	getJSON(t, h, "/v1/passage?version=kjv", 400)
	getJSON(t, h, "/v1/passage?q=3john+1:4", 400)
}

func TestSingleVerseNotFound(t *testing.T) {
	h := newServer(t).Handler()
	// 3 John 1 has 14 verses; 999 does not exist -> 404 per spec §7.
	getJSON(t, h, "/v1/kjv/3john/1/999", 404)
}

func TestSearch(t *testing.T) {
	h := newServer(t).Handler()
	m := getJSON(t, h, "/v1/search?q=truth&version=kjv", 200)
	hits, _ := m["hits"].([]any)
	if len(hits) == 0 {
		t.Error("want at least one hit for 'truth'")
	}
}

func TestSearchDefaultVersionAndBookFilter(t *testing.T) {
	h := newServer(t).Handler()
	m := getJSON(t, h, "/v1/search?q=truth&book=3john", 200)
	if m["total"] == nil {
		t.Fatalf("%v", m)
	}
	m = getJSON(t, h, "/v1/search?q=greater+joy&book=philemon", 200)
	if m["total"].(float64) != 0 {
		t.Errorf("philemon should not match 3john truth hits: %v", m)
	}
}

func TestSearchLimitCap(t *testing.T) {
	h := newServer(t).Handler()
	m := getJSON(t, h, "/v1/search?q=the&version=kjv&limit=9999", 200)
	hits, _ := m["hits"].([]any)
	if len(hits) > 200 {
		t.Errorf("limit cap: got %d", len(hits))
	}
}

func TestTranslationsCapabilities(t *testing.T) {
	h := newServer(t).Handler()
	m := getJSON(t, h, "/v1/translations", 200)
	ts, _ := m["translations"].([]any)
	if len(ts) == 0 {
		t.Fatal(m)
	}
	row, _ := ts[0].(map[string]any)
	if row["origin"] != "local" {
		t.Errorf("origin: %v", row)
	}
	caps, _ := row["capabilities"].([]any)
	joined := fmt.Sprint(caps)
	if !strings.Contains(joined, "corpus") {
		t.Errorf("caps: %v", caps)
	}
}

func TestBooksLocale(t *testing.T) {
	h := newServer(t).Handler()
	m := getJSON(t, h, "/v1/kjv/books?locale=id", 200)
	books, _ := m["books"].([]any)
	row, _ := books[0].(map[string]any)
	if row["name"] != "3 Yohanes" && row["id"] == "3john" {
		// sample book order: 3john then phlm
	}
	found := false
	for _, b := range books {
		row, _ := b.(map[string]any)
		if row["id"] == "3john" && row["name"] == "3 Yohanes" {
			found = true
		}
	}
	if !found {
		t.Errorf("want 3 Yohanes, got %v", books)
	}
}

func TestDailyDate(t *testing.T) {
	h := newServer(t).Handler()
	a := getJSON(t, h, "/v1/daily?version=kjv&date=2026-08-29", 200)
	b := getJSON(t, h, "/v1/daily?version=kjv&date=2026-08-29", 200)
	if a["verse"].(map[string]any)["content"] != b["verse"].(map[string]any)["content"] {
		t.Errorf("same date must agree")
	}
}

func TestAlsoSelf(t *testing.T) {
	h := newServer(t).Handler()
	m := getJSON(t, h, "/v1/kjv/3john/1/4?also=kjv", 200)
	also, _ := m["also"].(map[string]any)
	if also == nil {
		t.Fatalf("missing also: %v", m)
	}
}

func TestDiscoveryReadyzCORS(t *testing.T) {
	h := newServer(t).Handler()
	getJSON(t, h, "/v1", 200)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != 200 || rr.Body.String() != "ok" {
		t.Fatalf("readyz %d %s", rr.Code, rr.Body.String())
	}
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodOptions, "/v1/translations", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != 204 {
		t.Fatalf("OPTIONS %d", rr.Code)
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	h := newServer(t).Handler()
	getJSON(t, h, "/v1/search?q=&version=kjv", 400)
}

func TestDailyAndRandom(t *testing.T) {
	h := newServer(t).Handler()
	getJSON(t, h, "/v1/daily?version=kjv", 200)
	getJSON(t, h, "/v1/random?version=kjv", 200)
}
