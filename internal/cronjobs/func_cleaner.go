package cronjobs

import (
	"database/sql"
	"log"
	"time"
)

func cleanExpiredTokens(db *sql.DB) {
	_, err := db.Exec("DELETE FROM tokens WHERE expiration < $1", time.Now())
	if err != nil {
		log.Println("Error occurred while cleaning expired tokens:", err)
	}
}
