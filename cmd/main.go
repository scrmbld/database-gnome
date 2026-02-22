package main

import (
	"context"
	"database/sql"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/scrmbld/database-gnome/cmd/logging"
	_ "modernc.org/sqlite"
)

const PORT string = "4400"
const ADDR string = "0.0.0.0"

type PageData struct {
	Title string
	// currently, a list of product names
	Products []string
}

func addRoutes(mux *http.ServeMux, logger *log.Logger, db *sql.DB) {
	tmpl := template.Must(template.ParseFiles("./views/index.html"))

	fs := http.FileServer(http.Dir("./static"))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query("SELECT name FROM Name")
		if err != nil {
			logger.Printf("Database: %s", err)
			http.Error(w, "Internal database error", 500)
		}

		var names []string
		for rows.Next() {
			var name string
			rows.Scan(&name)
			names = append(names, name)
		}
		tmpl.Execute(w, PageData{"Home", names})
	})

	mux.Handle("/static", fs)
}

func NewServer(logger *log.Logger, db *sql.DB) http.Handler {
	mux := http.NewServeMux()
	addRoutes(mux, logger, db)

	var handler http.Handler = mux
	// middleware goes here
	handler = logging.LogWare(handler, logger)
	return handler
}

func run(ctx context.Context, logger *log.Logger, db *sql.DB) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	srv := NewServer(logger, db)

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

func main() {
	logger := log.New(os.Stderr, "HTTP: ", log.Ldate|log.Ltime|log.Lmsgprefix)
	db, err := sql.Open("sqlite", "./data/app.db")
	if err != nil {
		logger.Fatalf("failed to open database: %s", err)
	}
	ctx := context.Background()
	if err := run(ctx, logger, db); err != nil {
		logger.Printf("%s\n", err)
		os.Exit(1)
	}
}
