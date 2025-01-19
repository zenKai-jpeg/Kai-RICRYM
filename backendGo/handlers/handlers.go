package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"

	"backendGo/cache"
	"backendGo/config"
	"backendGo/models"
	"backendGo/utils"
)

// Paginated handler
func PaginatedHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	// Extract query parameters
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")
	search := r.URL.Query().Get("search")
	sort := r.URL.Query().Get("sort")
	order := r.URL.Query().Get("order")

	// Validate input
	page, limit, err := utils.ValidatePaginationParams(pageStr, limitStr)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Generate cache key
	cacheKey := cache.GenerateCacheKey(page, limit, search, sort, order)

	// Check cache or query the database
	result, isCached, err := cache.FetchFromCacheOrExecute(cacheKey, func() ([]byte, error) {
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
		utils.WriteJSONResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch accounts"})
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

func paginatedAccounts(db *sql.DB, page, limit int, search string) ([]models.AccountWithClassAndScore, int, int, error) {
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

	var results []models.AccountWithClassAndScore
	var total int
	for rows.Next() {
		var account models.AccountWithClassAndScore
		if err := rows.Scan(&account.AccID, &account.UserName, &account.Email, &account.ClassID, &account.Score, &account.Rank, &total); err != nil {
			return nil, 0, 0, err
		}
		results = append(results, account)
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	return results, total, totalPages, nil
}

// Validate pagination parameters
func validatePaginationParams(pageStr, limitStr string) (int, int, error) {
	page, limit := config.DefaultPage, config.DefaultLimit

	if pageStr != "" {
		p, err := strconv.Atoi(pageStr)
		if err != nil || p < 1 {
			return 0, 0, fmt.Errorf("invalid 'page' parameter: must be a positive integer")
		}
		page = p
	}

	if limitStr != "" {
		l, err := strconv.Atoi(limitStr)
		if err != nil || l < 1 || l > config.MaxResultsPerPage {
			return 0, 0, fmt.Errorf("invalid 'limit' parameter: must be between 1 and %d", config.MaxResultsPerPage)
		}
		limit = l
	}

	return page, limit, nil
}
