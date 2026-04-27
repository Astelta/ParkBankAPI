package handlers

import (
	"astelta-api-v2/internal/metrics"
	"astelta-api-v2/internal/utils"
	"database/sql"
	"encoding/json"
	"net/http"
)

func RegisterHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var requestData struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, "Error decoding request body", http.StatusBadRequest)
		return
	}

	if requestData.Username == "" || requestData.Password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}
	requestData.Username = utils.HashPassword(requestData.Username)
	requestData.Password = utils.HashPassword(requestData.Password)

	_, err := db.Query("INSERT INTO users (login, password) VALUES (?, ?)", requestData.Username, requestData.Password)
	if err != nil {
		http.Error(w, "Error inserting user", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("User registered successfully"))
}

func LoginHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var requestData struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, "Error decoding request body", http.StatusBadRequest)
		return
	}

	if requestData.Username == "" || requestData.Password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	// Query user by username
	var userID int64
	var storedPassword string
	err := db.QueryRow("SELECT id, password FROM users WHERE login = ?", utils.HashPassword(requestData.Username)).Scan(&userID, &storedPassword)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Invalid username or password", http.StatusUnauthorized)
			return
		}
		http.Error(w, "Error querying user", http.StatusInternalServerError)
		return
	}

	// Verify password
	if !utils.CheckPasswordHash(requestData.Password, storedPassword) {
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}

	// Create JWE token
	jweToken, err := utils.CreateAuthToken(userID, requestData.Username)
	if err != nil {
		http.Error(w, "Error creating token", http.StatusInternalServerError)
		return
	}

	// Return the JWE token
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"token": jweToken,
	})
}

// simple health check endpoint
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	status := "ok"
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(status))
}

// MetricsHandler returns system and application metrics in JSON format
func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := metrics.GetMetrics()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Error encoding metrics", http.StatusInternalServerError)
		return
	}
}

// ProfileHandler returns the user's profile data (id and balance)
func ProfileHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user ID from headers (set by AuthMiddleware)
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	// Query user profile
	var profile struct {
		ID      int64   `json:"id"`
		Balance float64 `json:"balance"`
	}

	err := db.QueryRow("SELECT id, balance FROM users WHERE id = ?", userID).Scan(&profile.ID, &profile.Balance)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Error querying user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(profile); err != nil {
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}
