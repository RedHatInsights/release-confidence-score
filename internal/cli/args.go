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
	MergeRequestIID int
	ShowHelp        bool
}

// Parse parses command-line arguments
func Parse() (*Args, error) {
	args := &Args{}

	// Define flags with both long and short forms
	flag.StringVar(&args.Mode, "mode", "", "Operation mode: 'standalone' (default) or 'app-interface'")
	flag.StringVar(&args.Mode, "m", "", "Operation mode (shorthand)")

	compareLinksStr := flag.String("compare-links", "", "Comma-separated list of GitHub/GitLab compare URLs (standalone mode)")
	flag.StringVar(compareLinksStr, "c", "", "Comma-separated compare URLs (shorthand)")

	flag.IntVar(&args.MergeRequestIID, "merge-request-iid", 0, "App-interface merge request IID (app-interface mode)")
	flag.IntVar(&args.MergeRequestIID, "mr", 0, "Merge request IID (shorthand)")

	flag.BoolVar(&args.ShowHelp, "help", false, "Show help message")
	flag.BoolVar(&args.ShowHelp, "h", false, "Show help message (shorthand)")

	flag.Parse()

	// Parse comma-separated compare links
	if *compareLinksStr != "" {
		args.CompareLinks = strings.Split(*compareLinksStr, ",")
		// Trim whitespace from each link
		for i, link := range args.CompareLinks {
			args.CompareLinks[i] = strings.TrimSpace(link)
		}
	}

	return args, nil
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
  -h, --help                         Show this help message

EXAMPLES:
  # Standalone mode
  rcs --compare-links "https://github.com/org/repo/compare/v1.0...v1.1,https://github.com/org/api/compare/v2.0...v2.1"
  rcs -c "https://github.com/org/repo/compare/v1.0...v1.1"

  # App-interface mode
  rcs --mode app-interface --merge-request-iid 160191
  rcs -m app-interface -mr 160191

CONFIGURATION:
  All configuration is set via environment variables.
  See .env.example for all available options.

For more information, visit: https://github.com/RedHatInsights/release-confidence-score`)
}
