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
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"
)

// Wrapper manages a Claude Code CLI subprocess.
type Wrapper struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	stderr  io.ReadCloser
	workDir string

	status     Status
	statusMu   sync.RWMutex
	eventCh    chan Event
	done       chan struct{}
	subscribers []chan Event
	subMu       sync.RWMutex
}

// Options configures the Claude wrapper.
type Options struct {
	WorkDir string
	Prompt  string // Initial prompt to send
}

// New creates a new Claude wrapper but doesn't start the process.
func New(opts Options) *Wrapper {
	return &Wrapper{
		workDir: opts.WorkDir,
		status:  StatusStopped,
		eventCh: make(chan Event, 100),
		done:    make(chan struct{}),
	}
}

// Start starts the Claude Code process.
func (w *Wrapper) Start(ctx context.Context) error {
	w.statusMu.Lock()
	if w.status == StatusRunning || w.status == StatusStarting {
		w.statusMu.Unlock()
		return fmt.Errorf("process already running")
	}
	w.status = StatusStarting
	w.statusMu.Unlock()

	// Create command - use --print for non-interactive output
	w.cmd = exec.CommandContext(ctx, "claude", "--print")
	w.cmd.Dir = w.workDir
	w.cmd.Env = append(os.Environ(), "CLAUDE_CODE_ENTRYPOINT=shannon")

	var err error
	w.stdin, err = w.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	w.stdout, err = w.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	w.stderr, err = w.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := w.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start claude: %w", err)
	}

	w.setStatus(StatusRunning)

	// Start output readers
	go w.readOutput(w.stdout, false)
	go w.readOutput(w.stderr, true)
	go w.broadcastEvents()
	go w.waitForExit()

	return nil
}

// Send sends input to the Claude process.
func (w *Wrapper) Send(input string) error {
	w.statusMu.RLock()
	status := w.status
	w.statusMu.RUnlock()

	if status != StatusRunning && status != StatusWaitingInput {
		return fmt.Errorf("process not running (status: %s)", status)
	}

	if w.stdin == nil {
		return fmt.Errorf("stdin not available")
	}

	_, err := fmt.Fprintln(w.stdin, input)
	if err != nil {
		return fmt.Errorf("failed to write to stdin: %w", err)
	}

	w.emit(Event{
		Type:      EventTypeOutput,
		Content:   fmt.Sprintf("> %s\n", input),
		Timestamp: time.Now(),
	})

	return nil
}

// Stop terminates the Claude process.
func (w *Wrapper) Stop() error {
	w.statusMu.RLock()
	status := w.status
	w.statusMu.RUnlock()

	if status == StatusStopped {
		return nil
	}

	if w.cmd == nil || w.cmd.Process == nil {
		return nil
	}

	// Try graceful termination first
	if w.stdin != nil {
		w.stdin.Close()
	}

	// Give it a moment to exit gracefully
	done := make(chan struct{})
	go func() {
		w.cmd.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Exited gracefully
	case <-time.After(5 * time.Second):
		// Force kill
		w.cmd.Process.Kill()
		<-done
	}

	w.setStatus(StatusStopped)
	close(w.done)

	return nil
}

// Subscribe returns a channel that receives output events.
func (w *Wrapper) Subscribe() <-chan Event {
	ch := make(chan Event, 100)

	w.subMu.Lock()
	w.subscribers = append(w.subscribers, ch)
	w.subMu.Unlock()

	return ch
}

// Unsubscribe removes a subscriber channel.
func (w *Wrapper) Unsubscribe(ch <-chan Event) {
	w.subMu.Lock()
	defer w.subMu.Unlock()

	for i, sub := range w.subscribers {
		if sub == ch {
			close(sub)
			w.subscribers = append(w.subscribers[:i], w.subscribers[i+1:]...)
			return
		}
	}
}

// Status returns the current process status.
func (w *Wrapper) Status() Status {
	w.statusMu.RLock()
	defer w.statusMu.RUnlock()
	return w.status
}

// Done returns a channel that closes when the process exits.
func (w *Wrapper) Done() <-chan struct{} {
	return w.done
}

func (w *Wrapper) setStatus(status Status) {
	w.statusMu.Lock()
	w.status = status
	w.statusMu.Unlock()

	w.emit(Event{
		Type:      EventTypeStatus,
		Content:   string(status),
		Timestamp: time.Now(),
	})
}

func (w *Wrapper) emit(event Event) {
	select {
	case w.eventCh <- event:
	default:
		// Channel full, drop event
	}
}

func (w *Wrapper) readOutput(r io.Reader, isStderr bool) {
	scanner := bufio.NewScanner(r)
	// Increase buffer for long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		eventType := EventTypeOutput
		if isStderr {
			eventType = EventTypeError
		}

		// Check for question patterns
		if isQuestionLine(line) {
			eventType = EventTypeQuestion
			w.setStatus(StatusWaitingInput)
		}

		w.emit(Event{
			Type:      eventType,
			Content:   line + "\n",
			Timestamp: time.Now(),
		})
	}
}

func (w *Wrapper) broadcastEvents() {
	for {
		select {
		case event := <-w.eventCh:
			w.subMu.RLock()
			for _, ch := range w.subscribers {
				select {
				case ch <- event:
				default:
					// Subscriber not keeping up, drop
				}
			}
			w.subMu.RUnlock()
		case <-w.done:
			return
		}
	}
}

func (w *Wrapper) waitForExit() {
	if w.cmd != nil {
		w.cmd.Wait()
	}

	w.statusMu.Lock()
	if w.status != StatusStopped {
		w.status = StatusStopped
	}
	w.statusMu.Unlock()
}

// SetStatusForTesting sets the wrapper status for testing purposes.
// This should only be used in tests.
func (w *Wrapper) SetStatusForTesting(status Status) {
	w.statusMu.Lock()
	w.status = status
	w.statusMu.Unlock()
}
