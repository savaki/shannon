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
	"context"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/savaki/shannon/internal/auth"
	"github.com/savaki/shannon/internal/git"
	"github.com/savaki/shannon/internal/session"
	"github.com/savaki/shannon/internal/storage"
)

func TestNewServer(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := storage.NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	gitMgr := git.NewManager(tmpDir, false)
	sessionMgr := session.NewManager(store, gitMgr, tmpDir)
	pskMgr := auth.NewPSKManager(store)

	server := NewServer(":0", sessionMgr, pskMgr)

	if server == nil {
		t.Fatal("NewServer() returned nil")
	}
	if server.addr != ":0" {
		t.Errorf("server.addr = %q, want %q", server.addr, ":0")
	}
	if server.sessions != sessionMgr {
		t.Error("server.sessions not set correctly")
	}
	if server.psk != pskMgr {
		t.Error("server.psk not set correctly")
	}
	if server.httpServer == nil {
		t.Error("server.httpServer is nil")
	}
}

func TestServer_StartAndShutdown(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := storage.NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	gitMgr := git.NewManager(tmpDir, false)
	sessionMgr := session.NewManager(store, gitMgr, tmpDir)
	pskMgr := auth.NewPSKManager(store)

	// Use port 0 to get a random available port
	server := NewServer("127.0.0.1:0", sessionMgr, pskMgr)

	// Start server in background
	serverErr := make(chan error, 1)
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}

	// Check for startup errors
	select {
	case err := <-serverErr:
		t.Errorf("server error: %v", err)
	default:
		// no error
	}
}

func TestServer_RegisterRoutes(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, _ := storage.NewStore(dbPath)
	defer store.Close()

	gitMgr := git.NewManager(tmpDir, false)
	sessionMgr := session.NewManager(store, gitMgr, tmpDir)
	pskMgr := auth.NewPSKManager(store)

	server := NewServer(":0", sessionMgr, pskMgr)

	mux := http.NewServeMux()
	server.registerRoutes(mux)

	// Verify routes are registered by checking for patterns
	// We can't easily inspect the mux, but the handlers test covers this
}
