package handlers

// Utility functions for environment variable loading and retrieval
import (
	"astelta-api-v2/internal/middleware"
	"database/sql"
	"net/http"
)

func Router(db *sql.DB) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", middleware.CorsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		HealthHandler(w, r)
	}))
	mux.HandleFunc("/metrics", middleware.CorsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		MetricsHandler(w, r, db)
	}))
	mux.HandleFunc("/register", middleware.CorsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		RegisterHandler(w, r, db)
	}))
	mux.HandleFunc("/login", middleware.CorsMiddleware(func(w http.ResponseWriter, r *http.Request) {
		LoginHandler(w, r, db)
	}))
	mux.HandleFunc("/profile", middleware.CorsMiddleware(middleware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		ProfileHandler(w, r, db)
	})))
	mux.HandleFunc("/transfer", middleware.CorsMiddleware(middleware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		TransferHandler(w, r, db)
	})))
	mux.HandleFunc("/logout", middleware.CorsMiddleware(middleware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		LogoutHandler(w, r, db)
	})))
	mux.HandleFunc("/user", middleware.CorsMiddleware(middleware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		UserHandler(w, r, db)
	})))
	mux.HandleFunc("/transactions", middleware.CorsMiddleware(middleware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		TransactionsHandler(w, r, db)
	})))

	return mux
}
