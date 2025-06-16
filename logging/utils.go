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

func uuid() string {
	// Generate a UUID v4 using random values
	// Based on RFC 4122 format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
	// where x is any random hex digit and y is one of 8, 9, A, or B

	const hexChars = "0123456789abcdef"
	u := make([]byte, 36)

	// Generate random bytes
	randBytes := make([]byte, 16)
	
	// Format according to UUID v4 layout
	for i, offset := 0, 0; i < 16; i++ {
		switch i {
		case 4, 6, 8, 10:
			u[offset] = '-'
			offset++
		}

		// Special handling for version and variant bits
		if i == 6 {
			u[offset] = hexChars[0x4] // Version 4
			u[offset+1] = hexChars[randBytes[i]&0x0f]
		} else if i == 8 {
			u[offset] = hexChars[0x8|randBytes[i]>>4&0x3] // Variant bits: 10xx
			u[offset+1] = hexChars[randBytes[i]&0x0f]
		} else {
			u[offset] = hexChars[randBytes[i]>>4]
			u[offset+1] = hexChars[randBytes[i]&0x0f]
		}
		offset += 2
	}

	return string(u)
}
