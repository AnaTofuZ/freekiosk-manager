#!/usr/bin/env bash
set -euo pipefail
npm ci
npm run build
golangci-lint fmt
oxfmt .
golangci-lint run
oxlint . --deny-warnings
go test ./...
go build ./...
nix build
