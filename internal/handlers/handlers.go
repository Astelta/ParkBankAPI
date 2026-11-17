package handlers

import (
	"astelta-api-v2/internal/metrics"
	"astelta-api-v2/internal/utils"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func RegisterHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodPost {
		utils.ReturnErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var requestData struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		utils.ReturnErrorResponse(w, http.StatusBadRequest, "Error decoding request body")
		return
	}

	if requestData.Username == "" || requestData.Password == "" {
		utils.ReturnErrorResponse(w, http.StatusBadRequest, "Username and password are required")
		return
	}
	requestData.Password = utils.HashPassword(requestData.Password)

	_, err := db.Query("INSERT INTO users (login, password) VALUES ($1, $2)", requestData.Username, requestData.Password)
	if err != nil {
		utils.ReturnErrorResponse(w, http.StatusInternalServerError, "Error inserting user")
		return
	}

	utils.ReturnSuccessResponse(w, http.StatusCreated, "User registered successfully", map[string]string{
		"username": requestData.Username,
	})
}

func LoginHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodPost {
		utils.ReturnErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var requestData struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		utils.ReturnErrorResponse(w, http.StatusBadRequest, "Error decoding request body")
		return
	}

	if requestData.Username == "" || requestData.Password == "" {
		utils.ReturnErrorResponse(w, http.StatusBadRequest, "Username and password are required")
		return
	}

	// Query user by username
	var userID int64
	var storedPassword string
	err := db.QueryRow("SELECT id, password FROM users WHERE login = $1", requestData.Username).Scan(&userID, &storedPassword)
	if err != nil {
		if err == sql.ErrNoRows {
			utils.ReturnErrorResponse(w, http.StatusUnauthorized, "Invalid username or password")
			return
		}
		utils.ReturnErrorResponse(w, http.StatusInternalServerError, "Error querying user")
		return
	}

	// Verify password
	if !utils.CheckPasswordHash(requestData.Password, storedPassword) {
		utils.ReturnErrorResponse(w, http.StatusUnauthorized, "Invalid username or password")
		return
	}

	// Create JWE token
	jweToken, err := utils.CreateAuthToken(userID, requestData.Username)
	if err != nil {
		utils.ReturnErrorResponse(w, http.StatusInternalServerError, "Error creating auth token")
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
		utils.ReturnErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	utils.ReturnSuccessResponse(w, http.StatusOK, "API is healthy", map[string]string{
		"status": "ok",
	})
}

// MetricsHandler returns system and application metrics in JSON format
func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.ReturnErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	response := metrics.GetMetrics()

	if err := json.NewEncoder(w).Encode(response); err != nil {
		utils.ReturnErrorResponse(w, http.StatusInternalServerError, "Error encoding metrics response")
		return
	}
	utils.ReturnSuccessResponse(w, http.StatusOK, "Metrics retrieved successfully", response)
}

// ProfileHandler returns the user's profile data (id and balance)
func ProfileHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodGet {
		utils.ReturnErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get user ID from headers (set by AuthMiddleware)
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		utils.ReturnErrorResponse(w, http.StatusUnauthorized, "User not authenticated")
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
			utils.ReturnErrorResponse(w, http.StatusNotFound, "User not found")
			return
		}
		utils.ReturnErrorResponse(w, http.StatusInternalServerError, "Error querying user")
		return
	}

	if err := json.NewEncoder(w).Encode(profile); err != nil {
		utils.ReturnErrorResponse(w, http.StatusInternalServerError, "Error encoding profile response")
		return
	}
	utils.ReturnSuccessResponse(w, http.StatusOK, "Profile retrieved successfully", profile)
}

func TransferHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodPost {
		utils.ReturnErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get user ID from headers (set by AuthMiddleware)
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		utils.ReturnErrorResponse(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	// Parse request body for transfer details
	var requestData struct {
		RecipientID int64   `json:"recipient_id"`
		Amount      float64 `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		utils.ReturnErrorResponse(w, http.StatusBadRequest, "Error decoding request body")
		return
	}

	// Validate transfer details
	if requestData.RecipientID <= 0 || requestData.Amount <= 0 {
		utils.ReturnErrorResponse(w, http.StatusBadRequest, "Invalid recipient ID or amount")
		return
	}

	//check if the recipient exists
	var recipientExists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", requestData.RecipientID).Scan(&recipientExists)
	if err != nil {
		utils.ReturnErrorResponse(w, http.StatusInternalServerError, "Error querying recipient")
		return
	}
	if !recipientExists {
		utils.ReturnErrorResponse(w, http.StatusNotFound, "Recipient not found")
		return
	}

	// Perform the transfer in a transaction
	tx, err := db.Begin()
	//check if the user has enough balance
	var senderBalance float64
	err = tx.QueryRow("SELECT balance FROM users WHERE id = $1", userID).Scan(&senderBalance)
	if err != nil {
		utils.ReturnErrorResponse(w, http.StatusInternalServerError, "Error querying sender balance")
		return
	}
	if senderBalance < requestData.Amount {
		utils.ReturnErrorResponse(w, http.StatusBadRequest, "Insufficient balance")
		return
	}

	// Deduct amount from sender
	_, err = tx.Exec("UPDATE users SET balance = balance - $1 WHERE id = $2", requestData.Amount, userID)
	if err != nil {
		tx.Rollback()
		utils.ReturnErrorResponse(w, http.StatusInternalServerError, "Error updating sender balance")
		return
	}
	// Add amount to recipient
	_, err = tx.Exec("UPDATE users SET balance = balance + $1 WHERE id = $2", requestData.Amount, requestData.RecipientID)
	if err != nil {
		tx.Rollback()
		utils.ReturnErrorResponse(w, http.StatusInternalServerError, "Error updating recipient balance")
		return
	}

	//Save the transaction in the transactions table
	_, err = tx.Exec("INSERT INTO transactions (sender_id, recipient_id, amount) VALUES ($1, $2, $3)", userID, requestData.RecipientID, requestData.Amount)
	if err != nil {
		tx.Rollback()
		utils.ReturnErrorResponse(w, http.StatusInternalServerError, "Error saving transaction")
		return
	}
	// Commit the transaction
	if err = tx.Commit(); err != nil {
		utils.ReturnErrorResponse(w, http.StatusInternalServerError, "Error committing transaction")
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
		utils.ReturnErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get user ID from headers (set by AuthMiddleware)
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		utils.ReturnErrorResponse(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	// Add the token to the blacklist in the database
	authHeader := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeader, "Bearer ")
	//get the expiration time of the token
	expiration := time.Now().Add(24 * time.Hour) // Assuming the token is valid for 24 hours
	_, err := db.Exec("INSERT INTO tokens (token, user_id, expiration) VALUES ($1, $2, $3)", token, userID, expiration)
	if err != nil {
		utils.ReturnErrorResponse(w, http.StatusInternalServerError, "Error blacklisting token")
		return
	}

	utils.ReturnSuccessResponse(w, http.StatusOK, "Logout successful", nil)
}

func UserHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodGet {
		utils.ReturnErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Check if the request containst an id or a nickname in the query parameters
	requestedUserID := r.URL.Query().Get("id")
	requestedNickname := r.URL.Query().Get("nickname")

	// Get requested user ID from query parameters
	if requestedUserID == "" && requestedNickname == "" {
		utils.ReturnErrorResponse(w, http.StatusBadRequest, "Missing user ID or nickname in query parameters")
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
			utils.ReturnErrorResponse(w, http.StatusNotFound, "User not found")
			return
		}
		utils.ReturnErrorResponse(w, http.StatusInternalServerError, "Error querying user")
		return
	}

	utils.ReturnSuccessResponse(w, http.StatusOK, "User retrieved successfully", user)
}

func TransactionsHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodGet {
		utils.ReturnErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	//get the limit and offset from the query parameters
	limit := r.URL.Query().Get("limit")
	//default limit to 10 if not provided
	if limit == "" {
		limit = "10"
	}
	// hard limit to 50 to prevent abuse
	if limitInt, err := strconv.Atoi(limit); err == nil && limitInt > 50 {
		limit = "50"
	}
	offset := r.URL.Query().Get("offset")
	//default offset to 0 if not provided
	if offset == "" {
		offset = "0"
	}

	limitInt, err := strconv.Atoi(limit)
	if err != nil {
		utils.ReturnErrorResponse(w, http.StatusBadRequest, "Invalid limit parameter")
		return
	}
	offsetInt, err := strconv.Atoi(offset)
	if err != nil {
		utils.ReturnErrorResponse(w, http.StatusBadRequest, "Invalid offset parameter")
		return
	}

	// Get user ID from headers (set by AuthMiddleware)
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		utils.ReturnErrorResponse(w, http.StatusUnauthorized, "User not authenticated")
		return
	}

	// Query transactions for the user
	rows, err := db.Query("SELECT id, sender_id, recipient_id, amount, created_at FROM transactions WHERE sender_id = $1 OR recipient_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3", userID, limitInt, offsetInt)
	if err != nil {
		utils.ReturnErrorResponse(w, http.StatusInternalServerError, "Error querying transactions")
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
			utils.ReturnErrorResponse(w, http.StatusInternalServerError, "Error scanning transaction")
			return
		}
		if rows.Err() != nil {
			utils.ReturnErrorResponse(w, http.StatusInternalServerError, "Error iterating over transactions")
			return
		}
		transactions = append(transactions, t)
	}
	//check if null transactions and return 404
	if len(transactions) == 0 {
		utils.ReturnErrorResponse(w, http.StatusNotFound, "No transactions found")
		return
	}

	utils.ReturnSuccessResponse(w, http.StatusOK, "Transactions retrieved successfully", transactions)
}
