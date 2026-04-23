package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

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
