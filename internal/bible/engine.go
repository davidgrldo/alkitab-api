package bible

import (
	"fmt"
	"strings"
	"sync"
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
