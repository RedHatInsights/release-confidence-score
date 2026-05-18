# Improve Documentation from Friction

Process captured friction events and propose documentation and eval improvements.

## Instructions

1. Read all markdown files from `.claude/friction/` directory. If no friction files exist, report "No friction files to process" and stop.

2. For each friction file, extract the YAML frontmatter (`type`, `severity`, `slug`, `doc_gap`, `session`, `date`) and the body description.

3. Analyze the friction events as a batch:
   - Group by `doc_gap` field to identify which guideline docs need updates.
   - Filter out noise: single low-severity events with no clear doc gap are likely random sampling errors or one-off edge cases. Flag them for the user but do not propose changes.
   - Identify patterns: multiple events pointing to the same guideline doc are high-confidence signals.

4. For each identified doc gap:
   - Read the existing guideline file (e.g., `docs/security-guidelines.md`).
   - Propose a specific edit: add a rule, add an example, clarify an existing rule.
   - The edit should prevent the friction from recurring.
   - Follow existing doc formatting conventions (numbered sections with `##` headers, code examples).
   - Keep additions concise — one rule per friction event, not a paragraph.
   - Do not reorganize existing content. Add to the relevant section.
   - Each guideline file must not exceed 200 lines after editing. If an edit would push a file over 200 lines, consolidate or tighten existing rules instead of appending.

5. For friction events where `doc_gap` is "none":
   - Determine if the friction points to a missing guideline topic.
   - If so, propose adding content to the most relevant existing guideline file, or suggest a new path-scoped rule in `.claude/rules/`.

6. Optionally propose new eval cases:
   - For friction events that are grader-friendly (the expected behavior can be verified with `contains`/`not-contains` or `llm-rubric` assertions), propose a new test case in `promptfoo.yaml`.
   - Each test case needs: `description`, `vars.task`, and at least one `assert`.
   - The task description should be a realistic coding request that would trigger the friction pattern.
   - Prefer `contains`/`not-contains` for concrete patterns, `llm-rubric` for nuanced intent.

7. Delete all processed friction files from `.claude/friction/`.

8. Create a new git branch named `friction/improve-docs-YYYY-MM-DD` (add a short suffix if the branch already exists).

9. Stage and commit all changes (doc edits, eval additions).
   Use commit message format: `docs: improve guidelines from friction capture`

10. Push the branch and open a PR using `gh pr create`:
    - Title: `docs: improve guidelines from friction capture`
    - Body: summary of what was changed and why, listing each friction event that drove each change. End with the standard `Generated with Claude Code` footer.
    - Base branch: `main`

11. After the PR is created, post a metrics comment on the PR using `gh pr comment` with this format:

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
