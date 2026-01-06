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

package auth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/savaki/shannon/internal/storage"
)

func newTestStore(t *testing.T) *storage.Store {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := storage.NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	return store
}

func TestGeneratePSK(t *testing.T) {
	psk1, err := GeneratePSK()
	if err != nil {
		t.Fatalf("GeneratePSK() error = %v", err)
	}

	// Should be base64 encoded 32 bytes
	if len(psk1) != 44 { // 32 bytes base64 = 44 chars (with padding)
		t.Errorf("PSK length = %d, want 44", len(psk1))
	}

	// Should be different each time
	psk2, err := GeneratePSK()
	if err != nil {
		t.Fatalf("GeneratePSK() second call error = %v", err)
	}

	if psk1 == psk2 {
		t.Error("GeneratePSK() should generate unique keys")
	}
}

func TestPSKManager_GetOrCreate(t *testing.T) {
	store := newTestStore(t)
	mgr := NewPSKManager(store)

	// First call should create
	psk1, err := mgr.GetOrCreate()
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}

	if psk1 == "" {
		t.Error("GetOrCreate() returned empty PSK")
	}

	// Second call should return same key
	psk2, err := mgr.GetOrCreate()
	if err != nil {
		t.Fatalf("GetOrCreate() second call error = %v", err)
	}

	if psk1 != psk2 {
		t.Errorf("GetOrCreate() returned different keys: %q vs %q", psk1, psk2)
	}
}

func TestPSKManager_Validate(t *testing.T) {
	store := newTestStore(t)
	mgr := NewPSKManager(store)

	psk, err := mgr.GetOrCreate()
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}

	// Valid key
	if !mgr.Validate(psk) {
		t.Error("Validate() returned false for valid PSK")
	}

	// Invalid key
	if mgr.Validate("invalid-key") {
		t.Error("Validate() returned true for invalid PSK")
	}

	// Empty key
	if mgr.Validate("") {
		t.Error("Validate() returned true for empty PSK")
	}
}

func TestPSKManager_Regenerate(t *testing.T) {
	store := newTestStore(t)
	mgr := NewPSKManager(store)

	psk1, err := mgr.GetOrCreate()
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}

	// Regenerate
	psk2, err := mgr.Regenerate()
	if err != nil {
		t.Fatalf("Regenerate() error = %v", err)
	}

	if psk1 == psk2 {
		t.Error("Regenerate() should create a new key")
	}

	// Old key should be invalid
	if mgr.Validate(psk1) {
		t.Error("old PSK should be invalid after regenerate")
	}

	// New key should be valid
	if !mgr.Validate(psk2) {
		t.Error("new PSK should be valid after regenerate")
	}
}

func TestPSKManager_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create first manager and generate PSK
	store1, err := storage.NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	mgr1 := NewPSKManager(store1)
	psk1, err := mgr1.GetOrCreate()
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}

	store1.Close()

	// Create second manager with same database
	store2, err := storage.NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create second store: %v", err)
	}
	defer store2.Close()

	mgr2 := NewPSKManager(store2)
	psk2, err := mgr2.GetOrCreate()
	if err != nil {
		t.Fatalf("GetOrCreate() second manager error = %v", err)
	}

	// Should be the same key
	if psk1 != psk2 {
		t.Error("PSK should persist across restarts")
	}
}

func TestPSKManager_Validate_WithoutLoad(t *testing.T) {
	store := newTestStore(t)
	mgr := NewPSKManager(store)

	// Generate PSK first
	psk, _ := mgr.GetOrCreate()

	// Create new manager without cached PSK
	mgr2 := NewPSKManager(store)

	// Validate should load from store
	if !mgr2.Validate(psk) {
		t.Error("Validate() should load PSK from store")
	}
}

func TestPSKManager_Validate_EmptyStore(t *testing.T) {
	store := newTestStore(t)
	mgr := NewPSKManager(store)

	// Don't create PSK, try to validate
	if mgr.Validate("some-key") {
		t.Error("Validate() should return false when no PSK stored")
	}
}

func TestGeneratePSK_Entropy(t *testing.T) {
	// Generate multiple PSKs and ensure they're all unique
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		psk, err := GeneratePSK()
		if err != nil {
			t.Fatalf("GeneratePSK() error = %v", err)
		}
		if seen[psk] {
			t.Fatalf("GeneratePSK() produced duplicate key on iteration %d", i)
		}
		seen[psk] = true
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
