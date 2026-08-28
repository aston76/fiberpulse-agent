#!/bin/sh
set -eu

version="${FIBERPULSE_VERSION:-0.1.0-dev}"
output_root="${1:-dist/macos}"
app_path="${output_root}/FiberPulse.app"

if [ -e "$app_path" ]; then
  echo "Refusing to overwrite existing app bundle: $app_path" >&2
  exit 1
fi

mkdir -p "$app_path/Contents/MacOS" "$app_path/Contents/Resources"
CGO_ENABLED=1 go build -trimpath -buildvcs=false -ldflags="-s -w -X main.version=$version" -o "$app_path/Contents/MacOS/fiberpulse" ./cmd/fiberpulse
CGO_ENABLED=1 go build -trimpath -buildvcs=false -ldflags="-s -w" -o "$app_path/Contents/MacOS/fiberpulse-updater" ./cmd/fiberpulse-updater
cp packaging/macos/Info.plist "$app_path/Contents/Info.plist"
cp LICENSE "$app_path/Contents/Resources/LICENSE"
codesign --force --deep --sign - "$app_path"
codesign --verify --deep --strict "$app_path"
ditto -c -k --keepParent "$app_path" "${output_root}/FiberPulse-${version}-macos.zip"
shasum -a 256 "${output_root}/FiberPulse-${version}-macos.zip"
