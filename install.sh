#!/bin/sh

set -eu

REPOSITORY="aston76/fiberpulse-agent"
RELEASE_API="https://api.github.com/repos/${REPOSITORY}/releases/latest"
TEMP_ROOT="${TMPDIR:-/tmp}"

fail() {
  echo "FiberPulse installation stopped: $*" >&2
  exit 1
}

for REQUIRED_TOOL in curl awk ditto codesign spctl; do
  command -v "$REQUIRED_TOOL" >/dev/null 2>&1 || fail "required tool '$REQUIRED_TOOL' is unavailable."
done

INSTALL_TEMP_DIR="$(mktemp -d "${TEMP_ROOT%/}/fiberpulse-install.XXXXXX")" || fail "could not create a temporary directory."
cleanup() {
  rm -rf "$INSTALL_TEMP_DIR"
}
trap cleanup EXIT HUP INT TERM

case "$INSTALL_TEMP_DIR" in
  "${TEMP_ROOT%/}"/fiberpulse-install.*) ;;
  *) fail "temporary directory validation failed." ;;
esac

RELEASE_JSON="$INSTALL_TEMP_DIR/release.json"
if ! curl --proto '=https' --tlsv1.2 -fsSL "$RELEASE_API" -o "$RELEASE_JSON"; then
  fail "no signed public release is available yet."
fi

ASSET_URL="$(awk -F'"' '/"browser_download_url":.*FiberPulse-.*-macos\.zip/ { print $4; exit }' "$RELEASE_JSON")"
case "$ASSET_URL" in
  "https://github.com/${REPOSITORY}/releases/download/"*/FiberPulse-*-macos.zip) ;;
  *) fail "the latest release does not contain the expected signed macOS application." ;;
esac

ARCHIVE_PATH="$INSTALL_TEMP_DIR/FiberPulse.zip"
curl --proto '=https' --tlsv1.2 -fL "$ASSET_URL" -o "$ARCHIVE_PATH" || fail "the release download failed."
ditto -x -k "$ARCHIVE_PATH" "$INSTALL_TEMP_DIR" || fail "the application archive could not be extracted."

SOURCE_APP="$INSTALL_TEMP_DIR/FiberPulse.app"
[ -d "$SOURCE_APP" ] || fail "FiberPulse.app is missing from the signed release."
codesign --verify --deep --strict "$SOURCE_APP" >/dev/null 2>&1 || fail "the macOS application signature is invalid."
spctl --assess --type execute "$SOURCE_APP" >/dev/null 2>&1 || fail "Gatekeeper did not approve this application."

TARGET_DIR="$HOME/Applications"
TARGET_APP="$TARGET_DIR/FiberPulse.app"
if [ -e "$TARGET_APP" ]; then
  echo "FiberPulse is already installed at $TARGET_APP. Use its built-in updater for new versions."
  open "$TARGET_APP"
  exit 0
fi

mkdir -p "$TARGET_DIR" || fail "could not create $TARGET_DIR."
ditto "$SOURCE_APP" "$TARGET_APP" || fail "the application could not be installed."
open "$TARGET_APP" || fail "FiberPulse was installed but could not be opened."

echo "FiberPulse was installed in $TARGET_APP."
