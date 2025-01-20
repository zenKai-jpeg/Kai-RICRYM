package cache

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"

	"backendGo/config"

	"github.com/patrickmn/go-cache"
)

// Global cache
var appCache *cache.Cache

// Initialize cache
func InitializeCache() {
	appCache = cache.New(config.CacheExpiration, config.CacheCleanupInterval)
}

// Generate cache key from query parameters, including the new filters (class, minScore, maxScore)
func GenerateCacheKey(page, limit int, search, sort, order, class, minScoreStr, maxScoreStr string) string {
	// Include class, minScore, and maxScore in the cache key generation
	rawKey := fmt.Sprintf("page:%d-limit:%d-search:%s-sort:%s-order:%s-class:%s-minScore:%s-maxScore:%s", page, limit, search, sort, order, class, minScoreStr, maxScoreStr)
	hash := md5.Sum([]byte(rawKey))
	return hex.EncodeToString(hash[:])
}

// Fetch from cache or execute query
func FetchFromCacheOrExecute(cacheKey string, queryFunc func() ([]byte, error)) ([]byte, bool, error) {
	if cachedData, found := appCache.Get(cacheKey); found {
		return cachedData.([]byte), true, nil
	}

	// Execute query if no cache hit
	result, err := queryFunc()
	if err != nil {
		return nil, false, err
	}

	// Cache the result
	appCache.Set(cacheKey, result, config.CacheExpiration)
	return result, false, nil
}
