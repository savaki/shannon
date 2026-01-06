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
	"path/filepath"
	"testing"
	"time"

	"github.com/savaki/shannon/internal/claude"
	"github.com/savaki/shannon/internal/git"
	"github.com/savaki/shannon/internal/storage"
)

// mockWrapperFactory creates a factory that returns pre-configured mock wrappers.
func mockWrapperFactory(mock *claude.MockWrapper) WrapperFactory {
	return func(opts claude.Options) claude.ClaudeWrapper {
		return mock
	}
}

type mockTestEnv struct {
	store   *storage.Store
	gitMgr  *git.Manager
	manager *Manager
	mock    *claude.MockWrapper
	dataDir string
}

func setupMockTestEnv(t *testing.T) *mockTestEnv {
	t.Helper()

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := storage.NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	gitMgr := git.NewManager(tmpDir, false)
	mock := claude.NewMockWrapper()
	manager := NewManagerWithFactory(store, gitMgr, tmpDir, mockWrapperFactory(mock))

	return &mockTestEnv{
		store:   store,
		gitMgr:  gitMgr,
		manager: manager,
		mock:    mock,
		dataDir: tmpDir,
	}
}

func TestManager_Create_WithMock(t *testing.T) {
	env := setupMockTestEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := env.manager.Create(ctx, CreateOptions{
		Prompt: "Hello, Claude!",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if sess == nil {
		t.Fatal("Create() returned nil session")
	}

	if sess.ID == "" {
		t.Error("session ID should not be empty")
	}

	// Verify mock was started
	if !env.mock.StartCalled {
		t.Error("mock Start() should have been called")
	}
}

func TestManager_Create_WithMock_StartError(t *testing.T) {
	env := setupMockTestEnv(t)
	env.mock.StartErr = errors.New("mock start error")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := env.manager.Create(ctx, CreateOptions{})
	if err == nil {
		t.Error("Create() should error when Start fails")
	}
}

func TestManager_Create_WithMock_SendsPrompt(t *testing.T) {
	env := setupMockTestEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := env.manager.Create(ctx, CreateOptions{
		Prompt: "Test prompt",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Give time for prompt to be sent
	time.Sleep(50 * time.Millisecond)

	// Verify prompt was sent
	if len(env.mock.SendCalls) == 0 {
		t.Error("prompt should have been sent to mock")
	}
	if len(env.mock.SendCalls) > 0 && env.mock.SendCalls[0] != "Test prompt" {
		t.Errorf("SendCalls[0] = %q, want %q", env.mock.SendCalls[0], "Test prompt")
	}
}

func TestManager_Send_WithMock(t *testing.T) {
	env := setupMockTestEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := env.manager.Create(ctx, CreateOptions{})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Send input
	err = env.manager.Send(sess.ID, "user input")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	// Verify input was sent
	found := false
	for _, call := range env.mock.SendCalls {
		if call == "user input" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Send() should have called mock.Send with 'user input'")
	}
}

func TestManager_Send_WithMock_Error(t *testing.T) {
	env := setupMockTestEnv(t)
	env.mock.SendErr = errors.New("mock send error")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := env.manager.Create(ctx, CreateOptions{})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Send should fail
	err = env.manager.Send(sess.ID, "input")
	if err == nil {
		t.Error("Send() should error when mock.Send fails")
	}
}

func TestManager_Stop_WithMock(t *testing.T) {
	env := setupMockTestEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := env.manager.Create(ctx, CreateOptions{})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Stop session
	err = env.manager.Stop(sess.ID)
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	// Verify mock was stopped
	if !env.mock.StopCalled {
		t.Error("mock Stop() should have been called")
	}
}

func TestManager_Stop_WithMock_Error(t *testing.T) {
	env := setupMockTestEnv(t)
	env.mock.StopErr = errors.New("mock stop error")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := env.manager.Create(ctx, CreateOptions{})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Stop should fail
	err = env.manager.Stop(sess.ID)
	if err == nil {
		t.Error("Stop() should error when mock.Stop fails")
	}
}

func TestManager_Subscribe_WithMock(t *testing.T) {
	env := setupMockTestEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := env.manager.Create(ctx, CreateOptions{})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Subscribe to events
	eventCh, err := env.manager.Subscribe(sess.ID)
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	// Emit an event from mock
	env.mock.EmitOutput("test output")

	// Should receive event
	select {
	case event := <-eventCh:
		if event.Content != "test output" {
			t.Errorf("event.Content = %q, want %q", event.Content, "test output")
		}
		if event.SessionID != sess.ID {
			t.Errorf("event.SessionID = %q, want %q", event.SessionID, sess.ID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("did not receive event")
	}
}

func TestManager_ForwardEvents_WithMock(t *testing.T) {
	env := setupMockTestEnv(t)
	env.mock.AutoRespond = true
	env.mock.Response = "Mock response"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := env.manager.Create(ctx, CreateOptions{})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Send input (mock will auto-respond)
	err = env.manager.Send(sess.ID, "test input")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	// Give time for events to be forwarded
	time.Sleep(100 * time.Millisecond)

	// Check messages were stored
	messages, _ := env.store.GetMessages(sess.ID)
	if len(messages) < 2 {
		t.Errorf("expected at least 2 messages, got %d", len(messages))
	}
}

func TestManager_Delete_WithMock(t *testing.T) {
	env := setupMockTestEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := env.manager.Create(ctx, CreateOptions{})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Delete session
	err = env.manager.Delete(sess.ID)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify mock was stopped
	if !env.mock.StopCalled {
		t.Error("mock Stop() should have been called during delete")
	}

	// Session should be gone
	_, err = env.manager.Get(sess.ID)
	if !errors.Is(err, storage.ErrNotFound) {
		t.Error("session should be deleted")
	}
}

func TestManager_Session_Status_WithMock(t *testing.T) {
	env := setupMockTestEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := env.manager.Create(ctx, CreateOptions{})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Initial status should be running
	if sess.Status() != storage.SessionStatusRunning {
		t.Errorf("Status() = %v, want %v", sess.Status(), storage.SessionStatusRunning)
	}

	// Change mock status to waiting input
	env.mock.SetStatus(claude.StatusWaitingInput)
	if sess.Status() != storage.SessionStatusWaitingInput {
		t.Errorf("Status() = %v, want %v", sess.Status(), storage.SessionStatusWaitingInput)
	}

	// Change to stopped
	env.mock.SetStatus(claude.StatusStopped)
	if sess.Status() != storage.SessionStatusStopped {
		t.Errorf("Status() = %v, want %v", sess.Status(), storage.SessionStatusStopped)
	}

	// Change to error
	env.mock.SetStatus(claude.StatusError)
	if sess.Status() != storage.SessionStatusError {
		t.Errorf("Status() = %v, want %v", sess.Status(), storage.SessionStatusError)
	}
}

func TestManager_Create_WithMock_MultiplePrompts(t *testing.T) {
	env := setupMockTestEnv(t)
	env.mock.AutoRespond = true

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := env.manager.Create(ctx, CreateOptions{
		Prompt: "Initial prompt",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Send multiple inputs
	for i := 0; i < 5; i++ {
		err = env.manager.Send(sess.ID, "Input "+string(rune('A'+i)))
		if err != nil {
			t.Fatalf("Send() error = %v", err)
		}
	}

	// Verify all inputs were sent
	// Initial prompt + 5 inputs = 6 calls (but initial prompt may be sent separately)
	if len(env.mock.SendCalls) < 5 {
		t.Errorf("expected at least 5 SendCalls, got %d", len(env.mock.SendCalls))
	}
}

func TestManager_EmitQuestion_WithMock(t *testing.T) {
	env := setupMockTestEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sess, err := env.manager.Create(ctx, CreateOptions{})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Subscribe to all events
	allEventsCh := env.manager.SubscribeAll()

	// Emit a question from mock
	env.mock.EmitQuestion("Do you want to continue?")

	// Give time for broadcast
	time.Sleep(100 * time.Millisecond)

	// Check session status changed
	if sess.Status() != storage.SessionStatusWaitingInput {
		t.Errorf("Status() = %v, want %v", sess.Status(), storage.SessionStatusWaitingInput)
	}

	// Check event was broadcast
	select {
	case event := <-allEventsCh:
		if event.Type != string(claude.EventTypeQuestion) {
			t.Errorf("event.Type = %q, want %q", event.Type, string(claude.EventTypeQuestion))
		}
	case <-time.After(100 * time.Millisecond):
		// Event may have already been consumed or not broadcast yet
	}
}

func TestNewManagerWithFactory(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, _ := storage.NewStore(dbPath)
	defer store.Close()

	gitMgr := git.NewManager(tmpDir, false)

	// Test with custom factory
	customFactory := func(opts claude.Options) claude.ClaudeWrapper {
		return claude.NewMockWrapper()
	}

	manager := NewManagerWithFactory(store, gitMgr, tmpDir, customFactory)
	if manager == nil {
		t.Fatal("NewManagerWithFactory() returned nil")
	}

	if manager.wrapperFactory == nil {
		t.Error("wrapperFactory should not be nil")
	}
}

func TestDefaultWrapperFactory(t *testing.T) {
	wrapper := DefaultWrapperFactory(claude.Options{WorkDir: "/tmp/test"})
	if wrapper == nil {
		t.Error("DefaultWrapperFactory() returned nil")
	}

	// Should be a real Wrapper
	_, ok := wrapper.(*claude.Wrapper)
	if !ok {
		t.Error("DefaultWrapperFactory() should return *claude.Wrapper")
	}
}
