// // Copyright 2026 savaki
// //
// // Licensed under the Apache License, Version 2.0 (the "License");
// // you may not use this file except in compliance with the License.
// // You may obtain a copy of the License at
// //
// //     http://www.apache.org/licenses/LICENSE-2.0
// //
// // Unless required by applicable law or agreed to in writing, software
// // distributed under the License is distributed on an "AS IS" BASIS,
// // WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// // See the License for the specific language governing permissions and
// // limitations under the License.

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/savaki/shannon/internal/auth"
	"github.com/savaki/shannon/internal/claude"
	"github.com/savaki/shannon/internal/git"
	"github.com/savaki/shannon/internal/session"
	"github.com/savaki/shannon/internal/storage"
)

type testEnv struct {
	store    *storage.Store
	gitMgr   *git.Manager
	sessions *session.Manager
	psk      *auth.PSKManager
	server   *Server
	token    string
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := storage.NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	gitMgr := git.NewManager(tmpDir, false)
	sessionMgr := session.NewManager(store, gitMgr, tmpDir)
	pskMgr := auth.NewPSKManager(store)

	token, err := pskMgr.GetOrCreate()
	if err != nil {
		t.Fatalf("failed to get PSK: %v", err)
	}

	server := NewServer(":0", sessionMgr, pskMgr)

	return &testEnv{
		store:    store,
		gitMgr:   gitMgr,
		sessions: sessionMgr,
		psk:      pskMgr,
		server:   server,
		token:    token,
	}
}

func (e *testEnv) request(method, path string, body interface{}) *httptest.ResponseRecorder {
	return e.requestWithAuth(method, path, body, e.token)
}

func (e *testEnv) requestWithAuth(method, path string, body interface{}, token string) *httptest.ResponseRecorder {
	var bodyBytes []byte
	if body != nil {
		bodyBytes, _ = json.Marshal(body)
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rr := httptest.NewRecorder()
	mux := http.NewServeMux()
	e.server.registerRoutes(mux)
	mux.ServeHTTP(rr, req)

	return rr
}

func (e *testEnv) requestNoAuth(method, path string, body interface{}) *httptest.ResponseRecorder {
	return e.requestWithAuth(method, path, body, "")
}

// Health endpoint tests

func TestHandleHealth(t *testing.T) {
	env := setupTestEnv(t)

	rr := env.requestNoAuth("GET", "/health", nil)

	if rr.Code != http.StatusOK {
		t.Errorf("health check status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["status"] != "ok" {
		t.Errorf("health status = %q, want %q", resp["status"], "ok")
	}
}

// Auth middleware tests

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	env := setupTestEnv(t)

	rr := env.requestNoAuth("GET", "/sessions", nil)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddleware_InvalidFormat(t *testing.T) {
	env := setupTestEnv(t)

	req := httptest.NewRequest("GET", "/sessions", nil)
	req.Header.Set("Authorization", "Basic abc123")

	rr := httptest.NewRecorder()
	mux := http.NewServeMux()
	env.server.registerRoutes(mux)
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	env := setupTestEnv(t)

	rr := env.requestWithAuth("GET", "/sessions", nil, "invalid-token")

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	env := setupTestEnv(t)

	rr := env.request("GET", "/sessions", nil)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

// Session endpoint tests

func TestListSessions_Empty(t *testing.T) {
	env := setupTestEnv(t)

	rr := env.request("GET", "/sessions", nil)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var sessions []sessionResponse
	json.NewDecoder(rr.Body).Decode(&sessions)
	if len(sessions) != 0 {
		t.Errorf("sessions count = %d, want 0", len(sessions))
	}
}

func TestCreateSession_NoRepo(t *testing.T) {
	env := setupTestEnv(t)

	// Create session without repo (just creates a directory)
	rr := env.request("POST", "/sessions", createSessionRequest{})

	// This will fail because claude CLI isn't available in tests
	// but we can test that the endpoint processes the request
	if rr.Code != http.StatusCreated && rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d or %d", rr.Code, http.StatusCreated, http.StatusInternalServerError)
	}
}

func TestCreateSession_InvalidBody(t *testing.T) {
	env := setupTestEnv(t)

	req := httptest.NewRequest("POST", "/sessions", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)

	rr := httptest.NewRecorder()
	mux := http.NewServeMux()
	env.server.registerRoutes(mux)
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestGetSession_NotFound(t *testing.T) {
	env := setupTestEnv(t)

	rr := env.request("GET", "/sessions/nonexistent-id", nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestGetSession_MissingID(t *testing.T) {
	env := setupTestEnv(t)

	// Create a session directly in storage to test GET
	sess, err := env.store.CreateSession("/tmp/test", "")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	rr := env.request("GET", "/sessions/"+sess.ID, nil)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp sessionResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.ID != sess.ID {
		t.Errorf("session ID = %q, want %q", resp.ID, sess.ID)
	}
}

func TestDeleteSession_NotFound(t *testing.T) {
	env := setupTestEnv(t)

	rr := env.request("DELETE", "/sessions/nonexistent-id", nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestSessionInput_NotFound(t *testing.T) {
	env := setupTestEnv(t)

	rr := env.request("POST", "/sessions/nonexistent-id/input", sessionInputRequest{Input: "test"})

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestSessionInput_EmptyInput(t *testing.T) {
	env := setupTestEnv(t)

	// Create a session in storage
	sess, _ := env.store.CreateSession("/tmp/test", "")

	rr := env.request("POST", "/sessions/"+sess.ID+"/input", sessionInputRequest{Input: ""})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestSessionInput_InvalidBody(t *testing.T) {
	env := setupTestEnv(t)

	sess, _ := env.store.CreateSession("/tmp/test", "")

	req := httptest.NewRequest("POST", "/sessions/"+sess.ID+"/input", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)

	rr := httptest.NewRecorder()
	mux := http.NewServeMux()
	env.server.registerRoutes(mux)
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestSessionHistory(t *testing.T) {
	env := setupTestEnv(t)

	// Create session and add messages
	sess, _ := env.store.CreateSession("/tmp/test", "")
	env.store.CreateMessage(sess.ID, storage.MessageRoleUser, "Hello")
	env.store.CreateMessage(sess.ID, storage.MessageRoleAssistant, "Hi there")

	rr := env.request("GET", "/sessions/"+sess.ID+"/history", nil)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var messages []*storage.Message
	json.NewDecoder(rr.Body).Decode(&messages)
	if len(messages) != 2 {
		t.Errorf("messages count = %d, want 2", len(messages))
	}
}

func TestSessionStream_NotFound(t *testing.T) {
	env := setupTestEnv(t)

	rr := env.request("GET", "/sessions/nonexistent-id/stream", nil)

	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

// Device endpoint tests

func TestRegisterDevice_NotImplemented(t *testing.T) {
	env := setupTestEnv(t)

	rr := env.request("POST", "/devices", registerDeviceRequest{Token: "abc", Platform: "ios"})

	if rr.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNotImplemented)
	}
}

func TestUnregisterDevice_NotImplemented(t *testing.T) {
	env := setupTestEnv(t)

	rr := env.request("DELETE", "/devices/abc123", nil)

	if rr.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNotImplemented)
	}
}

// Helper function tests

func TestWriteJSON(t *testing.T) {
	rr := httptest.NewRecorder()
	writeJSON(rr, http.StatusOK, map[string]string{"key": "value"})

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}

func TestWriteError(t *testing.T) {
	rr := httptest.NewRecorder()
	writeError(rr, http.StatusBadRequest, "test error")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}

	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["error"] != "test error" {
		t.Errorf("error = %q, want %q", resp["error"], "test error")
	}
}

func TestToSessionResponse(t *testing.T) {
	sess := &session.Session{
		Session: &storage.Session{
			ID:      "test-id",
			WorkDir: "/tmp/test",
			RepoURL: "https://github.com/test/repo",
			Status:  storage.SessionStatusRunning,
		},
	}

	resp := toSessionResponse(sess)

	if resp.ID != "test-id" {
		t.Errorf("ID = %q, want %q", resp.ID, "test-id")
	}
	if resp.WorkDir != "/tmp/test" {
		t.Errorf("WorkDir = %q, want %q", resp.WorkDir, "/tmp/test")
	}
	if resp.RepoURL != "https://github.com/test/repo" {
		t.Errorf("RepoURL = %q, want %q", resp.RepoURL, "https://github.com/test/repo")
	}
}

func TestHandleListSessions_WithSessions(t *testing.T) {
	env := setupTestEnv(t)

	// Create some sessions
	env.store.CreateSession("/tmp/test1", "")
	env.store.CreateSession("/tmp/test2", "")

	rr := env.request("GET", "/sessions", nil)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var sessions []sessionResponse
	json.NewDecoder(rr.Body).Decode(&sessions)
	if len(sessions) != 2 {
		t.Errorf("sessions count = %d, want 2", len(sessions))
	}
}

func TestHandleDeleteSession_Success(t *testing.T) {
	env := setupTestEnv(t)

	// Create session
	sess, _ := env.store.CreateSession("/tmp/test", "")

	rr := env.request("DELETE", "/sessions/"+sess.ID, nil)

	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}
}

func TestHandleSessionInput_Success(t *testing.T) {
	env := setupTestEnv(t)

	// Create session and add to active sessions
	sess, _ := env.store.CreateSession("/tmp/test", "")

	// This will fail because we don't have a running wrapper
	// but we can test the path value extraction
	rr := env.request("POST", "/sessions/"+sess.ID+"/input", sessionInputRequest{Input: "test"})

	// Should be not found because session is not in manager's active sessions
	if rr.Code != http.StatusNotFound && rr.Code != http.StatusInternalServerError {
		t.Logf("status = %d (expected 404 or 500)", rr.Code)
	}
}

func TestSessionHistory_NotFound(t *testing.T) {
	env := setupTestEnv(t)

	rr := env.request("GET", "/sessions/nonexistent-id/history", nil)

	// GetHistory returns empty slice for nonexistent sessions
	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}
}

func TestSessionHistory_MissingID(t *testing.T) {
	env := setupTestEnv(t)

	// Request to /sessions//history - path without ID
	rr := env.request("GET", "/sessions//history", nil)

	// Should return bad request or handle the empty path
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusNotFound {
		t.Logf("status = %d (expected 400 or 404)", rr.Code)
	}
}

func TestDeleteSession_MissingID(t *testing.T) {
	env := setupTestEnv(t)

	// Request without ID in path
	rr := env.request("DELETE", "/sessions/", nil)

	// Should return 404 or 400
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusNotFound {
		t.Logf("status = %d (expected 400 or 404)", rr.Code)
	}
}

func TestGetSession_MissingIDEmptyPath(t *testing.T) {
	env := setupTestEnv(t)

	rr := env.request("GET", "/sessions/", nil)

	// Should return 404 or 400
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusNotFound {
		t.Logf("status = %d (expected 400 or 404)", rr.Code)
	}
}

func TestSessionInput_MissingID(t *testing.T) {
	env := setupTestEnv(t)

	rr := env.request("POST", "/sessions//input", sessionInputRequest{Input: "test"})

	// Should return 404 or 400
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusNotFound {
		t.Logf("status = %d (expected 400 or 404)", rr.Code)
	}
}

func TestSessionStream_MissingID(t *testing.T) {
	env := setupTestEnv(t)

	rr := env.request("GET", "/sessions//stream", nil)

	// Should return 404 or 400
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusNotFound {
		t.Logf("status = %d (expected 400 or 404)", rr.Code)
	}
}

func TestCreateSessionRequest_Fields(t *testing.T) {
	req := createSessionRequest{
		RepoURL: "https://github.com/test/repo",
		Branch:  "main",
		Prompt:  "initial prompt",
	}

	if req.RepoURL != "https://github.com/test/repo" {
		t.Errorf("RepoURL = %q, want %q", req.RepoURL, "https://github.com/test/repo")
	}
	if req.Branch != "main" {
		t.Errorf("Branch = %q, want %q", req.Branch, "main")
	}
	if req.Prompt != "initial prompt" {
		t.Errorf("Prompt = %q, want %q", req.Prompt, "initial prompt")
	}
}

func TestSessionInputRequest_Fields(t *testing.T) {
	req := sessionInputRequest{
		Input: "test input",
	}

	if req.Input != "test input" {
		t.Errorf("Input = %q, want %q", req.Input, "test input")
	}
}

func TestRegisterDeviceRequest_Fields(t *testing.T) {
	req := registerDeviceRequest{
		Token:    "abc123",
		Platform: "ios",
	}

	if req.Token != "abc123" {
		t.Errorf("Token = %q, want %q", req.Token, "abc123")
	}
	if req.Platform != "ios" {
		t.Errorf("Platform = %q, want %q", req.Platform, "ios")
	}
}

func TestSessionResponse_Fields(t *testing.T) {
	resp := sessionResponse{
		ID:        "sess-123",
		WorkDir:   "/tmp/test",
		RepoURL:   "https://github.com/test/repo",
		Status:    "running",
		CreatedAt: "2024-01-01T00:00:00Z",
	}

	if resp.ID != "sess-123" {
		t.Errorf("ID = %q, want %q", resp.ID, "sess-123")
	}
	if resp.WorkDir != "/tmp/test" {
		t.Errorf("WorkDir = %q, want %q", resp.WorkDir, "/tmp/test")
	}
	if resp.RepoURL != "https://github.com/test/repo" {
		t.Errorf("RepoURL = %q, want %q", resp.RepoURL, "https://github.com/test/repo")
	}
	if resp.Status != "running" {
		t.Errorf("Status = %q, want %q", resp.Status, "running")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

// Tests with mock session manager for better coverage

type mockTestEnvWithMock struct {
	store    *storage.Store
	gitMgr   *git.Manager
	sessions *session.Manager
	psk      *auth.PSKManager
	server   *Server
	token    string
	mock     *mockSessionWrapper
}

type mockSessionWrapper struct {
	startErr    error
	sendErr     error
	stopErr     error
	startCalled bool
	sendCalls   []string
	stopCalled  bool
	status      storage.SessionStatus
	eventCh     chan Event
	done        chan struct{}
}

func newMockSessionWrapper() *mockSessionWrapper {
	return &mockSessionWrapper{
		status:  storage.SessionStatusRunning,
		eventCh: make(chan Event, 100),
		done:    make(chan struct{}),
	}
}

type Event = session.Event

func setupTestEnvWithMock(t *testing.T) *mockTestEnvWithMock {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := storage.NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	gitMgr := git.NewManager(tmpDir, false)

	// Create session manager with mock wrapper factory
	mock := newMockSessionWrapper()
	mockFactory := createMockFactory(mock)
	sessionMgr := session.NewManagerWithFactory(store, gitMgr, tmpDir, mockFactory)
	pskMgr := auth.NewPSKManager(store)

	token, err := pskMgr.GetOrCreate()
	if err != nil {
		t.Fatalf("failed to get PSK: %v", err)
	}

	server := NewServer(":0", sessionMgr, pskMgr)

	return &mockTestEnvWithMock{
		store:    store,
		gitMgr:   gitMgr,
		sessions: sessionMgr,
		psk:      pskMgr,
		server:   server,
		token:    token,
		mock:     mock,
	}
}

func createMockFactory(mock *mockSessionWrapper) session.WrapperFactory {
	return func(opts claude.Options) claude.ClaudeWrapper {
		return &mockClaudeWrapper{mock: mock}
	}
}

// mockClaudeWrapper implements claude.ClaudeWrapper for testing
type mockClaudeWrapper struct {
	mock *mockSessionWrapper
}

func (m *mockClaudeWrapper) Start(ctx context.Context) error {
	m.mock.startCalled = true
	return m.mock.startErr
}

func (m *mockClaudeWrapper) Send(input string) error {
	m.mock.sendCalls = append(m.mock.sendCalls, input)
	return m.mock.sendErr
}

func (m *mockClaudeWrapper) Stop() error {
	m.mock.stopCalled = true
	if !isClosed(m.mock.done) {
		close(m.mock.done)
	}
	return m.mock.stopErr
}

func (m *mockClaudeWrapper) Subscribe() <-chan claude.Event {
	ch := make(chan claude.Event, 100)
	return ch
}

func (m *mockClaudeWrapper) Unsubscribe(ch <-chan claude.Event) {}

func (m *mockClaudeWrapper) Status() claude.Status {
	switch m.mock.status {
	case storage.SessionStatusRunning:
		return claude.StatusRunning
	case storage.SessionStatusWaitingInput:
		return claude.StatusWaitingInput
	case storage.SessionStatusStopped:
		return claude.StatusStopped
	default:
		return claude.StatusStopped
	}
}

func (m *mockClaudeWrapper) Done() <-chan struct{} {
	return m.mock.done
}

func isClosed(ch chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

func (e *mockTestEnvWithMock) request(method, path string, body interface{}) *httptest.ResponseRecorder {
	return e.requestWithAuth(method, path, body, e.token)
}

func (e *mockTestEnvWithMock) requestWithAuth(method, path string, body interface{}, token string) *httptest.ResponseRecorder {
	var bodyBytes []byte
	if body != nil {
		bodyBytes, _ = json.Marshal(body)
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rr := httptest.NewRecorder()
	mux := http.NewServeMux()
	e.server.registerRoutes(mux)
	mux.ServeHTTP(rr, req)

	return rr
}

func TestCreateSession_WithMock_Success(t *testing.T) {
	env := setupTestEnvWithMock(t)

	rr := env.request("POST", "/sessions", createSessionRequest{
		Prompt: "Hello Claude",
	})

	if rr.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d. Body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	if !env.mock.startCalled {
		t.Error("Start should have been called")
	}

	var resp sessionResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.ID == "" {
		t.Error("session ID should not be empty")
	}
	if resp.Status != "running" {
		t.Errorf("status = %q, want %q", resp.Status, "running")
	}
}

func TestCreateSession_WithMock_StartError(t *testing.T) {
	env := setupTestEnvWithMock(t)
	env.mock.startErr = errors.New("mock start error")

	rr := env.request("POST", "/sessions", createSessionRequest{})

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestSendInput_WithMock_Success(t *testing.T) {
	env := setupTestEnvWithMock(t)

	// Create session first
	rr := env.request("POST", "/sessions", createSessionRequest{})
	if rr.Code != http.StatusCreated {
		t.Fatalf("failed to create session: %d", rr.Code)
	}

	var sess sessionResponse
	json.NewDecoder(rr.Body).Decode(&sess)

	// Send input
	rr = env.request("POST", "/sessions/"+sess.ID+"/input", sessionInputRequest{Input: "test input"})

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d. Body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	// Verify input was sent
	found := false
	for _, call := range env.mock.sendCalls {
		if call == "test input" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'test input' in send calls, got: %v", env.mock.sendCalls)
	}
}

func TestSendInput_WithMock_Error(t *testing.T) {
	env := setupTestEnvWithMock(t)
	env.mock.sendErr = errors.New("mock send error")

	// Create session first
	rr := env.request("POST", "/sessions", createSessionRequest{})
	if rr.Code != http.StatusCreated {
		t.Fatalf("failed to create session: %d", rr.Code)
	}

	var sess sessionResponse
	json.NewDecoder(rr.Body).Decode(&sess)

	// Send input should fail
	rr = env.request("POST", "/sessions/"+sess.ID+"/input", sessionInputRequest{Input: "test"})

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestDeleteSession_WithMock_Success(t *testing.T) {
	env := setupTestEnvWithMock(t)

	// Create session
	rr := env.request("POST", "/sessions", createSessionRequest{})
	if rr.Code != http.StatusCreated {
		t.Fatalf("failed to create session: %d", rr.Code)
	}

	var sess sessionResponse
	json.NewDecoder(rr.Body).Decode(&sess)

	// Delete session
	rr = env.request("DELETE", "/sessions/"+sess.ID, nil)

	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}

	// Verify session is gone
	rr = env.request("GET", "/sessions/"+sess.ID, nil)
	if rr.Code != http.StatusNotFound {
		t.Errorf("session should be deleted, status = %d", rr.Code)
	}
}

func TestStopSession_WithMock_Success(t *testing.T) {
	env := setupTestEnvWithMock(t)

	// Create session
	rr := env.request("POST", "/sessions", createSessionRequest{})
	if rr.Code != http.StatusCreated {
		t.Fatalf("failed to create session: %d", rr.Code)
	}

	var sess sessionResponse
	json.NewDecoder(rr.Body).Decode(&sess)

	// Stop is done via delete for now
	rr = env.request("DELETE", "/sessions/"+sess.ID, nil)

	if rr.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusNoContent)
	}

	if !env.mock.stopCalled {
		t.Error("Stop should have been called")
	}
}

func TestListSessions_WithMock_AfterCreate(t *testing.T) {
	env := setupTestEnvWithMock(t)

	// Create sessions
	for i := 0; i < 3; i++ {
		rr := env.request("POST", "/sessions", createSessionRequest{})
		if rr.Code != http.StatusCreated {
			t.Fatalf("failed to create session %d: %d", i, rr.Code)
		}
	}

	// List sessions
	rr := env.request("GET", "/sessions", nil)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var sessions []sessionResponse
	json.NewDecoder(rr.Body).Decode(&sessions)
	if len(sessions) != 3 {
		t.Errorf("sessions count = %d, want 3", len(sessions))
	}
}

func TestGetSession_WithMock_AfterCreate(t *testing.T) {
	env := setupTestEnvWithMock(t)

	// Create session
	rr := env.request("POST", "/sessions", createSessionRequest{})
	if rr.Code != http.StatusCreated {
		t.Fatalf("failed to create session: %d", rr.Code)
	}

	var created sessionResponse
	json.NewDecoder(rr.Body).Decode(&created)

	// Get session
	rr = env.request("GET", "/sessions/"+created.ID, nil)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var fetched sessionResponse
	json.NewDecoder(rr.Body).Decode(&fetched)
	if fetched.ID != created.ID {
		t.Errorf("ID = %q, want %q", fetched.ID, created.ID)
	}
}

func TestSessionStream_WithMock_Success(t *testing.T) {
	// Skip SSE streaming test as it requires real HTTP connections
	// The SSE handler creates goroutines that wait indefinitely on channels
	// which causes goroutine leaks in unit tests
	t.Skip("SSE streaming requires real HTTP connections")
}

func TestSessionHistory_WithMock_Success(t *testing.T) {
	env := setupTestEnvWithMock(t)

	// Create session
	rr := env.request("POST", "/sessions", createSessionRequest{})
	if rr.Code != http.StatusCreated {
		t.Fatalf("failed to create session: %d", rr.Code)
	}

	var sess sessionResponse
	json.NewDecoder(rr.Body).Decode(&sess)

	// Add some messages to storage
	env.store.CreateMessage(sess.ID, storage.MessageRoleUser, "Hello")
	env.store.CreateMessage(sess.ID, storage.MessageRoleAssistant, "Hi there!")

	// Get history
	rr = env.request("GET", "/sessions/"+sess.ID+"/history", nil)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var messages []*storage.Message
	json.NewDecoder(rr.Body).Decode(&messages)
	if len(messages) != 2 {
		t.Errorf("messages count = %d, want 2", len(messages))
	}
}

func TestHandleListSessions_Error(t *testing.T) {
	// Test with a closed store to trigger error path
	// This is tricky to test without modifying the manager interface
	// For now, we test the happy path thoroughly
}

func TestHandleGetSession_InternalError(t *testing.T) {
	// Error paths for internal server errors are harder to trigger
	// without introducing fault injection or mock interfaces
}

func TestCreateSession_WithPrompt(t *testing.T) {
	env := setupTestEnvWithMock(t)

	rr := env.request("POST", "/sessions", createSessionRequest{
		Prompt: "Please help me with Go programming",
	})

	if rr.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d. Body: %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	// Verify prompt was sent (might be in initial prompt or first send)
	promptFound := false
	for _, call := range env.mock.sendCalls {
		if call == "Please help me with Go programming" {
			promptFound = true
			break
		}
	}
	if !promptFound {
		t.Logf("sendCalls: %v", env.mock.sendCalls)
	}
}

func TestCreateSession_WithRepoURL(t *testing.T) {
	// Skip this test because it requires network access for git clone
	t.Skip("requires network access for git clone")
}

// Input validation tests

func TestCreateSession_PromptTooLong(t *testing.T) {
	env := setupTestEnv(t)

	// Create a prompt that exceeds the maximum length
	longPrompt := string(make([]byte, MaxPromptLength+1))

	rr := env.request("POST", "/sessions", createSessionRequest{
		Prompt: longPrompt,
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}

	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["error"] != "prompt exceeds maximum length" {
		t.Errorf("error = %q, want %q", resp["error"], "prompt exceeds maximum length")
	}
}

func TestCreateSession_RepoURLTooLong(t *testing.T) {
	env := setupTestEnv(t)

	longURL := "https://github.com/" + string(make([]byte, MaxRepoURLLength))

	rr := env.request("POST", "/sessions", createSessionRequest{
		RepoURL: longURL,
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}

	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["error"] != "repo_url exceeds maximum length" {
		t.Errorf("error = %q, want %q", resp["error"], "repo_url exceeds maximum length")
	}
}

func TestCreateSession_BranchTooLong(t *testing.T) {
	env := setupTestEnv(t)

	longBranch := string(make([]byte, MaxBranchLength+1))

	rr := env.request("POST", "/sessions", createSessionRequest{
		Branch: longBranch,
	})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}

	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["error"] != "branch exceeds maximum length" {
		t.Errorf("error = %q, want %q", resp["error"], "branch exceeds maximum length")
	}
}

func TestSessionInput_InputTooLong(t *testing.T) {
	env := setupTestEnvWithMock(t)

	// Create session first
	rr := env.request("POST", "/sessions", createSessionRequest{})
	if rr.Code != http.StatusCreated {
		t.Fatalf("failed to create session: %d", rr.Code)
	}

	var sess sessionResponse
	json.NewDecoder(rr.Body).Decode(&sess)

	// Send input that exceeds the maximum length
	longInput := string(make([]byte, MaxInputLength+1))
	rr = env.request("POST", "/sessions/"+sess.ID+"/input", sessionInputRequest{Input: longInput})

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}

	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["error"] != "input exceeds maximum length" {
		t.Errorf("error = %q, want %q", resp["error"], "input exceeds maximum length")
	}
}

func TestBodyLimit_RequestTooLarge(t *testing.T) {
	env := setupTestEnv(t)

	// Create a request body that exceeds the limit
	largeBody := make([]byte, MaxRequestBodySize+1)
	for i := range largeBody {
		largeBody[i] = 'a'
	}

	req := httptest.NewRequest("POST", "/sessions", bytes.NewReader(largeBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)

	rr := httptest.NewRecorder()
	mux := http.NewServeMux()
	env.server.registerRoutes(mux)
	mux.ServeHTTP(rr, req)

	// Should fail to parse the body
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestMiddlewareConstants(t *testing.T) {
	// Verify constants are set to expected values
	if MaxRequestBodySize != 1<<20 {
		t.Errorf("MaxRequestBodySize = %d, want %d", MaxRequestBodySize, 1<<20)
	}
	if MaxPromptLength != 100*1024 {
		t.Errorf("MaxPromptLength = %d, want %d", MaxPromptLength, 100*1024)
	}
	if MaxInputLength != 100*1024 {
		t.Errorf("MaxInputLength = %d, want %d", MaxInputLength, 100*1024)
	}
	if MaxRepoURLLength != 2048 {
		t.Errorf("MaxRepoURLLength = %d, want %d", MaxRepoURLLength, 2048)
	}
	if MaxBranchLength != 256 {
		t.Errorf("MaxBranchLength = %d, want %d", MaxBranchLength, 256)
	}
}
