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
