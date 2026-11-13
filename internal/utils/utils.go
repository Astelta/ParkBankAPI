package utils

// Utility functions for environment variable loading and retrieval
import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// very useful function to get env variable with default value :)
func GetEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// Extension of GetEnv to load variables from a .env file if it exists
func LoadDotEnv(path string) {
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

// Tworzy bezpieczny hash z hasła (to zapisujesz w bazie)
func HashPassword(password string) string {
	// Koszt 14 to obecnie dobry kompromis między bezpieczeństwem a wydajnością serwera
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	if err != nil {
		return err.Error()
	}
	return string(bytes)
}

// CheckPasswordHash compares a password with a hash
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateToken generates a random token
func GenerateToken() string {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(b)
}

// JWE represents a JSON Web Encryption structure
type JWE struct {
	Alg string `json:"alg"` // AES-GCM
	Enc string `json:"enc"` // encryption algorithm
	IV  string `json:"iv"`  // base64 encoded IV
	CT  string `json:"ct"`  // base64 encoded ciphertext
}

// GetEncryptionKey returns the AES key from environment or generates a default
func GetEncryptionKey() []byte {
	keyStr := GetEnv("JWE_SECRET_KEY", "")
	if keyStr != "" {
		key, _ := base64.StdEncoding.DecodeString(keyStr)
		if len(key) == 32 {
			return key
		}
	}

	newError := "JWE_SECRET_KEY must be a base64-encoded 32-byte key for AES-256 encryption"
	panic(newError)
}

// TokenPayload represents the data stored in the JWE token
type TokenPayload struct {
	UserID     int64  `json:"user_id"`
	Username   string `json:"username"`
	Token      string `json:"token"`
	Expiration int64  `json:"expiration"`
}

// EncryptJWE encrypts data using AES-GCM and returns a compact token string
func EncryptJWE(data interface{}) (string, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	key := GetEncryptionKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// gcm.Seal szyfruje dane i dokleja tag autoryzacyjny.
	// Przekazujemy nil jako pierwszy argument, aby nie nadpisywać nonce'a.
	ciphertext := gcm.Seal(nil, nonce, jsonData, nil)

	// Tworzymy trzy segmenty zakodowane w URL-safe Base64
	header := `{"alg":"AES-GCM","enc":"A256GCM"}`
	b64Header := base64.RawURLEncoding.EncodeToString([]byte(header))
	b64IV := base64.RawURLEncoding.EncodeToString(nonce)
	b64CT := base64.RawURLEncoding.EncodeToString(ciphertext)

	// Łączymy w kompaktowy token
	return b64Header + "." + b64IV + "." + b64CT, nil
}

// DecryptJWE decrypts a compact token string and unmarshals the data
func DecryptJWE(token string, dest interface{}) error {
	// Rozbijamy token po kropkach
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return errors.New("invalid token format")
	}

	// Dekodujemy IV (wektor inicjalizujący)
	nonce, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return err
	}

	// Dekodujemy szyfrogram
	ciphertext, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return err
	}

	key := GetEncryptionKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	// Odszyfrowujemy z weryfikacją tagu autoryzacyjnego
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return err
	}

	return json.Unmarshal(plaintext, dest)
}

// CreateAuthToken creates a JWE token for authenticated users
func CreateAuthToken(userID int64, username string) (string, error) {
	timestamp := time.Now().Add(24 * time.Hour).Unix()
	token := GenerateToken()

	payload := TokenPayload{
		UserID:     userID,
		Username:   username,
		Token:      token,
		Expiration: timestamp,
	}

	return EncryptJWE(payload)
}

// ValidateAuthToken validates a JWE token and returns the payload
func ValidateAuthToken(token string) (*TokenPayload, error) {
	var payload TokenPayload

	err := DecryptJWE(token, &payload)
	if err != nil {
		return nil, err
	}

	if time.Now().Unix() > payload.Expiration {
		return nil, errors.New("token expired")
	}

	return &payload, nil
}

func IsTokenBlacklisted(token string) (bool, error) {
	// Connect to the database
	db, err := sql.Open("postgres", GetEnv("DATABASE_URL", ""))
	if err != nil {
		return false, err
	}
	defer db.Close()

	// Query the database to check if the token is blacklisted
	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM tokens WHERE token = $1)", token).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}
