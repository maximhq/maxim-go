package logging

import (
	"time"
)

func utcNow() time.Time {
	return time.Now().UTC()
}

func utcNowPtr() *time.Time {
	now := time.Now().UTC()
	return &now
}

func removeDuplicateStrings(slice []string) []string {
	seen := make(map[string]struct{})
	unique := make([]string, 0, len(slice))
	for _, item := range slice {
		if _, exists := seen[item]; !exists {
			seen[item] = struct{}{}
			unique = append(unique, item)
		}
	}
	return unique
}
