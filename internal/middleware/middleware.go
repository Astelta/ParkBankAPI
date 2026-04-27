package middleware

import (
	"net/http"

	"astelta-api-v2/internal/utils"
)

// Add CORS headers to allow cross-origin requests from any domain
func CorsMiddleware(next http.HandlerFunc) http.HandlerFunc {
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

// AuthMiddleware validates JWE tokens and rejects invalid requests
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get token from Authorization header (Bearer token)
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// Extract token from "Bearer <token>" format
		token := authHeader
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			token = authHeader[7:]
		}

		// Validate the JWE token
		payload, err := utils.ValidateAuthToken(token)
		if err != nil {
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Store user info in request context for handlers to use
		r.Header.Set("X-User-ID", string(rune(payload.UserID)))
		r.Header.Set("X-Username", payload.Username)

		next(w, r)
	}
}
