package main

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

// TestCase represents a single test case with expected values
type SuggestedOutputTestCase struct {
	name string
	sql  string

	// Expected values (not full JSON/YAML)
	expectTotalStatements int
	expectResults         []SuggestedExpectedResult
}

type SuggestedExpectedResult struct {
	index      int    // Expected index in results array (0-based)
	lineNumber int    // Expected line number in input (1-based)
	sql        string // Expected SQL statement
	severity   string
	operation  string
	lockType   string
	tables     []SuggestionExpectedTable
	suggestion *ExpectedSuggestion
}

type SuggestionExpectedTable struct {
	name     string
	lockType string
}

type ExpectedSuggestion struct {
	steps []ExpectedStep
}

type ExpectedStep struct {
	description         string
	canRunInTransaction bool
	output              string
}

func TestSuggestionOutput(t *testing.T) {
	tests := []SuggestedOutputTestCase{
		{
			name:                  "No suggestion",
			sql:                   "INSERT INTO users (name) VALUES ('test');",
			expectTotalStatements: 1,
			expectResults: []SuggestedExpectedResult{
				{
					index:      0,
					lineNumber: 1,
					sql:        "INSERT INTO users (name) VALUES ('test');",
					severity:   "INFO",
					operation:  "INSERT",
					lockType:   "RowExclusive",
					tables: []SuggestionExpectedTable{
						{name: "users", lockType: "RowExclusive"},
					},
					suggestion: &ExpectedSuggestion{}, // no sugestion
				},
			},
		},
		{
			name:                  "UPDATE without WHERE with suggestion",
			sql:                   "UPDATE users SET active = false;",
			expectTotalStatements: 1,
			expectResults: []SuggestedExpectedResult{
				{
					index:      0,
					lineNumber: 1,
					sql:        "UPDATE users SET active = false;",
					severity:   "CRITICAL",
					operation:  "UPDATE without WHERE",
					lockType:   "RowExclusive",
					tables: []SuggestionExpectedTable{
						{name: "users", lockType: "RowExclusive"},
					},
					suggestion: &ExpectedSuggestion{
						steps: []ExpectedStep{
							{
								description:         "Export target row IDs to file",
								canRunInTransaction: true,
								output:              `\COPY (SELECT id FROM users ORDER BY id) TO '/path/to/target_ids.csv' CSV` + "\n",
							},
							{
								description:         "Process file in batches with progress tracking",
								canRunInTransaction: false,
								output:              "contains:Read ID file in chunks",
							},
						},
					},
				},
			},
		},
		{
			name:                  "DELETE without WHERE with suggestion",
			sql:                   "DELETE FROM sessions;",
			expectTotalStatements: 1,
			expectResults: []SuggestedExpectedResult{
				{
					index:      0,
					lineNumber: 1,
					sql:        "DELETE FROM sessions;",
					severity:   "CRITICAL",
					operation:  "DELETE without WHERE",
					lockType:   "RowExclusive",
					tables: []SuggestionExpectedTable{
						{name: "sessions", lockType: "RowExclusive"},
					},
					suggestion: &ExpectedSuggestion{
						steps: []ExpectedStep{
							{
								description:         "Export target row IDs to file",
								canRunInTransaction: true,
								output:              `\COPY (SELECT id FROM sessions ORDER BY id) TO '/path/to/target_ids.csv' CSV` + "\n",
							},
							{
								description:         "Process file in batches",
								canRunInTransaction: false,
								output:              "contains:Read ID file in chunks",
							},
						},
					},
				},
			},
		},
		{
			name:                  "MERGE without WHERE with suggestion",
			sql:                   "MERGE INTO users USING new_users ON users.id = new_users.id WHEN MATCHED THEN UPDATE SET email = new_users.email WHEN NOT MATCHED THEN INSERT (id, name, email, created_at, updated_at) VALUES (new_users.id, new_users.name, new_users.email, new_users.created_at, new_users.updated_at);",
			expectTotalStatements: 1,
			expectResults: []SuggestedExpectedResult{
				{
					index:      0,
					lineNumber: 1,
					sql:        "MERGE INTO users USING new_users ON users.id = new_users.id WHEN MATCHED THEN UPDATE SET email = new_users.email WHEN NOT MATCHED THEN INSERT (id, name, email, created_at, updated_at) VALUES (new_users.id, new_users.name, new_users.email, new_users.created_at, new_users.updated_at);",
					severity:   "CRITICAL",
					operation:  "MERGE without WHERE",
					lockType:   "RowExclusive",
					tables: []SuggestionExpectedTable{
						{name: "users", lockType: "RowExclusive"},
						{name: "new_users", lockType: "AccessShare"},
					},
					suggestion: &ExpectedSuggestion{
						steps: []ExpectedStep{
							{
								description:         "Export source data IDs to file",
								canRunInTransaction: true,
								output:              `\COPY (SELECT "id" FROM new_users ORDER BY id) TO '/path/to/source_ids.csv' CSV` + "\n",
							},
							{
								description:         "Add conditions to WHEN clauses or batch with subqueries",
								canRunInTransaction: false,
								output:              "contains:Read ID file in chunks",
							},
						},
					},
				},
			},
		},
		{
			name:                  "DROP INDEX with suggestion",
			sql:                   "DROP INDEX idx_users_email;",
			expectTotalStatements: 1,
			expectResults: []SuggestedExpectedResult{
				{
					index:      0,
					lineNumber: 1,
					sql:        "DROP INDEX idx_users_email;",
					severity:   "CRITICAL",
					operation:  "DROP INDEX",
					lockType:   "AccessExclusive",
					tables: []SuggestionExpectedTable{
						{name: "idx_users_email", lockType: "AccessExclusive"},
					},
					suggestion: &ExpectedSuggestion{
						steps: []ExpectedStep{
							{
								description:         "Use DROP INDEX CONCURRENTLY outside transaction",
								canRunInTransaction: false,
								output:              "DROP INDEX CONCURRENTLY idx_users_email;\n",
							},
						},
					},
				},
			},
		},
		{
			name:                  "CREATE INDEX with suggestion",
			sql:                   "CREATE INDEX idx_users_email ON users(email);",
			expectTotalStatements: 1,
			expectResults: []SuggestedExpectedResult{
				{
					index:      0,
					lineNumber: 1,
					sql:        "CREATE INDEX idx_users_email ON users(email);",
					severity:   "CRITICAL",
					operation:  "CREATE INDEX",
					lockType:   "Share",
					tables: []SuggestionExpectedTable{
						{name: "users", lockType: "Share"},
					},
					suggestion: &ExpectedSuggestion{
						steps: []ExpectedStep{
							{
								description:         "Use CREATE INDEX CONCURRENTLY outside transaction",
								canRunInTransaction: false,
								output:              "CREATE INDEX CONCURRENTLY idx_users_email ON users(email);\n",
							},
						},
					},
				},
			},
		},
		{
			name:                  "CREATE UNIQUE INDEX with suggestion",
			sql:                   "CREATE UNIQUE INDEX uniq_users_username ON users(username);",
			expectTotalStatements: 1,
			expectResults: []SuggestedExpectedResult{
				{
					index:      0,
					lineNumber: 1,
					sql:        "CREATE UNIQUE INDEX uniq_users_username ON users(username);",
					severity:   "CRITICAL",
					operation:  "CREATE UNIQUE INDEX",
					lockType:   "Share",
					tables: []SuggestionExpectedTable{
						{name: "users", lockType: "Share"},
					},
					suggestion: &ExpectedSuggestion{
						steps: []ExpectedStep{
							{
								description:         "Use CREATE UNIQUE INDEX CONCURRENTLY outside transaction",
								canRunInTransaction: false,
								output:              "CREATE UNIQUE INDEX CONCURRENTLY uniq_users_username ON users(username);\n",
							},
						},
					},
				},
			},
		},
		{
			name:                  "REINDEX with suggestion",
			sql:                   "REINDEX INDEX idx_users_email;",
			expectTotalStatements: 1,
			expectResults: []SuggestedExpectedResult{
				{
					index:      0,
					lineNumber: 1,
					sql:        "REINDEX INDEX idx_users_email;",
					severity:   "CRITICAL",
					operation:  "REINDEX",
					lockType:   "AccessExclusive",
					tables: []SuggestionExpectedTable{
						{name: "idx_users_email", lockType: "AccessExclusive"},
					},
					suggestion: &ExpectedSuggestion{
						steps: []ExpectedStep{
							{
								description:         "Use `REINDEX CONCURRENTLY` or CREATE new index + DROP old pattern",
								canRunInTransaction: false,
								output:              "REINDEX INDEX CONCURRENTLY idx_users_email;\n",
							},
						},
					},
				},
			},
		},
		{
			name:                  "REINDEX TABLE with suggestion",
			sql:                   "REINDEX TABLE users;",
			expectTotalStatements: 1,
			expectResults: []SuggestedExpectedResult{
				{
					index:      0,
					lineNumber: 1,
					sql:        "REINDEX TABLE users;",
					severity:   "CRITICAL",
					operation:  "REINDEX TABLE",
					lockType:   "AccessExclusive",
					tables: []SuggestionExpectedTable{
						{name: "users", lockType: "AccessExclusive"},
					},
					suggestion: &ExpectedSuggestion{
						steps: []ExpectedStep{
							{
								description:         "Export all index names for the table",
								canRunInTransaction: true,
								output:              `\COPY (SELECT indexname FROM pg_indexes WHERE tablename = 'users' ORDER BY indexname) TO '/path/to/table_indexes.csv' CSV` + "\n",
							},
							{
								description:         "Reindex each index individually",
								canRunInTransaction: false,
								output:              "contains:For each index from the exported file",
							},
						},
					},
				},
			},
		},
		{
			name:                  "REINDEX DATABASE with suggestion",
			sql:                   "REINDEX DATABASE mydb;",
			expectTotalStatements: 1,
			expectResults: []SuggestedExpectedResult{
				{
					index:      0,
					lineNumber: 1,
					sql:        "REINDEX DATABASE mydb;",
					severity:   "CRITICAL",
					operation:  "REINDEX DATABASE",
					lockType:   "AccessExclusive",
					tables: []SuggestionExpectedTable{
						{name: "mydb", lockType: "AccessExclusive"},
					},
					suggestion: &ExpectedSuggestion{
						steps: []ExpectedStep{
							{
								description:         "Export all index names in the database",
								canRunInTransaction: true,
								output:              `\COPY (SELECT schemaname || '.' || indexname FROM pg_indexes WHERE schemaname NOT IN ('pg_catalog', 'information_schema') ORDER BY schemaname, indexname) TO '/path/to/database_indexes.csv' CSV` + "\n",
							},
							{
								description:         "Reindex each index individually",
								canRunInTransaction: false,
								output:              "contains:For each index from the exported file",
							},
						},
					},
				},
			},
		},
		{
			name:                  "REINDEX SCHEMA with suggestion",
			sql:                   "REINDEX SCHEMA public;",
			expectTotalStatements: 1,
			expectResults: []SuggestedExpectedResult{
				{
					index:      0,
					lineNumber: 1,
					sql:        "REINDEX SCHEMA public;",
					severity:   "CRITICAL",
					operation:  "REINDEX SCHEMA",
					lockType:   "AccessExclusive",
					tables: []SuggestionExpectedTable{
						{name: "public", lockType: "AccessExclusive"},
					},
					suggestion: &ExpectedSuggestion{
						steps: []ExpectedStep{
							{
								description:         "Export all index names in the schema",
								canRunInTransaction: true,
								output:              `\COPY (SELECT indexname FROM pg_indexes WHERE schemaname = 'public' ORDER BY indexname) TO '/path/to/schema_indexes.csv' CSV` + "\n",
							},
							{
								description:         "Reindex each index individually",
								canRunInTransaction: false,
								output:              "contains:For each index from the exported file",
							},
						},
					},
				},
			},
		},
		{
			name:                  "ALTER TABLE ADD COLUMN with volatile DEFAULT with suggestion",
			sql:                   "ALTER TABLE users ADD COLUMN new_id uuid DEFAULT gen_random_uuid();",
			expectTotalStatements: 1,
			expectResults: []SuggestedExpectedResult{
				{
					index:      0,
					lineNumber: 1,
					sql:        "ALTER TABLE users ADD COLUMN new_id uuid DEFAULT gen_random_uuid();",
					severity:   "CRITICAL",
					operation:  "ALTER TABLE ADD COLUMN with volatile DEFAULT",
					lockType:   "AccessExclusive",
					tables: []SuggestionExpectedTable{
						{name: "users", lockType: "AccessExclusive"},
					},
					suggestion: &ExpectedSuggestion{
						steps: []ExpectedStep{
							{
								description:         "`ADD COLUMN` without default",
								canRunInTransaction: true,
								output:              "ALTER TABLE users ADD COLUMN new_id uuid;\n",
							},
							{
								description:         "Batch update with default values (separate transactions per batch)",
								canRunInTransaction: false,
								output:              "contains:Identify rows with NULL values",
							},
							{
								description:         "`ALTER COLUMN SET DEFAULT`",
								canRunInTransaction: true,
								output:              "ALTER TABLE users ALTER COLUMN new_id SET DEFAULT gen_random_uuid();\n",
							},
						},
					},
				},
			},
		},
		{
			name:                  "ALTER TABLE ALTER COLUMN TYPE with suggestion",
			sql:                   "ALTER TABLE users ALTER COLUMN email TYPE VARCHAR(255);",
			expectTotalStatements: 1,
			expectResults: []SuggestedExpectedResult{
				{
					index:      0,
					lineNumber: 1,
					sql:        "ALTER TABLE users ALTER COLUMN email TYPE VARCHAR(255);",
					severity:   "CRITICAL",
					operation:  "ALTER TABLE ALTER COLUMN TYPE",
					lockType:   "AccessExclusive",
					tables: []SuggestionExpectedTable{
						{name: "users", lockType: "AccessExclusive"},
					},
					suggestion: &ExpectedSuggestion{
						steps: []ExpectedStep{
							{
								description:         "Add new column",
								canRunInTransaction: true,
								output:              "ALTER TABLE users ADD COLUMN email_new VARCHAR(255);\n",
							},
							{
								description:         "Add sync trigger",
								canRunInTransaction: true,
								output:              "contains:CREATE OR REPLACE FUNCTION sync_users_email()",
							},
							{
								description:         "Backfill script",
								canRunInTransaction: false,
								output:              "",
							},
							{
								description:         "Atomic swap",
								canRunInTransaction: true,
								output:              "contains:BEGIN",
							},
						},
					},
				},
			},
		},
		{
			name:                  "ALTER TABLE ADD PRIMARY KEY with suggestion",
			sql:                   "ALTER TABLE users ADD PRIMARY KEY (id);",
			expectTotalStatements: 1,
			expectResults: []SuggestedExpectedResult{
				{
					index:      0,
					lineNumber: 1,
					sql:        "ALTER TABLE users ADD PRIMARY KEY (id);",
					severity:   "CRITICAL",
					operation:  "ALTER TABLE ADD PRIMARY KEY",
					lockType:   "AccessExclusive",
					tables: []SuggestionExpectedTable{
						{name: "users", lockType: "AccessExclusive"},
					},
					suggestion: &ExpectedSuggestion{
						steps: []ExpectedStep{
							{
								description:         "First `CREATE UNIQUE INDEX CONCURRENTLY`",
								canRunInTransaction: false,
								output:              "CREATE UNIQUE INDEX CONCURRENTLY users_pkey ON users (id);\n",
							},
							{
								description:         "Then `ALTER TABLE ADD CONSTRAINT pkey PRIMARY KEY USING INDEX`",
								canRunInTransaction: true,
								output:              "ALTER TABLE users ADD CONSTRAINT users_pkey PRIMARY KEY USING INDEX users_pkey;\n",
							},
						},
					},
				},
			},
		},
		{
			name:                  "ALTER TABLE ADD CONSTRAINT CHECK with suggestion",
			sql:                   "ALTER TABLE users ADD CONSTRAINT check_age CHECK (age >= 18);",
			expectTotalStatements: 1,
			expectResults: []SuggestedExpectedResult{
				{
					index:      0,
					lineNumber: 1,
					sql:        "ALTER TABLE users ADD CONSTRAINT check_age CHECK (age >= 18);",
					severity:   "CRITICAL",
					operation:  "ALTER TABLE ADD CONSTRAINT CHECK",
					lockType:   "AccessExclusive",
					tables: []SuggestionExpectedTable{
						{name: "users", lockType: "AccessExclusive"},
					},
					suggestion: &ExpectedSuggestion{
						steps: []ExpectedStep{
							{
								description:         "Use `ADD CONSTRAINT NOT VALID`",
								canRunInTransaction: true,
								output:              "ALTER TABLE users ADD CONSTRAINT check_age CHECK (age >= 18) NOT VALID;\n",
							},
							{
								description:         "Then `VALIDATE CONSTRAINT`",
								canRunInTransaction: true,
								output:              "ALTER TABLE users VALIDATE CONSTRAINT check_age;\n",
							},
						},
					},
				},
			},
		},
		{
			name:                  "ALTER TABLE SET NOT NULL with suggestion",
			sql:                   "ALTER TABLE users ALTER COLUMN email SET NOT NULL;",
			expectTotalStatements: 1,
			expectResults: []SuggestedExpectedResult{
				{
					index:      0,
					lineNumber: 1,
					sql:        "ALTER TABLE users ALTER COLUMN email SET NOT NULL;",
					severity:   "CRITICAL",
					operation:  "ALTER TABLE SET NOT NULL",
					lockType:   "AccessExclusive",
					tables: []SuggestionExpectedTable{
						{name: "users", lockType: "AccessExclusive"},
					},
					suggestion: &ExpectedSuggestion{
						steps: []ExpectedStep{
							{
								description:         "`ADD CONSTRAINT CHECK (col IS NOT NULL) NOT VALID`",
								canRunInTransaction: true,
								output:              "ALTER TABLE users ADD CONSTRAINT users_email_not_null CHECK (email IS NOT NULL) NOT VALID;\n",
							},
							{
								description:         "`VALIDATE CONSTRAINT`",
								canRunInTransaction: true,
								output:              "ALTER TABLE users VALIDATE CONSTRAINT users_email_not_null;\n",
							},
							{
								description:         "`SET NOT NULL`",
								canRunInTransaction: true,
								output:              "ALTER TABLE users ALTER COLUMN email SET NOT NULL;\n",
							},
							{
								description:         "Drop constraint",
								canRunInTransaction: true,
								output:              "ALTER TABLE users DROP CONSTRAINT users_email_not_null;\n",
							},
						},
					},
				},
			},
		},
		{
			name:                  "CLUSTER with suggestion",
			sql:                   "CLUSTER users USING idx_users_id;",
			expectTotalStatements: 1,
			expectResults: []SuggestedExpectedResult{
				{
					index:      0,
					lineNumber: 1,
					sql:        "CLUSTER users USING idx_users_id;",
					severity:   "CRITICAL",
					operation:  "CLUSTER",
					lockType:   "AccessExclusive",
					tables: []SuggestionExpectedTable{
						{name: "users", lockType: "AccessExclusive"},
					},
					suggestion: &ExpectedSuggestion{
						steps: []ExpectedStep{
							{
								description:         "Consider `pg_repack` extension for online reorganization",
								canRunInTransaction: false,
								output:              "pg_repack -t users -i idx_users_id -d <YOUR_DATABASE>\n",
							},
						},
					},
				},
			},
		},
		{
			name:                  "REFRESH MATERIALIZED VIEW with suggestion",
			sql:                   "REFRESH MATERIALIZED VIEW user_stats;",
			expectTotalStatements: 1,
			expectResults: []SuggestedExpectedResult{
				{
					index:      0,
					lineNumber: 1,
					sql:        "REFRESH MATERIALIZED VIEW user_stats;",
					severity:   "CRITICAL",
					operation:  "REFRESH MATERIALIZED VIEW",
					lockType:   "AccessExclusive",
					tables: []SuggestionExpectedTable{
						{name: "user_stats", lockType: "AccessExclusive"},
					},
					suggestion: &ExpectedSuggestion{
						steps: []ExpectedStep{
							{
								description:         "Use `REFRESH MATERIALIZED VIEW CONCURRENTLY` (requires unique index)",
								canRunInTransaction: false,
								output:              "REFRESH MATERIALIZED VIEW CONCURRENTLY user_stats;\n",
							},
						},
					},
				},
			},
		},
		{
			name:                  "VACUUM FULL with suggestion",
			sql:                   "VACUUM FULL users;",
			expectTotalStatements: 1,
			expectResults: []SuggestedExpectedResult{
				{
					index:      0,
					lineNumber: 1,
					sql:        "VACUUM FULL users;",
					severity:   "CRITICAL",
					operation:  "VACUUM FULL",
					lockType:   "AccessExclusive",
					tables: []SuggestionExpectedTable{
						{name: "users", lockType: "AccessExclusive"},
					},
					suggestion: &ExpectedSuggestion{
						steps: []ExpectedStep{
							{
								description:         "Use `pg_repack` extension instead",
								canRunInTransaction: false,
								output:              "pg_repack -n -t users -d <YOUR_DATABASE>\n",
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run with JSON output to get structured data
			args := []string{"-o", "json"}
			output, exitCode := runCommand(t, args, tt.sql)
			if exitCode != 0 {
				t.Fatalf("Command failed with exit code %d: %s", exitCode, output)
			}

			// Parse JSON output
			var result map[string]interface{}
			if err := json.Unmarshal([]byte(output), &result); err != nil {
				t.Fatalf("Output is not valid JSON: %v\nOutput: %s", err, output)
			}

			// Verify summary
			summary := result["summary"].(map[string]interface{})
			totalStatements := int(summary["total_statements"].(float64))
			if totalStatements != tt.expectTotalStatements {
				t.Errorf("Expected %d statements, got %d", tt.expectTotalStatements, totalStatements)
			}

			// Verify results
			results := result["results"].([]interface{})
			if len(results) != len(tt.expectResults) {
				t.Fatalf("Expected %d results, got %d", len(tt.expectResults), len(results))
			}

			for i, expected := range tt.expectResults {
				r := results[i].(map[string]interface{})

				// Check basic fields
				if idx := int(r["index"].(float64)); idx != expected.index {
					t.Errorf("Result %d: expected index %d, got %d", i, expected.index, idx)
				}
				if lineNum := int(r["line_number"].(float64)); lineNum != expected.lineNumber {
					t.Errorf("Result %d: expected line_number %d, got %d", i, expected.lineNumber, lineNum)
				}
				if sql := r["sql"].(string); sql != expected.sql {
					t.Errorf("Result %d: expected sql %q, got %q", i, expected.sql, sql)
				}
				if severity := r["severity"].(string); severity != expected.severity {
					t.Errorf("Result %d: expected severity %s, got %s", i, expected.severity, severity)
				}
				if operation := r["operation"].(string); operation != expected.operation {
					t.Errorf("Result %d: expected operation %s, got %s", i, expected.operation, operation)
				}
				if lockType := r["lock_type"].(string); lockType != expected.lockType {
					t.Errorf("Result %d: expected lock_type %s, got %s", i, expected.lockType, lockType)
				}

				// Check tables
				tables := r["tables"].([]interface{})
				if len(tables) != len(expected.tables) {
					t.Errorf("Result %d: expected %d tables, got %d", i, len(expected.tables), len(tables))
				} else {
					// Sort tables by name for consistent comparison
					sort.Slice(tables, func(a, b int) bool {
						return tables[a].(map[string]interface{})["name"].(string) <
							tables[b].(map[string]interface{})["name"].(string)
					})
					sortedExpected := make([]SuggestionExpectedTable, len(expected.tables))
					copy(sortedExpected, expected.tables)
					sort.Slice(sortedExpected, func(a, b int) bool {
						return sortedExpected[a].name < sortedExpected[b].name
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

				// Check suggestion
				if expected.suggestion != nil && len(expected.suggestion.steps) > 0 {
					suggestionRaw, hasSuggestion := r["suggestion"]
					if !hasSuggestion || suggestionRaw == nil {
						t.Errorf("Result %d: expected suggestion but got none", i)
						continue
					}
					suggestion := suggestionRaw.(map[string]interface{})
					steps := suggestion["steps"].([]interface{})
					if len(steps) != len(expected.suggestion.steps) {
						t.Errorf("Result %d: expected %d suggestion steps, got %d", i, len(expected.suggestion.steps), len(steps))
						continue
					}

					for j, expectedStep := range expected.suggestion.steps {
						step := steps[j].(map[string]interface{})

						if desc := step["description"].(string); desc != expectedStep.description {
							t.Errorf("Result %d, step %d: expected description %q, got %q", i, j, expectedStep.description, desc)
						}
						if canRun := step["can_run_in_transaction"].(bool); canRun != expectedStep.canRunInTransaction {
							t.Errorf("Result %d, step %d: expected can_run_in_transaction %v, got %v", i, j, expectedStep.canRunInTransaction, canRun)
						}

						// Check output - supports "contains:" prefix for substring matching
						if expectedStep.output != "" {
							stepOutput := step["output"].(string)
							if strings.HasPrefix(expectedStep.output, "contains:") {
								substr := strings.TrimPrefix(expectedStep.output, "contains:")
								if !strings.Contains(stepOutput, substr) {
									t.Errorf("Result %d, step %d: output should contain %q, got %q", i, j, substr, stepOutput)
								}
							} else {
								if stepOutput != expectedStep.output {
									t.Errorf("Result %d, step %d: expected output %q, got %q", i, j, expectedStep.output, stepOutput)
								}
							}
						}
					}
				} else if expected.suggestion != nil && len(expected.suggestion.steps) == 0 {
					// Expected no suggestion (empty steps means no suggestion expected)
					if suggestionRaw, hasSuggestion := r["suggestion"]; hasSuggestion && suggestionRaw != nil {
						t.Errorf("Result %d: expected no suggestion but got one", i)
					}
				}
			}
		})
	}
}
