#!/usr/bin/env bash
# Launches mock-controller on port 5000.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/mock-controller"

go run . --port=5000
