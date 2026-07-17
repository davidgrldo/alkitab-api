// Command alkitab-convert converts a common public-domain Bible JSON dump
// (an array of {name, chapters: [[verse, ...], ...]} — e.g. the files in
// github.com/thiagobodruk/bible) into the alkitab-api BYOD format.
//
//	alkitab-convert -id kjv -name "King James Version" -lang en en_kjv.json > kjv.json
//
// Book names are resolved against the 66-book canon (English or Indonesian,
// case-insensitive); unknown books abort with a clear error so a bad dump
// never produces a silently incomplete translation.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/davidgrldo/alkitab-api/bible"
)

type inBook struct {
	Name     string     `json:"name"`
	Abbrev   string     `json:"abbrev"`
	Chapters [][]string `json:"chapters"`
}

type outVerse struct {
	Verse   int    `json:"verse"`
	Type    string `json:"type"`
	Content string `json:"content"`
}

type outChapter struct {
	Number int        `json:"number"`
	Verses []outVerse `json:"verses"`
}

type outBook struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Abbr        string       `json:"abbr"`
	Testament   string       `json:"testament"`
	Chapters    int          `json:"chapters"`
	ChapterData []outChapter `json:"chapter_data"`
}

type outFile struct {
	Translation bible.Translation `json:"translation"`
	Books       []outBook         `json:"books"`
}

func main() {
	id := flag.String("id", "", "translation id, e.g. kjv (required)")
	name := flag.String("name", "", "translation display name (required)")
	lang := flag.String("lang", "en", "translation language code")
	flag.Parse()
	if *id == "" || *name == "" || flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: alkitab-convert -id kjv -name \"King James Version\" [-lang en] input.json > out.json")
		os.Exit(2)
	}

	raw, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	raw = bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf")) // some dumps ship a UTF-8 BOM

	var in []inBook
	if err := json.Unmarshal(raw, &in); err != nil {
		log.Fatalf("parse input: %v", err)
	}

	meta := map[string]bible.Book{}
	for _, b := range bible.CanonicalBooks() {
		meta[b.ID] = b
	}

	canon := bible.CanonicalBooks()
	out := outFile{Translation: bible.Translation{ID: *id, Name: *name, Language: *lang}}
	verses := 0
	positional := false
	for i, b := range in {
		bookID, err := bible.ResolveBookID(b.Name)
		if err != nil {
			// Dumps without a name field (only ad-hoc abbrevs) are mapped by
			// position — sound only for a complete canonical 66-book dump.
			if len(in) != len(canon) {
				log.Fatalf("unknown book %q and input has %d books (need %d for positional mapping)", b.Name, len(in), len(canon))
			}
			bookID = canon[i].ID
			positional = true
		}
		m := meta[bookID]
		ob := outBook{ID: bookID, Name: m.Name, Abbr: m.Abbreviation, Testament: m.Testament, Chapters: len(b.Chapters)}
		for ci, ch := range b.Chapters {
			oc := outChapter{Number: ci + 1}
			for vi, content := range ch {
				oc.Verses = append(oc.Verses, outVerse{Verse: vi + 1, Type: "content", Content: content})
				verses++
			}
			ob.ChapterData = append(ob.ChapterData, oc)
		}
		out.Books = append(out.Books, ob)
	}

	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(out); err != nil {
		log.Fatal(err)
	}
	if positional {
		log.Print("note: book names missing from input — mapped by canonical position; spot-check the output")
	}
	log.Printf("converted %q: %d books, %d verses", *id, len(out.Books), verses)
}
