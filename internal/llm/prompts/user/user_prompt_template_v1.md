Analyze these code changes for production release confidence and respond with structured JSON:

## Code Changes
{{.Diff}}

{{- if .TruncationMetadata}}
### ⚠️ Truncation Applied
**Level**: {{.TruncationMetadata.Level}} | **Preserved**: {{.TruncationMetadata.FilesPreserved}}/{{.TruncationMetadata.TotalFiles}} files | **Truncated**: {{.TruncationMetadata.FilesTruncated}} files

Patches truncated to fit context limits. Look for `[N lines omitted]` markers in diffs.
- **All metadata preserved**: filenames, change counts, commits, authors, PR/MR numbers, QE labels
- **Critical files** (DB, security, APIs) preserved completely at lower levels; low-risk files (tests, docs) truncated first
- **Patch edges preserved**: You see the beginning and end of each change, middle sections omitted

Analyze using the preserved context at patch boundaries combined with file metadata.

{{- end}}

{{- if .UserGuidance}}
## Additional Analysis Guidance
The following guidance was provided to guide your analysis:

{{- range .UserGuidance}}
- {{.}}
{{- end}}

Please incorporate this guidance into your analysis.

{{- end}}

{{- if .Documentation}}
## Documentation
{{.Documentation}}

{{- end}}

Provide your analysis in the exact JSON format specified in the system prompt. Include all required fields and ensure the JSON is valid.
