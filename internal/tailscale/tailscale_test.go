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

package tailscale

import (
	"encoding/json"
	"testing"
)

func TestStatusUnmarshal(t *testing.T) {
	// Test that we can parse actual Tailscale status output
	jsonData := `{
		"Self": {
			"DNSName": "myhost.tail-net.ts.net.",
			"HostName": "myhost",
			"TailscaleIPs": ["100.64.1.1", "fd7a:115c:a1e0::1"]
		}
	}`

	var status Status
	if err := json.Unmarshal([]byte(jsonData), &status); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if status.Self.DNSName != "myhost.tail-net.ts.net." {
		t.Errorf("expected DNSName 'myhost.tail-net.ts.net.', got %q", status.Self.DNSName)
	}
	if status.Self.HostName != "myhost" {
		t.Errorf("expected HostName 'myhost', got %q", status.Self.HostName)
	}
	if len(status.Self.TailscaleIPs) != 2 {
		t.Errorf("expected 2 TailscaleIPs, got %d", len(status.Self.TailscaleIPs))
	}
}

func TestGetHostnameOrDefault(t *testing.T) {
	// This test will use the default if Tailscale isn't available
	result := GetHostnameOrDefault("fallback-host")

	// We can't predict if Tailscale is available, but result should never be empty
	if result == "" {
		t.Error("GetHostnameOrDefault returned empty string")
	}

	// If Tailscale isn't available, it should return the default
	// If Tailscale IS available, it should return a non-empty hostname
	t.Logf("GetHostnameOrDefault returned: %q", result)
}
