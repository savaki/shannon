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
	"testing"
	"time"

	"github.com/savaki/shannon/internal/claude"
	"github.com/savaki/shannon/internal/storage"
)

func TestSession_Status_WithWrapper(t *testing.T) {
	wrapper := claude.New(claude.Options{WorkDir: "/tmp/test"})

	tests := []struct {
		claudeStatus claude.Status
		want         storage.SessionStatus
	}{
		{claude.StatusRunning, storage.SessionStatusRunning},
		{claude.StatusWaitingInput, storage.SessionStatusWaitingInput},
		{claude.StatusStopped, storage.SessionStatusStopped},
		{claude.StatusError, storage.SessionStatusError},
	}

	for _, tt := range tests {
		t.Run(string(tt.claudeStatus), func(t *testing.T) {
			// Set wrapper status directly (internal)
			wrapper.SetStatusForTesting(tt.claudeStatus)

			sess := &Session{
				Session: &storage.Session{
					Status: storage.SessionStatusPending,
				},
				wrapper: wrapper,
			}

			if got := sess.Status(); got != tt.want {
				t.Errorf("Status() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEventFromClaude(t *testing.T) {
	now := time.Now()
	claudeEvent := claude.Event{
		Type:      claude.EventTypeOutput,
		Content:   "test content",
		Timestamp: now,
	}

	event := EventFromClaude("sess-123", claudeEvent)

	if event.SessionID != "sess-123" {
		t.Errorf("SessionID = %q, want %q", event.SessionID, "sess-123")
	}
	if event.Type != string(claude.EventTypeOutput) {
		t.Errorf("Type = %q, want %q", event.Type, string(claude.EventTypeOutput))
	}
	if event.Content != "test content" {
		t.Errorf("Content = %q, want %q", event.Content, "test content")
	}
	if !event.Timestamp.Equal(now) {
		t.Errorf("Timestamp = %v, want %v", event.Timestamp, now)
	}
}

func TestEventFromClaude_AllTypes(t *testing.T) {
	types := []claude.EventType{
		claude.EventTypeOutput,
		claude.EventTypeQuestion,
		claude.EventTypeTool,
		claude.EventTypeStatus,
		claude.EventTypeError,
	}

	for _, eventType := range types {
		t.Run(string(eventType), func(t *testing.T) {
			claudeEvent := claude.Event{
				Type:      eventType,
				Content:   "content",
				Timestamp: time.Now(),
			}

			event := EventFromClaude("sess", claudeEvent)
			if event.Type != string(eventType) {
				t.Errorf("Type = %q, want %q", event.Type, string(eventType))
			}
		})
	}
}
