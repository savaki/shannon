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
	"regexp"
	"strings"
)

// Question patterns that indicate Claude is waiting for user input
var questionPatterns = []*regexp.Regexp{
	// Direct questions
	regexp.MustCompile(`\?\s*$`),
	// Common prompts
	regexp.MustCompile(`(?i)^(y/n|yes/no|\[y/n\])`),
	regexp.MustCompile(`(?i)please (confirm|select|choose|enter|provide|specify)`),
	regexp.MustCompile(`(?i)would you like`),
	regexp.MustCompile(`(?i)do you want`),
	regexp.MustCompile(`(?i)should I`),
	regexp.MustCompile(`(?i)which (one|option)`),
	// Input prompts
	regexp.MustCompile(`(?i)enter your`),
	regexp.MustCompile(`(?i)type your`),
	regexp.MustCompile(`(?i)input:`),
	regexp.MustCompile(`(?i)response:`),
}

// Tool patterns that indicate tool usage
var toolPatterns = []*regexp.Regexp{
	// Tool name followed by colon or path-like argument
	regexp.MustCompile(`^(Read|Write|Edit|Bash|Glob|Grep|Task)[\s:]+[/\w\.\*]`),
	regexp.MustCompile(`(?i)using tool`),
	regexp.MustCompile(`(?i)^executing\b`),
}

// isQuestionLine checks if a line indicates Claude is asking a question.
func isQuestionLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}

	for _, pattern := range questionPatterns {
		if pattern.MatchString(line) {
			return true
		}
	}

	return false
}

// isToolLine checks if a line indicates tool usage.
func isToolLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}

	for _, pattern := range toolPatterns {
		if pattern.MatchString(line) {
			return true
		}
	}

	return false
}

// ParseEventType determines the event type from a line of output.
func ParseEventType(line string) EventType {
	if isQuestionLine(line) {
		return EventTypeQuestion
	}
	if isToolLine(line) {
		return EventTypeTool
	}
	return EventTypeOutput
}
