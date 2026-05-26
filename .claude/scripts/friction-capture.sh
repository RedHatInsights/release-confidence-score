#!/bin/bash
# SessionEnd hook — captures friction events at the end of each Claude Code session.
#
# At the end of every session, Claude Code invokes this script with a JSON payload
# on stdin containing the transcript path. This script asks a small LLM (Haiku) to
# scan the transcript for friction events (corrections, mistakes, clarifications,
# denied tool calls) and writes one markdown file per event to .claude/friction/{session-id}/.
#
# Those files are later processed by /update-context-docs to propose doc improvements.
# Disable with FRICTION_CAPTURE=0 in .claude/settings.local.json.

if [ "${FRICTION_CAPTURE:-1}" != "1" ]; then
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
  SESSION_DIR="$FRICTION_DIR/${SESSION_ID:-unkn}"
  mkdir -p "$SESSION_DIR"

  # The transcript is a JSONL file where each line is one record. The first lines are
  # metadata (permissionMode, snapshot) written at session start; user/assistant turns
  # are appended throughout and may not be fully written when this hook fires.
  #
  # Poll until jq finds at least one user or assistant turn, or give up after 60 seconds.
  # A jq parse error means the file is structurally invalid and won't recover — exit immediately.
  #
  # Each turn is extracted as "[user]: ..." or "[assistant]: ..." text. The total is
  # capped at the last 200K bytes (tail -c) so recent turns — where friction lives —
  # are always preserved. Per-turn truncation is intentionally avoided: cutting an
  # assistant response or user correction mid-sentence would lose exactly the signal
  # we're trying to capture.
  WAIT=0
  TRANSCRIPT=""
  while [ $WAIT -lt 60 ]; do
    TRANSCRIPT=$(jq -r '
      select(.type == "user" or .type == "assistant") |
      "[" + .type + "]: " + (
        if (.message.content | type) == "array"
        then [.message.content[] | select(.type == "text") | .text] | join(" ")
        else (.message.content // "")
        end
      )
    ' "$TRANSCRIPT_PATH" 2>> "$LOG_FILE" | tail -c 200000)
    if [ "${PIPESTATUS[0]}" -ne 0 ]; then
      log "ERROR: Session $SHORT_SESSION: jq failed to parse transcript at $TRANSCRIPT_PATH"
      exit 0
    fi
    [ -n "$TRANSCRIPT" ] && break
    sleep 1
    WAIT=$((WAIT + 1))
  done
  log "DEBUG: transcript ready after ${WAIT}s"

  if [ -z "$TRANSCRIPT" ]; then
    log "ERROR: Session $SHORT_SESSION: no user/assistant turns found after 60s, skipping"
    exit 0
  fi

  if [ ${#TRANSCRIPT} -lt 200 ]; then
    log "INFO: Session $SHORT_SESSION: transcript too short (${#TRANSCRIPT} chars), skipping"
    exit 0
  fi

  PROMPT="Analyze this coding agent session for friction signals worth capturing as documentation gaps.

Friction = a moment where the human corrected the agent, the agent asked a question it
shouldn't have needed to ask, the agent made a wrong assumption, or a tool call was denied.

Before including an event, apply both filters:
1. Would adding a concrete rule, example, or convention to a specific project doc have
   prevented this in a future session?
2. Would the same misunderstanding likely recur on a similar task — not just this specific case?
If either answer is no, exclude the event.

Exclude the following — they are noise:
- User errors: the user made a mistake in their own request and corrected it themselves
- Tool denials that are one-off decisions (e.g. 'not yet', 'use this file instead') — but DO
  capture denials that reveal a standing project policy (e.g. 'we never run migrations directly')
- Mid-session scope changes or preference pivots by the user
- Transient or environmental errors (network issues, flaky tests, API timeouts)
- Corrections to generated code that are case-specific and would not recur on similar tasks
- Requirement clarifications that depend on context no project doc could have anticipated

For each qualifying event, output a JSON array. Each element must have:
- type: one of correction, clarification, denial, mistake
- slug: short-kebab-case-description
- doc_gap: path/to/target-file.md — the specific file where a rule should be added or clarified.
  If you cannot name a specific plausible target file, exclude the event entirely.
- description: one paragraph — what the agent did wrong, what project-specific knowledge was
  missing, and the concrete rule or example that would prevent it from recurring. If you cannot
  state that rule in one sentence, exclude the event.

If no qualifying friction was found, output an empty array: []

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
  # The /update-context-docs skill reads these files to propose documentation changes.
  echo "$RESULT" | jq -c '.[]' | while read -r event; do
    SLUG=$(echo "$event" | jq -r '.slug // "unknown"')
    TYPE=$(echo "$event" | jq -r '.type')
    DOC_GAP=$(echo "$event" | jq -r '.doc_gap')
    DESC=$(echo "$event" | jq -r '.description')

    FILENAME="${SLUG}.md"
    log "INFO: -> $SESSION_ID/$FILENAME ($TYPE)"
    cat > "$SESSION_DIR/$FILENAME" <<EOF
---
type: $TYPE
doc_gap: $DOC_GAP
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
