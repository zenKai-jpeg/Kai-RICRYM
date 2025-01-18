package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"sync"

	"github.com/brianvoe/gofakeit/v6"
	_ "github.com/lib/pq"
)

type Account struct {
	AccID    uint64 `json:"AccID"`
	UserName string `json:"Username"`
	Email    string `json:"Email"`
}

type Character struct {
	CharID  uint64
	AccID   uint64
	ClassID uint8
}

type Scores struct {
	ScoreID     uint64
	CharID      uint64
	RewardScore uint32
}

type AccountWithClassAndScore struct {
	AccID    uint64 `json:"AccID"`
	UserName string `json:"Username"`
	Email    string `json:"Email"`
	ClassID  int    `json:"ClassID"`
	Score    int    `json:"Score"`
	Rank     int    `json:"Rank"`
}

func main() {
	// Connect to the database
	db := connectDB()

	// Defer closing the connection until all database operations are complete
	defer db.Close()

	// Create tables if they don't exist
	createTables(db)

	// Generate fake data if the database is empty
	generateDataIfNeeded(db)

	// Set up the HTTP server with routes
	http.HandleFunc("/", handler)
	http.HandleFunc("/accounts", func(w http.ResponseWriter, r *http.Request) {
		paginatedHandler(w, r, db)
	})

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

	return db
}

func createTables(db *sql.DB) {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS accounts (
			acc_id BIGSERIAL PRIMARY KEY,           
			username VARCHAR(50) NOT NULL,
			email VARCHAR(50) NOT NULL UNIQUE
		)`,
		`CREATE TABLE IF NOT EXISTS characters (
			char_id BIGSERIAL PRIMARY KEY,          
			acc_id BIGINT NOT NULL,                 
			class_id SMALLINT NOT NULL,
			FOREIGN KEY (acc_id) REFERENCES accounts(acc_id)
		)`,
		`CREATE TABLE IF NOT EXISTS scores (
			score_id BIGSERIAL PRIMARY KEY,         
			char_id BIGINT NOT NULL,                
			reward_score INT NOT NULL,              
			FOREIGN KEY (char_id) REFERENCES characters(char_id)
		)`,
	}

	for _, query := range queries {
		_, err := db.Exec(query)
		if err != nil {
			log.Fatalf("Error creating table: %v", err)
		}
	}
	fmt.Println("Tables created or already exist.")
}

func generateDataIfNeeded(db *sql.DB) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM accounts").Scan(&count)
	if err != nil {
		log.Fatalf("Error counting accounts: %v", err)
	}

	if count > 0 {
		fmt.Println("Data already exists, skipping generation.")
		return
	}

	fmt.Println("Starting data generation...")

	// Create an error channel for goroutines to report errors
	errCh := make(chan error, 10)
	defer close(errCh)

	// Start a goroutine to listen for errors
	go func() {
		for err := range errCh {
			log.Println("Error:", err)
		}
	}()

	// Generate fake data with concurrency
	var wg sync.WaitGroup
	numWorkers := 10
	batchSize := 10000

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(start int) {
			defer wg.Done()
			generateFakeData(db, start, batchSize, errCh)
		}(i * batchSize)
	}

	wg.Wait()
	fmt.Println("Data generation complete.")
}

func generateFakeData(db *sql.DB, start int, batchSize int, errCh chan error) {
	var mu sync.Mutex
	var count int

	for i := start; i < start+batchSize; i++ {
		userName := gofakeit.Username()
		email := gofakeit.Email()
		var accID uint64
		err := db.QueryRow("INSERT INTO accounts (username, email) VALUES ($1, $2) RETURNING acc_id", userName, email).Scan(&accID)
		if err != nil {
			errCh <- fmt.Errorf("error inserting fake account: %v", err)
			continue
		}

		count++

		if count%1000 == 0 {
			mu.Lock()
			fmt.Printf("Progress: %d accounts processed (Batch starting at %d)\n", count, start)
			mu.Unlock()
		}

		for j := 0; j < 4; j++ {
			classID := uint8(gofakeit.Number(1, 8))
			var charID uint64
			err = db.QueryRow("INSERT INTO characters (acc_id, class_id) VALUES ($1, $2) RETURNING char_id", accID, classID).Scan(&charID)
			if err != nil {
				errCh <- fmt.Errorf("error inserting fake character: %v", err)
				continue
			}

			rewardScore := uint32(gofakeit.Number(0, 99999))
			_, err = db.Exec("INSERT INTO scores (char_id, reward_score) VALUES ($1, $2)", charID, rewardScore)
			if err != nil {
				errCh <- fmt.Errorf("error inserting fake score: %v", err)
			}
		}
	}
	fmt.Printf("Fake data generation complete for batch starting at %d\n", start)
}

func paginatedHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")
	search := r.URL.Query().Get("search")
	sort := r.URL.Query().Get("sort")
	order := r.URL.Query().Get("order")

	fmt.Printf("[DEBUG] Query Params - page: %s, limit: %s, search: %s, sort: %s, order: %s\n", pageStr, limitStr, search, sort, order)

	// Validate and parse `page`
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		http.Error(w, "Invalid 'page' parameter: must be a positive integer", http.StatusBadRequest)
		return
	}

	// Validate and parse `limit`
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		http.Error(w, "Invalid 'limit' parameter: must be a positive integer between 1 and 100", http.StatusBadRequest)
		return
	}

	// Fetch paginated accounts
	accounts, total, totalPages, err := paginatedAccounts(db, page, limit, search, sort, order)
	if err != nil {
		log.Printf("[ERROR] Inside paginatedHandler: %v\n", err)
		http.Error(w, "Failed to fetch accounts. Please try again later.", http.StatusInternalServerError)
		return
	}

	// Prepare and send the response
	response := map[string]interface{}{
		"data":        accounts,
		"total":       total,
		"totalPages":  totalPages,
		"currentPage": page,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func paginatedAccounts(db *sql.DB, page int, limit int, search, sort, order string) ([]AccountWithClassAndScore, int, int, error) {
	offset := (page - 1) * limit

	fmt.Println("[DEBUG] paginatedAccounts - Parameters:", page, limit, search, sort, order)

	baseQuery := `
        WITH ranked_accounts AS (
            SELECT 
                accounts.acc_id,
                accounts.username,
                accounts.email,
                characters.class_id,
                COALESCE(MAX(scores.reward_score), 0) AS score,
                RANK() OVER (ORDER BY COALESCE(MAX(scores.reward_score), 0) DESC) as rank
            FROM accounts
            INNER JOIN characters ON characters.acc_id = accounts.acc_id
            INNER JOIN scores ON scores.char_id = characters.char_id
            GROUP BY accounts.acc_id, accounts.username, accounts.email, characters.class_id
        )
        SELECT *
        FROM ranked_accounts
    `

	// Dynamically construct query conditions and parameters
	queryConditions := ""
	params := []interface{}{}

	// Add search conditions if provided
	if search != "" {
		queryConditions += " WHERE username ILIKE $1 OR email ILIKE $2"
		params = append(params, "%"+search+"%", "%"+search+"%")
	}

	// Add sorting logic
	sortOrder := "ASC"
	if order == "desc" {
		sortOrder = "DESC"
	}

	switch sort {
	case "Username":
		queryConditions += fmt.Sprintf(" ORDER BY username %s", sortOrder)
	case "ClassID":
		queryConditions += fmt.Sprintf(" ORDER BY class_id %s", sortOrder)
	default: // Default: Order by rank and score
		queryConditions += fmt.Sprintf(" ORDER BY rank %s, score %s", sortOrder, sortOrder)
	}

	// Add LIMIT and OFFSET
	params = append(params, limit, offset)
	queryConditions += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(params)-1, len(params)) // Use proper indexing

	// Combine everything into the final query
	finalQuery := baseQuery + queryConditions

	fmt.Println("[DEBUG] Final Query:", finalQuery)
	fmt.Println("[DEBUG] Params:", params)

	// Execute the query with the constructed parameters
	rows, err := db.Query(finalQuery, params...)
	if err != nil {
		fmt.Printf("[ERROR] Query Failure: %v\n", err)
		return nil, 0, 0, err
	}
	defer rows.Close()

	results := []AccountWithClassAndScore{}
	for rows.Next() {
		var account AccountWithClassAndScore
		err := rows.Scan(&account.AccID, &account.UserName, &account.Email, &account.ClassID, &account.Score, &account.Rank)
		if err != nil {
			fmt.Printf("[ERROR] Error scanning row: %v\n", err)
			return nil, 0, 0, err
		}
		results = append(results, account)
	}

	// Count total entries for pagination
	countQuery := `
        WITH ranked_accounts AS (
            SELECT 
                accounts.acc_id,
                accounts.username,
                accounts.email,
                characters.class_id,
                COALESCE(MAX(scores.reward_score), 0) AS score,
                RANK() OVER (ORDER BY COALESCE(MAX(scores.reward_score), 0) DESC) as rank
            FROM accounts
            INNER JOIN characters ON characters.acc_id = accounts.acc_id
            INNER JOIN scores ON scores.char_id = characters.char_id
            GROUP BY accounts.acc_id, accounts.username, accounts.email, characters.class_id
        )
        SELECT COUNT(*) FROM ranked_accounts
    `

	if search != "" {
		countQuery += " WHERE username ILIKE $1 OR email ILIKE $2"
	}

	fmt.Println("[DEBUG] Count Query:", countQuery)

	var total int
	err = db.QueryRow(countQuery, params[:len(params)-2]...).Scan(&total) // Pass only search params for count query
	if err != nil {
		fmt.Printf("[ERROR] Count Query Failed: %v\n", err)
		return nil, 0, 0, err
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	return results, total, totalPages, nil
}
