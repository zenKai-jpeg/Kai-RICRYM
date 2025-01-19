package utils

import (
	"encoding/json"
	"net/http"
	"strconv"

	"fmt"

	"backendGo/config"
)

// Unified JSON response function
func WriteJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// Validate pagination parameters
func ValidatePaginationParams(pageStr, limitStr string) (int, int, error) {
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
