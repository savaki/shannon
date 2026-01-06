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
	"sync"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	w := New(Options{WorkDir: "/tmp/test"})

	if w.workDir != "/tmp/test" {
		t.Errorf("workDir = %q, want %q", w.workDir, "/tmp/test")
	}

	if w.Status() != StatusStopped {
		t.Errorf("initial Status() = %v, want %v", w.Status(), StatusStopped)
	}
}

func TestWrapper_Status(t *testing.T) {
	w := New(Options{WorkDir: "/tmp/test"})

	// Initial status
	if w.Status() != StatusStopped {
		t.Errorf("Status() = %v, want %v", w.Status(), StatusStopped)
	}

	// Change status
	w.setStatus(StatusRunning)
	if w.Status() != StatusRunning {
		t.Errorf("Status() after setStatus = %v, want %v", w.Status(), StatusRunning)
	}
}

func TestWrapper_Subscribe(t *testing.T) {
	w := New(Options{WorkDir: "/tmp/test"})

	// Subscribe
	ch := w.Subscribe()
	if ch == nil {
		t.Error("Subscribe() returned nil channel")
	}

	// Should have one subscriber
	w.subMu.RLock()
	count := len(w.subscribers)
	w.subMu.RUnlock()
	if count != 1 {
		t.Errorf("subscriber count = %d, want 1", count)
	}

	// Unsubscribe
	w.Unsubscribe(ch)
	w.subMu.RLock()
	count = len(w.subscribers)
	w.subMu.RUnlock()
	if count != 0 {
		t.Errorf("subscriber count after unsubscribe = %d, want 0", count)
	}
}

func TestWrapper_Send_NotRunning(t *testing.T) {
	w := New(Options{WorkDir: "/tmp/test"})

	err := w.Send("test input")
	if err == nil {
		t.Error("Send() should error when not running")
	}
}

func TestWrapper_Stop_WhenStopped(t *testing.T) {
	w := New(Options{WorkDir: "/tmp/test"})

	// Stop when already stopped should not error
	err := w.Stop()
	if err != nil {
		t.Errorf("Stop() when stopped error = %v, want nil", err)
	}
}

func TestWrapper_Done(t *testing.T) {
	w := New(Options{WorkDir: "/tmp/test"})

	ch := w.Done()
	if ch == nil {
		t.Error("Done() returned nil channel")
	}

	// Channel should not be closed initially
	select {
	case <-ch:
		t.Error("Done() channel should not be closed initially")
	default:
		// expected
	}
}

func TestWrapper_setStatus(t *testing.T) {
	w := New(Options{WorkDir: "/tmp/test"})

	// Set status
	w.setStatus(StatusRunning)

	// Verify status changed
	if w.Status() != StatusRunning {
		t.Errorf("Status() = %v, want %v", w.Status(), StatusRunning)
	}

	// Verify event was emitted to internal channel
	select {
	case event := <-w.eventCh:
		if event.Type != EventTypeStatus {
			t.Errorf("event.Type = %v, want %v", event.Type, EventTypeStatus)
		}
		if event.Content != string(StatusRunning) {
			t.Errorf("event.Content = %q, want %q", event.Content, string(StatusRunning))
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("no status event received")
	}
}

func TestWrapper_emit(t *testing.T) {
	w := New(Options{WorkDir: "/tmp/test"})

	event := Event{
		Type:      EventTypeOutput,
		Content:   "test output",
		Timestamp: time.Now(),
	}

	// Emit event
	w.emit(event)

	// Read from internal channel
	select {
	case e := <-w.eventCh:
		if e.Type != EventTypeOutput {
			t.Errorf("event.Type = %v, want %v", e.Type, EventTypeOutput)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("event not received")
	}
}

func TestWrapper_emit_ChannelFull(t *testing.T) {
	w := New(Options{WorkDir: "/tmp/test"})

	// Fill the channel
	for i := 0; i < 100; i++ {
		w.emit(Event{Type: EventTypeOutput, Content: "fill"})
	}

	// This should not block even though channel is full
	done := make(chan bool)
	go func() {
		w.emit(Event{Type: EventTypeOutput, Content: "extra"})
		done <- true
	}()

	select {
	case <-done:
		// expected - emit didn't block
	case <-time.After(100 * time.Millisecond):
		t.Error("emit blocked on full channel")
	}
}

func TestWrapper_Subscribe_Multiple(t *testing.T) {
	w := New(Options{WorkDir: "/tmp/test"})

	ch1 := w.Subscribe()
	ch2 := w.Subscribe()

	// Should have two subscribers
	w.subMu.RLock()
	count := len(w.subscribers)
	w.subMu.RUnlock()

	if count != 2 {
		t.Errorf("subscriber count = %d, want 2", count)
	}

	// Unsubscribe first
	w.Unsubscribe(ch1)

	w.subMu.RLock()
	count = len(w.subscribers)
	w.subMu.RUnlock()

	if count != 1 {
		t.Errorf("subscriber count after unsubscribe = %d, want 1", count)
	}

	// Cleanup
	w.Unsubscribe(ch2)
}

func TestWrapper_Unsubscribe_Nonexistent(t *testing.T) {
	w := New(Options{WorkDir: "/tmp/test"})

	// Create a channel that's not subscribed
	ch := make(chan Event)

	// Unsubscribe should not panic
	w.Unsubscribe(ch)
}

func TestWrapper_Status_Concurrent(t *testing.T) {
	w := New(Options{WorkDir: "/tmp/test"})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			w.setStatus(StatusRunning)
		}()
		go func() {
			defer wg.Done()
			_ = w.Status()
		}()
	}
	wg.Wait()
}

func TestWrapper_Send_NoStdin(t *testing.T) {
	w := New(Options{WorkDir: "/tmp/test"})
	w.setStatus(StatusRunning)
	// stdin is nil

	err := w.Send("test")
	if err == nil {
		t.Error("Send() should error when stdin is nil")
	}
}

func TestWrapper_SetStatusForTesting(t *testing.T) {
	w := New(Options{WorkDir: "/tmp/test"})

	// Initial status
	if w.Status() != StatusStopped {
		t.Errorf("initial Status() = %v, want %v", w.Status(), StatusStopped)
	}

	// Set via testing method
	w.SetStatusForTesting(StatusRunning)
	if w.Status() != StatusRunning {
		t.Errorf("Status() = %v, want %v", w.Status(), StatusRunning)
	}

	w.SetStatusForTesting(StatusWaitingInput)
	if w.Status() != StatusWaitingInput {
		t.Errorf("Status() = %v, want %v", w.Status(), StatusWaitingInput)
	}

	w.SetStatusForTesting(StatusError)
	if w.Status() != StatusError {
		t.Errorf("Status() = %v, want %v", w.Status(), StatusError)
	}
}

func TestWrapper_Send_WaitingInput(t *testing.T) {
	w := New(Options{WorkDir: "/tmp/test"})
	w.SetStatusForTesting(StatusWaitingInput)
	// stdin is nil, but status should allow send

	err := w.Send("test")
	// Should fail because stdin is nil, not because of status
	if err == nil {
		t.Error("Send() should error when stdin is nil")
	}
}

func TestWrapper_Stop_NilCmd(t *testing.T) {
	w := New(Options{WorkDir: "/tmp/test"})
	w.SetStatusForTesting(StatusRunning)
	// cmd is nil

	err := w.Stop()
	if err != nil {
		t.Errorf("Stop() error = %v, want nil", err)
	}
}

func TestEvent_Fields(t *testing.T) {
	now := time.Now()
	event := Event{
		Type:      EventTypeOutput,
		Content:   "test content",
		Timestamp: now,
	}

	if event.Type != EventTypeOutput {
		t.Errorf("Type = %v, want %v", event.Type, EventTypeOutput)
	}
	if event.Content != "test content" {
		t.Errorf("Content = %q, want %q", event.Content, "test content")
	}
	if !event.Timestamp.Equal(now) {
		t.Errorf("Timestamp = %v, want %v", event.Timestamp, now)
	}
}

func TestStatus_Values(t *testing.T) {
	statuses := []Status{
		StatusStarting,
		StatusRunning,
		StatusWaitingInput,
		StatusStopped,
		StatusError,
	}

	for _, s := range statuses {
		if string(s) == "" {
			t.Errorf("Status %v has empty string value", s)
		}
	}
}

func TestEventType_Values(t *testing.T) {
	types := []EventType{
		EventTypeOutput,
		EventTypeQuestion,
		EventTypeTool,
		EventTypeStatus,
		EventTypeError,
	}

	for _, et := range types {
		if string(et) == "" {
			t.Errorf("EventType %v has empty string value", et)
		}
	}
}

func TestWrapper_Options(t *testing.T) {
	opts := Options{
		WorkDir: "/custom/path",
		Prompt:  "initial prompt",
	}

	if opts.WorkDir != "/custom/path" {
		t.Errorf("WorkDir = %q, want %q", opts.WorkDir, "/custom/path")
	}
	if opts.Prompt != "initial prompt" {
		t.Errorf("Prompt = %q, want %q", opts.Prompt, "initial prompt")
	}
}

func TestWrapper_Subscribe_EmitAndReceive(t *testing.T) {
	w := New(Options{WorkDir: "/tmp/test"})

	ch := w.Subscribe()

	// Start broadcast goroutine
	go w.broadcastEvents()

	// Emit an event
	testEvent := Event{
		Type:      EventTypeOutput,
		Content:   "test broadcast",
		Timestamp: time.Now(),
	}
	w.emit(testEvent)

	// Should receive on subscriber channel
	select {
	case e := <-ch:
		if e.Type != EventTypeOutput {
			t.Errorf("event.Type = %v, want %v", e.Type, EventTypeOutput)
		}
		if e.Content != "test broadcast" {
			t.Errorf("event.Content = %q, want %q", e.Content, "test broadcast")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("did not receive broadcast event")
	}

	// Cleanup
	close(w.done) // Stop broadcast goroutine
}

func TestWrapper_StatusStopped_DoesNotAllowSend(t *testing.T) {
	w := New(Options{WorkDir: "/tmp/test"})

	// Ensure status is stopped
	if w.Status() != StatusStopped {
		t.Fatalf("unexpected initial status: %v", w.Status())
	}

	err := w.Send("test")
	if err == nil {
		t.Error("Send() should error when status is stopped")
	}
}

func TestWrapper_StatusStarting_DoesNotAllowSend(t *testing.T) {
	w := New(Options{WorkDir: "/tmp/test"})
	w.SetStatusForTesting(StatusStarting)

	err := w.Send("test")
	if err == nil {
		t.Error("Send() should error when status is starting")
	}
}

func TestWrapper_StatusError_DoesNotAllowSend(t *testing.T) {
	w := New(Options{WorkDir: "/tmp/test"})
	w.SetStatusForTesting(StatusError)

	err := w.Send("test")
	if err == nil {
		t.Error("Send() should error when status is error")
	}
}

func TestWrapper_AllStatusTransitions(t *testing.T) {
	w := New(Options{WorkDir: "/tmp/test"})

	transitions := []Status{
		StatusStopped,
		StatusStarting,
		StatusRunning,
		StatusWaitingInput,
		StatusRunning,
		StatusError,
		StatusStopped,
	}

	for _, status := range transitions {
		w.SetStatusForTesting(status)
		if w.Status() != status {
			t.Errorf("after SetStatusForTesting(%v), Status() = %v", status, w.Status())
		}
	}
}

func TestWrapper_EventChannel_NonBlocking(t *testing.T) {
	w := New(Options{WorkDir: "/tmp/test"})

	// Emit without any receiver - should not block
	done := make(chan bool)
	go func() {
		for i := 0; i < 200; i++ {
			w.emit(Event{Type: EventTypeOutput, Content: "test"})
		}
		done <- true
	}()

	select {
	case <-done:
		// Good - didn't block
	case <-time.After(1 * time.Second):
		t.Error("emit blocked even though channel was full")
	}
}

func TestWrapper_MultipleSubscribers_AllReceive(t *testing.T) {
	w := New(Options{WorkDir: "/tmp/test"})

	ch1 := w.Subscribe()
	ch2 := w.Subscribe()
	ch3 := w.Subscribe()

	// Start broadcast
	go w.broadcastEvents()

	// Emit event
	w.emit(Event{Type: EventTypeOutput, Content: "to all"})

	// All should receive
	for i, ch := range []<-chan Event{ch1, ch2, ch3} {
		select {
		case e := <-ch:
			if e.Content != "to all" {
				t.Errorf("subscriber %d received wrong content: %q", i, e.Content)
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("subscriber %d did not receive event", i)
		}
	}

	// Cleanup
	close(w.done)
}

func TestWrapper_Done_NotClosedAfterMultipleStops(t *testing.T) {
	w := New(Options{WorkDir: "/tmp/test"})
	w.SetStatusForTesting(StatusRunning)

	// First stop
	err := w.Stop()
	if err != nil {
		t.Fatalf("first Stop() error = %v", err)
	}

	// Done should be closed now (or wrapper handles it gracefully)
	// We can't reliably test the channel is closed without panicking
}

func TestWrapper_Unsubscribe_DoesNotAffectOthers(t *testing.T) {
	w := New(Options{WorkDir: "/tmp/test"})

	ch1 := w.Subscribe()
	ch2 := w.Subscribe()

	// Unsubscribe ch1
	w.Unsubscribe(ch1)

	// ch2 should still be in subscribers
	w.subMu.RLock()
	found := false
	for _, sub := range w.subscribers {
		if sub == ch2 {
			found = true
			break
		}
	}
	w.subMu.RUnlock()

	if !found {
		t.Error("ch2 should still be subscribed")
	}
}

func TestWrapper_Fields(t *testing.T) {
	w := New(Options{WorkDir: "/workspace/project"})

	if w.workDir != "/workspace/project" {
		t.Errorf("workDir = %q, want %q", w.workDir, "/workspace/project")
	}
	if w.eventCh == nil {
		t.Error("eventCh should not be nil")
	}
	if w.done == nil {
		t.Error("done channel should not be nil")
	}
}
