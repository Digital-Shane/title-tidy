package provider

import (
	"sync"
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	t.Run("AllowsRequestsWithinLimit", func(t *testing.T) {
		rl := newRateLimiter(5, 1*time.Second)

		// Should allow 5 requests immediately
		for i := 0; i < 5; i++ {
			if err := rl.wait(); err != nil {
				t.Errorf("wait() request %d error = %v, want nil", i+1, err)
			}
		}
	})

	t.Run("BlocksExcessRequests", func(t *testing.T) {
		rl := newRateLimiter(2, 500*time.Millisecond)

		// First 2 requests should be immediate
		start := time.Now()
		for i := 0; i < 2; i++ {
			if err := rl.wait(); err != nil {
				t.Errorf("wait() request %d error = %v, want nil", i+1, err)
			}
		}

		// 3rd request should be delayed (at least 250ms min spacing + window time)
		if err := rl.wait(); err != nil {
			t.Errorf("wait() request 3 error = %v, want nil", err)
		}

		elapsed := time.Since(start)
		// With min spacing of 250ms between requests, expect at least 500ms total
		if elapsed < 500*time.Millisecond {
			t.Errorf("3rd request took %v, expected at least 500ms delay", elapsed)
		}
	})

	t.Run("CleansUpOldRequests", func(t *testing.T) {
		rl := newRateLimiter(3, 200*time.Millisecond)

		// Make 3 requests
		for i := 0; i < 3; i++ {
			if err := rl.wait(); err != nil {
				t.Errorf("wait() initial request %d error = %v", i+1, err)
			}
			time.Sleep(10 * time.Millisecond) // Small delay between requests
		}

		// Wait for window to pass completely
		time.Sleep(300 * time.Millisecond)

		// Should be able to make 3 more requests with only min spacing delays
		start := time.Now()
		for i := 0; i < 3; i++ {
			if err := rl.wait(); err != nil {
				t.Errorf("wait() after window request %d error = %v", i+1, err)
			}
		}

		elapsed := time.Since(start)
		// With min spacing of 250ms between requests, expect at least 500ms for 3 requests
		if elapsed > 800*time.Millisecond {
			t.Errorf("Requests after window took %v, expected around 500-750ms", elapsed)
		}
	})

	t.Run("ConcurrentRequests", func(t *testing.T) {
		rl := newRateLimiter(25, 2*time.Second) // Increased limits for concurrent test
		rl.maxRetries = 10                      // More retries for concurrent scenario

		var wg sync.WaitGroup
		successCount := 0
		var mu sync.Mutex

		// Launch 20 goroutines trying to make requests
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := rl.wait(); err == nil {
					mu.Lock()
					successCount++
					mu.Unlock()
				}
			}()
		}

		wg.Wait()

		// At least 10 requests should succeed (given the rate limit allows it)
		if successCount < 10 {
			t.Errorf("Only %d concurrent requests succeeded, expected at least 10", successCount)
		}
	})

	t.Run("ExponentialBackoff", func(t *testing.T) {
		rl := newRateLimiter(1, 100*time.Millisecond)
		rl.maxRetries = 2 // Reduce for testing

		// First request should succeed
		if err := rl.wait(); err != nil {
			t.Errorf("wait() first request error = %v", err)
		}

		// Second request should wait (min spacing 250ms)
		start := time.Now()
		if err := rl.wait(); err != nil {
			t.Errorf("wait() second request error = %v", err)
		}
		elapsed := time.Since(start)

		// With min spacing of 250ms, expect at least that
		if elapsed < 250*time.Millisecond || elapsed > 350*time.Millisecond {
			t.Errorf("Second request took %v, expected ~250ms (min spacing)", elapsed)
		}

		// Quick third request should use backoff
		time.Sleep(10 * time.Millisecond) // Small gap
		start = time.Now()
		if err := rl.wait(); err != nil {
			t.Errorf("wait() third request error = %v", err)
		}
		elapsed = time.Since(start)

		// Should have longer delay due to backoff
		if elapsed < 100*time.Millisecond {
			t.Errorf("Third request took %v, expected backoff delay", elapsed)
		}
	})

	t.Run("MaxRetriesExceeded", func(t *testing.T) {
		// Use waitWithJitter directly to test rate limiting logic
		rl := newRateLimiter(1, 100*time.Millisecond)
		rl.maxRetries = 2 // Very low for testing

		// First request succeeds
		if err := rl.waitWithJitter(0); err != nil {
			t.Errorf("wait() first request error = %v", err)
		}

		// Immediately try again - should wait but succeed (retry 1)
		if err := rl.waitWithJitter(0); err != nil {
			t.Errorf("wait() second request error = %v", err)
		}

		// Third request immediately - should wait but succeed (retry 2)
		if err := rl.waitWithJitter(0); err != nil {
			t.Errorf("wait() third request error = %v", err)
		}

		// Force the rate limiter state to exceed retries
		rl.mu.Lock()
		// Set retry count to max
		rl.retryCount = rl.maxRetries
		// Ensure the window is still full
		rl.requests = []time.Time{time.Now()}
		rl.mu.Unlock()

		// This should now exceed max retries
		err := rl.waitWithJitter(0)
		if err != ErrRateLimited {
			t.Errorf("wait() after max retries = %v, want ErrRateLimited", err)
		}
	})
}
