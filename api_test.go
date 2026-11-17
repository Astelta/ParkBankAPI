package main

import (
	"astelta-api-v2/internal/handlers"
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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

// doJSONRequest wysyła żądanie HTTP do routera i wymusza poprawny nagłówek oraz składnię JSON w odpowiedzi.
func doJSONRequest(t *testing.T, router http.Handler, method, target, token string, payload any) *httptest.ResponseRecorder {
	t.Helper()

	var bodyReader io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("failed to marshal request payload for %s %s: %v", method, target, err)
		}
		bodyReader = bytes.NewReader(raw)
	}

	req := httptest.NewRequest(method, target, bodyReader)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("%s %s: expected Content-Type application/json, got %q", method, target, ct)
	}

	if !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("%s %s: response body is not valid JSON: %s", method, target, rec.Body.String())
	}

	return rec
}

// extractToken wyciąga token JWE z odpowiedzi /login niezależnie od tego, czy znajduje się wewnątrz pola "data".
func extractToken(t *testing.T, body []byte) string {
	t.Helper()

	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		t.Fatalf("failed to decode login response: %v; body: %s", err, string(body))
	}

	if dataMap, ok := root["data"].(map[string]any); ok {
		if tok, ok := dataMap["token"].(string); ok && tok != "" {
			return tok
		}
	}
	if tok, ok := root["data"].(string); ok && tok != "" {
		return tok
	}
	if tok, ok := root["token"].(string); ok && tok != "" {
		return tok
	}

	t.Fatalf("token not found in /login response: %s", string(body))
	return ""
}

func TestAllEndpoints(t *testing.T) {
	db := setupTestDB(t)
	router := handlers.Router(db)

	user1 := map[string]string{"nickname": "alice", "password": "strongpassword123"}
	user2 := map[string]string{"nickname": "bob", "password": "strongpassword123"}

	var token1 string

	// 1. GET /health
	t.Run("GET /health", func(t *testing.T) {
		rec := doJSONRequest(t, router, http.MethodGet, "/health", "", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
		}
	})

	// 2. GET /metrics
	t.Run("GET /metrics", func(t *testing.T) {
		rec := doJSONRequest(t, router, http.MethodGet, "/metrics", "", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
		}
	})

	// 3. POST /register (rejestracja dwóch kont + próba duplikatu)
	t.Run("POST /register", func(t *testing.T) {
		rec1 := doJSONRequest(t, router, http.MethodPost, "/register", "", user1)
		if rec1.Code != http.StatusCreated && rec1.Code != http.StatusOK {
			t.Fatalf("user1 register: expected 200/201, got %d; body: %s", rec1.Code, rec1.Body.String())
		}

		rec2 := doJSONRequest(t, router, http.MethodPost, "/register", "", user2)
		if rec2.Code != http.StatusCreated && rec2.Code != http.StatusOK {
			t.Fatalf("user2 register: expected 200/201, got %d; body: %s", rec2.Code, rec2.Body.String())
		}

		recDup := doJSONRequest(t, router, http.MethodPost, "/register", "", user1)
		if recDup.Code != http.StatusConflict && recDup.Code != http.StatusBadRequest {
			t.Fatalf("duplicate register: expected 400/409, got %d; body: %s", recDup.Code, recDup.Body.String())
		}
	})

	// 4. POST /login (poprawne logowanie + odrzucenie błędnego hasła)
	t.Run("POST /login", func(t *testing.T) {
		recBad := doJSONRequest(t, router, http.MethodPost, "/login", "", map[string]string{
			"nickname": user1["nickname"],
			"password": "wrongpassword",
		})
		if recBad.Code != http.StatusUnauthorized {
			t.Fatalf("bad password: expected 401, got %d; body: %s", recBad.Code, recBad.Body.String())
		}

		recOK := doJSONRequest(t, router, http.MethodPost, "/login", "", user1)
		if recOK.Code != http.StatusOK {
			t.Fatalf("valid login: expected 200, got %d; body: %s", recOK.Code, recOK.Body.String())
		}
		token1 = extractToken(t, recOK.Body.Bytes())
	})

	// 5. GET /profile (brak tokena vs poprawny token)
	t.Run("GET /profile", func(t *testing.T) {
		recUnauth := doJSONRequest(t, router, http.MethodGet, "/profile", "", nil)
		if recUnauth.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated: expected 401, got %d; body: %s", recUnauth.Code, recUnauth.Body.String())
		}

		recOK := doJSONRequest(t, router, http.MethodGet, "/profile", token1, nil)
		if recOK.Code != http.StatusOK {
			t.Fatalf("authenticated: expected 200, got %d; body: %s", recOK.Code, recOK.Body.String())
		}
	})

	// 6. GET /user (wyszukiwanie po id oraz po nickname)
	t.Run("GET /user", func(t *testing.T) {
		recByID := doJSONRequest(t, router, http.MethodGet, "/user?id=1", token1, nil)
		if recByID.Code != http.StatusOK {
			t.Fatalf("by id: expected 200, got %d; body: %s", recByID.Code, recByID.Body.String())
		}

		recByNick := doJSONRequest(t, router, http.MethodGet, "/user?nickname=bob", token1, nil)
		if recByNick.Code != http.StatusOK {
			t.Fatalf("by nickname: expected 200, got %d; body: %s", recByNick.Code, recByNick.Body.String())
		}
	})

	// 7. POST /transfer (przelew środków + weryfikacja sald w DB + brak środków)
	t.Run("POST /transfer", func(t *testing.T) {
		// Ustawienie początkowego salda 1000 dla użytkownika o ID 1
		if _, err := db.Exec(`UPDATE users SET balance = 1000 WHERE id = 1`); err != nil {
			t.Fatalf("failed to seed balance: %v", err)
		}

		recOK := doJSONRequest(t, router, http.MethodPost, "/transfer", token1, map[string]any{
			"recipient_id": 2,
			"amount":       250,
		})
		if recOK.Code != http.StatusOK {
			t.Fatalf("valid transfer: expected 200, got %d; body: %s", recOK.Code, recOK.Body.String())
		}

		var senderBal, recipientBal int64
		if err := db.QueryRow(`SELECT balance FROM users WHERE id = 1`).Scan(&senderBal); err != nil {
			t.Fatalf("failed to query sender balance: %v", err)
		}
		if err := db.QueryRow(`SELECT balance FROM users WHERE id = 2`).Scan(&recipientBal); err != nil {
			t.Fatalf("failed to query recipient balance: %v", err)
		}
		if senderBal != 750 || recipientBal != 250 {
			t.Fatalf("unexpected balances after transfer: sender=%d (want 750), recipient=%d (want 250)", senderBal, recipientBal)
		}

		// Próba przelania kwoty większej niż dostępne saldo
		recInsufficient := doJSONRequest(t, router, http.MethodPost, "/transfer", token1, map[string]any{
			"recipient_id": 2,
			"amount":       999999,
		})
		if recInsufficient.Code != http.StatusBadRequest {
			t.Fatalf("insufficient funds: expected 400, got %d; body: %s", recInsufficient.Code, recInsufficient.Body.String())
		}
	})

	// 8. GET /transactions (poprawna paginacja + przekroczenie limitu 50)
	t.Run("GET /transactions", func(t *testing.T) {
		recOK := doJSONRequest(t, router, http.MethodGet, "/transactions?limit=10&offset=0", token1, nil)
		if recOK.Code != http.StatusOK {
			t.Fatalf("valid pagination: expected 200, got %d; body: %s", recOK.Code, recOK.Body.String())
		}

		recLimit := doJSONRequest(t, router, http.MethodGet, "/transactions?limit=51&offset=0", token1, nil)
		if recLimit.Code != http.StatusBadRequest {
			t.Fatalf("limit > 50: expected 400, got %d; body: %s", recLimit.Code, recLimit.Body.String())
		}
	})

	// 9. POST /logout (wylogowanie + weryfikacja, że unieważniony token nie przechodzi przez AuthMiddleware)
	t.Run("POST /logout", func(t *testing.T) {
		recLogout := doJSONRequest(t, router, http.MethodPost, "/logout", token1, nil)
		if recLogout.Code != http.StatusOK {
			t.Fatalf("logout: expected 200, got %d; body: %s", recLogout.Code, recLogout.Body.String())
		}

		recAfter := doJSONRequest(t, router, http.MethodGet, "/profile", token1, nil)
		if recAfter.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 when reusing logged-out token, got %d; body: %s", recAfter.Code, recAfter.Body.String())
		}
	})
}
