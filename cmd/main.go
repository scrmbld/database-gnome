package main

import (
	"context"
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/scrmbld/database-gnome/cmd/glue"
	"github.com/scrmbld/database-gnome/cmd/logging"

	"github.com/joho/godotenv"
	_ "modernc.org/sqlite"
)

const PORT string = "4400"
const ADDR string = "0.0.0.0"

const model string = "openai/gpt-oss-20b"

const sqlTemplate string = "SELECT Name.name, Observation.mpg, Observation.cylinders, Observation.horsepower, Observation.weight, Observation.model_year, Observation.acceleration FROM Observation INNER JOIN Name ON Name.name_id=Observation.name_id INNER JOIN Origin on Origin.origin_id=Observation.origin_id WHERE"

const systemPrompt string = "You are a part of a database agent system that enables users to filter products on an ecommerce website using natural language queries. Your task is to help generate sql queries based on the user input. Remember that, since you only help users filter, sort, and search product listings, you have no ability to perform any write operations to the database -- that includes DELETE, UPDATE, and INSERT operations. Also, you cannot influence the information shown on product listings, only which listings are shown and what order they are in. Here is the databse schema:\\n```sql\\nCREATE TABLE IF NOT EXISTS \\\"Observation\\\" (\\n\\tmpg FLOAT,\\n\\tcylinders BIGINT,\\n\\tdisplacement FLOAT,\\n\\thorsepower FLOAT,\\n\\tweight BIGINT,\\n\\tacceleration FLOAT,\\n\\tmodel_year BIGINT,\\n\\torigin_id BIGINT,\\n\\tname_id BIGINT\\n);\\nCREATE TABLE IF NOT EXISTS \\\"Origin\\\" (\\n\\torigin_id BIGINT,\\n\\torigin TEXT\\n);\\nCREATE TABLE IF NOT EXISTS \\\"Name\\\" (\\n\\tname_id BIGINT,\\n\\tname TEXT\\n);\\n```\\nThe system will run your SQL code to get a list of name_id values that match your filters. This list is then used to generate the web view. Your output must ONLY include SQL code in plain text format (no markdown). Anything else WILL break the system.\\n\\nWhen you receive a user request, complete the following SQL so that it returns the name_id of all products that match the said user request.\\n```sql\\n" + sqlTemplate + "```\\nDo not repeat the already provided SQL code in your response, only include the parts that you have come up with."

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

func getProductInfo(logger *log.Logger, db *sql.DB, modelFilters string) ([]ProductRecord, error) {
	query := fmt.Sprintf("%s %s", sqlTemplate, modelFilters)
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

func addRoutes(mux *http.ServeMux, logger *log.Logger, db *sql.DB) {
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

		query := r.FormValue("filter-request")
		response, err := glue.Request(logger, model, systemPrompt, query)
		if err != nil {
			logger.Printf("Model: %s", err)
			http.Error(w, "AI error", 500)
		}

		// find all the items that match our filters
		productsList, err := getProductInfo(logger, db, response.Choices[0].Message.Content)
		if err != nil {
			logger.Printf("Database: %s", err)
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
