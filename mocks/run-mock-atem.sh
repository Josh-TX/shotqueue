#!/usr/bin/env bash
# Launches mock-atem. Control UI port defaults to 9000; pass a port as $1 to override.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/mock-atem"

if [ ! -d node_modules ]; then
	npm install
fi

npx ts-node src/index.ts "$@"
