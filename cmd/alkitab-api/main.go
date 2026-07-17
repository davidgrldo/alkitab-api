package main

import (
	"log"
	"net/http"
	"os"

	"github.com/davidgrldo/alkitab-api/internal/bible"
	"github.com/davidgrldo/alkitab-api/internal/local"
	"github.com/davidgrldo/alkitab-api/internal/scrape"
	"github.com/davidgrldo/alkitab-api/internal/server"
)

func main() {
	port := getenv("ALKITAB_PORT", "3000")

	loc, err := local.New(os.Getenv("ALKITAB_DATA_DIR"))
	if err != nil {
		log.Fatalf("local: %v", err)
	}

	var src bible.Source = loc
	if os.Getenv("ALKITAB_SCRAPE") == "1" {
		sc := scrape.New(os.Getenv("ALKITAB_BASE_URL"))
		src = bible.NewChain(loc, sc)
	}

	srv := server.New(bible.New(src))
	log.Printf("alkitab-api listening on :%s (scrape=%v)", port, os.Getenv("ALKITAB_SCRAPE") == "1")
	log.Fatal(http.ListenAndServe(":"+port, srv.Handler()))
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
