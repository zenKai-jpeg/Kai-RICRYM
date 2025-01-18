package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"

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
	reward_score uint64
}

func main() {
	// Connect to the database
	db := connectDB()

	// Defer closing the connection until all database operations are complete
	defer db.Close()

	// Create tables if they don't exist
	createTables(db)

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
