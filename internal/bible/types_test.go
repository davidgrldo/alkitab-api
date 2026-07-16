package bible

import (
	"errors"
	"fmt"
	"testing"
)

func TestTypesAndErrors(t *testing.T) {
	v := Verse{Number: 1, Content: "In the beginning", Type: "content", Order: 0}
	if v.Number != 1 || v.Type != "content" {
		t.Fatalf("verse not constructed: %+v", v)
	}
	if !errors.Is(ErrNotFound, ErrNotFound) {
		t.Fatal("ErrNotFound must support errors.Is")
	}
}

// fakeSource is reused by engine tests in later tasks.
type fakeSource struct {
	trans []Translation
	books []Book
	chaps map[string]*Chapter
	calls int
}

func (f *fakeSource) Translations() []Translation          { return f.trans }
func (f *fakeSource) Books(version string) ([]Book, error) { return f.books, nil }
func (f *fakeSource) Chapter(version, book string, chapter int) (*Chapter, error) {
	f.calls++
	c, ok := f.chaps[fmt.Sprintf("%s:%s:%d", version, book, chapter)]
	if !ok {
		return nil, ErrNotFound
	}
	return c, nil
}
