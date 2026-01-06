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
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/skip2/go-qrcode"
)

// ConnectionConfig contains the information needed to connect to the server.
type ConnectionConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
	PSK  string `json:"psk"`
}

// GenerateQR creates a QR code containing connection information.
func GenerateQR(config ConnectionConfig) (string, error) {
	data, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}

	qr, err := qrcode.New(string(data), qrcode.Medium)
	if err != nil {
		return "", fmt.Errorf("failed to create QR code: %w", err)
	}

	// Generate ASCII art representation
	return qr.ToSmallString(false), nil
}

// GenerateQRPNG creates a PNG QR code image.
func GenerateQRPNG(config ConnectionConfig, size int) ([]byte, error) {
	data, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	png, err := qrcode.Encode(string(data), qrcode.Medium, size)
	if err != nil {
		return nil, fmt.Errorf("failed to encode QR code: %w", err)
	}

	return png, nil
}

// PrintQR prints the QR code to stdout with a border.
func PrintQR(config ConnectionConfig) error {
	qrStr, err := GenerateQR(config)
	if err != nil {
		return err
	}

	const width = 60

	fmt.Println()
	fmt.Println("╔" + strings.Repeat("═", width) + "╗")
	fmt.Println("║" + centerText("Scan this QR code with Shannon mobile", width) + "║")
	fmt.Println("╠" + strings.Repeat("═", width) + "╣")
	fmt.Println("║" + strings.Repeat(" ", width) + "║")

	// Print QR with padding
	lines := strings.Split(qrStr, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		fmt.Println("║" + centerText(line, width) + "║")
	}

	fmt.Println("║" + strings.Repeat(" ", width) + "║")
	fmt.Println("╠" + strings.Repeat("═", width) + "╣")
	fmt.Printf("║  Host: %-52s║\n", config.Host)
	fmt.Printf("║  Port: %-52d║\n", config.Port)
	fmt.Println("╚" + strings.Repeat("═", width) + "╝")
	fmt.Println()

	return nil
}

// centerText centers text within a given width, truncating if necessary.
func centerText(text string, width int) string {
	textLen := utf8.RuneCountInString(text)
	if textLen >= width {
		// Truncate by runes, not bytes
		runes := []rune(text)
		return string(runes[:width])
	}
	leftPad := (width - textLen) / 2
	rightPad := width - textLen - leftPad
	return strings.Repeat(" ", leftPad) + text + strings.Repeat(" ", rightPad)
}

// ParseQR parses a QR code payload back to ConnectionConfig.
func ParseQR(data string) (*ConnectionConfig, error) {
	var config ConnectionConfig
	if err := json.Unmarshal([]byte(data), &config); err != nil {
		return nil, fmt.Errorf("failed to parse QR data: %w", err)
	}
	return &config, nil
}
