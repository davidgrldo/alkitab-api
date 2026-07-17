package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/davidgrldo/alkitab-api/internal/bible"
	"github.com/davidgrldo/alkitab-api/internal/local"
	"github.com/davidgrldo/alkitab-api/internal/scrape"
	"github.com/davidgrldo/alkitab-api/internal/server"
)

func main() {
	port := getenv("ALKITAB_PORT", "3000")
	if !validPort(port) {
		log.Fatalf("invalid ALKITAB_PORT %q", port)
	}

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
	httpSrv := &http.Server{
		Addr:              ":" + port,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Fatal(httpSrv.ListenAndServe())
}

func validPort(p string) bool {
	n, err := strconv.Atoi(p)
	return err == nil && n >= 1 && n <= 65535
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
