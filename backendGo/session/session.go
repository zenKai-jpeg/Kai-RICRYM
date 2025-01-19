package session

import (
	"database/sql"
	"log"
	"time"

	"github.com/brianvoe/gofakeit/v6"
)

// Generate random sessions for a given account
func GenerateRandomSessions(db *sql.DB, accountID uint64) {
	// Generate a new random session ID and metadata
	sessionID := gofakeit.UUID()                                                           // Random UUID for session ID
	metadata := gofakeit.IPv4Address()                                                     // Random IP address for metadata
	expiryDateTime := time.Now().Add(time.Minute * time.Duration(gofakeit.Number(10, 60))) // Random expiry time between 10 to 60 minutes

	// Check if a session already exists for this account
	var existingSessionID string
	err := db.QueryRow("SELECT session_id FROM sessions WHERE acc_id = $1", accountID).Scan(&existingSessionID)

	if err != nil && err != sql.ErrNoRows {
		log.Printf("Error checking for existing session: %v", err)
		return
	}

	if existingSessionID != "" {
		// If session exists, update the session with a new session ID and metadata
		_, err = db.Exec("UPDATE sessions SET session_id = $1, metadata = $2, expiry_datetime = $3 WHERE acc_id = $4", sessionID, metadata, expiryDateTime, accountID)
		if err != nil {
			log.Printf("Error updating session for account %d: %v", accountID, err)
			return
		}
		log.Printf("Session for account %d updated with new session ID", accountID)
	} else {
		// If no session exists, insert a new session
		_, err = db.Exec("INSERT INTO sessions (session_id, acc_id, metadata, expiry_datetime) VALUES ($1, $2, $3, $4)", sessionID, accountID, metadata, expiryDateTime)
		if err != nil {
			log.Printf("Error creating session for account %d: %v", accountID, err)
			return
		}
		log.Printf("New session generated for account %d", accountID)
	}
}

func GenerateSessionsForAllAccounts(db *sql.DB) {
	log.Printf("Generating sessions for all test accounts")

	// Check the current number of sessions
	var sessionCount int
	err := db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&sessionCount)
	if err != nil {
		log.Printf("Error fetching session count: %v", err)
		return
	}

	// Skip session generation if there are already 100,000 sessions
	if sessionCount >= 100000 {
		log.Printf("Session generation skipped, already %d test session made.", sessionCount)
		return
	}

	// Get the total account count
	var accountCount int
	err = db.QueryRow("SELECT COUNT(*) FROM accounts").Scan(&accountCount)
	if err != nil {
		log.Printf("Error fetching account count: %v", err)
		return
	}

	// Fetch all accounts at once
	rows, err := db.Query("SELECT acc_id FROM accounts LIMIT $1", accountCount)
	if err != nil {
		log.Printf("Error fetching accounts: %v", err)
		return
	}
	defer rows.Close()

	// Iterate through all the fetched accounts
	for rows.Next() {
		var accID uint64
		if err := rows.Scan(&accID); err != nil {
			log.Printf("Error scanning account ID: %v", err)
			continue
		}

		// Generate random sessions for the account
		GenerateRandomSessions(db, accID)
	}

	// Check for errors from iterating over rows
	if err := rows.Err(); err != nil {
		log.Printf("Error iterating over accounts: %v", err)
	}
}
