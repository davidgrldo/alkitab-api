// Package scrape is an opt-in bible.Source that proxies alkitab.mobi at
// runtime — nothing is redistributed. It throttles and identifies itself
// upstream, and does not implement bible.Corpus, so search/daily/random
// honestly report unsupported.
package scrape

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/davidgrldo/alkitab-api/bible"
)

// maxBodyBytes caps how much of an upstream response is buffered before
// parsing, so a hostile or broken upstream cannot OOM the process.
const maxBodyBytes = 4 << 20

// staticVersions: the scrape adapter cannot enumerate alkitab.mobi's catalog
// without scraping, so it advertises a curated set verified against the
// upstream version index (names as published there).
var staticVersions = []bible.Translation{
	{ID: "tb", Name: "Terjemahan Baru (1974)", Language: "id"},
	{ID: "tl", Name: "Terjemahan Lama (1954)", Language: "id"},
	{ID: "bis", Name: "Alkitab Kabar Baik — Bahasa Indonesia Sehari-hari (1985)", Language: "id"},
	{ID: "vmd", Name: "Versi Mudah Dibaca (2005)", Language: "id"},
	{ID: "tsi", Name: "Terjemahan Sederhana Indonesia", Language: "id"},
	{ID: "ayt", Name: "Alkitab Yang Terbuka", Language: "id"},
	{ID: "ende", Name: "Alkitab Ende (1970)", Language: "id"},
	{ID: "jawa", Name: "Alkitab Bahasa Jawa", Language: "jv"},
	{ID: "sunda", Name: "Alkitab Bahasa Sunda (1991)", Language: "su"},
}

// userAgent identifies this proxy to the upstream site.
const userAgent = "alkitab-api (+https://github.com/davidgrldo/alkitab-api)"

// minInterval spaces out upstream requests so a burst of API traffic does not
// hammer alkitab.mobi. Cache misses are the only thing that reaches upstream.
const minInterval = 500 * time.Millisecond

// Scrape is a bible.Source that fetches chapters from alkitab.mobi.
// It does NOT implement Corpus (per-query scrape search is infeasible).
type Scrape struct {
	base   string
	client *http.Client

	mu   sync.Mutex
	last time.Time
}

func New(baseURL string) *Scrape {
	if baseURL == "" {
		baseURL = "https://alkitab.mobi"
	}
	return &Scrape{base: strings.TrimRight(baseURL, "/"), client: &http.Client{Timeout: 10 * time.Second}}
}

func (s *Scrape) Translations() []bible.Translation { return staticVersions }

func (s *Scrape) Books(version string) ([]bible.Book, error) {
	if !versionSupported(version) {
		return nil, bible.ErrUnsupportedVersion
	}
	return bible.CanonicalBooks(), nil
}

// throttle enforces minInterval between upstream requests.
// ponytail: menahan lock selama sleep memang menserikan semua permintaan upstream — itu intinya proxy yang sopan.
func (s *Scrape) throttle(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if wait := minInterval - time.Since(s.last); wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	s.last = time.Now()
	return nil
}

func (s *Scrape) Chapter(version, book string, chapter int) (*bible.Chapter, error) {
	return s.ChapterContext(context.Background(), version, book, chapter)
}

func (s *Scrape) ChapterContext(ctx context.Context, version, book string, chapter int) (*bible.Chapter, error) {
	if !versionSupported(version) {
		return nil, bible.ErrUnsupportedVersion
	}
	bookName := bible.IndonesianBookName(book)
	if bookName == "" {
		return nil, bible.ErrNotFound
	}
	url := strings.Join([]string{s.base, version, bookName, strconv.Itoa(chapter)}, "/")
	if err := s.throttle(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: request failed", bible.ErrUpstream)
	}
	req.Header.Set("User-Agent", userAgent)
	res, err := s.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%w: request failed", bible.ErrUpstream)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return nil, bible.ErrNotFound
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: HTTP %d", bible.ErrUpstream, res.StatusCode)
	}
	verses, err := parse(io.LimitReader(res.Body, maxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("%w: parse failed", bible.ErrUpstream)
	}
	if len(verses) == 0 {
		return nil, bible.ErrNotFound
	}
	return &bible.Chapter{Translation: version, Book: book, Number: chapter, Verses: verses}, nil
}

func parse(r io.Reader) ([]bible.Verse, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, err
	}
	var items []bible.Verse
	lastVerse := 0
	doc.Find("p").Each(func(i int, p *goquery.Selection) {
		if _, hidden := p.Attr("hidden"); hidden {
			return
		}
		if p.HasClass("loading") || p.HasClass("error") {
			return
		}
		content := strings.TrimSpace(p.Find("[data-begin]").First().Text())
		title := strings.TrimSpace(p.Find(".paragraphtitle").First().Text())
		verseText := strings.TrimSpace(p.Find(".reftext").Children().First().Text())
		verse := 0
		if verseText != "" {
			verse, _ = strconv.Atoi(verseText)
		}
		if title == "" && content == "" {
			p.Find(".reftext").Remove()
			content = strings.TrimSpace(p.Text())
		}
		var typ string
		switch {
		case title != "":
			typ = "title"
			content = title
			verse = lastVerse + 1
		case content != "":
			typ = "content"
			lastVerse = verse
		}
		if typ != "" {
			items = append(items, bible.Verse{Number: verse, Content: content, Type: typ, Order: i})
		}
	})
	return items, nil
}

func versionSupported(v string) bool {
	for _, t := range staticVersions {
		if t.ID == v {
			return true
		}
	}
	return false
}
