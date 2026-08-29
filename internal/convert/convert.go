package convert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

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

type OutBook struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Abbr        string       `json:"abbr"`
	Testament   string       `json:"testament"`
	Chapters    int          `json:"chapters"`
	ChapterData []outChapter `json:"chapter_data"`
}

type OutFile struct {
	Translation bible.Translation `json:"translation"`
	Books       []OutBook         `json:"books"`
	Positional  bool              `json:"-"`
	VerseCount  int               `json:"-"`
}

type Options struct {
	ID       string
	Name     string
	Lang     string
	Locale   string
	Validate bool
}

func FromJSON(raw []byte, opt Options) (OutFile, error) {
	raw = bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))
	var in []inBook
	if err := json.Unmarshal(raw, &in); err != nil {
		return OutFile{}, fmt.Errorf("parse input: %w", err)
	}
	canon := bible.CanonicalBooks()
	out := OutFile{Translation: bible.Translation{ID: opt.ID, Name: opt.Name, Language: opt.Lang}}
	for i, b := range in {
		bookID, err := bible.ResolveBookID(b.Name)
		if err != nil {
			if len(in) != len(canon) {
				return OutFile{}, fmt.Errorf("unknown book %q and input has %d books (need %d for positional mapping)", b.Name, len(in), len(canon))
			}
			bookID = canon[i].ID
			out.Positional = true
		}
		ob, n, err := buildBook(bookID, opt.Locale, b.Chapters)
		if err != nil {
			return OutFile{}, err
		}
		out.VerseCount += n
		out.Books = append(out.Books, ob)
	}
	if err := maybeValidate(out, opt.Validate); err != nil {
		return OutFile{}, err
	}
	return out, nil
}

func FromUSFM(raw []byte, opt Options) (OutFile, error) {
	raw = bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))
	out := OutFile{Translation: bible.Translation{ID: opt.ID, Name: opt.Name, Language: opt.Lang}}
	var cur OutBook
	hasBook := false
	chapIdx := -1
	lastVerse := 0
	flush := func() {
		if hasBook {
			out.Books = append(out.Books, cur)
		}
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		tag, rest, _ := strings.Cut(line, " ")
		rest = strings.TrimSpace(rest)
		switch {
		case tag == `\id`:
			flush()
			code, _, _ := strings.Cut(rest, " ")
			id, err := resolveUSFM(code)
			if err != nil {
				return OutFile{}, err
			}
			ob, _, err := buildBook(id, opt.Locale, nil)
			if err != nil {
				return OutFile{}, err
			}
			cur = ob
			hasBook = true
			chapIdx = -1
			lastVerse = 0
		case tag == `\c`:
			if !hasBook {
				return OutFile{}, fmt.Errorf("usfm: \\c before \\id")
			}
			fields := strings.Fields(rest)
			if len(fields) == 0 {
				return OutFile{}, fmt.Errorf("usfm: missing chapter number")
			}
			n, err := strconv.Atoi(fields[0])
			if err != nil {
				return OutFile{}, fmt.Errorf("usfm: bad chapter %q", rest)
			}
			cur.ChapterData = append(cur.ChapterData, outChapter{Number: n})
			chapIdx = len(cur.ChapterData) - 1
			cur.Chapters = len(cur.ChapterData)
			lastVerse = 0
		case tag == `\v`:
			if chapIdx < 0 {
				return OutFile{}, fmt.Errorf("usfm: \\v before \\c")
			}
			numS, text, _ := strings.Cut(rest, " ")
			vn, err := strconv.Atoi(numS)
			if err != nil {
				return OutFile{}, fmt.Errorf("usfm: bad verse %q", rest)
			}
			cur.ChapterData[chapIdx].Verses = append(cur.ChapterData[chapIdx].Verses, outVerse{Verse: vn, Type: "content", Content: strings.TrimSpace(text)})
			lastVerse = vn
			out.VerseCount++
		case strings.HasPrefix(tag, `\s`):
			if chapIdx < 0 || rest == "" {
				continue
			}
			cur.ChapterData[chapIdx].Verses = append(cur.ChapterData[chapIdx].Verses, outVerse{Verse: lastVerse + 1, Type: "title", Content: rest})
		}
	}
	flush()
	if len(out.Books) == 0 {
		return OutFile{}, fmt.Errorf("usfm: no books")
	}
	if err := maybeValidate(out, opt.Validate); err != nil {
		return OutFile{}, err
	}
	return out, nil
}

func DetectUSFM(raw []byte) bool {
	s := strings.TrimSpace(string(bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))))
	return strings.HasPrefix(s, `\id`)
}

func buildBook(bookID, locale string, chapters [][]string) (OutBook, int, error) {
	m, ok := bible.DisplayBook(bookID, locale)
	if !ok {
		return OutBook{}, 0, fmt.Errorf("unknown book id %q", bookID)
	}
	ob := OutBook{ID: m.ID, Name: m.Name, Abbr: m.Abbreviation, Testament: m.Testament, Chapters: len(chapters)}
	n := 0
	for ci, ch := range chapters {
		oc := outChapter{Number: ci + 1}
		for vi, content := range ch {
			oc.Verses = append(oc.Verses, outVerse{Verse: vi + 1, Type: "content", Content: content})
			n++
		}
		ob.ChapterData = append(ob.ChapterData, oc)
	}
	if chapters == nil {
		ob.Chapters = 0
	}
	return ob, n, nil
}

func maybeValidate(out OutFile, validate bool) error {
	if !validate {
		return nil
	}
	canon := map[string]bible.Book{}
	for _, b := range bible.CanonicalBooks() {
		canon[b.ID] = b
	}
	for _, b := range out.Books {
		c, ok := canon[b.ID]
		if !ok {
			return fmt.Errorf("validate: unknown book %s", b.ID)
		}
		if b.Chapters != c.Chapters {
			return fmt.Errorf("validate: %s has %d chapters, canon has %d", b.ID, b.Chapters, c.Chapters)
		}
	}
	return nil
}

var usfmID = map[string]string{
	"GEN": "gen", "EXO": "exod", "LEV": "lev", "NUM": "num", "DEU": "deut",
	"JOS": "josh", "JDG": "judg", "RUT": "ruth", "1SA": "1sam", "2SA": "2sam",
	"1KI": "1kgs", "2KI": "2kgs", "1CH": "1chr", "2CH": "2chr", "EZR": "ezra",
	"NEH": "neh", "EST": "esth", "JOB": "job", "PSA": "ps", "PRO": "prov",
	"ECC": "eccl", "SNG": "song", "ISA": "isa", "JER": "jer", "LAM": "lam",
	"EZK": "ezek", "DAN": "dan", "HOS": "hos", "JOL": "joel", "AMO": "amos",
	"OBA": "obad", "JON": "jonah", "MIC": "mic", "NAM": "nah", "HAB": "hab",
	"ZEP": "zeph", "HAG": "hag", "ZEC": "zech", "MAL": "mal", "MAT": "matt",
	"MRK": "mark", "LUK": "luke", "JHN": "john", "ACT": "acts", "ROM": "rom",
	"1CO": "1cor", "2CO": "2cor", "GAL": "gal", "EPH": "eph", "PHP": "phil",
	"COL": "col", "1TH": "1thess", "2TH": "2thess", "1TI": "1tim", "2TI": "2tim",
	"TIT": "titus", "PHM": "phlm", "HEB": "heb", "JAS": "jas", "1PE": "1pet",
	"2PE": "2pet", "1JN": "1john", "2JN": "2john", "3JN": "3john", "JUD": "jude",
	"REV": "rev",
}

func resolveUSFM(code string) (string, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if id, ok := usfmID[code]; ok {
		return id, nil
	}
	return bible.ResolveBookID(code)
}
