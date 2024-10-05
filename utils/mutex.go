package utils

import (
	"errors"
	"sync"
	"time"
)

// Mutex represents a custom mutex implementation
type Mutex struct {
	mu                sync.Mutex
	initialRetryDelay time.Duration
	maxRetries        int
}

// NewMutex creates a new Mutex instance
func NewMutex() *Mutex {
	return &Mutex{
		initialRetryDelay: 100 * time.Millisecond,
		maxRetries:        10,
	}
}

// tryAcquire attempts to acquire the lock without blocking
func (m *Mutex) tryAcquire() bool {
	return m.mu.TryLock()
}

// Release releases the lock
func (m *Mutex) Release() {
	m.mu.Unlock()
}

// Acquire attempts to acquire the lock with retries and exponential backoff
func (m *Mutex) Acquire() error {
	retries := 0
	retryDelay := m.initialRetryDelay

	for !m.tryAcquire() {
		if retries >= m.maxRetries {
			return errors.New("maximum number of retries exceeded")
		}
		time.Sleep(retryDelay)
		retryDelay *= 2 // Exponential backoff
		retries++
	}

	return nil
}

// SetInitialRetryDelay sets the initial retry delay
func (m *Mutex) SetInitialRetryDelay(delay time.Duration) {
	m.initialRetryDelay = delay
}

// SetMaxRetries sets the maximum number of retries
func (m *Mutex) SetMaxRetries(maxRetries int) {
	m.maxRetries = maxRetries
}
