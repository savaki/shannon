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

// Package git provides git repository management for shannon sessions.
package git

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrRepoNotCloned is returned when trying to create a worktree without a base clone.
var ErrRepoNotCloned = errors.New("repository not cloned")

// Manager handles git operations for session workspaces.
type Manager struct {
	dataDir     string
	useWorktree bool
}

// NewManager creates a new git Manager.
func NewManager(dataDir string, useWorktree bool) *Manager {
	return &Manager{
		dataDir:     dataDir,
		useWorktree: useWorktree,
	}
}

// reposDir returns the path to the repos directory.
func (m *Manager) reposDir() string {
	return filepath.Join(m.dataDir, "repos")
}

// sessionsDir returns the path to the sessions directory.
func (m *Manager) sessionsDir() string {
	return filepath.Join(m.dataDir, "sessions")
}

// repoHash returns a stable hash for a repository URL.
func repoHash(url string) string {
	h := sha256.Sum256([]byte(url))
	return hex.EncodeToString(h[:8]) // First 8 bytes = 16 hex chars
}

// baseRepoPath returns the path to the base clone for a repo URL.
func (m *Manager) baseRepoPath(repoURL string) string {
	return filepath.Join(m.reposDir(), repoHash(repoURL))
}

// sessionPath returns the path for a session workspace.
func (m *Manager) sessionPath(sessionID string) string {
	return filepath.Join(m.sessionsDir(), sessionID)
}

// Clone clones a repository to the base repos directory.
// It is idempotent - calling it multiple times reuses the existing clone.
func (m *Manager) Clone(repoURL string) (string, error) {
	if repoURL == "" {
		return "", errors.New("repository URL is required")
	}

	basePath := m.baseRepoPath(repoURL)

	// Check if already cloned (bare repo has HEAD file directly)
	if _, err := os.Stat(filepath.Join(basePath, "HEAD")); err == nil {
		// Already cloned, fetch updates
		cmd := exec.Command("git", "fetch", "--all")
		cmd.Dir = basePath
		if output, err := cmd.CombinedOutput(); err != nil {
			return basePath, fmt.Errorf("git fetch failed: %s: %w", string(output), err)
		}
		return basePath, nil
	}

	// Ensure repos directory exists
	if err := os.MkdirAll(m.reposDir(), 0755); err != nil {
		return "", fmt.Errorf("failed to create repos directory: %w", err)
	}

	// Clone the repository
	cmd := exec.Command("git", "clone", "--bare", repoURL, basePath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git clone failed: %s: %w", strings.TrimSpace(string(output)), err)
	}

	return basePath, nil
}

// IsCloned checks if a repository has been cloned.
func (m *Manager) IsCloned(repoURL string) bool {
	basePath := m.baseRepoPath(repoURL)
	_, err := os.Stat(basePath)
	return err == nil
}

// CreateWorktree creates a git worktree for a session.
// Requires the repository to be cloned first.
func (m *Manager) CreateWorktree(repoURL, sessionID, branch string) (string, error) {
	if !m.useWorktree {
		return m.CloneForSession(repoURL, sessionID)
	}

	basePath := m.baseRepoPath(repoURL)

	// Verify base repo exists
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return "", fmt.Errorf("%w: clone the repository first", ErrRepoNotCloned)
	}

	sessionPath := m.sessionPath(sessionID)

	// Ensure sessions directory exists
	if err := os.MkdirAll(m.sessionsDir(), 0755); err != nil {
		return "", fmt.Errorf("failed to create sessions directory: %w", err)
	}

	// Determine branch to use
	if branch == "" {
		branch = "HEAD"
	}

	// Create worktree
	cmd := exec.Command("git", "worktree", "add", sessionPath, branch)
	cmd.Dir = basePath
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git worktree add failed: %s: %w", strings.TrimSpace(string(output)), err)
	}

	return sessionPath, nil
}

// CloneForSession creates a full clone for a session (non-worktree mode).
func (m *Manager) CloneForSession(repoURL, sessionID string) (string, error) {
	if repoURL == "" {
		return "", errors.New("repository URL is required")
	}

	sessionPath := m.sessionPath(sessionID)

	// Ensure sessions directory exists
	if err := os.MkdirAll(m.sessionsDir(), 0755); err != nil {
		return "", fmt.Errorf("failed to create sessions directory: %w", err)
	}

	// Clone directly to session path
	cmd := exec.Command("git", "clone", repoURL, sessionPath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git clone failed: %s: %w", strings.TrimSpace(string(output)), err)
	}

	return sessionPath, nil
}

// CreateSessionDir creates a session directory without git.
// Used when no repository URL is provided.
func (m *Manager) CreateSessionDir(sessionID string) (string, error) {
	sessionPath := m.sessionPath(sessionID)

	if err := os.MkdirAll(sessionPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create session directory: %w", err)
	}

	return sessionPath, nil
}

// RemoveWorktree removes a worktree for a session.
func (m *Manager) RemoveWorktree(repoURL, sessionID string) error {
	if !m.useWorktree {
		return m.Cleanup(sessionID)
	}

	basePath := m.baseRepoPath(repoURL)
	sessionPath := m.sessionPath(sessionID)

	// Check if base repo exists
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		// Base repo doesn't exist, just clean up the directory
		return m.Cleanup(sessionID)
	}

	// Remove worktree from git
	cmd := exec.Command("git", "worktree", "remove", "--force", sessionPath)
	cmd.Dir = basePath
	if output, err := cmd.CombinedOutput(); err != nil {
		// If worktree remove fails, try manual cleanup
		if cleanErr := m.Cleanup(sessionID); cleanErr != nil {
			return fmt.Errorf("git worktree remove failed: %s, cleanup also failed: %w", string(output), cleanErr)
		}
	}

	return nil
}

// Cleanup removes a session directory regardless of git mode.
func (m *Manager) Cleanup(sessionID string) error {
	sessionPath := m.sessionPath(sessionID)

	// Check if path exists
	if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
		return nil // Already cleaned up
	}

	if err := os.RemoveAll(sessionPath); err != nil {
		return fmt.Errorf("failed to remove session directory: %w", err)
	}

	return nil
}

// SessionPath returns the workspace path for a session.
func (m *Manager) SessionPath(sessionID string) string {
	return m.sessionPath(sessionID)
}
