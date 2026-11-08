#!/usr/bin/env bash
# Launches one mock-tail2 (UDP 57110; replies to 57120). Add it in Settings as type "OBSBOT Tail 2",
# host localhost, port 57110. Only one at a time: shotqueue matches replies by source IP.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/mock-tail2"
exec node index.js "$@"
