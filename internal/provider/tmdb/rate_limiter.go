package tmdb

import (
	"sync"
	"time"
)

// rateLimiter implements a simple sliding window rate limiter
type rateLimiter struct {
	mu          sync.Mutex
	requests    []time.Time
	maxRequests int
	window      time.Duration
}

// newRateLimiter creates a new rate limiter
func newRateLimiter(maxRequests int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		maxRequests: maxRequests,
		window:      window,
		requests:    make([]time.Time, 0, maxRequests),
	}
}

// wait blocks until a request can be made within rate limits
// This method never returns an error - it always waits until a request can be made
func (r *rateLimiter) wait() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()

	// Clean up old requests outside the window
	cutoff := now.Add(-r.window)
	validRequests := make([]time.Time, 0, r.maxRequests)
	for _, req := range r.requests {
		if req.After(cutoff) {
			validRequests = append(validRequests, req)
		}
	}
	r.requests = validRequests

	// If we're under the limit, allow the request immediately
	if len(r.requests) < r.maxRequests {
		r.requests = append(r.requests, now)
		return nil
	}

	// We need to wait. Calculate when the oldest request will expire
	oldestRequest := r.requests[0]
	waitTime := r.window - now.Sub(oldestRequest)

	// Add a small buffer to ensure the request has actually expired
	waitTime += 10 * time.Millisecond

	// Release the lock before sleeping
	r.mu.Unlock()

	// Wait for the window to allow another request
	time.Sleep(waitTime)

	// Re-acquire lock and record the request
	r.mu.Lock()

	// Clean up again after waiting
	now = time.Now()
	cutoff = now.Add(-r.window)
	validRequests = make([]time.Time, 0, r.maxRequests)
	for _, req := range r.requests {
		if req.After(cutoff) {
			validRequests = append(validRequests, req)
		}
	}
	r.requests = validRequests

	// Record this request
	r.requests = append(r.requests, now)

	return nil
}
