package bible

import "context"

func lastContent(verses []Verse) int {
	n := 0
	for _, v := range verses {
		if (v.Type == "" || v.Type == "content") && v.Number > n {
			n = v.Number
		}
	}
	return n
}

func firstContent(verses []Verse) int {
	n := 0
	for _, v := range verses {
		if v.Type != "" && v.Type != "content" {
			continue
		}
		if n == 0 || v.Number < n {
			n = v.Number
		}
	}
	return n
}

func prevContentInChapter(verses []Verse, before int) int {
	n := 0
	for _, v := range verses {
		if v.Type != "" && v.Type != "content" {
			continue
		}
		if v.Number < before && v.Number > n {
			n = v.Number
		}
	}
	return n
}

func nextContentInChapter(verses []Verse, after int) int {
	n := 0
	for _, v := range verses {
		if v.Type != "" && v.Type != "content" {
			continue
		}
		if v.Number > after && (n == 0 || v.Number < n) {
			n = v.Number
		}
	}
	return n
}

func bookIndex(books []Book, id string) int {
	for i, b := range books {
		if b.ID == id {
			return i
		}
	}
	return -1
}

func lastChapter(b Book) int {
	if b.Chapters < 1 {
		return 1
	}
	return b.Chapters
}

func (e *Engine) AttachNeighbors(ctx context.Context, version string, ch *Chapter, spec *VerseSpec) {
	if ch == nil {
		return
	}
	books, err := e.src.Books(version)
	if err != nil || len(books) == 0 {
		return
	}
	load := func(book string, chapter int) *Chapter {
		c, err := e.ChapterContext(ctx, version, book, chapter)
		if err != nil {
			return nil
		}
		return c
	}
	if spec == nil {
		ch.Prev = prevChapterRef(books, ch.Book, ch.Number, load)
		ch.Next = nextChapterRef(books, ch.Book, ch.Number, load)
		return
	}
	ch.Prev = prevVerseRef(books, ch, spec.From, load)
	ch.Next = nextVerseRef(books, ch, spec.To, load)
}

func prevChapterRef(books []Book, book string, chapter int, load func(string, int) *Chapter) *Ref {
	if chapter > 1 {
		c := load(book, chapter-1)
		if c == nil {
			return nil
		}
		if n := lastContent(c.Verses); n > 0 {
			return &Ref{Book: book, Chapter: chapter - 1, Verse: n}
		}
		return nil
	}
	i := bookIndex(books, book)
	if i <= 0 {
		return nil
	}
	prev := books[i-1]
	ch := lastChapter(prev)
	c := load(prev.ID, ch)
	if c == nil {
		return nil
	}
	if n := lastContent(c.Verses); n > 0 {
		return &Ref{Book: prev.ID, Chapter: ch, Verse: n}
	}
	return nil
}

func nextChapterRef(books []Book, book string, chapter int, load func(string, int) *Chapter) *Ref {
	i := bookIndex(books, book)
	if i < 0 {
		return nil
	}
	if chapter < lastChapter(books[i]) {
		c := load(book, chapter+1)
		if c == nil {
			return nil
		}
		if n := firstContent(c.Verses); n > 0 {
			return &Ref{Book: book, Chapter: chapter + 1, Verse: n}
		}
		return nil
	}
	if i+1 >= len(books) {
		return nil
	}
	next := books[i+1]
	c := load(next.ID, 1)
	if c == nil {
		return nil
	}
	if n := firstContent(c.Verses); n > 0 {
		return &Ref{Book: next.ID, Chapter: 1, Verse: n}
	}
	return nil
}

func prevVerseRef(books []Book, ch *Chapter, from int, load func(string, int) *Chapter) *Ref {
	if n := prevContentInChapter(ch.Verses, from); n > 0 {
		return &Ref{Book: ch.Book, Chapter: ch.Number, Verse: n}
	}
	return prevChapterRef(books, ch.Book, ch.Number, load)
}

func nextVerseRef(books []Book, ch *Chapter, to int, load func(string, int) *Chapter) *Ref {
	if n := nextContentInChapter(ch.Verses, to); n > 0 {
		return &Ref{Book: ch.Book, Chapter: ch.Number, Verse: n}
	}
	return nextChapterRef(books, ch.Book, ch.Number, load)
}
