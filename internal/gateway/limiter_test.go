package gateway_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-learning/internal/gateway"
)

// TestFixedWindowLimiter_ConcurrentAllow_SingleKey verifies that concurrent calls
// from multiple goroutines hitting the same client key are safely serialized by the mutex,
// correctly permitting exactly the quota limit and rejecting subsequent requests.
func TestFixedWindowLimiter_ConcurrentAllow_SingleKey(t *testing.T) {
	const limit = 25
	const totalRequests = 100
	const window = 2 * time.Second

	limiter := gateway.NewFixedWindowLimiter(limit, window)
	defer limiter.Stop()

	ctx := context.Background()
	var allowedCount int64
	var deniedCount int64

	var wg sync.WaitGroup
	wg.Add(totalRequests)

	for i := 0; i < totalRequests; i++ {
		go func() {
			defer wg.Done()
			allowed, _ := limiter.Allow(ctx, "192.168.1.100")
			if allowed {
				atomic.AddInt64(&allowedCount, 1)
			} else {
				atomic.AddInt64(&deniedCount, 1)
			}
		}()
	}

	wg.Wait()

	if allowedCount != limit {
		t.Fatalf("expected exactly %d allowed requests, got %d", limit, allowedCount)
	}
	if deniedCount != (totalRequests - limit) {
		t.Fatalf("expected exactly %d denied requests, got %d", totalRequests-limit, deniedCount)
	}
}

// TestFixedWindowLimiter_ConcurrentAllow_MultipleKeys verifies that concurrent operations
// across distinct client keys safely manipulate the shared map without data races.
func TestFixedWindowLimiter_ConcurrentAllow_MultipleKeys(t *testing.T) {
	const limitPerClient = 5
	const numClients = 10
	const requestsPerClient = 20
	const window = 2 * time.Second

	limiter := gateway.NewFixedWindowLimiter(limitPerClient, window)
	defer limiter.Stop()

	ctx := context.Background()
	var wg sync.WaitGroup
	allowedCounts := make([]int64, numClients)

	for clientIdx := 0; clientIdx < numClients; clientIdx++ {
		cIdx := clientIdx
		clientKey := fmt.Sprintf("client-%d", cIdx)
		for req := 0; req < requestsPerClient; req++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				allowed, _ := limiter.Allow(ctx, clientKey)
				if allowed {
					atomic.AddInt64(&allowedCounts[cIdx], 1)
				}
			}()
		}
	}

	wg.Wait()

	for i, count := range allowedCounts {
		if count != limitPerClient {
			t.Errorf("client-%d: expected %d allowed requests, got %d", i, limitPerClient, count)
		}
	}
}

// TestFixedWindowLimiter_ConcurrentAllowAndStop exercises concurrent Allow invocations
// alongside Stop() to ensure the background cleanup goroutine and channels terminate safely.
func TestFixedWindowLimiter_ConcurrentAllowAndStop(t *testing.T) {
	limiter := gateway.NewFixedWindowLimiter(100, 100*time.Millisecond)
	ctx := context.Background()

	var wg sync.WaitGroup
	const workers = 20

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			key := fmt.Sprintf("worker-%d", workerID)
			for j := 0; j < 50; j++ {
				_, _ = limiter.Allow(ctx, key)
				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	// Trigger Stop while workers are active
	time.Sleep(10 * time.Millisecond)
	limiter.Stop()

	wg.Wait()
}
