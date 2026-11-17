package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"astelta-api-v2/internal/utils"
)

// Add CORS headers to allow cross-origin requests from any domain
func CorsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			utils.ReturnSuccessResponse(w, http.StatusOK, "CORS preflight request successful", nil)
			return
		}
		next(w, r)
	}
}

// AuthMiddleware validates JWE tokens and rejects invalid requests
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Pobranie nagłówka
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			utils.ReturnErrorResponse(w, http.StatusUnauthorized, "Missing Authorization header")
			return
		}

		// Bezpieczne i czyste usunięcie prefiksu "Bearer "
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader {
			// Opcjonalnie: odrzucenie żądania, jeśli nagłówek nie zaczynał się od "Bearer "
			utils.ReturnErrorResponse(w, http.StatusUnauthorized, "Invalid Authorization header format")
			return
		}

		// Check if the token is blacklisted
		isBlacklisted, err := utils.IsTokenBlacklisted(token)
		if err != nil {
			utils.ReturnErrorResponse(w, http.StatusInternalServerError, "Error checking token blacklist")
			return
		}
		if isBlacklisted {
			utils.ReturnErrorResponse(w, http.StatusUnauthorized, "Token is blacklisted")
			return
		}

		// Walidacja tokena JWE
		payload, err := utils.ValidateAuthToken(token)
		if err != nil {
			utils.ReturnErrorResponse(w, http.StatusUnauthorized, "Invalid or expired token")
			return
		}

		// Prawidłowa konwersja int64 na string z bazą 10 (system dziesiętny)
		r.Header.Set("X-User-ID", strconv.FormatInt(payload.UserID, 10))
		r.Header.Set("X-Username", payload.Username)

		next(w, r)
	}
}
