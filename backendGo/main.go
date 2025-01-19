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
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

// Global cache
var appCache *cache.Cache

// Structs
type Account struct {
	AccID             uint64 `json:"AccID"`
	UserName          string `json:"Username"`
	Email             string `json:"Email"`
	EncryptedPassword string `json:"EncryptedPassword"` // For storing the password securely
	SecretKey2FA      string `json:"SecretKey2FA"`      // For 2FA secret key (used for generating OTP)
}

// Struct for managing sessions
type Session struct {
	SessionID      string    `json:"SessionID"`
	AccID          uint64    `json:"AccID"`
	Metadata       string    `json:"Metadata"`       // Additional metadata, like user agent, IP, etc.
	ExpiryDateTime time.Time `json:"ExpiryDateTime"` // Expiry of the session
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
	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		loginHandler(w, r, db)
	})

	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		registerHandler(w, r, db)
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
		`CREATE TABLE IF NOT EXISTS accounts (acc_id BIGSERIAL PRIMARY KEY, username VARCHAR(50) NOT NULL, email VARCHAR(50) NOT NULL, encrypted_password TEXT NOT NULL, secretkey_2fa TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS characters (char_id BIGSERIAL PRIMARY KEY, acc_id BIGINT REFERENCES accounts(acc_id), class_id SMALLINT)`,
		`CREATE TABLE IF NOT EXISTS scores (score_id BIGSERIAL PRIMARY KEY, char_id BIGINT REFERENCES characters(char_id), reward_score INT)`,
		`CREATE TABLE IF NOT EXISTS sessions (score_id BIGSERIAL PRIMARY KEY, char_id BIGINT REFERENCES characters(char_id), reward_score INT)`,
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
		// Insert account with username and email, return the account ID
		err := db.QueryRow("INSERT INTO accounts (username, email) VALUES ($1, $2) RETURNING acc_id", username, email).Scan(&accID)
		if err != nil {
			log.Printf("Error creating account: %v", err)
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

// Hash password
func hashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedPassword), nil
}

// Compare passwords
func checkPassword(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

// Generate 2FA key for the account
func generate2FASecret() (string, string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "MyApp",            // Change to your app name
		AccountName: "user@example.com", // Replace with user email
	})
	if err != nil {
		return "", "", err
	}
	return key.Secret(), key.URL(), nil
}

// Verify 2FA code
func verify2FACode(secret, code string) bool {
	return totp.Validate(code, secret)
}

// Login Handler
func loginHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	// Extract login details
	var loginDetails struct {
		Username  string `json:"Username"`
		Password  string `json:"Password"`
		TwoFACode string `json:"TwoFACode"`
	}
	err := json.NewDecoder(r.Body).Decode(&loginDetails)
	if err != nil {
		writeJSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	// Query the account by username
	var account Account
	err = db.QueryRow("SELECT acc_id, username, email, encrypted_password, secretkey_2fa FROM accounts WHERE username = $1", loginDetails.Username).Scan(
		&account.AccID, &account.UserName, &account.Email, &account.EncryptedPassword, &account.SecretKey2FA,
	)
	if err != nil {
		writeJSONResponse(w, http.StatusUnauthorized, map[string]string{"error": "Invalid credentials"})
		return
	}

	// Check password
	if !checkPassword(account.EncryptedPassword, loginDetails.Password) {
		writeJSONResponse(w, http.StatusUnauthorized, map[string]string{"error": "Invalid credentials"})
		return
	}

	// Verify 2FA code
	if !verify2FACode(account.SecretKey2FA, loginDetails.TwoFACode) {
		writeJSONResponse(w, http.StatusUnauthorized, map[string]string{"error": "Invalid 2FA code"})
		return
	}

	// Create session or any necessary login steps here (omitted for brevity)
	writeJSONResponse(w, http.StatusOK, map[string]string{"message": "Login successful"})
}

// Register Handler
func registerHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	// Extract account details
	var accountDetails struct {
		Username string `json:"Username"`
		Email    string `json:"Email"`
		Password string `json:"Password"`
	}
	err := json.NewDecoder(r.Body).Decode(&accountDetails)
	if err != nil {
		writeJSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	// Check if username already exists
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM accounts WHERE username = $1", accountDetails.Username).Scan(&count)
	if err != nil {
		writeJSONResponse(w, http.StatusInternalServerError, map[string]string{"error": "Error checking username"})
		return
	}

	if count > 0 {
		writeJSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Username already taken"})
		return
	}

	// Hash password
	hashedPassword, err := hashPassword(accountDetails.Password)
	if err != nil {
		writeJSONResponse(w, http.StatusInternalServerError, map[string]string{"error": "Error hashing password"})
		return
	}

	// Debugging: Log the hashed password to check if it's correct
	log.Printf("Hashed Password: %s", hashedPassword)

	// Ensure hashedPassword is not empty before proceeding
	if hashedPassword == "" {
		writeJSONResponse(w, http.StatusInternalServerError, map[string]string{"error": "Password hashing failed"})
		return
	}

	// Generate 2FA secret
	secret, _, err := generate2FASecret()
	if err != nil {
		writeJSONResponse(w, http.StatusInternalServerError, map[string]string{"error": "Error generating 2FA secret"})
		return
	}

	// Insert new account into the database
	var accID uint64
	err = db.QueryRow("INSERT INTO accounts (username, email, encrypted_password, secretkey_2fa) VALUES ($1, $2, $3, $4) RETURNING acc_id", accountDetails.Username, accountDetails.Email, hashedPassword, secret).Scan(&accID)
	if err != nil {
		writeJSONResponse(w, http.StatusInternalServerError, map[string]string{"error": "Error creating account"})
		return
	}

	// Respond with success message
	writeJSONResponse(w, http.StatusCreated, map[string]string{"message": "Account successfully created"})
}
