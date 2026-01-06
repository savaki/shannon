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

// Package config provides configuration management for shannon.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds all configuration for the shannon server.
type Config struct {
	// Port is the HTTP server port (1-65535).
	Port int

	// DataDir is the directory for SQLite database and session data.
	DataDir string

	// Repo is the git repository URL to clone for sessions.
	Repo string

	// UseWorktree enables git worktrees instead of full clones.
	UseWorktree bool

	// FirebaseCreds is the path to Firebase service account JSON.
	FirebaseCreds string

	// LogLevel is the logging verbosity (debug, info, warn, error).
	LogLevel string
}

// Validate checks the configuration and returns an error if invalid.
func (c *Config) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("invalid port %d: must be between 1 and 65535", c.Port)
	}

	if c.DataDir == "" {
		return errors.New("data-dir is required")
	}

	switch c.LogLevel {
	case "debug", "info", "warn", "error", "":
		// valid
	default:
		return fmt.Errorf("invalid log-level %q: must be one of debug, info, warn, error", c.LogLevel)
	}

	return nil
}

// EnsureDataDir creates the data directory if it doesn't exist.
func (c *Config) EnsureDataDir() error {
	if c.DataDir == "" {
		return errors.New("data-dir is required")
	}

	// Expand ~ to home directory
	if len(c.DataDir) > 0 && c.DataDir[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		c.DataDir = filepath.Join(home, c.DataDir[1:])
	}

	if err := os.MkdirAll(c.DataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory %s: %w", c.DataDir, err)
	}

	return nil
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	dataDir := filepath.Join(home, ".shannon")

	return &Config{
		Port:        8080,
		DataDir:     dataDir,
		UseWorktree: true,
		LogLevel:    "info",
	}
}
