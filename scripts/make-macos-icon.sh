#!/bin/sh
# Builds the macOS app icon from the Icon Composer document
# build/darwin/AppIcon.icon (a glyph layer over a solid fill):
#
# - build/darwin/Assets.car: compiled by actool; macOS 26 and later render
#   it themselves (Liquid Glass, dark and tinted variants).
# - build/darwin/icons.icns: for older macOS. ictool renders the icon body in
#   Apple's shape, which is placed on Apple's 1024 px grid (824 px body, 100 px
#   margin) with the drop shadow macOS icons draw themselves.
#
# Both results are committed; run this again only after changing the .icon
# (for example in Icon Composer). Needs Xcode 26 or later, ImageMagick 7
# (`magick`) and iconutil.
#
#   scripts/make-macos-icon.sh

set -eu

cd "$(dirname "$0")/.."
SRC=build/darwin/AppIcon.icon
NAME=AppIcon
OUT=build/darwin

CANVAS=1024
BODY=824
SHADOW="30x12+0+12" # opacity% x blur sigma + x offset + y offset, at 1024 px
MIN_MACOS=12.0      # keep in line with LSMinimumSystemVersion in the plists

ICTOOL="$(xcode-select -p)/../Applications/Icon Composer.app/Contents/Executables/ictool"
SRGB="/System/Library/ColorSync/Profiles/sRGB Profile.icc"

[ -x "$ICTOOL" ] || { echo "ictool not found; Xcode 26 or later is required" >&2; exit 1; }
command -v magick >/dev/null || { echo "ImageMagick 7 (magick) is required" >&2; exit 1; }

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

# Assets.car for macOS 26+. actool also writes an .icns, but only up to
# 256 px, so the one below is used instead.
xcrun actool "$SRC" --compile "$TMP" --output-partial-info-plist "$TMP/partial.plist" \
  --app-icon "$NAME" --platform macosx --target-device mac \
  --minimum-deployment-target "$MIN_MACOS" --enable-on-demand-resources NO \
  --development-region en --errors --warnings >/dev/null 2>&1 ||
  { echo "actool failed to compile $SRC" >&2; exit 1; }
cp "$TMP/Assets.car" "$OUT/Assets.car"

# The icon body, full bleed. ictool tags it Display P3; convert to sRGB.
"$ICTOOL" "$SRC" --export-image --output-file "$TMP/body-p3.png" --platform macOS \
  --rendition Default --width "$BODY" --height "$BODY" --scale 1 >/dev/null
magick "$TMP/body-p3.png" -profile "$SRGB" -depth 8 "$TMP/body.png"

# Shadow under the body, both centred on the canvas.
magick -size "${CANVAS}x${CANVAS}" xc:none \
  \( "$TMP/body.png" -background black -shadow "$SHADOW" \) \
  -gravity center -geometry +0+12 -compose over -composite \
  "$TMP/body.png" -gravity center -geometry +0+0 -composite \
  "$TMP/icon_1024.png"

ICONSET="$TMP/icons.iconset"
mkdir "$ICONSET"
for size in 16 32 128 256 512; do
  magick "$TMP/icon_1024.png" -resize "${size}x${size}" "$ICONSET/icon_${size}x${size}.png"
  double=$((size * 2))
  magick "$TMP/icon_1024.png" -resize "${double}x${double}" "$ICONSET/icon_${size}x${size}@2x.png"
done
iconutil -c icns "$ICONSET" -o "$OUT/icons.icns"

echo "wrote $OUT/Assets.car and $OUT/icons.icns"
