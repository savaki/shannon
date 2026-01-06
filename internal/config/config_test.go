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

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfig_Validate_ValidPort(t *testing.T) {
	tests := []struct {
		name    string
		port    int
		wantErr bool
	}{
		{"min valid port", 1, false},
		{"max valid port", 65535, false},
		{"common port", 8080, false},
		{"zero port", 0, true},
		{"negative port", -1, true},
		{"port too high", 65536, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Port:    tt.port,
				DataDir: "/tmp/test",
			}
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfig_Validate_DataDir(t *testing.T) {
	cfg := &Config{
		Port:    8080,
		DataDir: "",
	}
	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should error on empty DataDir")
	}
}

func TestConfig_Validate_LogLevel(t *testing.T) {
	tests := []struct {
		name     string
		logLevel string
		wantErr  bool
	}{
		{"debug", "debug", false},
		{"info", "info", false},
		{"warn", "warn", false},
		{"error", "error", false},
		{"empty (default)", "", false},
		{"invalid", "invalid", true},
		{"trace (invalid)", "trace", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Port:     8080,
				DataDir:  "/tmp/test",
				LogLevel: tt.logLevel,
			}
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfig_EnsureDataDir_Creates(t *testing.T) {
	tmpDir := t.TempDir()
	testDir := filepath.Join(tmpDir, "shannon-test")

	cfg := &Config{
		Port:    8080,
		DataDir: testDir,
	}

	// Directory shouldn't exist yet
	if _, err := os.Stat(testDir); !os.IsNotExist(err) {
		t.Fatal("test directory already exists")
	}

	// EnsureDataDir should create it
	if err := cfg.EnsureDataDir(); err != nil {
		t.Fatalf("EnsureDataDir() error = %v", err)
	}

	// Directory should now exist
	info, err := os.Stat(testDir)
	if err != nil {
		t.Fatalf("directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("created path is not a directory")
	}
}

func TestConfig_EnsureDataDir_ExistingDir(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Port:    8080,
		DataDir: tmpDir,
	}

	// Should succeed even if directory exists
	if err := cfg.EnsureDataDir(); err != nil {
		t.Fatalf("EnsureDataDir() error = %v", err)
	}
}

func TestConfig_EnsureDataDir_Empty(t *testing.T) {
	cfg := &Config{
		Port:    8080,
		DataDir: "",
	}

	err := cfg.EnsureDataDir()
	if err == nil {
		t.Error("EnsureDataDir() should error on empty DataDir")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Port != 8080 {
		t.Errorf("default Port = %d, want 8080", cfg.Port)
	}

	if cfg.DataDir == "" {
		t.Error("default DataDir should not be empty")
	}

	if !cfg.UseWorktree {
		t.Error("default UseWorktree should be true")
	}

	if cfg.LogLevel != "info" {
		t.Errorf("default LogLevel = %q, want \"info\"", cfg.LogLevel)
	}
}

func TestConfig_EnsureDataDir_TildeExpansion(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot get home directory")
	}

	tmpName := ".shannon-test-" + filepath.Base(t.TempDir())
	cfg := &Config{
		Port:    8080,
		DataDir: "~/" + tmpName,
	}

	if err := cfg.EnsureDataDir(); err != nil {
		t.Fatalf("EnsureDataDir() error = %v", err)
	}

	// Verify tilde was expanded
	expectedPath := filepath.Join(home, tmpName)
	if cfg.DataDir != expectedPath {
		t.Errorf("DataDir = %q, want %q", cfg.DataDir, expectedPath)
	}

	// Cleanup
	os.RemoveAll(expectedPath)
}

func TestConfig_EnsureDataDir_NestedPath(t *testing.T) {
	tmpDir := t.TempDir()
	nestedPath := filepath.Join(tmpDir, "a", "b", "c", "d")

	cfg := &Config{
		Port:    8080,
		DataDir: nestedPath,
	}

	if err := cfg.EnsureDataDir(); err != nil {
		t.Fatalf("EnsureDataDir() error = %v", err)
	}

	// Verify all directories were created
	info, err := os.Stat(nestedPath)
	if err != nil {
		t.Fatalf("nested path not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("created path is not a directory")
	}
}

func TestConfig_Validate_AllFields(t *testing.T) {
	cfg := &Config{
		Port:          8080,
		DataDir:       "/tmp/test",
		Repo:          "https://github.com/test/repo",
		UseWorktree:   true,
		FirebaseCreds: "/path/to/creds.json",
		LogLevel:      "debug",
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}

func TestConfig_Validate_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		port     int
		dataDir  string
		logLevel string
		wantErr  bool
	}{
		{"port 1", 1, "/tmp", "info", false},
		{"port 65535", 65535, "/tmp", "info", false},
		{"port 0", 0, "/tmp", "info", true},
		{"port 65536", 65536, "/tmp", "info", true},
		{"empty dataDir", 8080, "", "info", true},
		{"empty logLevel", 8080, "/tmp", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Port:     tt.port,
				DataDir:  tt.dataDir,
				LogLevel: tt.logLevel,
			}
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
