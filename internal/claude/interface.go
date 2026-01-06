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

import "context"

// ClaudeWrapper defines the interface for interacting with Claude Code.
// This interface enables mocking for testing purposes.
type ClaudeWrapper interface {
	// Start starts the Claude Code process.
	Start(ctx context.Context) error

	// Send sends input to the Claude process.
	Send(input string) error

	// Stop terminates the Claude process.
	Stop() error

	// Subscribe returns a channel that receives output events.
	Subscribe() <-chan Event

	// Unsubscribe removes a subscriber channel.
	Unsubscribe(ch <-chan Event)

	// Status returns the current process status.
	Status() Status

	// Done returns a channel that closes when the process exits.
	Done() <-chan struct{}
}

// Ensure Wrapper implements ClaudeWrapper
var _ ClaudeWrapper = (*Wrapper)(nil)
