#!/usr/bin/env bash
# Build "Storytel Player.app" (Wails) into build/bin/. Unsigned: for local use.
set -euo pipefail
cd "$(dirname "$0")/.."

APP_NAME="Storytel Player"
VERSION=$(node -p "require('./package.json').version")
OUT="build/bin/${APP_NAME}.app"

(cd client && npm install --no-audit --no-fund && npm run build:wails)

# Wails' darwin objects target macOS 15; match it to avoid linker warnings.
export MACOSX_DEPLOYMENT_TARGET=15.0
export CGO_LDFLAGS="-mmacosx-version-min=15.0"
(cd server && go build -tags production -trimpath -ldflags "-s -w" -o ../build/bin/storytel-player .)

rm -rf "$OUT"
mkdir -p "$OUT/Contents/MacOS" "$OUT/Contents/Resources"
cp build/bin/storytel-player "$OUT/Contents/MacOS/storytel-player"

ICONSET=$(mktemp -d)/icon.iconset
mkdir -p "$ICONSET"
for s in 16 32 128 256 512; do
  sips -z $s $s server/assets/icon.png --out "$ICONSET/icon_${s}x${s}.png" >/dev/null
  sips -z $((s*2)) $((s*2)) server/assets/icon.png --out "$ICONSET/icon_${s}x${s}@2x.png" >/dev/null
done
iconutil -c icns "$ICONSET" -o "$OUT/Contents/Resources/icon.icns"

cat > "$OUT/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key><string>${APP_NAME}</string>
  <key>CFBundleDisplayName</key><string>${APP_NAME}</string>
  <key>CFBundleIdentifier</key><string>com.storytel.player.wails</string>
  <key>CFBundleExecutable</key><string>storytel-player</string>
  <key>CFBundleIconFile</key><string>icon</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleShortVersionString</key><string>${VERSION}</string>
  <key>CFBundleVersion</key><string>${VERSION}</string>
  <key>LSMinimumSystemVersion</key><string>15.0</string>
  <key>LSApplicationCategoryType</key><string>public.app-category.music</string>
  <key>NSHighResolutionCapable</key><true/>
</dict>
</plist>
PLIST

codesign --force --deep --sign - "$OUT" >/dev/null 2>&1 || true
echo "Built: $OUT"
du -sh "$OUT"
