package handlers

import (
	"astelta-api-v2/internal/metrics"
	"astelta-api-v2/internal/utils"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq"
)

func RegisterHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodPost {
		utils.ReturnErrorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	var requestData struct {
		Nickname string `json:"nickname"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		utils.ReturnErrorResponse(w, http.StatusBadRequest, "BAD_REQUEST", "Error decoding request body")
		return
	}

	if requestData.Nickname == "" || requestData.Password == "" {
		utils.ReturnErrorResponse(w, http.StatusBadRequest, "BAD_REQUEST", "nickname and password are required")
		return
	}
	requestData.Password = utils.HashPassword(requestData.Password)

	_, err := db.Exec("INSERT INTO users (login, password) VALUES ($1, $2)", requestData.Nickname, requestData.Password)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			utils.ReturnErrorResponse(w, http.StatusConflict, "USER_ALREADY_EXISTS", "Nickname is already taken")
			return
		}
		utils.ReturnErrorResponse(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Error inserting user")
		return
	}

	utils.ReturnSuccessResponse(w, http.StatusCreated, "User registered successfully", map[string]string{
		"nickname": requestData.Nickname,
	})
}

func LoginHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodPost {
		utils.ReturnErrorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	var requestData struct {
		Nickname string `json:"nickname"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		utils.ReturnErrorResponse(w, http.StatusBadRequest, "BAD_REQUEST", "Error decoding request body")
		return
	}

	if requestData.Nickname == "" || requestData.Password == "" {
		utils.ReturnErrorResponse(w, http.StatusBadRequest, "BAD_REQUEST", "nickname and password are required")
		return
	}

	// Query user by nickname
	var userID int64
	var storedPassword string
	err := db.QueryRow("SELECT id, password FROM users WHERE login = $1", requestData.Nickname).Scan(&userID, &storedPassword)
	if err != nil {
		if err == sql.ErrNoRows {
			utils.ReturnErrorResponse(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid nickname or password")
			return
		}
		utils.ReturnErrorResponse(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Error querying user")
		return
	}

	// Verify password
	if !utils.CheckPasswordHash(requestData.Password, storedPassword) {
		utils.ReturnErrorResponse(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid nickname or password")
		return
	}

	// Create JWE token
	jweToken, err := utils.CreateAuthToken(userID, requestData.Nickname)
	if err != nil {
		utils.ReturnErrorResponse(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Error creating auth token")
		return
	}

	// Return the JWE token
	utils.ReturnSuccessResponse(w, http.StatusOK, "Login successful", map[string]string{
		"token": jweToken,
	})
}

// simple health check endpoint
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.ReturnErrorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}
	utils.ReturnSuccessResponse(w, http.StatusOK, "API is healthy", map[string]string{
		"status": "ok",
	})
}

// MetricsHandler returns system and application metrics in JSON format
func MetricsHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodGet {
		utils.ReturnErrorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	response := metrics.GetMetrics()
	utils.ReturnSuccessResponse(w, http.StatusOK, "Metrics retrieved successfully", response)
}

// ProfileHandler returns the user's profile data (id and balance)
func ProfileHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodGet {
		utils.ReturnErrorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	// Get user ID from headers (set by AuthMiddleware)
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		utils.ReturnErrorResponse(w, http.StatusUnauthorized, "USER_NOT_AUTHENTICATED", "User not authenticated")
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
			utils.ReturnErrorResponse(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
			return
		}
		utils.ReturnErrorResponse(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Error querying user")
		return
	}

	utils.ReturnSuccessResponse(w, http.StatusOK, "Profile retrieved successfully", profile)
}

func TransferHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodPost {
		utils.ReturnErrorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	// Get user ID from headers (set by AuthMiddleware)
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		utils.ReturnErrorResponse(w, http.StatusUnauthorized, "USER_NOT_AUTHENTICATED", "User not authenticated")
		return
	}

	// Parse request body for transfer details
	var requestData struct {
		RecipientID int64   `json:"recipient_id"`
		Amount      float64 `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		utils.ReturnErrorResponse(w, http.StatusBadRequest, "BAD_REQUEST", "Error decoding request body")
		return
	}

	// Validate transfer details
	if requestData.RecipientID <= 0 || requestData.Amount <= 0 {
		utils.ReturnErrorResponse(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid recipient ID or amount")
		return
	}

	//check if the recipient exists
	var recipientExists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", requestData.RecipientID).Scan(&recipientExists)
	if err != nil {
		utils.ReturnErrorResponse(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Error querying recipient")
		return
	}
	if !recipientExists {
		utils.ReturnErrorResponse(w, http.StatusNotFound, "RECIPIENT_NOT_FOUND", "Recipient not found")
		return
	}

	// Perform the transfer in a transaction
	tx, err := db.Begin()
	//check if the user has enough balance
	var senderBalance float64
	err = tx.QueryRow("SELECT balance FROM users WHERE id = $1", userID).Scan(&senderBalance)
	if err != nil {
		utils.ReturnErrorResponse(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Error querying sender balance")
		return
	}
	if senderBalance < requestData.Amount {
		utils.ReturnErrorResponse(w, http.StatusBadRequest, "INSUFFICIENT_BALANCE", "Insufficient balance")
		return
	}

	// Deduct amount from sender
	_, err = tx.Exec("UPDATE users SET balance = balance - $1 WHERE id = $2", requestData.Amount, userID)
	if err != nil {
		tx.Rollback()
		utils.ReturnErrorResponse(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Error updating sender balance")
		return
	}
	// Add amount to recipient
	_, err = tx.Exec("UPDATE users SET balance = balance + $1 WHERE id = $2", requestData.Amount, requestData.RecipientID)
	if err != nil {
		tx.Rollback()
		utils.ReturnErrorResponse(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Error updating recipient balance")
		return
	}

	//Save the transaction in the transactions table
	_, err = tx.Exec("INSERT INTO transactions (sender_id, recipient_id, amount) VALUES ($1, $2, $3)", userID, requestData.RecipientID, requestData.Amount)
	if err != nil {
		tx.Rollback()
		utils.ReturnErrorResponse(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Error saving transaction")
		return
	}
	// Commit the transaction
	if err = tx.Commit(); err != nil {
		utils.ReturnErrorResponse(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Error committing transaction")
		return
	}

	utils.ReturnSuccessResponse(w, http.StatusOK, "Transfer successful", map[string]any{
		"sender_id":    userID,
		"recipient_id": requestData.RecipientID,
		"amount":       requestData.Amount,
	})
}

func LogoutHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodPost {
		utils.ReturnErrorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	// Get user ID from headers (set by AuthMiddleware)
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		utils.ReturnErrorResponse(w, http.StatusUnauthorized, "USER_NOT_AUTHENTICATED", "User not authenticated")
		return
	}

	// Add the token to the blacklist in the database
	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")
	//get the expiration time of the token
	expiration := time.Now().Add(24 * time.Hour) // Assuming the token is valid for 24 hours
	_, err := db.Exec("INSERT INTO tokens (token, user_id, expiration) VALUES ($1, $2, $3)", token, userID, expiration)
	if err != nil {
		utils.ReturnErrorResponse(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Error blacklisting token")
		return
	}

	utils.ReturnSuccessResponse(w, http.StatusOK, "Logout successful", map[string]string{
		"status": "ok",
	})
}

func UserHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodGet {
		utils.ReturnErrorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	// Check if the request containst an id or a nickname in the query parameters
	requestedUserID := r.URL.Query().Get("id")
	requestedNickname := r.URL.Query().Get("nickname")

	// Get requested user ID from query parameters
	if requestedUserID == "" && requestedNickname == "" {
		utils.ReturnErrorResponse(w, http.StatusBadRequest, "MISSING_USER_ID_OR_NICKNAME", "Missing user ID or nickname in query parameters")
		return
	}

	var user struct {
		ID       int64  `json:"id"`
		Nickname string `json:"nickname"`
	}

	// lookup requested user data from the database
	var err error
	if requestedUserID != "" {
		err = db.QueryRow("SELECT id, login FROM users WHERE id = $1", requestedUserID).Scan(&user.ID, &user.Nickname)
	} else {
		err = db.QueryRow("SELECT id, login FROM users WHERE login = $1", requestedNickname).Scan(&user.ID, &user.Nickname)
	}
	if err != nil {
		if err == sql.ErrNoRows {
			utils.ReturnErrorResponse(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
			return
		}
		utils.ReturnErrorResponse(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Error querying user")
		return
	}

	utils.ReturnSuccessResponse(w, http.StatusOK, "User retrieved successfully", user)
}

func TransactionsHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodGet {
		utils.ReturnErrorResponse(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	//get the limit and offset from the query parameters
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))

	if err != nil {
		utils.ReturnErrorResponse(w, http.StatusBadRequest, "INVALID_LIMIT_PARAMETER", "Invalid limit parameter")
		return
	}

	offset, err := strconv.Atoi(r.URL.Query().Get("offset"))

	if err != nil {
		utils.ReturnErrorResponse(w, http.StatusBadRequest, "INVALID_OFFSET_PARAMETER", "Invalid offset parameter")
	}

	if limit < 1 || limit > 50 || offset < 0 {
		utils.ReturnErrorResponse(w, http.StatusBadRequest, "INVALID_PAGINATION", "Limit must be between 1 and 50, and offset must be non-negative")
		return
	}

	// Get user ID from headers (set by AuthMiddleware)
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		utils.ReturnErrorResponse(w, http.StatusUnauthorized, "USER_NOT_AUTHENTICATED", "User not authenticated")
		return
	}

	// Query transactions for the user
	rows, err := db.Query("SELECT id, sender_id, recipient_id, amount, created_at FROM transactions WHERE sender_id = $1 OR recipient_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3", userID, limit, offset)
	if err != nil {
		utils.ReturnErrorResponse(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Error querying transactions")
		return
	}
	defer rows.Close()

	var transactions []struct {
		ID          int64     `json:"id"`
		SenderID    int64     `json:"sender_id"`
		RecipientID int64     `json:"recipient_id"`
		Amount      float64   `json:"amount"`
		CreatedAt   time.Time `json:"created_at"`
	}
	for rows.Next() {
		var t struct {
			ID          int64     `json:"id"`
			SenderID    int64     `json:"sender_id"`
			RecipientID int64     `json:"recipient_id"`
			Amount      float64   `json:"amount"`
			CreatedAt   time.Time `json:"created_at"`
		}
		if err := rows.Scan(&t.ID, &t.SenderID, &t.RecipientID, &t.Amount, &t.CreatedAt); err != nil {
			utils.ReturnErrorResponse(w, http.StatusInternalServerError, "ERROR_SCANNING_TRANSACTION", "Error scanning transaction")
			return
		}
		if rows.Err() != nil {
			utils.ReturnErrorResponse(w, http.StatusInternalServerError, "ERROR_ITERATING_TRANSACTIONS", "Error iterating over transactions")
			return
		}
		transactions = append(transactions, t)
	}
	//check if null transactions and return 404
	if len(transactions) == 0 {
		utils.ReturnErrorResponse(w, http.StatusNotFound, "NO_TRANSACTIONS_FOUND", "No transactions found")
		return
	}

	utils.ReturnSuccessResponse(w, http.StatusOK, "Transactions retrieved successfully", transactions)
}
