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

package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func hasGit() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

func skipWithoutGit(t *testing.T) {
	t.Helper()
	if !hasGit() {
		t.Skip("git not available")
	}
}

func TestRepoHash(t *testing.T) {
	// Same URL should produce same hash
	hash1 := repoHash("https://github.com/test/repo")
	hash2 := repoHash("https://github.com/test/repo")
	if hash1 != hash2 {
		t.Error("repoHash() should be deterministic")
	}

	// Different URLs should produce different hashes
	hash3 := repoHash("https://github.com/test/other")
	if hash1 == hash3 {
		t.Error("repoHash() should produce different hashes for different URLs")
	}

	// Hash should be 16 characters (8 bytes hex encoded)
	if len(hash1) != 16 {
		t.Errorf("repoHash() length = %d, want 16", len(hash1))
	}
}

func TestManager_SessionPath(t *testing.T) {
	m := NewManager("/data", true)

	path := m.SessionPath("session-123")
	expected := "/data/sessions/session-123"
	if path != expected {
		t.Errorf("SessionPath() = %q, want %q", path, expected)
	}
}

func TestManager_CreateSessionDir(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir, true)

	path, err := m.CreateSessionDir("test-session")
	if err != nil {
		t.Fatalf("CreateSessionDir() error = %v", err)
	}

	// Verify directory was created
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("session directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("created path is not a directory")
	}
}

func TestManager_Cleanup(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir, true)

	// Create session directory
	path, err := m.CreateSessionDir("test-session")
	if err != nil {
		t.Fatalf("CreateSessionDir() error = %v", err)
	}

	// Create a file inside
	testFile := filepath.Join(path, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Cleanup
	err = m.Cleanup("test-session")
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	// Verify directory was removed
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("session directory was not removed")
	}
}

func TestManager_Cleanup_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir, true)

	// Cleanup non-existent session should not error
	err := m.Cleanup("nonexistent-session")
	if err != nil {
		t.Errorf("Cleanup() on non-existent session error = %v, want nil", err)
	}
}

func TestManager_Clone_EmptyURL(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir, true)

	_, err := m.Clone("")
	if err == nil {
		t.Error("Clone() with empty URL should error")
	}
}

func TestManager_Clone_InvalidURL(t *testing.T) {
	skipWithoutGit(t)

	tmpDir := t.TempDir()
	m := NewManager(tmpDir, true)

	_, err := m.Clone("not-a-valid-git-url")
	if err == nil {
		t.Error("Clone() with invalid URL should error")
	}
}

func TestManager_CloneForSession_EmptyURL(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir, false)

	_, err := m.CloneForSession("", "session-123")
	if err == nil {
		t.Error("CloneForSession() with empty URL should error")
	}
}

func TestManager_CreateWorktree_NotCloned(t *testing.T) {
	skipWithoutGit(t)

	tmpDir := t.TempDir()
	m := NewManager(tmpDir, true)

	_, err := m.CreateWorktree("https://github.com/test/repo", "session-123", "")
	if err == nil {
		t.Error("CreateWorktree() without clone should error")
	}
}

func TestManager_IsCloned_False(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir, true)

	if m.IsCloned("https://github.com/test/repo") {
		t.Error("IsCloned() should return false for non-existent repo")
	}
}

// Integration test - requires git and network
func TestManager_Clone_Integration(t *testing.T) {
	skipWithoutGit(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	m := NewManager(tmpDir, true)

	// Use a small public repo for testing
	repoURL := "https://github.com/octocat/Hello-World.git"

	path, err := m.Clone(repoURL)
	if err != nil {
		t.Fatalf("Clone() error = %v", err)
	}

	// Verify it's a bare repo
	if _, err := os.Stat(filepath.Join(path, "HEAD")); err != nil {
		t.Error("cloned repo doesn't have HEAD file (not a bare repo)")
	}

	// Clone again should be idempotent
	path2, err := m.Clone(repoURL)
	if err != nil {
		t.Fatalf("Clone() second call error = %v", err)
	}
	if path != path2 {
		t.Error("Clone() should return same path on second call")
	}

	// IsCloned should return true
	if !m.IsCloned(repoURL) {
		t.Error("IsCloned() should return true after clone")
	}
}

func TestManager_Worktree_Integration(t *testing.T) {
	skipWithoutGit(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	m := NewManager(tmpDir, true)

	repoURL := "https://github.com/octocat/Hello-World.git"

	// Clone first
	_, err := m.Clone(repoURL)
	if err != nil {
		t.Fatalf("Clone() error = %v", err)
	}

	// Create worktree
	sessionPath, err := m.CreateWorktree(repoURL, "session-123", "master")
	if err != nil {
		t.Fatalf("CreateWorktree() error = %v", err)
	}

	// Verify worktree has .git file (not directory)
	gitPath := filepath.Join(sessionPath, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		t.Fatalf("worktree .git not found: %v", err)
	}
	if info.IsDir() {
		t.Error("worktree .git should be a file, not a directory")
	}

	// Remove worktree
	err = m.RemoveWorktree(repoURL, "session-123")
	if err != nil {
		t.Fatalf("RemoveWorktree() error = %v", err)
	}

	// Verify directory was removed
	if _, err := os.Stat(sessionPath); !os.IsNotExist(err) {
		t.Error("worktree directory was not removed")
	}
}

func TestManager_NonWorktreeMode(t *testing.T) {
	skipWithoutGit(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	m := NewManager(tmpDir, false) // worktree disabled

	repoURL := "https://github.com/octocat/Hello-World.git"

	// CreateWorktree should fall back to CloneForSession
	sessionPath, err := m.CreateWorktree(repoURL, "session-123", "")
	if err != nil {
		t.Fatalf("CreateWorktree() (non-worktree mode) error = %v", err)
	}

	// Verify it's a full clone (has .git directory)
	gitPath := filepath.Join(sessionPath, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		t.Fatalf("clone .git not found: %v", err)
	}
	if !info.IsDir() {
		t.Error("full clone .git should be a directory")
	}
}

func TestManager_reposDir(t *testing.T) {
	m := NewManager("/data", true)
	if got := m.reposDir(); got != "/data/repos" {
		t.Errorf("reposDir() = %q, want %q", got, "/data/repos")
	}
}

func TestManager_sessionsDir(t *testing.T) {
	m := NewManager("/data", true)
	if got := m.sessionsDir(); got != "/data/sessions" {
		t.Errorf("sessionsDir() = %q, want %q", got, "/data/sessions")
	}
}

func TestManager_baseRepoPath(t *testing.T) {
	m := NewManager("/data", true)
	path := m.baseRepoPath("https://github.com/test/repo")

	// Should start with repos directory
	if !strings.HasPrefix(path, "/data/repos/") {
		t.Errorf("baseRepoPath() = %q, should start with /data/repos/", path)
	}
}

func TestManager_RemoveWorktree_NonWorktreeMode(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir, false) // worktree disabled

	// Create a session directory
	sessionPath := filepath.Join(tmpDir, "sessions", "test-session")
	if err := os.MkdirAll(sessionPath, 0755); err != nil {
		t.Fatalf("failed to create session dir: %v", err)
	}

	// RemoveWorktree should just cleanup
	err := m.RemoveWorktree("https://github.com/test/repo", "test-session")
	if err != nil {
		t.Errorf("RemoveWorktree() error = %v", err)
	}

	// Verify removed
	if _, err := os.Stat(sessionPath); !os.IsNotExist(err) {
		t.Error("session directory should be removed")
	}
}

func TestManager_RemoveWorktree_NoBaseRepo(t *testing.T) {
	skipWithoutGit(t)

	tmpDir := t.TempDir()
	m := NewManager(tmpDir, true)

	// Create a session directory
	sessionPath := filepath.Join(tmpDir, "sessions", "test-session")
	if err := os.MkdirAll(sessionPath, 0755); err != nil {
		t.Fatalf("failed to create session dir: %v", err)
	}

	// RemoveWorktree without base repo should just cleanup
	err := m.RemoveWorktree("https://github.com/nonexistent/repo", "test-session")
	if err != nil {
		t.Errorf("RemoveWorktree() error = %v", err)
	}
}

func TestManager_CreateSessionDir_Multiple(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir, true)

	// Create multiple session directories
	for i := 0; i < 3; i++ {
		sessionID := fmt.Sprintf("session-%d", i)
		path, err := m.CreateSessionDir(sessionID)
		if err != nil {
			t.Fatalf("CreateSessionDir() error = %v", err)
		}

		if _, err := os.Stat(path); err != nil {
			t.Errorf("session directory %s not created", sessionID)
		}
	}
}

func TestManager_sessionPath(t *testing.T) {
	m := NewManager("/data", true)
	path := m.sessionPath("session-123")

	expected := "/data/sessions/session-123"
	if path != expected {
		t.Errorf("sessionPath() = %q, want %q", path, expected)
	}
}

func TestNewManager(t *testing.T) {
	m := NewManager("/test/data", true)

	if m.dataDir != "/test/data" {
		t.Errorf("dataDir = %q, want %q", m.dataDir, "/test/data")
	}
	if !m.useWorktree {
		t.Error("useWorktree should be true")
	}

	m2 := NewManager("/test/data2", false)
	if m2.useWorktree {
		t.Error("useWorktree should be false")
	}
}

func TestManager_CreateSessionDir_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir, true)

	// Create same session directory twice
	path1, err := m.CreateSessionDir("test-session")
	if err != nil {
		t.Fatalf("CreateSessionDir() first call error = %v", err)
	}

	path2, err := m.CreateSessionDir("test-session")
	if err != nil {
		t.Fatalf("CreateSessionDir() second call error = %v", err)
	}

	if path1 != path2 {
		t.Errorf("CreateSessionDir() paths differ: %q vs %q", path1, path2)
	}
}

func TestManager_CloneForSession_InvalidURL(t *testing.T) {
	skipWithoutGit(t)

	tmpDir := t.TempDir()
	m := NewManager(tmpDir, false)

	_, err := m.CloneForSession("not-a-valid-git-url", "session-123")
	if err == nil {
		t.Error("CloneForSession() with invalid URL should error")
	}
}

func TestManager_CreateSessionDir_WithParentCreation(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir, true)

	// Sessions directory doesn't exist yet
	sessionsDir := filepath.Join(tmpDir, "sessions")
	if _, err := os.Stat(sessionsDir); !os.IsNotExist(err) {
		t.Skip("sessions directory already exists")
	}

	// CreateSessionDir should create parent directories
	path, err := m.CreateSessionDir("test-session")
	if err != nil {
		t.Fatalf("CreateSessionDir() error = %v", err)
	}

	// Verify both parent and session directory exist
	if _, err := os.Stat(sessionsDir); err != nil {
		t.Error("sessions directory should be created")
	}
	if _, err := os.Stat(path); err != nil {
		t.Error("session directory should be created")
	}
}

func TestRepoHash_Consistency(t *testing.T) {
	urls := []string{
		"https://github.com/test/repo",
		"https://github.com/other/project",
		"git@github.com:user/repo.git",
		"",
	}

	for _, url := range urls {
		hash1 := repoHash(url)
		hash2 := repoHash(url)
		if hash1 != hash2 {
			t.Errorf("repoHash(%q) not consistent: %q vs %q", url, hash1, hash2)
		}
	}
}
