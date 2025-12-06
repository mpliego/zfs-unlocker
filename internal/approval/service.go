package approval

import (
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	mu              sync.RWMutex
	pendingRequests map[string]chan bool
}

func New() *Service {
	return &Service{
		pendingRequests: make(map[string]chan bool),
	}
}

// NewRequest creates a new approval request, returns its ID and a channel to wait on.
func (s *Service) NewRequest() (string, <-chan bool) {
	id := uuid.New().String()
	ch := make(chan bool, 1) // Buffered to prevent blocking if sender is fast/receiver slow (though usually 1-1)

	s.mu.Lock()
	s.pendingRequests[id] = ch
	s.mu.Unlock()

	log.Printf("Created new approval request: %s", id)
	return id, ch
}

// ResolveRequest resolves a pending request with the given approval status.
// It returns true if the request was found and resolved, false otherwise.
func (s *Service) ResolveRequest(reqID string, approved bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch, exists := s.pendingRequests[reqID]
	if !exists {
		log.Printf("Attempted to resolve unknown request: %s", reqID)
		return false
	}

	// Non-blocking send in case the receiver has already given up (timeout), 
	// though for this use case blocking for a bit is usually fine.
	select {
	case ch <- approved:
		log.Printf("Resolved request %s with status: %v", reqID, approved)
	case <-time.After(1 * time.Second):
		log.Printf("Timeout sending to request channel %s", reqID)
	}

	close(ch)
	delete(s.pendingRequests, reqID)
	return true
}
