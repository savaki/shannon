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
	"errors"
	"testing"
	"time"
)

func TestMockWrapper_New(t *testing.T) {
	mock := NewMockWrapper()
	if mock == nil {
		t.Fatal("NewMockWrapper() returned nil")
	}

	if mock.Status() != StatusStopped {
		t.Errorf("initial Status() = %v, want %v", mock.Status(), StatusStopped)
	}
}

func TestMockWrapper_Start(t *testing.T) {
	mock := NewMockWrapper()

	err := mock.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if !mock.StartCalled {
		t.Error("StartCalled should be true")
	}

	if mock.Status() != StatusRunning {
		t.Errorf("Status() = %v, want %v", mock.Status(), StatusRunning)
	}
}

func TestMockWrapper_Start_Error(t *testing.T) {
	mock := NewMockWrapper()
	mock.StartErr = errors.New("start error")

	err := mock.Start(context.Background())
	if err == nil {
		t.Error("Start() should error when StartErr is set")
	}
}

func TestMockWrapper_Send(t *testing.T) {
	mock := NewMockWrapper()

	err := mock.Send("test input")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if len(mock.SendCalls) != 1 {
		t.Fatalf("SendCalls length = %d, want 1", len(mock.SendCalls))
	}

	if mock.SendCalls[0] != "test input" {
		t.Errorf("SendCalls[0] = %q, want %q", mock.SendCalls[0], "test input")
	}
}

func TestMockWrapper_Send_Error(t *testing.T) {
	mock := NewMockWrapper()
	mock.SendErr = errors.New("send error")

	err := mock.Send("test")
	if err == nil {
		t.Error("Send() should error when SendErr is set")
	}
}

func TestMockWrapper_Send_AutoRespond(t *testing.T) {
	mock := NewMockWrapper()
	mock.AutoRespond = true
	mock.Response = "Custom response"

	ch := mock.Subscribe()

	err := mock.Send("input")
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	// Should receive echo and response
	events := []Event{}
collectEvents:
	for i := 0; i < 2; i++ {
		select {
		case e := <-ch:
			events = append(events, e)
		case <-time.After(100 * time.Millisecond):
			break collectEvents
		}
	}

	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}
}

func TestMockWrapper_Stop(t *testing.T) {
	mock := NewMockWrapper()
	mock.Start(context.Background())

	err := mock.Stop()
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	if !mock.StopCalled {
		t.Error("StopCalled should be true")
	}

	if mock.Status() != StatusStopped {
		t.Errorf("Status() = %v, want %v", mock.Status(), StatusStopped)
	}
}

func TestMockWrapper_Stop_Error(t *testing.T) {
	mock := NewMockWrapper()
	mock.Start(context.Background())
	mock.StopErr = errors.New("stop error")

	err := mock.Stop()
	if err == nil {
		t.Error("Stop() should error when StopErr is set")
	}
}

func TestMockWrapper_Subscribe(t *testing.T) {
	mock := NewMockWrapper()

	ch := mock.Subscribe()
	if ch == nil {
		t.Error("Subscribe() returned nil")
	}
}

func TestMockWrapper_Unsubscribe(t *testing.T) {
	mock := NewMockWrapper()

	ch := mock.Subscribe()
	mock.Unsubscribe(ch)

	// Channel should be closed
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("channel should be closed after unsubscribe")
		}
	default:
		// Channel might be empty but closed
	}
}

func TestMockWrapper_Done(t *testing.T) {
	mock := NewMockWrapper()

	done := mock.Done()
	if done == nil {
		t.Error("Done() returned nil")
	}

	// Should not be closed initially
	select {
	case <-done:
		t.Error("done channel should not be closed initially")
	default:
	}

	// After stop, should be closed
	mock.Start(context.Background())
	mock.Stop()

	select {
	case <-done:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("done channel should be closed after stop")
	}
}

func TestMockWrapper_SetStatus(t *testing.T) {
	mock := NewMockWrapper()

	mock.SetStatus(StatusRunning)
	if mock.Status() != StatusRunning {
		t.Errorf("Status() = %v, want %v", mock.Status(), StatusRunning)
	}

	mock.SetStatus(StatusWaitingInput)
	if mock.Status() != StatusWaitingInput {
		t.Errorf("Status() = %v, want %v", mock.Status(), StatusWaitingInput)
	}
}

func TestMockWrapper_EmitEvent(t *testing.T) {
	mock := NewMockWrapper()

	ch := mock.Subscribe()

	event := Event{
		Type:      EventTypeOutput,
		Content:   "test content",
		Timestamp: time.Now(),
	}
	mock.EmitEvent(event)

	select {
	case e := <-ch:
		if e.Content != "test content" {
			t.Errorf("Content = %q, want %q", e.Content, "test content")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("did not receive event")
	}
}

func TestMockWrapper_EmitOutput(t *testing.T) {
	mock := NewMockWrapper()

	ch := mock.Subscribe()
	mock.EmitOutput("output content")

	select {
	case e := <-ch:
		if e.Type != EventTypeOutput {
			t.Errorf("Type = %v, want %v", e.Type, EventTypeOutput)
		}
		if e.Content != "output content" {
			t.Errorf("Content = %q, want %q", e.Content, "output content")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("did not receive event")
	}
}

func TestMockWrapper_EmitQuestion(t *testing.T) {
	mock := NewMockWrapper()

	ch := mock.Subscribe()
	mock.EmitQuestion("question content")

	// Status should change
	if mock.Status() != StatusWaitingInput {
		t.Errorf("Status() = %v, want %v", mock.Status(), StatusWaitingInput)
	}

	select {
	case e := <-ch:
		if e.Type != EventTypeQuestion {
			t.Errorf("Type = %v, want %v", e.Type, EventTypeQuestion)
		}
		if e.Content != "question content" {
			t.Errorf("Content = %q, want %q", e.Content, "question content")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("did not receive event")
	}
}

func TestMockWrapper_MultipleSubscribers(t *testing.T) {
	mock := NewMockWrapper()

	ch1 := mock.Subscribe()
	ch2 := mock.Subscribe()
	ch3 := mock.Subscribe()

	mock.EmitOutput("broadcast")

	// All subscribers should receive the event
	for i, ch := range []<-chan Event{ch1, ch2, ch3} {
		select {
		case e := <-ch:
			if e.Content != "broadcast" {
				t.Errorf("subscriber %d: Content = %q, want %q", i, e.Content, "broadcast")
			}
		case <-time.After(100 * time.Millisecond):
			t.Errorf("subscriber %d did not receive event", i)
		}
	}
}

func TestMockWrapper_ImplementsInterface(t *testing.T) {
	var _ ClaudeWrapper = (*MockWrapper)(nil)
}

func TestMockWrapper_Stop_AlreadyStopped(t *testing.T) {
	mock := NewMockWrapper()

	// Stop without starting - should not panic
	err := mock.Stop()
	if err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	// Stop again - should not panic or error
	err = mock.Stop()
	if err != nil {
		t.Fatalf("Stop() second call error = %v", err)
	}
}

func TestMockWrapper_Send_Multiple(t *testing.T) {
	mock := NewMockWrapper()

	inputs := []string{"input1", "input2", "input3"}
	for _, input := range inputs {
		if err := mock.Send(input); err != nil {
			t.Fatalf("Send(%q) error = %v", input, err)
		}
	}

	if len(mock.SendCalls) != 3 {
		t.Errorf("SendCalls length = %d, want 3", len(mock.SendCalls))
	}

	for i, input := range inputs {
		if mock.SendCalls[i] != input {
			t.Errorf("SendCalls[%d] = %q, want %q", i, mock.SendCalls[i], input)
		}
	}
}
