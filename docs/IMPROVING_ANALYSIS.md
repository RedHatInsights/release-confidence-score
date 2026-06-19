# Improving Your Release Confidence Analysis

This guide explains how to get more accurate confidence scores and better analysis from the Release Confidence Score tool.

## Quick Wins

### 1. Add Repository Documentation

**Impact: High** | **Effort: Medium** | **One-time setup**

Create a `.release-confidence-docs.md` file in your repository root. This gives the AI context about your service, its criticality, dependencies, and known risk areas.

```markdown
# Release Documentation

## Service Overview
**Service Criticality:** High
**Description:** User authentication service handling 10M requests/day
**SLA:** 99.9% uptime, <200ms P95 response time

## Deployment Risks & Considerations
### High-Risk Changes
1. **Database migrations** - Require 15-minute maintenance window
2. **OAuth configuration** - Affects all user logins

## Rollback Procedures
1. Revert to previous container tag
2. Run database down migration if needed

## Additional Documentation
- [Runbook](./docs/runbook.md)
- [Architecture](./docs/architecture.md)
```

**Why this helps:**
- AI understands your service's criticality level
- Analysis considers your specific risk areas
- Recommendations are tailored to your architecture

See [`.release-confidence-docs.example.md`](../.release-confidence-docs.example.md) for a comprehensive template.

---

### 2. Use User Guidance Comments

**Impact: High** | **Effort: Low** | **Per-PR/MR**

Add `/rcs note` comments in your pull request or merge request to provide context the AI can't infer from code alone:

```
/rcs note This change updates the rate limiting logic. The new limits have been
load tested and approved by the platform team. No database changes required.
```

**Effective guidance includes:**
- Why changes are safe despite appearing risky
- Context about testing that was performed
- Dependencies or sequencing requirements
- Business context the AI can't see

**Authorization:** Only guidance from PR/MR authors and approvers is used in the analysis. Other comments are shown in the report but not factored into scoring.

---

### 3. Apply QE Testing Labels

**Impact: Medium** | **Effort: Low** | **Per-PR/MR**

Apply labels to indicate QE testing status:
- `rcs/qe-tested` - Changes verified by QE team
- `rcs/needs-qe-testing` - Changes require QE verification

These labels work on both GitHub PRs and GitLab MRs.

**Why this helps:**
- AI factors testing coverage into confidence scoring
- Untested critical changes are flagged appropriately
- Report shows QE status for each commit

---

### 4. Write Clear Commit Messages

**Impact: Medium** | **Effort: Low** | **Per-commit**

Use descriptive commit messages that explain intent, not just action:

**Less helpful:**
```
fix bug
update config
```

**More helpful:**
```
fix: prevent session leak when user logs out during request

The previous implementation didn't cancel in-flight requests,
leaving orphaned sessions in Redis.
```

**Why this helps:**
- AI understands the purpose and impact of changes
- Risk assessment is more accurate
- Related changes are easier to identify

---

## Optimizing Your Workflow

### 5. Keep Changes Focused

**Impact: Medium** | **Effort: Medium**

Separate high-risk and low-risk changes:

**High-risk (separate PRs/MRs):**
- Database schema changes
- Authentication/authorization
- API contract modifications
- Infrastructure configuration
- Security-related code

**Low-risk (can combine):**
- Documentation updates
- Test additions
- Code formatting
- Comment improvements

**Why this helps:**
- Easier to analyze and understand each change's impact
- More accurate confidence scores
- Simpler rollback if issues arise

---

### 6. Manage Diff Size

**Impact: Low-Medium** | **Effort: Medium**

The tool handles large diffs through intelligent truncation, but smaller diffs receive more thorough analysis:

| Size | Analysis Quality |
|------|------------------|
| < 500 lines | Full analysis, no truncation |
| 500-1,500 lines | Full analysis in most cases |
| 1,500-5,000 lines | May trigger light truncation |
| > 5,000 lines | Progressive truncation applied |

**Truncation preserves:**
- All file metadata and statistics
- Complete patches for critical files (database, auth, security, API)
- Complete patches for infrastructure files
- Start and end of truncated patches

**Tips for large changes:**
- Break features into incremental PRs/MRs
- Separate refactoring from functional changes
- Use feature flags for gradual rollout

---

### 7. Link Related Documentation

**Impact: Low-Medium** | **Effort: Low**

In your `.release-confidence-docs.md`, add links in the "Additional Documentation" section:

```markdown
## Additional Documentation
- [Runbook](./docs/runbook.md)
- [API Reference](./docs/api.md)
- [Architecture Overview](https://wiki.example.com/my-service)
```

**Suggested priority when choosing what to link:**
1. Runbooks - Deployment and operational procedures
2. Monitoring guides - Health checks and alerting
3. Architecture docs - System design and dependencies
4. API documentation - Contract and integration details

The tool fetches all links equally - this is just guidance on what tends to be most useful for risk analysis. Only links in this section are fetched; links elsewhere are for human reference only.

---

## Understanding Your Score

### Score Interpretation

Default thresholds (configurable via environment variables):

| Score | Recommendation |
|-------|----------------|
| 80-100 | Recommended for release |
| 60-79 | Manual review required |
| 0-59 | Release not recommended |

### Factors That Improve Scores

- Comprehensive repository documentation
- QE testing completed (`rcs/qe-tested` label)
- Small, focused changes
- Clear commit messages
- User guidance providing context
- Changes to low-risk files (tests, docs)
- Documented rollback procedures

### Factors That Lower Scores

- Missing or sparse documentation
- High-risk changes (database, auth, API contracts)
- Large, complex diffs
- Multiple unrelated changes in one PR/MR
- No QE testing on critical paths
- Unclear purpose or impact
- Infrastructure changes without context

### Risk Categories

The tool classifies files by risk level for truncation decisions:

**Critical (never truncated):**
- Database: migrations, schema files, SQL
- Security: auth, tokens, credentials, permissions
- APIs: OpenAPI specs, protobuf, GraphQL schemas

**High (preserved in most modes):**
- Infrastructure: Dockerfile, Terraform, Kubernetes
- Configuration: CI/CD pipelines, build configs
- Deployment: Helm charts, Ansible playbooks

**Medium (truncated in aggressive modes):**
- Dependencies: package.json, go.mod, requirements.txt
- Lock files: package-lock.json, go.sum

**Low (truncated first):**
- Tests: *_test.go, *.spec.js, test_*.py
- Documentation: *.md, docs/
- IDE/tooling: .vscode/, .editorconfig

---

## Common Scenarios

### "My score is lower than expected"

**Checklist:**
- [ ] Did you add `/rcs note` guidance explaining the changes?
- [ ] Is repository documentation present and current?
- [ ] Are commit messages descriptive?
- [ ] Did you apply QE testing labels?
- [ ] Is the PR/MR focused on one concern?

**Quick fix:** Add a `/rcs note` comment explaining why changes are safe.

---

### "The AI doesn't understand my service"

**Solution:** Create or improve `.release-confidence-docs.md`

Include:
- Service criticality and SLA
- Known risk areas specific to your service
- Dependencies and integration points
- Rollback procedures
- Historical issues to watch for

---

### "Low-risk changes are flagged as risky"

**Common causes:**
1. File names match high-risk patterns (e.g., `config` in the path)
2. Missing context about what the files do
3. Mixed with higher-risk changes

**Solutions:**
- Add `/rcs note` guidance: `/rcs note These config files are test fixtures, not production configuration`
- Separate low-risk changes into their own PR/MR
- Update repository documentation to clarify

---

### "Large refactoring PR/MR"

**Best practices:**
1. Add detailed `/rcs note` guidance explaining the scope
2. Confirm behavior is unchanged: `/rcs note Pure refactoring, no behavior changes. All tests pass.`
3. Reference test results or QE validation
4. Consider breaking into smaller incremental changes

---

## Best Practices Summary

### Before Submitting

- [ ] Repository has `.release-confidence-docs.md`
- [ ] Added `/rcs note` guidance with relevant context
- [ ] Commit messages explain "why" not just "what"
- [ ] Applied QE testing labels if applicable
- [ ] PR/MR is focused on a single concern
- [ ] Documentation updated to reflect changes

### After Receiving Report

- [ ] Review all action items, especially "Critical"
- [ ] Understand identified risk factors
- [ ] Address documentation recommendations
- [ ] Add clarifying `/rcs note` guidance if AI missed context
- [ ] Re-run analysis if significant context was added
- [ ] If proceeding despite a "Not Recommended" result, post an `/rcs override` comment in the MR with your justification (see below)

---

## Overriding a "Not Recommended" Recommendation

> **Note:** This section applies to **app-interface mode** only, where the report is posted to a GitLab merge request. In standalone mode, there is no associated MR/PR to post the justification to.

When RCS produces a **"Release Not Recommended"** result and you decide to proceed anyway, post a comment in the merge request using `/rcs override <your justification>`.

**Examples:**

```
/rcs override The database migration was load-tested on a production-sized staging environment.
Index creation completed in under 60 seconds with no lock contention. Team is on standby for rollback.
```

```
/rcs override All critical action items addressed — deployment split into two phases per the report
recommendation. The remaining concerns are informational only and will be monitored post-release.
```

**What makes a good justification:**
- Explains why the specific concerns raised are acceptable or already mitigated
- References testing, staging results, or on-call coverage if relevant
- Notes any action items that were resolved before proceeding

**Why this matters:** The justification is recorded in the PR/MR thread, creating an audit trail. It also surfaces false positives that can be used to improve the tool — if RCS flagged something that turned out to be safe, reporting it helps calibrate future analyses.

To report a false positive or false negative, open an issue at the [project repository](https://github.com/RedHatInsights/release-confidence-score/issues).

---

## Configuration Reference

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `RCS_SCORE_THRESHOLD_AUTO_DEPLOY` | 80 | Score for "recommended" status |
| `RCS_SCORE_THRESHOLD_REVIEW_REQUIRED` | 60 | Score below which review is required |
| `RCS_SYSTEM_PROMPT_VERSION` | v1 | System prompt version (v1, v2) |

### QE Labels

| Label | Meaning |
|-------|---------|
| `rcs/qe-tested` | Changes verified by QE |
| `rcs/needs-qe-testing` | Changes require QE verification |

---

## Getting Help

### The Analysis Seems Wrong

1. **Add context via `/rcs note`** - The AI works with available information
2. **Update documentation** - Ensure `.release-confidence-docs.md` reflects reality
3. **Check truncation** - Large diffs may have relevant code truncated
4. **Review action items** - Sometimes the AI catches real issues

### Contributing

Report issues or suggestions at the [project repository](https://github.com/RedHatInsights/release-confidence-score).

---

**Remember:** The tool provides guidance to support decision-making, not replace it. Use it to surface risks and structure your analysis, combined with your team's judgment and domain expertise.
