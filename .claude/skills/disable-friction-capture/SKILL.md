---
name: disable-friction-capture
description: Disable friction capture for this repo
allowedTools:
  - Bash(bash *)
  - Bash(jq *)
  - Bash(mkdir *)
  - Bash(mktemp *)
  - Bash(mv *)
---

# Instructions

Disable friction capture for this repository by setting `FRICTION_CAPTURE=0` in `.claude/settings.local.json`.

Run:

```bash
mkdir -p .claude
FILE=".claude/settings.local.json"
[ -f "$FILE" ] || echo '{}' > "$FILE"
# jq reads the file directly and writes to a temp file — settings.local.json never
# passes through stdout, which would expose its contents to the Claude session.
TMP=$(mktemp)
jq '.env.FRICTION_CAPTURE = "0"' "$FILE" > "$TMP"
mv "$TMP" "$FILE"
```

Then tell the user:

> Friction capture is now disabled for this repo. The session-start reminder and session-end capture will no longer run. To re-enable, delete the `FRICTION_CAPTURE` key from `.claude/settings.local.json` or set it to `"1"`.
