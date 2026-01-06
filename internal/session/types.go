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

// Package session provides session management for shannon.
package session

import (
	"time"

	"github.com/savaki/shannon/internal/claude"
	"github.com/savaki/shannon/internal/storage"
)

// Session represents an active Claude Code session.
type Session struct {
	*storage.Session
	wrapper claude.ClaudeWrapper
}

// CreateOptions configures session creation.
type CreateOptions struct {
	RepoURL string // Git repository URL (optional)
	Branch  string // Git branch to checkout (optional)
	Prompt  string // Initial prompt to send (optional)
}

// Status returns the current session status.
func (s *Session) Status() storage.SessionStatus {
	if s.wrapper == nil {
		return s.Session.Status
	}

	switch s.wrapper.Status() {
	case claude.StatusRunning:
		return storage.SessionStatusRunning
	case claude.StatusWaitingInput:
		return storage.SessionStatusWaitingInput
	case claude.StatusStopped:
		return storage.SessionStatusStopped
	case claude.StatusError:
		return storage.SessionStatusError
	default:
		return s.Session.Status
	}
}

// Event represents a session event to be broadcast.
type Event struct {
	SessionID string       `json:"session_id"`
	Type      string       `json:"type"`
	Content   string       `json:"content"`
	Timestamp time.Time    `json:"timestamp"`
}

// EventFromClaude converts a Claude event to a session event.
func EventFromClaude(sessionID string, e claude.Event) Event {
	return Event{
		SessionID: sessionID,
		Type:      string(e.Type),
		Content:   e.Content,
		Timestamp: e.Timestamp,
	}
}
