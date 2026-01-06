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

import "testing"

func TestIsQuestionLine(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		// Questions with ?
		{"What file would you like to edit?", true},
		{"Which option do you prefer?", true},

		// Y/N prompts
		{"y/n", true},
		{"[Y/n]", true},
		{"Yes/No", true},

		// Common question phrases
		{"Would you like me to continue?", true},
		{"Do you want to proceed?", true},
		{"Should I create a new file?", true},
		{"Please confirm the changes", true},
		{"Please select an option", true},
		{"Please choose from the following", true},
		{"Please enter your name", true},
		{"Which one should I use?", true},

		// Input prompts
		{"Enter your API key:", true},
		{"Type your response:", true},
		{"Input:", true},

		// Not questions
		{"This is a regular statement.", false},
		{"Processing files...", false},
		{"Done!", false},
		{"", false},
		{"   ", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got := isQuestionLine(tt.line)
			if got != tt.want {
				t.Errorf("isQuestionLine(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

func TestIsToolLine(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		// Tool names
		{"Read file.txt", true},
		{"Write output.json", true},
		{"Edit config.yaml", true},
		{"Bash: npm install", true},
		{"Glob *.go", true},
		{"Grep pattern", true},
		{"Task: explore", true},

		// Tool phrases
		{"Using tool to search", true},
		{"Executing command", true},

		// Not tools
		{"Reading about the topic", false},
		{"Writing a story", false},
		{"Hello world", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got := isToolLine(tt.line)
			if got != tt.want {
				t.Errorf("isToolLine(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

func TestParseEventType(t *testing.T) {
	tests := []struct {
		line string
		want EventType
	}{
		{"What would you like?", EventTypeQuestion},
		{"Read file.txt", EventTypeTool},
		{"Regular output", EventTypeOutput},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got := ParseEventType(tt.line)
			if got != tt.want {
				t.Errorf("ParseEventType(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}
