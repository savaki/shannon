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

// Package claude provides a wrapper for the Claude Code CLI.
package claude

import "time"

// EventType indicates the type of output event.
type EventType string

const (
	EventTypeOutput   EventType = "output"   // Regular output text
	EventTypeQuestion EventType = "question" // Claude is asking a question
	EventTypeTool     EventType = "tool"     // Tool usage notification
	EventTypeStatus   EventType = "status"   // Status change
	EventTypeError    EventType = "error"    // Error occurred
)

// Event represents an output event from Claude Code.
type Event struct {
	Type      EventType `json:"type"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// Status represents the current state of a Claude process.
type Status string

const (
	StatusStarting     Status = "starting"
	StatusRunning      Status = "running"
	StatusWaitingInput Status = "waiting_input"
	StatusStopped      Status = "stopped"
	StatusError        Status = "error"
)
