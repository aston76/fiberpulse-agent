#!/bin/sh

set -eu

REPOSITORY="aston76/fiberpulse-agent"
RELEASE_API="https://api.github.com/repos/${REPOSITORY}/releases/latest"
TEMP_ROOT="${TMPDIR:-/tmp}"

fail() {
  echo "FiberPulse installation stopped: $*" >&2
  exit 1
}

for REQUIRED_TOOL in curl awk ditto codesign spctl plutil launchctl id install; do
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

LAUNCH_AGENT_DIR="$HOME/Library/LaunchAgents"
LAUNCH_AGENT_PATH="$LAUNCH_AGENT_DIR/dev.fiberpulse.agent.plist"
LOG_DIR="$HOME/Library/Logs/FiberPulse"
LAUNCH_AGENT_TEMPLATE="$TARGET_APP/Contents/Resources/dev.fiberpulse.agent.plist"
STAGED_LAUNCH_AGENT="$INSTALL_TEMP_DIR/dev.fiberpulse.agent.plist"
[ -f "$LAUNCH_AGENT_TEMPLATE" ] || fail "the signed release is missing its start-at-login definition."
mkdir -p "$LAUNCH_AGENT_DIR" "$LOG_DIR" || fail "the start-at-login directories could not be created."
cp "$LAUNCH_AGENT_TEMPLATE" "$STAGED_LAUNCH_AGENT" || fail "the start-at-login definition could not be prepared."
plutil -replace ProgramArguments.0 -string "$TARGET_APP/Contents/MacOS/fiberpulse" "$STAGED_LAUNCH_AGENT" || fail "the application launch path could not be configured."
plutil -replace StandardOutPath -string "$LOG_DIR/launchd.stdout.log" "$STAGED_LAUNCH_AGENT" || fail "the output log path could not be configured."
plutil -replace StandardErrorPath -string "$LOG_DIR/launchd.stderr.log" "$STAGED_LAUNCH_AGENT" || fail "the error log path could not be configured."
plutil -lint "$STAGED_LAUNCH_AGENT" >/dev/null || fail "the start-at-login definition is invalid."
install -m 600 "$STAGED_LAUNCH_AGENT" "$LAUNCH_AGENT_PATH" || fail "start-at-login could not be installed."

LAUNCH_DOMAIN="gui/$(id -u)"
launchctl bootout "$LAUNCH_DOMAIN/dev.fiberpulse.agent" >/dev/null 2>&1 || true
if ! launchctl bootstrap "$LAUNCH_DOMAIN" "$LAUNCH_AGENT_PATH"; then
  mv "$LAUNCH_AGENT_PATH" "$INSTALL_TEMP_DIR/failed-launch-agent.plist" 2>/dev/null || true
  fail "FiberPulse was installed, but start-at-login could not be activated. Open it once from $TARGET_APP."
fi

echo "FiberPulse was installed in $TARGET_APP and will start automatically when you sign in."
