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

// Package api provides the HTTP API server for shannon.
package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/savaki/shannon/internal/auth"
	"github.com/savaki/shannon/internal/session"
)

// Server is the HTTP API server.
type Server struct {
	addr       string
	sessions   *session.Manager
	psk        *auth.PSKManager
	httpServer *http.Server
}

// NewServer creates a new API server.
func NewServer(addr string, sessions *session.Manager, psk *auth.PSKManager) *Server {
	s := &Server{
		addr:     addr,
		sessions: sessions,
		psk:      psk,
	}

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // Disable for SSE
		IdleTimeout:  120 * time.Second,
	}

	return s
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	slog.Info("API server listening", "addr", s.addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
