# Release Confidence Score

## Overview

**Stop guessing about releases. Get data-driven confidence scores and actionable risk assessments in seconds.**

Release Confidence Score analyzes code changes and generates objective confidence scores (0-100) with comprehensive risk reports.
Whether you're making manual release decisions or using automated gates in your continuous delivery pipeline, RCS provides consistent, AI-powered risk assessment to replace gut feelings with data.

### What You Get

- **Objective Confidence Scores**: Clear 0-100 ratings with consistent evaluation criteria across all releases.
- **Comprehensive Risk Analysis**: Detects critical risks including database migrations, authentication changes, API contracts, infrastructure modifications, and dependency updates.
- **Smart Context Integration**: Leverages repository documentation, QE test results, and user guidance comments for informed recommendations.
- **Actionable Reports**: Categorized action items (critical/important/follow-up) with specific release guidance and risk mitigation steps.

### Why Teams Choose RCS

- **Reduce Decision Stress**: Consistent, data-driven criteria replace subjective judgment calls for manual releases.
- **Enable Automated Gates**: Use confidence scores as quality gates in CI/CD pipelines for safe continuous delivery.
- **Save Time**: Automated analysis of diffs, changelogs, and documentation in one comprehensive report.
- **Prevent Production Issues**: Identify high-risk changes before they reach users.
- **Scale Your Expertise**: Apply seasoned release judgment consistently across all releases.

## Prerequisites

- API access to one of the supported AI providers (Claude, Gemini or Llama).
- GitHub and GitLab personal access tokens.

## Quick Start

### Using Docker Compose (Recommended)

1. **Clone the repository**
   ```bash
   git clone git@github.com:RedHatInsights/release-confidence-score.git
   cd release-confidence-score
   ```

2. **Configure environment**
   ```bash
   cp .env.example .env
   # Edit .env with your credentials
   ```

3. **Build and run**
   ```bash
   docker compose build
   docker compose run --rm release-confidence-score <app-interface-merge-request-iid>
   ```

   Example:
   ```bash
   docker compose run --rm release-confidence-score 160191
   ```

### Using Go Directly

1. **Clone the repository**
   ```bash
   git clone git@github.com:RedHatInsights/release-confidence-score.git
   cd release-confidence-score
   ```

2. **Set environment variables**
   ```bash
   export RCS_GITHUB_TOKEN="your_github_token"
   export RCS_GITLAB_BASE_URL="https://gitlab.cee.redhat.com/"
   export RCS_GITLAB_TOKEN="your_gitlab_token"
   export RCS_MODEL_PROVIDER="claude" # or gemini, llama
   export RCS_CLAUDE_MODEL_API="your_claude_api_endpoint"
   export RCS_CLAUDE_MODEL_ID="claude-sonnet-4@20250514"
   export RCS_CLAUDE_USER_KEY="your_claude_api_key"
   ```

3. **Run the application**
   ```bash
   go run main.go <app-interface-merge-request-iid>
   ```

   Example:
   ```bash
   go run main.go 160191
   ```

## Configuration

### Required Environment Variables

**GitHub Integration:**
- `RCS_GITHUB_TOKEN`: GitHub personal access token.

**GitLab Configuration:**
- `RCS_GITLAB_BASE_URL`: GitLab instance URL.
- `RCS_GITLAB_TOKEN`: GitLab personal access token.

**Provider-Specific Configuration:**

For Claude (default):
- `RCS_CLAUDE_MODEL_API`: Claude API endpoint.
- `RCS_CLAUDE_MODEL_ID`: Model identifier (e.g., `claude-sonnet-4@20250514`).
- `RCS_CLAUDE_USER_KEY`: Authentication key.

For Gemini:
- `RCS_GEMINI_MODEL_API`: Gemini API endpoint.
- `RCS_GEMINI_MODEL_ID`: Model identifier (e.g., `gemini-2.5-pro`).
- `RCS_GEMINI_USER_KEY`: Authentication key.

For Llama:
- `RCS_LLAMA_MODEL_API`: Llama API endpoint.
- `RCS_LLAMA_MODEL_ID`: Model identifier (e.g., `RedHatAI/Llama-3.3-70B-Instruct-FP8-dynamic`).
- `RCS_LLAMA_USER_KEY`: Authentication key.

### Optional Environment Variables

**AI Provider Selection:**
- `RCS_MODEL_PROVIDER`: Choose `claude` (default), `gemini`, or `llama`.

**Model Configuration:**
- `RCS_MODEL_SKIP_SSL_VERIFY`: Skip SSL verification for AI provider (default: false).
- `RCS_MODEL_MAX_RESPONSE_TOKENS`: Maximum tokens in AI response (default: 2000).
- `RCS_MODEL_TIMEOUT_SECONDS`: Request timeout in seconds (default: 120).

**GitLab Configuration:**
- `RCS_GITLAB_SKIP_SSL_VERIFY`: Skip SSL verification (default: false).

**Score Thresholds:**
- `RCS_SCORE_THRESHOLD_AUTO_DEPLOY`: Minimum score for auto-deployment recommendation (default: 80).
- `RCS_SCORE_THRESHOLD_REVIEW_REQUIRED`: Minimum score before manual review required (default: 60).

See `.env.example` for a complete configuration template.

## How RCS Works

1. **App-Interface Data Collection**: Fetches merge request details from GitLab app-interface repository, including diff URLs and user guidance from merge request comments.
2. **Repository Data Collection**: Retrieves commits, documentation, user guidance, and QE testing labels from GitHub and GitLab repositories being released.
3. **Data Processing**: Analyzes and formats collected data, builds changelogs from commits, processes QE testing labels to assess test coverage, and prepares consolidated context for AI analysis.
4. **AI Analysis**: Sends consolidated data with specialized system prompt to the configured AI provider for risk assessment.
5. **Report Generation**: Produces a detailed report with confidence score, risk factors, release recommendations, changelogs, user guidance (with author authorization status), diff truncation details when applicable, and tips for improving future analysis with better documentation.

![How RCS Works](./how-rcs-works.svg)

[Diagram Source](https://excalidraw.com/#json=rx2mPOLX3f5C61e068VE4,WDdT3hHbWEDJ1xDv2EBN5Q)

## Features

### Smart Diff Handling

Automatically handles large diffs that exceed AI context windows using progressive truncation:
- **First attempt**: Analyzes full diff content without any truncation.
- **Progressive retry**: If context window is exceeded, automatically retries with increasing truncation levels (moderate → aggressive → extreme → ultimate).
- **Risk-based preservation**: Prioritizes critical files (database migrations, security code, API contracts, infrastructure) while truncating low-risk files (tests, documentation, generated files).
- **Transparent reporting**: Reports truncation level and impact in the final analysis.

### Repository Documentation Integration

RCS automatically fetches `.release-confidence-docs.md` from repository roots to provide release context that improves AI analysis accuracy.
Links listed in the "Additional Documentation" section are also fetched and analyzed.
The AI uses this documentation to provide context-aware risk assessment tailored to your specific service.
