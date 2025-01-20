package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"

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
	class := r.URL.Query().Get("class")          // New parameter for class filter
	minScoreStr := r.URL.Query().Get("minScore") // New parameter for minimum score filter
	maxScoreStr := r.URL.Query().Get("maxScore") // New parameter for maximum score filter

	// Validate input
	page, limit, err := validatePaginationParams(pageStr, limitStr)
	if err != nil {
		utils.WriteJSONResponse(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Generate cache key (including the new filters)
	cacheKey := cache.GenerateCacheKey(page, limit, search, sort, order, class, minScoreStr, maxScoreStr)

	// Debugging log for cache key
	fmt.Println("Cache Key:", cacheKey)

	// Check cache or query the database
	result, isCached, err := cache.FetchFromCacheOrExecute(cacheKey, func() ([]byte, error) {
		accounts, total, totalPages, err := paginatedAccounts(db, page, limit, search, sort, order, class, minScoreStr, maxScoreStr)
		if err != nil {
			// Log error if query fails
			fmt.Println("Error in paginatedAccounts query:", err)
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
		// Log the error and return the response
		fmt.Println("Failed to fetch accounts:", err)
		utils.WriteJSONResponse(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch accounts"})
		return
	}

	// Serve cached or fresh data
	if isCached {
		fmt.Println("[DEBUG] Cache hit for:", cacheKey)
	} else {
		fmt.Println("[DEBUG] Cache miss. Querying DB and caching result.")
	}

	// Log the response result for debugging
	fmt.Println("Final Response:", string(result))

	w.Write(result)
}
func paginatedAccounts(db *sql.DB, page, limit int, search, sort, order, class, minScoreStr, maxScoreStr string) ([]models.AccountWithClassAndScore, int, int, error) {
	offset := (page - 1) * limit

	// Whitelist sorting columns
	sortColumn := "rank"
	switch sort {
	case "rank", "username", "class_id", "score":
		sortColumn = sort
	}

	// Validate order direction
	sortOrder := "ASC"
	if order == "desc" {
		sortOrder = "DESC"
	}

	var queryBuilder strings.Builder
	queryBuilder.WriteString(`
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
		WHERE 1=1 -- Start with a condition that is always true
	`)

	params := make([]interface{}, 0)
	paramIndex := 1

	if search != "" {
		queryBuilder.WriteString(fmt.Sprintf(" AND (username ILIKE $%d OR email ILIKE $%d)", paramIndex, paramIndex+1))
		params = append(params, "%"+search+"%", "%"+search+"%")
		paramIndex += 2
	}

	if class != "" {
		queryBuilder.WriteString(fmt.Sprintf(" AND class_id = $%d", paramIndex))
		params = append(params, class)
		paramIndex++
	}

	if minScoreStr != "" {
		if minScore, err := strconv.Atoi(minScoreStr); err == nil {
			queryBuilder.WriteString(fmt.Sprintf(" AND score >= $%d", paramIndex))
			params = append(params, minScore)
			paramIndex++
		} else {
			fmt.Println("Invalid minScore value:", minScoreStr)
		}
	}

	if maxScoreStr != "" {
		if maxScore, err := strconv.Atoi(maxScoreStr); err == nil {
			queryBuilder.WriteString(fmt.Sprintf(" AND score <= $%d", paramIndex))
			params = append(params, maxScore)
			paramIndex++
		} else {
			fmt.Println("Invalid maxScore value:", maxScoreStr)
		}
	}

	queryBuilder.WriteString(fmt.Sprintf(" ORDER BY %s %s LIMIT $%d OFFSET $%d", sortColumn, sortOrder, paramIndex, paramIndex+1))
	params = append(params, limit, offset)

	// fmt.Println("Executing query:", queryBuilder.String())

	rows, err := db.Query(queryBuilder.String(), params...)
	if err != nil {
		fmt.Println("Query execution error:", err)
		return nil, 0, 0, err
	}
	defer rows.Close()

	var results []models.AccountWithClassAndScore
	var total int
	for rows.Next() {
		var account models.AccountWithClassAndScore
		if err := rows.Scan(&account.AccID, &account.UserName, &account.Email, &account.ClassID, &account.Score, &account.Rank, &total); err != nil {
			// Log error if row scan fails
			fmt.Println("Error scanning row:", err)
			return nil, 0, 0, err
		}
		results = append(results, account)
	}

	// Calculate total pages for pagination
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
