#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

APP_NAME="ShotQueue"
APP_VERSION="${APP_VERSION:-1.0.0}"
APP_BUNDLE="${APP_NAME}.app"
APP_PATH="dist/${APP_BUNDLE}"
DMG_PATH="dist/${APP_NAME}-v${APP_VERSION}.dmg"
DMG_SRC="dist/dmg-src"
BINARY_PATH="$APP_PATH/Contents/Resources/shotqueue-bin"
LAUNCHER_PATH="$APP_PATH/Contents/MacOS/${APP_NAME}"
ICON_SOURCE="frontend/public/android-chrome-192x192.png"
ICONSET_DIR="$APP_PATH/Contents/Resources/AppIcon.iconset"
SDK_PATH="$(xcrun --sdk macosx --show-sdk-path)"

./scripts/build.sh

rm -rf "$APP_PATH" "$DMG_PATH" "$DMG_SRC"
mkdir -p "$APP_PATH/Contents/MacOS" "$APP_PATH/Contents/Resources" "$DMG_SRC" "$ICONSET_DIR"

cp "$ICON_SOURCE" "$ICONSET_DIR/icon_192x192.png"

sips -z 16 16   "$ICON_SOURCE" --out "$ICONSET_DIR/icon_16x16.png" >/dev/null
sips -z 32 32   "$ICON_SOURCE" --out "$ICONSET_DIR/icon_16x16@2x.png" >/dev/null
sips -z 32 32   "$ICON_SOURCE" --out "$ICONSET_DIR/icon_32x32.png" >/dev/null
sips -z 64 64   "$ICON_SOURCE" --out "$ICONSET_DIR/icon_32x32@2x.png" >/dev/null
sips -z 128 128 "$ICON_SOURCE" --out "$ICONSET_DIR/icon_128x128.png" >/dev/null
sips -z 256 256 "$ICON_SOURCE" --out "$ICONSET_DIR/icon_128x128@2x.png" >/dev/null
sips -z 256 256 "$ICON_SOURCE" --out "$ICONSET_DIR/icon_256x256.png" >/dev/null
sips -z 512 512 "$ICON_SOURCE" --out "$ICONSET_DIR/icon_256x256@2x.png" >/dev/null
sips -z 512 512 "$ICON_SOURCE" --out "$ICONSET_DIR/icon_512x512.png" >/dev/null
sips -z 1024 1024 "$ICON_SOURCE" --out "$ICONSET_DIR/icon_512x512@2x.png" >/dev/null
iconutil -c icns "$ICONSET_DIR" -o "$APP_PATH/Contents/Resources/AppIcon.icns" >/dev/null
cp "$ICONSET_DIR/icon_512x512@2x.png" "$APP_PATH/Contents/Resources/AppIcon.png" 2>/dev/null || true

(
  cd backend
  GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ../dist/shotqueue-arm64 .
)
(
  cd backend
  GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ../dist/shotqueue-amd64 .
)
lipo -create dist/shotqueue-arm64 dist/shotqueue-amd64 -output "$BINARY_PATH"
chmod +x "$BINARY_PATH"

swiftc -sdk "$SDK_PATH" -target arm64-apple-macosx10.15 -o "$APP_PATH/Contents/MacOS/ShotQueue-arm64" scripts/ShotQueueApp.swift -framework Cocoa >/dev/null
swiftc -sdk "$SDK_PATH" -target x86_64-apple-macosx10.15 -o "$APP_PATH/Contents/MacOS/ShotQueue-amd64" scripts/ShotQueueApp.swift -framework Cocoa >/dev/null
lipo -create "$APP_PATH/Contents/MacOS/ShotQueue-arm64" "$APP_PATH/Contents/MacOS/ShotQueue-amd64" -output "$LAUNCHER_PATH"
chmod +x "$LAUNCHER_PATH"
rm -f "$APP_PATH/Contents/MacOS/ShotQueue-arm64" "$APP_PATH/Contents/MacOS/ShotQueue-amd64" dist/shotqueue-arm64 dist/shotqueue-amd64

cat > "$APP_PATH/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>${APP_NAME}</string>
    <key>CFBundleIdentifier</key>
    <string>com.shotqueue.${APP_NAME}</string>
    <key>CFBundleName</key>
    <string>${APP_NAME}</string>
    <key>CFBundleDisplayName</key>
    <string>${APP_NAME}</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleSignature</key>
    <string>????</string>
    <key>CFBundleIconFile</key>
    <string>AppIcon</string>
    <key>CFBundleVersion</key>
    <string>${APP_VERSION}</string>
    <key>CFBundleShortVersionString</key>
    <string>${APP_VERSION}</string>
    <key>LSMinimumSystemVersion</key>
    <string>10.15</string>
    <key>NSHighResolutionCapable</key>
    <true/>
</dict>
</plist>
EOF

cp -R "$APP_PATH" "$DMG_SRC/"
ln -s /Applications "$DMG_SRC/Applications"

hdiutil create -volname "$APP_NAME" \
  -srcfolder "$DMG_SRC" \
  -ov -format UDBZ \
  "$DMG_PATH"

echo "Created ${DMG_PATH}"
