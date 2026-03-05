package analyzer

import (
	"sort"
	"testing"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// parseFirstStmtNode parses SQL and returns the first statement's AST node
func parseFirstStmtNode(t *testing.T, sql string) *pg_query.Node {
	t.Helper()
	result, err := pg_query.Parse(sql)
	if err != nil {
		t.Fatalf("Failed to parse SQL %q: %v", sql, err)
	}
	if len(result.Stmts) == 0 {
		t.Fatalf("No statements parsed from %q", sql)
	}
	return result.Stmts[0].Stmt
}

func TestExtractTables(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		expected []string
	}{
		{
			name:     "simple SELECT",
			sql:      "SELECT * FROM users",
			expected: []string{"users"},
		},
		{
			name:     "SELECT with JOIN",
			sql:      "SELECT * FROM users JOIN orders ON users.id = orders.user_id",
			expected: []string{"orders", "users"},
		},
		{
			name:     "UPDATE single table",
			sql:      "UPDATE users SET active = true",
			expected: []string{"users"},
		},
		{
			name:     "UPDATE with FROM",
			sql:      "UPDATE users SET active = true FROM accounts WHERE users.account_id = accounts.id",
			expected: []string{"accounts", "users"},
		},
		{
			name:     "DELETE single table",
			sql:      "DELETE FROM users",
			expected: []string{"users"},
		},
		{
			name:     "DELETE with USING",
			sql:      "DELETE FROM users USING inactive_users WHERE users.id = inactive_users.id",
			expected: []string{"inactive_users", "users"},
		},
		{
			name:     "INSERT simple",
			sql:      "INSERT INTO users (name) VALUES ('test')",
			expected: []string{"users"},
		},
		{
			name:     "INSERT SELECT",
			sql:      "INSERT INTO users_archive SELECT * FROM users WHERE deleted = true",
			expected: []string{"users", "users_archive"},
		},
		{
			name:     "schema-qualified table",
			sql:      "SELECT * FROM public.users",
			expected: []string{"public.users"},
		},
		{
			name:     "subquery",
			sql:      "SELECT * FROM (SELECT id FROM users) sub",
			expected: []string{"users"},
		},
		{
			name:     "CREATE INDEX",
			sql:      "CREATE INDEX idx_users_email ON users(email)",
			expected: []string{"users"},
		},
		{
			name:     "DROP TABLE",
			sql:      "DROP TABLE users",
			expected: []string{"users"},
		},
		{
			name:     "TRUNCATE",
			sql:      "TRUNCATE users",
			expected: []string{"users"},
		},
		{
			name:     "ALTER TABLE",
			sql:      "ALTER TABLE users ADD COLUMN email TEXT",
			expected: []string{"users"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := parseFirstStmtNode(t, tt.sql)
			tables := extractTables(node)
			sort.Strings(tables)
			sort.Strings(tt.expected)

			if len(tables) != len(tt.expected) {
				t.Fatalf("extractTables(%q) = %v, want %v", tt.sql, tables, tt.expected)
			}
			for i, table := range tables {
				if table != tt.expected[i] {
					t.Errorf("extractTables(%q)[%d] = %q, want %q", tt.sql, i, table, tt.expected[i])
				}
			}
		})
	}
}

func TestExtractTablesNilInput(t *testing.T) {
	tables := extractTables(nil)
	if tables != nil {
		t.Errorf("extractTables(nil) = %v, want nil", tables)
	}
}

func TestExtractTablesWithContext(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		expected map[string]LockType
	}{
		{
			name:     "SELECT gets AccessShare",
			sql:      "SELECT * FROM users",
			expected: map[string]LockType{"users": AccessShare},
		},
		{
			name:     "UPDATE gets RowExclusive",
			sql:      "UPDATE users SET active = true",
			expected: map[string]LockType{"users": RowExclusive},
		},
		{
			name:     "DELETE gets RowExclusive",
			sql:      "DELETE FROM users",
			expected: map[string]LockType{"users": RowExclusive},
		},
		{
			name: "INSERT SELECT - target RowExclusive, source AccessShare",
			sql:  "INSERT INTO users_archive SELECT * FROM users",
			expected: map[string]LockType{
				"users_archive": RowExclusive,
				"users":         AccessShare,
			},
		},
		{
			name: "UPDATE with FROM - target RowExclusive, source AccessShare",
			sql:  "UPDATE users SET active = true FROM accounts WHERE users.account_id = accounts.id",
			expected: map[string]LockType{
				"users":    RowExclusive,
				"accounts": AccessShare,
			},
		},
		{
			name: "DELETE with USING - target RowExclusive, source AccessShare",
			sql:  "DELETE FROM users USING inactive_users WHERE users.id = inactive_users.id",
			expected: map[string]LockType{
				"users":          RowExclusive,
				"inactive_users": AccessShare,
			},
		},
		{
			name: "MERGE - target RowExclusive, source AccessShare",
			sql:  "MERGE INTO users USING new_users ON users.id = new_users.id WHEN MATCHED THEN UPDATE SET name = new_users.name WHEN NOT MATCHED THEN INSERT (id, name) VALUES (new_users.id, new_users.name)",
			expected: map[string]LockType{
				"users":     RowExclusive,
				"new_users": AccessShare,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := parseFirstStmtNode(t, tt.sql)
			result := extractTablesWithContext(node)

			if len(result) != len(tt.expected) {
				t.Fatalf("extractTablesWithContext(%q) = %v, want %v", tt.sql, result, tt.expected)
			}
			for table, expectedLock := range tt.expected {
				if gotLock, ok := result[table]; !ok {
					t.Errorf("extractTablesWithContext(%q): missing table %q", tt.sql, table)
				} else if gotLock != expectedLock {
					t.Errorf("extractTablesWithContext(%q)[%q] = %v, want %v", tt.sql, table, gotLock, expectedLock)
				}
			}
		})
	}
}

func TestExtractTablesWithContextNilInput(t *testing.T) {
	result := extractTablesWithContext(nil)
	if result != nil {
		t.Errorf("extractTablesWithContext(nil) = %v, want nil", result)
	}
}
