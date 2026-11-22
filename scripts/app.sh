#!/bin/sh
# app.sh makes the macOS app bundle of a darwin binary of tarhche: Tarhche.app,
# which Finder and the Dock open as the desktop app, in the directory named,
# and beside it tarhche_<version>_darwin_app.zip, which a release publishes
# and tarhche upgrade replaces a bundle with. GoReleaser runs it on the
# universal binary, and make app on a binary for this Mac. Given
# SOURCE_DATE_EPOCH, every file in it is dated then, so that the same binary
# makes the same zip.
#
#   scripts/app.sh <binary> <version> <directory>
set -eu

if [ $# -ne 3 ]; then
	echo "usage: $0 <binary> <version> <directory>" >&2
	exit 2
fi

binary=$1
version=$2
directory=$3

cd "$(dirname "$0")/.."

mkdir -p "$directory"
directory=$(cd "$directory" && pwd)
bundle=$directory/Tarhche.app
archive=$directory/tarhche_${version}_darwin_app.zip
bundle_version=$(printf '%s' "$version" | sed 's/[^0-9.].*//; s/\.$//')

rm -rf "$bundle" "$archive"
mkdir -p "$bundle/Contents/MacOS" "$bundle/Contents/Resources"
install -m 0755 "$binary" "$bundle/Contents/MacOS/tarhche"
install -m 0644 build/darwin/icons.icns "$bundle/Contents/Resources/icons.icns"
sed "s/@VERSION@/${bundle_version:-0.0.0}/" build/darwin/Info.plist > "$bundle/Contents/Info.plist"

if [ -n "${SOURCE_DATE_EPOCH:-}" ]; then
	find "$bundle" -exec touch -h -d "@$SOURCE_DATE_EPOCH" {} +
fi

(cd "$directory" && zip -qrX "$archive" Tarhche.app)

echo "$archive"
