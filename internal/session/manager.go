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
	"fmt"
	"sync"

	"github.com/savaki/shannon/internal/claude"
	"github.com/savaki/shannon/internal/git"
	"github.com/savaki/shannon/internal/storage"
)

// WrapperFactory creates Claude wrappers. This allows injection of mock wrappers for testing.
type WrapperFactory func(opts claude.Options) claude.ClaudeWrapper

// DefaultWrapperFactory creates real Claude wrappers.
func DefaultWrapperFactory(opts claude.Options) claude.ClaudeWrapper {
	return claude.New(opts)
}

// Manager handles session lifecycle operations.
type Manager struct {
	store          *storage.Store
	gitMgr         *git.Manager
	dataDir        string
	wrapperFactory WrapperFactory

	sessions   map[string]*Session
	sessionsMu sync.RWMutex

	// Global event channel for push notifications
	eventCh     chan Event
	subscribers []chan Event
	subMu       sync.RWMutex
}

// NewManager creates a new session manager.
func NewManager(store *storage.Store, gitMgr *git.Manager, dataDir string) *Manager {
	return NewManagerWithFactory(store, gitMgr, dataDir, DefaultWrapperFactory)
}

// NewManagerWithFactory creates a new session manager with a custom wrapper factory.
// This is primarily used for testing with mock wrappers.
func NewManagerWithFactory(store *storage.Store, gitMgr *git.Manager, dataDir string, factory WrapperFactory) *Manager {
	m := &Manager{
		store:          store,
		gitMgr:         gitMgr,
		dataDir:        dataDir,
		wrapperFactory: factory,
		sessions:       make(map[string]*Session),
		eventCh:        make(chan Event, 100),
	}

	go m.broadcastEvents()

	return m
}

// Create creates a new session.
func (m *Manager) Create(ctx context.Context, opts CreateOptions) (*Session, error) {
	var workDir string
	var err error

	// Setup workspace
	if opts.RepoURL != "" {
		// Clone or create worktree
		if _, err := m.gitMgr.Clone(opts.RepoURL); err != nil {
			return nil, fmt.Errorf("failed to clone repository: %w", err)
		}
	}

	// Create storage record first to get session ID
	storageSession, err := m.store.CreateSession("", opts.RepoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create session record: %w", err)
	}

	// Create workspace directory
	if opts.RepoURL != "" {
		workDir, err = m.gitMgr.CreateWorktree(opts.RepoURL, storageSession.ID, opts.Branch)
		if err != nil {
			m.store.DeleteSession(storageSession.ID)
			return nil, fmt.Errorf("failed to create workspace: %w", err)
		}
	} else {
		workDir, err = m.gitMgr.CreateSessionDir(storageSession.ID)
		if err != nil {
			m.store.DeleteSession(storageSession.ID)
			return nil, fmt.Errorf("failed to create session directory: %w", err)
		}
	}

	// Update storage with work directory
	storageSession.WorkDir = workDir

	// Create Claude wrapper using factory
	wrapper := m.wrapperFactory(claude.Options{
		WorkDir: workDir,
		Prompt:  opts.Prompt,
	})

	session := &Session{
		Session: storageSession,
		wrapper: wrapper,
	}

	// Store in memory
	m.sessionsMu.Lock()
	m.sessions[storageSession.ID] = session
	m.sessionsMu.Unlock()

	// Start Claude process
	if err := wrapper.Start(ctx); err != nil {
		m.cleanup(storageSession.ID, opts.RepoURL)
		return nil, fmt.Errorf("failed to start claude: %w", err)
	}

	// Update status
	m.store.UpdateSessionStatus(storageSession.ID, storage.SessionStatusRunning)

	// Forward events
	go m.forwardEvents(storageSession.ID, wrapper)

	// Send initial prompt if provided
	if opts.Prompt != "" {
		if err := wrapper.Send(opts.Prompt); err != nil {
			// Log but don't fail - session is running
		}
	}

	return session, nil
}

// Get retrieves a session by ID.
func (m *Manager) Get(id string) (*Session, error) {
	m.sessionsMu.RLock()
	session, ok := m.sessions[id]
	m.sessionsMu.RUnlock()

	if ok {
		return session, nil
	}

	// Try to load from storage
	storageSession, err := m.store.GetSession(id)
	if err != nil {
		return nil, err
	}

	return &Session{Session: storageSession}, nil
}

// List returns all sessions.
func (m *Manager) List() ([]*Session, error) {
	storageSessions, err := m.store.ListSessions()
	if err != nil {
		return nil, err
	}

	sessions := make([]*Session, len(storageSessions))
	m.sessionsMu.RLock()
	for i, ss := range storageSessions {
		if active, ok := m.sessions[ss.ID]; ok {
			sessions[i] = active
		} else {
			sessions[i] = &Session{Session: ss}
		}
	}
	m.sessionsMu.RUnlock()

	return sessions, nil
}

// Stop stops a running session.
func (m *Manager) Stop(id string) error {
	m.sessionsMu.Lock()
	session, ok := m.sessions[id]
	m.sessionsMu.Unlock()

	if !ok {
		return storage.ErrNotFound
	}

	if session.wrapper != nil {
		if err := session.wrapper.Stop(); err != nil {
			return fmt.Errorf("failed to stop claude: %w", err)
		}
	}

	m.store.UpdateSessionStatus(id, storage.SessionStatusStopped)

	return nil
}

// Send sends input to a running session.
func (m *Manager) Send(id string, input string) error {
	m.sessionsMu.RLock()
	session, ok := m.sessions[id]
	m.sessionsMu.RUnlock()

	if !ok {
		return storage.ErrNotFound
	}

	if session.wrapper == nil {
		return fmt.Errorf("session not running")
	}

	// Store message
	m.store.CreateMessage(id, storage.MessageRoleUser, input)

	// Send to Claude
	return session.wrapper.Send(input)
}

// Subscribe returns a channel for session output events.
func (m *Manager) Subscribe(id string) (<-chan Event, error) {
	m.sessionsMu.RLock()
	session, ok := m.sessions[id]
	m.sessionsMu.RUnlock()

	if !ok {
		return nil, storage.ErrNotFound
	}

	if session.wrapper == nil {
		return nil, fmt.Errorf("session not running")
	}

	// Create wrapped channel
	ch := make(chan Event, 100)

	// Subscribe to claude events and convert
	claudeCh := session.wrapper.Subscribe()
	go func() {
		defer close(ch)
		for event := range claudeCh {
			ch <- EventFromClaude(id, event)
		}
	}()

	return ch, nil
}

// SubscribeAll returns a channel for events from all sessions.
func (m *Manager) SubscribeAll() <-chan Event {
	ch := make(chan Event, 100)

	m.subMu.Lock()
	m.subscribers = append(m.subscribers, ch)
	m.subMu.Unlock()

	return ch
}

// Delete stops and removes a session.
func (m *Manager) Delete(id string) error {
	// Get session info for cleanup
	m.sessionsMu.RLock()
	session, ok := m.sessions[id]
	m.sessionsMu.RUnlock()

	var repoURL string
	if ok {
		repoURL = session.RepoURL
		if session.wrapper != nil {
			session.wrapper.Stop()
		}
	} else {
		// Load from storage for cleanup info
		ss, err := m.store.GetSession(id)
		if err != nil {
			return err
		}
		repoURL = ss.RepoURL
	}

	// Remove from memory
	m.sessionsMu.Lock()
	delete(m.sessions, id)
	m.sessionsMu.Unlock()

	// Cleanup workspace
	m.cleanup(id, repoURL)

	// Delete from storage (cascades to messages)
	return m.store.DeleteSession(id)
}

// GetHistory returns the conversation history for a session.
func (m *Manager) GetHistory(id string) ([]*storage.Message, error) {
	return m.store.GetMessages(id)
}

// cleanup removes session workspace.
func (m *Manager) cleanup(sessionID, repoURL string) {
	if repoURL != "" {
		m.gitMgr.RemoveWorktree(repoURL, sessionID)
	} else {
		m.gitMgr.Cleanup(sessionID)
	}
}

// forwardEvents forwards Claude events to storage and global broadcast.
func (m *Manager) forwardEvents(sessionID string, wrapper claude.ClaudeWrapper) {
	ch := wrapper.Subscribe()
	for event := range ch {
		// Store assistant messages
		if event.Type == claude.EventTypeOutput {
			m.store.CreateMessage(sessionID, storage.MessageRoleAssistant, event.Content)
		}

		// Broadcast to global subscribers
		sessionEvent := EventFromClaude(sessionID, event)
		select {
		case m.eventCh <- sessionEvent:
		default:
		}
	}
}

// broadcastEvents broadcasts events to all subscribers.
func (m *Manager) broadcastEvents() {
	for event := range m.eventCh {
		m.subMu.RLock()
		for _, ch := range m.subscribers {
			select {
			case ch <- event:
			default:
			}
		}
		m.subMu.RUnlock()
	}
}
