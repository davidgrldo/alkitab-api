package bible

import (
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"strings"
	"sync"
	"time"
)

// Engine wraps a Source with a chapter cache and corpus-backed operations.
type Engine struct {
	src   Source
	mu    sync.RWMutex
	cache map[string]*Chapter
}

func New(src Source) *Engine {
	return &Engine{src: src, cache: make(map[string]*Chapter)}
}

func chapterKey(version, book string, chapter int) string {
	return fmt.Sprintf("%s:%s:%d", version, book, chapter)
}

func (e *Engine) Chapter(version, book string, chapter int) (*Chapter, error) {
	key := chapterKey(version, book, chapter)
	e.mu.RLock()
	if c, ok := e.cache[key]; ok {
		e.mu.RUnlock()
		return c, nil
	}
	e.mu.RUnlock()

	c, err := e.src.Chapter(version, book, chapter)
	if err != nil {
		return nil, err
	}
	e.mu.Lock()
	e.cache[key] = c
	e.mu.Unlock()
	return c, nil
}

func (e *Engine) corpus() (Corpus, bool) {
	c, ok := e.src.(Corpus)
	return c, ok
}

// Search returns verses whose content contains query (case-insensitive).
func (e *Engine) Search(version, query string) ([]VerseHit, error) {
	c, ok := e.corpus()
	if !ok {
		return nil, ErrUnsupportedFeature
	}
	all, err := c.AllVerses(version)
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(query)
	var hits []VerseHit
	for _, h := range all {
		if strings.Contains(strings.ToLower(h.Verse.Content), q) {
			hits = append(hits, h)
		}
	}
	return hits, nil
}

// DailyVerse returns a deterministic verse for the given date and version:
// seed = fnv(date+version) % len(corpus). Same date+version always agrees.
func (e *Engine) DailyVerse(version string, t time.Time) (*VerseHit, error) {
	c, ok := e.corpus()
	if !ok {
		return nil, ErrUnsupportedFeature
	}
	all, err := c.AllVerses(version)
	if err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return nil, ErrNotFound
	}
	seed := hashSeed(fmt.Sprintf("%04d%02d%02d%s", t.Year(), int(t.Month()), t.Day(), version))
	h := all[seed%uint32(len(all))]
	return &h, nil
}

// RandomVerse returns an unpredictable verse. Auto-seeded via math/rand/v2.
func (e *Engine) RandomVerse(version string) (*VerseHit, error) {
	c, ok := e.corpus()
	if !ok {
		return nil, ErrUnsupportedFeature
	}
	all, err := c.AllVerses(version)
	if err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return nil, ErrNotFound
	}
	h := all[rand.IntN(len(all))]
	return &h, nil
}

func hashSeed(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
