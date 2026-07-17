package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/davidgrldo/alkitab-api/bible"
	"github.com/davidgrldo/alkitab-api/local"
	"github.com/davidgrldo/alkitab-api/scrape"
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	<-ctx.Done()
	log.Print("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
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
