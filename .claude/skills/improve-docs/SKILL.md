# Improve Documentation from Friction

Process captured friction events to improve documentation and evals. Checks for toolkit updates on every run.

## Instructions

### Phase 1: Health and version check

**Verify installation:**

Check that both files exist:
- `.claude/scripts/capture-friction.sh`
- `.claude/skills/improve-docs/SKILL.md`

If either is missing, print this warning and stop:
```
Warning: friction capture installation is incomplete.
Re-run /setup-friction-capture to repair it.
```

**Check for toolkit updates (best effort — skip silently on any network or parse error):**

1. Read `.claude/.friction-capture-version`. If missing, skip the version check.
2. Fetch the latest release tag:
   ```bash
   curl -fsSL https://api.github.com/repos/gwenneg/claude-engineering-toolkit/releases/latest \
     | jq -r '.tag_name'
   ```
3. Compare as semver (strip leading `v` before comparing). If the remote tag is newer, print:
   ```
   Notice: a newer version of the friction capture toolkit is available.
     Installed: {local} → Latest: {remote}
     Re-run /setup-friction-capture to update.
   ```
   Then continue.

### Phase 2: Read friction events

Read all `*.md` files from `.claude/friction/`. If none exist, report "No friction events to process." and stop.

For each file, extract the YAML frontmatter fields (`type`, `severity`, `slug`, `doc_gap`, `session`, `date`) and the body paragraph.

### Phase 3: Analyze

Group events by `doc_gap`. Apply these rules:

- `doc_gap` names an existing file → that file is the edit target
- `doc_gap` names a file that does not exist → map to the closest discovered file, or note that one should be created
- `doc_gap: none` → determine the most relevant existing file, or propose a new `.claude/rules/` entry
- Single low-severity event with no clear doc gap and no pattern → flag as noise, skip it

Multiple events pointing to the same file are a high-confidence signal; single events are weak signals.

### Phase 4: Discover documentation structure

Find candidate files for improvement:
- `docs/*.md` and `docs/**/*.md`
- `.claude/rules/*.md`
- Root-level `AGENTS.md`, `CLAUDE.md`, `CONTRIBUTING.md`

Only edit files that exist.

### Phase 5: Apply edits

For each confirmed gap:
- Read the target file
- Add a rule, example, or clarification that prevents the friction from recurring
- Follow the file's existing formatting conventions
- Do not reorganize existing content
- Do not push a file past 200 lines — consolidate instead of appending

### Phase 6: Propose eval cases (optional)

If a `promptfoo.yaml` or similar eval config exists, propose a new test case for each grader-friendly friction event. Each case needs: `description`, `vars.task`, and at least one `contains`/`not-contains` or `llm-rubric` assertion.

### Phase 7: Clean up

Delete all processed friction files from `.claude/friction/`.

### Phase 8: Commit and open a PR

Create branch `friction/improve-docs-YYYY-MM-DD` (add a short suffix if that branch already exists).

Commit all changes with message: `docs: improve guidelines from friction capture`

Push and open a PR:
- Title: `docs: improve guidelines from friction capture`
- Body: for each friction event, one line describing what it was and what changed. End with the standard `Generated with Claude Code` footer.
- Base branch: `main`

### Phase 9: Post metrics

After the PR is created, post a comment:

```
## Friction Metrics

| Metric | Count |
|---|---|
| Total friction events | N |
| Corrections | N |
| Clarifications | N |
| Denials | N |
| Mistakes | N |
| Docs improved | N |
| Eval cases added | N |
| Noise (discarded) | N |

**Docs improved:** `file1.md`, `file2.md`
```
