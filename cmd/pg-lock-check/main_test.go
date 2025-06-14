package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// Test the core run function without building binary
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
			wantError: "Usage:",
		},
		{
			name:       "help flag",
			args:       []string{"-h"},
			wantExit:   0,
			wantOutput: "Usage:",
		},
		{
			name:       "version flag",
			args:       []string{"-v"},
			wantExit:   0,
			wantOutput: "pg-lock-check",
		},
		{
			name:       "simple SELECT",
			args:       []string{"SELECT * FROM users"},
			wantExit:   0,
			wantOutput: "Summary:",
		},
		{
			name:       "UPDATE without WHERE shows critical",
			args:       []string{"UPDATE users SET x = 1"},
			wantExit:   0,
			wantOutput: "CRITICAL",
		},
		{
			name:      "invalid SQL",
			args:      []string{"INVALID SQL"},
			wantExit:  2,
			wantError: "syntax error",
		},
		{
			name:       "file input",
			args:       []string{"-f", "testdata/simple.sql"},
			wantExit:   0,
			wantOutput: "Summary:",
		},
		{
			name:      "non-existent file",
			args:      []string{"-f", "does-not-exist.sql"},
			wantExit:  1,
			wantError: "no such file",
		},
		{
			name:       "stdin input",
			args:       []string{},
			stdin:      "SELECT 1",
			wantExit:   0,
			wantOutput: "Summary:",
		},
		{
			name:       "JSON output",
			args:       []string{"-o", "json", "SELECT 1"},
			wantExit:   0,
			wantOutput: `"results"`,
		},
		{
			name:       "no-transaction mode",
			args:       []string{"--no-transaction", "CREATE INDEX CONCURRENTLY idx ON users(id)"},
			wantExit:   0,
			wantOutput: "WARNING",
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
				wIn.WriteString(tt.stdin)
				wIn.Close()
			}
			
			// Run the function
			exitCode := run(tt.args)
			
			// Restore
			w.Close()
			wErr.Close()
			os.Stdout = oldStdout
			os.Stderr = oldStderr
			os.Stdin = oldStdin
			
			// Read output
			stdout.ReadFrom(r)
			stderr.ReadFrom(rErr)
			
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
	os.MkdirAll("testdata", 0755)
	
	// Create simple test file
	os.WriteFile("testdata/simple.sql", []byte("SELECT * FROM users;"), 0644)
	
	// Run tests
	code := m.Run()
	
	// Cleanup
	os.RemoveAll("testdata")
	
	os.Exit(code)
}

// Optional acceptance test - only runs with -acceptance flag
func TestAcceptance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping acceptance test in short mode")
	}
	
	// This would test the built binary
	// For now, we skip this as it's optional
}