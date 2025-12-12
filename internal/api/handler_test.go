package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"zfs-unlocker/internal/approval"
	"zfs-unlocker/internal/config"

	"github.com/gin-gonic/gin"
)

// --- Mocks ---

type MockNotifier struct {
	CapturedReqID string
}

func (m *MockNotifier) RequestApproval(reqID, description string) error {
	m.CapturedReqID = reqID
	return nil
}

type MockVault struct {
	SecretToReturn map[string]interface{}
	ErrToReturn    error
}

func (m *MockVault) GetSecret(ctx context.Context, keyPrefix, volumeID string) (map[string]interface{}, error) {
	if m.ErrToReturn != nil {
		return nil, m.ErrToReturn
	}
	return m.SecretToReturn, nil
}

// --- Tests ---

func TestHandler_Auth_MissingKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup dependencies
	approvalSvc := approval.New()
	mockBot := &MockNotifier{}
	mockVault := &MockVault{}

	// Empty config
	handler := New([]config.APIKey{}, approvalSvc, mockVault, mockBot)

	r := gin.New()
	handler.RegisterRoutes(r)

	req, _ := http.NewRequest("GET", "/unlock/missing/vol1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized, got %d", w.Code)
	}
}

func TestHandler_Unlock_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup dependencies
	approvalSvc := approval.New()
	mockBot := &MockNotifier{}
	mockVault := &MockVault{
		// Base64 for "top-secret-zfs-key" is "dG9wLXNlY3JldC16ZnMta2V5"
		SecretToReturn: map[string]interface{}{"key": "dG9wLXNlY3JldC16ZnMta2V5"},
	}

	keys := []config.APIKey{
		{Key: "test-key", PathPrefix: "server-1"},
	}

	handler := New(keys, approvalSvc, mockVault, mockBot)

	r := gin.New()
	handler.RegisterRoutes(r)

	// Start request in goroutine because it blocks waiting for approval
	done := make(chan bool)
	w := httptest.NewRecorder()

	go func() {
		req, _ := http.NewRequest("GET", "/unlock/test-key/vol-data", nil)
		r.ServeHTTP(w, req)
		close(done)
	}()

	// Wait for the request to register and bot to be called
	// In a real test we might need a small sleep or a better synchronization mechanism (like checking len(pendingRequests))
	// Since we are using a real approval service, we can poll it via public API or just wait briefly.
	time.Sleep(50 * time.Millisecond)

	// Check if bot got a request ID
	if mockBot.CapturedReqID == "" {
		t.Fatal("Bot was not called with a request ID")
	}

	// Approve the request
	success := approvalSvc.ResolveRequest(mockBot.CapturedReqID, true)
	if !success {
		t.Fatal("Failed to resolve request (maybe ID mismatch?)")
	}

	// Wait for HTTP handler to finish
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("HTTP handler timed out")
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d. Body: %s", w.Code, w.Body.String())
	}

	// The handler returns raw bytes (decoded from Base64)
	if w.Body.String() != "top-secret-zfs-key" {
		t.Errorf("Expected body 'top-secret-zfs-key', got '%s'", w.Body.String())
	}
}

func TestHandler_Unlock_Deny(t *testing.T) {
	gin.SetMode(gin.TestMode)

	approvalSvc := approval.New()
	mockBot := &MockNotifier{}
	mockVault := &MockVault{}

	keys := []config.APIKey{{Key: "test-key"}}
	handler := New(keys, approvalSvc, mockVault, mockBot)

	r := gin.New()
	handler.RegisterRoutes(r)

	done := make(chan bool)
	w := httptest.NewRecorder()

	go func() {
		req, _ := http.NewRequest("GET", "/unlock/test-key/vol-data", nil)
		r.ServeHTTP(w, req)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	if mockBot.CapturedReqID == "" {
		t.Fatal("Bot was not called")
	}

	// Deny the request
	approvalSvc.ResolveRequest(mockBot.CapturedReqID, false)

	<-done

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden, got %d", w.Code)
	}
}

func TestHandler_Unlock_AlwaysRaw(t *testing.T) {
	gin.SetMode(gin.TestMode)
	approvalSvc := approval.New()
	mockBot := &MockNotifier{}
	// Base64 for "hello" is "aGVsbG8="
	mockVault := &MockVault{
		SecretToReturn: map[string]interface{}{"key": "aGVsbG8="},
	}
	keys := []config.APIKey{{Key: "test-key"}}
	handler := New(keys, approvalSvc, mockVault, mockBot)

	r := gin.New()
	handler.RegisterRoutes(r)
	done := make(chan bool)
	w := httptest.NewRecorder()

	go func() {
		// No query param needed, should default to raw decoding
		req, _ := http.NewRequest("GET", "/unlock/test-key/vol-raw", nil)
		r.ServeHTTP(w, req)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	approvalSvc.ResolveRequest(mockBot.CapturedReqID, true)
	<-done

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", w.Code)
	}
	if w.Body.String() != "hello" {
		t.Errorf("Expected decoded body 'hello', got '%s'", w.Body.String())
	}
}
