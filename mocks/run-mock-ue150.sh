#!/usr/bin/env bash
# Launches 3 mock-ue150 instances (ports 5001-5003), one per sample image.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/mock-ue150"

if [ ! -d node_modules ]; then
	npm install
fi

images=(images/input.png images/choir.png images/cam4.png)
ports=(5001 5002 5003)
pids=()

cleanup() {
	kill "${pids[@]}" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

for i in "${!ports[@]}"; do
	node index.js "${images[$i]}" --port="${ports[$i]}" --auth=digest &
	pids+=("$!")
done

wait
