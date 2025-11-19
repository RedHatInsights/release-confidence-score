package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"release-confidence-score/internal"
	"release-confidence-score/internal/logger"
)

func main() {
	logger.Setup()

	if len(os.Args) < 2 {
		log.Fatal("Usage: go run main.go <merge-request-iid>")
	}

	mergeRequestIID, err := strconv.Atoi(os.Args[1])
	if err != nil {
		log.Fatalf("Invalid merge request IID '%s': %v", os.Args[1], err)
	}

	releaseAnalyzer, err := internal.New()
	if err != nil {
		log.Fatalf("Failed to create release analyzer: %v", err)
	}

	_, report, err := releaseAnalyzer.Analyze(mergeRequestIID)
	if err != nil {
		log.Fatalf("Failed to run release analysis: %v", err)
	}

	fmt.Printf("Report:\n%s", report)
}
