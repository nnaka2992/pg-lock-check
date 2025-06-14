package parser

import (
	"fmt"
	"os"
	pathutil "path/filepath"
	"testing"
)

func TestParseSQL(t *testing.T) {
	tests := []struct {
		name    string
		sql     string
		wantErr bool
		checks  func(t *testing.T, result *ParseResult)
	}{
		{
			name: "single SELECT statement",
			sql:  "SELECT * FROM users;",
			checks: func(t *testing.T, result *ParseResult) {
				if len(result.Statements) != 1 {
					t.Errorf("expected 1 statement, got %d", len(result.Statements))
				}
				stmt := result.Statements[0]
				if stmt.SQL != "SELECT * FROM users" {
					t.Errorf("unexpected SQL: %s", stmt.SQL)
				}
				if stmt.LineNumber != 1 {
					t.Errorf("expected line number 1, got %d", stmt.LineNumber)
				}
				if stmt.AST == nil {
					t.Error("AST should not be nil")
				}
			},
		},
		{
			name: "multiple statements",
			sql: `CREATE TABLE users (id INT);
INSERT INTO users VALUES (1);
UPDATE users SET id = 2 WHERE id = 1;`,
			checks: func(t *testing.T, result *ParseResult) {
				if len(result.Statements) != 3 {
					t.Errorf("expected 3 statements, got %d", len(result.Statements))
				}

				expectedSQL := []string{
					"CREATE TABLE users (id INT)",
					"INSERT INTO users VALUES (1)",
					"UPDATE users SET id = 2 WHERE id = 1",
				}
				expectedLines := []int{1, 2, 3}

				for i, stmt := range result.Statements {
					if stmt.SQL != expectedSQL[i] {
						t.Errorf("statement %d: expected SQL %q, got %q", i, expectedSQL[i], stmt.SQL)
					}
					if stmt.LineNumber != expectedLines[i] {
						t.Errorf("statement %d: expected line %d, got %d", i, expectedLines[i], stmt.LineNumber)
					}
					if stmt.AST == nil {
						t.Errorf("statement %d: AST should not be nil", i)
					}
				}
			},
		},
		{
			name: "statements with empty lines",
			sql: `SELECT 1;

SELECT 2;


SELECT 3;`,
			checks: func(t *testing.T, result *ParseResult) {
				if len(result.Statements) != 3 {
					t.Errorf("expected 3 statements, got %d", len(result.Statements))
				}
				expectedLines := []int{1, 3, 6}
				for i, stmt := range result.Statements {
					if stmt.LineNumber != expectedLines[i] {
						t.Errorf("statement %d: expected line %d, got %d", i, expectedLines[i], stmt.LineNumber)
					}
				}
			},
		},
		{
			name: "statements with comments",
			sql: `-- This is a comment
SELECT 1;
/* Multi-line
   comment */
SELECT 2;
-- Another comment
SELECT 3;`,
			checks: func(t *testing.T, result *ParseResult) {
				if len(result.Statements) != 3 {
					t.Errorf("expected 3 statements, got %d", len(result.Statements))
				}
				// Note: pg_query includes comments as part of statements,
				// so line numbers may not be exactly where the SQL starts
				if len(result.Statements) >= 3 {
					// Just verify we have ascending line numbers
					for i := 1; i < len(result.Statements); i++ {
						if result.Statements[i].LineNumber <= result.Statements[i-1].LineNumber {
							t.Errorf("statement %d line number %d should be greater than statement %d line number %d",
								i, result.Statements[i].LineNumber, i-1, result.Statements[i-1].LineNumber)
						}
					}
				}
			},
		},
		{
			name: "complex DDL statement",
			sql: `CREATE TABLE products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    price DECIMAL(10, 2),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);`,
			checks: func(t *testing.T, result *ParseResult) {
				if len(result.Statements) != 1 {
					t.Errorf("expected 1 statement, got %d", len(result.Statements))
				}
				if result.Statements[0].LineNumber != 1 {
					t.Errorf("expected line number 1, got %d", result.Statements[0].LineNumber)
				}
			},
		},
		{
			name:    "invalid SQL",
			sql:     "SELECT * FROM WHERE;",
			wantErr: true,
		},
		{
			name: "empty SQL",
			sql:  "",
			checks: func(t *testing.T, result *ParseResult) {
				if len(result.Statements) != 0 {
					t.Errorf("expected 0 statements for empty SQL, got %d", len(result.Statements))
				}
			},
		},
		{
			name: "only comments",
			sql:  "-- Just a comment\n/* Another comment */",
			checks: func(t *testing.T, result *ParseResult) {
				if len(result.Statements) != 0 {
					t.Errorf("expected 0 statements for comment-only SQL, got %d", len(result.Statements))
				}
			},
		},
		{
			name: "transaction blocks",
			sql: `BEGIN;
UPDATE users SET status = 'active';
COMMIT;`,
			checks: func(t *testing.T, result *ParseResult) {
				if len(result.Statements) != 3 {
					t.Errorf("expected 3 statements, got %d", len(result.Statements))
				}
				expectedSQL := []string{
					"BEGIN",
					"UPDATE users SET status = 'active'",
					"COMMIT",
				}
				for i, stmt := range result.Statements {
					if stmt.SQL != expectedSQL[i] {
						t.Errorf("statement %d: expected SQL %q, got %q", i, expectedSQL[i], stmt.SQL)
					}
				}
			},
		},
		{
			name: "various transaction statements",
			sql: `BEGIN;
START TRANSACTION;
SAVEPOINT sp1;
RELEASE SAVEPOINT sp1;
ROLLBACK TO SAVEPOINT sp1;
COMMIT;
END;
ROLLBACK;`,
			checks: func(t *testing.T, result *ParseResult) {
				if len(result.Statements) != 8 {
					t.Errorf("expected 8 statements, got %d", len(result.Statements))
				}
			},
		},
		{
			name: "transaction with isolation level",
			sql: `BEGIN ISOLATION LEVEL SERIALIZABLE;
SELECT * FROM users;
COMMIT;`,
			checks: func(t *testing.T, result *ParseResult) {
				if len(result.Statements) != 3 {
					t.Errorf("expected 3 statements, got %d", len(result.Statements))
				}
			},
		},
		{
			name: "semicolon in string literal",
			sql:  `SELECT 'hello; world' AS greeting;`,
			checks: func(t *testing.T, result *ParseResult) {
				if len(result.Statements) != 1 {
					t.Errorf("expected 1 statement, got %d", len(result.Statements))
				}
				if result.Statements[0].SQL != `SELECT 'hello; world' AS greeting` {
					t.Errorf("unexpected SQL: %s", result.Statements[0].SQL)
				}
			},
		},
		{
			name: "statement without trailing semicolon",
			sql:  "SELECT 1",
			checks: func(t *testing.T, result *ParseResult) {
				if len(result.Statements) != 1 {
					t.Errorf("expected 1 statement, got %d", len(result.Statements))
					return
				}
				if result.Statements[0].SQL != "SELECT 1" {
					t.Errorf("unexpected SQL: %s", result.Statements[0].SQL)
				}
			},
		},
		{
			name: "mixed statements with and without semicolons",
			sql: `SELECT 1;
SELECT 2`,
			checks: func(t *testing.T, result *ParseResult) {
				if len(result.Statements) != 2 {
					t.Errorf("expected 2 statements, got %d", len(result.Statements))
				}
			},
		},
	}

	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseSQL(tt.sql)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSQL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.checks != nil {
				tt.checks(t, result)
			}
		})
	}
}

func TestParseFile(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name     string
		filename string
		content  string
		wantErr  bool
		checks   func(t *testing.T, result *ParseResult)
	}{
		{
			name:     "valid SQL file",
			filename: "valid.sql",
			content: `CREATE TABLE users (id INT);
INSERT INTO users VALUES (1);`,
			checks: func(t *testing.T, result *ParseResult) {
				if len(result.Statements) != 2 {
					t.Errorf("expected 2 statements, got %d", len(result.Statements))
				}
			},
		},
		{
			name:     "empty file",
			filename: "empty.sql",
			content:  "",
			checks: func(t *testing.T, result *ParseResult) {
				if len(result.Statements) != 0 {
					t.Errorf("expected 0 statements, got %d", len(result.Statements))
				}
			},
		},
		{
			name:     "file with BOM",
			filename: "bom.sql",
			content:  "\xEF\xBB\xBFSELECT 1;",
			checks: func(t *testing.T, result *ParseResult) {
				if len(result.Statements) != 1 {
					t.Errorf("expected 1 statement, got %d", len(result.Statements))
				}
				if result.Statements[0].SQL != "SELECT 1" {
					t.Errorf("BOM should be stripped, got %q", result.Statements[0].SQL)
				}
			},
		},
		{
			name:     "non-existent file",
			filename: "/non/existent/path/file.sql",
			wantErr:  true,
		},
		{
			name:     "empty filepath",
			filename: "",
			wantErr:  true,
		},
		{
			name:     "large file with many statements",
			filename: "large.sql",
			content:  generateLargeSQL(100), // Helper function to generate 100 statements
			checks: func(t *testing.T, result *ParseResult) {
				if len(result.Statements) != 100 {
					t.Errorf("expected 100 statements, got %d", len(result.Statements))
				}
			},
		},
		{
			name:     "file with transaction",
			filename: "transaction.sql",
			content: `BEGIN;
CREATE TABLE users (id INT);
INSERT INTO users VALUES (1);
COMMIT;`,
			checks: func(t *testing.T, result *ParseResult) {
				if len(result.Statements) != 4 {
					t.Errorf("expected 4 statements, got %d", len(result.Statements))
				}
			},
		},
	}

	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var filepath string
			if tt.name != "non-existent file" && tt.name != "empty filepath" {
				filepath = pathutil.Join(tempDir, tt.filename)
				err := os.WriteFile(filepath, []byte(tt.content), 0644)
				if err != nil {
					t.Fatalf("failed to create test file: %v", err)
				}
			} else {
				filepath = tt.filename
			}

			result, err := parser.ParseFile(filepath)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.checks != nil {
				tt.checks(t, result)
			}
		})
	}
}

func TestParseFiles(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files
	files := map[string]string{
		"1_create_table.sql": "CREATE TABLE users (id INT);",
		"2_insert_data.sql":  "INSERT INTO users VALUES (1), (2);",
		"3_update_data.sql":  "UPDATE users SET id = id + 1;",
		"4_transaction.sql": `BEGIN;
DELETE FROM users WHERE id > 10;
COMMIT;`,
		"invalid.sql": "SELECT * FROM WHERE;",
	}

	createdFiles := make(map[string]string)
	for filename, content := range files {
		path := pathutil.Join(tempDir, filename)
		err := os.WriteFile(path, []byte(content), 0644)
		if err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
		createdFiles[filename] = path
	}

	tests := []struct {
		name    string
		files   []string
		wantErr bool
		checks  func(t *testing.T, result *ParseResult)
	}{
		{
			name: "multiple valid files",
			files: []string{
				createdFiles["1_create_table.sql"],
				createdFiles["2_insert_data.sql"],
				createdFiles["3_update_data.sql"],
				createdFiles["4_transaction.sql"],
			},
			checks: func(t *testing.T, result *ParseResult) {
				if len(result.Statements) != 6 { // 1 + 1 + 1 + 3 statements
					t.Errorf("expected 6 statements total, got %d", len(result.Statements))
				}
			},
		},
		{
			name: "including invalid file",
			files: []string{
				createdFiles["1_create_table.sql"],
				createdFiles["invalid.sql"],
			},
			wantErr: true,
		},
		{
			name:  "empty file list",
			files: []string{},
			checks: func(t *testing.T, result *ParseResult) {
				if len(result.Statements) != 0 {
					t.Errorf("expected 0 statements, got %d", len(result.Statements))
				}
			},
		},
		{
			name: "non-existent file in list",
			files: []string{
				createdFiles["1_create_table.sql"],
				"/non/existent/file.sql",
			},
			wantErr: true,
		},
		{
			name: "duplicate files",
			files: []string{
				createdFiles["1_create_table.sql"],
				createdFiles["1_create_table.sql"],
			},
			checks: func(t *testing.T, result *ParseResult) {
				if len(result.Statements) != 2 {
					t.Errorf("expected 2 statements (duplicated), got %d", len(result.Statements))
				}
			},
		},
	}

	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseFiles(tt.files)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.checks != nil {
				tt.checks(t, result)
			}
		})
	}
}

func TestLineNumberCalculation(t *testing.T) {
	tests := []struct {
		name          string
		sql           string
		expectedLines []int
	}{
		{
			name:          "Unix line endings (LF)",
			sql:           "SELECT 1;\nSELECT 2;\nSELECT 3;",
			expectedLines: []int{1, 2, 3},
		},
		{
			name:          "Windows line endings (CRLF)",
			sql:           "SELECT 1;\r\nSELECT 2;\r\nSELECT 3;",
			expectedLines: []int{1, 2, 3},
		},
		{
			name:          "Mixed line endings",
			sql:           "SELECT 1;\nSELECT 2;\r\nSELECT 3;",
			expectedLines: []int{1, 2, 3},
		},
		{
			name: "Multi-line statement",
			sql: `SELECT 
    id,
    name
FROM users;
SELECT * FROM products;`,
			expectedLines: []int{1, 5},
		},
		{
			name: "Statement after multiple empty lines",
			sql: `SELECT 1;



SELECT 2;`,
			expectedLines: []int{1, 5},
		},
		{
			name: "Transaction with multi-line statements",
			sql: `BEGIN;
UPDATE users 
  SET status = 'active'
  WHERE created_at < NOW();
COMMIT;`,
			expectedLines: []int{1, 2, 5},
		},
	}

	parser := NewParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseSQL(tt.sql)
			if err != nil {
				t.Fatalf("ParseSQL() error = %v", err)
			}

			if len(result.Statements) != len(tt.expectedLines) {
				t.Errorf("expected %d statements, got %d", len(tt.expectedLines), len(result.Statements))
				return
			}

			for i, stmt := range result.Statements {
				if stmt.LineNumber != tt.expectedLines[i] {
					t.Errorf("statement %d: expected line %d, got %d", i, tt.expectedLines[i], stmt.LineNumber)
				}
			}
		})
	}
}

// Helper function to generate large SQL content for testing
func generateLargeSQL(numStatements int) string {
	var sql string
	for i := 0; i < numStatements; i++ {
		sql += fmt.Sprintf("INSERT INTO test_table VALUES (%d);\n", i)
	}
	return sql
}

// Test to ensure AST is properly populated
func TestASTContent(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		sql      string
		checkAST func(t *testing.T, stmt ParsedStatement)
	}{
		{
			name: "SELECT statement AST",
			sql:  "SELECT id, name FROM users WHERE id = 1;",
			checkAST: func(t *testing.T, stmt ParsedStatement) {
				if stmt.AST == nil {
					t.Fatal("AST should not be nil")
				}
				if len(stmt.AST.Stmts) != 1 {
					t.Errorf("expected 1 statement in AST, got %d", len(stmt.AST.Stmts))
				}
				if stmt.AST.Stmts[0].Stmt == nil {
					t.Error("AST statement node should not be nil")
				}
			},
		},
		{
			name: "Transaction statement AST",
			sql:  "BEGIN;",
			checkAST: func(t *testing.T, stmt ParsedStatement) {
				if stmt.AST == nil {
					t.Fatal("AST should not be nil")
				}
				if len(stmt.AST.Stmts) != 1 {
					t.Errorf("expected 1 statement in AST, got %d", len(stmt.AST.Stmts))
				}
			},
		},
		{
			name: "DDL statement AST",
			sql:  "CREATE INDEX idx_users_email ON users(email);",
			checkAST: func(t *testing.T, stmt ParsedStatement) {
				if stmt.AST == nil {
					t.Fatal("AST should not be nil")
				}
				if len(stmt.AST.Stmts) != 1 {
					t.Errorf("expected 1 statement in AST, got %d", len(stmt.AST.Stmts))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseSQL(tt.sql)
			if err != nil {
				t.Fatalf("ParseSQL() error = %v", err)
			}

			if len(result.Statements) != 1 {
				t.Fatalf("expected 1 statement, got %d", len(result.Statements))
			}

			tt.checkAST(t, result.Statements[0])
		})
	}
}
