package metadata

import (
	"testing"

	"github.com/nnaka2992/pg-lock-check/internal/parser"
)

func TestExtractMetadata(t *testing.T) {
	p := parser.NewParser()

	tests := []struct {
		name               string
		sql                string
		operation          string
		expectedMetadata   map[string]interface{}
		shouldHaveIDColumn bool // Some operations need ID column
	}{
		{
			name:      "UPDATE without WHERE",
			sql:       "UPDATE users SET active = false, updated_at = now();",
			operation: "UPDATE without WHERE",
			expectedMetadata: map[string]interface{}{
				"tableName":     "users",
				"columnsValues": "active = false, updated_at = now()",
				"idColumn":      "id",
			},
			shouldHaveIDColumn: true,
		},
		{
			name:      "DELETE without WHERE",
			sql:       "DELETE FROM sessions;",
			operation: "DELETE without WHERE",
			expectedMetadata: map[string]interface{}{
				"tableName": "sessions",
				"idColumn":  "id",
			},
			shouldHaveIDColumn: true,
		},
		{
			name:      "CREATE INDEX",
			sql:       "CREATE INDEX idx_users_email ON users(email);",
			operation: "CREATE INDEX",
			expectedMetadata: map[string]interface{}{
				"indexName": "idx_users_email",
				"tableName": "users",
				"columns":   "email",
			},
		},
		{
			name:      "CREATE UNIQUE INDEX",
			sql:       "CREATE UNIQUE INDEX uniq_users_username ON users(username);",
			operation: "CREATE UNIQUE INDEX",
			expectedMetadata: map[string]interface{}{
				"indexName": "uniq_users_username",
				"tableName": "users",
				"columns":   "username",
			},
		},
		{
			name:      "DROP INDEX",
			sql:       "DROP INDEX idx_users_email;",
			operation: "DROP INDEX",
			expectedMetadata: map[string]interface{}{
				"indexName": "idx_users_email",
			},
		},
		{
			name:      "REINDEX",
			sql:       "REINDEX INDEX idx_users_email;",
			operation: "REINDEX",
			expectedMetadata: map[string]interface{}{
				"indexName": "idx_users_email",
			},
		},
		{
			name:      "REINDEX TABLE",
			sql:       "REINDEX TABLE users;",
			operation: "REINDEX TABLE",
			expectedMetadata: map[string]interface{}{
				"tableName": "users",
			},
		},
		{
			name:      "REINDEX SCHEMA",
			sql:       "REINDEX SCHEMA public;",
			operation: "REINDEX SCHEMA",
			expectedMetadata: map[string]interface{}{
				"schema": "public",
			},
		},
		{
			name:      "ALTER TABLE ADD COLUMN with volatile DEFAULT",
			sql:       "ALTER TABLE users ADD COLUMN new_id uuid DEFAULT gen_random_uuid();",
			operation: "ALTER TABLE ADD COLUMN with volatile DEFAULT",
			expectedMetadata: map[string]interface{}{
				"tableName":    "users",
				"columnName":   "new_id",
				"dataType":     "uuid",
				"defaultValue": "gen_random_uuid()",
			},
		},
		{
			name:      "ALTER TABLE ALTER COLUMN TYPE",
			sql:       "ALTER TABLE users ALTER COLUMN email TYPE VARCHAR(255);",
			operation: "ALTER TABLE ALTER COLUMN TYPE",
			expectedMetadata: map[string]interface{}{
				"tableName":  "users",
				"columnName": "email",
				"newType":    "VARCHAR(255)",
			},
		},
		{
			name:      "ALTER TABLE ADD PRIMARY KEY",
			sql:       "ALTER TABLE users ADD PRIMARY KEY (id);",
			operation: "ALTER TABLE ADD PRIMARY KEY",
			expectedMetadata: map[string]interface{}{
				"tableName": "users",
				"columns":   "id",
			},
		},
		{
			name:      "ALTER TABLE ADD CONSTRAINT CHECK",
			sql:       "ALTER TABLE users ADD CONSTRAINT check_age CHECK (age >= 18);",
			operation: "ALTER TABLE ADD CONSTRAINT CHECK",
			expectedMetadata: map[string]interface{}{
				"tableName":       "users",
				"constraintName":  "check_age",
				"checkExpression": "CHECK(...)",
			},
		},
		{
			name:      "ALTER TABLE SET NOT NULL",
			sql:       "ALTER TABLE users ALTER COLUMN email SET NOT NULL;",
			operation: "ALTER TABLE ALTER COLUMN SET NOT NULL",
			expectedMetadata: map[string]interface{}{
				"tableName": "users",
				"column":    "email",
			},
		},
		{
			name:      "CLUSTER",
			sql:       "CLUSTER users USING idx_users_id;",
			operation: "CLUSTER",
			expectedMetadata: map[string]interface{}{
				"tableName": "users",
				"indexName": "idx_users_id",
			},
		},
		{
			name:      "REFRESH MATERIALIZED VIEW",
			sql:       "REFRESH MATERIALIZED VIEW user_stats;",
			operation: "REFRESH MATERIALIZED VIEW",
			expectedMetadata: map[string]interface{}{
				"viewName": "user_stats",
			},
		},
		{
			name:      "VACUUM FULL",
			sql:       "VACUUM FULL users;",
			operation: "VACUUM FULL",
			expectedMetadata: map[string]interface{}{
				"tableName": "users",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse SQL
			parseResult, err := p.ParseSQL(tt.sql)
			if err != nil {
				t.Fatalf("Failed to parse SQL: %v", err)
			}
			if len(parseResult.Statements) != 1 {
				t.Fatalf("Expected 1 statement, got %d", len(parseResult.Statements))
			}

			// Extract metadata
			extractor := NewExtractor()
			metadata := extractor.Extract(parseResult.Statements[0].AST.Stmts[0].Stmt, tt.operation)

			// Check expected fields
			for key, expectedValue := range tt.expectedMetadata {
				actualValue, exists := metadata[key]
				if !exists {
					t.Errorf("Missing metadata field %q", key)
					continue
				}

				// Convert both to strings for comparison
				expectedStr := toString(expectedValue)
				actualStr := toString(actualValue)

				if actualStr != expectedStr {
					t.Errorf("Metadata field %q: got %q, want %q", key, actualStr, expectedStr)
				}
			}

			// Check for unexpected fields (except idColumn which we know might be guessed)
			for key := range metadata {
				if _, expected := tt.expectedMetadata[key]; !expected && key != "idColumn" {
					if !tt.shouldHaveIDColumn || key != "idColumn" {
						t.Errorf("Unexpected metadata field %q with value %v", key, metadata[key])
					}
				}
			}
		})
	}
}

// Helper function to convert values to strings for comparison
func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case []string:
		if len(val) == 1 {
			return val[0]
		}
		return ""
	default:
		return ""
	}
}
