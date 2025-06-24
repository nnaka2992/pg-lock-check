package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"sort"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestCase represents a single test case with expected values
type OutputTestCase struct {
	name string
	sql  string
	args []string // Additional args like --no-transaction

	// Expected values (not full JSON/YAML)
	expectTotalStatements int
	expectResults         []ExpectedResult
}

type ExpectedResult struct {
	index      int    // Expected index in results array (0-based)
	lineNumber int    // Expected line number in input (1-based)
	sql        string // Expected SQL statement
	severity   string
	operation  string
	lockType   string
	tables     []ExpectedTable
}

type ExpectedTable struct {
	name     string
	lockType string
}

var allTestCases = []OutputTestCase{
	// Basic DML Operations
	{
		name:                  "SELECT with table",
		sql:                   "SELECT * FROM users",
		expectTotalStatements: 1,
		expectResults: []ExpectedResult{
			{
				index:      0,
				lineNumber: 1,
				sql:        "SELECT * FROM users",
				severity:   "INFO",
				operation:  "SELECT",
				lockType:   "AccessShare",
				tables: []ExpectedTable{
					{name: "users", lockType: "AccessShare"},
				},
			},
		},
	},
	{
		name:                  "INSERT",
		sql:                   "INSERT INTO users (name) VALUES ('test')",
		expectTotalStatements: 1,
		expectResults: []ExpectedResult{
			{
				index:      0,
				lineNumber: 1,
				sql:        "INSERT INTO users (name) VALUES ('test')",
				severity:   "INFO",
				operation:  "INSERT",
				lockType:   "RowExclusive",
				tables: []ExpectedTable{
					{name: "users", lockType: "RowExclusive"},
				},
			},
		},
	},
	{
		name:                  "UPDATE without WHERE",
		sql:                   "UPDATE users SET active = false",
		expectTotalStatements: 1,
		expectResults: []ExpectedResult{
			{
				index:      0,
				lineNumber: 1,
				sql:        "UPDATE users SET active = false",
				severity:   "CRITICAL",
				operation:  "UPDATE without WHERE",
				lockType:   "RowExclusive",
				tables: []ExpectedTable{
					{name: "users", lockType: "RowExclusive"},
				},
			},
		},
	},
	{
		name:                  "UPDATE with WHERE",
		sql:                   "UPDATE users SET active = false WHERE id = 1",
		expectTotalStatements: 1,
		expectResults: []ExpectedResult{
			{
				index:      0,
				lineNumber: 1,
				sql:        "UPDATE users SET active = false WHERE id = 1",
				severity:   "WARNING",
				operation:  "UPDATE with WHERE",
				lockType:   "RowExclusive",
				tables: []ExpectedTable{
					{name: "users", lockType: "RowExclusive"},
				},
			},
		},
	},
	{
		name:                  "DELETE without WHERE",
		sql:                   "DELETE FROM users",
		expectTotalStatements: 1,
		expectResults: []ExpectedResult{
			{
				index:      0,
				lineNumber: 1,
				sql:        "DELETE FROM users",
				severity:   "CRITICAL",
				operation:  "DELETE without WHERE",
				lockType:   "RowExclusive",
				tables: []ExpectedTable{
					{name: "users", lockType: "RowExclusive"},
				},
			},
		},
	},
	{
		name:                  "DELETE with WHERE",
		sql:                   "DELETE FROM users WHERE id = 1",
		expectTotalStatements: 1,
		expectResults: []ExpectedResult{
			{
				index:      0,
				lineNumber: 1,
				sql:        "DELETE FROM users WHERE id = 1",
				severity:   "WARNING",
				operation:  "DELETE with WHERE",
				lockType:   "RowExclusive",
				tables: []ExpectedTable{
					{name: "users", lockType: "RowExclusive"},
				},
			},
		},
	},

	// Common DDL Operations
	{
		name:                  "CREATE TABLE",
		sql:                   "CREATE TABLE users (id INT)",
		expectTotalStatements: 1,
		expectResults: []ExpectedResult{
			{
				index:      0,
				lineNumber: 1,
				sql:        "CREATE TABLE users (id INT)",
				severity:   "INFO",
				operation:  "CREATE TABLE",
				lockType:   "AccessExclusive",
				tables:     []ExpectedTable{}, // No existing tables locked
			},
		},
	},
	{
		name:                  "DROP TABLE",
		sql:                   "DROP TABLE users",
		expectTotalStatements: 1,
		expectResults: []ExpectedResult{
			{
				index:      0,
				lineNumber: 1,
				sql:        "DROP TABLE users",
				severity:   "CRITICAL",
				operation:  "DROP TABLE",
				lockType:   "AccessExclusive",
				tables: []ExpectedTable{
					{name: "users", lockType: "AccessExclusive"},
				},
			},
		},
	},
	{
		name:                  "ALTER TABLE ADD COLUMN",
		sql:                   "ALTER TABLE users ADD COLUMN email TEXT",
		expectTotalStatements: 1,
		expectResults: []ExpectedResult{
			{
				index:      0,
				lineNumber: 1,
				sql:        "ALTER TABLE users ADD COLUMN email TEXT",
				severity:   "INFO",
				operation:  "ALTER TABLE ADD COLUMN without DEFAULT",
				lockType:   "AccessExclusive",
				tables: []ExpectedTable{
					{name: "users", lockType: "AccessExclusive"},
				},
			},
		},
	},
	{
		name:                  "CREATE INDEX",
		sql:                   "CREATE INDEX idx_users_email ON users(email)",
		expectTotalStatements: 1,
		expectResults: []ExpectedResult{
			{
				index:      0,
				lineNumber: 1,
				sql:        "CREATE INDEX idx_users_email ON users(email)",
				severity:   "CRITICAL",
				operation:  "CREATE INDEX",
				lockType:   "Share",
				tables: []ExpectedTable{
					{name: "users", lockType: "Share"},
				},
			},
		},
	},
	{
		name:                  "TRUNCATE",
		sql:                   "TRUNCATE users",
		expectTotalStatements: 1,
		expectResults: []ExpectedResult{
			{
				index:      0,
				lineNumber: 1,
				sql:        "TRUNCATE users",
				severity:   "CRITICAL",
				operation:  "TRUNCATE",
				lockType:   "AccessExclusive",
				tables: []ExpectedTable{
					{name: "users", lockType: "AccessExclusive"},
				},
			},
		},
	},

	// Transaction mode differences
	{
		name:                  "VACUUM in transaction mode",
		sql:                   "VACUUM",
		expectTotalStatements: 1,
		expectResults: []ExpectedResult{
			{
				index:      0,
				lineNumber: 1,
				sql:        "VACUUM",
				severity:   "ERROR",
				operation:  "VACUUM",
				lockType:   "", // ERROR operations have no lock
				tables:     []ExpectedTable{},
			},
		},
	},
	{
		name:                  "VACUUM in no-transaction mode",
		sql:                   "VACUUM",
		args:                  []string{"--no-transaction"},
		expectTotalStatements: 1,
		expectResults: []ExpectedResult{
			{
				index:      0,
				lineNumber: 1,
				sql:        "VACUUM",
				severity:   "WARNING",
				operation:  "VACUUM",
				lockType:   "ShareUpdateExclusive",
				tables:     []ExpectedTable{},
			},
		},
	},
	{
		name:                  "CREATE INDEX CONCURRENTLY in transaction mode",
		sql:                   "CREATE INDEX CONCURRENTLY idx ON users(email)",
		expectTotalStatements: 1,
		expectResults: []ExpectedResult{
			{
				index:      0,
				lineNumber: 1,
				sql:        "CREATE INDEX CONCURRENTLY idx ON users(email)",
				severity:   "ERROR",
				operation:  "CREATE INDEX CONCURRENTLY",
				lockType:   "",
				tables: []ExpectedTable{
					{name: "users", lockType: "ShareUpdateExclusive"},
				},
			},
		},
	},

	// Edge cases
	{
		name:                  "SELECT without table",
		sql:                   "SELECT 1",
		expectTotalStatements: 1,
		expectResults: []ExpectedResult{
			{
				index:      0,
				lineNumber: 1,
				sql:        "SELECT 1",
				severity:   "INFO",
				operation:  "SELECT",
				lockType:   "AccessShare",
				tables:     []ExpectedTable{}, // No tables
			},
		},
	},

	// Multi-statement test cases
	{
		name: "Multiple statements with different severities",
		sql: `SELECT * FROM users;
UPDATE users SET last_login = NOW() WHERE id = 1;
DELETE FROM old_sessions WHERE created < '2023-01-01';`,
		expectTotalStatements: 3,
		expectResults: []ExpectedResult{
			{
				index:      0,
				lineNumber: 1,
				sql:        "SELECT * FROM users",
				severity:   "INFO",
				operation:  "SELECT",
				lockType:   "AccessShare",
				tables: []ExpectedTable{
					{name: "users", lockType: "AccessShare"},
				},
			},
			{
				index:      1,
				lineNumber: 2,
				sql:        "UPDATE users SET last_login = NOW() WHERE id = 1",
				severity:   "WARNING",
				operation:  "UPDATE with WHERE",
				lockType:   "RowExclusive",
				tables: []ExpectedTable{
					{name: "users", lockType: "RowExclusive"},
				},
			},
			{
				index:      2,
				lineNumber: 3,
				sql:        "DELETE FROM old_sessions WHERE created < '2023-01-01'",
				severity:   "WARNING",
				operation:  "DELETE with WHERE",
				lockType:   "RowExclusive",
				tables: []ExpectedTable{
					{name: "old_sessions", lockType: "RowExclusive"},
				},
			},
		},
	},
	{
		name: "Multi-line formatted SQL statements",
		sql: `-- First: Complex SELECT with JOIN
SELECT 
    u.id,
    u.name,
    COUNT(o.id) as order_count
FROM users u
LEFT JOIN orders o ON u.id = o.user_id
WHERE u.active = true
GROUP BY u.id, u.name;

-- Second: Multi-line UPDATE
UPDATE products
SET 
    price = price * 1.1,
    updated_at = NOW()
WHERE 
    category = 'electronics'
    AND last_updated < '2023-01-01';

-- Third: CREATE TABLE with multiple columns
CREATE TABLE audit_logs (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    action VARCHAR(50) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    metadata JSONB
);`,
		expectTotalStatements: 3,
		expectResults: []ExpectedResult{
			{
				index:      0,
				lineNumber: 1,
				sql: `-- First: Complex SELECT with JOIN
SELECT 
    u.id,
    u.name,
    COUNT(o.id) as order_count
FROM users u
LEFT JOIN orders o ON u.id = o.user_id
WHERE u.active = true
GROUP BY u.id, u.name`,
				severity:  "INFO",
				operation: "SELECT",
				lockType:  "AccessShare",
				tables: []ExpectedTable{
					{name: "users", lockType: "AccessShare"},
					{name: "orders", lockType: "AccessShare"},
				},
			},
			{
				index:      1,
				lineNumber: 11,
				sql: `-- Second: Multi-line UPDATE
UPDATE products
SET 
    price = price * 1.1,
    updated_at = NOW()
WHERE 
    category = 'electronics'
    AND last_updated < '2023-01-01'`,
				severity:  "WARNING",
				operation: "UPDATE with WHERE",
				lockType:  "RowExclusive",
				tables: []ExpectedTable{
					{name: "products", lockType: "RowExclusive"},
				},
			},
			{
				index:      2,
				lineNumber: 20,
				sql: `-- Third: CREATE TABLE with multiple columns
CREATE TABLE audit_logs (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    action VARCHAR(50) NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    metadata JSONB
)`,
				severity:  "INFO",
				operation: "CREATE TABLE",
				lockType:  "AccessExclusive",
				tables:    []ExpectedTable{}, // No existing tables locked
			},
		},
	},
	{
		name: "Statements with embedded comments and empty lines",
		sql: `-- Start of migration script

SELECT version FROM schema_info;

-- This UPDATE is critical!
UPDATE users 
SET status = 'inactive'
-- Be careful with this one
WHERE last_login < CURRENT_DATE - INTERVAL '90 days';

-- Clean up old data

DELETE FROM sessions;  -- This will lock everything!`,
		expectTotalStatements: 3,
		expectResults: []ExpectedResult{
			{
				index:      0,
				lineNumber: 1,
				sql:        "-- Start of migration script\n\nSELECT version FROM schema_info",
				severity:   "INFO",
				operation:  "SELECT",
				lockType:   "AccessShare",
				tables: []ExpectedTable{
					{name: "schema_info", lockType: "AccessShare"},
				},
			},
			{
				index:      1,
				lineNumber: 5,
				sql: `-- This UPDATE is critical!
UPDATE users 
SET status = 'inactive'
-- Be careful with this one
WHERE last_login < CURRENT_DATE - INTERVAL '90 days'`,
				severity:  "WARNING",
				operation: "UPDATE with WHERE",
				lockType:  "RowExclusive",
				tables: []ExpectedTable{
					{name: "users", lockType: "RowExclusive"},
				},
			},
			{
				index:      2,
				lineNumber: 11,
				sql:        "-- Clean up old data\n\nDELETE FROM sessions",
				severity:   "CRITICAL",
				operation:  "DELETE without WHERE",
				lockType:   "RowExclusive",
				tables: []ExpectedTable{
					{name: "sessions", lockType: "RowExclusive"},
				},
			},
		},
	},
}

// TestJSONOutputFormat tests that JSON output is valid and contains correct data
func TestJSONOutputFormat(t *testing.T) {
	for _, tc := range allTestCases {
		t.Run(tc.name, func(t *testing.T) {
			// Build args with -o json
			args := []string{"-o", "json"}
			args = append(args, tc.args...)

			// Always use stdin for SQL input (more realistic for complex SQL)
			output, exitCode := runCommand(t, args, tc.sql)
			if exitCode != 0 {
				t.Fatalf("Command failed with exit code %d: %s", exitCode, output)
			}

			// Verify it's valid JSON
			var result map[string]interface{}
			if err := json.Unmarshal([]byte(output), &result); err != nil {
				t.Fatalf("Output is not valid JSON: %v\nOutput: %s", err, output)
			}

			// Check basic structure
			summary, ok := result["summary"].(map[string]interface{})
			if !ok {
				t.Fatal("Missing or invalid 'summary' field")
			}

			results, ok := result["results"].([]interface{})
			if !ok {
				t.Fatal("Missing or invalid 'results' field")
			}

			// Verify summary
			totalStatements := int(summary["total_statements"].(float64))
			if totalStatements != tc.expectTotalStatements {
				t.Errorf("Expected %d statements, got %d", tc.expectTotalStatements, totalStatements)
			}

			// Verify each result
			if len(results) != len(tc.expectResults) {
				t.Fatalf("Expected %d results, got %d", len(tc.expectResults), len(results))
			}

			for i, expected := range tc.expectResults {
				result := results[i].(map[string]interface{})

				// Check index
				if idx := int(result["index"].(float64)); idx != expected.index {
					t.Errorf("Result %d: expected index %d, got %d", i, expected.index, idx)
				}

				// Check line number
				if lineNum := int(result["line_number"].(float64)); lineNum != expected.lineNumber {
					t.Errorf("Result %d: expected line_number %d, got %d", i, expected.lineNumber, lineNum)
				}

				// Check SQL
				if sql := result["sql"].(string); sql != expected.sql {
					t.Errorf("Result %d: expected sql %q, got %q", i, expected.sql, sql)
				}

				// Check severity
				if severity := result["severity"].(string); severity != expected.severity {
					t.Errorf("Result %d: expected severity %s, got %s", i, expected.severity, severity)
				}

				// Check operation
				if operation := result["operation"].(string); operation != expected.operation {
					t.Errorf("Result %d: expected operation %s, got %s", i, expected.operation, operation)
				}

				// Check lock type
				if lockType := result["lock_type"].(string); lockType != expected.lockType {
					t.Errorf("Result %d: expected lock_type %s, got %s", i, expected.lockType, lockType)
				}

				// Check tables
				tables := result["tables"].([]interface{})
				if len(tables) != len(expected.tables) {
					t.Errorf("Result %d: expected %d tables, got %d", i, len(expected.tables), len(tables))
					continue
				}

				// Sort tables by name for consistent comparison
				sort.Slice(tables, func(i, j int) bool {
					return tables[i].(map[string]interface{})["name"].(string) <
						tables[j].(map[string]interface{})["name"].(string)
				})

				// Sort expected tables too
				sortedExpected := make([]ExpectedTable, len(expected.tables))
				copy(sortedExpected, expected.tables)
				sort.Slice(sortedExpected, func(i, j int) bool {
					return sortedExpected[i].name < sortedExpected[j].name
				})

				for j, expectedTable := range sortedExpected {
					table := tables[j].(map[string]interface{})
					if name := table["name"].(string); name != expectedTable.name {
						t.Errorf("Result %d, table %d: expected name %s, got %s", i, j, expectedTable.name, name)
					}
					if lockType := table["lock_type"].(string); lockType != expectedTable.lockType {
						t.Errorf("Result %d, table %d: expected lock_type %s, got %s", i, j, expectedTable.lockType, lockType)
					}
				}
			}

			// Verify severity counts
			bySeverity := summary["by_severity"].(map[string]interface{})
			expectedCounts := map[string]int{
				"ERROR":    0,
				"CRITICAL": 0,
				"WARNING":  0,
				"INFO":     0,
			}
			for _, result := range tc.expectResults {
				expectedCounts[result.severity]++
			}

			for severity, expectedCount := range expectedCounts {
				if count := int(bySeverity[severity].(float64)); count != expectedCount {
					t.Errorf("Expected %d %s results, got %d", expectedCount, severity, count)
				}
			}
		})
	}
}

// TestYAMLOutputFormat tests that YAML output is valid and contains correct data
func TestYAMLOutputFormat(t *testing.T) {
	for _, tc := range allTestCases {
		t.Run(tc.name, func(t *testing.T) {
			// Build args with -o yaml
			args := []string{"-o", "yaml"}
			args = append(args, tc.args...)

			// Always use stdin for SQL input (more realistic for complex SQL)
			output, exitCode := runCommand(t, args, tc.sql)
			if exitCode != 0 {
				t.Fatalf("Command failed with exit code %d: %s", exitCode, output)
			}

			// Verify it's valid YAML
			var result map[string]interface{}
			if err := yaml.Unmarshal([]byte(output), &result); err != nil {
				t.Fatalf("Output is not valid YAML: %v\nOutput: %s", err, output)
			}

			// Basic structure checks (similar to JSON but less detailed)
			summary, ok := result["summary"].(map[string]interface{})
			if !ok {
				t.Fatal("Missing or invalid 'summary' field in YAML")
			}

			results, ok := result["results"].([]interface{})
			if !ok {
				t.Fatal("Missing or invalid 'results' field in YAML")
			}

			// Verify counts
			totalStatements := summary["total_statements"].(int)
			if totalStatements != tc.expectTotalStatements {
				t.Errorf("Expected %d statements, got %d", tc.expectTotalStatements, totalStatements)
			}

			if len(results) != len(tc.expectResults) {
				t.Errorf("Expected %d results, got %d", len(tc.expectResults), len(results))
			}
		})
	}
}

// Helper to run command and capture output
func runCommand(t *testing.T, args []string, stdin string) (string, int) {
	t.Helper()

	// Capture stdout and stderr
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	oldStdin := os.Stdin

	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	// Set up stdin if provided
	if stdin != "" {
		rIn, wIn, _ := os.Pipe()
		os.Stdin = rIn
		go func() {
			defer func() { _ = wIn.Close() }()
			_, _ = wIn.WriteString(stdin)
		}()
	}

	// Run the command
	exitCode := run(args)

	// Close writers and restore
	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	os.Stdin = oldStdin

	// Read output
	var outBuf, errBuf bytes.Buffer
	_, _ = io.Copy(&outBuf, rOut)
	_, _ = io.Copy(&errBuf, rErr)

	// If there's an error, return stderr; otherwise return stdout
	if exitCode != 0 && errBuf.Len() > 0 {
		return errBuf.String(), exitCode
	}
	return outBuf.String(), exitCode
}
