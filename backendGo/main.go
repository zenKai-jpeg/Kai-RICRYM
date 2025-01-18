package main

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	_ "github.com/lib/pq"
	"github.com/patrickmn/go-cache"
)

// Global cache
var appCache *cache.Cache

// Structs
type Account struct {
	AccID    uint64 `json:"AccID"`
	UserName string `json:"Username"`
	Email    string `json:"Email"`
}

type AccountWithClassAndScore struct {
	AccID    uint64 `json:"AccID"`
	UserName string `json:"Username"`
	Email    string `json:"Email"`
	ClassID  int    `json:"ClassID"`
	Score    int    `json:"Score"`
	Rank     int    `json:"Rank"`
}

// Cache configuration constants
const (
	CacheExpiration      = 5 * time.Minute
	CacheCleanupInterval = 10 * time.Minute
	DefaultPage          = 1
	DefaultLimit         = 10
	MaxResultsPerPage    = 100
)

func main() {
	// Initialize the cache
	initializeCache()

	// Connect to the database
	db := connectDB()
	defer db.Close()

	// Create tables if needed
	createTables(db)

	// Populate the database with fake data if it is empty
	generateDataIfNeeded(db)

	// Set up HTTP routes
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeJSONResponse(w, http.StatusOK, map[string]string{
			"message": "Welcome to the API!",
		})
	})
	http.HandleFunc("/accounts", func(w http.ResponseWriter, r *http.Request) {
		paginatedHandler(w, r, db)
	})

	// Start HTTP server
	port := ":8080"
	fmt.Printf("Server is running at %s\n", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// Initialize cache
func initializeCache() {
	appCache = cache.New(CacheExpiration, CacheCleanupInterval)
}

// Unified JSON response function
func writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// Validate pagination parameters
func validatePaginationParams(pageStr, limitStr string) (int, int, error) {
	page, limit := DefaultPage, DefaultLimit

	if pageStr != "" {
		p, err := strconv.Atoi(pageStr)
		if err != nil || p < 1 {
			return 0, 0, fmt.Errorf("invalid 'page' parameter: must be a positive integer")
		}
		page = p
	}

	if limitStr != "" {
		l, err := strconv.Atoi(limitStr)
		if err != nil || l < 1 || l > MaxResultsPerPage {
			return 0, 0, fmt.Errorf("invalid 'limit' parameter: must be between 1 and %d", MaxResultsPerPage)
		}
		limit = l
	}

	return page, limit, nil
}

/* --------------------------------------------------------------------------------- */

// Generate cache key from query parameters
func generateCacheKey(page, limit int, search, sort, order string) string {
	rawKey := fmt.Sprintf("page:%d-limit:%d-search:%s-sort:%s-order:%s", page, limit, search, sort, order)
	hash := md5.Sum([]byte(rawKey))
	return hex.EncodeToString(hash[:])
}

// Fetch from cache or execute query
func fetchFromCacheOrExecute(cacheKey string, queryFunc func() ([]byte, error)) ([]byte, bool, error) {
	if cachedData, found := appCache.Get(cacheKey); found {
		return cachedData.([]byte), true, nil
	}

	// Execute query if no cache hit
	result, err := queryFunc()
	if err != nil {
		return nil, false, err
	}

	// Cache the result
	appCache.Set(cacheKey, result, CacheExpiration)
	return result, false, nil
}

/* --------------------------------------------------------------------------------- */

// Paginated handler
func paginatedHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	// Extract query parameters
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")
	search := r.URL.Query().Get("search")
	sort := r.URL.Query().Get("sort")
	order := r.URL.Query().Get("order")

	// Validate input
	page, limit, err := validatePaginationParams(pageStr, limitStr)
	if err != nil {
		writeJSONResponse(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Generate cache key
	cacheKey := generateCacheKey(page, limit, search, sort, order)

	// Check cache or query the database
	result, isCached, err := fetchFromCacheOrExecute(cacheKey, func() ([]byte, error) {
		accounts, total, totalPages, err := paginatedAccounts(db, page, limit, search)
		if err != nil {
			return nil, err
		}

		// Prepare response payload
		response := map[string]interface{}{
			"data":            accounts,
			"total":           total,
			"totalPages":      totalPages,
			"currentPage":     page,
			"hasNextPage":     page < totalPages,
			"hasPreviousPage": page > 1,
		}
		return json.Marshal(response)
	})

	if err != nil {
		writeJSONResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch accounts"})
		return
	}

	// Serve cached or fresh data
	if isCached {
		fmt.Println("[DEBUG] Cache hit for:", cacheKey)
	} else {
		fmt.Println("[DEBUG] Cache miss. Querying DB and caching result.")
	}
	w.Write(result)
}

/* --------------------------------------------------------------------------------- */

// Database connection
func connectDB() *sql.DB {
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
func createTables(db *sql.DB) {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS accounts (acc_id BIGSERIAL PRIMARY KEY, username VARCHAR(50), email VARCHAR(50))`,
		`CREATE TABLE IF NOT EXISTS characters (char_id BIGSERIAL PRIMARY KEY, acc_id BIGINT REFERENCES accounts(acc_id), class_id SMALLINT)`,
		`CREATE TABLE IF NOT EXISTS scores (score_id BIGSERIAL PRIMARY KEY, char_id BIGINT REFERENCES characters(char_id), reward_score INT)`,
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
func generateDataIfNeeded(db *sql.DB) {
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

func generateFakeData(db *sql.DB, start, count int) {
	for i := start; i < start+count; i++ {
		username := gofakeit.Username()
		email := gofakeit.Email()

		var accID uint64
		err := db.QueryRow("INSERT INTO accounts (username, email) VALUES ($1, $2) RETURNING acc_id", username, email).Scan(&accID)
		if err != nil {
			log.Printf("Error creating account: %v", err)
			continue
		}

		for j := 0; j < 4; j++ {
			classID := gofakeit.Number(1, 8)
			var charID uint64
			err = db.QueryRow("INSERT INTO characters (acc_id, class_id) VALUES ($1, $2) RETURNING char_id", accID, classID).Scan(&charID)
			if err != nil {
				log.Printf("Error creating character: %v", err)
				continue
			}

			rewardScore := gofakeit.Number(10, 1000)
			_, err = db.Exec("INSERT INTO scores (char_id, reward_score) VALUES ($1, $2)", charID, rewardScore)
			if err != nil {
				log.Printf("Error creating score: %v", err)
			}
		}
	}
}

// Paginated accounts query
func paginatedAccounts(db *sql.DB, page, limit int, search string) ([]AccountWithClassAndScore, int, int, error) {
	offset := (page - 1) * limit

	query := `
		WITH ranked_accounts AS (
			SELECT 
				accounts.acc_id,
				accounts.username,
				accounts.email,
				characters.class_id,
				COALESCE(MAX(scores.reward_score), 0) AS score,
				RANK() OVER (ORDER BY COALESCE(MAX(scores.reward_score), 0) DESC) AS rank
			FROM accounts
			INNER JOIN characters ON characters.acc_id = accounts.acc_id
			INNER JOIN scores ON scores.char_id = characters.char_id
			GROUP BY accounts.acc_id, accounts.username, accounts.email, characters.class_id
		)
		SELECT *, COUNT(*) OVER() AS total_count
		FROM ranked_accounts
		WHERE username ILIKE $1 OR email ILIKE $2
		ORDER BY rank ASC, score ASC
		LIMIT $3 OFFSET $4`

	rows, err := db.Query(query, "%"+search+"%", "%"+search+"%", limit, offset)
	if err != nil {
		return nil, 0, 0, err
	}
	defer rows.Close()

	var results []AccountWithClassAndScore
	var total int
	for rows.Next() {
		var account AccountWithClassAndScore
		if err := rows.Scan(&account.AccID, &account.UserName, &account.Email, &account.ClassID, &account.Score, &account.Rank, &total); err != nil {
			return nil, 0, 0, err
		}
		results = append(results, account)
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	return results, total, totalPages, nil
}
