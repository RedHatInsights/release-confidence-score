package shared

// TruncationMetadata contains information about diff truncation
type TruncationMetadata struct {
	Truncated          bool
	Level              string   // "moderate" or "aggressive"
	FilesPreserved     int      // Number of files with full patches
	FilesTruncated     int      // Number of files with truncated patches
	TruncatedFilesList []string // Names of truncated files
	TotalFiles         int      // Total number of files
}
