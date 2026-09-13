#!/usr/bin/env bash
# Launches go-atem-listener-v2, listening on UDP port 9910.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/go-atem-listener-v2"

go run .
