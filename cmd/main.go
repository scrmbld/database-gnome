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

	"github.com/scrmbld/database-gnome/cmd/glue"
	"github.com/scrmbld/database-gnome/cmd/gnome"
	"github.com/scrmbld/database-gnome/cmd/logging"

	"github.com/joho/godotenv"
	_ "modernc.org/sqlite"
)

const PORT string = "4400"
const ADDR string = "0.0.0.0"

const model string = "openai/gpt-oss-20b"

type ProductRecord struct {
	Name         string
	Mpg          sql.NullFloat64
	Cylinders    sql.NullInt32
	Horsepower   sql.NullFloat64
	Weight       sql.NullInt32
	ModelYear    sql.NullInt32
	Acceleration sql.NullFloat64
}

type PageData struct {
	Title    string
	Products []ProductRecord
}

func getProductInfo(logger *log.Logger, db *sql.DB, query string) ([]ProductRecord, error) {
	logger.Printf("Database: %s", query)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}

	var productsList []ProductRecord
	for rows.Next() {
		var name string
		var mpg, horsepower, acceleration sql.NullFloat64
		var cylinders, weight, modelYear sql.NullInt32

		err := rows.Scan(
			&name,
			&mpg,
			&cylinders,
			&horsepower,
			&weight,
			&modelYear,
			&acceleration,
		)
		if err != nil {
			return nil, err
		}
		productsList = append(productsList, ProductRecord{
			Name:         name,
			Mpg:          mpg,
			Cylinders:    cylinders,
			Horsepower:   horsepower,
			Weight:       weight,
			ModelYear:    modelYear,
			Acceleration: acceleration,
		})
	}

	return productsList, nil
}

func addRoutes(mux *http.ServeMux, httpLogger *log.Logger, db *sql.DB) {
	dbLogger := log.New(os.Stderr, "DB: ", log.Ldate|log.Ltime|log.Lmsgprefix)
	aiLogger := log.New(os.Stderr, "Model: ", log.Ldate|log.Ltime|log.Lmsgprefix)
	GroqModel := glue.NewGroqModel(model, aiLogger)
	dbGnome := gnome.NewGnome(aiLogger, &GroqModel)
	tmpl := template.Must(template.ParseFiles("./views/index.html"))

	fs := http.FileServer(http.Dir("./static"))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl.ExecuteTemplate(w, "index", PageData{"Home", []ProductRecord{}})
	})

	mux.HandleFunc("/filter", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", 405)
			return
		}

		userQuery := r.FormValue("filter-request")
		response, err := dbGnome.Query(userQuery)
		if err != nil {
			httpLogger.Printf("dbGnome: %s", err)
			http.Error(w, "AI error", 500)
		}

		// find all the items that match our filters
		productsList, err := getProductInfo(dbLogger, db, response)
		if err != nil {
			httpLogger.Printf("Database: %s", err)
			http.Error(w, "Database error", 500)
		}

		tmpl.ExecuteTemplate(w, "productList", productsList)
	})

	mux.Handle("/static/", http.StripPrefix("/static/", fs))
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
	godotenv.Load()
	httpLogger := log.New(os.Stderr, "HTTP: ", log.Ldate|log.Ltime|log.Lmsgprefix)
	db, err := sql.Open("sqlite", "./data/app.db")
	if err != nil {
		httpLogger.Fatalf("failed to open database: %s", err)
	}
	ctx := context.Background()
	if err := run(ctx, httpLogger, db); err != nil {
		httpLogger.Printf("%s\n", err)
		os.Exit(1)
	}
}
