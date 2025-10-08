package prompt

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/maximhq/maxim-go/apis"
)

var (
	promptVersionCache sync.Map = sync.Map{} // key: sha256 of "baseUrl:apiKey:versionId:promptId", value: *apis.PromptVersion
)

// GetPromptVersion fetches a specific version of a prompt with caching.
//
// Parameters:
//   - baseUrl: The base URL of the API endpoint.
//   - apiKey: The API key for authentication.
//   - versionId: The version ID you want to query.
//   - promptId: The prompt ID whose versions you want to query.
//
// Returns:
//   - *apis.PromptVersion: The prompt version if successful, or nil with an error.
//   - error: An error if the request fails.
func GetPromptVersion(baseUrl, apiKey, versionId, promptId string) (*apis.PromptVersion, error) {
	// Create cache key from both IDs
	hashInput := fmt.Sprintf("%q|%q|%q|%q", baseUrl, apiKey, versionId, promptId)
	hash := sha256.Sum256([]byte(hashInput))
	cacheKey := hex.EncodeToString(hash[:])

	// Check cache first
	if cached, ok := promptVersionCache.Load(cacheKey); ok {
		return cached.(*apis.PromptVersion), nil
	}

	// Fetch from API if not cached
	resp, err := apis.GetPromptVersion(baseUrl, apiKey, versionId, promptId)
	if err != nil {
		return nil, fmt.Errorf("failed to get prompt version: %s", err.Message)
	}

	// Store in cache
	promptVersionCache.Store(cacheKey, resp)
	return resp, nil
}
