package bible

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"sync"
	"time"
)

// Engine wraps a Source with a chapter cache and corpus-backed operations.
type Engine struct {
	src Source
	mu  sync.RWMutex
	// cache holds *Chapter values that are treated as immutable after store;
	// callers must not mutate them.
	cache   *chapterLRU
	indexes map[string]*searchIndex
}

func New(src Source) *Engine {
	return newEngine(src, maxCacheEntries)
}

func newEngine(src Source, cacheCap int) *Engine {
	return &Engine{src: src, cache: newChapterLRU(cacheCap), indexes: make(map[string]*searchIndex)}
}

// Source returns the underlying source (used by the server for listings).
func (e *Engine) Source() Source { return e.src }

func chapterKey(version, book string, chapter int) string {
	return fmt.Sprintf("%s:%s:%d", version, book, chapter)
}

func (e *Engine) Chapter(version, book string, chapter int) (*Chapter, error) {
	return e.ChapterContext(context.Background(), version, book, chapter)
}

func (e *Engine) ChapterContext(ctx context.Context, version, book string, chapter int) (*Chapter, error) {
	key := chapterKey(version, book, chapter)
	e.mu.Lock()
	if c, ok := e.cache.get(key); ok {
		e.mu.Unlock()
		return c, nil
	}
	e.mu.Unlock()

	c, err := fetchChapter(ctx, e.src, version, book, chapter)
	if err != nil {
		return nil, err
	}
	e.mu.Lock()
	if existing, ok := e.cache.get(key); ok {
		e.mu.Unlock()
		return existing, nil
	}
	e.cache.put(key, c)
	e.mu.Unlock()
	return c, nil
}

// LoadPassage returns filtered chapters for a parsed reference, without
// mutating cached Chapter values.
func (e *Engine) LoadPassage(ctx context.Context, version string, ref PassageRef) ([]Chapter, error) {
	spans := ref.Spans
	if len(spans) == 0 {
		spans = []ChapterSpan{{Chapter: ref.Chapter, Verses: []VerseSpec{ref.Verses}}}
	}
	out := make([]Chapter, 0, len(spans))
	for _, span := range spans {
		c, err := e.ChapterContext(ctx, version, ref.BookID, span.Chapter)
		if err != nil {
			return nil, err
		}
		ch := Chapter{Translation: c.Translation, Book: c.Book, Number: c.Number, Verses: c.Verses}
		if len(span.Verses) > 0 {
			ch.Verses = FilterVerses(c.Verses, span.Verses...)
		}
		if len(ch.Verses) == 0 {
			return nil, ErrNotFound
		}
		out = append(out, ch)
	}
	return out, nil
}

// ContextSource is implemented by adapters that honor cancellation (scrape).
type ContextSource interface {
	ChapterContext(ctx context.Context, version, book string, chapter int) (*Chapter, error)
}

func fetchChapter(ctx context.Context, src Source, version, book string, chapter int) (*Chapter, error) {
	if cs, ok := src.(ContextSource); ok {
		return cs.ChapterContext(ctx, version, book, chapter)
	}
	return src.Chapter(version, book, chapter)
}

// maxCacheEntries ≈ jumlah pasal satu Alkitab penuh (1.189) dengan ruang lega.
const maxCacheEntries = 2048

type lruNode struct {
	key        string
	val        *Chapter
	prev, next *lruNode
}

type chapterLRU struct {
	cap        int
	items      map[string]*lruNode
	head, tail *lruNode
}

func newChapterLRU(cap int) *chapterLRU {
	if cap < 1 {
		cap = maxCacheEntries
	}
	head, tail := &lruNode{}, &lruNode{}
	head.next, tail.prev = tail, head
	return &chapterLRU{cap: cap, items: make(map[string]*lruNode), head: head, tail: tail}
}

func (c *chapterLRU) get(key string) (*Chapter, bool) {
	n, ok := c.items[key]
	if !ok {
		return nil, false
	}
	c.moveFront(n)
	return n.val, true
}

func (c *chapterLRU) put(key string, val *Chapter) {
	if n, ok := c.items[key]; ok {
		n.val = val
		c.moveFront(n)
		return
	}
	n := &lruNode{key: key, val: val}
	c.items[key] = n
	c.insertFront(n)
	if len(c.items) > c.cap {
		victim := c.tail.prev
		c.remove(victim)
		delete(c.items, victim.key)
	}
}

func (c *chapterLRU) insertFront(n *lruNode) {
	n.next = c.head.next
	n.prev = c.head
	c.head.next.prev = n
	c.head.next = n
}

func (c *chapterLRU) remove(n *lruNode) {
	n.prev.next = n.next
	n.next.prev = n.prev
	n.prev, n.next = nil, nil
}

func (c *chapterLRU) moveFront(n *lruNode) {
	c.remove(n)
	c.insertFront(n)
}

func (e *Engine) corpus() (Corpus, bool) {
	c, ok := e.src.(Corpus)
	return c, ok
}

// Search returns verses whose content contains query (case-insensitive).
// Titles are skipped. Pass a zero SearchFilter for unscoped substring match.
func (e *Engine) Search(version, query string) ([]VerseHit, error) {
	return e.SearchFiltered(version, query, SearchFilter{})
}

func (e *Engine) SearchFiltered(version, query string, f SearchFilter) ([]VerseHit, error) {
	ix, err := e.ensureIndex(version)
	if err != nil {
		return nil, err
	}
	return ix.lookup(query, f), nil
}

func (e *Engine) DailyVerse(version string, t time.Time) (*VerseHit, error) {
	return e.DailyVerseFiltered(version, t, SampleFilter{})
}

func (e *Engine) DailyVerseFiltered(version string, t time.Time, f SampleFilter) (*VerseHit, error) {
	all, err := e.samplePool(version, f)
	if err != nil {
		return nil, err
	}
	seedKey := fmt.Sprintf("%04d%02d%02d%s", t.Year(), int(t.Month()), t.Day(), version)
	if f.Book != "" {
		seedKey += "b" + f.Book
	}
	if f.Testament != "" {
		seedKey += "t" + f.Testament
	}
	seed := hashSeed(seedKey)
	h := all[seed%uint32(len(all))]
	return &h, nil
}

func (e *Engine) RandomVerse(version string) (*VerseHit, error) {
	return e.RandomVerseFiltered(version, SampleFilter{})
}

func (e *Engine) RandomVerseFiltered(version string, f SampleFilter) (*VerseHit, error) {
	all, err := e.samplePool(version, f)
	if err != nil {
		return nil, err
	}
	h := all[rand.IntN(len(all))]
	if f.Seed != nil {
		rng := rand.New(rand.NewPCG(uint64(*f.Seed), uint64(*f.Seed)))
		h = all[rng.IntN(len(all))]
	}
	return &h, nil
}

func (e *Engine) samplePool(version string, f SampleFilter) ([]VerseHit, error) {
	ix, err := e.ensureIndex(version)
	if err != nil {
		return nil, err
	}
	all := filterHits(ix.hits, f.Book, f.Testament)
	if len(all) == 0 {
		return nil, ErrNotFound
	}
	return all, nil
}

func (e *Engine) ensureIndex(version string) (*searchIndex, error) {
	e.mu.RLock()
	if ix, ok := e.indexes[version]; ok {
		e.mu.RUnlock()
		return ix, nil
	}
	e.mu.RUnlock()

	c, ok := e.corpus()
	if !ok {
		return nil, ErrUnsupportedFeature
	}
	all, err := c.AllVerses(version)
	if err != nil {
		return nil, err
	}
	ix := buildIndex(contentHits(all))
	e.mu.Lock()
	if existing, ok := e.indexes[version]; ok {
		e.mu.Unlock()
		return existing, nil
	}
	e.indexes[version] = ix
	e.mu.Unlock()
	return ix, nil
}

func hashSeed(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

// Chain is a Source that delegates to members in order, returning the first
// non-ErrNotFound / non-ErrUnsupportedVersion result. Books returns the first
// non-error result; Translations are merged across members.
type Chain struct {
	sources []Source
}

func NewChain(sources ...Source) *Chain {
	return &Chain{sources: sources}
}

func (c *Chain) Translations() []Translation {
	var out []Translation
	seen := map[string]bool{}
	for _, s := range c.sources {
		for _, t := range s.Translations() {
			if !seen[t.ID] {
				seen[t.ID] = true
				out = append(out, t)
			}
		}
	}
	return out
}

func (c *Chain) Books(version string) ([]Book, error) {
	for _, s := range c.sources {
		b, err := s.Books(version)
		if err == nil {
			return b, nil
		}
	}
	return nil, ErrNotFound
}

func (c *Chain) Chapter(version, book string, chapter int) (*Chapter, error) {
	return c.ChapterContext(context.Background(), version, book, chapter)
}

func (c *Chain) ChapterContext(ctx context.Context, version, book string, chapter int) (*Chapter, error) {
	for _, s := range c.sources {
		ch, err := fetchChapter(ctx, s, version, book, chapter)
		if err == nil {
			return ch, nil
		}
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrUnsupportedVersion) {
			continue
		}
		return nil, err
	}
	return nil, ErrNotFound
}

// AllVerses delegates to the first member that implements Corpus, so that
// Search/Daily/Random keep working when local and scrape are chained.
func (c *Chain) AllVerses(version string) ([]VerseHit, error) {
	for _, s := range c.sources {
		if corp, ok := s.(Corpus); ok {
			return corp.AllVerses(version)
		}
	}
	return nil, ErrUnsupportedFeature
}
