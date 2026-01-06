#!/usr/bin/env bash
set -euo pipefail

echo "==> Checking license headers..."
find . -name "*.go" -type f | xargs addlicense -check -f boilerplate.go.txt

echo "==> Verifying dependencies..."
go mod download
go mod verify

echo "==> Running go vet..."
go vet ./...

echo "==> Running staticcheck..."
staticcheck ./...

echo "==> Running tests..."
go test -race -cover ./...

echo "==> All checks passed!"
