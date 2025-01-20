package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"backendGo/auth"

	"github.com/brianvoe/gofakeit/v6"
	_ "github.com/lib/pq"
)

// Database connection
func ConnectDB() *sql.DB {
	connStr := os.Getenv("DB_CONNECTION")
	if connStr == "" {
		connStr = "user=postgres password=admin123 dbname=wiraDB sslmode=disable"
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}

	fmt.Println("Database connected successfully!")
	return db
}

// Create tables
func CreateTables(db *sql.DB) {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS accounts (acc_id BIGSERIAL PRIMARY KEY, username VARCHAR(50) NOT NULL, email VARCHAR(50) NOT NULL, encrypted_password TEXT NOT NULL, secretkey_2fa TEXT, is_email_verified BOOLEAN DEFAULT FALSE)`,
		`CREATE TABLE IF NOT EXISTS characters (char_id BIGSERIAL PRIMARY KEY, acc_id BIGINT REFERENCES accounts(acc_id), class_id SMALLINT)`,
		`CREATE TABLE IF NOT EXISTS scores (score_id BIGSERIAL PRIMARY KEY, char_id BIGINT REFERENCES characters(char_id), reward_score INT)`,
		`CREATE TABLE IF NOT EXISTS sessions (session_id UUID PRIMARY KEY, acc_id BIGINT NOT NULL, metadata TEXT, expiry_datetime TIMESTAMPTZ NOT NULL, FOREIGN KEY (acc_id) REFERENCES accounts(acc_id))`,
		`CREATE TABLE IF NOT EXISTS email_verifications (id BIGSERIAL PRIMARY KEY, acc_id BIGINT NOT NULL REFERENCES accounts(acc_id), verification_token UUID UNIQUE NOT NULL, secret_key_2fa TEXT NOT NULL, created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP)`,
	}

	for _, q := range queries {
		_, err := db.Exec(q)
		if err != nil {
			log.Fatalf("Failed to create table: %v", err)
		}
	}
	fmt.Println("Tables verified or created.")
}

// Populate database
func GenerateDataIfNeeded(db *sql.DB) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM accounts").Scan(&count)
	if err != nil {
		log.Fatalf("Error checking data: %v", err)
	}

	if count > 0 {
		fmt.Println("Data exists. Skipping generation.")
		return
	}

	fmt.Println("Generating fake data...")
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(batch int) {
			defer wg.Done()
			generateFakeData(db, batch*10000, 10000)
		}(i)
	}
	wg.Wait()
	fmt.Println("Fake data generation complete!")
}

// Function to generate the fake data (without hashing passwords)
func generateFakeData(db *sql.DB, start, count int) {
	var accounts []struct {
		Username string
		Email    string
		Password string
		Secret   string
		AccID    uint64
	}

	// Step 1: Generate fake data
	for i := start; i < start+count; i++ {
		username := gofakeit.Username()
		email := gofakeit.Email()
		password := gofakeit.Password(true, true, true, true, false, 12) // Random password

		// Generate 2FA secret
		secret, _, err := auth.Generate2FASecret()
		if err != nil {
			log.Printf("Error generating 2FA secret for account %s: %v", username, err)
			continue
		}

		// Store the account data temporarily
		accounts = append(accounts, struct {
			Username string
			Email    string
			Password string
			Secret   string
			AccID    uint64
		}{
			Username: username,
			Email:    email,
			Password: password,
			Secret:   secret,
		})

		// Insert account (without hashing password yet)
		var accID uint64
		err = db.QueryRow("INSERT INTO accounts (username, email, encrypted_password, secretkey_2fa, is_email_verified) VALUES ($1, $2, '', $3, TRUE) RETURNING acc_id", username, email, secret).Scan(&accID)
		if err != nil {
			log.Printf("Error creating account %s: %v", username, err)
			continue
		}

		// Update AccID for later use
		accounts[len(accounts)-1].AccID = accID

		// Create characters and scores
		for classID := 1; classID <= 8; classID++ {
			var charID uint64
			// Insert character
			err = db.QueryRow("INSERT INTO characters (acc_id, class_id) VALUES ($1, $2) RETURNING char_id", accID, classID).Scan(&charID)
			if err != nil {
				log.Printf("Error creating character for class %d: %v", classID, err)
				continue
			}

			// Generate reward score
			rewardScore := gofakeit.Number(10, 1000)

			// Insert score for character
			_, err = db.Exec("INSERT INTO scores (char_id, reward_score) VALUES ($1, $2)", charID, rewardScore)
			if err != nil {
				log.Printf("Error creating score for class %d: %v", classID, err)
			}
		}
	}

	// Step 2: Call the new function to hash passwords after data generation
	hashPasswords(db, accounts)

	log.Println("Fake data generation and password hashing complete!")
}

// Function to hash passwords
func hashPasswords(db *sql.DB, accounts []struct {
	Username string
	Email    string
	Password string
	Secret   string
	AccID    uint64
}) {
	// Print statement indicating the start of password hashing process
	fmt.Println("Starting password hashing process...")

	// Determine the number of goroutines to use (adjust based on your needs)
	numGoroutines := 4
	chunkSize := len(accounts) / numGoroutines

	var wg sync.WaitGroup
	// Divide the accounts into chunks for concurrency
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			start := i * chunkSize
			end := start + chunkSize
			if i == numGoroutines-1 { // Ensure the last chunk includes all remaining accounts
				end = len(accounts)
			}

			// Process a chunk of accounts
			var values []string
			for _, account := range accounts[start:end] {
				// Hash the password
				hashedPassword, err := auth.HashPassword(account.Password)
				if err != nil {
					log.Printf("Error hashing password for account %s: %v", account.Username, err)
					continue
				}
				// Add to values slice for batch update
				values = append(values, fmt.Sprintf("(%d, '%s')", account.AccID, hashedPassword))
			}

			// Perform a batch update for the chunk
			if len(values) > 0 {
				query := fmt.Sprintf("UPDATE accounts SET encrypted_password = subquery.hashed_password FROM (VALUES %s) AS subquery(acc_id, hashed_password) WHERE accounts.acc_id = subquery.acc_id", strings.Join(values, ","))
				_, err := db.Exec(query)
				if err != nil {
					log.Printf("Error updating hashed passwords: %v", err)
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	// Print statement indicating the completion of the password hashing process
	fmt.Println("Password hashing process completed!")
}
