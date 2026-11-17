package main

import (
	"astelta-api-v2/internal/handlers"
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	_, err = db.Exec(`TRUNCATE TABLE transactions, tokens, users RESTART IDENTITY CASCADE;`)
	if err != nil {
		t.Fatalf("failed to truncate tables: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	return db
}

func TestHealthAndRegisterJSON(t *testing.T) {
	db := setupTestDB(t)
	router := handlers.Router(db)

	// 1. Test /health
	reqHealth := httptest.NewRequest(http.MethodGet, "/health", nil)
	recHealth := httptest.NewRecorder()
	router.ServeHTTP(recHealth, reqHealth)

	if recHealth.Code != http.StatusOK {
		t.Fatalf("expected 200 on /health, got %d; body: %s", recHealth.Code, recHealth.Body.String())
	}
	if ct := recHealth.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	// 2. Test /register (dopasuj klucze JSON do pól oczekiwanych przez RegisterHandler, np. login/username)
	payload := []byte(`{"nickname":"test_player","password":"strongpassword123"}`)
	reqReg := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(payload))
	reqReg.Header.Set("Content-Type", "application/json")
	recReg := httptest.NewRecorder()
	router.ServeHTTP(recReg, reqReg)

	if recReg.Code != http.StatusCreated && recReg.Code != http.StatusOK {
		t.Fatalf("expected 200/201 on /register, got %d; body: %s", recReg.Code, recReg.Body.String())
	}

	var parsed map[string]any
	if err := json.Unmarshal(recReg.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("response from /register is not valid JSON: %v", err)
	}
}
