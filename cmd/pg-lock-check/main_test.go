package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestValidFlags(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantExit  int
		wantError string // substring in stderr
	}{
		// Help flag combinations
		{
			name:     "help flag short",
			args:     []string{"-h"},
			wantExit: 0,
		},
		{
			name:     "help flag long",
			args:     []string{"--help"},
			wantExit: 0,
		},
		// Version flag combinations
		{
			name:     "version flag short",
			args:     []string{"-v"},
			wantExit: 0,
		},
		{
			name:     "version flag long",
			args:     []string{"--version"},
			wantExit: 0,
		},
		// File input combinations
		{
			name:     "file flag short",
			args:     []string{"-f", "testdata/simple.sql"},
			wantExit: 0,
		},
		{
			name:     "file flag long",
			args:     []string{"--file", "testdata/simple.sql"},
			wantExit: 0,
		},
		// Output format combinations
		{
			name:     "output flag short text",
			args:     []string{"-o", "text", "SELECT 1"},
			wantExit: 0,
		},
		{
			name:     "output flag long json",
			args:     []string{"--output", "json", "SELECT 1"},
			wantExit: 0,
		},
		{
			name:     "output flag yaml",
			args:     []string{"-o", "yaml", "SELECT 1"},
			wantExit: 0,
		},
		// Transaction mode combinations
		{
			name:     "no-transaction flag",
			args:     []string{"--no-transaction", "SELECT 1"},
			wantExit: 0,
		},
		// Quiet mode combinations
		{
			name:     "quiet flag short",
			args:     []string{"-q", "SELECT 1"},
			wantExit: 0,
		},
		{
			name:     "quiet flag long",
			args:     []string{"--quiet", "SELECT 1"},
			wantExit: 0,
		},
		// Verbose mode
		{
			name:     "verbose flag",
			args:     []string{"--verbose", "SELECT 1"},
			wantExit: 0,
		},
		// No color mode
		{
			name:     "no-color flag",
			args:     []string{"--no-color", "SELECT 1"},
			wantExit: 0,
		},
		// Common flag combinations
		{
			name:     "file with json output",
			args:     []string{"-f", "testdata/simple.sql", "-o", "json"},
			wantExit: 0,
		},
		{
			name:     "no-transaction with file input",
			args:     []string{"--no-transaction", "-f", "testdata/simple.sql"},
			wantExit: 0,
		},
		{
			name:     "quiet with json output",
			args:     []string{"-q", "-o", "json", "SELECT 1"},
			wantExit: 0,
		},
		{
			name:     "verbose with yaml output",
			args:     []string{"--verbose", "-o", "yaml", "SELECT 1"},
			wantExit: 0,
		},
		// Suggestion flags (only --no-suggestion supported, suggestions are default)
		{
			name:     "no-suggestion flag",
			args:     []string{"--no-suggestion", "SELECT 1"},
			wantExit: 0,
		},
		// Complex combinations
		{
			name:     "multiple flags with SQL",
			args:     []string{"--no-color", "--verbose", "-o", "text", "UPDATE users SET x = 1"},
			wantExit: 0,
		},
		{
			name:     "all formatting flags",
			args:     []string{"--no-color", "--no-transaction", "-o", "json", "CREATE INDEX idx ON users(id)"},
			wantExit: 0,
		},
		// Error cases
		{
			name:      "missing file argument",
			args:      []string{"-f"},
			wantExit:  1,
			wantError: "flag needs an argument",
		},
		{
			name:      "non-existent file",
			args:      []string{"-f", "does-not-exist.sql"},
			wantExit:  1,
			wantError: "reading file: open does-not-exist.sql: no such file or directory",
		},
		{
			name:      "unknown flag",
			args:      []string{"--unknown-flag", "SELECT 1"},
			wantExit:  1,
			wantError: "unknown flag: --unknown-flag",
		},
		{
			name:      "no SQL provided with flags",
			args:      []string{"--no-color", "--verbose"},
			wantExit:  1,
			wantError: "no SQL provided",
		},
		{
			name:      "invalid short flag",
			args:      []string{"-x", "SELECT 1"},
			wantExit:  1,
			wantError: "unknown shorthand flag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout and stderr
			oldStdout := os.Stdout
			oldStderr := os.Stderr

			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			r, w, _ := os.Pipe()
			os.Stdout = w

			rErr, wErr, _ := os.Pipe()
			os.Stderr = wErr

			// Run the function
			exitCode := run(tt.args)

			// Restore
			_ = w.Close()
			_ = wErr.Close()
			os.Stdout = oldStdout
			os.Stderr = oldStderr

			// Read output
			_, _ = stdout.ReadFrom(r)
			_, _ = stderr.ReadFrom(rErr)

			// Check exit code
			if exitCode != tt.wantExit {
				t.Errorf("exit code = %d, want %d", exitCode, tt.wantExit)
			}

			// Check stderr
			if tt.wantError != "" && !strings.Contains(stderr.String(), tt.wantError) {
				t.Errorf("stderr missing %q\nGot: %s", tt.wantError, stderr.String())
			}
		})
	}
}

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantExit   int
		wantOutput string // substring to check
		wantError  string // substring in stderr
	}{
		{
			name:      "no arguments shows usage",
			args:      []string{},
			wantExit:  1,
			wantError: `Error: no SQL provided`,
		},
		{
			name:     "simple SELECT with --no-suggestion",
			args:     []string{"--no-suggestion", "SELECT * FROM users"},
			wantExit: 0,
			wantOutput: `[INFO] SELECT * FROM users

Summary: 1 statements analyzed`,
		},
		{
			name:     "UPDATE without WHERE shows critical with --no-suggestion",
			args:     []string{"--no-suggestion", "UPDATE users SET x = 1"},
			wantExit: 0,
			wantOutput: `[CRITICAL] UPDATE users SET x = 1

Summary: 1 statements analyzed`,
		},
		{
			name:     "simple SELECT",
			args:     []string{"SELECT * FROM users"},
			wantExit: 0,
			wantOutput: `[INFO] SELECT * FROM users

Summary: 1 statements analyzed`,
		},
		{
			name:     "UPDATE without WHERE shows critical",
			args:     []string{"UPDATE users SET active = false"},
			wantExit: 0,
			wantOutput: `[CRITICAL] UPDATE users SET active = false
Suggestion for safe migration:
  Step: Export target row IDs to file
    Can run in transaction: Yes
    SQL:
      \COPY (SELECT id FROM users ORDER BY id) TO '/path/to/target_ids.csv' CSV
  Step: Process file in batches with progress tracking
    Can run in transaction: No
    Instructions:
      1. Read ID file in chunks (e.g., 1000-5000 rows)
      2. For each chunk:
         - Build explicit ID list
         - Execute UPDATE users SET active = false WHERE id IN (chunk_ids)
         - Commit transaction
         - Log progress (line number or ID range)
         - Sleep 100-500ms between batches
         - Monitor replication lag
      3. Handle failures with resume capability

Summary: 1 statements analyzed`,
		},
		{
			name:      "invalid SQL",
			args:      []string{"INVALID SQL"},
			wantExit:  2,
			wantError: "syntax error",
		},
		{
			name:     "stdin input",
			args:     []string{},
			stdin:    "SELECT 1",
			wantExit: 0,
			wantOutput: `[INFO] SELECT 1

Summary: 1 statements analyzed`,
		},
		{
			name:     "no-transaction mode",
			args:     []string{"--no-transaction", "CREATE INDEX CONCURRENTLY idx ON users(id)"},
			wantExit: 0,
			wantOutput: `[WARNING] CREATE INDEX CONCURRENTLY idx ON users(id)

Summary: 1 statements analyzed`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout and stderr
			oldStdout := os.Stdout
			oldStderr := os.Stderr
			oldStdin := os.Stdin

			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}

			r, w, _ := os.Pipe()
			os.Stdout = w

			rErr, wErr, _ := os.Pipe()
			os.Stderr = wErr

			if tt.stdin != "" {
				rIn, wIn, _ := os.Pipe()
				os.Stdin = rIn
				_, _ = wIn.WriteString(tt.stdin)
				_ = wIn.Close()
			}

			// Run the function
			exitCode := run(tt.args)

			// Restore
			_ = w.Close()
			_ = wErr.Close()
			os.Stdout = oldStdout
			os.Stderr = oldStderr
			os.Stdin = oldStdin

			// Read output
			_, _ = stdout.ReadFrom(r)
			_, _ = stderr.ReadFrom(rErr)

			// Check exit code
			if exitCode != tt.wantExit {
				t.Errorf("exit code = %d, want %d", exitCode, tt.wantExit)
			}

			// Check stdout
			if tt.wantOutput != "" && !strings.Contains(stdout.String(), tt.wantOutput) {
				t.Errorf("stdout missing %q\nGot: %s", tt.wantOutput, stdout.String())
			}

			// Check stderr
			if tt.wantError != "" && !strings.Contains(stderr.String(), tt.wantError) {
				t.Errorf("stderr missing %q\nGot: %s", tt.wantError, stderr.String())
			}
		})
	}
}

// Test data setup
func TestMain(m *testing.M) {
	// Create test data directory
	_ = os.MkdirAll("testdata", 0755)

	// Create simple test file
	_ = os.WriteFile("testdata/simple.sql", []byte("SELECT * FROM users;"), 0644)

	// Run tests
	code := m.Run()

	// Cleanup
	_ = os.RemoveAll("testdata")

	os.Exit(code)
}
