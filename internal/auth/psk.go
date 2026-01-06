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

// Package auth provides authentication for shannon.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"

	"github.com/savaki/shannon/internal/storage"
)

const (
	// PSKConfigKey is the storage key for the pre-shared key.
	PSKConfigKey = "psk"
	// PSKLength is the length of the pre-shared key in bytes.
	PSKLength = 32 // 256 bits
)

// PSKManager handles pre-shared key operations.
type PSKManager struct {
	store *storage.Store
	psk   string
}

// NewPSKManager creates a new PSK manager.
func NewPSKManager(store *storage.Store) *PSKManager {
	return &PSKManager{store: store}
}

// GetOrCreate retrieves the existing PSK or creates a new one.
func (m *PSKManager) GetOrCreate() (string, error) {
	// Check if we have a cached PSK
	if m.psk != "" {
		return m.psk, nil
	}

	// Try to load from storage
	psk, err := m.store.GetConfig(PSKConfigKey)
	if err == nil && psk != "" {
		m.psk = psk
		return psk, nil
	}

	// Generate new PSK
	psk, err = GeneratePSK()
	if err != nil {
		return "", fmt.Errorf("failed to generate PSK: %w", err)
	}

	// Store it
	if err := m.store.SetConfig(PSKConfigKey, psk); err != nil {
		return "", fmt.Errorf("failed to store PSK: %w", err)
	}

	m.psk = psk
	return psk, nil
}

// Validate checks if the provided key matches the stored PSK.
func (m *PSKManager) Validate(key string) bool {
	if m.psk == "" {
		// Try to load
		psk, err := m.store.GetConfig(PSKConfigKey)
		if err != nil {
			return false
		}
		m.psk = psk
	}

	// Constant-time comparison to prevent timing attacks
	return subtle.ConstantTimeCompare([]byte(key), []byte(m.psk)) == 1
}

// Regenerate creates a new PSK and invalidates the old one.
func (m *PSKManager) Regenerate() (string, error) {
	psk, err := GeneratePSK()
	if err != nil {
		return "", fmt.Errorf("failed to generate PSK: %w", err)
	}

	if err := m.store.SetConfig(PSKConfigKey, psk); err != nil {
		return "", fmt.Errorf("failed to store PSK: %w", err)
	}

	m.psk = psk
	return psk, nil
}

// GeneratePSK generates a cryptographically secure pre-shared key.
func GeneratePSK() (string, error) {
	bytes := make([]byte, PSKLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}
