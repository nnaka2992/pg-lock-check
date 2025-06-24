package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/nnaka2992/pg-lock-check/internal/analyzer"
	"github.com/nnaka2992/pg-lock-check/internal/metadata"
	"github.com/nnaka2992/pg-lock-check/internal/parser"
	"github.com/nnaka2992/pg-lock-check/internal/suggester"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// CLI configuration
var (
	version = "0.1.2"

	// Flags
	fileFlag          string
	outputFormat      string
	noTransactionFlag bool
	noColorFlag       bool
	quietFlag         bool
	verboseFlag       bool
	noSuggestionFlag  bool
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	cmd := buildCommand()
	cmd.SetArgs(args)

	var exitCode int
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		err := runAnalysis(cmd, args)
		if err != nil {
			exitCode = determineExitCode(err)
		}
		return err
	}

	if err := cmd.Execute(); err != nil {
		if exitCode == 0 {
			return 1 // Default error code for flag parsing errors
		}
		return exitCode
	}

	return 0
}

func buildCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "pg-lock-check [SQL]",
		Short:        "PostgreSQL lock analyzer",
		Version:      version,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
	}

	// Add flags
	cmd.Flags().StringVarP(&fileFlag, "file", "f", "", "read SQL from file")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "text", "output format: text, json, yaml")
	cmd.Flags().BoolVar(&noTransactionFlag, "no-transaction", false, "analyze without transaction wrapper")
	cmd.Flags().BoolVar(&noColorFlag, "no-color", false, "disable colored output")
	cmd.Flags().BoolVarP(&quietFlag, "quiet", "q", false, "quiet mode")
	cmd.Flags().BoolVar(&verboseFlag, "verbose", false, "verbose output")
	cmd.Flags().BoolVar(&noSuggestionFlag, "no-suggestion", false, "disable safe migration suggestions")

	return cmd
}

func runAnalysis(cmd *cobra.Command, args []string) error {
	// Get SQL input
	sql, err := getSQLInput(cmd, args)
	if err != nil {
		return err
	}

	// Parse SQL
	p := parser.NewParser()
	parsed, err := p.ParseSQL(sql)
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	// Analyze
	mode := analyzer.InTransaction
	if noTransactionFlag {
		mode = analyzer.NoTransaction
	}

	a := analyzer.New()
	results, err := a.Analyze(parsed, mode)
	if err != nil {
		return fmt.Errorf("analysis error: %w", err)
	}

	// Create suggester if enabled
	var s suggester.Suggester
	if !noSuggestionFlag {
		s = suggester.NewSuggester()
	}

	// Output results
	return outputResults(parsed, results, s)
}

// getSQLInput retrieves SQL from command args, file, or stdin
func getSQLInput(cmd *cobra.Command, args []string) (string, error) {
	// Priority: file flag > command args > stdin
	if fileFlag != "" {
		content, err := os.ReadFile(fileFlag)
		if err != nil {
			return "", fmt.Errorf("reading file: %w", err)
		}
		return string(content), nil
	}

	if len(args) > 0 {
		return args[0], nil
	}

	// Check stdin
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Data is being piped
		content, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("reading stdin: %w", err)
		}
		return string(content), nil
	}

	// No input provided
	_ = cmd.Usage()
	return "", fmt.Errorf("no SQL provided")
}

// outputResults handles different output formats
func outputResults(parsed *parser.ParseResult, results []*analyzer.Result, s suggester.Suggester) error {
	switch outputFormat {
	case "json":
		return outputJSON(parsed, results, s)
	case "yaml":
		return outputYAML(parsed, results, s)
	default:
		return outputText(parsed, results, s)
	}
}

// outputText formats results as human-readable text
func outputText(parsed *parser.ParseResult, results []*analyzer.Result, s suggester.Suggester) error {
	for i, result := range results {
		// Get statement SQL
		stmt := ""
		if i < len(parsed.Statements) {
			stmt = parsed.Statements[i].SQL
		}

		// Print severity and statement
		severity := getSeverityName(result.Severity)
		fmt.Printf("[%s] %s\n", severity, stmt)

		// Show suggestions for CRITICAL operations
		if shouldShowSuggestion(result, s) {
			showSuggestion(parsed, i, result, s)
		}
	}

	// Summary
	fmt.Printf("\nSummary: %d statements analyzed\n", len(results))
	return nil
}

// shouldShowSuggestion checks if we should display a suggestion
func shouldShowSuggestion(result *analyzer.Result, s suggester.Suggester) bool {
	return result.Severity == analyzer.SeverityCritical &&
		s != nil &&
		s.HasSuggestion(result.Operation())
}

// showSuggestion displays a suggestion for a critical operation
func showSuggestion(parsed *parser.ParseResult, index int, result *analyzer.Result, s suggester.Suggester) {
	if index >= len(parsed.Statements) || len(parsed.Statements[index].AST.Stmts) == 0 {
		return
	}

	// Extract metadata
	extractor := metadata.NewExtractor()
	metadata := extractor.Extract(parsed.Statements[index].AST.Stmts[0].Stmt, result.Operation())

	// Get and display suggestion
	suggestion, err := s.GetSuggestion(result.Operation(), suggester.OperationMetadata(metadata))
	if err != nil {
		return
	}

	fmt.Println("Suggestion for safe migration:")
	for _, step := range suggestion.Steps {
		fmt.Printf("  Step: %s\n", step.Description)

		// Show transaction capability
		if step.CanRunInTransaction {
			fmt.Println("    Can run in transaction: Yes")
		} else {
			fmt.Println("    Can run in transaction: No")
		}

		// Show step details
		if step.SQL != "" {
			printIndented("    SQL:", step.SQL)
		} else if step.Command != "" {
			printIndented("    Command:", step.Command)
		} else if step.Notes != "" {
			printIndented("    Instructions:", step.Notes)
		}
	}
}

// outputJSON formats results as JSON
func outputJSON(parsed *parser.ParseResult, results []*analyzer.Result, s suggester.Suggester) error {
	output := buildOutput(parsed, results, s)
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
	}
	return nil
}

// outputYAML formats results as YAML
func outputYAML(parsed *parser.ParseResult, results []*analyzer.Result, s suggester.Suggester) error {
	output := buildOutput(parsed, results, s)
	encoder := yaml.NewEncoder(os.Stdout)
	encoder.SetIndent(2)
	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("encoding YAML: %w", err)
	}
	return nil
}

// buildOutput creates the structured output for JSON/YAML formats
func buildOutput(parsed *parser.ParseResult, results []*analyzer.Result, s suggester.Suggester) Output {
	// Initialize severity counts
	severityCounts := map[string]int{
		"ERROR":    0,
		"CRITICAL": 0,
		"WARNING":  0,
		"INFO":     0,
	}

	// Build results
	outputResults := make([]OutputResult, len(results))
	for i, result := range results {
		outputResults[i] = buildOutputResult(i, result, parsed, s, severityCounts)
	}

	return Output{
		Summary: OutputSummary{
			TotalStatements: len(results),
			BySeverity:      severityCounts,
		},
		Results: outputResults,
	}
}

// buildOutputResult creates a single output result
func buildOutputResult(index int, result *analyzer.Result, parsed *parser.ParseResult, s suggester.Suggester, severityCounts map[string]int) OutputResult {
	severityName := getSeverityName(result.Severity)
	severityCounts[severityName]++

	// Get SQL and line number
	sql := ""
	lineNumber := 1
	if index < len(parsed.Statements) {
		sql = parsed.Statements[index].SQL
		lineNumber = parsed.Statements[index].LineNumber
	}

	// Build table locks
	tables := buildTableLocks(result.TableLocks())

	// Handle empty lock type for ERROR severity
	lockType := string(result.LockType())
	if result.Severity == analyzer.SeverityError {
		lockType = ""
	}

	outputResult := OutputResult{
		Index:      index,
		SQL:        sql,
		LineNumber: lineNumber,
		Severity:   severityName,
		Operation:  result.Operation(),
		LockType:   lockType,
		Tables:     tables,
	}

	// Add suggestion if applicable
	if shouldShowSuggestion(result, s) && index < len(parsed.Statements) && len(parsed.Statements[index].AST.Stmts) > 0 {
		extractor := metadata.NewExtractor()
		metadata := extractor.Extract(parsed.Statements[index].AST.Stmts[0].Stmt, result.Operation())
		if suggestion, err := s.GetSuggestion(result.Operation(), suggester.OperationMetadata(metadata)); err == nil {
			outputResult.Suggestion = convertSuggestion(suggestion)
		}
	}

	return outputResult
}

// buildTableLocks parses table lock strings into structured format
func buildTableLocks(tableLocks []string) []TableLock {
	tables := []TableLock{}
	for _, tableLock := range tableLocks {
		// Parse table lock format "table_name:lock_type"
		parts := strings.Split(tableLock, ":")
		if len(parts) == 2 {
			tables = append(tables, TableLock{
				Name:     strings.TrimSpace(parts[0]),
				LockType: strings.TrimSpace(parts[1]),
			})
		}
	}
	return tables
}

// Helper functions

func determineExitCode(err error) int {
	if isParseError(err) {
		return 2
	}
	return 1
}

func isParseError(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "parse error") ||
		strings.Contains(err.Error(), "syntax error"))
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

func convertSuggestion(suggestion *suggester.Suggestion) *OutputSuggestion {
	if suggestion == nil {
		return nil
	}

	outputSuggestion := &OutputSuggestion{
		Steps: make([]OutputStep, 0, len(suggestion.Steps)),
	}

	for _, step := range suggestion.Steps {
		outputStep := OutputStep{
			Description:         step.Description,
			CanRunInTransaction: step.CanRunInTransaction,
		}

		// Combine SQL, Command, and Notes into Output field
		if step.SQL != "" {
			outputStep.Output = step.SQL
		} else if step.Command != "" {
			outputStep.Output = step.Command
		} else if step.Notes != "" {
			outputStep.Output = step.Notes
		}

		outputSuggestion.Steps = append(outputSuggestion.Steps, outputStep)
	}

	return outputSuggestion
}

// printIndented prints a header followed by content with proper indentation
func printIndented(header, content string) {
	fmt.Println(header)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if line != "" {
			fmt.Printf("      %s\n", line)
		}
	}
}

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
	Index      int               `json:"index" yaml:"index"`
	SQL        string            `json:"sql" yaml:"sql"`
	LineNumber int               `json:"line_number" yaml:"line_number"`
	Severity   string            `json:"severity" yaml:"severity"`
	Operation  string            `json:"operation" yaml:"operation"`
	LockType   string            `json:"lock_type" yaml:"lock_type"`
	Tables     []TableLock       `json:"tables" yaml:"tables"`
	Suggestion *OutputSuggestion `json:"suggestion,omitempty" yaml:"suggestion,omitempty"`
}

type OutputSuggestion struct {
	Steps []OutputStep `json:"steps" yaml:"steps"`
}

type OutputStep struct {
	Description         string `json:"description" yaml:"description"`
	CanRunInTransaction bool   `json:"can_run_in_transaction" yaml:"can_run_in_transaction"`
	Output              string `json:"output" yaml:"output"`
}

type TableLock struct {
	Name     string `json:"name" yaml:"name"`
	LockType string `json:"lock_type" yaml:"lock_type"`
}
