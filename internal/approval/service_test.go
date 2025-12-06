package approval

import (
	"sync"
	"testing"
	"time"
)

func TestService_Flow(t *testing.T) {
	svc := New()

	// 1. Create a Request
	reqID, ch := svc.NewRequest()
	if reqID == "" {
		t.Fatal("Expected valid reqID, got empty")
	}

	// 2. Simulate async approval in a goroutine
	go func() {
		// Wait a tiny bit to simulate human reaction
		time.Sleep(10 * time.Millisecond)
		resolved := svc.ResolveRequest(reqID, true)
		if !resolved {
			t.Errorf("Expected ResolveRequest to return true for valid ID %s", reqID)
		}
	}()

	// 3. Wait for result
	select {
	case approved := <-ch:
		if !approved {
			t.Fatal("Expected approved=true, got false")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timed out waiting for approval channel")
	}

	// 4. Verify cleanup
	svc.mu.RLock()
	_, exists := svc.pendingRequests[reqID]
	svc.mu.RUnlock()
	if exists {
		t.Error("Request ID should have been removed from map after resolution")
	}
}

func TestService_ResolveUnknown(t *testing.T) {
	svc := New()
	success := svc.ResolveRequest("super-fake-id", true)
	if success {
		t.Error("Expected ResolveRequest to return false for unknown ID")
	}
}

func TestService_Concurrency(t *testing.T) {
	svc := New()
	var wg sync.WaitGroup

	// Spawn 100 requests
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, ch := svc.NewRequest()

			// Immediately resolve it in another goroutine
			go svc.ResolveRequest(id, true)

			select {
			case <-ch:
				// Success
			case <-time.After(500 * time.Millisecond):
				t.Errorf("Timeout waiting for conccurent request %s", id)
			}
		}()
	}

	wg.Wait()

	svc.mu.RLock()
	count := len(svc.pendingRequests)
	svc.mu.RUnlock()

	if count != 0 {
		t.Errorf("Expected cleaned map, found %d lingering requests", count)
	}
}
