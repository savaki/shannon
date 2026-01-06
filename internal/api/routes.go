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

import "net/http"

// registerRoutes sets up all API routes.
func (s *Server) registerRoutes(mux *http.ServeMux) {
	// Health check (no auth)
	mux.HandleFunc("GET /health", s.handleHealth)

	// Session endpoints (with auth and body limits for POST)
	mux.HandleFunc("POST /sessions", s.withAuth(withBodyLimit(s.handleCreateSession)))
	mux.HandleFunc("GET /sessions", s.withAuth(s.handleListSessions))
	mux.HandleFunc("GET /sessions/{id}", s.withAuth(s.handleGetSession))
	mux.HandleFunc("DELETE /sessions/{id}", s.withAuth(s.handleDeleteSession))
	mux.HandleFunc("POST /sessions/{id}/input", s.withAuth(withBodyLimit(s.handleSessionInput)))
	mux.HandleFunc("GET /sessions/{id}/stream", s.withAuth(s.handleSessionStream))
	mux.HandleFunc("GET /sessions/{id}/history", s.withAuth(s.handleSessionHistory))

	// Device registration (with auth and body limits)
	mux.HandleFunc("POST /devices", s.withAuth(withBodyLimit(s.handleRegisterDevice)))
	mux.HandleFunc("DELETE /devices/{id}", s.withAuth(s.handleUnregisterDevice))
}
