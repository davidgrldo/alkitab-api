package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/davidgrldo/alkitab-api/bible"
)

const maxSearchLimit = 200

type Server struct {
	eng *bible.Engine
}

func New(e *bible.Engine) *Server { return &Server{eng: e} }

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1", s.discovery)
	mux.HandleFunc("GET /v1/translations", s.translations)
	mux.HandleFunc("GET /v1/{version}/books", s.books)
	mux.HandleFunc("GET /v1/{version}/{book}/{chapter}/{verse}", s.chapterVerse)
	mux.HandleFunc("GET /v1/{version}/{book}/{chapter}", s.chapter)
	mux.HandleFunc("GET /v1/passage", s.passage)
	mux.HandleFunc("GET /v1/search", s.search)
	mux.HandleFunc("GET /v1/daily", s.daily)
	mux.HandleFunc("GET /v1/random", s.random)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /readyz", s.readyz)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		switch {
		case r.URL.Path == "/healthz", r.URL.Path == "/readyz", strings.HasPrefix(r.URL.Path, "/v1/random"):
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
	case errors.Is(err, bible.ErrInvalidRef):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, bible.ErrUnsupportedFeature):
		writeError(w, http.StatusNotImplemented, err.Error())
	case errors.Is(err, bible.ErrUpstream):
		writeError(w, http.StatusBadGateway, err.Error())
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		writeError(w, http.StatusGatewayTimeout, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func (s *Server) discovery(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"version": "v1",
		"endpoints": []string{
			"GET /v1/translations",
			"GET /v1/{version}/books",
			"GET /v1/{version}/{book}/{chapter}",
			"GET /v1/{version}/{book}/{chapter}/{verse}",
			"GET /v1/passage",
			"GET /v1/search",
			"GET /v1/daily",
			"GET /v1/random",
		},
	})
}

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	if len(s.eng.Source().Translations()) == 0 {
		writeError(w, http.StatusServiceUnavailable, "no translations loaded")
		return
	}
	w.Write([]byte("ok"))
}

func (s *Server) translations(w http.ResponseWriter, r *http.Request) {
	ts := s.eng.Catalog()
	sort.Slice(ts, func(i, j int) bool { return ts[i].ID < ts[j].ID })
	writeJSON(w, map[string]any{"translations": ts})
}

func (s *Server) books(w http.ResponseWriter, r *http.Request) {
	version := r.PathValue("version")
	b, err := s.eng.Source().Books(version)
	if err != nil {
		s.mapErr(w, err)
		return
	}
	b = bible.LocalizeBooks(b, r.URL.Query().Get("locale"))
	writeJSON(w, map[string]any{"books": b})
}

func (s *Server) chapter(w http.ResponseWriter, r *http.Request) {
	c, err := s.resolveChapter(r)
	if err != nil {
		s.mapErr(w, err)
		return
	}
	s.writeChapter(w, r, c, nil)
}

func (s *Server) chapterVerse(w http.ResponseWriter, r *http.Request) {
	c, err := s.resolveChapter(r)
	if err != nil {
		s.mapErr(w, err)
		return
	}
	specs, err := bible.ParseVerseList(r.PathValue("verse"))
	if err != nil {
		s.mapErr(w, err)
		return
	}
	s.writeChapter(w, r, c, specs)
}

func (s *Server) passage(w http.ResponseWriter, r *http.Request) {
	version := r.URL.Query().Get("version")
	q := r.URL.Query().Get("q")
	if version == "" || q == "" {
		s.mapErr(w, badRequest("missing query parameter 'version' or 'q'"))
		return
	}
	ref, err := bible.ParsePassage(q)
	if err != nil {
		s.mapErr(w, err)
		return
	}
	if len(ref.Spans) <= 1 {
		c, err := s.eng.ChapterContext(r.Context(), version, ref.BookID, ref.Chapter)
		if err != nil {
			s.mapErr(w, err)
			return
		}
		s.writeChapter(w, r, c, ref.Spans[0].Verses)
		return
	}
	chs, err := s.eng.LoadPassage(r.Context(), version, ref)
	if err != nil {
		s.mapErr(w, err)
		return
	}
	first, err := s.eng.ChapterContext(r.Context(), version, ref.BookID, ref.Spans[0].Chapter)
	if err != nil {
		s.mapErr(w, err)
		return
	}
	last, err := s.eng.ChapterContext(r.Context(), version, ref.BookID, ref.Spans[len(ref.Spans)-1].Chapter)
	if err != nil {
		s.mapErr(w, err)
		return
	}
	navFirst := bible.Chapter{Translation: first.Translation, Book: first.Book, Number: first.Number, Verses: first.Verses}
	navLast := bible.Chapter{Translation: last.Translation, Book: last.Book, Number: last.Number, Verses: last.Verses}
	from, _ := specBounds(ref.Spans[0].Verses)
	_, to := specBounds(ref.Spans[len(ref.Spans)-1].Verses)
	s.eng.AttachNeighbors(r.Context(), navFirst.Translation, &navFirst, verseSpecPtr(from, from))
	s.eng.AttachNeighbors(r.Context(), navLast.Translation, &navLast, verseSpecPtr(to, to))
	slim := make([]map[string]any, 0, len(chs))
	for _, ch := range chs {
		slim = append(slim, map[string]any{"chapter": ch.Number, "verses": ch.Verses})
	}
	s.writeCachedJSON(w, r, map[string]any{
		"version":  version,
		"book":     ref.BookID,
		"chapters": slim,
		"prev":     navFirst.Prev,
		"next":     navLast.Next,
	})
}

func specBounds(specs []bible.VerseSpec) (from, to int) {
	if len(specs) == 0 {
		return 0, 0
	}
	from, to = specs[0].From, specs[0].To
	for _, s := range specs[1:] {
		if s.From < from {
			from = s.From
		}
		if s.To > to {
			to = s.To
		}
	}
	return from, to
}

func verseSpecPtr(from, to int) *bible.VerseSpec {
	if from < 1 {
		return nil
	}
	return &bible.VerseSpec{From: from, To: to}
}

func (s *Server) writeChapter(w http.ResponseWriter, r *http.Request, c *bible.Chapter, specs []bible.VerseSpec) {
	out := bible.Chapter{Translation: c.Translation, Book: c.Book, Number: c.Number, Verses: c.Verses}
	if len(specs) > 0 {
		out.Verses = bible.FilterVerses(c.Verses, specs...)
		if len(out.Verses) == 0 {
			s.mapErr(w, bible.ErrNotFound)
			return
		}
	}
	also := s.loadAlso(r.Context(), r.URL.Query().Get("also"), out.Book, out.Number, specs)
	nav := bible.Chapter{Translation: c.Translation, Book: c.Book, Number: c.Number, Verses: c.Verses}
	var spec *bible.VerseSpec
	if len(specs) > 0 {
		from, to := specBounds(specs)
		spec = &bible.VerseSpec{From: from, To: to}
	}
	s.eng.AttachNeighbors(r.Context(), nav.Translation, &nav, spec)
	out.Prev, out.Next = nav.Prev, nav.Next
	var payload any = out
	if len(also) > 0 {
		payload = struct {
			bible.Chapter
			Also map[string]bible.Chapter `json:"also"`
		}{Chapter: out, Also: also}
	}
	s.writeCachedJSON(w, r, payload)
}

func (s *Server) writeCachedJSON(w http.ResponseWriter, r *http.Request, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		s.mapErr(w, err)
		return
	}
	sum := sha256.Sum256(body)
	etag := `"` + hex.EncodeToString(sum[:8]) + `"`
	w.Header().Set("ETag", etag)
	if match := r.Header.Get("If-None-Match"); match != "" && etagMatch(match, etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(append(body, '\n'))
}

func etagMatch(header, etag string) bool {
	for _, part := range strings.Split(header, ",") {
		p := strings.TrimSpace(part)
		if strings.HasPrefix(p, "W/") {
			p = strings.TrimSpace(p[2:])
		}
		if p == etag || p == "*" {
			return true
		}
	}
	return false
}

func (s *Server) loadAlso(ctx context.Context, raw, book string, chapter int, specs []bible.VerseSpec) map[string]bible.Chapter {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	out := map[string]bible.Chapter{}
	for _, id := range strings.Split(raw, ",") {
		id = strings.TrimSpace(strings.ToLower(id))
		if id == "" {
			continue
		}
		c, err := s.eng.ChapterContext(ctx, id, book, chapter)
		if err != nil {
			continue
		}
		ch := bible.Chapter{Translation: c.Translation, Book: c.Book, Number: c.Number, Verses: c.Verses}
		if len(specs) > 0 {
			ch.Verses = bible.FilterVerses(c.Verses, specs...)
			if len(ch.Verses) == 0 {
				continue
			}
		}
		out[id] = ch
	}
	return out
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
	return s.eng.ChapterContext(r.Context(), version, bookID, chap)
}

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, http.StatusBadRequest, "missing query parameter 'q'")
		return
	}
	limit := 50
	if ls := r.URL.Query().Get("limit"); ls != "" {
		n, err := strconv.Atoi(ls)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		limit = n
	}
	if limit > maxSearchLimit {
		limit = maxSearchLimit
	}
	offset := 0
	if os := r.URL.Query().Get("offset"); os != "" {
		n, err := strconv.Atoi(os)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "invalid offset")
			return
		}
		offset = n
	}
	version := r.URL.Query().Get("version")
	if version == "" {
		v, err := s.eng.DefaultCorpusVersion()
		if err != nil {
			s.mapErr(w, badRequest("missing query parameter 'version'"))
			return
		}
		version = v
	}
	var f bible.SearchFilter
	if bs := r.URL.Query().Get("book"); bs != "" {
		id, err := bible.ResolveBookID(bs)
		if err != nil {
			s.mapErr(w, err)
			return
		}
		f.Book = id
	}
	if ts := r.URL.Query().Get("testament"); ts != "" {
		u := strings.ToUpper(ts)
		if u != "OT" && u != "NT" {
			s.mapErr(w, badRequest("invalid testament"))
			return
		}
		f.Testament = u
	}
	wh := strings.ToLower(r.URL.Query().Get("whole"))
	f.WholeWord = wh == "1" || wh == "true"
	hits, err := s.eng.SearchFiltered(version, q, f)
	if err != nil {
		s.mapErr(w, err)
		return
	}
	total := len(hits)
	if offset > total {
		offset = total
	}
	hits = hits[offset:]
	if len(hits) > limit {
		hits = hits[:limit]
	}
	writeJSON(w, map[string]any{"hits": hits, "total": total, "offset": offset})
}

func (s *Server) daily(w http.ResponseWriter, r *http.Request) {
	version, f, err := s.sampleParams(r)
	if err != nil {
		s.mapErr(w, err)
		return
	}
	t, err := parseDailyTime(r)
	if err != nil {
		s.mapErr(w, err)
		return
	}
	h, err := s.eng.DailyVerseFiltered(version, t, f)
	if err != nil {
		s.mapErr(w, err)
		return
	}
	writeJSON(w, h)
}

func (s *Server) random(w http.ResponseWriter, r *http.Request) {
	version, f, err := s.sampleParams(r)
	if err != nil {
		s.mapErr(w, err)
		return
	}
	h, err := s.eng.RandomVerseFiltered(version, f)
	if err != nil {
		s.mapErr(w, err)
		return
	}
	writeJSON(w, h)
}

func (s *Server) sampleParams(r *http.Request) (string, bible.SampleFilter, error) {
	version := r.URL.Query().Get("version")
	if version == "" {
		v, err := s.eng.DefaultCorpusVersion()
		if err != nil {
			return "", bible.SampleFilter{}, badRequest("missing query parameter 'version'")
		}
		version = v
	}
	var f bible.SampleFilter
	if bs := r.URL.Query().Get("book"); bs != "" {
		id, err := bible.ResolveBookID(bs)
		if err != nil {
			return "", f, err
		}
		f.Book = id
	}
	if ts := r.URL.Query().Get("testament"); ts != "" {
		u := strings.ToUpper(ts)
		if u != "OT" && u != "NT" {
			return "", f, badRequest("invalid testament")
		}
		f.Testament = u
	}
	if ss := r.URL.Query().Get("seed"); ss != "" {
		n, err := strconv.ParseInt(ss, 10, 64)
		if err != nil {
			return "", f, badRequest("invalid seed")
		}
		f.Seed = &n
	}
	return version, f, nil
}

func parseDailyTime(r *http.Request) (time.Time, error) {
	loc := time.UTC
	if tz := r.URL.Query().Get("tz"); tz != "" {
		l, err := time.LoadLocation(tz)
		if err != nil {
			return time.Time{}, badRequest("invalid tz")
		}
		loc = l
	}
	if ds := r.URL.Query().Get("date"); ds != "" {
		t, err := time.ParseInLocation("2006-01-02", ds, loc)
		if err != nil {
			return time.Time{}, badRequest("invalid date")
		}
		return t, nil
	}
	return time.Now().In(loc), nil
}
