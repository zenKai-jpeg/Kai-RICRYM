package config

import "time"

// Cache configuration constants
const (
	CacheExpiration      = 5 * time.Minute
	CacheCleanupInterval = 10 * time.Minute
)

// Pagination configuration constants
const (
	DefaultPage       = 1
	DefaultLimit      = 10
	MaxResultsPerPage = 100
)
