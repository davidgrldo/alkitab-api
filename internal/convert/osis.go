package convert

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/davidgrldo/alkitab-api/bible"
)

func DetectOSIS(raw []byte) bool {
	s := strings.ToLower(string(bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))))
	return strings.Contains(s, "<osis")
}

func FromOSIS(raw []byte, opt Options) (OutFile, error) {
	raw = bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))
	dec := xml.NewDecoder(bytes.NewReader(raw))
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
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return OutFile{}, fmt.Errorf("osis: %w", err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch local(se.Name) {
		case "div":
			if attr(se, "type") != "book" {
				continue
			}
			flush()
			id, err := resolveOSISBook(attr(se, "osisID"))
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
		case "chapter":
			if !hasBook {
				return OutFile{}, fmt.Errorf("osis: chapter before book")
			}
			n := osisChapterNum(attr(se, "osisID"))
			if n < 1 {
				return OutFile{}, fmt.Errorf("osis: bad chapter id %q", attr(se, "osisID"))
			}
			cur.ChapterData = append(cur.ChapterData, outChapter{Number: n})
			chapIdx = len(cur.ChapterData) - 1
			cur.Chapters = len(cur.ChapterData)
			lastVerse = 0
		case "verse":
			if chapIdx < 0 {
				return OutFile{}, fmt.Errorf("osis: verse before chapter")
			}
			vn := osisVerseNum(attr(se, "osisID"))
			text, err := innerText(dec, se.Name)
			if err != nil {
				return OutFile{}, err
			}
			if vn < 1 {
				return OutFile{}, fmt.Errorf("osis: bad verse id %q", attr(se, "osisID"))
			}
			cur.ChapterData[chapIdx].Verses = append(cur.ChapterData[chapIdx].Verses, outVerse{Verse: vn, Type: "content", Content: text})
			lastVerse = vn
			out.VerseCount++
		case "title":
			if chapIdx < 0 {
				_, _ = innerText(dec, se.Name)
				continue
			}
			text, err := innerText(dec, se.Name)
			if err != nil {
				return OutFile{}, err
			}
			if text == "" {
				continue
			}
			cur.ChapterData[chapIdx].Verses = append(cur.ChapterData[chapIdx].Verses, outVerse{Verse: lastVerse + 1, Type: "title", Content: text})
		}
	}
	flush()
	if len(out.Books) == 0 {
		return OutFile{}, fmt.Errorf("osis: no books")
	}
	if err := maybeValidate(out, opt.Validate); err != nil {
		return OutFile{}, err
	}
	return out, nil
}

func local(n xml.Name) string { return n.Local }

func attr(se xml.StartElement, name string) string {
	for _, a := range se.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

func innerText(dec *xml.Decoder, until xml.Name) (string, error) {
	var b strings.Builder
	depth := 1
	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		case xml.CharData:
			b.Write(t)
		}
	}
	_ = until
	return strings.TrimSpace(b.String()), nil
}

func osisChapterNum(id string) int {
	parts := strings.Split(id, ".")
	if len(parts) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(parts[len(parts)-1])
	return n
}

func osisVerseNum(id string) int {
	parts := strings.Split(id, ".")
	if len(parts) < 3 {
		return 0
	}
	n, _ := strconv.Atoi(parts[len(parts)-1])
	return n
}

func resolveOSISBook(id string) (string, error) {
	id = strings.TrimSpace(strings.Split(id, ".")[0])
	if id == "" {
		return "", fmt.Errorf("osis: missing book id")
	}
	if resolved, err := resolveUSFM(id); err == nil {
		return resolved, nil
	}
	return bible.ResolveBookID(id)
}
