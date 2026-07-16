package bible

import (
	"fmt"
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
