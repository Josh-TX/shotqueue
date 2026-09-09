#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

echo "Building frontend..."
(cd frontend && npm ci && npm run build)

echo "Embedding frontend into backend..."
find backend/webdist -mindepth 1 ! -name placeholder -delete
cp -r frontend/dist/. backend/webdist/

echo "Building backend binary..."
mkdir -p dist
(cd backend && go build -trimpath -ldflags="-s -w" -o ../dist/shotqueue .)

echo "Built dist/shotqueue"
