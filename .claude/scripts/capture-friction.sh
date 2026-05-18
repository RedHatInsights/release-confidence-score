#!/bin/bash
# Friction capture script — runs as a Claude Code SessionEnd hook.
#
# At the end of every session, Claude Code invokes this script with a JSON payload
# on stdin containing the transcript path. This script asks a small LLM (Haiku) to
# scan the transcript for friction events (corrections, mistakes, clarifications,
# denied tool calls) and writes one markdown file per event to .claude/friction/.
#
# Those files are later processed by /improve-docs to propose doc improvements.
# Enable with FRICTION_CAPTURE=1 in .claude/settings.local.json.

if [ "$FRICTION_CAPTURE" != "1" ]; then
  exit 0
fi

PROJECT_DIR="${CLAUDE_PROJECT_DIR:-.}"
FRICTION_DIR="$PROJECT_DIR/.claude/friction"
LOG_FILE="$FRICTION_DIR/capture.log"

mkdir -p "$FRICTION_DIR"

log() {
  echo "$(date +%H:%M:%S) $1" >> "$LOG_FILE"
}

LOCKFILE="$FRICTION_DIR/.capture.lock"

# Read the JSON payload from stdin synchronously. This must happen before we
# background anything: Claude Code closes stdin when the hook process exits, so
# a background subshell cannot read it.
INPUT=$(cat)

# Everything below runs in a background subshell so this hook returns immediately
# and Claude Code is not blocked waiting for the LLM call to finish.
#
# True async requires two things beyond just `&`:
#
# 1. Redirect stdin/stdout/stderr away from Claude Code's pipes.
#    Claude Code captures the hook's output via pipes and waits for EOF on them.
#    If the background subshell inherits those pipe FDs, Claude Code blocks until
#    the subshell exits — making `disown` ineffective. Redirecting to /dev/null
#    and the log file closes the inherited FDs immediately, so Claude Code can
#    proceed as soon as the hook script exits.
#
# 2. disown the background job so the shell doesn't report it on exit.
{
  # Acquire a non-blocking lock inside the background block (not before it) so
  # the hook exits immediately even if another instance is still running.
  #
  # This also prevents infinite recursion: running `claude -p` below starts a new
  # Claude session, which fires another SessionEnd hook when it exits. That nested
  # hook arrives here, fails flock -n, and exits instantly.
  exec 9>"$LOCKFILE"
  if ! flock -n 9; then
    exit 0
  fi

  TRANSCRIPT_PATH=$(echo "$INPUT" | jq -r '.transcript_path // empty')
  SESSION_ID=$(echo "$INPUT" | jq -r '.session_id // empty')
  SHORT_SESSION="${SESSION_ID:0:4}"
  [ -z "$SHORT_SESSION" ] && SHORT_SESSION="unkn"

  if [ -z "$TRANSCRIPT_PATH" ]; then
    log "ERROR: Session $SHORT_SESSION: missing transcript_path in input, cannot proceed"
    exit 0
  fi

  log "INFO: Session $SHORT_SESSION: friction capture started"
  log "DEBUG: transcript_path = $TRANSCRIPT_PATH"

  TODAY=$(date +%Y-%m-%d)

  # The transcript is JSONL. Extract text content from user and assistant turns.
  # Turns are kept whole — truncating individual turns risks cutting the end of an
  # assistant response or a user correction, which is exactly where friction lives.
  # Instead, cap the total at 200K chars by dropping from the front: recent turns
  # matter most for friction detection, and Haiku's context window is not a concern.
  TRANSCRIPT=$(jq -r '
  select(.type == "user" or .type == "assistant") |
  "[" + .type + "]: " + (
    if (.message.content | type) == "array"
    then [.message.content[] | select(.type == "text") | .text] | join(" ")
    else (.message.content // "")
    end
  )
' "$TRANSCRIPT_PATH" 2>> "$LOG_FILE")
  TRANSCRIPT="${TRANSCRIPT: -200000}"

  if [ $? -ne 0 ]; then
    log "ERROR: Session $SHORT_SESSION: jq failed to parse transcript at $TRANSCRIPT_PATH"
    exit 0
  fi

  if [ ${#TRANSCRIPT} -lt 200 ]; then
    log "INFO: Session $SHORT_SESSION: transcript too short (${#TRANSCRIPT} chars), skipping"
    exit 0
  fi

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

<transcript>
$TRANSCRIPT
</transcript>

Output ONLY valid JSON. Do not use markdown formatting, code fences, or any other text."

  # `claude -p` runs a non-interactive prompt using the contributor's local auth.
  T0=$SECONDS
  RESULT=$(echo "$PROMPT" | timeout 300 claude -p --model haiku 2>> "$LOG_FILE")
  CLAUDE_EXIT=$?

  log "DEBUG: claude took $((SECONDS - T0))s, exit code = $CLAUDE_EXIT, result length = ${#RESULT}"

  if [ $CLAUDE_EXIT -eq 124 ]; then
    log "ERROR: Session $SHORT_SESSION: claude timed out after 300s"
    exit 0
  fi

  if [ $CLAUDE_EXIT -ne 0 ]; then
    log "ERROR: Session $SHORT_SESSION: claude exited with code $CLAUDE_EXIT"
    exit 0
  fi

  # Models sometimes wrap JSON in markdown fences despite explicit instructions not to.
  RESULT=$(echo "$RESULT" | sed '/^```/d')

  if [ -z "$RESULT" ] || [ "$RESULT" = "[]" ]; then
    log "INFO: Session $SHORT_SESSION: no friction found"
    exit 0
  fi

  if ! echo "$RESULT" | jq empty 2>/dev/null; then
    log "ERROR: Session $SHORT_SESSION: Haiku returned invalid JSON: $(echo "$RESULT" | head -c 200)"
    exit 0
  fi

  EVENT_COUNT=$(echo "$RESULT" | jq 'length')
  log "INFO: found $EVENT_COUNT friction event(s)"

  # Write one markdown file per friction event with YAML frontmatter.
  # The /improve-docs skill reads these files to propose documentation changes.
  echo "$RESULT" | jq -c '.[]' | while read -r event; do
    SLUG=$(echo "$event" | jq -r '.slug // "unknown"')
    TYPE=$(echo "$event" | jq -r '.type')
    SEVERITY=$(echo "$event" | jq -r '.severity')
    DOC_GAP=$(echo "$event" | jq -r '.doc_gap')
    DESC=$(echo "$event" | jq -r '.description')

    FILENAME="${TODAY}-${SHORT_SESSION}-${SLUG}.md"
    log "INFO: -> $FILENAME ($TYPE, $SEVERITY)"
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

  log "INFO: Session $SHORT_SESSION: friction capture complete"

# Redirect stdin/stdout/stderr so the background subshell does not hold open
# Claude Code's pipes (see async explanation at the top of the block).
} </dev/null >>"$LOG_FILE" 2>&1 &
disown
