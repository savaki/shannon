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
	"encoding/json"
	"os/exec"
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

// GetHostname returns the Tailscale DNS name for this machine.
// It strips the trailing dot from the DNS name if present.
// Returns an empty string and error if Tailscale is not available.
func GetHostname() (string, error) {
	cmd := exec.Command("tailscale", "status", "--json")
	output, err := cmd.Output()
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
