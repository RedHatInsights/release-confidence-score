package main

import (
	"fmt"
	"log"
	"os"

	"release-confidence-score/internal"
	"release-confidence-score/internal/cli"
	"release-confidence-score/internal/config"
	"release-confidence-score/internal/git/gitlab"
	"release-confidence-score/internal/logger"
)

func main() {
	// Parse command-line arguments
	args, err := cli.Parse()
	if err != nil {
		log.Fatalf("Failed to parse arguments: %v", err)
	}

	// Handle help flag
	if args.ShowHelp {
		cli.ShowUsage()
		os.Exit(0)
	}

	// Determine mode from arguments
	mode := determineMode(args)

	// Validate mode-specific arguments
	if err := validateArgs(mode, args); err != nil {
		log.Fatalf("%v", err)
	}

	// Load configuration from environment variables
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate mode-specific configuration
	if err := validateConfig(mode, cfg); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Setup logging
	logger.Setup(cfg)

	// Create release analyzer
	releaseAnalyzer, err := internal.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create release analyzer: %v", err)
	}

	// Run analysis based on mode
	var report string

	switch mode {
	case "app-interface":
		_, report, err = releaseAnalyzer.AnalyzeAppInterface(args.MergeRequestIID)
		if err != nil {
			log.Fatalf("Failed to run app-interface analysis: %v", err)
		}
	case "standalone":
		_, report, err = releaseAnalyzer.AnalyzeStandalone(args.CompareLinks)
		if err != nil {
			log.Fatalf("Failed to run standalone analysis: %v", err)
		}
	default:
		log.Fatalf("Invalid mode '%s': must be 'standalone' or 'app-interface'", mode)
	}

	// Output report
	if args.PostToMR {
		// Post to GitLab MR
		if err := postReportToMR(report, args.MergeRequestIID, cfg); err != nil {
			log.Fatalf("Failed to post report to MR: %v", err)
		}
	} else {
		// Print to stdout (user can redirect to file if needed)
		fmt.Print(report)
	}
}

func determineMode(args *cli.Args) string {
	// Explicit mode flag takes precedence
	if args.Mode != "" {
		return args.Mode
	}

	// Infer mode from provided arguments
	if args.MergeRequestIID > 0 {
		return "app-interface"
	}
	if len(args.CompareLinks) > 0 {
		return "standalone"
	}

	// Default to standalone
	return "standalone"
}

func validateArgs(mode string, args *cli.Args) error {
	switch mode {
	case "app-interface":
		if args.MergeRequestIID == 0 {
			return fmt.Errorf("app-interface mode requires --merge-request-iid\n\nTry:\n  rcs --mode app-interface --merge-request-iid <iid>\n\nOr run 'rcs --help' for more information")
		}
	case "standalone":
		if len(args.CompareLinks) == 0 {
			return fmt.Errorf("standalone mode requires compare URLs\n\nTry:\n  rcs --compare-links <url1>,<url2>\n\nOr run 'rcs --help' for more information")
		}
	}

	// Validate --post-to-mr is only used in app-interface mode
	if args.PostToMR && mode != "app-interface" {
		return fmt.Errorf("--post-to-mr is only available in app-interface mode")
	}

	return nil
}

func validateConfig(mode string, cfg *config.Config) error {
	if mode == "app-interface" {
		if cfg.GitLabBaseURL == "" {
			return fmt.Errorf("GITLAB_BASE_URL environment variable is required for app-interface mode")
		}
	}
	return nil
}

func postReportToMR(report string, mrIID int, cfg *config.Config) error {
	gitlabClient := gitlab.NewClient(cfg)
	projectID := "service/app-interface"

	if err := gitlab.PostMergeRequestComment(gitlabClient, projectID, mrIID, report); err != nil {
		return fmt.Errorf("failed to post comment to MR: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Report posted to merge request !%d\n", mrIID)
	return nil
}
