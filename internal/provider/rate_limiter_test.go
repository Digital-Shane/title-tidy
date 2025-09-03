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
		start := time.Now()
		for i := 0; i < 5; i++ {
			if err := rl.wait(); err != nil {
				t.Errorf("wait() request %d error = %v, want nil", i+1, err)
			}
		}
		elapsed := time.Since(start)

		// Should be very fast since we're under the limit
		if elapsed > 100*time.Millisecond {
			t.Errorf("5 requests under limit took %v, expected < 100ms", elapsed)
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

		// 3rd request should be delayed until window allows it
		if err := rl.wait(); err != nil {
			t.Errorf("wait() request 3 error = %v, want nil", err)
		}

		elapsed := time.Since(start)
		// Should wait at least the window duration (500ms)
		if elapsed < 500*time.Millisecond {
			t.Errorf("3rd request took %v, expected at least 500ms delay", elapsed)
		}
	})

	t.Run("CleansUpOldRequests", func(t *testing.T) {
		rl := newRateLimiter(3, 200*time.Millisecond)

		// Make 3 requests to fill the limit
		for i := 0; i < 3; i++ {
			if err := rl.wait(); err != nil {
				t.Errorf("wait() initial request %d error = %v", i+1, err)
			}
		}

		// Wait for window to pass completely
		time.Sleep(250 * time.Millisecond)

		// Should be able to make 3 more requests quickly
		start := time.Now()
		for i := 0; i < 3; i++ {
			if err := rl.wait(); err != nil {
				t.Errorf("wait() after window request %d error = %v", i+1, err)
			}
		}

		elapsed := time.Since(start)
		// Should be fast since old requests have expired
		if elapsed > 100*time.Millisecond {
			t.Errorf("Requests after window took %v, expected < 100ms", elapsed)
		}
	})

	t.Run("ConcurrentRequests", func(t *testing.T) {
		rl := newRateLimiter(10, 1*time.Second)

		var wg sync.WaitGroup
		successCount := 0
		var mu sync.Mutex

		// Launch 15 goroutines trying to make requests
		for i := 0; i < 15; i++ {
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

		// All requests should succeed (they'll just be throttled)
		if successCount != 15 {
			t.Errorf("Only %d concurrent requests succeeded, expected 15", successCount)
		}
	})

	t.Run("NeverReturnsRateLimitError", func(t *testing.T) {
		rl := newRateLimiter(1, 100*time.Millisecond)

		// Fill the limit
		if err := rl.wait(); err != nil {
			t.Errorf("wait() first request error = %v", err)
		}

		// Additional requests should wait but never return error
		for i := 0; i < 5; i++ {
			if err := rl.wait(); err != nil {
				t.Errorf("wait() request %d returned error %v, should never return error", i+2, err)
			}
		}
	})

	t.Run("RespectsSlidingWindow", func(t *testing.T) {
		rl := newRateLimiter(3, 300*time.Millisecond)

		// Make 3 requests quickly
		start := time.Now()
		for i := 0; i < 3; i++ {
			if err := rl.wait(); err != nil {
				t.Errorf("wait() request %d error = %v", i+1, err)
			}
		}

		// 4th request should wait for window
		if err := rl.wait(); err != nil {
			t.Errorf("wait() 4th request error = %v", err)
		}

		elapsed := time.Since(start)
		// Should wait at least the window duration
		if elapsed < 300*time.Millisecond {
			t.Errorf("4th request took %v, expected at least 300ms", elapsed)
		}

		// But shouldn't wait much longer (should be efficient)
		if elapsed > 400*time.Millisecond {
			t.Errorf("4th request took %v, expected around 300ms (too slow)", elapsed)
		}
	})

	t.Run("TMDBRealWorldScenario", func(t *testing.T) {
		// Test with TMDB-like limits: 38 requests per 10 seconds
		rl := newRateLimiter(38, 10*time.Second)

		start := time.Now()

		// Make 38 requests - should all go through quickly
		for i := 0; i < 38; i++ {
			if err := rl.wait(); err != nil {
				t.Errorf("wait() request %d error = %v", i+1, err)
			}
		}

		firstBatchTime := time.Since(start)
		// Should be fast for first batch
		if firstBatchTime > 1*time.Second {
			t.Errorf("First 38 requests took %v, expected < 1s", firstBatchTime)
		}

		// 39th request should wait for window
		start = time.Now()
		if err := rl.wait(); err != nil {
			t.Errorf("wait() 39th request error = %v", err)
		}
		waitTime := time.Since(start)

		// Should wait close to 10 seconds (when first request expires)
		if waitTime < 9*time.Second || waitTime > 11*time.Second {
			t.Errorf("39th request waited %v, expected ~10s", waitTime)
		}
	})
}
