package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"

	// Znak "_" (blank identifier) przed importem oznacza, że importujemy
	// ten pakiet wyłącznie dla jego "efektów ubocznych".
	// W tym przypadku kod inicjalizacyjny wewnątrz pakietu "pq"
	// rejestruje ten sterownik w pakiecie "database/sql" pod nazwą "postgres".
	// Bezpośrednio w kodzie nie używamy żadnych funkcji z pakietu "pq".
	_ "github.com/lib/pq"
)

// Zmienna globalna dla połączenia z bazą, aby handler miał do niej dostęp.
// W aplikacjach produkcyjnych zazwyczaj wstrzykuje się ją jako zależność,
// ale dla czystości tego przykładu używamy zmiennej na poziomie pakietu.
var db *sql.DB
var startTime time.Time

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func loadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}

func main() {
	startTime = time.Now()
	loadDotEnv(".env")
	// 1. Definiowanie łańcucha połączenia (Data Source Name - DSN)
	// Zastąp te wartości swoimi danymi do zdalnej bazy PostgreSQL.
	host := getEnv("DB_HOST", "")
	dbPort := getEnv("DB_PORT", "")
	user := getEnv("DB_USER", "")
	password := getEnv("DB_PASSWORD", "")
	dbname := getEnv("DB_NAME", "")
	ca_certPath := getEnv("CA_CERT_PATH", "")
	serviceURI := getEnv("DATABASE_URL", "")

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

	// 2. Otwieranie "połączenia" z bazą danych.
	// Funkcja sql.Open NIE NAWIĄZUJE fizycznego połączenia w tym momencie.
	// Ona jedynie sprawdza poprawność argumentów i przygotowuje pulę połączeń (connection pool).
	db, err = sql.Open("postgres", conn.String())
	if err != nil {
		log.Fatal(err)
	}

	if err = db.Ping(); err != nil {
		log.Fatalf("Nie można połączyć się z bazą: %v", err)
	}

	// Słowo kluczowe defer oznacza, że funkcja db.Close() zostanie wywołana
	// w momencie, gdy funkcja main() zakończy swoje działanie.
	// Gwarantuje to poprawne zamknięcie i zwolnienie zasobów bazy danych.
	defer db.Close()

	// 3. Rejestracja endpointu API
	// Wiążemy ścieżkę "/db-status" z naszą funkcją statusHandler.
	http.HandleFunc("/db-status", statusHandler)
	http.HandleFunc("/health", corsMiddleware(healthHandler))
	http.HandleFunc("/metrics", corsMiddleware(metricsHandler))

	// 4. Uruchomienie serwera HTTP
	port := ":8097"
	fmt.Printf("Uruchamianie serwera API na porcie %s...\n", port)

	// http.ListenAndServe blokuje główny wątek (program się nie kończy).
	// Zaczyna nasłuchiwać na wskazanym porcie. Jeśli z jakiegoś powodu
	// serwer nie będzie mógł wystartować (np. port jest już zajęty), funkcja zwróci błąd.
	err = http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatalf("Błąd serwera HTTP: %v", err)
	}
}

// statusHandler to nasza funkcja obsługująca żądania HTTP
func statusHandler(w http.ResponseWriter, r *http.Request) {
	// Używamy metody zapytania, aby sprawdzić, co klient chce zrobić.
	if r.Method != http.MethodGet {
		// Używamy http.Error, aby szybko wysłać kod błędu HTTP i wiadomość tekstową
		http.Error(w, "Metoda nieobsługiwana", http.StatusMethodNotAllowed)
		return
	}

	// Query the database version
	rows, err := db.Query("SELECT version()")
	if err != nil {
		http.Error(w, "Error querying database", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	if rows.Next() {
		var result string
		err = rows.Scan(&result)
		if err != nil {
			http.Error(w, "Error scanning result", http.StatusInternalServerError)
			return
		}
		// Jeśli wszystko jest w porządku, wysyłamy status 200 OK
		w.WriteHeader(http.StatusOK)
		// Fmt.Fprintf pozwala na bezpośrednie pisanie do http.ResponseWriter,
		// dzięki czemu tekst trafia prosto do klienta jako odpowiedź HTTP.
		fmt.Fprintf(w, "API działa. PostgreSQL version: %s", result)
	} else {
		http.Error(w, "No result", http.StatusInternalServerError)
		return
	}

	if err = rows.Err(); err != nil {
		http.Error(w, "Error reading rows", http.StatusInternalServerError)
		return
	}
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	status := "ok"
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(status))
}

// MetricsResponse represents the JSON structure for metrics endpoint
type MetricsResponse struct {
	Uptime          string        `json:"uptime"`
	DatabaseStatus  string        `json:"database_status"`
	DatabaseLatency time.Duration `json:"database_latency_ms"`
	GoVersion       string        `json:"go_version"`
	MemoryStats     MemoryStats   `json:"memory_stats"`
	Timestamp       time.Time     `json:"timestamp"`
}

// MemoryStats represents memory usage information
type MemoryStats struct {
	Alloc        uint64 `json:"alloc_bytes"`
	TotalAlloc   uint64 `json:"total_alloc_bytes"`
	Sys          uint64 `json:"sys_bytes"`
	Lookups      uint64 `json:"lookups"`
	Mallocs      uint64 `json:"mallocs"`
	Frees        uint64 `json:"frees"`
	HeapAlloc    uint64 `json:"heap_alloc_bytes"`
	HeapSys      uint64 `json:"heap_sys_bytes"`
	HeapIdle     uint64 `json:"heap_idle_bytes"`
	HeapInuse    uint64 `json:"heap_inuse_bytes"`
	HeapReleased uint64 `json:"heap_released_bytes"`
	HeapObjects  uint64 `json:"heap_objects"`
	StackInuse   uint64 `json:"stack_inuse_bytes"`
	StackSys     uint64 `json:"stack_sys_bytes"`
	GCSys        uint64 `json:"gc_sys_bytes"`
	NextGC       uint64 `json:"next_gc_bytes"`
	LastGC       string `json:"last_gc"`
	NumGC        uint32 `json:"num_gc"`
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Calculate uptime
	uptime := time.Since(startTime)

	// Check database status and measure latency
	dbStatus := "disconnected"
	var dbLatency time.Duration

	if db != nil {
		start := time.Now()
		err := db.Ping()
		dbLatency = time.Since(start)

		if err == nil {
			dbStatus = "connected"
		} else {
			dbStatus = "error"
			dbLatency = 0
		}
	}

	// Get memory statistics
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	memoryStats := MemoryStats{
		Alloc:        m.Alloc,
		TotalAlloc:   m.TotalAlloc,
		Sys:          m.Sys,
		Lookups:      m.Lookups,
		Mallocs:      m.Mallocs,
		Frees:        m.Frees,
		HeapAlloc:    m.HeapAlloc,
		HeapSys:      m.HeapSys,
		HeapIdle:     m.HeapIdle,
		HeapInuse:    m.HeapInuse,
		HeapReleased: m.HeapReleased,
		HeapObjects:  m.HeapObjects,
		StackInuse:   m.StackInuse,
		StackSys:     m.StackSys,
		GCSys:        m.GCSys,
		NextGC:       m.NextGC,
		LastGC:       time.Unix(0, int64(m.LastGC)).Format(time.RFC3339),
		NumGC:        m.NumGC,
	}

	// Create response
	response := MetricsResponse{
		Uptime:          uptime.String(),
		DatabaseStatus:  dbStatus,
		DatabaseLatency: dbLatency,
		GoVersion:       runtime.Version(),
		MemoryStats:     memoryStats,
		Timestamp:       time.Now(),
	}

	// Set JSON content type
	w.Header().Set("Content-Type", "application/json")

	// Encode and send response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Error encoding metrics", http.StatusInternalServerError)
		return
	}
}
