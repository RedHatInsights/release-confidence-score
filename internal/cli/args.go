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
	PostToMR        bool
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

	flag.BoolVar(&args.PostToMR, "post-to-mr", false, "Post report as comment to merge request (app-interface mode only)")
	flag.BoolVar(&args.PostToMR, "p", false, "Post to MR (shorthand)")

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
