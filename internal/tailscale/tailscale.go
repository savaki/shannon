// Copyright 2026 savaki
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package tailscale provides integration with Tailscale for hostname discovery.
package tailscale

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Status represents the relevant parts of tailscale status --json output.
type Status struct {
	Self Self `json:"Self"`
}

// Self represents the local machine's Tailscale status.
type Self struct {
	DNSName      string   `json:"DNSName"`
	HostName     string   `json:"HostName"`
	TailscaleIPs []string `json:"TailscaleIPs"`
}

// tailscalePaths lists possible locations for the tailscale binary.
var tailscalePaths = []string{
	"tailscale", // In PATH
	"/Applications/Tailscale.app/Contents/MacOS/Tailscale", // macOS
	"/usr/bin/tailscale",                                   // Linux
	"/usr/local/bin/tailscale",                             // Linux alternate
}

// GetHostname returns the Tailscale DNS name for this machine.
// It strips the trailing dot from the DNS name if present.
// Returns an empty string and error if Tailscale is not available.
func GetHostname() (string, error) {
	var output []byte
	var err error

	for _, path := range tailscalePaths {
		cmd := exec.Command(path, "status", "--json")
		output, err = cmd.Output()
		if err == nil {
			break
		}
	}
	if err != nil {
		return "", err
	}

	var status Status
	if err := json.Unmarshal(output, &status); err != nil {
		return "", err
	}

	// Strip trailing dot from DNS name (e.g., "host.tail-net.ts.net." -> "host.tail-net.ts.net")
	hostname := strings.TrimSuffix(status.Self.DNSName, ".")
	return hostname, nil
}

// GetHostnameOrDefault returns the Tailscale hostname, or the provided default if unavailable.
func GetHostnameOrDefault(defaultHost string) string {
	hostname, err := GetHostname()
	if err != nil || hostname == "" {
		return defaultHost
	}
	return hostname
}

// findTailscale returns the path to the tailscale binary.
func findTailscale() (string, error) {
	for _, path := range tailscalePaths {
		if _, err := exec.LookPath(path); err == nil {
			return path, nil
		}
		// Also check if the absolute path exists
		if filepath.IsAbs(path) {
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
	}
	return "", fmt.Errorf("tailscale binary not found")
}

// GetCertificate fetches a TLS certificate from Tailscale for the given hostname.
// The certificate is fetched on-demand and kept only in memory.
// If hostname is empty, it will be auto-detected from Tailscale.
func GetCertificate(hostname string) (*tls.Certificate, error) {
	// Auto-detect hostname if not provided
	if hostname == "" {
		var err error
		hostname, err = GetHostname()
		if err != nil {
			return nil, fmt.Errorf("failed to get tailscale hostname: %w", err)
		}
	}

	// Find tailscale binary
	tsPath, err := findTailscale()
	if err != nil {
		return nil, err
	}

	// Create temp directory for cert files
	tmpDir, err := os.MkdirTemp("", "tailscale-cert-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir) // Clean up immediately after reading

	certFile := filepath.Join(tmpDir, "cert.crt")
	keyFile := filepath.Join(tmpDir, "cert.key")

	// Fetch certificate using tailscale cert command
	cmd := exec.Command(tsPath, "cert",
		"--cert-file", certFile,
		"--key-file", keyFile,
		hostname)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("tailscale cert failed: %w: %s", err, output)
	}

	// Read cert and key into memory
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate: %w", err)
	}
	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read key: %w", err)
	}

	// Parse the certificate
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return &cert, nil
}
