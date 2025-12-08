package main

import (
	"fmt"
	"log"
	"os"

	"release-confidence-score/internal"
	"release-confidence-score/internal/cli"
	"release-confidence-score/internal/config"
	"release-confidence-score/internal/logger"
)

func main() {
	// Parse command-line arguments
	args, err := cli.Parse()
	if err != nil {
		log.Fatalf("Failed to parse CLI arguments: %v", err)
	}

	// Handle help flag
	if args.ShowHelp {
		cli.ShowUsage()
		os.Exit(0) // Early return if help flag is used
	}

	// Load and validate configuration from environment variables
	isAppInterfaceMode := args.Mode == "app-interface"
	cfg, err := config.Load(isAppInterfaceMode)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
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

	switch args.Mode {
	case "app-interface":
		_, report, err = releaseAnalyzer.AnalyzeAppInterface(args.MergeRequestIID, args.PostToMR)
		if err != nil {
			log.Fatalf("Failed to run app-interface analysis: %v", err)
		}
	case "standalone":
		_, report, err = releaseAnalyzer.AnalyzeStandalone(args.CompareLinks)
		if err != nil {
			log.Fatalf("Failed to run standalone analysis: %v", err)
		}
	default:
		// This should never happen as cli.Parse() validates mode
		log.Fatalf("Invalid mode '%s': must be 'standalone' or 'app-interface'", args.Mode)
	}

	// Print report to stdout regardless of mode or post-to-mr flag
	fmt.Print(report)
}
