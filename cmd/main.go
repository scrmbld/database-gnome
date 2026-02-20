package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/scrmbld/database-gnome/cmd/logging"
)

const PORT string = "4400"
const ADDR string = "0.0.0.0"

func NewServer(
	logger *log.Logger,
) http.Handler {
	mux := http.NewServeMux()
	addRoutes(mux, logger)

	var handler http.Handler = mux
	// middleware goes here
	handler = logging.LogWare(handler, logger)
	return handler
}

func run(ctx context.Context, logger *log.Logger) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	srv := NewServer(logger)

	httpServer := &http.Server{
		Addr:    net.JoinHostPort(ADDR, PORT),
		Handler: srv,
	}

	go func() {
		logger.Printf("listening on %s\n", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Printf("error listening and serving: %s\n", err)
		}
	}()

	// handle stopping gracefully
	var wg sync.WaitGroup
	wg.Go(func() {
		defer wg.Done()
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Printf("error shutting down http server")
		}
	})

	wg.Wait()
	return nil
}

func addRoutes(mux *http.ServeMux, logger *log.Logger) {
	fs := http.FileServer(http.Dir("./static"))
	mux.Handle("/", fs)
}

func main() {
	logger := log.New(os.Stderr, "HTTP: ", log.Ldate|log.Ltime|log.Lmsgprefix)
	ctx := context.Background()
	if err := run(ctx, logger); err != nil {
		logger.Printf("%s\n", err)
		os.Exit(1)
	}
}
