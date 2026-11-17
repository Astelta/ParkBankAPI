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
	"astelta-api-v2/internal/utils"

	//postgres driver
	_ "github.com/lib/pq"
)

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
	db, err := sql.Open("postgres", conn.String())
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

	//start the cron jobs
	cronjobs.StartAllCronJobs(db)

	// Zamiast nil przekazujemy naszą instancję ServeMux
	router := handlers.Router(db)

	// Start the server
	port := ":8097"
	fmt.Printf("Starting API server on port %s...\n", port)
	err = http.ListenAndServe(port, router)
	if err != nil {
		log.Fatalf("Error http: %v", err)
	}
}
