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
	"bytes"
	"image/png"
	"io"
	"os"
	"testing"
)

func TestGenerateQR(t *testing.T) {
	config := ConnectionConfig{
		Host: "100.64.0.1",
		Port: 8080,
		PSK:  "test-key-123",
	}

	qr, err := GenerateQR(config)
	if err != nil {
		t.Fatalf("GenerateQR() error = %v", err)
	}

	if qr == "" {
		t.Error("GenerateQR() returned empty string")
	}

	// Should contain QR characters (block elements)
	if len(qr) < 100 {
		t.Error("GenerateQR() output seems too short")
	}
}

func TestGenerateQRPNG(t *testing.T) {
	config := ConnectionConfig{
		Host: "100.64.0.1",
		Port: 8080,
		PSK:  "test-key-123",
	}

	data, err := GenerateQRPNG(config, 256)
	if err != nil {
		t.Fatalf("GenerateQRPNG() error = %v", err)
	}

	if len(data) == 0 {
		t.Error("GenerateQRPNG() returned empty data")
	}

	// Verify it's valid PNG
	_, err = png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Errorf("GenerateQRPNG() output is not valid PNG: %v", err)
	}
}

func TestParseQR(t *testing.T) {
	original := ConnectionConfig{
		Host: "100.64.0.1",
		Port: 8080,
		PSK:  "test-key-123",
	}

	// Generate and parse should round-trip
	// Note: We need to use the JSON directly since QR ASCII isn't parseable
	jsonData := `{"host":"100.64.0.1","port":8080,"psk":"test-key-123"}`

	parsed, err := ParseQR(jsonData)
	if err != nil {
		t.Fatalf("ParseQR() error = %v", err)
	}

	if parsed.Host != original.Host {
		t.Errorf("Host = %q, want %q", parsed.Host, original.Host)
	}
	if parsed.Port != original.Port {
		t.Errorf("Port = %d, want %d", parsed.Port, original.Port)
	}
	if parsed.PSK != original.PSK {
		t.Errorf("PSK = %q, want %q", parsed.PSK, original.PSK)
	}
}

func TestParseQR_Invalid(t *testing.T) {
	_, err := ParseQR("not json")
	if err == nil {
		t.Error("ParseQR() should error on invalid JSON")
	}
}

func TestPrintQR(t *testing.T) {
	config := ConnectionConfig{
		Host: "100.64.0.1",
		Port: 8080,
		PSK:  "test-key",
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := PrintQR(config)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("PrintQR() error = %v", err)
	}

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Verify output contains expected elements
	if len(output) < 100 {
		t.Error("PrintQR() output seems too short")
	}

	// Should contain URL info with host and port
	if !bytes.Contains([]byte(output), []byte("URL:")) {
		t.Error("PrintQR() output should contain URL")
	}
	if !bytes.Contains([]byte(output), []byte("100.64.0.1:8080")) {
		t.Error("PrintQR() output should contain host:port")
	}
}

func TestCenterText(t *testing.T) {
	tests := []struct {
		text  string
		width int
		want  int // expected length
	}{
		{"hello", 10, 10},
		{"hi", 6, 6},
		{"", 5, 5},
		{"longer text than width", 10, 10}, // truncated
	}

	for _, tt := range tests {
		result := centerText(tt.text, tt.width)
		if len(result) != tt.want {
			t.Errorf("centerText(%q, %d) length = %d, want %d", tt.text, tt.width, len(result), tt.want)
		}
	}
}

func TestCenterText_Centering(t *testing.T) {
	result := centerText("hi", 6)
	// "hi" with width 6 should be "  hi  "
	if result != "  hi  " {
		t.Errorf("centerText(\"hi\", 6) = %q, want \"  hi  \"", result)
	}
}

func TestGenerateQR_EmptyConfig(t *testing.T) {
	config := ConnectionConfig{}
	qr, err := GenerateQR(config)
	if err != nil {
		t.Fatalf("GenerateQR() with empty config error = %v", err)
	}
	if qr == "" {
		t.Error("GenerateQR() returned empty string")
	}
}

func TestGenerateQRPNG_SmallSize(t *testing.T) {
	config := ConnectionConfig{
		Host: "localhost",
		Port: 8080,
		PSK:  "key",
	}

	data, err := GenerateQRPNG(config, 64)
	if err != nil {
		t.Fatalf("GenerateQRPNG() error = %v", err)
	}

	// Verify it's valid PNG
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("invalid PNG: %v", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != 64 || bounds.Dy() != 64 {
		t.Errorf("PNG size = %dx%d, want 64x64", bounds.Dx(), bounds.Dy())
	}
}

func TestConnectionConfig_Fields(t *testing.T) {
	config := ConnectionConfig{
		Host: "192.168.1.1",
		Port: 3000,
		PSK:  "secret-key",
	}

	if config.Host != "192.168.1.1" {
		t.Errorf("Host = %q, want %q", config.Host, "192.168.1.1")
	}
	if config.Port != 3000 {
		t.Errorf("Port = %d, want %d", config.Port, 3000)
	}
	if config.PSK != "secret-key" {
		t.Errorf("PSK = %q, want %q", config.PSK, "secret-key")
	}
}

func TestCenterText_ExactWidth(t *testing.T) {
	result := centerText("hello", 5)
	if result != "hello" {
		t.Errorf("centerText(\"hello\", 5) = %q, want \"hello\"", result)
	}
}

func TestCenterText_OddPadding(t *testing.T) {
	// "ab" with width 5 should have 1 left pad, 2 right pad = " ab  "
	result := centerText("ab", 5)
	if len(result) != 5 {
		t.Errorf("length = %d, want 5", len(result))
	}
	if result != " ab  " {
		t.Errorf("centerText(\"ab\", 5) = %q, want \" ab  \"", result)
	}
}

func TestParseQR_EmptyString(t *testing.T) {
	_, err := ParseQR("")
	if err == nil {
		t.Error("ParseQR(\"\") should error")
	}
}

func TestParseQR_PartialJSON(t *testing.T) {
	// Missing closing brace
	_, err := ParseQR(`{"host":"localhost"`)
	if err == nil {
		t.Error("ParseQR() should error on incomplete JSON")
	}
}

func TestGenerateQRPNG_LargeSize(t *testing.T) {
	config := ConnectionConfig{
		Host: "10.0.0.1",
		Port: 443,
		PSK:  "large-key",
	}

	data, err := GenerateQRPNG(config, 512)
	if err != nil {
		t.Fatalf("GenerateQRPNG() error = %v", err)
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("invalid PNG: %v", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != 512 || bounds.Dy() != 512 {
		t.Errorf("PNG size = %dx%d, want 512x512", bounds.Dx(), bounds.Dy())
	}
}

func TestGenerateQR_SpecialCharacters(t *testing.T) {
	config := ConnectionConfig{
		Host: "host.example.com",
		Port: 8080,
		PSK:  "key+with/special=chars",
	}

	qr, err := GenerateQR(config)
	if err != nil {
		t.Fatalf("GenerateQR() error = %v", err)
	}
	if qr == "" {
		t.Error("GenerateQR() returned empty string")
	}
}
