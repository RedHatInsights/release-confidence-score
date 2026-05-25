#!/bin/bash
# SessionStart hook — reminds developers to process accumulated friction files.
#
# At the start of each new conversation, Claude Code invokes this script. If enough
# sessions have accumulated unprocessed friction files, it prints a friendly reminder
# to run /update-context-docs.
#
# Disable with FRICTION_CAPTURE=0 in .claude/settings.local.json.

if [ "${FRICTION_CAPTURE:-1}" != "1" ]; then
  exit 0
fi

PROJECT_DIR="${CLAUDE_PROJECT_DIR:-.}"
FRICTION_DIR="$PROJECT_DIR/.claude/friction"
THRESHOLD="${FRICTION_SESSION_THRESHOLD:-3}"

SESSION_COUNT=$(find "$FRICTION_DIR" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | wc -l | tr -d ' ')

if [ "${SESSION_COUNT:-0}" -lt "$THRESHOLD" ]; then
  exit 0
fi

EVENT_COUNT=$(find "$FRICTION_DIR" -mindepth 2 -maxdepth 2 -name "*.md" 2>/dev/null | wc -l | tr -d ' ')

if [ "${EVENT_COUNT:-0}" -eq 0 ]; then
  exit 0
fi

B='\033[1m'
C='\033[36m'
Y='\033[33m'
D='\033[2m'
R='\033[0m'

# Each title must be exactly 50 visual chars (box interior width).
# Emoji count as 2 columns; trailing spaces pad to 50.
TITLES=(
  "  🤖 YOUR AI IS ASKING FOR HELP                   "
  "  🚨 THIS IS NOT A DRILL                          "
  "  📢 YOUR AI HAS AN ANNOUNCEMENT                  "
  "  🤖 BEEP BOOP IMPORTANT MESSAGE                  "
  "  🤖 YOUR AI FILED A BUG REPORT                   "
)

MESSAGES=(
  "  ${SESSION_COUNT} sessions. ${EVENT_COUNT} friction events. I counted.|  The docs won't update themselves. (I've tried.)"
  "  ${SESSION_COUNT} sessions, ${EVENT_COUNT} stumbles. I'm not proud.|  Update the docs. For both our sakes."
  "  Allegedly intelligent. Stumbled ${EVENT_COUNT} times.|  Evidence suggests the docs need updating."
  "  Good news: ${EVENT_COUNT} insights across ${SESSION_COUNT} sessions!|  Bad news: they're rotting in .claude/friction/."
  "  ERROR: ${EVENT_COUNT} friction events across ${SESSION_COUNT} sessions.|  RECOMMENDED ACTION: update the docs. Please."
)

TIDX=$((RANDOM % 5))
IDX=$((RANDOM % 5))
IFS='|' read -r MSG_LINE1 MSG_LINE2 <<< "${MESSAGES[$IDX]}"

printf "\n"
printf "${B}${C}╔══════════════════════════════════════════════════╗${R}\n"
printf "${B}${C}║${TITLES[$TIDX]}║${R}\n"
printf "${B}${C}╠══════════════════════════════════════════════════╣${R}\n"
printf "${B}${C}║${R}%-50s${B}${C}║${R}\n" ""
printf "${B}${C}║${R}%-50s${B}${C}║${R}\n" "$MSG_LINE1"
printf "${B}${C}║${R}%-50s${B}${C}║${R}\n" "$MSG_LINE2"
printf "${B}${C}║${R}%-50s${B}${C}║${R}\n" ""
printf "${B}${C}║  ${R}${Y}→ /update-context-docs${R}${B}${C}%-26s║${R}\n" ""
printf "${B}${C}║${R}%-50s${B}${C}║${R}\n" ""
printf "${B}${C}║${R}${D}%-50s${R}${B}${C}║${R}\n" "  Not your thing? Run /disable-friction-capture."
printf "${B}${C}╚══════════════════════════════════════════════════╝${R}\n"
printf "\n"
