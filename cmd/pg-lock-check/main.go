package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/nnaka2992/pg-lock-check/internal/analyzer"
	"github.com/nnaka2992/pg-lock-check/internal/parser"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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

// Output structures for JSON/YAML
type Output struct {
	Summary OutputSummary  `json:"summary" yaml:"summary"`
	Results []OutputResult `json:"results" yaml:"results"`
}

type OutputSummary struct {
	TotalStatements int            `json:"total_statements" yaml:"total_statements"`
	BySeverity      map[string]int `json:"by_severity" yaml:"by_severity"`
}

type OutputResult struct {
	Index      int         `json:"index" yaml:"index"`
	SQL        string      `json:"sql" yaml:"sql"`
	LineNumber int         `json:"line_number" yaml:"line_number"`
	Severity   string      `json:"severity" yaml:"severity"`
	Operation  string      `json:"operation" yaml:"operation"`
	LockType   string      `json:"lock_type" yaml:"lock_type"`
	Tables     []TableLock `json:"tables" yaml:"tables"`
}

type TableLock struct {
	Name     string `json:"name" yaml:"name"`
	LockType string `json:"lock_type" yaml:"lock_type"`
}

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
	return err != nil && (contains(err.Error(), "parse error") ||
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
			_ = cmd.Usage()
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
		output := buildOutput(parsed, results)
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(output); err != nil {
			return fmt.Errorf("encoding JSON: %w", err)
		}
	case "yaml":
		output := buildOutput(parsed, results)
		encoder := yaml.NewEncoder(os.Stdout)
		encoder.SetIndent(2)
		if err := encoder.Encode(output); err != nil {
			return fmt.Errorf("encoding YAML: %w", err)
		}
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

func buildOutput(parsed *parser.ParseResult, results []*analyzer.Result) Output {
	// Initialize severity counts
	severityCounts := map[string]int{
		"ERROR":    0,
		"CRITICAL": 0,
		"WARNING":  0,
		"INFO":     0,
	}

	// Build results and count severities
	outputResults := make([]OutputResult, len(results))
	for i, result := range results {
		severityName := getSeverityName(result.Severity)
		severityCounts[severityName]++

		// Get SQL statement and line number
		sql := ""
		lineNumber := 1
		if i < len(parsed.Statements) {
			sql = parsed.Statements[i].SQL
			lineNumber = parsed.Statements[i].LineNumber
		}

		// Build tables array
		tables := []TableLock{}
		for _, tableLock := range result.TableLocks() {
			// Parse table lock format "table_name:lock_type"
			parts := strings.Split(tableLock, ":")
			if len(parts) == 2 {
				tables = append(tables, TableLock{
					Name:     strings.TrimSpace(parts[0]),
					LockType: strings.TrimSpace(parts[1]),
				})
			}
		}

		// Handle empty lock type for ERROR severity
		lockType := string(result.LockType())
		if result.Severity == analyzer.SeverityError {
			lockType = ""
		}

		outputResults[i] = OutputResult{
			Index:      i,
			SQL:        sql,
			LineNumber: lineNumber,
			Severity:   severityName,
			Operation:  result.Operation(),
			LockType:   lockType,
			Tables:     tables,
		}
	}

	return Output{
		Summary: OutputSummary{
			TotalStatements: len(results),
			BySeverity:      severityCounts,
		},
		Results: outputResults,
	}
}
