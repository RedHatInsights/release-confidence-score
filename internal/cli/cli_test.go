package cli

import (
	"flag"
	"os"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
		errMsg  string
		check   func(*testing.T, *Args)
	}{
		{
			name:    "help flag should not trigger validation",
			args:    []string{"cmd", "--help"},
			wantErr: false,
			check: func(t *testing.T, args *Args) {
				if !args.ShowHelp {
					t.Error("ShowHelp should be true")
				}
			},
		},
		{
			name:    "help shorthand should not trigger validation",
			args:    []string{"cmd", "-h"},
			wantErr: false,
			check: func(t *testing.T, args *Args) {
				if !args.ShowHelp {
					t.Error("ShowHelp should be true")
				}
			},
		},
		{
			name:    "help flag with invalid args should still work",
			args:    []string{"cmd", "--help", "--mode", "invalid"},
			wantErr: false,
			check: func(t *testing.T, args *Args) {
				if !args.ShowHelp {
					t.Error("ShowHelp should be true")
				}
			},
		},
		{
			name:    "valid standalone mode",
			args:    []string{"cmd", "-c", "https://github.com/org/repo/compare/v1...v2"},
			wantErr: false,
			check: func(t *testing.T, args *Args) {
				if args.Mode != "standalone" {
					t.Errorf("Mode = %v, expected standalone", args.Mode)
				}
				if len(args.CompareLinks) != 1 {
					t.Errorf("len(CompareLinks) = %v, expected 1", len(args.CompareLinks))
				}
			},
		},
		{
			name:    "valid app-interface mode",
			args:    []string{"cmd", "-m", "app-interface", "-mr", "12345"},
			wantErr: false,
			check: func(t *testing.T, args *Args) {
				if args.Mode != "app-interface" {
					t.Errorf("Mode = %v, expected app-interface", args.Mode)
				}
				if args.MergeRequestIID != 12345 {
					t.Errorf("MergeRequestIID = %v, expected 12345", args.MergeRequestIID)
				}
			},
		},
		{
			name:    "standalone mode missing compare links should fail",
			args:    []string{"cmd"},
			wantErr: true,
			errMsg:  "standalone mode requires compare URLs",
		},
		{
			name:    "app-interface mode missing MR IID should fail",
			args:    []string{"cmd", "-m", "app-interface"},
			wantErr: true,
			errMsg:  "app-interface mode requires --merge-request-iid",
		},
		{
			name:    "post-to-mr in standalone mode should fail",
			args:    []string{"cmd", "-c", "https://github.com/org/repo/compare/v1...v2", "-p"},
			wantErr: true,
			errMsg:  "--post-to-mr is only available in app-interface mode",
		},
		{
			name:    "multiple compare links",
			args:    []string{"cmd", "-c", "https://github.com/org/repo1/compare/v1...v2,https://github.com/org/repo2/compare/v1...v2"},
			wantErr: false,
			check: func(t *testing.T, args *Args) {
				if len(args.CompareLinks) != 2 {
					t.Errorf("len(CompareLinks) = %v, expected 2", len(args.CompareLinks))
				}
			},
		},
		{
			name:    "compare links with whitespace should be trimmed",
			args:    []string{"cmd", "-c", "https://url1.com , https://url2.com"},
			wantErr: false,
			check: func(t *testing.T, args *Args) {
				if len(args.CompareLinks) != 2 {
					t.Errorf("len(CompareLinks) = %v, expected 2", len(args.CompareLinks))
				}
				if args.CompareLinks[0] != "https://url1.com" {
					t.Errorf("CompareLinks[0] = %v, expected trimmed URL", args.CompareLinks[0])
				}
				if args.CompareLinks[1] != "https://url2.com" {
					t.Errorf("CompareLinks[1] = %v, expected trimmed URL", args.CompareLinks[1])
				}
			},
		},
		{
			name:    "mode inferred from compare links without explicit mode flag",
			args:    []string{"cmd", "-c", "https://github.com/org/repo/compare/v1...v2"},
			wantErr: false,
			check: func(t *testing.T, args *Args) {
				if args.Mode != "standalone" {
					t.Errorf("Mode = %v, expected standalone (inferred from compare links)", args.Mode)
				}
				if len(args.CompareLinks) != 1 {
					t.Errorf("len(CompareLinks) = %v, expected 1", len(args.CompareLinks))
				}
			},
		},
		{
			name:    "mode inferred from merge request IID without explicit mode flag",
			args:    []string{"cmd", "-mr", "12345"},
			wantErr: false,
			check: func(t *testing.T, args *Args) {
				if args.Mode != "app-interface" {
					t.Errorf("Mode = %v, expected app-interface (inferred from MR IID)", args.Mode)
				}
				if args.MergeRequestIID != 12345 {
					t.Errorf("MergeRequestIID = %v, expected 12345", args.MergeRequestIID)
				}
			},
		},
		{
			name:    "invalid mode value should fail",
			args:    []string{"cmd", "--mode", "invalid-mode"},
			wantErr: true,
			errMsg:  "invalid mode 'invalid-mode': must be 'standalone' or 'app-interface'",
		},
		{
			name:    "invalid mode value with shorthand should fail",
			args:    []string{"cmd", "-m", "bad"},
			wantErr: true,
			errMsg:  "invalid mode 'bad': must be 'standalone' or 'app-interface'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flag.CommandLine for each test
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

			// Save and restore os.Args
			oldArgs := os.Args
			defer func() { os.Args = oldArgs }()
			os.Args = tt.args

			args, err := Parse()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Parse() expected error but got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Parse() error = %v, expected to contain %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Parse() unexpected error = %v", err)
					return
				}
				if tt.check != nil {
					tt.check(t, args)
				}
			}
		})
	}
}

func TestDetermineMode(t *testing.T) {
	tests := []struct {
		name     string
		args     *Args
		expected string
	}{
		{
			name: "explicit mode app-interface",
			args: &Args{
				Mode: "app-interface",
			},
			expected: "app-interface",
		},
		{
			name: "explicit mode standalone",
			args: &Args{
				Mode: "standalone",
			},
			expected: "standalone",
		},
		{
			name: "infer app-interface from merge request IID",
			args: &Args{
				MergeRequestIID: 12345,
			},
			expected: "app-interface",
		},
		{
			name: "infer standalone from compare links",
			args: &Args{
				CompareLinks: []string{"https://github.com/org/repo/compare/v1...v2"},
			},
			expected: "standalone",
		},
		{
			name:     "default to standalone when no args",
			args:     &Args{},
			expected: "standalone",
		},
		{
			name: "explicit mode takes precedence over merge request IID",
			args: &Args{
				Mode:            "standalone",
				MergeRequestIID: 12345,
			},
			expected: "standalone",
		},
		{
			name: "explicit mode takes precedence over compare links",
			args: &Args{
				Mode:         "app-interface",
				CompareLinks: []string{"https://github.com/org/repo/compare/v1...v2"},
			},
			expected: "app-interface",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.args.determineMode()
			if result != tt.expected {
				t.Errorf("determineMode() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		args    *Args
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid app-interface mode",
			args: &Args{
				Mode:            "app-interface",
				MergeRequestIID: 12345,
			},
			wantErr: false,
		},
		{
			name: "valid standalone mode",
			args: &Args{
				Mode:         "standalone",
				CompareLinks: []string{"https://github.com/org/repo/compare/v1...v2"},
			},
			wantErr: false,
		},
		{
			name: "app-interface mode missing merge request IID",
			args: &Args{
				Mode:            "app-interface",
				MergeRequestIID: 0,
			},
			wantErr: true,
			errMsg:  "app-interface mode requires --merge-request-iid",
		},
		{
			name: "standalone mode missing compare links",
			args: &Args{
				Mode:         "standalone",
				CompareLinks: []string{},
			},
			wantErr: true,
			errMsg:  "standalone mode requires compare URLs",
		},
		{
			name: "standalone mode with nil compare links",
			args: &Args{
				Mode:         "standalone",
				CompareLinks: nil,
			},
			wantErr: true,
			errMsg:  "standalone mode requires compare URLs",
		},
		{
			name: "post-to-mr flag in app-interface mode is valid",
			args: &Args{
				Mode:            "app-interface",
				MergeRequestIID: 12345,
				PostToMR:        true,
			},
			wantErr: false,
		},
		{
			name: "post-to-mr flag in standalone mode is invalid",
			args: &Args{
				Mode:         "standalone",
				CompareLinks: []string{"https://github.com/org/repo/compare/v1...v2"},
				PostToMR:     true,
			},
			wantErr: true,
			errMsg:  "--post-to-mr is only available in app-interface mode",
		},
		{
			name: "valid standalone mode with multiple compare links",
			args: &Args{
				Mode: "standalone",
				CompareLinks: []string{
					"https://github.com/org/repo1/compare/v1...v2",
					"https://github.com/org/repo2/compare/v1...v2",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.args.validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("validate() expected error but got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("validate() error = %v, expected to contain %v", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("validate() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestShowUsage(t *testing.T) {
	// Test that ShowUsage doesn't panic
	// We can't easily test the output without redirecting stdout,
	// but we can at least ensure it runs without errors
	t.Run("ShowUsage does not panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("ShowUsage() panicked: %v", r)
			}
		}()
		ShowUsage()
	})
}

func TestArgsStruct(t *testing.T) {
	// Test that Args struct can be created and fields are accessible
	args := &Args{
		Mode:            "app-interface",
		CompareLinks:    []string{"https://example.com"},
		MergeRequestIID: 123,
		PostToMR:        true,
		ShowHelp:        false,
	}

	if args.Mode != "app-interface" {
		t.Errorf("Mode = %v, expected app-interface", args.Mode)
	}
	if len(args.CompareLinks) != 1 {
		t.Errorf("len(CompareLinks) = %v, expected 1", len(args.CompareLinks))
	}
	if args.MergeRequestIID != 123 {
		t.Errorf("MergeRequestIID = %v, expected 123", args.MergeRequestIID)
	}
	if !args.PostToMR {
		t.Errorf("PostToMR = %v, expected true", args.PostToMR)
	}
	if args.ShowHelp {
		t.Errorf("ShowHelp = %v, expected false", args.ShowHelp)
	}
}
