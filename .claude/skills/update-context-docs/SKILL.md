---
name: update-context-docs
description: Process captured friction events to improve context documentation
allowedTools:
  - Bash(bash *)
  - Bash(curl *)
  - Bash(find *)
  - Bash(git branch *)
  - Bash(git checkout *)
  - Bash(git fetch *)
  - Bash(git rev-parse *)
  - Edit
  - Read
---

# Instructions

## Phase 1: Version check

Check for toolkit updates:

1. Read `.claude/.friction-capture-version`.
2. Fetch the latest release tag:
   ```bash
   curl -fsSL https://api.github.com/repos/gwenneg/blog-ai-friction-loop/releases/latest \
     | jq -r '.tag_name'
   ```
3. Compare as semver (strip leading `v` before comparing). If the remote tag is newer, use `AskUserQuestion` to offer updating:

   > A newer version of the friction capture toolkit is available.
   > Installed: {local} → Latest: {remote}
   > Update now before processing friction events?

   Options: **Yes** / **No, continue anyway**

   If the user selects **Yes**, run:
   ```bash
   bash .claude/scripts/install-friction-capture.sh
   ```
   - Exit 0: tell the user "Friction capture updated to {version}.", then continue to Phase 2.
   - Exit 2: tell the user "Friction capture updated to {version}. The skill definition was updated — please restart Claude and re-run /update-context-docs." Then stop.
   - Any other non-zero exit: report the error and stop.

## Phase 2: Read friction events

Read all `*.md` files from `.claude/friction/` recursively (each session has its own subfolder: `.claude/friction/{session-id}/{slug}.md`). If none exist, report "No friction events to process." and stop.

For each file, extract the YAML frontmatter fields (`type`, `doc_gap`, `date`) and the body paragraph. Group events by `doc_gap` — the named file is the edit target.

## Phase 3: User exclusion

Print a numbered list of all events:
```
Events to be processed:
  1. one-sentence summary of what friction it captured
  2. ...
```

Then use `AskUserQuestion` with a single question: "Enter the numbers of any events to exclude (comma-separated), or proceed with all." Provide one option — "Proceed with all" — and rely on the "Other" free-text input for exclusions.

Events the user excludes are tracked as "Skipped by user" and must not result in any doc edits.

## Phase 4: Create branch

Detect the default branch with `git rev-parse --abbrev-ref origin/HEAD` and check whether `friction/update-context-docs-YYYY-MM-DD` already exists with `git branch --list` (increment a numeric suffix until the name is unique, e.g. `-2`, `-3`). Then run:

```bash
git fetch origin
git checkout -b friction/update-context-docs-YYYY-MM-DD origin/<default-branch>
```

## Phase 5: Apply edits

For each non-excluded event:
- Only edit files that exist
- Read the target file
- Assess severity based on the event description, the target file's existing content, and event count for that file:
  - **low**: edge case or minor clarification — add one targeted rule or example, no restructuring
  - **medium**: recurring pattern or missing convention — add a rule with an example
  - **high**: fundamental gap affecting core behavior — broader edit, may touch related sections
- Add a rule, example, or clarification that prevents the friction from recurring
- Follow the file's existing formatting conventions
- Do not reorganize existing content
- Do not push a file past 200 lines — consolidate instead of appending

## Phase 6: Propose eval cases (optional)

If a `promptfoo.yaml` or similar eval config exists, propose a new test case for each friction event backed by multiple events whose expected behavior can be verified with a `contains`/`not-contains` assertion rather than LLM judgment. Each case needs: `description`, `vars.task`, and at least one `contains`/`not-contains` or `llm-rubric` assertion.

## Phase 7: Clean up

Delete the session subfolders for all processed friction events from `.claude/friction/`. Each subfolder is named after the session ID; deleting it removes all friction files for that session at once.

## Phase 8: Commit and open a PR

Run `git add -A`, commit with message `docs: improve context docs from friction capture`, push, and open a PR. The PR body must contain:
- One line per friction event in the format `YYYY-MM-DD — <what the friction was> → <what changed>`
- A metrics section:

```
## Friction Metrics

### Events
| Type | Count |
|---|---|
| Corrections | N |
| Clarifications | N |
| Denials | N |
| Mistakes | N |
| **Total** | **N** |

### Outcomes
| Result | Count |
|---|---|
| Docs improved | N |
| Eval cases added | N |
| Skipped by user | N |

**Docs improved:** `file1.md`, `file2.md`
```

- The standard `Generated with Claude Code` footer
