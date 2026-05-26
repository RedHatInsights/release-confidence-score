#!/bin/bash
# Installs or updates the friction capture toolkit to the latest release.
# Exit codes: 0 = success, 1 = error, 2 = success and Claude restart required
set -euo pipefail

REPO="gwenneg/blog-ai-friction-loop"
BASE_URL="https://raw.githubusercontent.com/${REPO}"
VERSION_FILE=".claude/.friction-capture-version"
DISABLE_SKILL_FILE=".claude/skills/disable-friction-capture/SKILL.md"
UPDATE_SKILL_FILE=".claude/skills/update-context-docs/SKILL.md"

TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | jq -r '.tag_name')

if [[ -z "$TAG" || "$TAG" == "null" ]]; then
  echo "Error: could not fetch latest release tag." >&2
  exit 1
fi

disable_skill_before=$(sha256sum "$DISABLE_SKILL_FILE" 2>/dev/null | awk '{print $1}' || echo "")
update_skill_before=$(sha256sum "$UPDATE_SKILL_FILE" 2>/dev/null | awk '{print $1}' || echo "")

mkdir -p "$(dirname "$DISABLE_SKILL_FILE")"
curl -fsSL "${BASE_URL}/${TAG}/skills/disable-friction-capture/SKILL.md" \
  -o "$DISABLE_SKILL_FILE"

mkdir -p "$(dirname "$UPDATE_SKILL_FILE")"
curl -fsSL "${BASE_URL}/${TAG}/skills/update-context-docs/SKILL.md" \
  -o "$UPDATE_SKILL_FILE"

curl -fsSL "${BASE_URL}/${TAG}/skills/setup-friction-capture/scripts/friction-capture.sh" \
  -o .claude/scripts/friction-capture.sh
chmod +x .claude/scripts/friction-capture.sh

curl -fsSL "${BASE_URL}/${TAG}/skills/setup-friction-capture/scripts/friction-reminder.sh" \
  -o .claude/scripts/friction-reminder.sh
chmod +x .claude/scripts/friction-reminder.sh

curl -fsSL "${BASE_URL}/${TAG}/skills/setup-friction-capture/scripts/install-friction-capture.sh" \
  -o .claude/scripts/install-friction-capture.sh
chmod +x .claude/scripts/install-friction-capture.sh

echo "$TAG" > "$VERSION_FILE"

disable_skill_after=$(sha256sum "$DISABLE_SKILL_FILE" | awk '{print $1}')
update_skill_after=$(sha256sum "$UPDATE_SKILL_FILE" | awk '{print $1}')
if [[ "$disable_skill_before" != "$disable_skill_after" || "$update_skill_before" != "$update_skill_after" ]]; then
  exit 2
fi
