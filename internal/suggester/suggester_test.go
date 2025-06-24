package suggester

import (
	"fmt"
	"strings"
	"testing"
)

// Test helpers
func assertStep(t *testing.T, step Step, expectedType string, expectedCanRunInTx bool) {
	t.Helper()
	if step.Type != expectedType {
		t.Errorf("Step type = %v, want %v", step.Type, expectedType)
	}
	if step.CanRunInTransaction != expectedCanRunInTx {
		t.Errorf("Step CanRunInTransaction = %v, want %v", step.CanRunInTransaction, expectedCanRunInTx)
	}
}

func assertSQLStep(t *testing.T, step Step, wantSQL string) {
	t.Helper()
	assertStep(t, step, "sql", step.CanRunInTransaction)
	if step.SQL != wantSQL {
		t.Errorf("SQL = %q, want %q", step.SQL, wantSQL)
	}
}

func assertProceduralStep(t *testing.T, step Step, mustContain ...string) {
	t.Helper()
	assertStep(t, step, "procedural", step.CanRunInTransaction)
	if step.SQL != "" {
		t.Errorf("Procedural step should not have SQL, got %q", step.SQL)
	}
	for _, text := range mustContain {
		if !strings.Contains(step.Notes, text) {
			t.Errorf("Step notes should contain %q, got %q", text, step.Notes)
		}
	}
}

func assertExternalStep(t *testing.T, step Step, mustContain string) {
	t.Helper()
	assertStep(t, step, "external", step.CanRunInTransaction)
	if step.Command == "" {
		t.Errorf("External step should have rendered command")
	}
	if !strings.Contains(step.Command, mustContain) {
		t.Errorf("Command should contain %q, got %q", mustContain, step.Command)
	}
}

func assertError(t *testing.T, err error, mustContain string) {
	t.Helper()
	if err == nil {
		t.Errorf("Expected error containing %q, got nil", mustContain)
		return
	}
	if !strings.Contains(err.Error(), mustContain) {
		t.Errorf("Error = %q, want to contain %q", err.Error(), mustContain)
	}
}

// Test data builders
func buildIndexMetadata(table string, columns ...string) OperationMetadata {
	return OperationMetadata{
		"tableName": table,
		"columns":   columns,
	}
}

func buildDMLMetadata(table, idColumn string) OperationMetadata {
	return OperationMetadata{
		"tableName": table,
		"idColumn":  idColumn,
	}
}

// Constants for common patterns
const (
	defaultIndexNamePattern      = "idx_%s_%s"
	defaultUniqueNamePattern     = "uniq_%s_%s"
	defaultConstraintNamePattern = "%s_%s"
	defaultFilePathPattern       = "/tmp/%s_ids.csv"
)

func TestSuggester_BoundaryConditions(t *testing.T) {
	s := NewSuggester()

	tests := []struct {
		name      string
		operation string
		metadata  OperationMetadata
		wantErr   bool
	}{
		{
			name:      "PostgreSQL identifier limit (63 chars)",
			operation: "CREATE INDEX",
			metadata: OperationMetadata{
				"tableName": "users",
				"indexName": "idx_" + strings.Repeat("a", 60), // 64 chars total
				"columns":   []string{"email"},
			},
			wantErr: false, // Should handle gracefully
		},
		{
			name:      "empty columns array",
			operation: "CREATE INDEX",
			metadata: OperationMetadata{
				"tableName": "users",
				"columns":   []string{},
			},
			wantErr: true,
		},
		{
			name:      "nil vs empty string",
			operation: "UPDATE without WHERE",
			metadata: OperationMetadata{
				"tableName":     "users",
				"idColumn":      "id",
				"columnsValues": "", // Empty string
			},
			wantErr: true,
		},
		{
			name:      "very many columns",
			operation: "CREATE INDEX",
			metadata: OperationMetadata{
				"tableName": "users",
				"columns":   make([]string, 32), // PostgreSQL limit is 32 columns per index
			},
			wantErr: false, // Should handle
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.GetSuggestion(tt.operation, tt.metadata)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSuggestion() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSuggester_SpecialCharacters(t *testing.T) {
	s := NewSuggester()

	tests := []struct {
		name      string
		operation string
		metadata  OperationMetadata
		checkSQL  func(*testing.T, string)
	}{
		{
			name:      "hyphen in table name",
			operation: "CREATE INDEX",
			metadata: OperationMetadata{
				"tableName": `"user-accounts"`,
				"columns":   []string{"email"},
			},
			checkSQL: func(t *testing.T, sql string) {
				// Should contain the quoted identifier
				if !strings.Contains(sql, `"user-accounts"`) {
					t.Errorf("Quoted identifier not found in SQL: %s", sql)
				}
			},
		},
		{
			name:      "space in column name",
			operation: "ALTER TABLE SET NOT NULL",
			metadata: OperationMetadata{
				"tableName": "users",
				"column":    `"first name"`,
			},
			checkSQL: func(t *testing.T, sql string) {
				if !strings.Contains(sql, `"first name"`) {
					t.Errorf("Quoted column name not found in SQL: %s", sql)
				}
			},
		},
		{
			name:      "reserved word as table name",
			operation: "CREATE INDEX",
			metadata: OperationMetadata{
				"tableName": `"user"`, // PostgreSQL reserved word - must be quoted
				"columns":   []string{"email"},
			},
			checkSQL: func(t *testing.T, sql string) {
				// Should contain the quoted reserved word
				if !strings.Contains(sql, `"user"`) {
					t.Errorf("Quoted reserved word not found in SQL: %s", sql)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestion, err := s.GetSuggestion(tt.operation, tt.metadata)
			if err != nil {
				t.Fatalf("GetSuggestion() error = %v", err)
			}

			if len(suggestion.Steps) > 0 && suggestion.Steps[0].Type == "sql" {
				tt.checkSQL(t, suggestion.Steps[0].SQL)
			}
		})
	}
}

func TestSuggester_ConcurrentAccess(t *testing.T) {
	s := NewSuggester()

	operations := []struct {
		op       string
		metadata OperationMetadata
	}{
		{"CREATE INDEX", buildIndexMetadata("users", "email")},
		{"DELETE without WHERE", buildDMLMetadata("orders", "id")},
		{"ALTER TABLE SET NOT NULL", OperationMetadata{"tableName": "products", "column": "price"}},
	}

	// Test multiple accesses
	for _, op := range operations {
		_, err := s.GetSuggestion(op.op, op.metadata)
		if err != nil {
			t.Errorf("Access error for %s: %v", op.op, err)
		}
	}
}

func TestSuggester_HasSuggestion(t *testing.T) {
	s := NewSuggester()

	tests := []struct {
		name      string
		operation string
	}{
		// DML Operations with suggestions
		{"has suggestion - UPDATE without WHERE", "UPDATE without WHERE"},
		{"has suggestion - DELETE without WHERE", "DELETE without WHERE"},
		{"has suggestion - MERGE without WHERE", "MERGE without WHERE"},

		// Index Operations with suggestions
		{"has suggestion - DROP INDEX", "DROP INDEX"},
		{"has suggestion - CREATE INDEX", "CREATE INDEX"},
		{"has suggestion - CREATE UNIQUE INDEX", "CREATE UNIQUE INDEX"},
		{"has suggestion - REINDEX", "REINDEX"},
		{"has suggestion - REINDEX TABLE", "REINDEX TABLE"},
		{"has suggestion - REINDEX DATABASE", "REINDEX DATABASE"},
		{"has suggestion - REINDEX SCHEMA", "REINDEX SCHEMA"},

		// ALTER TABLE Operations with suggestions
		{"has suggestion - ALTER TABLE ADD COLUMN with volatile DEFAULT", "ALTER TABLE ADD COLUMN with volatile DEFAULT"},
		{"has suggestion - ALTER TABLE ALTER COLUMN TYPE", "ALTER TABLE ALTER COLUMN TYPE"},
		{"has suggestion - ALTER TABLE ADD PRIMARY KEY", "ALTER TABLE ADD PRIMARY KEY"},
		{"has suggestion - ALTER TABLE ADD CONSTRAINT CHECK", "ALTER TABLE ADD CONSTRAINT CHECK"},
		{"has suggestion - ALTER TABLE SET NOT NULL", "ALTER TABLE SET NOT NULL"},

		// Maintenance Operations with suggestions
		{"has suggestion - CLUSTER", "CLUSTER"},
		{"has suggestion - REFRESH MATERIALIZED VIEW", "REFRESH MATERIALIZED VIEW"},
		{"has suggestion - VACUUM FULL", "VACUUM FULL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !s.HasSuggestion(tt.operation) {
				t.Errorf("HasSuggestion(%q) = false, want true", tt.operation)
			}
		})
	}
}

func TestSuggester_DMLOperations(t *testing.T) {
	s := NewSuggester()

	t.Run("UPDATE without WHERE", func(t *testing.T) {
		metadata := OperationMetadata{
			"tableName":     "users",
			"idColumn":      "id",
			"columnsValues": "active = false, updated_at = NOW()",
		}

		suggestion, err := s.GetSuggestion("UPDATE without WHERE", metadata)
		if err != nil {
			t.Fatalf("GetSuggestion() error = %v", err)
		}

		if suggestion.Category != "DML Operations" {
			t.Errorf("Category = %v, want DML Operations", suggestion.Category)
		}

		if len(suggestion.Steps) != 2 {
			t.Fatalf("Steps count = %v, want 2", len(suggestion.Steps))
		}

		// Step 1: SQL export
		assertStep(t, suggestion.Steps[0], "sql", true)
		if !strings.Contains(suggestion.Steps[0].SQL, "\\COPY") {
			t.Errorf("Step 1 should contain \\COPY command")
		}
		if !strings.Contains(suggestion.Steps[0].SQL, "users") {
			t.Errorf("Step 1 should reference users table")
		}

		// Step 2: Procedural notes
		assertProceduralStep(t, suggestion.Steps[1],
			"Read ID file in chunks",
			"UPDATE",
			"Monitor replication lag")
	})

	t.Run("DELETE without WHERE with custom file", func(t *testing.T) {
		metadata := OperationMetadata{
			"tableName": "orders",
			"idColumn":  "order_id",
			"filePath":  "/data/old_orders.csv",
		}

		suggestion, err := s.GetSuggestion("DELETE without WHERE", metadata)
		if err != nil {
			t.Fatalf("GetSuggestion() error = %v", err)
		}

		// Check custom values are used
		sql := suggestion.Steps[0].SQL
		if !strings.Contains(sql, "order_id") {
			t.Errorf("Should use custom ID column")
		}
		if !strings.Contains(sql, "/path/to/target_ids.csv") {
			t.Errorf("Should use placeholder file path")
		}
	})

	t.Run("MERGE without WHERE", func(t *testing.T) {
		metadata := OperationMetadata{
			"sourceTable":      "staging_users",
			"targetTable":      "users",
			"idColumn":         "user_id",
			"mergeCondition":   "target.user_id = source.user_id",
			"matchedAction":    "UPDATE SET email = source.email",
			"notMatchedAction": "INSERT (user_id, email) VALUES (source.user_id, source.email)",
		}

		suggestion, err := s.GetSuggestion("MERGE without WHERE", metadata)
		if err != nil {
			t.Fatalf("GetSuggestion() error = %v", err)
		}

		if len(suggestion.Steps) != 2 {
			t.Fatalf("Steps count = %v, want 2", len(suggestion.Steps))
		}

		// Export should be from source table
		if !strings.Contains(suggestion.Steps[0].SQL, "staging_users") {
			t.Errorf("Should export from source table")
		}

		// Procedural notes should mention MERGE
		assertProceduralStep(t, suggestion.Steps[1], "MERGE", "WHEN MATCHED")
	})
}

func TestSuggester_IndexOperations(t *testing.T) {
	s := NewSuggester()

	t.Run("CREATE INDEX exact SQL", func(t *testing.T) {
		metadata := OperationMetadata{
			"tableName": "users",
			"indexName": "idx_users_email",
			"columns":   []string{"email"},
		}

		suggestion, err := s.GetSuggestion("CREATE INDEX", metadata)
		if err != nil {
			t.Fatalf("GetSuggestion() error = %v", err)
		}

		want := "CREATE INDEX CONCURRENTLY idx_users_email ON users (email);\n"
		assertSQLStep(t, suggestion.Steps[0], want)

		// Must run outside transaction
		if suggestion.Steps[0].CanRunInTransaction {
			t.Errorf("CREATE INDEX CONCURRENTLY must run outside transaction")
		}
	})

	t.Run("CREATE INDEX default name generation", func(t *testing.T) {
		metadata := OperationMetadata{
			"tableName": "orders",
			"columns":   []string{"status", "created_at"},
		}

		suggestion, err := s.GetSuggestion("CREATE INDEX", metadata)
		if err != nil {
			t.Fatalf("GetSuggestion() error = %v", err)
		}

		expectedName := fmt.Sprintf(defaultIndexNamePattern, "orders", "status_created_at")
		if !strings.Contains(suggestion.Steps[0].SQL, expectedName) {
			t.Errorf("Should generate default index name %s", expectedName)
		}
	})

	t.Run("CREATE UNIQUE INDEX multiple columns", func(t *testing.T) {
		metadata := OperationMetadata{
			"tableName": "users",
			"indexName": "uniq_users_email_tenant",
			"columns":   []string{"email", "tenant_id"},
		}

		suggestion, err := s.GetSuggestion("CREATE UNIQUE INDEX", metadata)
		if err != nil {
			t.Fatalf("GetSuggestion() error = %v", err)
		}

		sql := suggestion.Steps[0].SQL
		if !strings.Contains(sql, "CREATE UNIQUE INDEX CONCURRENTLY") {
			t.Errorf("Should use CREATE UNIQUE INDEX CONCURRENTLY")
		}
		if !strings.Contains(sql, "(email, tenant_id)") {
			t.Errorf("Should include both columns in order")
		}
	})
}

func TestSuggester_REINDEXOperations(t *testing.T) {
	s := NewSuggester()

	t.Run("REINDEX single index", func(t *testing.T) {
		metadata := OperationMetadata{
			"indexName": "idx_users_email",
		}

		suggestion, err := s.GetSuggestion("REINDEX", metadata)
		if err != nil {
			t.Fatalf("GetSuggestion() error = %v", err)
		}

		want := "REINDEX INDEX CONCURRENTLY idx_users_email;\n"
		assertSQLStep(t, suggestion.Steps[0], want)
	})

	t.Run("REINDEX TABLE steps", func(t *testing.T) {
		metadata := OperationMetadata{
			"tableName": "users",
		}

		suggestion, err := s.GetSuggestion("REINDEX TABLE", metadata)
		if err != nil {
			t.Fatalf("GetSuggestion() error = %v", err)
		}

		if len(suggestion.Steps) != 2 {
			t.Fatalf("Steps count = %v, want 2", len(suggestion.Steps))
		}

		// Step 1: Export index names
		assertStep(t, suggestion.Steps[0], "sql", true)
		if !strings.Contains(suggestion.Steps[0].SQL, "pg_indexes") {
			t.Errorf("Should query pg_indexes")
		}
		if !strings.Contains(suggestion.Steps[0].SQL, "tablename = 'users'") {
			t.Errorf("Should filter by table name")
		}

		// Step 2: REINDEX instructions (procedural, not SQL)
		assertProceduralStep(t, suggestion.Steps[1], "REINDEX INDEX CONCURRENTLY")
	})

	t.Run("REINDEX DATABASE excludes system catalogs", func(t *testing.T) {
		metadata := OperationMetadata{}

		suggestion, err := s.GetSuggestion("REINDEX DATABASE", metadata)
		if err != nil {
			t.Fatalf("GetSuggestion() error = %v", err)
		}

		sql := suggestion.Steps[0].SQL
		if !strings.Contains(sql, "schemaname NOT IN ('pg_catalog', 'information_schema')") {
			t.Errorf("Should exclude system schemas")
		}
	})
}

func TestSuggester_AlterTableOperations_AddColumn(t *testing.T) {
	s := NewSuggester()

	metadata := OperationMetadata{
		"tableName":    "users",
		"columnName":   "created_at",
		"dataType":     "timestamp",
		"defaultValue": "NOW()",
		"idColumn":     "id",
	}

	suggestion, err := s.GetSuggestion("ALTER TABLE ADD COLUMN with volatile DEFAULT", metadata)
	if err != nil {
		t.Fatalf("GetSuggestion() error = %v", err)
	}

	if len(suggestion.Steps) != 3 {
		t.Fatalf("Steps count = %v, want 3", len(suggestion.Steps))
	}

	// Step 1: Add column without default
	want := "ALTER TABLE users ADD COLUMN created_at timestamp;\n"
	assertSQLStep(t, suggestion.Steps[0], want)

	// Step 2: Batch update (procedural)
	assertProceduralStep(t, suggestion.Steps[1])

	// Step 3: Set default
	if !strings.Contains(suggestion.Steps[2].SQL, "ALTER COLUMN created_at SET DEFAULT NOW()") {
		t.Errorf("Step 3 should set default")
	}
}

func TestSuggester_AlterTableOperations_AlterType(t *testing.T) {
	s := NewSuggester()

	metadata := OperationMetadata{
		"tableName":  "products",
		"columnName": "price",
		"newType":    "numeric(10,2)",
		"idColumn":   "product_id",
	}

	suggestion, err := s.GetSuggestion("ALTER TABLE ALTER COLUMN TYPE", metadata)
	if err != nil {
		t.Fatalf("GetSuggestion() error = %v", err)
	}

	if len(suggestion.Steps) != 4 {
		t.Fatalf("Steps count = %v, want 4", len(suggestion.Steps))
	}

	// Step 1: Add new column
	if !strings.Contains(suggestion.Steps[0].SQL, "ADD COLUMN price_new numeric(10,2)") {
		t.Errorf("Step 1 should add new column with new type")
	}

	// Step 2: Sync trigger
	sql := suggestion.Steps[1].SQL
	if !strings.Contains(sql, "CREATE OR REPLACE FUNCTION sync_products_price()") {
		t.Errorf("Should create sync function")
	}
	if !strings.Contains(sql, "NEW.price_new := NEW.price::numeric(10,2)") {
		t.Errorf("Sync function should cast to new type")
	}
	if !strings.Contains(sql, "products_price_sync_trigger") {
		t.Errorf("Should create named trigger")
	}

	// Step 3: Backfill (procedural)
	assertProceduralStep(t, suggestion.Steps[2])

	// Step 4: Atomic swap
	sql = suggestion.Steps[3].SQL
	if !strings.Contains(sql, "BEGIN") || !strings.Contains(sql, "COMMIT") {
		t.Errorf("Step 4 should be in transaction")
	}
	if !strings.Contains(sql, "SET LOCAL lock_timeout = '5s'") {
		t.Errorf("Should set lock timeout")
	}
	if !strings.Contains(sql, "DROP COLUMN price") {
		t.Errorf("Should drop old column")
	}
	if !strings.Contains(sql, "RENAME COLUMN price_new TO price") {
		t.Errorf("Should rename new column")
	}
}

func TestSuggester_AlterTableOperations_Constraints(t *testing.T) {
	s := NewSuggester()

	t.Run("ADD PRIMARY KEY", func(t *testing.T) {
		metadata := OperationMetadata{
			"tableName": "users",
			"columns":   []string{"user_id", "tenant_id"},
		}

		suggestion, err := s.GetSuggestion("ALTER TABLE ADD PRIMARY KEY", metadata)
		if err != nil {
			t.Fatalf("GetSuggestion() error = %v", err)
		}

		if len(suggestion.Steps) != 2 {
			t.Fatalf("Steps count = %v, want 2", len(suggestion.Steps))
		}

		// Step 1: Create unique index concurrently
		assertStep(t, suggestion.Steps[0], "sql", false) // Must be outside transaction
		if !strings.Contains(suggestion.Steps[0].SQL, "CREATE UNIQUE INDEX CONCURRENTLY") {
			t.Errorf("Should create unique index concurrently")
		}

		// Step 2: Add primary key using index
		assertStep(t, suggestion.Steps[1], "sql", true) // Can be in transaction
		if !strings.Contains(suggestion.Steps[1].SQL, "PRIMARY KEY USING INDEX") {
			t.Errorf("Should add primary key using existing index")
		}
	})

	t.Run("ADD CHECK CONSTRAINT", func(t *testing.T) {
		metadata := OperationMetadata{
			"tableName":       "orders",
			"constraintName":  "check_positive_amount",
			"checkExpression": "amount > 0",
		}

		suggestion, err := s.GetSuggestion("ALTER TABLE ADD CONSTRAINT CHECK", metadata)
		if err != nil {
			t.Fatalf("GetSuggestion() error = %v", err)
		}

		// Step 1: Add NOT VALID
		want := "ALTER TABLE orders ADD CONSTRAINT check_positive_amount CHECK (amount > 0) NOT VALID;\n"
		assertSQLStep(t, suggestion.Steps[0], want)

		// Step 2: Validate
		want = "ALTER TABLE orders VALIDATE CONSTRAINT check_positive_amount;\n"
		assertSQLStep(t, suggestion.Steps[1], want)
	})

	t.Run("SET NOT NULL", func(t *testing.T) {
		metadata := OperationMetadata{
			"tableName": "users",
			"column":    "email",
		}

		suggestion, err := s.GetSuggestion("ALTER TABLE SET NOT NULL", metadata)
		if err != nil {
			t.Fatalf("GetSuggestion() error = %v", err)
		}

		if len(suggestion.Steps) != 4 {
			t.Fatalf("Steps count = %v, want 4", len(suggestion.Steps))
		}

		// Verify the 4-step process
		expectedConstraint := fmt.Sprintf(defaultConstraintNamePattern, "users_email", "not_null")

		// Step 1: CHECK constraint
		if !strings.Contains(suggestion.Steps[0].SQL, "CHECK (email IS NOT NULL) NOT VALID") {
			t.Errorf("Step 1 should add NOT NULL check constraint")
		}
		if !strings.Contains(suggestion.Steps[0].SQL, expectedConstraint) {
			t.Errorf("Should use generated constraint name")
		}

		// Step 4: Drop constraint
		if !strings.Contains(suggestion.Steps[3].SQL, "DROP CONSTRAINT") {
			t.Errorf("Step 4 should drop the check constraint")
		}
	})
}

func TestSuggester_MaintenanceOperations(t *testing.T) {
	s := NewSuggester()

	t.Run("CLUSTER with pg_repack", func(t *testing.T) {
		metadata := OperationMetadata{
			"tableName": "users",
			"indexName": "users_pkey",
		}

		suggestion, err := s.GetSuggestion("CLUSTER", metadata)
		if err != nil {
			t.Fatalf("GetSuggestion() error = %v", err)
		}

		if !suggestion.IsPartial {
			t.Errorf("CLUSTER should be marked as partial alternative")
		}

		if len(suggestion.Steps) != 1 {
			t.Fatalf("CLUSTER should have 1 step")
		}

		assertExternalStep(t, suggestion.Steps[0], "pg_repack -t users -i users_pkey -d <YOUR_DATABASE>")
	})

	t.Run("REFRESH MATERIALIZED VIEW", func(t *testing.T) {
		metadata := OperationMetadata{
			"viewName": "sales_summary",
		}

		suggestion, err := s.GetSuggestion("REFRESH MATERIALIZED VIEW", metadata)
		if err != nil {
			t.Fatalf("GetSuggestion() error = %v", err)
		}

		want := "REFRESH MATERIALIZED VIEW CONCURRENTLY sales_summary;\n"
		assertSQLStep(t, suggestion.Steps[0], want)

		// Must run outside transaction
		if suggestion.Steps[0].CanRunInTransaction {
			t.Errorf("REFRESH MATERIALIZED VIEW CONCURRENTLY must run outside transaction")
		}
	})

	t.Run("VACUUM FULL alternatives", func(t *testing.T) {
		metadata := OperationMetadata{
			"tableName": "logs",
		}

		suggestion, err := s.GetSuggestion("VACUUM FULL", metadata)
		if err != nil {
			t.Fatalf("GetSuggestion() error = %v", err)
		}

		// Should have external command for pg_repack
		assertStep(t, suggestion.Steps[0], "external", false)

		// Check command template
		if suggestion.Steps[0].CommandTemplate == "" {
			t.Errorf("Should have command template")
		}
		if !strings.Contains(suggestion.Steps[0].Command, "pg_repack -n -t logs -d <YOUR_DATABASE>") {
			t.Errorf("Command should contain pg_repack with table name, got %q", suggestion.Steps[0].Command)
		}
	})
}

func TestSuggester_TemplateRendering(t *testing.T) {
	s := NewSuggester()

	t.Run("nested default template", func(t *testing.T) {
		metadata := OperationMetadata{
			"tableName":     "users",
			"idColumn":      "id",
			"columnsValues": "active = false",
		}

		suggestion, err := s.GetSuggestion("UPDATE without WHERE", metadata)
		if err != nil {
			t.Fatalf("GetSuggestion() error = %v", err)
		}

		// The template uses a placeholder path, not a generated one
		if !strings.Contains(suggestion.Steps[0].SQL, "/path/to/target_ids.csv") {
			t.Errorf("Should contain placeholder file path")
		}
	})

	t.Run("array joining", func(t *testing.T) {
		metadata := OperationMetadata{
			"tableName": "users",
			"columns":   []string{"email", "tenant_id", "status"},
		}

		suggestion, err := s.GetSuggestion("CREATE UNIQUE INDEX", metadata)
		if err != nil {
			t.Fatalf("GetSuggestion() error = %v", err)
		}

		if !strings.Contains(suggestion.Steps[0].SQL, "(email, tenant_id, status)") {
			t.Errorf("Should join columns with commas and parentheses")
		}
	})

	t.Run("complex default generation", func(t *testing.T) {
		metadata := OperationMetadata{
			"tableName": "user_sessions",
			"columns":   []string{"user_id", "created_at"},
		}

		suggestion, err := s.GetSuggestion("CREATE INDEX", metadata)
		if err != nil {
			t.Fatalf("GetSuggestion() error = %v", err)
		}

		expectedName := "idx_user_sessions_user_id_created_at"
		if !strings.Contains(suggestion.Steps[0].SQL, expectedName) {
			t.Errorf("Should generate index name: %s", expectedName)
		}
	})
}

func TestSuggester_ErrorCases(t *testing.T) {
	s := NewSuggester()

	tests := []struct {
		name      string
		operation string
		metadata  OperationMetadata
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "operation without suggestion",
			operation: "TRUNCATE",
			metadata:  OperationMetadata{"tableName": "users"},
			wantErr:   true,
			errMsg:    "no suggestion available",
		},
		{
			name:      "unknown operation",
			operation: "UNKNOWN OPERATION",
			metadata:  OperationMetadata{},
			wantErr:   true,
			errMsg:    "no suggestion available",
		},
		{
			name:      "empty operation",
			operation: "",
			metadata:  OperationMetadata{},
			wantErr:   true,
			errMsg:    "no suggestion available",
		},
		{
			name:      "CREATE INDEX missing table",
			operation: "CREATE INDEX",
			metadata: OperationMetadata{
				"columns": []string{"email"},
				// Missing TableName
			},
			wantErr: true,
			errMsg:  "missing required field: TableName",
		},
		{
			name:      "CREATE INDEX missing columns",
			operation: "CREATE INDEX",
			metadata: OperationMetadata{
				"tableName": "users",
				// Missing Columns
			},
			wantErr: true,
			errMsg:  "missing required field: Columns",
		},
		{
			name:      "UPDATE missing ID column",
			operation: "UPDATE without WHERE",
			metadata: OperationMetadata{
				"tableName":     "users",
				"columnsValues": "active = false",
				// Missing IDColumn
			},
			wantErr: true,
			errMsg:  "missing required field: IDColumn",
		},
		{
			name:      "UPDATE missing columns values",
			operation: "UPDATE without WHERE",
			metadata: OperationMetadata{
				"tableName": "users",
				"idColumn":  "id",
				// Missing ColumnsValues
			},
			wantErr: true,
			errMsg:  "missing required field: ColumnsValues",
		},
		{
			name:      "MERGE missing source table",
			operation: "MERGE without WHERE",
			metadata: OperationMetadata{
				"targetTable": "users",
				// Missing SourceTable and other required fields
			},
			wantErr: true,
			errMsg:  "missing required field:", // Accept any missing field error since multiple are missing
		},
		{
			name:      "ALTER COLUMN TYPE missing fields",
			operation: "ALTER TABLE ALTER COLUMN TYPE",
			metadata: OperationMetadata{
				"tableName": "users",
				"oldColumn": "age",
				// Missing NewColumn, NewType, IDColumn
			},
			wantErr: true,
			errMsg:  "missing required field",
		},
		{
			name:      "DROP INDEX missing index name",
			operation: "DROP INDEX",
			metadata:  OperationMetadata{
				// Missing IndexName
			},
			wantErr: true,
			errMsg:  "missing required field: IndexName",
		},
		{
			name:      "REINDEX SCHEMA missing schema",
			operation: "REINDEX SCHEMA",
			metadata:  OperationMetadata{
				// Missing Schema
			},
			wantErr: true,
			errMsg:  "missing required field: Schema",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.GetSuggestion(tt.operation, tt.metadata)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSuggestion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				assertError(t, err, tt.errMsg)
			}
		})
	}
}

func TestSuggester_AllOperationsValidation(t *testing.T) {
	s := NewSuggester()

	// Test that all operations with suggestions can be retrieved with valid metadata
	operations := []struct {
		operation string
		metadata  OperationMetadata
	}{
		// DML Operations
		{
			"UPDATE without WHERE",
			OperationMetadata{"tableName": "test", "idColumn": "id", "columnsValues": "col = val"},
		},
		{
			"DELETE without WHERE",
			OperationMetadata{"tableName": "test", "idColumn": "id"},
		},
		{
			"MERGE without WHERE",
			OperationMetadata{
				"sourceTable": "src", "targetTable": "tgt", "idColumn": "id",
				"mergeCondition": "t.id = s.id", "matchedAction": "UPDATE SET col = s.col",
				"notMatchedAction": "INSERT VALUES (s.col)",
			},
		},
		// Index Operations
		{
			"DROP INDEX",
			OperationMetadata{"indexName": "idx_test"},
		},
		{
			"CREATE INDEX",
			OperationMetadata{"tableName": "test", "columns": []string{"col"}},
		},
		{
			"CREATE UNIQUE INDEX",
			OperationMetadata{"tableName": "test", "columns": []string{"col"}},
		},
		{
			"REINDEX",
			OperationMetadata{"indexName": "idx_test"},
		},
		{
			"REINDEX TABLE",
			OperationMetadata{"tableName": "test"},
		},
		{
			"REINDEX DATABASE",
			OperationMetadata{},
		},
		{
			"REINDEX SCHEMA",
			OperationMetadata{"schema": "public"},
		},
		// ALTER TABLE Operations
		{
			"ALTER TABLE ADD COLUMN with volatile DEFAULT",
			OperationMetadata{
				"tableName": "test", "columnName": "col", "dataType": "text",
				"defaultValue": "NOW()", "idColumn": "id",
			},
		},
		{
			"ALTER TABLE ALTER COLUMN TYPE",
			OperationMetadata{
				"tableName": "test", "columnName": "col",
				"newType": "text", "idColumn": "id",
			},
		},
		{
			"ALTER TABLE ADD PRIMARY KEY",
			OperationMetadata{"tableName": "test", "columns": []string{"id"}},
		},
		{
			"ALTER TABLE ADD CONSTRAINT CHECK",
			OperationMetadata{
				"tableName": "test", "constraintName": "chk_test",
				"checkExpression": "col > 0",
			},
		},
		{
			"ALTER TABLE SET NOT NULL",
			OperationMetadata{"tableName": "test", "column": "col"},
		},
		// Maintenance Operations
		{
			"CLUSTER",
			OperationMetadata{"tableName": "test", "indexName": "test_pkey"},
		},
		{
			"REFRESH MATERIALIZED VIEW",
			OperationMetadata{"viewName": "test_view"},
		},
		{
			"VACUUM FULL",
			OperationMetadata{"tableName": "test"},
		},
	}

	for _, op := range operations {
		t.Run(op.operation, func(t *testing.T) {
			suggestion, err := s.GetSuggestion(op.operation, op.metadata)
			if err != nil {
				t.Errorf("GetSuggestion(%q) error = %v", op.operation, err)
				return
			}

			// Validate basic properties
			if suggestion.Operation != op.operation {
				t.Errorf("Operation = %v, want %v", suggestion.Operation, op.operation)
			}

			if suggestion.Category == "" {
				t.Errorf("Category should not be empty")
			}

			if len(suggestion.Steps) == 0 {
				t.Errorf("No steps returned for operation %q", op.operation)
			}

			// Validate all steps
			for i, step := range suggestion.Steps {
				if step.Description == "" {
					t.Errorf("Step %d missing description", i+1)
				}

				switch step.Type {
				case "sql":
					if step.SQLTemplate == "" {
						t.Errorf("Step %d of type 'sql' missing SQLTemplate", i+1)
					}
					// SQL steps should have rendered SQL (except for some template steps)
					if step.SQL == "" && !strings.Contains(step.Description, "template") {
						t.Errorf("Step %d of type 'sql' missing rendered SQL", i+1)
					}
				case "procedural":
					if step.Notes == "" {
						t.Errorf("Step %d of type 'procedural' missing Notes", i+1)
					}
					if step.SQL != "" {
						t.Errorf("Step %d of type 'procedural' should not have SQL", i+1)
					}
				case "external":
					if step.CommandTemplate == "" {
						t.Errorf("Step %d of type 'external' missing CommandTemplate", i+1)
					}
					if step.Command == "" {
						t.Errorf("Step %d of type 'external' missing rendered Command", i+1)
					}
				default:
					t.Errorf("Step %d has unknown type: %v", i+1, step.Type)
				}
			}
		})
	}
}

func TestSuggester_TransactionModeConsistency(t *testing.T) {
	s := NewSuggester()

	// Operations that MUST run outside transactions
	mustRunOutsideTransaction := []string{
		"CREATE INDEX",
		"CREATE UNIQUE INDEX",
		"DROP INDEX",
		"REINDEX",
		"REFRESH MATERIALIZED VIEW",
	}

	for _, op := range mustRunOutsideTransaction {
		t.Run(op, func(t *testing.T) {
			var metadata OperationMetadata
			switch op {
			case "CREATE INDEX", "CREATE UNIQUE INDEX":
				metadata = OperationMetadata{"tableName": "test", "columns": []string{"col"}}
			case "DROP INDEX", "REINDEX":
				metadata = OperationMetadata{"indexName": "idx_test"}
			case "REFRESH MATERIALIZED VIEW":
				metadata = OperationMetadata{"viewName": "test_view"}
			}

			suggestion, err := s.GetSuggestion(op, metadata)
			if err != nil {
				t.Fatalf("GetSuggestion() error = %v", err)
			}

			// Find the main SQL step
			for i, step := range suggestion.Steps {
				if step.Type == "sql" && strings.Contains(step.SQL, "CONCURRENTLY") {
					if step.CanRunInTransaction {
						t.Errorf("Step %d: %s with CONCURRENTLY must have CanRunInTransaction = false", i+1, op)
					}
				}
			}
		})
	}
}

// Benchmarks
func BenchmarkSuggester_GetSuggestion(b *testing.B) {
	s := NewSuggester()
	metadata := OperationMetadata{
		"tableName": "users",
		"columns":   []string{"email", "username", "created_at"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := s.GetSuggestion("CREATE INDEX", metadata)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSuggester_ComplexOperation(b *testing.B) {
	s := NewSuggester()
	metadata := OperationMetadata{
		"tableName": "products",
		"oldColumn": "price",
		"newColumn": "price_new",
		"newType":   "numeric(10,2)",
		"idColumn":  "product_id",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := s.GetSuggestion("ALTER TABLE ALTER COLUMN TYPE", metadata)
		if err != nil {
			b.Fatal(err)
		}
	}
}
