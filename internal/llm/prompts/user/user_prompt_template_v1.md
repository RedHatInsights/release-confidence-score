Analyze this GitLab merge request for production release confidence and respond with structured JSON:

## Code Changes
{{.Diff}}

{{if .TruncationMetadata}}
### ⚠️ Analysis Limitations
Due to diff size, some patches were truncated to fit context window:
- **Truncation level**: {{.TruncationMetadata.Level}}
- **Files fully analyzed**: {{.TruncationMetadata.FilesPreserved}}/{{.TruncationMetadata.TotalFiles}}
- **Files with truncated patches**: {{.TruncationMetadata.FilesTruncated}} (primarily low-risk files like tests/docs)
- **Critical/security files**: fully preserved
- **All file metadata** (names, change stats): preserved for complete risk assessment

**How to identify truncated patches**: Look for the marker `... [TRUNCATED: ~N lines removed for brevity] ...` within a file's diff. When you see this, the patch shows only the beginning and end of the changes - the middle portion was removed to save space.

Despite truncation, you still have comprehensive information about all changes. Use the file metadata, preserved critical code, and beginning/end context from truncated patches to perform a thorough risk analysis.

{{end}}
{{if .UserGuidance}}
## Additional Analysis Guidance
The following guidance was provided in the merge request comments to guide your analysis:

{{range .UserGuidance}}- {{.}}
{{end}}
Please incorporate this guidance into your analysis.

{{end}}
{{if .QETesting}}
## QE Testing Status

{{if .QETesting.Tested}}
### QE Tested Commits
{{range .QETesting.Tested}}
**Repository**: {{.RepoURL}}
{{range .Commits}}- {{.}}
{{end}}
{{end}}
{{end}}
{{if .QETesting.NeedsTesting}}
### Needs QE Testing Commits
{{range .QETesting.NeedsTesting}}
**Repository**: {{.RepoURL}}
{{range .Commits}}- {{.}}
{{end}}
{{end}}
{{end}}
*Evaluate confidence impact based on testing status and the criticality of each change.*

{{end}}
{{if .Documentation}}

## Documentation
{{.Documentation}}

{{end}}
Provide your analysis in the exact JSON format specified in the system prompt. Include all required fields and ensure the JSON is valid.
