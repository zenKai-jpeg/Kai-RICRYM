package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
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
		`CREATE TABLE IF NOT EXISTS accounts (acc_id BIGSERIAL PRIMARY KEY, username VARCHAR(50) NOT NULL, email VARCHAR(50) NOT NULL, encrypted_password TEXT NOT NULL, secretkey_2fa TEXT, is_email_verified BOOLEAN DEFAULT FALSE)`, // Added is_email_verified
		`CREATE TABLE IF NOT EXISTS characters (char_id BIGSERIAL PRIMARY KEY, acc_id BIGINT REFERENCES accounts(acc_id), class_id SMALLINT)`,
		`CREATE TABLE IF NOT EXISTS scores (score_id BIGSERIAL PRIMARY KEY, char_id BIGINT REFERENCES characters(char_id), reward_score INT)`,
		`CREATE TABLE IF NOT EXISTS sessions (session_id UUID PRIMARY KEY, acc_id BIGINT NOT NULL, metadata TEXT, expiry_datetime TIMESTAMPTZ NOT NULL, FOREIGN KEY (acc_id) REFERENCES accounts(acc_id))`,
		`CREATE TABLE IF NOT EXISTS email_verifications (
			id BIGSERIAL PRIMARY KEY,
			acc_id BIGINT NOT NULL REFERENCES accounts(acc_id),
			verification_token UUID UNIQUE NOT NULL,
			secret_key_2fa TEXT NOT NULL,
			created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
		)`, // Added email_verifications table
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

// Modify the generateFakeData function
func generateFakeData(db *sql.DB, start, count int) {
	for i := start; i < start+count; i++ {
		username := gofakeit.Username()
		email := gofakeit.Email()
		password := gofakeit.Password(true, true, true, true, false, 12) // Random password

		// Hash the password
		hashedPassword, err := auth.HashPassword(password)
		if err != nil {
			log.Printf("Error hashing password for account %s: %v", username, err)
			continue
		}

		// Log the hashed password for debugging
		log.Printf("Hashed Password for %s: %s", username, hashedPassword)

		// Generate 2FA secret
		secret, _, err := auth.Generate2FASecret()
		if err != nil {
			log.Printf("Error generating 2FA secret for account %s: %v", username, err)
			continue
		}

		// Insert account with username, email, hashed password, and 2FA secret
		var accID uint64
		err = db.QueryRow("INSERT INTO accounts (username, email, encrypted_password, secretkey_2fa, is_email_verified) VALUES ($1, $2, $3, $4, TRUE) RETURNING acc_id", username, email, hashedPassword, secret).Scan(&accID)
		if err != nil {
			log.Printf("Error creating account %s: %v", username, err)
			continue
		}

		// For each account, create 8 classes (one for each class type)
		for classID := 1; classID <= 8; classID++ {
			var charID uint64
			// Insert a character for the current class ID, returning the character ID
			err = db.QueryRow("INSERT INTO characters (acc_id, class_id) VALUES ($1, $2) RETURNING char_id", accID, classID).Scan(&charID)
			if err != nil {
				log.Printf("Error creating character for class %d: %v", classID, err)
				continue
			}

			// Generate a reward score for the class
			rewardScore := gofakeit.Number(10, 1000)

			// Insert the score for the character and class combination
			_, err = db.Exec("INSERT INTO scores (char_id, reward_score) VALUES ($1, $2)", charID, rewardScore)
			if err != nil {
				log.Printf("Error creating score for class %d: %v", classID, err)
			}
		}
	}

	log.Println("Fake data generation complete!")
}
