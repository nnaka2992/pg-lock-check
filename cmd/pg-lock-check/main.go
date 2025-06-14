package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/nnaka2992/pg-lock-check/internal/analyzer"
	"github.com/nnaka2992/pg-lock-check/internal/parser"
)

var (
	version = "0.1.0"
	
	// Flags
	fileFlag          string
	outputFormat      string
	noTransactionFlag bool
	noColorFlag       bool
	quietFlag         bool
	verboseFlag       bool
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	var exitCode int
	
	rootCmd := &cobra.Command{
		Use:     "pg-lock-check [SQL]",
		Short:   "PostgreSQL lock analyzer",
		Version: version,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := runAnalysis(cmd, args)
			if err != nil {
				// Set proper exit code based on error type
				if isParseError(err) {
					exitCode = 2
				} else {
					exitCode = 1
				}
			}
			return err
		},
		SilenceUsage: true,
	}
	
	// Add flags
	rootCmd.Flags().StringVarP(&fileFlag, "file", "f", "", "read SQL from file")
	rootCmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "output format: text, json, yaml")
	rootCmd.Flags().BoolVar(&noTransactionFlag, "no-transaction", false, "analyze without transaction wrapper")
	rootCmd.Flags().BoolVar(&noColorFlag, "no-color", false, "disable colored output")
	rootCmd.Flags().BoolVarP(&quietFlag, "quiet", "q", false, "quiet mode")
	rootCmd.Flags().BoolVar(&verboseFlag, "verbose", false, "verbose output")
	
	rootCmd.SetArgs(args)
	
	if err := rootCmd.Execute(); err != nil {
		return exitCode
	}
	
	return 0
}

func isParseError(err error) bool {
	return err != nil && (
		contains(err.Error(), "parse error") ||
		contains(err.Error(), "syntax error"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && strings.Contains(s, substr))
}

func runAnalysis(cmd *cobra.Command, args []string) error {
	var sql string
	
	// Get SQL input
	if fileFlag != "" {
		content, err := os.ReadFile(fileFlag)
		if err != nil {
			return fmt.Errorf("reading file: %w", err)
		}
		sql = string(content)
	} else if len(args) > 0 {
		sql = args[0]
	} else {
		// Check stdin
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			// Data is being piped
			content, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("reading stdin: %w", err)
			}
			sql = string(content)
		} else {
			// No input provided
			cmd.Usage()
			return fmt.Errorf("no SQL provided")
		}
	}
	
	// Parse
	p := parser.NewParser()
	parsed, err := p.ParseSQL(sql)
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}
	
	// Analyze
	a := analyzer.New()
	mode := analyzer.InTransaction
	if noTransactionFlag {
		mode = analyzer.NoTransaction
	}
	
	results, err := a.Analyze(parsed, mode)
	if err != nil {
		return fmt.Errorf("analysis error: %w", err)
	}
	
	// Output
	switch outputFormat {
	case "json":
		fmt.Println(`{"results": []}`) // TODO: Implement
	default:
		// Text output
		for i, result := range results {
			stmt := ""
			if i < len(parsed.Statements) {
				stmt = parsed.Statements[i].SQL
			}
			
			severity := getSeverityName(result.Severity)
			fmt.Printf("[%s] %s\n", severity, stmt)
		}
		
		// Summary
		fmt.Printf("\nSummary: %d statements analyzed\n", len(results))
	}
	
	return nil
}

func getSeverityName(s analyzer.Severity) string {
	switch s {
	case analyzer.SeverityError:
		return "ERROR"
	case analyzer.SeverityCritical:
		return "CRITICAL"
	case analyzer.SeverityWarning:
		return "WARNING"
	case analyzer.SeverityInfo:
		return "INFO"
	default:
		return "UNKNOWN"
	}
}