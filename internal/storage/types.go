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

// Package storage provides SQLite-based persistence for shannon.
package storage

import "time"

// SessionStatus represents the current state of a session.
type SessionStatus string

const (
	SessionStatusPending      SessionStatus = "pending"
	SessionStatusRunning      SessionStatus = "running"
	SessionStatusWaitingInput SessionStatus = "waiting_input"
	SessionStatusStopped      SessionStatus = "stopped"
	SessionStatusError        SessionStatus = "error"
)

// Session represents a Claude Code session.
type Session struct {
	ID        string        `json:"id"`
	WorkDir   string        `json:"work_dir"`
	RepoURL   string        `json:"repo_url,omitempty"`
	Status    SessionStatus `json:"status"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// MessageRole indicates who sent the message.
type MessageRole string

const (
	MessageRoleUser      MessageRole = "user"
	MessageRoleAssistant MessageRole = "assistant"
	MessageRoleSystem    MessageRole = "system"
)

// Message represents a single message in a session conversation.
type Message struct {
	ID        string      `json:"id"`
	SessionID string      `json:"session_id"`
	Role      MessageRole `json:"role"`
	Content   string      `json:"content"`
	CreatedAt time.Time   `json:"created_at"`
}

// Device represents a registered mobile device for push notifications.
type Device struct {
	ID        string    `json:"id"`
	Token     string    `json:"token"`
	Platform  string    `json:"platform"` // ios, android
	CreatedAt time.Time `json:"created_at"`
}
