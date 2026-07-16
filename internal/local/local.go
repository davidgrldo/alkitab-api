package local

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/davidgrldo/alkitab-api/internal/bible"
)

//go:embed data/*.json
var embedded embed.FS

type fileData struct {
	Translation bible.Translation `json:"translation"`
	Books       []struct {
		bible.Book
		ChapterData []struct {
			Number int           `json:"number"`
			Verses []bible.Verse `json:"verses"`
		} `json:"chapter_data"`
	} `json:"books"`
}

type translationData struct {
	Translation bible.Translation
	Books       []bible.Book
	Chapters    map[string]*bible.Chapter // key: book:chapter
}

// Local is a bible.Source backed by JSON files (embedded + optional data dir).
type Local struct {
	data map[string]*translationData
}

func New(dataDir string) (*Local, error) {
	l := &Local{data: map[string]*translationData{}}

	// embedded defaults
	entries, err := embedded.ReadDir("data")
	if err != nil {
		return nil, fmt.Errorf("local: read embed: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		b, err := embedded.ReadFile(filepath.Join("data", e.Name()))
		if err != nil {
			return nil, err
		}
		if err := l.load(b); err != nil {
			return nil, fmt.Errorf("local: parse %s: %w", e.Name(), err)
		}
	}

	// runtime overrides
	if dataDir != "" {
		des, err := os.ReadDir(dataDir)
		if err != nil {
			return nil, fmt.Errorf("local: read data dir %q: %w", dataDir, err)
		}
		for _, e := range des {
			if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
				continue
			}
			b, err := os.ReadFile(filepath.Join(dataDir, e.Name()))
			if err != nil {
				return nil, err
			}
			if err := l.load(b); err != nil {
				return nil, fmt.Errorf("local: parse %s: %w", e.Name(), err)
			}
		}
	}
	return l, nil
}

func (l *Local) load(raw []byte) error {
	var fd fileData
	if err := json.Unmarshal(raw, &fd); err != nil {
		return err
	}
	td := &translationData{
		Translation: fd.Translation,
		Chapters:    map[string]*bible.Chapter{},
	}
	for _, bk := range fd.Books {
		b := bk.Book
		td.Books = append(td.Books, b)
		for _, ch := range bk.ChapterData {
			td.Chapters[fmt.Sprintf("%s:%d", b.ID, ch.Number)] = &bible.Chapter{
				Translation: fd.Translation.ID,
				Book:        b.ID,
				Number:      ch.Number,
				Verses:      ch.Verses,
			}
		}
	}
	l.data[fd.Translation.ID] = td // later load with same id overrides earlier
	return nil
}

func (l *Local) Translations() []bible.Translation {
	out := make([]bible.Translation, 0, len(l.data))
	for _, td := range l.data {
		out = append(out, td.Translation)
	}
	return out
}

func (l *Local) Books(version string) ([]bible.Book, error) {
	td, ok := l.data[version]
	if !ok {
		return nil, bible.ErrUnsupportedVersion
	}
	return td.Books, nil
}

func (l *Local) Chapter(version, book string, chapter int) (*bible.Chapter, error) {
	td, ok := l.data[version]
	if !ok {
		return nil, bible.ErrUnsupportedVersion
	}
	c, ok := td.Chapters[fmt.Sprintf("%s:%d", book, chapter)]
	if !ok {
		return nil, bible.ErrNotFound
	}
	return c, nil
}

// AllVerses flattens a translation's corpus into VerseHit order.
// Books are emitted in the order they appear in the JSON; within a chapter,
// verses are emitted in stored order.
func (l *Local) AllVerses(version string) ([]bible.VerseHit, error) {
	td, ok := l.data[version]
	if !ok {
		return nil, bible.ErrUnsupportedVersion
	}
	var out []bible.VerseHit
	for _, b := range td.Books {
		for ch := 1; ch <= b.Chapters; ch++ {
			c, ok := td.Chapters[fmt.Sprintf("%s:%d", b.ID, ch)]
			if !ok {
				continue
			}
			for _, v := range c.Verses {
				out = append(out, bible.VerseHit{
					Translation: version, Book: b.ID, Chapter: ch, Verse: v,
				})
			}
		}
	}
	return out, nil
}
