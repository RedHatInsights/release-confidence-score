package cli

import (
	"flag"
	"fmt"
	"strings"
)

// Args holds the parsed command-line arguments
type Args struct {
	Mode            string
	CompareLinks    []string
	MergeRequestIID int64
	PostToMR        bool
	ShowHelp        bool
}

// Parse parses command-line arguments
func Parse() (*Args, error) {
	args := &Args{}

	// Define flags with both long and short forms
	flag.StringVar(&args.Mode, "mode", "", "Operation mode: 'standalone' (default) or 'app-interface'")
	flag.StringVar(&args.Mode, "m", "", "Operation mode (shorthand)")

	var compareLinksStr string
	flag.StringVar(&compareLinksStr, "compare-links", "", "Comma-separated list of GitHub/GitLab compare URLs (standalone mode)")
	flag.StringVar(&compareLinksStr, "c", "", "Comma-separated compare URLs (shorthand)")

	flag.Int64Var(&args.MergeRequestIID, "merge-request-iid", 0, "App-interface merge request IID (app-interface mode)")
	flag.Int64Var(&args.MergeRequestIID, "mr", 0, "Merge request IID (shorthand)")

	flag.BoolVar(&args.PostToMR, "post-to-mr", false, "Post report as comment to merge request (app-interface mode only)")
	flag.BoolVar(&args.PostToMR, "p", false, "Post to MR (shorthand)")

	flag.BoolVar(&args.ShowHelp, "help", false, "Show help message")
	flag.BoolVar(&args.ShowHelp, "h", false, "Show help message (shorthand)")

	flag.Parse()

	// Check for help flag early - no need to validate if user just wants help
	if args.ShowHelp {
		return args, nil
	}

	// Parse comma-separated compare links
	if compareLinksStr != "" {
		args.CompareLinks = strings.Split(compareLinksStr, ",")
		// Trim whitespace from each link
		for i, link := range args.CompareLinks {
			args.CompareLinks[i] = strings.TrimSpace(link)
		}
	}

	// Determine mode (infer if not explicitly set)
	args.Mode = args.determineMode()

	// Validate arguments
	if err := args.validate(); err != nil {
		return nil, err
	}

	return args, nil
}

// determineMode determines the operation mode from arguments
func (a *Args) determineMode() string {
	// Explicit mode flag takes precedence
	if a.Mode != "" {
		return a.Mode
	}

	// Infer mode from provided arguments
	if a.MergeRequestIID > 0 {
		return "app-interface"
	}
	if len(a.CompareLinks) > 0 {
		return "standalone"
	}

	// Default to standalone
	return "standalone"
}

// Validate validates the parsed arguments based on the determined mode
func (a *Args) validate() error {
	// Validate mode value first
	if a.Mode != "standalone" && a.Mode != "app-interface" {
		return fmt.Errorf("invalid mode '%s': must be 'standalone' or 'app-interface'", a.Mode)
	}

	switch a.Mode {
	case "app-interface":
		if a.MergeRequestIID == 0 {
			return fmt.Errorf("app-interface mode requires --merge-request-iid\n\nTry:\n  rcs --mode app-interface --merge-request-iid <iid>\n\nOr run 'rcs --help' for more information")
		}
	case "standalone":
		if len(a.CompareLinks) == 0 {
			return fmt.Errorf("standalone mode requires compare URLs\n\nTry:\n  rcs --compare-links <url1>,<url2>\n\nOr run 'rcs --help' for more information")
		}
	}

	// Validate --post-to-mr is only used in app-interface mode
	if a.PostToMR && a.Mode != "app-interface" {
		return fmt.Errorf("--post-to-mr is only available in app-interface mode")
	}

	return nil
}

// ShowUsage displays usage information
func ShowUsage() {
	fmt.Println(`Release Confidence Score - AI-powered release risk assessment

USAGE:
  Standalone mode (default):
    rcs --compare-links <url1>,<url2>,...

  App-interface mode:
    rcs --mode app-interface --merge-request-iid <iid>

FLAGS:
  -m, --mode <mode>                  Operation mode: 'standalone' or 'app-interface'
  -c, --compare-links <urls>         Comma-separated compare URLs (standalone)
  -mr, --merge-request-iid <iid>     Merge request IID (app-interface)
  -p, --post-to-mr                   Post report to merge request (app-interface only)
  -h, --help                         Show this help message

EXAMPLES:
  # Standalone mode - print to stdout
  rcs -c "https://github.com/org/repo/compare/v1.0...v1.1"

  # Standalone mode - save to file
  rcs -c "https://github.com/org/repo/compare/v1.0...v1.1" > report.md

  # App-interface mode - print to stdout
  rcs -m app-interface -mr 160191

  # App-interface mode - save to file
  rcs -m app-interface -mr 160191 > report.md

  # App-interface mode - post to MR
  rcs -m app-interface -mr 160191 -p

CONFIGURATION:
  All configuration is set via environment variables.
  See .env.example for all available options.

For more information, visit: https://github.com/RedHatInsights/release-confidence-score`)
}
