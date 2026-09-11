#!/usr/bin/env bash
set -euo pipefail
npm ci
golangci-lint fmt --diff
npm run fmt:check
djlint web/templates -e gohtml --check
djlint web/templates -e gohtml --lint
npm run lint
npm run build
golangci-lint run
go test ./...
go build ./...
nix build
