package logging_test

import (
	"sync"
	"time"

	"github.com/maximhq/maxim-go/logging"
)

// MockWriter is a mock implementation of the writer interface for testing
type MockWriter struct {
	mu               sync.Mutex
	commits          []*logging.CommitLog
	flushCalled      bool
	cleanupCalled    bool
	isDebug          bool
	autoFlush        bool
	flushCount       int
	config           *MockWriterConfig
	ticker           *time.Ticker
	stopChan         chan bool
}

// MockWriterConfig contains configuration for the mock writer
type MockWriterConfig struct {
	IsDebug              bool
	AutoFlush            bool
	FlushIntervalSeconds int
}

// NewMockWriter creates a new mock writer instance
func NewMockWriter(config *MockWriterConfig) *MockWriter {
	if config == nil {
		config = &MockWriterConfig{
			IsDebug:              false,
			AutoFlush:            false,
			FlushIntervalSeconds: 10,
		}
	}

	mw := &MockWriter{
		commits:   make([]*logging.CommitLog, 0),
		config:    config,
		isDebug:   config.IsDebug,
		autoFlush: config.AutoFlush,
		stopChan:  make(chan bool),
	}

	if config.AutoFlush {
		mw.ticker = time.NewTicker(time.Duration(config.FlushIntervalSeconds) * time.Second)
		go mw.autoFlushLoop()
	}

	return mw
}

// autoFlushLoop handles automatic flushing when enabled
func (mw *MockWriter) autoFlushLoop() {
	for {
		select {
		case <-mw.ticker.C:
			mw.Flush()
		case <-mw.stopChan:
			return
		}
	}
}

// Commit records a commit log entry
func (mw *MockWriter) Commit(cl *logging.CommitLog) {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	mw.commits = append(mw.commits, cl)
}

// commit is the internal method used by logging entities (lowercase to match writer interface)
func (mw *MockWriter) commit(cl *logging.CommitLog) {
	mw.Commit(cl)
}

// Flush simulates flushing logs and increments flush count
func (mw *MockWriter) Flush() {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	mw.flushCalled = true
	mw.flushCount++
}

// Cleanup simulates cleanup operations
func (mw *MockWriter) Cleanup() {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	mw.cleanupCalled = true
	if mw.ticker != nil {
		mw.ticker.Stop()
	}
	close(mw.stopChan)
}

// GetCommits returns all recorded commit logs
func (mw *MockWriter) GetCommits() []*logging.CommitLog {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	// Return a copy to prevent concurrent modification
	commits := make([]*logging.CommitLog, len(mw.commits))
	copy(commits, mw.commits)
	return commits
}

// GetCommitCount returns the number of commits recorded
func (mw *MockWriter) GetCommitCount() int {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	return len(mw.commits)
}

// GetCommitsByEntity returns commits filtered by entity type
func (mw *MockWriter) GetCommitsByEntity(entity logging.Entity) []*logging.CommitLog {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	var filtered []*logging.CommitLog
	for _, commit := range mw.commits {
		if commit.GetEntity() == entity {
			filtered = append(filtered, commit)
		}
	}
	return filtered
}

// GetCommitsByAction returns commits filtered by action
func (mw *MockWriter) GetCommitsByAction(action string) []*logging.CommitLog {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	var filtered []*logging.CommitLog
	for _, commit := range mw.commits {
		if commit.GetAction() == action {
			filtered = append(filtered, commit)
		}
	}
	return filtered
}

// GetCommitsByEntityAndAction returns commits filtered by both entity and action
func (mw *MockWriter) GetCommitsByEntityAndAction(entity logging.Entity, action string) []*logging.CommitLog {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	var filtered []*logging.CommitLog
	for _, commit := range mw.commits {
		if commit.GetEntity() == entity && commit.GetAction() == action {
			filtered = append(filtered, commit)
		}
	}
	return filtered
}

// GetLastCommit returns the most recent commit log
func (mw *MockWriter) GetLastCommit() *logging.CommitLog {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	if len(mw.commits) == 0 {
		return nil
	}
	return mw.commits[len(mw.commits)-1]
}

// GetFlushCount returns the number of times flush was called
func (mw *MockWriter) GetFlushCount() int {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	return mw.flushCount
}

// WasFlushCalled returns whether flush was called at least once
func (mw *MockWriter) WasFlushCalled() bool {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	return mw.flushCalled
}

// WasCleanupCalled returns whether cleanup was called
func (mw *MockWriter) WasCleanupCalled() bool {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	return mw.cleanupCalled
}

// Clear removes all recorded commits and resets counters
func (mw *MockWriter) Clear() {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	mw.commits = make([]*logging.CommitLog, 0)
	mw.flushCalled = false
	mw.cleanupCalled = false
	mw.flushCount = 0
}

// AssertCommitCount checks if the commit count matches expected value
func (mw *MockWriter) AssertCommitCount(expected int) bool {
	return mw.GetCommitCount() == expected
}

// AssertEntityCommitCount checks if commits for a specific entity match expected count
func (mw *MockWriter) AssertEntityCommitCount(entity logging.Entity, expected int) bool {
	return len(mw.GetCommitsByEntity(entity)) == expected
}

// AssertActionCommitCount checks if commits for a specific action match expected count
func (mw *MockWriter) AssertActionCommitCount(action string, expected int) bool {
	return len(mw.GetCommitsByAction(action)) == expected
}

// PrintCommits prints all commits for debugging (useful when IsDebug is true)
func (mw *MockWriter) PrintCommits() {
	commits := mw.GetCommits()
	for i, commit := range commits {
		println("Commit", i+1, ":", commit.Serialize())
	}
}


