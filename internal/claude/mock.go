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

package claude

import (
	"context"
	"sync"
	"time"
)

// MockWrapper is a mock implementation of ClaudeWrapper for testing.
type MockWrapper struct {
	mu          sync.RWMutex
	status      Status
	done        chan struct{}
	subscribers []chan Event
	subMu       sync.RWMutex

	// Configurable behavior
	StartErr    error
	SendErr     error
	StopErr     error
	AutoRespond bool   // If true, automatically emit response events
	Response    string // Response content when AutoRespond is true

	// Tracking
	StartCalled bool
	SendCalls   []string
	StopCalled  bool
}

// NewMockWrapper creates a new mock wrapper for testing.
func NewMockWrapper() *MockWrapper {
	return &MockWrapper{
		status: StatusStopped,
		done:   make(chan struct{}),
	}
}

// Start implements ClaudeWrapper.
func (m *MockWrapper) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.StartCalled = true

	if m.StartErr != nil {
		return m.StartErr
	}

	m.status = StatusRunning
	return nil
}

// Send implements ClaudeWrapper.
func (m *MockWrapper) Send(input string) error {
	m.mu.Lock()
	m.SendCalls = append(m.SendCalls, input)
	m.mu.Unlock()

	if m.SendErr != nil {
		return m.SendErr
	}

	// Emit input echo
	m.emit(Event{
		Type:      EventTypeOutput,
		Content:   "> " + input + "\n",
		Timestamp: time.Now(),
	})

	// Auto-respond if configured
	if m.AutoRespond {
		response := m.Response
		if response == "" {
			response = "Mock response to: " + input
		}
		m.emit(Event{
			Type:      EventTypeOutput,
			Content:   response + "\n",
			Timestamp: time.Now(),
		})
	}

	return nil
}

// Stop implements ClaudeWrapper.
func (m *MockWrapper) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.StopCalled = true

	if m.StopErr != nil {
		return m.StopErr
	}

	if m.status != StatusStopped {
		m.status = StatusStopped
		select {
		case <-m.done:
			// Already closed
		default:
			close(m.done)
		}
	}

	return nil
}

// Subscribe implements ClaudeWrapper.
func (m *MockWrapper) Subscribe() <-chan Event {
	ch := make(chan Event, 100)

	m.subMu.Lock()
	m.subscribers = append(m.subscribers, ch)
	m.subMu.Unlock()

	return ch
}

// Unsubscribe implements ClaudeWrapper.
func (m *MockWrapper) Unsubscribe(ch <-chan Event) {
	m.subMu.Lock()
	defer m.subMu.Unlock()

	for i, sub := range m.subscribers {
		if sub == ch {
			close(sub)
			m.subscribers = append(m.subscribers[:i], m.subscribers[i+1:]...)
			return
		}
	}
}

// Status implements ClaudeWrapper.
func (m *MockWrapper) Status() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

// Done implements ClaudeWrapper.
func (m *MockWrapper) Done() <-chan struct{} {
	return m.done
}

// SetStatus sets the mock status (for testing).
func (m *MockWrapper) SetStatus(status Status) {
	m.mu.Lock()
	m.status = status
	m.mu.Unlock()
}

// emit sends an event to all subscribers.
func (m *MockWrapper) emit(event Event) {
	m.subMu.RLock()
	defer m.subMu.RUnlock()

	for _, ch := range m.subscribers {
		select {
		case ch <- event:
		default:
			// Subscriber not keeping up
		}
	}
}

// EmitEvent allows tests to inject events.
func (m *MockWrapper) EmitEvent(event Event) {
	m.emit(event)
}

// EmitOutput is a convenience method to emit an output event.
func (m *MockWrapper) EmitOutput(content string) {
	m.emit(Event{
		Type:      EventTypeOutput,
		Content:   content,
		Timestamp: time.Now(),
	})
}

// EmitQuestion is a convenience method to emit a question event.
func (m *MockWrapper) EmitQuestion(content string) {
	m.SetStatus(StatusWaitingInput)
	m.emit(Event{
		Type:      EventTypeQuestion,
		Content:   content,
		Timestamp: time.Now(),
	})
}

// Ensure MockWrapper implements ClaudeWrapper
var _ ClaudeWrapper = (*MockWrapper)(nil)
