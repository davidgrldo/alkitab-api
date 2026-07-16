package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/davidgrldo/alkitab-api/internal/bible"
	"github.com/davidgrldo/alkitab-api/internal/local"
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

func TestSingleVerse(t *testing.T) {
	h := newServer(t).Handler()
	m := getJSON(t, h, "/v1/kjv/3john/1/4", 200)
	verses, _ := m["verses"].([]any)
	if len(verses) != 1 {
		t.Errorf("want 1 verse filtered, got %d", len(verses))
	}
}
