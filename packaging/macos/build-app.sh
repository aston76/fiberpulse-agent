#!/bin/sh
set -eu

version="${FIBERPULSE_VERSION:-0.1.2-dev}"
share_url="${FIBERPULSE_SHARE_URL:-}"
update_feed_url="${FIBERPULSE_UPDATE_FEED_URL:-}"
update_public_key="${FIBERPULSE_UPDATE_PUBLIC_KEY:-}"
output_root="${1:-dist/macos}"
app_path="${output_root}/FiberPulse.app"
deployment_target="${MACOSX_DEPLOYMENT_TARGET:-13.0}"
signing_identity="${FIBERPULSE_CODESIGN_IDENTITY:--}"
build_number="${FIBERPULSE_BUILD_NUMBER:-4}"
build_root="$(mktemp -d "${TMPDIR:-/tmp}/fiberpulse-macos-build.XXXXXX")"

cleanup() {
  rm -rf "$build_root"
}
trap cleanup EXIT HUP INT TERM

build_binary() {
  output_path="$1"
  package_path="$2"
  linker_flags="$3"
  CGO_ENABLED=1 \
    GOOS=darwin \
    GOARCH="$goarch" \
    CC=clang \
    CXX=clang++ \
    MACOSX_DEPLOYMENT_TARGET="$deployment_target" \
    CGO_CFLAGS="-arch $clang_arch -mmacosx-version-min=$deployment_target" \
    CGO_LDFLAGS="-arch $clang_arch -mmacosx-version-min=$deployment_target" \
    go build -trimpath -buildvcs=false -ldflags="$linker_flags" -o "$output_path" "$package_path"
}

if [ -e "$app_path" ]; then
  echo "Refusing to overwrite existing app bundle: $app_path" >&2
  exit 1
fi

mkdir -p "$app_path/Contents/MacOS" "$app_path/Contents/Resources"
for goarch in arm64 amd64; do
  case "$goarch" in
    arm64) clang_arch="arm64" ;;
    amd64) clang_arch="x86_64" ;;
    *) echo "Unsupported macOS architecture: $goarch" >&2; exit 1 ;;
  esac
  arch_root="$build_root/$goarch"
  mkdir -p "$arch_root"
  build_binary "$arch_root/fiberpulse" ./cmd/fiberpulse "-s -w -X main.version=$version -X main.sharingEndpoint=$share_url -X main.updateFeedURL=$update_feed_url -X main.updatePublicKey=$update_public_key"
  build_binary "$arch_root/fiberpulse-updater" ./cmd/fiberpulse-updater "-s -w"
done
lipo -create "$build_root/arm64/fiberpulse" "$build_root/amd64/fiberpulse" -output "$app_path/Contents/MacOS/fiberpulse"
lipo -create "$build_root/arm64/fiberpulse-updater" "$build_root/amd64/fiberpulse-updater" -output "$app_path/Contents/MacOS/fiberpulse-updater"
lipo "$app_path/Contents/MacOS/fiberpulse" -verify_arch arm64 x86_64
lipo "$app_path/Contents/MacOS/fiberpulse-updater" -verify_arch arm64 x86_64
cp packaging/macos/Info.plist "$app_path/Contents/Info.plist"
cp packaging/macos/FiberPulse.icns "$app_path/Contents/Resources/FiberPulse.icns"
cp packaging/macos/dev.fiberpulse.agent.plist "$app_path/Contents/Resources/dev.fiberpulse.agent.plist"
cp LICENSE "$app_path/Contents/Resources/LICENSE"
/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString $version" "$app_path/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Set :CFBundleVersion $build_number" "$app_path/Contents/Info.plist"
if [ "$signing_identity" = "-" ]; then
  codesign --force --sign - "$app_path/Contents/MacOS/fiberpulse-updater"
  codesign --force --sign - "$app_path/Contents/MacOS/fiberpulse"
  codesign --force --sign - "$app_path"
else
  codesign --force --options runtime --timestamp --sign "$signing_identity" "$app_path/Contents/MacOS/fiberpulse-updater"
  codesign --force --options runtime --timestamp --sign "$signing_identity" "$app_path/Contents/MacOS/fiberpulse"
  codesign --force --options runtime --timestamp --sign "$signing_identity" "$app_path"
fi
codesign --verify --deep --strict "$app_path"
ditto -c -k --keepParent "$app_path" "${output_root}/FiberPulse-${version}-macos.zip"
shasum -a 256 "${output_root}/FiberPulse-${version}-macos.zip"
