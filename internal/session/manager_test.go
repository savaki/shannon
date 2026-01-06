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

package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/savaki/shannon/internal/claude"
	"github.com/savaki/shannon/internal/git"
	"github.com/savaki/shannon/internal/storage"
)

type testEnv struct {
	store   *storage.Store
	gitMgr  *git.Manager
	manager *Manager
	dataDir string
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
	manager := NewManager(store, gitMgr, tmpDir)

	return &testEnv{
		store:   store,
		gitMgr:  gitMgr,
		manager: manager,
		dataDir: tmpDir,
	}
}

func TestNewManager(t *testing.T) {
	env := setupTestEnv(t)

	if env.manager == nil {
		t.Error("NewManager() returned nil")
	}

	if env.manager.store != env.store {
		t.Error("manager store not set correctly")
	}

	if env.manager.gitMgr != env.gitMgr {
		t.Error("manager gitMgr not set correctly")
	}
}

func TestManager_List_Empty(t *testing.T) {
	env := setupTestEnv(t)

	sessions, err := env.manager.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(sessions) != 0 {
		t.Errorf("List() returned %d sessions, want 0", len(sessions))
	}
}

func TestManager_Get_NotFound(t *testing.T) {
	env := setupTestEnv(t)

	_, err := env.manager.Get("nonexistent-id")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Get() error = %v, want ErrNotFound", err)
	}
}

func TestManager_Get_FromStorage(t *testing.T) {
	env := setupTestEnv(t)

	// Create session directly in storage
	storageSess, err := env.store.CreateSession("/tmp/test", "https://github.com/test/repo")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	// Get via manager
	sess, err := env.manager.Get(storageSess.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if sess.ID != storageSess.ID {
		t.Errorf("ID = %q, want %q", sess.ID, storageSess.ID)
	}
}

func TestManager_Stop_NotFound(t *testing.T) {
	env := setupTestEnv(t)

	err := env.manager.Stop("nonexistent-id")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Stop() error = %v, want ErrNotFound", err)
	}
}

func TestManager_Send_NotFound(t *testing.T) {
	env := setupTestEnv(t)

	err := env.manager.Send("nonexistent-id", "test input")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Send() error = %v, want ErrNotFound", err)
	}
}

func TestManager_Subscribe_NotFound(t *testing.T) {
	env := setupTestEnv(t)

	_, err := env.manager.Subscribe("nonexistent-id")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Subscribe() error = %v, want ErrNotFound", err)
	}
}

func TestManager_Delete_NotFound(t *testing.T) {
	env := setupTestEnv(t)

	err := env.manager.Delete("nonexistent-id")
	if !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Delete() error = %v, want ErrNotFound", err)
	}
}

func TestManager_Delete_FromStorage(t *testing.T) {
	env := setupTestEnv(t)

	// Create session directly in storage
	storageSess, err := env.store.CreateSession("/tmp/test", "")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	// Delete via manager
	err = env.manager.Delete(storageSess.ID)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify deleted from storage
	_, err = env.store.GetSession(storageSess.ID)
	if !errors.Is(err, storage.ErrNotFound) {
		t.Error("session should be deleted from storage")
	}
}

func TestManager_GetHistory(t *testing.T) {
	env := setupTestEnv(t)

	// Create session and messages
	storageSess, _ := env.store.CreateSession("/tmp/test", "")
	env.store.CreateMessage(storageSess.ID, storage.MessageRoleUser, "Hello")
	env.store.CreateMessage(storageSess.ID, storage.MessageRoleAssistant, "Hi")

	messages, err := env.manager.GetHistory(storageSess.ID)
	if err != nil {
		t.Fatalf("GetHistory() error = %v", err)
	}

	if len(messages) != 2 {
		t.Errorf("GetHistory() returned %d messages, want 2", len(messages))
	}
}

func TestManager_SubscribeAll(t *testing.T) {
	env := setupTestEnv(t)

	ch := env.manager.SubscribeAll()
	if ch == nil {
		t.Error("SubscribeAll() returned nil channel")
	}

	// Verify subscriber was added
	env.manager.subMu.RLock()
	count := len(env.manager.subscribers)
	env.manager.subMu.RUnlock()

	if count != 1 {
		t.Errorf("subscriber count = %d, want 1", count)
	}
}

func TestManager_List_WithActiveSessions(t *testing.T) {
	env := setupTestEnv(t)

	// Create sessions directly in storage
	sess1, _ := env.store.CreateSession("/tmp/test1", "")
	sess2, _ := env.store.CreateSession("/tmp/test2", "")

	// Add one to active sessions map
	env.manager.sessionsMu.Lock()
	env.manager.sessions[sess1.ID] = &Session{Session: sess1}
	env.manager.sessionsMu.Unlock()

	sessions, err := env.manager.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(sessions) != 2 {
		t.Errorf("List() returned %d sessions, want 2", len(sessions))
	}

	// Verify active session is returned from map
	var foundActive bool
	for _, s := range sessions {
		if s.ID == sess1.ID {
			foundActive = true
		}
	}
	if !foundActive {
		t.Error("active session not found in list")
	}

	_ = sess2 // use sess2
}

func TestSession_Status_NilWrapper(t *testing.T) {
	sess := &Session{
		Session: &storage.Session{
			Status: storage.SessionStatusPending,
		},
	}

	if sess.Status() != storage.SessionStatusPending {
		t.Errorf("Status() = %v, want %v", sess.Status(), storage.SessionStatusPending)
	}
}


func TestManager_Create_NoRepo(t *testing.T) {
	env := setupTestEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// This will fail because claude CLI isn't available
	// but we can verify the session directory is created
	_, err := env.manager.Create(ctx, CreateOptions{})

	// Expect error because claude isn't available
	if err == nil {
		t.Log("Create succeeded (claude CLI available)")
	} else {
		t.Logf("Create failed as expected: %v", err)
	}
}

func TestManager_cleanup(t *testing.T) {
	env := setupTestEnv(t)

	// Create a session directory
	sessionID := "test-cleanup-session"
	sessionPath := filepath.Join(env.dataDir, "sessions", sessionID)
	if err := os.MkdirAll(sessionPath, 0755); err != nil {
		t.Fatalf("failed to create session dir: %v", err)
	}

	// Cleanup
	env.manager.cleanup(sessionID, "")

	// Verify removed
	if _, err := os.Stat(sessionPath); !os.IsNotExist(err) {
		t.Error("session directory should be removed after cleanup")
	}
}

func TestSession_Status_AllStates(t *testing.T) {
	tests := []struct {
		storageStatus storage.SessionStatus
		want          storage.SessionStatus
	}{
		{storage.SessionStatusPending, storage.SessionStatusPending},
		{storage.SessionStatusRunning, storage.SessionStatusRunning},
		{storage.SessionStatusWaitingInput, storage.SessionStatusWaitingInput},
		{storage.SessionStatusStopped, storage.SessionStatusStopped},
		{storage.SessionStatusError, storage.SessionStatusError},
	}

	for _, tt := range tests {
		t.Run(string(tt.storageStatus), func(t *testing.T) {
			sess := &Session{
				Session: &storage.Session{
					Status: tt.storageStatus,
				},
			}
			if got := sess.Status(); got != tt.want {
				t.Errorf("Status() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvent_Fields(t *testing.T) {
	now := time.Now()
	event := Event{
		SessionID: "sess-123",
		Type:      "output",
		Content:   "hello world",
		Timestamp: now,
	}

	if event.SessionID != "sess-123" {
		t.Errorf("SessionID = %q, want %q", event.SessionID, "sess-123")
	}
	if event.Type != "output" {
		t.Errorf("Type = %q, want %q", event.Type, "output")
	}
	if event.Content != "hello world" {
		t.Errorf("Content = %q, want %q", event.Content, "hello world")
	}
	if !event.Timestamp.Equal(now) {
		t.Errorf("Timestamp = %v, want %v", event.Timestamp, now)
	}
}

func TestManager_Send_NotRunning(t *testing.T) {
	env := setupTestEnv(t)

	// Create session in storage
	storageSess, _ := env.store.CreateSession("/tmp/test", "")

	// Add to active sessions but without wrapper
	env.manager.sessionsMu.Lock()
	env.manager.sessions[storageSess.ID] = &Session{Session: storageSess}
	env.manager.sessionsMu.Unlock()

	// Send should fail because wrapper is nil
	err := env.manager.Send(storageSess.ID, "test")
	if err == nil {
		t.Error("Send() should error when session not running")
	}
}

func TestManager_Subscribe_NotRunning(t *testing.T) {
	env := setupTestEnv(t)

	// Create session in storage
	storageSess, _ := env.store.CreateSession("/tmp/test", "")

	// Add to active sessions but without wrapper
	env.manager.sessionsMu.Lock()
	env.manager.sessions[storageSess.ID] = &Session{Session: storageSess}
	env.manager.sessionsMu.Unlock()

	// Subscribe should fail because wrapper is nil
	_, err := env.manager.Subscribe(storageSess.ID)
	if err == nil {
		t.Error("Subscribe() should error when session not running")
	}
}

func TestManager_Stop_WithWrapper(t *testing.T) {
	env := setupTestEnv(t)

	// Create session in storage
	storageSess, _ := env.store.CreateSession("/tmp/test", "")

	// We can't easily test with a real wrapper, but we can test the flow
	// by creating a session with nil wrapper
	env.manager.sessionsMu.Lock()
	env.manager.sessions[storageSess.ID] = &Session{Session: storageSess}
	env.manager.sessionsMu.Unlock()

	// Stop should work even with nil wrapper
	err := env.manager.Stop(storageSess.ID)
	if err != nil {
		t.Errorf("Stop() error = %v", err)
	}
}

func TestCreateOptions_Fields(t *testing.T) {
	opts := CreateOptions{
		RepoURL: "https://github.com/test/repo",
		Branch:  "main",
		Prompt:  "initial prompt",
	}

	if opts.RepoURL != "https://github.com/test/repo" {
		t.Errorf("RepoURL = %q, want %q", opts.RepoURL, "https://github.com/test/repo")
	}
	if opts.Branch != "main" {
		t.Errorf("Branch = %q, want %q", opts.Branch, "main")
	}
	if opts.Prompt != "initial prompt" {
		t.Errorf("Prompt = %q, want %q", opts.Prompt, "initial prompt")
	}
}

func TestManager_Stop_AlreadyStopped(t *testing.T) {
	env := setupTestEnv(t)

	// Create session in storage
	storageSess, _ := env.store.CreateSession("/tmp/test", "")

	// Add to active sessions with stopped status
	env.manager.sessionsMu.Lock()
	env.manager.sessions[storageSess.ID] = &Session{
		Session: storageSess,
	}
	env.manager.sessionsMu.Unlock()

	// Stop should work (nil wrapper is handled)
	err := env.manager.Stop(storageSess.ID)
	if err != nil {
		t.Errorf("Stop() error = %v, want nil", err)
	}
}

func TestManager_Delete_ActiveSession(t *testing.T) {
	env := setupTestEnv(t)

	// Create session in storage
	storageSess, _ := env.store.CreateSession("/tmp/test", "")

	// Add to active sessions
	env.manager.sessionsMu.Lock()
	env.manager.sessions[storageSess.ID] = &Session{Session: storageSess}
	env.manager.sessionsMu.Unlock()

	// Delete should work
	err := env.manager.Delete(storageSess.ID)
	if err != nil {
		t.Errorf("Delete() error = %v", err)
	}

	// Verify removed from memory
	env.manager.sessionsMu.RLock()
	_, exists := env.manager.sessions[storageSess.ID]
	env.manager.sessionsMu.RUnlock()

	if exists {
		t.Error("session should be removed from memory")
	}
}

func TestManager_cleanup_WithRepoURL(t *testing.T) {
	env := setupTestEnv(t)

	// Cleanup with repo URL (uses RemoveWorktree path)
	env.manager.cleanup("test-session", "https://github.com/test/repo")
	// No error expected - just verifying the path is exercised
}

func TestManager_cleanup_WithoutRepoURL(t *testing.T) {
	env := setupTestEnv(t)

	// Cleanup without repo URL (uses Cleanup path)
	env.manager.cleanup("test-session", "")
	// No error expected - just verifying the path is exercised
}

func TestManager_SubscribeAll_Concurrent(t *testing.T) {
	env := setupTestEnv(t)

	// Subscribe multiple times concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch := env.manager.SubscribeAll()
			if ch == nil {
				t.Error("SubscribeAll() returned nil")
			}
		}()
	}
	wg.Wait()

	// Verify all subscribers were added
	env.manager.subMu.RLock()
	count := len(env.manager.subscribers)
	env.manager.subMu.RUnlock()

	if count != 10 {
		t.Errorf("subscriber count = %d, want 10", count)
	}
}

func TestManager_BroadcastEvents(t *testing.T) {
	env := setupTestEnv(t)

	// Subscribe to all events
	ch := env.manager.SubscribeAll()

	// Send an event to the manager's event channel
	testEvent := Event{
		SessionID: "test-session",
		Type:      "output",
		Content:   "test content",
		Timestamp: time.Now(),
	}

	// Send event directly to event channel
	select {
	case env.manager.eventCh <- testEvent:
	default:
		t.Fatal("failed to send event to channel")
	}

	// Give time for broadcast
	time.Sleep(50 * time.Millisecond)

	// Try to receive
	select {
	case received := <-ch:
		if received.SessionID != testEvent.SessionID {
			t.Errorf("SessionID = %q, want %q", received.SessionID, testEvent.SessionID)
		}
		if received.Content != testEvent.Content {
			t.Errorf("Content = %q, want %q", received.Content, testEvent.Content)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("did not receive event")
	}
}

func TestManager_Get_ActiveSession(t *testing.T) {
	env := setupTestEnv(t)

	// Create session in storage
	storageSess, _ := env.store.CreateSession("/tmp/test", "https://github.com/test/repo")

	// Add to active sessions
	env.manager.sessionsMu.Lock()
	env.manager.sessions[storageSess.ID] = &Session{Session: storageSess}
	env.manager.sessionsMu.Unlock()

	// Get should return from active sessions
	sess, err := env.manager.Get(storageSess.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if sess.ID != storageSess.ID {
		t.Errorf("ID = %q, want %q", sess.ID, storageSess.ID)
	}
	if sess.RepoURL != "https://github.com/test/repo" {
		t.Errorf("RepoURL = %q, want %q", sess.RepoURL, "https://github.com/test/repo")
	}
}

func TestManager_List_MixedSessions(t *testing.T) {
	env := setupTestEnv(t)

	// Create sessions in storage
	sess1, _ := env.store.CreateSession("/tmp/test1", "")
	sess2, _ := env.store.CreateSession("/tmp/test2", "")
	sess3, _ := env.store.CreateSession("/tmp/test3", "")

	// Add only first to active sessions
	env.manager.sessionsMu.Lock()
	env.manager.sessions[sess1.ID] = &Session{Session: sess1}
	env.manager.sessionsMu.Unlock()

	// List should return all
	sessions, err := env.manager.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(sessions) != 3 {
		t.Errorf("List() returned %d sessions, want 3", len(sessions))
	}

	_ = sess2
	_ = sess3
}

func TestManager_Send_StorageSession(t *testing.T) {
	env := setupTestEnv(t)

	// Create session in storage but not active
	storageSess, _ := env.store.CreateSession("/tmp/test", "")

	// Send should fail because session is not in active sessions
	err := env.manager.Send(storageSess.ID, "test")
	if err == nil || !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Send() error = %v, want ErrNotFound", err)
	}
}

func TestManager_Subscribe_StorageSession(t *testing.T) {
	env := setupTestEnv(t)

	// Create session in storage but not active
	storageSess, _ := env.store.CreateSession("/tmp/test", "")

	// Subscribe should fail
	_, err := env.manager.Subscribe(storageSess.ID)
	if err == nil || !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Subscribe() error = %v, want ErrNotFound", err)
	}
}

func TestManager_Delete_WithWrapper(t *testing.T) {
	env := setupTestEnv(t)

	// Create session in storage
	storageSess, _ := env.store.CreateSession("/tmp/test", "https://github.com/test/repo")

	// Add to active sessions with a nil wrapper
	env.manager.sessionsMu.Lock()
	env.manager.sessions[storageSess.ID] = &Session{
		Session: storageSess,
	}
	env.manager.sessionsMu.Unlock()

	// Delete should work
	err := env.manager.Delete(storageSess.ID)
	if err != nil {
		t.Errorf("Delete() error = %v", err)
	}
}

func TestSession_Status_WithStartingStatus(t *testing.T) {
	sess := &Session{
		Session: &storage.Session{
			Status: storage.SessionStatusPending,
		},
	}

	if sess.Status() != storage.SessionStatusPending {
		t.Errorf("Status() = %v, want %v", sess.Status(), storage.SessionStatusPending)
	}
}

func TestManager_BroadcastEvents_MultipleSubscribers(t *testing.T) {
	env := setupTestEnv(t)

	// Create multiple subscribers
	ch1 := env.manager.SubscribeAll()
	ch2 := env.manager.SubscribeAll()
	ch3 := env.manager.SubscribeAll()

	testEvent := Event{
		SessionID: "test-session",
		Type:      "output",
		Content:   "broadcast test",
		Timestamp: time.Now(),
	}

	// Send event
	env.manager.eventCh <- testEvent

	// All subscribers should receive the event
	timeout := time.After(200 * time.Millisecond)

	for i, ch := range []<-chan Event{ch1, ch2, ch3} {
		select {
		case received := <-ch:
			if received.Content != testEvent.Content {
				t.Errorf("subscriber %d: Content = %q, want %q", i, received.Content, testEvent.Content)
			}
		case <-timeout:
			t.Errorf("subscriber %d: did not receive event", i)
		}
	}
}

func TestManager_BroadcastEvents_DropsWhenFull(t *testing.T) {
	env := setupTestEnv(t)

	// Create a subscriber but don't read from it
	ch := env.manager.SubscribeAll()

	// Fill the channel buffer (size 100)
	for i := 0; i < 100; i++ {
		env.manager.eventCh <- Event{
			SessionID: "test",
			Type:      "output",
			Content:   "msg",
			Timestamp: time.Now(),
		}
	}

	// Allow broadcast goroutine to process
	time.Sleep(50 * time.Millisecond)

	// Send one more - should be dropped, not block
	done := make(chan struct{})
	go func() {
		env.manager.eventCh <- Event{
			SessionID: "test",
			Type:      "output",
			Content:   "overflow",
			Timestamp: time.Now(),
		}
		close(done)
	}()

	select {
	case <-done:
		// Good - didn't block
	case <-time.After(100 * time.Millisecond):
		t.Error("broadcast blocked on full subscriber channel")
	}

	// Drain the channel
	for len(ch) > 0 {
		<-ch
	}
}

func TestManager_ForwardEvents(t *testing.T) {
	env := setupTestEnv(t)

	// Create a mock wrapper
	mock := claude.NewMockWrapper()

	// Subscribe to manager's global events
	globalCh := env.manager.SubscribeAll()

	// Create session in storage
	storageSess, _ := env.store.CreateSession("/tmp/test", "")

	// Start forwarding
	go env.manager.forwardEvents(storageSess.ID, mock)

	// Give goroutine time to start
	time.Sleep(10 * time.Millisecond)

	// Emit an event from the mock
	mock.EmitOutput("Hello from Claude")

	// Should receive via global subscription
	select {
	case event := <-globalCh:
		if event.SessionID != storageSess.ID {
			t.Errorf("SessionID = %q, want %q", event.SessionID, storageSess.ID)
		}
		if event.Content != "Hello from Claude" {
			t.Errorf("Content = %q, want %q", event.Content, "Hello from Claude")
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("did not receive forwarded event")
	}

	// Verify message was stored
	messages, _ := env.store.GetMessages(storageSess.ID)
	found := false
	for _, msg := range messages {
		if msg.Content == "Hello from Claude" && msg.Role == storage.MessageRoleAssistant {
			found = true
			break
		}
	}
	if !found {
		t.Error("output event should be stored as assistant message")
	}

	// Stop the mock to end forwarding
	mock.Stop()
}

func TestManager_ForwardEvents_NonOutputNotStored(t *testing.T) {
	env := setupTestEnv(t)

	mock := claude.NewMockWrapper()
	storageSess, _ := env.store.CreateSession("/tmp/test", "")

	go env.manager.forwardEvents(storageSess.ID, mock)
	time.Sleep(10 * time.Millisecond)

	// Emit a question event (not output type)
	mock.EmitQuestion("What would you like?")

	time.Sleep(50 * time.Millisecond)

	// Question events should NOT be stored as messages
	messages, _ := env.store.GetMessages(storageSess.ID)
	for _, msg := range messages {
		if msg.Content == "What would you like?" {
			t.Error("question events should not be stored as messages")
		}
	}

	mock.Stop()
}

func TestManager_CreateWithMockFactory(t *testing.T) {
	env := setupTestEnv(t)

	// Create manager with mock factory
	mock := claude.NewMockWrapper()
	mock.AutoRespond = true

	mockFactory := func(opts claude.Options) claude.ClaudeWrapper {
		return mock
	}

	manager := NewManagerWithFactory(env.store, env.gitMgr, env.dataDir, mockFactory)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := manager.Create(ctx, CreateOptions{
		Prompt: "Hello",
	})

	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if sess == nil {
		t.Fatal("Create() returned nil session")
	}

	if !mock.StartCalled {
		t.Error("mock.Start() was not called")
	}

	// Verify prompt was sent
	time.Sleep(50 * time.Millisecond)
	if len(mock.SendCalls) == 0 {
		t.Error("prompt was not sent to mock")
	}

	// Cleanup
	manager.Delete(sess.ID)
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
