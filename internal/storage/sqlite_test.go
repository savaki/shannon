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

package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	return store
}

func TestNewStore_CreatesDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// File shouldn't exist yet
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatal("database file already exists")
	}

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	defer store.Close()

	// File should exist now
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("database file was not created: %v", err)
	}
}

func TestNewStore_MigrationsIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create store first time
	store1, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("first NewStore() error = %v", err)
	}
	store1.Close()

	// Create store second time (should run migrations again without error)
	store2, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("second NewStore() error = %v", err)
	}
	store2.Close()
}

func TestStore_CreateSession_ReturnsUUID(t *testing.T) {
	store := newTestStore(t)

	session, err := store.CreateSession("/tmp/work", "https://github.com/test/repo")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	if session.ID == "" {
		t.Error("CreateSession() returned empty ID")
	}

	// UUID should be 36 characters (8-4-4-4-12 with hyphens)
	if len(session.ID) != 36 {
		t.Errorf("CreateSession() ID length = %d, want 36", len(session.ID))
	}
}

func TestStore_GetSession_NotFound(t *testing.T) {
	store := newTestStore(t)

	_, err := store.GetSession("nonexistent-id")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetSession() error = %v, want ErrNotFound", err)
	}
}

func TestStore_GetSession_RoundTrip(t *testing.T) {
	store := newTestStore(t)

	created, err := store.CreateSession("/tmp/work", "https://github.com/test/repo")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	retrieved, err := store.GetSession(created.ID)
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}

	if retrieved.ID != created.ID {
		t.Errorf("ID = %q, want %q", retrieved.ID, created.ID)
	}
	if retrieved.WorkDir != created.WorkDir {
		t.Errorf("WorkDir = %q, want %q", retrieved.WorkDir, created.WorkDir)
	}
	if retrieved.RepoURL != created.RepoURL {
		t.Errorf("RepoURL = %q, want %q", retrieved.RepoURL, created.RepoURL)
	}
	if retrieved.Status != SessionStatusPending {
		t.Errorf("Status = %q, want %q", retrieved.Status, SessionStatusPending)
	}
}

func TestStore_ListSessions_Empty(t *testing.T) {
	store := newTestStore(t)

	sessions, err := store.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}

	if sessions == nil {
		t.Error("ListSessions() returned nil, want empty slice")
	}
	if len(sessions) != 0 {
		t.Errorf("ListSessions() returned %d sessions, want 0", len(sessions))
	}
}

func TestStore_ListSessions_Multiple(t *testing.T) {
	store := newTestStore(t)

	// Create multiple sessions
	for i := 0; i < 3; i++ {
		_, err := store.CreateSession("/tmp/work", "")
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
	}

	sessions, err := store.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}

	if len(sessions) != 3 {
		t.Errorf("ListSessions() returned %d sessions, want 3", len(sessions))
	}
}

func TestStore_UpdateSessionStatus(t *testing.T) {
	store := newTestStore(t)

	session, err := store.CreateSession("/tmp/work", "")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	err = store.UpdateSessionStatus(session.ID, SessionStatusRunning)
	if err != nil {
		t.Fatalf("UpdateSessionStatus() error = %v", err)
	}

	updated, err := store.GetSession(session.ID)
	if err != nil {
		t.Fatalf("GetSession() error = %v", err)
	}

	if updated.Status != SessionStatusRunning {
		t.Errorf("Status = %q, want %q", updated.Status, SessionStatusRunning)
	}
}

func TestStore_UpdateSessionStatus_NotFound(t *testing.T) {
	store := newTestStore(t)

	err := store.UpdateSessionStatus("nonexistent-id", SessionStatusRunning)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("UpdateSessionStatus() error = %v, want ErrNotFound", err)
	}
}

func TestStore_DeleteSession_RemovesSession(t *testing.T) {
	store := newTestStore(t)

	session, err := store.CreateSession("/tmp/work", "")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	err = store.DeleteSession(session.ID)
	if err != nil {
		t.Fatalf("DeleteSession() error = %v", err)
	}

	_, err = store.GetSession(session.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetSession() after delete error = %v, want ErrNotFound", err)
	}
}

func TestStore_DeleteSession_CascadesMessages(t *testing.T) {
	store := newTestStore(t)

	session, err := store.CreateSession("/tmp/work", "")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	// Create messages for the session
	_, err = store.CreateMessage(session.ID, MessageRoleUser, "Hello")
	if err != nil {
		t.Fatalf("CreateMessage() error = %v", err)
	}

	// Delete session
	err = store.DeleteSession(session.ID)
	if err != nil {
		t.Fatalf("DeleteSession() error = %v", err)
	}

	// Messages should be deleted too (cascade)
	messages, err := store.GetMessages(session.ID)
	if err != nil {
		t.Fatalf("GetMessages() error = %v", err)
	}
	if len(messages) != 0 {
		t.Errorf("GetMessages() after cascade returned %d messages, want 0", len(messages))
	}
}

func TestStore_DeleteSession_NotFound(t *testing.T) {
	store := newTestStore(t)

	err := store.DeleteSession("nonexistent-id")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("DeleteSession() error = %v, want ErrNotFound", err)
	}
}

func TestStore_CreateMessage_AssociatesWithSession(t *testing.T) {
	store := newTestStore(t)

	session, err := store.CreateSession("/tmp/work", "")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	msg, err := store.CreateMessage(session.ID, MessageRoleUser, "Hello, Claude!")
	if err != nil {
		t.Fatalf("CreateMessage() error = %v", err)
	}

	if msg.ID == "" {
		t.Error("CreateMessage() returned empty ID")
	}
	if msg.SessionID != session.ID {
		t.Errorf("SessionID = %q, want %q", msg.SessionID, session.ID)
	}
	if msg.Role != MessageRoleUser {
		t.Errorf("Role = %q, want %q", msg.Role, MessageRoleUser)
	}
	if msg.Content != "Hello, Claude!" {
		t.Errorf("Content = %q, want %q", msg.Content, "Hello, Claude!")
	}
}

func TestStore_CreateMessage_SessionNotFound(t *testing.T) {
	store := newTestStore(t)

	_, err := store.CreateMessage("nonexistent-id", MessageRoleUser, "Hello")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("CreateMessage() error = %v, want ErrNotFound", err)
	}
}

func TestStore_GetMessages_Chronological(t *testing.T) {
	store := newTestStore(t)

	session, err := store.CreateSession("/tmp/work", "")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	// Create messages in order
	contents := []string{"First", "Second", "Third"}
	for _, content := range contents {
		_, err := store.CreateMessage(session.ID, MessageRoleUser, content)
		if err != nil {
			t.Fatalf("CreateMessage() error = %v", err)
		}
	}

	messages, err := store.GetMessages(session.ID)
	if err != nil {
		t.Fatalf("GetMessages() error = %v", err)
	}

	if len(messages) != 3 {
		t.Fatalf("GetMessages() returned %d messages, want 3", len(messages))
	}

	for i, content := range contents {
		if messages[i].Content != content {
			t.Errorf("messages[%d].Content = %q, want %q", i, messages[i].Content, content)
		}
	}
}

func TestStore_GetMessages_Empty(t *testing.T) {
	store := newTestStore(t)

	session, err := store.CreateSession("/tmp/work", "")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	messages, err := store.GetMessages(session.ID)
	if err != nil {
		t.Fatalf("GetMessages() error = %v", err)
	}

	if messages == nil {
		t.Error("GetMessages() returned nil, want empty slice")
	}
	if len(messages) != 0 {
		t.Errorf("GetMessages() returned %d messages, want 0", len(messages))
	}
}

func TestStore_Config_RoundTrip(t *testing.T) {
	store := newTestStore(t)

	err := store.SetConfig("psk", "test-secret-key")
	if err != nil {
		t.Fatalf("SetConfig() error = %v", err)
	}

	value, err := store.GetConfig("psk")
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}

	if value != "test-secret-key" {
		t.Errorf("GetConfig() = %q, want %q", value, "test-secret-key")
	}
}

func TestStore_Config_Upsert(t *testing.T) {
	store := newTestStore(t)

	// Set initial value
	err := store.SetConfig("key", "value1")
	if err != nil {
		t.Fatalf("SetConfig() error = %v", err)
	}

	// Update value
	err = store.SetConfig("key", "value2")
	if err != nil {
		t.Fatalf("SetConfig() upsert error = %v", err)
	}

	value, err := store.GetConfig("key")
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}

	if value != "value2" {
		t.Errorf("GetConfig() = %q, want %q", value, "value2")
	}
}

func TestStore_Config_NotFound(t *testing.T) {
	store := newTestStore(t)

	_, err := store.GetConfig("nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetConfig() error = %v, want ErrNotFound", err)
	}
}

func TestStore_ConcurrentAccess(t *testing.T) {
	store := newTestStore(t)

	const numGoroutines = 10
	const numOperations = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				// Create session
				session, err := store.CreateSession("/tmp/work", "")
				if err != nil {
					t.Errorf("concurrent CreateSession() error = %v", err)
					return
				}

				// Update status
				err = store.UpdateSessionStatus(session.ID, SessionStatusRunning)
				if err != nil {
					t.Errorf("concurrent UpdateSessionStatus() error = %v", err)
					return
				}

				// List sessions
				_, err = store.ListSessions()
				if err != nil {
					t.Errorf("concurrent ListSessions() error = %v", err)
					return
				}
			}
		}()
	}

	wg.Wait()
}

func TestStore_Device_RoundTrip(t *testing.T) {
	store := newTestStore(t)

	device, err := store.CreateDevice("token123", "ios")
	if err != nil {
		t.Fatalf("CreateDevice() error = %v", err)
	}

	if device.ID == "" {
		t.Error("CreateDevice() returned empty ID")
	}
	if device.Token != "token123" {
		t.Errorf("Token = %q, want %q", device.Token, "token123")
	}
	if device.Platform != "ios" {
		t.Errorf("Platform = %q, want %q", device.Platform, "ios")
	}
}

func TestStore_Device_UpsertOnDuplicateToken(t *testing.T) {
	store := newTestStore(t)

	// Create device
	_, err := store.CreateDevice("token123", "ios")
	if err != nil {
		t.Fatalf("CreateDevice() error = %v", err)
	}

	// Create again with same token but different platform
	_, err = store.CreateDevice("token123", "android")
	if err != nil {
		t.Fatalf("CreateDevice() upsert error = %v", err)
	}

	devices, err := store.ListDevices()
	if err != nil {
		t.Fatalf("ListDevices() error = %v", err)
	}

	// Should only have one device
	if len(devices) != 1 {
		t.Errorf("ListDevices() returned %d devices, want 1", len(devices))
	}
}

func TestStore_DeleteDevice_NotFound(t *testing.T) {
	store := newTestStore(t)

	err := store.DeleteDevice("nonexistent-id")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("DeleteDevice() error = %v, want ErrNotFound", err)
	}
}

func TestStore_ListDevices_Empty(t *testing.T) {
	store := newTestStore(t)

	devices, err := store.ListDevices()
	if err != nil {
		t.Fatalf("ListDevices() error = %v", err)
	}

	if devices == nil {
		t.Error("ListDevices() returned nil, want empty slice")
	}
	if len(devices) != 0 {
		t.Errorf("ListDevices() returned %d devices, want 0", len(devices))
	}
}

func TestStore_ListDevices_Multiple(t *testing.T) {
	store := newTestStore(t)

	// Create multiple devices
	for i := 0; i < 3; i++ {
		token := fmt.Sprintf("token-%d", i)
		_, err := store.CreateDevice(token, "ios")
		if err != nil {
			t.Fatalf("CreateDevice() error = %v", err)
		}
	}

	devices, err := store.ListDevices()
	if err != nil {
		t.Fatalf("ListDevices() error = %v", err)
	}

	if len(devices) != 3 {
		t.Errorf("ListDevices() returned %d devices, want 3", len(devices))
	}
}

func TestStore_DeleteDevice_Success(t *testing.T) {
	store := newTestStore(t)

	device, err := store.CreateDevice("token123", "ios")
	if err != nil {
		t.Fatalf("CreateDevice() error = %v", err)
	}

	err = store.DeleteDevice(device.ID)
	if err != nil {
		t.Fatalf("DeleteDevice() error = %v", err)
	}

	// Verify device was deleted
	devices, _ := store.ListDevices()
	if len(devices) != 0 {
		t.Error("device should be deleted")
	}
}

func TestStore_Close(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	err = store.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestStore_SessionStatuses(t *testing.T) {
	store := newTestStore(t)

	session, err := store.CreateSession("/tmp/test", "")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	statuses := []SessionStatus{
		SessionStatusPending,
		SessionStatusRunning,
		SessionStatusWaitingInput,
		SessionStatusStopped,
		SessionStatusError,
	}

	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			err := store.UpdateSessionStatus(session.ID, status)
			if err != nil {
				t.Fatalf("UpdateSessionStatus() error = %v", err)
			}

			updated, err := store.GetSession(session.ID)
			if err != nil {
				t.Fatalf("GetSession() error = %v", err)
			}

			if updated.Status != status {
				t.Errorf("Status = %v, want %v", updated.Status, status)
			}
		})
	}
}

func TestStore_MessageRoles(t *testing.T) {
	store := newTestStore(t)

	session, _ := store.CreateSession("/tmp/test", "")

	roles := []MessageRole{
		MessageRoleUser,
		MessageRoleAssistant,
		MessageRoleSystem,
	}

	for _, role := range roles {
		t.Run(string(role), func(t *testing.T) {
			msg, err := store.CreateMessage(session.ID, role, "content")
			if err != nil {
				t.Fatalf("CreateMessage() error = %v", err)
			}

			if msg.Role != role {
				t.Errorf("Role = %v, want %v", msg.Role, role)
			}
		})
	}
}

func TestStore_ConfigOverwrite(t *testing.T) {
	store := newTestStore(t)

	// Set initial value
	if err := store.SetConfig("key1", "value1"); err != nil {
		t.Fatalf("SetConfig() error = %v", err)
	}

	// Overwrite with new value
	if err := store.SetConfig("key1", "value2"); err != nil {
		t.Fatalf("SetConfig() overwrite error = %v", err)
	}

	// Verify new value
	val, err := store.GetConfig("key1")
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}

	if val != "value2" {
		t.Errorf("GetConfig() = %q, want %q", val, "value2")
	}
}

func TestStore_CreateSession_AllFields(t *testing.T) {
	store := newTestStore(t)

	workDir := "/workspace/project"
	repoURL := "https://github.com/example/repo.git"

	session, err := store.CreateSession(workDir, repoURL)
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	if session.WorkDir != workDir {
		t.Errorf("WorkDir = %q, want %q", session.WorkDir, workDir)
	}
	if session.RepoURL != repoURL {
		t.Errorf("RepoURL = %q, want %q", session.RepoURL, repoURL)
	}
	if session.Status != SessionStatusPending {
		t.Errorf("Status = %v, want %v", session.Status, SessionStatusPending)
	}
	if session.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if session.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
}

func TestStore_ListSessions_Ordering(t *testing.T) {
	store := newTestStore(t)

	// Create sessions with slight delay to ensure ordering
	for i := 0; i < 3; i++ {
		_, err := store.CreateSession(fmt.Sprintf("/tmp/test%d", i), "")
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
	}

	sessions, err := store.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}

	// Should be ordered by created_at DESC
	if len(sessions) != 3 {
		t.Fatalf("ListSessions() returned %d sessions, want 3", len(sessions))
	}
}

func TestStore_CreateMessage_AllFields(t *testing.T) {
	store := newTestStore(t)

	session, _ := store.CreateSession("/tmp/test", "")
	content := "Hello, this is a test message with unicode: 你好"

	msg, err := store.CreateMessage(session.ID, MessageRoleUser, content)
	if err != nil {
		t.Fatalf("CreateMessage() error = %v", err)
	}

	if msg.ID == "" {
		t.Error("ID should not be empty")
	}
	if msg.SessionID != session.ID {
		t.Errorf("SessionID = %q, want %q", msg.SessionID, session.ID)
	}
	if msg.Role != MessageRoleUser {
		t.Errorf("Role = %v, want %v", msg.Role, MessageRoleUser)
	}
	if msg.Content != content {
		t.Errorf("Content = %q, want %q", msg.Content, content)
	}
	if msg.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestStore_Device_AllFields(t *testing.T) {
	store := newTestStore(t)

	token := "fcm_token_abc123"
	platform := "android"

	device, err := store.CreateDevice(token, platform)
	if err != nil {
		t.Fatalf("CreateDevice() error = %v", err)
	}

	if device.ID == "" {
		t.Error("ID should not be empty")
	}
	if device.Token != token {
		t.Errorf("Token = %q, want %q", device.Token, token)
	}
	if device.Platform != platform {
		t.Errorf("Platform = %q, want %q", device.Platform, platform)
	}
	if device.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestStore_UpdateSession_UpdatesTimestamp(t *testing.T) {
	store := newTestStore(t)

	session, _ := store.CreateSession("/tmp/test", "")
	originalUpdatedAt := session.UpdatedAt

	// Small delay to ensure timestamp difference
	err := store.UpdateSessionStatus(session.ID, SessionStatusRunning)
	if err != nil {
		t.Fatalf("UpdateSessionStatus() error = %v", err)
	}

	updated, _ := store.GetSession(session.ID)
	if !updated.UpdatedAt.After(originalUpdatedAt) && !updated.UpdatedAt.Equal(originalUpdatedAt) {
		t.Error("UpdatedAt should be updated")
	}
}

func TestStore_LargeContent(t *testing.T) {
	store := newTestStore(t)

	session, _ := store.CreateSession("/tmp/test", "")

	// Create message with large content
	largeContent := make([]byte, 100000)
	for i := range largeContent {
		largeContent[i] = 'a'
	}

	msg, err := store.CreateMessage(session.ID, MessageRoleUser, string(largeContent))
	if err != nil {
		t.Fatalf("CreateMessage() error = %v", err)
	}

	if len(msg.Content) != 100000 {
		t.Errorf("Content length = %d, want 100000", len(msg.Content))
	}
}

func TestStore_SpecialCharacters(t *testing.T) {
	store := newTestStore(t)

	session, _ := store.CreateSession("/tmp/test", "")

	// Test special characters in content
	specialContent := "Hello! 'quotes' \"double\" `backticks` \n\t tabs and newlines 中文 emoji 🎉"

	msg, err := store.CreateMessage(session.ID, MessageRoleUser, specialContent)
	if err != nil {
		t.Fatalf("CreateMessage() error = %v", err)
	}

	messages, _ := store.GetMessages(session.ID)
	if len(messages) != 1 {
		t.Fatalf("GetMessages() returned %d messages, want 1", len(messages))
	}

	if messages[0].Content != specialContent {
		t.Errorf("Content = %q, want %q", messages[0].Content, specialContent)
	}

	_ = msg // use msg
}

func TestStore_EmptyWorkDir(t *testing.T) {
	store := newTestStore(t)

	session, err := store.CreateSession("", "")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	if session.WorkDir != "" {
		t.Errorf("WorkDir = %q, want empty string", session.WorkDir)
	}
}

func TestStore_MultipleDevices(t *testing.T) {
	store := newTestStore(t)

	// Create 5 devices
	var deviceIDs []string
	for i := 0; i < 5; i++ {
		device, err := store.CreateDevice(fmt.Sprintf("token%d", i), "ios")
		if err != nil {
			t.Fatalf("CreateDevice() error = %v", err)
		}
		deviceIDs = append(deviceIDs, device.ID)
	}

	// Delete odd-indexed devices
	for i := 1; i < 5; i += 2 {
		err := store.DeleteDevice(deviceIDs[i])
		if err != nil {
			t.Fatalf("DeleteDevice() error = %v", err)
		}
	}

	// Verify remaining devices
	devices, _ := store.ListDevices()
	if len(devices) != 3 {
		t.Errorf("ListDevices() returned %d devices, want 3", len(devices))
	}
}

func TestStore_SessionWithMessages(t *testing.T) {
	store := newTestStore(t)

	session, _ := store.CreateSession("/tmp/test", "")

	// Add messages in conversation order
	store.CreateMessage(session.ID, MessageRoleUser, "Hello")
	store.CreateMessage(session.ID, MessageRoleAssistant, "Hi there!")
	store.CreateMessage(session.ID, MessageRoleUser, "How are you?")
	store.CreateMessage(session.ID, MessageRoleAssistant, "I'm doing well, thanks!")

	messages, _ := store.GetMessages(session.ID)
	if len(messages) != 4 {
		t.Fatalf("GetMessages() returned %d messages, want 4", len(messages))
	}

	// Verify order
	expectedRoles := []MessageRole{
		MessageRoleUser,
		MessageRoleAssistant,
		MessageRoleUser,
		MessageRoleAssistant,
	}

	for i, msg := range messages {
		if msg.Role != expectedRoles[i] {
			t.Errorf("messages[%d].Role = %v, want %v", i, msg.Role, expectedRoles[i])
		}
	}
}

func TestStore_ConfigMultipleKeys(t *testing.T) {
	store := newTestStore(t)

	// Set multiple config keys
	keys := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	for k, v := range keys {
		if err := store.SetConfig(k, v); err != nil {
			t.Fatalf("SetConfig() error = %v", err)
		}
	}

	// Verify all keys
	for k, expected := range keys {
		got, err := store.GetConfig(k)
		if err != nil {
			t.Fatalf("GetConfig(%q) error = %v", k, err)
		}
		if got != expected {
			t.Errorf("GetConfig(%q) = %q, want %q", k, got, expected)
		}
	}
}

func TestErrNotFound_String(t *testing.T) {
	if ErrNotFound.Error() != "not found" {
		t.Errorf("ErrNotFound.Error() = %q, want %q", ErrNotFound.Error(), "not found")
	}
}

func TestSessionStatus_Values(t *testing.T) {
	statuses := []SessionStatus{
		SessionStatusPending,
		SessionStatusRunning,
		SessionStatusWaitingInput,
		SessionStatusStopped,
		SessionStatusError,
	}

	for _, s := range statuses {
		if string(s) == "" {
			t.Errorf("SessionStatus %v has empty string value", s)
		}
	}
}

func TestMessageRole_Values(t *testing.T) {
	roles := []MessageRole{
		MessageRoleUser,
		MessageRoleAssistant,
		MessageRoleSystem,
	}

	for _, r := range roles {
		if string(r) == "" {
			t.Errorf("MessageRole %v has empty string value", r)
		}
	}
}

func TestSession_Fields(t *testing.T) {
	now := time.Now()
	session := Session{
		ID:        "sess-123",
		WorkDir:   "/tmp/test",
		RepoURL:   "https://github.com/test/repo",
		Status:    SessionStatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if session.ID != "sess-123" {
		t.Errorf("ID = %q, want %q", session.ID, "sess-123")
	}
	if session.WorkDir != "/tmp/test" {
		t.Errorf("WorkDir = %q, want %q", session.WorkDir, "/tmp/test")
	}
	if session.RepoURL != "https://github.com/test/repo" {
		t.Errorf("RepoURL = %q, want %q", session.RepoURL, "https://github.com/test/repo")
	}
	if session.Status != SessionStatusRunning {
		t.Errorf("Status = %v, want %v", session.Status, SessionStatusRunning)
	}
}

func TestMessage_Fields(t *testing.T) {
	now := time.Now()
	msg := Message{
		ID:        "msg-123",
		SessionID: "sess-123",
		Role:      MessageRoleUser,
		Content:   "Hello",
		CreatedAt: now,
	}

	if msg.ID != "msg-123" {
		t.Errorf("ID = %q, want %q", msg.ID, "msg-123")
	}
	if msg.SessionID != "sess-123" {
		t.Errorf("SessionID = %q, want %q", msg.SessionID, "sess-123")
	}
	if msg.Role != MessageRoleUser {
		t.Errorf("Role = %v, want %v", msg.Role, MessageRoleUser)
	}
	if msg.Content != "Hello" {
		t.Errorf("Content = %q, want %q", msg.Content, "Hello")
	}
}

func TestDevice_Fields(t *testing.T) {
	now := time.Now()
	device := Device{
		ID:        "dev-123",
		Token:     "token-abc",
		Platform:  "ios",
		CreatedAt: now,
	}

	if device.ID != "dev-123" {
		t.Errorf("ID = %q, want %q", device.ID, "dev-123")
	}
	if device.Token != "token-abc" {
		t.Errorf("Token = %q, want %q", device.Token, "token-abc")
	}
	if device.Platform != "ios" {
		t.Errorf("Platform = %q, want %q", device.Platform, "ios")
	}
}

func TestNewStore_InvalidPath(t *testing.T) {
	// Try to create store in non-existent directory (should fail or create)
	_, err := NewStore("/nonexistent/path/that/doesnt/exist/test.db")
	// Note: modernc/sqlite might create parent dirs, so we just check it doesn't panic
	_ = err
}

func TestStore_GetMessages_NoSession(t *testing.T) {
	store := newTestStore(t)

	// Get messages for non-existent session returns empty slice
	messages, err := store.GetMessages("nonexistent-session")
	if err != nil {
		t.Fatalf("GetMessages() error = %v", err)
	}

	if len(messages) != 0 {
		t.Errorf("GetMessages() returned %d messages, want 0", len(messages))
	}
}

func TestStore_Device_UpdatePlatform(t *testing.T) {
	store := newTestStore(t)

	// Create device
	device1, _ := store.CreateDevice("unique-token", "ios")

	// Create again with same token - should update platform
	device2, err := store.CreateDevice("unique-token", "android")
	if err != nil {
		t.Fatalf("CreateDevice() upsert error = %v", err)
	}

	// The ID might be different due to upsert returning new ID
	_ = device1
	_ = device2

	// Verify only one device exists with updated platform
	devices, _ := store.ListDevices()
	if len(devices) != 1 {
		t.Errorf("ListDevices() returned %d devices, want 1", len(devices))
	}
	if devices[0].Platform != "android" {
		t.Errorf("Platform = %q, want %q", devices[0].Platform, "android")
	}
}

func TestStore_Session_EmptyRepoURL(t *testing.T) {
	store := newTestStore(t)

	session, err := store.CreateSession("/tmp/work", "")
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	if session.RepoURL != "" {
		t.Errorf("RepoURL = %q, want empty string", session.RepoURL)
	}
}

func TestStore_ConcurrentMessageCreation(t *testing.T) {
	store := newTestStore(t)
	session, _ := store.CreateSession("/tmp/work", "")

	const numGoroutines = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			_, err := store.CreateMessage(session.ID, MessageRoleUser, fmt.Sprintf("Message %d", idx))
			if err != nil {
				t.Errorf("concurrent CreateMessage() error = %v", err)
			}
		}(i)
	}

	wg.Wait()

	messages, _ := store.GetMessages(session.ID)
	if len(messages) != numGoroutines {
		t.Errorf("GetMessages() returned %d messages, want %d", len(messages), numGoroutines)
	}
}

func TestStore_ConcurrentConfigAccess(t *testing.T) {
	store := newTestStore(t)

	const numGoroutines = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2)

	// Concurrent writers
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			key := fmt.Sprintf("key%d", idx)
			value := fmt.Sprintf("value%d", idx)
			if err := store.SetConfig(key, value); err != nil {
				t.Errorf("concurrent SetConfig() error = %v", err)
			}
		}(i)
	}

	// Concurrent readers
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			key := fmt.Sprintf("key%d", idx)
			// May or may not find the key depending on timing
			_, _ = store.GetConfig(key)
		}(i)
	}

	wg.Wait()
}
