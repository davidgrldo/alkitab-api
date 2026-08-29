// Command alkitab-convert converts a public-domain Bible dump (thiagobodruk JSON,
// USFM, or OSIS) into the alkitab-api BYOD format.
//
//	alkitab-convert -id kjv -name "King James Version" -lang en en_kjv.json > kjv.json
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/davidgrldo/alkitab-api/internal/convert"
)

func main() {
	id := flag.String("id", "", "translation id, e.g. kjv (required)")
	name := flag.String("name", "", "translation display name (required)")
	lang := flag.String("lang", "en", "translation language code")
	locale := flag.String("name-locale", "en", "book names: en or id")
	format := flag.String("format", "auto", "auto, json, usfm, osis, or csv")
	validate := flag.Bool("validate", false, "require canon chapter counts")
	outFmt := flag.String("out", "json", "json or csv")
	flag.Parse()
	if *id == "" || *name == "" || flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: alkitab-convert -id kjv -name \"King James Version\" [-lang en] [-name-locale en] [-format auto] [-out json] [-validate] input > out.json")
		os.Exit(2)
	}

	raw, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	opt := convert.Options{ID: *id, Name: *name, Lang: *lang, Locale: *locale, Validate: *validate}

	out, err := convert.From(raw, *format, opt)
	if err != nil {
		log.Fatal(err)
	}
	switch strings.ToLower(*outFmt) {
	case "csv":
		b, err := convert.ToCSV(out)
		if err != nil {
			log.Fatal(err)
		}
		if _, err := os.Stdout.Write(b); err != nil {
			log.Fatal(err)
		}
	default:
		enc := json.NewEncoder(os.Stdout)
		if err := enc.Encode(out); err != nil {
			log.Fatal(err)
		}
	}
	if out.Positional {
		log.Print("note: book names missing from input — mapped by canonical position; spot-check the output")
	}
	log.Printf("converted %q: %d books, %d verses", *id, len(out.Books), out.VerseCount)
}
