**⚠️ AI-Generated Report** — This report is AI-generated and advisory. Always review AI-generated content prior to use.

# 🚀 Release Confidence Report

## 🎯 Summary

**Confidence score:** {{.Analysis.Score}}/100

**Recommendation:** {{.ReleaseRecommendation}}

{{.Analysis.Summary}}

{{- if and .AppInterfaceMode (contains .ReleaseRecommendation "NOT RECOMMENDED")}}

**🔓 Override Justification Required** — If you proceed with this release despite this recommendation, post a comment in this merge request using `/rcs override <your justification>`. This creates an audit trail and helps improve the tool.

{{- end}}

---

{{- if .TruncationInfo}}

<details>
<summary><strong>✂️ Diff Truncation Applied</strong></summary>

Due to the large size of the code changes, diff truncation was applied to fit within the LLM context window:

- **Truncation Level**: {{.TruncationInfo.Level}}
- **Files Fully Analyzed**: {{.TruncationInfo.FilesPreserved}}/{{.TruncationInfo.TotalFiles}}
- **Files Partially Truncated**: {{.TruncationInfo.FilesTruncated}}

**What was preserved:**
- All file metadata (names, change statistics)
- Complete patches for critical files (database, security, auth, API contracts)
- Complete patches for high-risk files (infrastructure, deployment, config)
- Beginning and end sections of truncated patches

**What was truncated:**
- Middle sections of low-risk files (primarily tests and documentation)
{{- if eq .TruncationInfo.Level "aggressive"}}
- Middle sections of medium-risk files (dependencies, lock files)
{{- end}}

The LLM was informed about the truncation and used file metadata, preserved critical code, and partial context to perform the risk analysis.

</details>

---
{{- end}}

## 🔍 Risk Analysis

{{- if .Analysis.RiskSummary.Concerns}}

### Concerns

***Severity:** 🔥 Critical · ⚠️ High · 🟡 Medium · 🟢 Low*

| | Details |
|----------|---------|
{{- range .Analysis.RiskSummary.Concerns}}
{{- if eq .Severity "critical"}}
| 🔥 | {{.Description}} |
{{- else if eq .Severity "high"}}
| ⚠️ | {{.Description}} |
{{- else if eq .Severity "medium"}}
| 🟡 | {{.Description}} |
{{- else}}
| 🟢 | {{.Description}} |
{{- end}}
{{- end}}
{{- end}}

{{- if .Analysis.RiskSummary.Positives}}

### Positive Factors
{{- range .Analysis.RiskSummary.Positives}}
- {{.}}
{{- end}}
{{- end}}

---

## 📋 Action Items

{{- if .Analysis.ActionItems.Critical}}

### 🔥 Critical (Complete Before Release)
{{- range .Analysis.ActionItems.Critical}}
- {{.}}
{{- end}}
{{- end}}

{{- if .Analysis.ActionItems.Important}}

### ⚠️ Important (Recommended Before Release)
{{- range .Analysis.ActionItems.Important}}
- {{.}}
{{- end}}
{{- end}}

{{- if .Analysis.ActionItems.Followup}}

### 📝 Follow-up (Post-Release)
{{- range .Analysis.ActionItems.Followup}}
- {{.}}
{{- end}}
{{- end}}

{{- if .AllUserGuidance}}

---

## 📝 User Guidance

The following user guidance was provided in GitLab MR and GitHub PR discussions:

| Guidance | Author | Date | Status | Comment |
|----------|--------|------|--------|---------|
{{- range .AllUserGuidance}}
| {{.Content}} | {{formatAuthor .Author .CommentURL}} | {{formatDate .Date}} | {{authorizationStatus .IsAuthorized}} | [View]({{.CommentURL}}) |
{{- end}}

**Note:** Only authorized `/rcs note` guidance is used in the LLM analysis. For GitHub PRs, this includes guidance from PR authors and meaningful approvers. For GitLab MRs, all guidance is considered authorized. Unauthorized guidance is listed here for transparency but is ignored during scoring.
{{- end}}

---

<details>
<summary><strong>🛠️ Technical Details</strong></summary>

{{- if .Analysis.TechnicalDetails.Code}}

### 📝 Code Changes
{{- range .Analysis.TechnicalDetails.Code}}
- {{.}}
{{- end}}
{{- end}}

{{- if .Analysis.TechnicalDetails.Infrastructure}}

### 🏗️ Infrastructure Changes
{{- range .Analysis.TechnicalDetails.Infrastructure}}
- {{.}}
{{- end}}
{{- end}}

{{- if .Analysis.TechnicalDetails.Dependencies}}

### 🔗 Dependency Changes
{{- range .Analysis.TechnicalDetails.Dependencies}}
- {{.}}
{{- end}}
{{- end}}

</details>

---

<details>
<summary><strong>📋 Changelogs</strong></summary>

{{- if .Comparisons}}
{{- range $i, $comparison := .Comparisons}}
{{- if gt $i 0}}

{{- end}}

### [{{$comparison.RepoURL}}]({{$comparison.DiffURL}})

{{- if $comparison.Commits}}
*Total commits: {{len $comparison.Commits}}*

| SHA | Message | Author | PR | QE Status |
|-----|---------|--------|----|-----------|
{{- range $comparison.Commits}}
| {{commitLink .ShortSHA .SHA $comparison.RepoURL}} | {{escapePipes .Message}} | {{escapePipes .Author}} | {{prLink .PRNumber $comparison.RepoURL}} | {{qeStatus .QETestingLabel}} |
{{- end}}
{{- else}}
*No commits found in this comparison.*
{{- end}}
{{- end}}
{{- else}}
No repository changelog data available.
{{- end}}

</details>

---

<details>
<summary><strong>📚 Release Documentation</strong></summary>

### 📁 Documentation Sources Analyzed
{{- if .Documentation}}
{{- range .Documentation}}
{{- $doc := .}}
{{docFileInfo .MainDocFile .Repository.URL .Repository.DefaultBranch .MainDocContent}}
{{- range $filename := .AdditionalDocsOrder}}
{{- if index $doc.AdditionalDocsContent $filename}}
{{docFileInfo $filename $doc.Repository.URL $doc.Repository.DefaultBranch (index $doc.AdditionalDocsContent $filename)}}
{{- end}}
{{- end}}
{{- if .FailedAdditionalDocs}}

**Failed to fetch the following additional documentation:**
{{- range $displayName, $errorMsg := .FailedAdditionalDocs}}
- **{{$displayName}}**: {{$errorMsg}}
{{- end}}
{{- end}}
{{- end}}
{{- else}}
No repository documentation was found or analyzed.
{{- end}}

### 🔍 Overall Assessment
{{.Analysis.DocumentationQuality}}

### 💡 Improvement Recommendations
{{.Analysis.DocumentationRecommendations}}

</details>

---

<details>
<summary><strong>📈 Want Better Analysis Results?</strong></summary>

Learn how to improve your confidence scores and get more accurate analysis:

👉 **[Guide: Improving Your Release Confidence Analysis](https://github.com/RedHatInsights/release-confidence-score/blob/main/docs/IMPROVING_ANALYSIS.md)**

**Quick tips:**
- Add `.release-confidence-docs.md` to your repository for context-aware analysis
- Use `/rcs note` comments to provide context the AI can't infer from code
- Keep PRs/MRs focused and reasonably sized for better analysis quality
- Apply `rcs/qe-tested` or `rcs/needs-qe-testing` labels

</details>

---

{{- if .FeedbackURL}}

💬 **[Share your feedback on this report]({{.FeedbackURL}})** — Were the risks accurate? Were the action items useful? Your feedback helps improve RCS.

{{- end}}

*🤖 Generated by [Release Confidence Score](https://github.com/RedHatInsights/release-confidence-score) | {{.Metadata.ModelID}} | {{.Metadata.GenerationTime.Format "2006-01-02 15:04:05 UTC"}}*
