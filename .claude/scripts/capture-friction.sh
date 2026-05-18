#!/bin/bash
# Friction capture script — runs as a Claude Code Stop hook.
# Analyzes the session transcript for friction events and writes
# one markdown file per event to .claude/friction/.

LOG_FILE="${CLAUDE_PROJECT_DIR:-.}/.claude/friction/capture.log"

log() {
  echo "$(date +%H:%M:%S) $1" >> "$LOG_FILE"
}

# Opt-in: set FRICTION_CAPTURE=1 in .claude/settings.local.json
if [ "$FRICTION_CAPTURE" != "1" ]; then
  exit 0
fi

# Stop hook receives JSON on stdin with transcript_path and session_id
INPUT=$(cat)

# Prevent infinite recursion if this hook triggers another Stop
if echo "$INPUT" | jq -e '.stop_hook_active == true' > /dev/null 2>&1; then
  exit 0
fi

TRANSCRIPT_PATH=$(echo "$INPUT" | jq -r '.transcript_path')
SESSION_ID=$(echo "$INPUT" | jq -r '.session_id')
PROJECT_DIR="${CLAUDE_PROJECT_DIR:-.}"
FRICTION_DIR="$PROJECT_DIR/.claude/friction"
TODAY=$(date +%Y-%m-%d)
SHORT_SESSION=$(echo "$SESSION_ID" | cut -c1-4)

# Extract human/assistant messages from the JSONL transcript, truncate each to 2000 chars
TRANSCRIPT=$(jq -r '
  select(.role == "user" or .role == "assistant") |
  "[" + .role + "]: " + (
    if (.content | type) == "array"
    then [.content[] | select(.type == "text") | .text] | join(" ")
    else (.content // "")
    end
  )[:2000]
' "$TRANSCRIPT_PATH")

mkdir -p "$(dirname "$LOG_FILE")"

# Skip near-empty sessions
if [ ${#TRANSCRIPT} -lt 200 ]; then
  log "Session $SHORT_SESSION: transcript too short, skipping"
  exit 0
fi

# Ask Haiku to identify friction events as a JSON array
PROMPT="Analyze this coding agent session for friction signals.

Friction = moments where the human corrected the agent, the agent asked a question
it shouldn't have needed to ask, the agent made a wrong assumption, or a tool call
was denied.

For each friction event found, output a JSON array. Each element must have these fields:
- type: one of correction, clarification, denial, mistake
- severity: one of low, medium, high
- slug: short-kebab-case-description
- doc_gap: path/to/guideline.md or none
- description: one paragraph describing what went wrong and what the correct behavior should have been

If no friction was found, output an empty array: []

Output ONLY the JSON array, no other text.

TRANSCRIPT:
$TRANSCRIPT"

log "Session $SHORT_SESSION: analyzing transcript for friction..."

# Uses the claude CLI so the hook inherits the contributor's existing auth
RESULT=$(echo "$PROMPT" | claude -p --model haiku)

if [ -z "$RESULT" ] || [ "$RESULT" = "[]" ]; then
  log "Session $SHORT_SESSION: no friction found"
  exit 0
fi

if ! echo "$RESULT" | jq empty 2>/dev/null; then
  log "Session $SHORT_SESSION: Haiku returned invalid JSON, skipping"
  exit 0
fi

EVENT_COUNT=$(echo "$RESULT" | jq 'length')
log "Session $SHORT_SESSION: found $EVENT_COUNT friction event(s)"

mkdir -p "$FRICTION_DIR"

# Write one markdown file per friction event
echo "$RESULT" | jq -c '.[]' | while read -r event; do
  SLUG=$(echo "$event" | jq -r '.slug // "unknown"')
  TYPE=$(echo "$event" | jq -r '.type')
  SEVERITY=$(echo "$event" | jq -r '.severity')
  DOC_GAP=$(echo "$event" | jq -r '.doc_gap')
  DESC=$(echo "$event" | jq -r '.description')

  FILENAME="${TODAY}-${SHORT_SESSION}-${SLUG}.md"
  log "  -> $FILENAME ($TYPE, $SEVERITY)"
  cat > "$FRICTION_DIR/$FILENAME" <<EOF
---
type: $TYPE
severity: $SEVERITY
slug: $SLUG
doc_gap: $DOC_GAP
session: $SHORT_SESSION
date: $TODAY
---

$DESC
EOF
done

log "Session $SHORT_SESSION: friction capture complete"
