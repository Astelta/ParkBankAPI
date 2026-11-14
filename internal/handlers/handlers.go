package handlers

import (
	"astelta-api-v2/internal/metrics"
	"astelta-api-v2/internal/utils"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"
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
	requestData.Password = utils.HashPassword(requestData.Password)

	_, err := db.Query("INSERT INTO users (login, password) VALUES ($1, $2)", requestData.Username, requestData.Password)
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
	err := db.QueryRow("SELECT id, password FROM users WHERE login = $1", requestData.Username).Scan(&userID, &storedPassword)
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

	err := db.QueryRow("SELECT id, balance FROM users WHERE id = $1", userID).Scan(&profile.ID, &profile.Balance)
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

func TransferHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user ID from headers (set by AuthMiddleware)
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	// Parse request body for transfer details
	var requestData struct {
		RecipientID int64   `json:"recipient_id"`
		Amount      float64 `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, "Error parsing request body", http.StatusBadRequest)
		return
	}

	// Validate transfer details
	if requestData.RecipientID <= 0 || requestData.Amount <= 0 {
		http.Error(w, "Invalid recipient ID or amount", http.StatusBadRequest)
		return
	}

	//check if the recipient exists
	var recipientExists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", requestData.RecipientID).Scan(&recipientExists)
	if err != nil {
		http.Error(w, "Error querying recipient", http.StatusInternalServerError)
		return
	}
	if !recipientExists {
		http.Error(w, "Recipient not found", http.StatusNotFound)
		return
	}

	// Perform the transfer in a transaction
	tx, err := db.Begin()
	//check if the user has enough balance
	var senderBalance float64
	err = tx.QueryRow("SELECT balance FROM users WHERE id = $1", userID).Scan(&senderBalance)
	if err != nil {
		http.Error(w, "Error querying sender balance", http.StatusInternalServerError)
		return
	}
	if senderBalance < requestData.Amount {
		http.Error(w, "Insufficient balance", http.StatusBadRequest)
		return
	}
	// Deduct amount from sender
	_, err = tx.Exec("UPDATE users SET balance = balance - $1 WHERE id = $2", requestData.Amount, userID)
	if err != nil {
		tx.Rollback()
		http.Error(w, "Error updating sender balance", http.StatusInternalServerError)
		return
	}
	// Add amount to recipient
	_, err = tx.Exec("UPDATE users SET balance = balance + $1 WHERE id = $2", requestData.Amount, requestData.RecipientID)
	if err != nil {
		tx.Rollback()
		http.Error(w, "Error updating recipient balance", http.StatusInternalServerError)
		return
	}
	// Commit the transaction
	if err = tx.Commit(); err != nil {
		http.Error(w, "Error committing transaction", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Transfer successful"))
}

func LogoutHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user ID from headers (set by AuthMiddleware)
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	// Add the token to the blacklist in the database
	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")
	//get the expiration time of the token
	expiration := time.Now().Add(24 * time.Hour) // Assuming the token is valid for 24 hours
	_, err := db.Exec("INSERT INTO tokens (token, user_id, expiration) VALUES ($1, $2, $3)", token, userID, expiration)
	if err != nil {
		http.Error(w, "Error blacklisting token", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Logout successful"))
}

func UserHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if the request containst an id or a nickname in the query parameters
	requestedUserID := r.URL.Query().Get("id")
	requestedUsername := r.URL.Query().Get("nickname")

	// Get requested user ID from query parameters
	if requestedUserID == "" && requestedUsername == "" {
		http.Error(w, "User ID or username is required", http.StatusBadRequest)
		return
	}

	var user struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
	}

	// lookup requested user data from the database
	var err error
	if requestedUserID != "" {
		err = db.QueryRow("SELECT id, login FROM users WHERE id = $1", requestedUserID).Scan(&user.ID, &user.Username)
	} else {
		err = db.QueryRow("SELECT id, login FROM users WHERE login = $1", requestedUsername).Scan(&user.ID, &user.Username)
	}
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Error querying user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}
