package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/brianvoe/gofakeit/v6"
	_ "github.com/lib/pq"
)

type Account struct {
	acc_id   uint64
	userName string
	email    string
}

type character struct {
	char_id  uint64
	acc_id   uint64
	class_id uint8
}

type scores struct {
	score_id     uint64
	char_id      uint64
	reward_score uint16
}

func main() {
	// Connect to the database
	db := connectDB()

	// Defer closing the connection until all database operations are complete
	defer db.Close()

	// Create tables if they don't exist
	createTables(db)

	// Create an error channel for goroutines to report errors
	errCh := make(chan error, 10)

	// Check if data already exists in the 'accounts' table
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM accounts").Scan(&count)
	if err != nil {
		log.Fatalf("Error counting accounts: %v", err)
	}

	if count > 0 {
		fmt.Println("Data exists, skipping generation.")
		return
	}

	// Start a goroutine to listen for errors
	go func() {
		for err := range errCh {
			fmt.Println("Error:", err)
		}
	}()

	// Generate fake data with concurrency
	var wg sync.WaitGroup
	numWorkers := 10
	batchSize := 10000

	// Create worker goroutines
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(start int) {
			defer wg.Done()
			generateFakeData(db, start, batchSize, errCh) // Pass the errCh to worker
		}(i * batchSize)
	}

	// Wait for all workers to finish
	wg.Wait()

	// Close the error channel after all workers finish
	close(errCh)

	// Indicate that data creation is complete
	fmt.Println("Data creation complete.")

	// Set up the HTTP server
	http.HandleFunc("/", handler)
	fmt.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Println("Error starting server:", err)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, World!")
}

func connectDB() *sql.DB {
	// Connection string
	connStr := "user=postgres password=admin123 dbname=wiraDB sslmode=disable"

	// Open a connection to the database
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Error opening database connection: %v", err)
	}

	// Ensure the database connection is working
	if err := db.Ping(); err != nil {
		log.Fatalf("Could not connect to the database: %v", err)
	}
	fmt.Println("Database connection successful!")

	// Test connection with a query
	var testValue int
	err = db.QueryRow("SELECT 1").Scan(&testValue)
	if err != nil {
		log.Fatalf("Database query test failed: %v", err)
	}
	fmt.Printf("Test query successful, received value: %d\n", testValue)

	// Don't defer db.Close() here, let it be handled in main() instead
	return db
}

func createTables(db *sql.DB) {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS accounts (
			acc_id SERIAL PRIMARY KEY,
			userName VARCHAR(50) NOT NULL,
			email VARCHAR(50) NOT NULL UNIQUE
		)`,
		`CREATE TABLE IF NOT EXISTS characters (
			char_id SERIAL PRIMARY KEY,
			acc_id INTEGER NOT NULL,
			class_id SMALLINT NOT NULL,
			FOREIGN KEY (acc_id) REFERENCES accounts(acc_id)
		)`,
		`CREATE TABLE IF NOT EXISTS scores (
			score_id SERIAL PRIMARY KEY,
			char_id INTEGER NOT NULL,
			reward_score BIGINT NOT NULL,
			FOREIGN KEY (char_id) REFERENCES characters(char_id)
		)`,
	}

	// Execute each query to create the tables
	for _, query := range queries {
		_, err := db.Exec(query)
		if err != nil {
			log.Fatalf("Error creating table: %v", err)
		}
	}
	fmt.Println("Tables created or already exist.")
}

func generateFakeData(db *sql.DB, start int, batchSize int, errCh chan error) {
	// Mutex for synchronized printing
	var mu sync.Mutex
	// Progress counter
	var count int

	for i := start; i < start+batchSize; i++ {
		userName := gofakeit.Username()
		email := gofakeit.Email()
		var accID uint64
		err := db.QueryRow("INSERT INTO accounts (userName, email) VALUES ($1, $2) RETURNING acc_id", userName, email).Scan(&accID)
		if err != nil {
			errCh <- fmt.Errorf("error inserting fake account: %v", err)
			continue
		}

		// Increment counter for accounts processed
		count++

		// Periodically print progress (every 10000 accounts)
		if count%10000 == 0 {
			mu.Lock() // Lock to ensure safe printing
			fmt.Printf("Progress: %d accounts processed (Batch starting at %d)\n", count, start)
			mu.Unlock()
		}

		// Insert 4 characters per account
		for j := 0; j < 4; j++ { // Loop for 4 characters per account
			classID := uint8(gofakeit.Number(1, 8)) // Class ID from 1 to 8
			var charID uint64
			err = db.QueryRow("INSERT INTO characters (acc_id, class_id) VALUES ($1, $2) RETURNING char_id", accID, classID).Scan(&charID)
			if err != nil {
				errCh <- fmt.Errorf("error inserting fake character: %v", err)
				continue
			}

			// Insert a single score per character
			rewardScore := uint16(gofakeit.Number(0, 9999)) // Assuming reward_score ranges from 0 to 9999
			_, err = db.Exec("INSERT INTO scores (char_id, reward_score) VALUES ($1, $2)", charID, rewardScore)
			if err != nil {
				errCh <- fmt.Errorf("error inserting fake score: %v", err)
				continue
			}
		}
	}

	// Print completion message for the batch
	fmt.Printf("Fake data generation complete for batch starting at %d\n", start)
}
