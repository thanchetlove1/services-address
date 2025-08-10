// Package search provides unified Meilisearch client wrapper
package search

import (
	"fmt"
	ms "github.com/meilisearch/meilisearch-go"
)

// ClientWrapper wraps Meilisearch client with compatible API for v1.5.x
type ClientWrapper struct {
	cli ms.ServiceManager
}

// NewClientWrapper creates new Meilisearch client wrapper
func NewClientWrapper(url, key string) *ClientWrapper {
	client := ms.New(url, ms.WithAPIKey(key))
	return &ClientWrapper{
		cli: client,
	}
}

// SearchIndex performs unified search with compatible parameters for Meilisearch 1.5.x
func (c *ClientWrapper) SearchIndex(index string, q string, filter string, limit int64, matching string) (*ms.SearchResponse, error) {
	idx := c.cli.Index(index)
	
	// Use only compatible fields for Meilisearch 1.5.x
	req := &ms.SearchRequest{
		Limit:  limit,
		Filter: filter, // e.g., "level = 3 AND parent_id = \"79\""
	}
	
	// Skip MatchingStrategy to avoid compatibility issues
	// Meilisearch 1.5.x handles matching automatically via typo tolerance
	
	return idx.Search(q, req)
}

// FilterLevelParent creates filter string for level and parent_id
func FilterLevelParent(level int, parentID string) string {
	if parentID == "" {
		return fmt.Sprintf("level = %d", level)
	}
	return fmt.Sprintf("level = %d AND parent_id = %q", level, parentID)
}

// FilterLevel creates simple level filter
func FilterLevel(level int) string {
	return fmt.Sprintf("level = %d", level)
}
