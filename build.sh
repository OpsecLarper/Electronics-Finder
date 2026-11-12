#!/usr/bin/env bash
# Cross-compiles ElectronicsFinder.exe for Windows (no CGO needed).
set -euo pipefail
cd "$(dirname "$0")"
if command -v go-winres >/dev/null || [ -x "$(go env GOPATH)/bin/go-winres" ]; then
  "$(command -v go-winres || echo "$(go env GOPATH)/bin/go-winres")" make --in winres/winres.json --arch amd64
fi
mkdir -p dist
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w -H windowsgui" -o dist/ElectronicsFinder.exe .
echo "Built dist/ElectronicsFinder.exe"
