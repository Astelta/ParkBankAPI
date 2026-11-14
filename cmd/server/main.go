package main

import (
	//standard library
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	//my packages
	"astelta-api-v2/internal/cronjobs"
	"astelta-api-v2/internal/handlers"
	"astelta-api-v2/internal/metrics"
	"astelta-api-v2/internal/middleware"
	"astelta-api-v2/internal/utils"

	//postgres driver
	_ "github.com/lib/pq"
)

// global variables for database connection and server start time
var db *sql.DB
var startTime time.Time

func main() {
	startTime = time.Now()
	utils.LoadDotEnv(".env")

	//initial env vars
	host := utils.GetEnv("DB_HOST", "")
	dbPort := utils.GetEnv("DB_PORT", "")
	user := utils.GetEnv("DB_USER", "")
	password := utils.GetEnv("DB_PASSWORD", "")
	dbname := utils.GetEnv("DB_NAME", "")
	ca_certPath := utils.GetEnv("CA_CERT_PATH", "")
	serviceURI := utils.GetEnv("DATABASE_URL", "")

	if serviceURI == "" {
		missing := []string{}
		if host == "" {
			missing = append(missing, "DB_HOST")
		}
		if dbPort == "" {
			missing = append(missing, "DB_PORT")
		}
		if user == "" {
			missing = append(missing, "DB_USER")
		}
		if password == "" {
			missing = append(missing, "DB_PASSWORD")
		}
		if dbname == "" {
			missing = append(missing, "DB_NAME")
		}
		if len(missing) > 0 {
			log.Fatalf("DATABASE_URL not set and required env vars missing: %v", missing)
		}
		serviceURI = fmt.Sprintf("postgres://%s:%s@%s:%s/%s", user, password, host, dbPort, dbname)
		log.Printf("Using DB_* env vars to build connection string: host=%s port=%s dbname=%s", host, dbPort, dbname)
	} else {
		log.Println("Using DATABASE_URL from environment")
	}

	conn, err := url.Parse(serviceURI)
	if err != nil {
		log.Fatalf("Error parsing URI: %v", err)
	}

	query := conn.Query()
	if ca_certPath != "" {
		query.Set("sslmode", "verify-ca")
		query.Set("sslrootcert", ca_certPath)
	} else if query.Get("sslmode") == "" {
		query.Set("sslmode", "require")
	}
	conn.RawQuery = query.Encode()

	//create connection pool
	db, err = sql.Open("postgres", conn.String())
	if err != nil {
		log.Fatal(err)
	}

	if err = db.Ping(); err != nil {
		log.Fatalf("Cannot connect to database: %v", err)
	}

	//will close with main
	defer db.Close()

	// Refresh metrics once at startup so /metrics returns a cached server value.
	metrics.RefreshMetrics(db, startTime)
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			metrics.RefreshMetrics(db, startTime)
		}
	}()

	//endpoints
	http.HandleFunc("/health", middleware.CorsMiddleware(handlers.HealthHandler))
	http.HandleFunc("/metrics", middleware.CorsMiddleware(handlers.MetricsHandler))
	http.HandleFunc("/register", middleware.CorsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		handlers.RegisterHandler(w, r, db)
	}))
	http.HandleFunc("/login", middleware.CorsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		handlers.LoginHandler(w, r, db)
	}))
	http.HandleFunc("/profile", middleware.CorsMiddleware(middleware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		handlers.ProfileHandler(w, r, db)
	})))
	http.HandleFunc("/transfer", middleware.CorsMiddleware(middleware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		handlers.TransferHandler(w, r, db)
	})))
	http.HandleFunc("/logout", middleware.CorsMiddleware(middleware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		handlers.LogoutHandler(w, r, db)
	})))
	http.HandleFunc("/user", middleware.CorsMiddleware(middleware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		handlers.UserHandler(w, r, db)
	})))
	http.HandleFunc("/transactions", middleware.CorsMiddleware(middleware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		handlers.TransactionsHandler(w, r, db)
	})))

	//start the cron jobs
	cronjobs.StartAllCronJobs(db)

	// Start the server
	port := ":8097"
	fmt.Printf("Starting API server on port %s...\n", port)
	err = http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatalf("Error http: %v", err)
	}

}
