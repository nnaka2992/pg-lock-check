package parser

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// Constants for parser operations
const (
	// bomSize is the size of UTF-8 BOM in bytes
	bomSize = 3

	// initialLineNumber is the starting line number for SQL statements
	initialLineNumber = 1
)

// utf8BOM represents the UTF-8 byte order mark
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// ParsedStatement represents a single parsed SQL statement with its metadata
type ParsedStatement struct {
	// AST is the raw abstract syntax tree from pg_query_go
	AST *pg_query.ParseResult

	// SQL is the original SQL text for this statement
	SQL string

	// LineNumber is the line number where this statement starts (1-based)
	LineNumber int
}

// ParseResult represents the result of parsing SQL content
type ParseResult struct {
	// Statements contains all successfully parsed SQL statements in order
	Statements []ParsedStatement
}

// Parser interface defines the contract for SQL parsing operations
type Parser interface {
	// ParseSQL parses a SQL string and returns parsed statements
	ParseSQL(sql string) (*ParseResult, error)

	// ParseFile reads and parses SQL from a file
	ParseFile(filepath string) (*ParseResult, error)

	// ParseFiles reads and parses multiple SQL files
	ParseFiles(filepaths []string) (*ParseResult, error)
}

// parser implements the Parser interface
type parser struct{}

// NewParser creates a new parser instance
func NewParser() Parser {
	return &parser{}
}

// ParseSQL parses SQL string and returns parsed statements
func (p *parser) ParseSQL(sql string) (*ParseResult, error) {
	if sql == "" {
		return emptyParseResult(), nil
	}

	// Clean the SQL input
	sql = cleanSQL(sql)

	// Split SQL into individual statements
	statements, err := pg_query.SplitWithScanner(sql, true)
	if err != nil {
		return nil, fmt.Errorf("failed to split SQL statements: %w", err)
	}

	if len(statements) == 0 {
		return emptyParseResult(), nil
	}

	return p.parseStatements(sql, statements)
}

// ParseFile reads and parses SQL from a file
func (p *parser) ParseFile(filepath string) (*ParseResult, error) {
	if filepath == "" {
		return nil, fmt.Errorf("filepath cannot be empty")
	}

	content, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %q: %w", filepath, err)
	}

	return p.ParseSQL(string(content))
}

// ParseFiles reads and parses multiple SQL files
func (p *parser) ParseFiles(filepaths []string) (*ParseResult, error) {
	if len(filepaths) == 0 {
		return emptyParseResult(), nil
	}

	// Pre-calculate capacity to avoid multiple allocations
	estimatedCapacity := len(filepaths) * 10 // Estimate 10 statements per file
	allStatements := make([]ParsedStatement, 0, estimatedCapacity)

	for _, filepath := range filepaths {
		result, err := p.ParseFile(filepath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse file %q: %w", filepath, err)
		}
		allStatements = append(allStatements, result.Statements...)
	}

	return &ParseResult{Statements: allStatements}, nil
}

// parseStatements processes individual SQL statements and creates ParsedStatement objects
func (p *parser) parseStatements(originalSQL string, statements []string) (*ParseResult, error) {
	result := &ParseResult{
		Statements: make([]ParsedStatement, 0, len(statements)),
	}

	offset := 0
	for i, stmtSQL := range statements {
		// Find where this statement appears in the original SQL
		idx := strings.Index(originalSQL[offset:], stmtSQL)
		if idx == -1 {
			// This should rarely happen, but handle it gracefully
			continue
		}

		stmtStart := offset + idx
		lineNum := calculateLineNumber(originalSQL, stmtStart)

		// Parse individual statement to get its AST
		ast, err := pg_query.Parse(stmtSQL)
		if err != nil {
			return nil, fmt.Errorf("parse error at line %d, statement %d: %w", lineNum, i+1, err)
		}

		result.Statements = append(result.Statements, ParsedStatement{
			AST:        ast,
			SQL:        stmtSQL,
			LineNumber: lineNum,
		})

		// Move offset forward for next search
		offset = stmtStart + len(stmtSQL)
	}

	return result, nil
}

// cleanSQL removes BOM and normalizes the SQL string
func cleanSQL(sql string) string {
	return string(stripBOM([]byte(sql)))
}

// emptyParseResult returns an empty ParseResult
func emptyParseResult() *ParseResult {
	return &ParseResult{Statements: []ParsedStatement{}}
}

// calculateLineNumber calculates the line number for a given position in the SQL string
func calculateLineNumber(sql string, position int) int {
	if position == 0 {
		return initialLineNumber
	}

	lineNumber := initialLineNumber
	for i := 0; i < position && i < len(sql); i++ {
		if sql[i] == '\n' {
			lineNumber++
		}
	}
	return lineNumber
}

// stripBOM removes the UTF-8 BOM if present
func stripBOM(content []byte) []byte {
	if len(content) >= bomSize && bytes.HasPrefix(content, utf8BOM) {
		return content[bomSize:]
	}
	return content
}
