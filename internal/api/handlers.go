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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/savaki/shannon/internal/session"
	"github.com/savaki/shannon/internal/storage"
)

// JSON response helpers

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// Health check

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Session handlers

type createSessionRequest struct {
	RepoURL string `json:"repo_url"`
	Branch  string `json:"branch"`
	Prompt  string `json:"prompt"`
}

type sessionResponse struct {
	ID        string `json:"id"`
	WorkDir   string `json:"work_dir"`
	RepoURL   string `json:"repo_url,omitempty"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

func toSessionResponse(sess *session.Session) sessionResponse {
	return sessionResponse{
		ID:        sess.ID,
		WorkDir:   sess.WorkDir,
		RepoURL:   sess.RepoURL,
		Status:    string(sess.Status()),
		CreatedAt: sess.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req createSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate input lengths
	if len(req.Prompt) > MaxPromptLength {
		writeError(w, http.StatusBadRequest, "prompt exceeds maximum length")
		return
	}
	if len(req.RepoURL) > MaxRepoURLLength {
		writeError(w, http.StatusBadRequest, "repo_url exceeds maximum length")
		return
	}
	if len(req.Branch) > MaxBranchLength {
		writeError(w, http.StatusBadRequest, "branch exceeds maximum length")
		return
	}

	sess, err := s.sessions.Create(r.Context(), session.CreateOptions{
		RepoURL: req.RepoURL,
		Branch:  req.Branch,
		Prompt:  req.Prompt,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, toSessionResponse(sess))
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := s.sessions.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response := make([]sessionResponse, len(sessions))
	for i, sess := range sessions {
		response[i] = toSessionResponse(sess)
	}

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing session id")
		return
	}

	sess, err := s.sessions.Get(id)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, toSessionResponse(sess))
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing session id")
		return
	}

	err := s.sessions.Delete(id)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type sessionInputRequest struct {
	Input string `json:"input"`
}

func (s *Server) handleSessionInput(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing session id")
		return
	}

	var req sessionInputRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Input == "" {
		writeError(w, http.StatusBadRequest, "input is required")
		return
	}

	if len(req.Input) > MaxInputLength {
		writeError(w, http.StatusBadRequest, "input exceeds maximum length")
		return
	}

	err := s.sessions.Send(id, req.Input)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

func (s *Server) handleSessionHistory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing session id")
		return
	}

	messages, err := s.sessions.GetHistory(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, messages)
}

// SSE streaming handler

func (s *Server) handleSessionStream(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing session id")
		return
	}

	eventCh, err := s.sessions.Subscribe(id)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // For nginx

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Send initial connection event
	fmt.Fprintf(w, "event: connected\ndata: {\"session_id\":\"%s\"}\n\n", id)
	flusher.Flush()

	// Stream events
	for {
		select {
		case event, ok := <-eventCh:
			if !ok {
				// Channel closed, session ended
				fmt.Fprintf(w, "event: closed\ndata: {\"reason\":\"session ended\"}\n\n")
				flusher.Flush()
				return
			}

			// Send event
			data, _ := json.Marshal(event)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
			flusher.Flush()

		case <-r.Context().Done():
			// Client disconnected
			return
		}
	}
}

// Device handlers

type registerDeviceRequest struct {
	Token    string `json:"token"`
	Platform string `json:"platform"`
}

func (s *Server) handleRegisterDevice(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement when push notifications are added
	writeError(w, http.StatusNotImplemented, "push notifications not yet implemented")
}

func (s *Server) handleUnregisterDevice(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement when push notifications are added
	writeError(w, http.StatusNotImplemented, "push notifications not yet implemented")
}
