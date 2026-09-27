#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -lt 1 ]; then
  echo "Usage: $0 <version>" >&2
  echo "Example: $0 1.2.3" >&2
  exit 1
fi

VERSION="$1"
cd "$(dirname "$0")/.."
APP_VERSION="$VERSION" ./scripts/package-mac-dmg.sh

echo "Built DMG for version ${VERSION}"
ls -1 dist/ShotQueue-v${VERSION}.dmg
