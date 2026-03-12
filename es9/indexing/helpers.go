package indexing

import (
	"context"
	"fmt"
	"sync"
)

// waitWithContext wraps a WaitGroup and returns a channel that closes when the WaitGroup is done
// This eliminates the need for manual channel-wrapping of WaitGroups
func waitWithContext(wg *sync.WaitGroup) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	return done
}

// ErrorCollector provides thread-safe error collection from multiple goroutines
// Replaces the pattern of buffered error channels with simpler mutex-based collection
type ErrorCollector struct {
	errors []error
	mu     sync.Mutex
}

// NewErrorCollector creates a new error collector
func NewErrorCollector() *ErrorCollector {
	return &ErrorCollector{
		errors: make([]error, 0),
	}
}

// Add adds an error to the collector (thread-safe)
func (ec *ErrorCollector) Add(err error) {
	if err == nil {
		return
	}
	ec.mu.Lock()
	ec.errors = append(ec.errors, err)
	ec.mu.Unlock()
}

// Error returns a combined error if any errors were collected, nil otherwise
func (ec *ErrorCollector) Error() error {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	if len(ec.errors) == 0 {
		return nil
	}

	if len(ec.errors) == 1 {
		return ec.errors[0]
	}

	return fmt.Errorf("multiple errors (%d): %v", len(ec.errors), ec.errors)
}

// HasErrors returns true if any errors were collected
func (ec *ErrorCollector) HasErrors() bool {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	return len(ec.errors) > 0
}

// ResultCollector provides generic result collection from multiple goroutines
// Replaces the pattern of buffered result channels with automatic collection
type ResultCollector[T any] struct {
	results []T
	ch      chan T
	mu      sync.Mutex
}

// NewResultCollector creates a new result collector with the given buffer size
func NewResultCollector[T any](bufferSize int) *ResultCollector[T] {
	return &ResultCollector[T]{
		results: make([]T, 0, bufferSize),
		ch:      make(chan T, bufferSize),
	}
}

// Chan returns the send channel for workers to send results
func (rc *ResultCollector[T]) Chan() chan<- T {
	return rc.ch
}

// Close closes the result channel (call after all workers finish)
func (rc *ResultCollector[T]) Close() {
	close(rc.ch)
}

// Collect drains the channel and returns all collected results
// Must be called after Close() to ensure all results are collected
func (rc *ResultCollector[T]) Collect() []T {
	for result := range rc.ch {
		rc.results = append(rc.results, result)
	}
	return rc.results
}

// CollectWithContext collects results until context is cancelled or channel is closed
func (rc *ResultCollector[T]) CollectWithContext(ctx context.Context) []T {
	for {
		select {
		case result, ok := <-rc.ch:
			if !ok {
				return rc.results
			}
			rc.results = append(rc.results, result)
		case <-ctx.Done():
			return rc.results
		}
	}
}
