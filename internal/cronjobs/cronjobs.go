package cronjobs

import (
	"database/sql"
	"time"
)

func StartAllCronJobs(db *sql.DB) {
	//start a goroutine to run any cron job every x hours
	go func() {
		for {
			cleanExpiredTokens(db)
			// Sleep for 12 hours before running the cron job again
			time.Sleep(12 * time.Hour)
		}
	}()
}
