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

package api

import (
	"net/http"
	"strings"
)

// Request size limits
const (
	// MaxRequestBodySize is the maximum size for request bodies (1MB).
	MaxRequestBodySize = 1 << 20

	// MaxPromptLength is the maximum length for prompts (100KB).
	MaxPromptLength = 100 * 1024

	// MaxInputLength is the maximum length for session input (100KB).
	MaxInputLength = 100 * 1024

	// MaxRepoURLLength is the maximum length for repository URLs.
	MaxRepoURLLength = 2048

	// MaxBranchLength is the maximum length for branch names.
	MaxBranchLength = 256
)

// withBodyLimit wraps a handler with a request body size limit.
func withBodyLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodySize)
		}
		next(w, r)
	}
}

// withAuth wraps a handler with authentication.
func (s *Server) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get Authorization header
		auth := r.Header.Get("Authorization")
		if auth == "" {
			writeError(w, http.StatusUnauthorized, "missing authorization header")
			return
		}

		// Extract Bearer token
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			writeError(w, http.StatusUnauthorized, "invalid authorization format, expected: Bearer <token>")
			return
		}

		token := parts[1]
		if !s.psk.Validate(token) {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		next(w, r)
	}
}
