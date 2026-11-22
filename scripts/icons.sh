#!/bin/sh
# icons.sh renders every icon the app ships from the Tarhche logo,
# build/icon.svg and its light twin build/icon-dark.svg: the app's own, the
# tray's, the macOS bundle's, the Windows executable's and the Linux desktop
# entry's. make icons runs it in the develop container.
set -eu

cd "$(dirname "$0")/.."

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

magick=$(command -v magick || command -v convert)

# The logo, cropped to itself, on a white rounded square with a soft shadow,
# on the grid macOS draws app icons on (824 of 1024, rounded by 185), so that
# it sits in a Dock as other apps do.
echo '<svg xmlns="http://www.w3.org/2000/svg" width="1024" height="1024"><rect x="100" y="100" width="824" height="824" rx="185" fill="#ffffff"/></svg>' > "$work/square.svg"
rsvg-convert -w 1024 -h 1024 "$work/square.svg" -o "$work/square.png"
rsvg-convert -w 2048 -h 2048 build/icon.svg -o "$work/logo.png"
"$magick" "$work/logo.png" -trim +repage -resize 600x600 "$work/mark.png"
"$magick" "$work/square.png" \( +clone -background '#0f1c26' -shadow 28x14+0+10 \) +swap \
	-background none -layers merge +repage -gravity center -extent 1024x1024 \
	"$work/mark.png" -gravity center -composite build/appicon.png

"$magick" build/appicon.png -resize 512x512 internal/desktop/icons/app.png
"$magick" build/appicon.png -resize 512x512 build/linux/tarhche.png

# A tray holds a glyph, cropped to the logo and squared: black on a light tray
# and as macOS's template, which it tints, and white on a dark one.
tray() {
	rsvg-convert -w 1024 -h 1024 "$1" -o "$work/glyph.png"
	"$magick" "$work/glyph.png" -fill "$2" -colorize 100 -trim +repage \
		-gravity center -background none -extent '%[fx:max(w,h)]x%[fx:max(w,h)]' \
		-resize 64x64 "$3"
}

tray build/icon.svg black internal/desktop/icons/tray.png
tray build/icon-dark.svg white internal/desktop/icons/tray-dark.png

wails3 generate icons -input build/appicon.png -macfilename build/darwin/icons.icns -windowsfilename build/windows/icon.ico

for arch in 386 amd64 arm64; do
	wails3 generate syso -arch "$arch" -icon build/windows/icon.ico \
		-manifest build/windows/wails.exe.manifest -info build/windows/info.json \
		-out "cmd/tarhche/rsrc_windows_$arch.syso"
done
