package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/davidgrldo/alkitab-api/internal/bible"
)

type Server struct {
	eng *bible.Engine
}

func New(e *bible.Engine) *Server { return &Server{eng: e} }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/translations", s.translations)
	mux.HandleFunc("GET /v1/{version}/books", s.books)
	mux.HandleFunc("GET /v1/{version}/{book}/{chapter}/{verse}", s.chapterVerse)
	mux.HandleFunc("GET /v1/{version}/{book}/{chapter}", s.chapter)
	mux.HandleFunc("GET /v1/search", s.search)
	mux.HandleFunc("GET /v1/daily", s.daily)
	mux.HandleFunc("GET /v1/random", s.random)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	// ponytail: public read-only API — blanket ACAO:* so browser pages (e.g. the microsite demo) can fetch it.
	// Verses are immutable, so most responses are safely cacheable; random must never be, daily only briefly.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		switch {
		case r.URL.Path == "/healthz", strings.HasPrefix(r.URL.Path, "/v1/random"):
			w.Header().Set("Cache-Control", "no-store")
		case strings.HasPrefix(r.URL.Path, "/v1/daily"):
			w.Header().Set("Cache-Control", "public, max-age=300")
		default:
			w.Header().Set("Cache-Control", "public, max-age=86400")
		}
		mux.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// httpError is a sentinel for HTTP-level errors (e.g. 400) that mapErr translates.
type httpError struct {
	status int
	msg    string
}

func (e *httpError) Error() string { return e.msg }

func badRequest(msg string) error { return &httpError{status: http.StatusBadRequest, msg: msg} }

func (s *Server) mapErr(w http.ResponseWriter, err error) {
	var he *httpError
	if errors.As(err, &he) {
		writeError(w, he.status, he.msg)
		return
	}
	switch {
	case errors.Is(err, bible.ErrNotFound), errors.Is(err, bible.ErrUnsupportedVersion):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, bible.ErrUnsupportedFeature):
		writeError(w, http.StatusNotImplemented, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func (s *Server) translations(w http.ResponseWriter, r *http.Request) {
	// ponytail: deterministic order — sort.Slice by ID; raw map iteration in Source().Translations() is non-deterministic.
	ts := s.eng.Source().Translations()
	out := make([]bible.Translation, len(ts))
	copy(out, ts)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	writeJSON(w, map[string]any{"translations": out})
}

func (s *Server) books(w http.ResponseWriter, r *http.Request) {
	version := r.PathValue("version")
	b, err := s.eng.Source().Books(version)
	if err != nil {
		s.mapErr(w, err)
		return
	}
	writeJSON(w, map[string]any{"books": b})
}

func (s *Server) chapter(w http.ResponseWriter, r *http.Request) {
	c, err := s.resolveChapter(r)
	if err != nil {
		s.mapErr(w, err)
		return
	}
	writeJSON(w, c)
}

func (s *Server) chapterVerse(w http.ResponseWriter, r *http.Request) {
	c, err := s.resolveChapter(r)
	if err != nil {
		s.mapErr(w, err)
		return
	}
	vn, err := strconv.Atoi(r.PathValue("verse"))
	if err != nil {
		s.mapErr(w, badRequest("invalid verse"))
		return
	}
	filtered := make([]bible.Verse, 0)
	for _, v := range c.Verses {
		if v.Number == vn {
			filtered = append(filtered, v)
		}
	}
	if len(filtered) == 0 {
		s.mapErr(w, bible.ErrNotFound)
		return
	}
	// ponytail: pakai struct Chapter agar urutan field konsisten dengan endpoint pasal (map = key alfabetis).
	writeJSON(w, bible.Chapter{Translation: c.Translation, Book: c.Book, Number: c.Number, Verses: filtered})
}

func (s *Server) resolveChapter(r *http.Request) (*bible.Chapter, error) {
	version := r.PathValue("version")
	chap, err := strconv.Atoi(r.PathValue("chapter"))
	if err != nil {
		return nil, badRequest("invalid chapter")
	}
	bookID, err := bible.ResolveBookID(r.PathValue("book"))
	if err != nil {
		return nil, err
	}
	return s.eng.Chapter(version, bookID, chap)
}

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, http.StatusBadRequest, "missing query parameter 'q'")
		return
	}
	// ponytail: limit di handler, bukan engine — engine tetap mengembalikan semua; total memberi tahu klien ada truncation.
	limit := 50
	if ls := r.URL.Query().Get("limit"); ls != "" {
		n, err := strconv.Atoi(ls)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		limit = n
	}
	version := r.URL.Query().Get("version")
	hits, err := s.eng.Search(version, q)
	if err != nil {
		s.mapErr(w, err)
		return
	}
	total := len(hits)
	if total > limit {
		hits = hits[:limit]
	}
	writeJSON(w, map[string]any{"hits": hits, "total": total})
}

func (s *Server) daily(w http.ResponseWriter, r *http.Request) {
	version := r.URL.Query().Get("version")
	h, err := s.eng.DailyVerse(version, time.Now().UTC())
	if err != nil {
		s.mapErr(w, err)
		return
	}
	writeJSON(w, h)
}

func (s *Server) random(w http.ResponseWriter, r *http.Request) {
	version := r.URL.Query().Get("version")
	h, err := s.eng.RandomVerse(version)
	if err != nil {
		s.mapErr(w, err)
		return
	}
	writeJSON(w, h)
}
