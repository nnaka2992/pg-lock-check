package analyzer

import (
	"strings"
	"testing"

	"github.com/nnaka2992/pg-lock-check/internal/parser"
)

// ===== 1. DML OPERATIONS =====

func TestAnalyzer_DML_Basic(t *testing.T) {
	tests := []struct {
		name             string
		sql              string
		mode             TransactionMode
		expectedSeverity Severity
		expectedOp       string
		expectedLocks    map[string]string
	}{
		// UPDATE variations
		{
			name:             "UPDATE without WHERE",
			sql:              "UPDATE users SET active = false",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "UPDATE without WHERE",
			expectedLocks:    map[string]string{"users": "RowExclusive"},
		},
		{
			name:             "UPDATE with WHERE",
			sql:              "UPDATE users SET active = false WHERE id = 1",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "UPDATE with WHERE",
			expectedLocks:    map[string]string{"users": "RowExclusive"},
		},
		{
			name:             "UPDATE with subquery",
			sql:              "UPDATE users SET active = false WHERE id IN (SELECT user_id FROM inactive_sessions)",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "UPDATE with WHERE",
			expectedLocks:    map[string]string{"users": "RowExclusive", "inactive_sessions": "AccessShare"},
		},
		{
			name:             "UPDATE with FROM clause (multi-table)",
			sql:              "UPDATE users u SET active = false FROM sessions s WHERE u.id = s.user_id AND s.expired = true",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "UPDATE with WHERE",
			expectedLocks:    map[string]string{"users": "RowExclusive", "sessions": "AccessShare"},
		},

		// DELETE variations
		{
			name:             "DELETE without WHERE",
			sql:              "DELETE FROM sessions",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "DELETE without WHERE",
			expectedLocks:    map[string]string{"sessions": "RowExclusive"},
		},
		{
			name:             "DELETE with WHERE",
			sql:              "DELETE FROM sessions WHERE expired = true",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "DELETE with WHERE",
			expectedLocks:    map[string]string{"sessions": "RowExclusive"},
		},
		{
			name:             "DELETE with USING (multi-table)",
			sql:              "DELETE FROM sessions USING users WHERE sessions.user_id = users.id AND users.inactive = true",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "DELETE with WHERE",
			expectedLocks:    map[string]string{"sessions": "RowExclusive", "users": "AccessShare"},
		},

		// MERGE operations
		{
			name:             "MERGE without WHERE",
			sql:              "MERGE INTO target USING source ON target.id = source.id WHEN MATCHED THEN UPDATE SET value = source.value",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "MERGE without WHERE",
			expectedLocks:    map[string]string{"target": "RowExclusive", "source": "AccessShare"},
		},
		{
			name:             "MERGE with WHERE",
			sql:              "MERGE INTO target USING source ON target.id = source.id WHEN MATCHED AND target.active THEN UPDATE SET value = source.value",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "MERGE with WHERE",
			expectedLocks:    map[string]string{"target": "RowExclusive", "source": "AccessShare"},
		},

		// INSERT variations
		{
			name:             "INSERT simple",
			sql:              "INSERT INTO users (name, email) VALUES ('John', 'john@example.com')",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "INSERT",
			expectedLocks:    map[string]string{"users": "RowExclusive"},
		},
		{
			name:             "INSERT ON CONFLICT",
			sql:              "INSERT INTO users (id, name) VALUES (1, 'John') ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "INSERT ON CONFLICT",
			expectedLocks:    map[string]string{"users": "RowExclusive"},
		},
		{
			name:             "INSERT SELECT from large table",
			sql:              "INSERT INTO archived_users SELECT * FROM users WHERE created_at < '2020-01-01'",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "INSERT SELECT",
			expectedLocks:    map[string]string{"archived_users": "RowExclusive", "users": "AccessShare"},
		},
		{
			name:             "INSERT RETURNING",
			sql:              "INSERT INTO users (name, email) VALUES ('John', 'john@example.com') RETURNING id",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "INSERT",
			expectedLocks:    map[string]string{"users": "RowExclusive"},
		},

		// COPY operations
		{
			name:             "COPY FROM",
			sql:              "COPY users FROM '/tmp/users.csv' CSV HEADER",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "COPY FROM",
			expectedLocks:    map[string]string{"users": "RowExclusive"},
		},
		{
			name:             "COPY TO",
			sql:              "COPY users TO '/tmp/users.csv' CSV HEADER",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "COPY TO",
			expectedLocks:    map[string]string{"users": "AccessShare"},
		},
	}

	runAnalyzerTests(t, tests)
}

func TestAnalyzer_DML_SelectLocking(t *testing.T) {
	tests := []struct {
		name             string
		sql              string
		mode             TransactionMode
		expectedSeverity Severity
		expectedOp       string
		expectedLocks    map[string]string
	}{
		// SELECT ... FOR UPDATE variations
		{
			name:             "SELECT FOR UPDATE without WHERE",
			sql:              "SELECT * FROM users FOR UPDATE",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "SELECT FOR UPDATE without WHERE",
			expectedLocks:    map[string]string{"users": "RowShare"},
		},
		{
			name:             "SELECT FOR UPDATE with WHERE",
			sql:              "SELECT * FROM users WHERE id = 1 FOR UPDATE",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "SELECT FOR UPDATE with WHERE",
			expectedLocks:    map[string]string{"users": "RowShare"},
		},

		// SELECT ... FOR NO KEY UPDATE
		{
			name:             "SELECT FOR NO KEY UPDATE without WHERE",
			sql:              "SELECT * FROM users FOR NO KEY UPDATE",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "SELECT FOR NO KEY UPDATE without WHERE",
			expectedLocks:    map[string]string{"users": "RowShare"},
		},
		{
			name:             "SELECT FOR NO KEY UPDATE with WHERE",
			sql:              "SELECT * FROM users WHERE id = 1 FOR NO KEY UPDATE",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "SELECT FOR NO KEY UPDATE with WHERE",
			expectedLocks:    map[string]string{"users": "RowShare"},
		},

		// SELECT ... FOR SHARE
		{
			name:             "SELECT FOR SHARE without WHERE",
			sql:              "SELECT * FROM users FOR SHARE",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "SELECT FOR SHARE without WHERE",
			expectedLocks:    map[string]string{"users": "RowShare"},
		},
		{
			name:             "SELECT FOR SHARE with WHERE",
			sql:              "SELECT * FROM users WHERE id = 1 FOR SHARE",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "SELECT FOR SHARE with WHERE",
			expectedLocks:    map[string]string{"users": "RowShare"},
		},

		// SELECT ... FOR KEY SHARE
		{
			name:             "SELECT FOR KEY SHARE",
			sql:              "SELECT * FROM users WHERE id = 1 FOR KEY SHARE",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "SELECT FOR KEY SHARE",
			expectedLocks:    map[string]string{"users": "RowShare"},
		},
	}

	runAnalyzerTests(t, tests)
}

// ===== 2. DDL OPERATIONS =====

func TestAnalyzer_DDL_Tables(t *testing.T) {
	tests := []struct {
		name             string
		sql              string
		mode             TransactionMode
		expectedSeverity Severity
		expectedOp       string
		expectedLocks    map[string]string
	}{
		// CREATE operations
		{
			name:             "CREATE TABLE",
			sql:              "CREATE TABLE users (id INT PRIMARY KEY, name TEXT)",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "CREATE TABLE",
			expectedLocks:    map[string]string{},
		},
		{
			name:             "CREATE TABLE AS",
			sql:              "CREATE TABLE archived_users AS SELECT * FROM users WHERE created_at < '2020-01-01'",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "CREATE TABLE AS",
			expectedLocks:    map[string]string{"users": "AccessShare"},
		},
		{
			name:             "CREATE TEMPORARY TABLE",
			sql:              "CREATE TEMPORARY TABLE temp_results (id INT, value TEXT)",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "CREATE TEMPORARY TABLE",
		},
		{
			name:             "SELECT INTO",
			sql:              "SELECT * INTO archived_users FROM users WHERE created_at < '2020-01-01'",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "SELECT INTO",
			expectedLocks:    map[string]string{"users": "AccessShare"},
		},

		// DROP operations
		{
			name:             "DROP TABLE",
			sql:              "DROP TABLE users",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "DROP TABLE",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},
		{
			name:             "DROP TABLE CASCADE",
			sql:              "DROP TABLE users CASCADE",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "DROP TABLE",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},

		// TRUNCATE
		{
			name:             "TRUNCATE single table",
			sql:              "TRUNCATE users",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "TRUNCATE",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},
		{
			name:             "TRUNCATE multiple tables",
			sql:              "TRUNCATE users, sessions, logs",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "TRUNCATE",
			expectedLocks:    map[string]string{"users": "AccessExclusive", "sessions": "AccessExclusive", "logs": "AccessExclusive"},
		},
	}

	runAnalyzerTests(t, tests)
}

func TestAnalyzer_DDL_AlterTable(t *testing.T) {
	tests := []struct {
		name             string
		sql              string
		mode             TransactionMode
		expectedSeverity Severity
		expectedOp       string
		expectedLocks    map[string]string
	}{
		// Column operations
		{
			name:             "ALTER TABLE ADD COLUMN without DEFAULT",
			sql:              "ALTER TABLE users ADD COLUMN age INT",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "ALTER TABLE ADD COLUMN without DEFAULT",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},
		{
			name:             "ALTER TABLE ADD COLUMN with constant DEFAULT",
			sql:              "ALTER TABLE users ADD COLUMN status TEXT DEFAULT 'active'",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "ALTER TABLE ADD COLUMN with constant DEFAULT",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},
		{
			name:             "ALTER TABLE ADD COLUMN with volatile DEFAULT",
			sql:              "ALTER TABLE users ADD COLUMN uuid TEXT DEFAULT gen_random_uuid()",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "ALTER TABLE ADD COLUMN with volatile DEFAULT",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},
		{
			name:             "ALTER TABLE DROP COLUMN",
			sql:              "ALTER TABLE users DROP COLUMN obsolete_field",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "ALTER TABLE DROP COLUMN",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},
		{
			name:             "ALTER TABLE ALTER COLUMN TYPE",
			sql:              "ALTER TABLE users ALTER COLUMN age TYPE BIGINT",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "ALTER TABLE ALTER COLUMN TYPE",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},
		{
			name:             "ALTER TABLE SET DEFAULT",
			sql:              "ALTER TABLE users ALTER COLUMN status SET DEFAULT 'active'",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "ALTER TABLE SET DEFAULT",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},
		{
			name:             "ALTER TABLE DROP DEFAULT",
			sql:              "ALTER TABLE users ALTER COLUMN status DROP DEFAULT",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "ALTER TABLE DROP DEFAULT",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},
		{
			name:             "ALTER TABLE SET NOT NULL",
			sql:              "ALTER TABLE users ALTER COLUMN email SET NOT NULL",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "ALTER TABLE SET NOT NULL",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},
		{
			name:             "ALTER TABLE DROP NOT NULL",
			sql:              "ALTER TABLE users ALTER COLUMN email DROP NOT NULL",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "ALTER TABLE DROP NOT NULL",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},
		{
			name:             "ALTER TABLE RENAME COLUMN",
			sql:              "ALTER TABLE users RENAME COLUMN username TO user_name",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "ALTER TABLE RENAME COLUMN",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},

		// Table-level operations
		{
			name:             "ALTER TABLE RENAME TO",
			sql:              "ALTER TABLE users RENAME TO app_users",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "ALTER TABLE RENAME TO",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},
		{
			name:             "ALTER TABLE SET SCHEMA",
			sql:              "ALTER TABLE users SET SCHEMA archive",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "ALTER TABLE SET SCHEMA",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},
		{
			name:             "ALTER TABLE SET TABLESPACE",
			sql:              "ALTER TABLE users SET TABLESPACE fast_ssd",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "ALTER TABLE SET TABLESPACE",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},
		{
			name:             "ALTER TABLE SET LOGGED",
			sql:              "ALTER TABLE users SET LOGGED",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "ALTER TABLE SET LOGGED",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},
		{
			name:             "ALTER TABLE SET UNLOGGED",
			sql:              "ALTER TABLE users SET UNLOGGED",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "ALTER TABLE SET UNLOGGED",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},

		// Constraint operations
		{
			name:             "ALTER TABLE ADD PRIMARY KEY",
			sql:              "ALTER TABLE users ADD PRIMARY KEY (id)",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "ALTER TABLE ADD PRIMARY KEY",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},
		{
			name:             "ALTER TABLE ADD FOREIGN KEY",
			sql:              "ALTER TABLE orders ADD FOREIGN KEY (user_id) REFERENCES users(id)",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "ALTER TABLE ADD FOREIGN KEY",
			expectedLocks:    map[string]string{"orders": "ShareRowExclusive", "users": "RowShare"},
		},
		{
			name:             "ALTER TABLE ADD CHECK CONSTRAINT",
			sql:              "ALTER TABLE users ADD CONSTRAINT age_check CHECK (age >= 0)",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "ALTER TABLE ADD CONSTRAINT CHECK",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},
		{
			name:             "ALTER TABLE ADD UNIQUE CONSTRAINT",
			sql:              "ALTER TABLE users ADD CONSTRAINT email_unique UNIQUE (email)",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "ALTER TABLE ADD CONSTRAINT UNIQUE",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},
		{
			name:             "ALTER TABLE ADD EXCLUDE CONSTRAINT",
			sql:              "ALTER TABLE reservations ADD CONSTRAINT no_overlap EXCLUDE USING gist (room_id WITH =, tsrange(start_time, end_time) WITH &&)",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "ALTER TABLE ADD CONSTRAINT EXCLUDE",
			expectedLocks:    map[string]string{"reservations": "AccessExclusive"},
		},
		{
			name:             "ALTER TABLE VALIDATE CONSTRAINT",
			sql:              "ALTER TABLE users VALIDATE CONSTRAINT age_check",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "ALTER TABLE VALIDATE CONSTRAINT",
			expectedLocks:    map[string]string{"users": "ShareUpdateExclusive"},
		},
		{
			name:             "ALTER TABLE DROP CONSTRAINT",
			sql:              "ALTER TABLE users DROP CONSTRAINT age_check",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "ALTER TABLE DROP CONSTRAINT",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},

		// Storage parameters
		{
			name:             "ALTER TABLE SET storage parameter",
			sql:              "ALTER TABLE users SET (fillfactor = 70)",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "ALTER TABLE SET",
			expectedLocks:    map[string]string{"users": "ShareUpdateExclusive"},
		},
		{
			name:             "ALTER TABLE RESET storage parameter",
			sql:              "ALTER TABLE users RESET (fillfactor)",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "ALTER TABLE RESET",
			expectedLocks:    map[string]string{"users": "ShareUpdateExclusive"},
		},
		{
			name:             "ALTER TABLE ALTER COLUMN SET STATISTICS",
			sql:              "ALTER TABLE users ALTER COLUMN email SET STATISTICS 1000",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "ALTER TABLE ALTER COLUMN SET STATISTICS",
			expectedLocks:    map[string]string{"users": "ShareUpdateExclusive"},
		},
		{
			name:             "ALTER TABLE ALTER COLUMN SET STORAGE",
			sql:              "ALTER TABLE users ALTER COLUMN data SET STORAGE EXTERNAL",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "ALTER TABLE ALTER COLUMN SET STORAGE",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},

		// Inheritance and partitioning
		{
			name:             "ALTER TABLE INHERIT",
			sql:              "ALTER TABLE child_table INHERIT parent_table",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "ALTER TABLE INHERIT",
			expectedLocks:    map[string]string{"child_table": "AccessExclusive", "parent_table": "ShareUpdateExclusive"},
		},
		{
			name:             "ALTER TABLE NO INHERIT",
			sql:              "ALTER TABLE child_table NO INHERIT parent_table",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "ALTER TABLE NO INHERIT",
			expectedLocks:    map[string]string{"child_table": "AccessExclusive", "parent_table": "ShareUpdateExclusive"},
		},
		{
			name:             "ALTER TABLE ATTACH PARTITION",
			sql:              "ALTER TABLE parent ATTACH PARTITION child FOR VALUES FROM (0) TO (100)",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "ALTER TABLE ATTACH PARTITION",
			expectedLocks:    map[string]string{"parent": "ShareUpdateExclusive", "child": "ShareUpdateExclusive"},
		},
		{
			name:             "ALTER TABLE DETACH PARTITION",
			sql:              "ALTER TABLE parent DETACH PARTITION child",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "ALTER TABLE DETACH PARTITION",
			expectedLocks:    map[string]string{"parent": "ShareUpdateExclusive", "child": "ShareUpdateExclusive"},
		},
		{
			name:             "ALTER TABLE DETACH PARTITION CONCURRENTLY - transaction",
			sql:              "ALTER TABLE parent DETACH PARTITION child CONCURRENTLY",
			mode:             InTransaction,
			expectedSeverity: SeverityError,
			expectedOp:       "ALTER TABLE DETACH PARTITION CONCURRENTLY",
		},
		{
			name:             "ALTER TABLE DETACH PARTITION CONCURRENTLY - no transaction",
			sql:              "ALTER TABLE parent DETACH PARTITION child CONCURRENTLY",
			mode:             NoTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "ALTER TABLE DETACH PARTITION CONCURRENTLY",
			expectedLocks:    map[string]string{"parent": "ShareUpdateExclusive", "child": "ShareUpdateExclusive"},
		},
		{
			name:             "ALTER TABLE SET ACCESS METHOD",
			sql:              "ALTER TABLE large_table SET ACCESS METHOD heap",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "ALTER TABLE SET ACCESS METHOD",
			expectedLocks:    map[string]string{"large_table": "AccessExclusive"},
		},

		// Type binding
		{
			name:             "ALTER TABLE OF",
			sql:              "ALTER TABLE users OF user_type",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "ALTER TABLE OF",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},
		{
			name:             "ALTER TABLE NOT OF",
			sql:              "ALTER TABLE users NOT OF",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "ALTER TABLE NOT OF",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},

		// Replication and ownership
		{
			name:             "ALTER TABLE REPLICA IDENTITY",
			sql:              "ALTER TABLE users REPLICA IDENTITY FULL",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "ALTER TABLE REPLICA IDENTITY",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},
		{
			name:             "ALTER TABLE OWNER TO",
			sql:              "ALTER TABLE users OWNER TO new_owner",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "ALTER TABLE OWNER TO",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},

		// Clustering
		{
			name:             "ALTER TABLE CLUSTER ON",
			sql:              "ALTER TABLE users CLUSTER ON users_pkey",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "ALTER TABLE CLUSTER ON",
			expectedLocks:    map[string]string{"users": "ShareUpdateExclusive"},
		},
		{
			name:             "ALTER TABLE SET WITHOUT CLUSTER",
			sql:              "ALTER TABLE users SET WITHOUT CLUSTER",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "ALTER TABLE SET WITHOUT CLUSTER",
			expectedLocks:    map[string]string{"users": "ShareUpdateExclusive"},
		},

		// Triggers and rules
		{
			name:             "ALTER TABLE ENABLE TRIGGER",
			sql:              "ALTER TABLE users ENABLE TRIGGER audit_trigger",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "ALTER TABLE ENABLE TRIGGER",
			expectedLocks:    map[string]string{"users": "ShareRowExclusive"},
		},
		{
			name:             "ALTER TABLE DISABLE TRIGGER",
			sql:              "ALTER TABLE users DISABLE TRIGGER audit_trigger",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "ALTER TABLE DISABLE TRIGGER",
			expectedLocks:    map[string]string{"users": "ShareRowExclusive"},
		},
		{
			name:             "ALTER TABLE ENABLE RULE",
			sql:              "ALTER TABLE users ENABLE RULE audit_rule",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "ALTER TABLE ENABLE RULE",
			expectedLocks:    map[string]string{"users": "ShareRowExclusive"},
		},
		{
			name:             "ALTER TABLE DISABLE RULE",
			sql:              "ALTER TABLE users DISABLE RULE audit_rule",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "ALTER TABLE DISABLE RULE",
			expectedLocks:    map[string]string{"users": "ShareRowExclusive"},
		},

		// Row-level security
		{
			name:             "ALTER TABLE ENABLE ROW LEVEL SECURITY",
			sql:              "ALTER TABLE users ENABLE ROW LEVEL SECURITY",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "ALTER TABLE ENABLE ROW LEVEL SECURITY",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},
		{
			name:             "ALTER TABLE DISABLE ROW LEVEL SECURITY",
			sql:              "ALTER TABLE users DISABLE ROW LEVEL SECURITY",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "ALTER TABLE DISABLE ROW LEVEL SECURITY",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},
		{
			name:             "ALTER TABLE FORCE ROW LEVEL SECURITY",
			sql:              "ALTER TABLE users FORCE ROW LEVEL SECURITY",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "ALTER TABLE FORCE ROW LEVEL SECURITY",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},
		{
			name:             "ALTER TABLE NO FORCE ROW LEVEL SECURITY",
			sql:              "ALTER TABLE users NO FORCE ROW LEVEL SECURITY",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "ALTER TABLE NO FORCE ROW LEVEL SECURITY",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},
	}

	runAnalyzerTests(t, tests)
}

func TestAnalyzer_DDL_Indexes(t *testing.T) {
	tests := []struct {
		name             string
		sql              string
		mode             TransactionMode
		expectedSeverity Severity
		expectedOp       string
		expectedLocks    map[string]string
	}{
		// CREATE INDEX variations
		{
			name:             "CREATE INDEX",
			sql:              "CREATE INDEX idx_users_email ON users(email)",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "CREATE INDEX",
			expectedLocks:    map[string]string{"users": "Share"},
		},
		{
			name:             "CREATE UNIQUE INDEX",
			sql:              "CREATE UNIQUE INDEX idx_users_email ON users(email)",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "CREATE UNIQUE INDEX",
			expectedLocks:    map[string]string{"users": "Share"},
		},
		{
			name:             "CREATE INDEX CONCURRENTLY - transaction",
			sql:              "CREATE INDEX CONCURRENTLY idx_users_email ON users(email)",
			mode:             InTransaction,
			expectedSeverity: SeverityError,
			expectedOp:       "CREATE INDEX CONCURRENTLY",
		},
		{
			name:             "CREATE INDEX CONCURRENTLY - no transaction",
			sql:              "CREATE INDEX CONCURRENTLY idx_users_email ON users(email)",
			mode:             NoTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "CREATE INDEX CONCURRENTLY",
			expectedLocks:    map[string]string{"users": "ShareUpdateExclusive"},
		},

		// DROP INDEX
		{
			name:             "DROP INDEX",
			sql:              "DROP INDEX idx_users_email",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "DROP INDEX",
			expectedLocks:    map[string]string{"idx_users_email": "AccessExclusive"},
		},
		{
			name:             "DROP INDEX CONCURRENTLY - transaction",
			sql:              "DROP INDEX CONCURRENTLY idx_users_email",
			mode:             InTransaction,
			expectedSeverity: SeverityError,
			expectedOp:       "DROP INDEX CONCURRENTLY",
		},
		{
			name:             "DROP INDEX CONCURRENTLY - no transaction",
			sql:              "DROP INDEX CONCURRENTLY idx_users_email",
			mode:             NoTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "DROP INDEX CONCURRENTLY",
			expectedLocks:    map[string]string{"idx_users_email": "ShareUpdateExclusive"},
		},

		// REINDEX
		{
			name:             "REINDEX INDEX",
			sql:              "REINDEX INDEX idx_users_email",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "REINDEX",
			expectedLocks:    map[string]string{"idx_users_email": "AccessExclusive"},
		},
		{
			name:             "REINDEX TABLE",
			sql:              "REINDEX TABLE users",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "REINDEX TABLE",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},
		{
			name:             "REINDEX SCHEMA",
			sql:              "REINDEX SCHEMA public",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "REINDEX SCHEMA",
			expectedLocks:    map[string]string{},  // Schema-wide locks on all tables
		},
		{
			name:             "REINDEX DATABASE",
			sql:              "REINDEX DATABASE mydb",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "REINDEX DATABASE",
			expectedLocks:    map[string]string{},  // Database-wide locks
		},
		{
			name:             "REINDEX SYSTEM",
			sql:              "REINDEX SYSTEM mydb",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "REINDEX SYSTEM",
			expectedLocks:    map[string]string{},  // System catalog locks
		},
		{
			name:             "REINDEX CONCURRENTLY - transaction",
			sql:              "REINDEX (CONCURRENTLY) INDEX idx_users_email",
			mode:             InTransaction,
			expectedSeverity: SeverityError,
			expectedOp:       "REINDEX CONCURRENTLY",
		},
		{
			name:             "REINDEX CONCURRENTLY - no transaction",
			sql:              "REINDEX (CONCURRENTLY) INDEX idx_users_email",
			mode:             NoTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "REINDEX CONCURRENTLY",
			expectedLocks:    map[string]string{"idx_users_email": "ShareUpdateExclusive"},
		},

		// ALTER INDEX
		{
			name:             "ALTER INDEX RENAME",
			sql:              "ALTER INDEX idx_old RENAME TO idx_new",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "ALTER INDEX",
			expectedLocks:    map[string]string{"idx_old": "AccessExclusive"},
		},
		{
			name:             "ALTER INDEX SET TABLESPACE",
			sql:              "ALTER INDEX idx_users_email SET TABLESPACE fast_ssd",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "ALTER INDEX",
			expectedLocks:    map[string]string{"idx_users_email": "AccessExclusive"},
		},
	}

	runAnalyzerTests(t, tests)
}

func TestAnalyzer_DDL_SchemaDatabaseObjects(t *testing.T) {
	tests := []struct {
		name             string
		sql              string
		mode             TransactionMode
		expectedSeverity Severity
		expectedOp       string
		expectedLocks    map[string]string
	}{
		// Schema operations
		{
			name:             "CREATE SCHEMA",
			sql:              "CREATE SCHEMA archive",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "CREATE SCHEMA",
		},
		{
			name:             "DROP SCHEMA",
			sql:              "DROP SCHEMA archive",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "DROP SCHEMA",
		},
		{
			name:             "DROP SCHEMA CASCADE",
			sql:              "DROP SCHEMA archive CASCADE",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "DROP SCHEMA CASCADE",
		},

		// Database operations
		{
			name:             "CREATE DATABASE - transaction",
			sql:              "CREATE DATABASE testdb",
			mode:             InTransaction,
			expectedSeverity: SeverityError,
			expectedOp:       "CREATE DATABASE",
		},
		{
			name:             "CREATE DATABASE - no transaction",
			sql:              "CREATE DATABASE testdb",
			mode:             NoTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "CREATE DATABASE",
		},
		{
			name:             "DROP DATABASE - transaction",
			sql:              "DROP DATABASE testdb",
			mode:             InTransaction,
			expectedSeverity: SeverityError,
			expectedOp:       "DROP DATABASE",
		},
		{
			name:             "DROP DATABASE - no transaction",
			sql:              "DROP DATABASE testdb",
			mode:             NoTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "DROP DATABASE",
		},
		{
			name:             "ALTER DATABASE",
			sql:              "ALTER DATABASE mydb SET work_mem = '256MB'",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "ALTER DATABASE",
		},

		// Tablespace operations
		{
			name:             "CREATE TABLESPACE - transaction",
			sql:              "CREATE TABLESPACE fast_ssd LOCATION '/mnt/ssd'",
			mode:             InTransaction,
			expectedSeverity: SeverityError,
			expectedOp:       "CREATE TABLESPACE",
		},
		{
			name:             "CREATE TABLESPACE - no transaction",
			sql:              "CREATE TABLESPACE fast_ssd LOCATION '/mnt/ssd'",
			mode:             NoTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "CREATE TABLESPACE",
		},
		{
			name:             "DROP TABLESPACE - transaction",
			sql:              "DROP TABLESPACE fast_ssd",
			mode:             InTransaction,
			expectedSeverity: SeverityError,
			expectedOp:       "DROP TABLESPACE",
		},
		{
			name:             "DROP TABLESPACE - no transaction",
			sql:              "DROP TABLESPACE fast_ssd",
			mode:             NoTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "DROP TABLESPACE",
		},
		{
			name:             "ALTER TABLESPACE - transaction",
			sql:              "ALTER TABLESPACE fast_ssd RENAME TO faster_ssd",
			mode:             InTransaction,
			expectedSeverity: SeverityError,
			expectedOp:       "ALTER TABLESPACE",
		},
		{
			name:             "ALTER TABLESPACE - no transaction",
			sql:              "ALTER TABLESPACE fast_ssd RENAME TO faster_ssd",
			mode:             NoTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "ALTER TABLESPACE",
		},
	}

	runAnalyzerTests(t, tests)
}

func TestAnalyzer_DDL_ViewsAndMaterializedViews(t *testing.T) {
	tests := []struct {
		name             string
		sql              string
		mode             TransactionMode
		expectedSeverity Severity
		expectedOp       string
		expectedLocks    map[string]string
	}{
		// Regular views
		{
			name:             "CREATE VIEW",
			sql:              "CREATE VIEW active_users AS SELECT * FROM users WHERE active = true",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "CREATE VIEW",
			expectedLocks:    map[string]string{"users": "AccessShare"},
		},
		{
			name:             "DROP VIEW",
			sql:              "DROP VIEW active_users",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "DROP VIEW",
			expectedLocks:    map[string]string{"active_users": "AccessExclusive"},
		},
		{
			name:             "ALTER VIEW",
			sql:              "ALTER VIEW active_users RENAME TO current_users",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "ALTER VIEW",
			expectedLocks:    map[string]string{"active_users": "AccessExclusive"},
		},

		// Materialized views
		{
			name:             "CREATE MATERIALIZED VIEW",
			sql:              "CREATE MATERIALIZED VIEW user_stats AS SELECT user_id, COUNT(*) FROM orders GROUP BY user_id",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "CREATE MATERIALIZED VIEW",
			expectedLocks:    map[string]string{"orders": "AccessShare"},
		},
		{
			name:             "DROP MATERIALIZED VIEW",
			sql:              "DROP MATERIALIZED VIEW user_stats",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "DROP MATERIALIZED VIEW",
			expectedLocks:    map[string]string{"user_stats": "AccessExclusive"},
		},
		{
			name:             "REFRESH MATERIALIZED VIEW",
			sql:              "REFRESH MATERIALIZED VIEW user_stats",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "REFRESH MATERIALIZED VIEW",
			expectedLocks:    map[string]string{"user_stats": "AccessExclusive"},
		},
		{
			name:             "REFRESH MATERIALIZED VIEW CONCURRENTLY - transaction",
			sql:              "REFRESH MATERIALIZED VIEW CONCURRENTLY user_stats",
			mode:             InTransaction,
			expectedSeverity: SeverityError,
			expectedOp:       "REFRESH MATERIALIZED VIEW CONCURRENTLY",
		},
		{
			name:             "REFRESH MATERIALIZED VIEW CONCURRENTLY - no transaction",
			sql:              "REFRESH MATERIALIZED VIEW CONCURRENTLY user_stats",
			mode:             NoTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "REFRESH MATERIALIZED VIEW CONCURRENTLY",
			expectedLocks:    map[string]string{"user_stats": "Exclusive"},
		},
	}

	runAnalyzerTests(t, tests)
}

func TestAnalyzer_DDL_OtherObjects(t *testing.T) {
	tests := []struct {
		name             string
		sql              string
		mode             TransactionMode
		expectedSeverity Severity
		expectedOp       string
		expectedLocks    map[string]string
	}{
		// Sequences
		{
			name:             "CREATE SEQUENCE",
			sql:              "CREATE SEQUENCE user_id_seq",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "CREATE SEQUENCE",
		},
		{
			name:             "DROP SEQUENCE",
			sql:              "DROP SEQUENCE user_id_seq",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "DROP SEQUENCE",
			expectedLocks:    map[string]string{"user_id_seq": "AccessExclusive"},
		},
		{
			name:             "ALTER SEQUENCE",
			sql:              "ALTER SEQUENCE user_id_seq RESTART WITH 1000",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "ALTER SEQUENCE",
			expectedLocks:    map[string]string{"user_id_seq": "AccessExclusive"},
		},

		// Types and domains
		{
			name:             "CREATE TYPE",
			sql:              "CREATE TYPE mood AS ENUM ('happy', 'sad', 'neutral')",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "CREATE TYPE",
		},
		{
			name:             "DROP TYPE",
			sql:              "DROP TYPE mood",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "DROP TYPE",
		},
		{
			name:             "ALTER TYPE",
			sql:              "ALTER TYPE mood RENAME TO emotion",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "ALTER TYPE",
		},
		{
			name:             "ALTER TYPE ADD VALUE - transaction",
			sql:              "ALTER TYPE mood ADD VALUE 'angry'",
			mode:             InTransaction,
			expectedSeverity: SeverityError,
			expectedOp:       "ALTER TYPE ADD VALUE",
		},
		{
			name:             "ALTER TYPE ADD VALUE - no transaction",
			sql:              "ALTER TYPE mood ADD VALUE 'angry'",
			mode:             NoTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "ALTER TYPE ADD VALUE",
		},
		{
			name:             "CREATE DOMAIN",
			sql:              "CREATE DOMAIN email AS TEXT CHECK (VALUE ~ '^[^@]+@[^@]+$')",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "CREATE DOMAIN",
		},
		{
			name:             "DROP DOMAIN",
			sql:              "DROP DOMAIN email",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "DROP DOMAIN",
		},
		{
			name:             "ALTER DOMAIN",
			sql:              "ALTER DOMAIN email ADD CONSTRAINT email_length CHECK (LENGTH(VALUE) <= 255)",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "ALTER DOMAIN",
		},

		// Extensions
		{
			name:             "CREATE EXTENSION",
			sql:              "CREATE EXTENSION IF NOT EXISTS pg_stat_statements",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "CREATE EXTENSION",
		},
		{
			name:             "DROP EXTENSION",
			sql:              "DROP EXTENSION pg_stat_statements",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "DROP EXTENSION",
		},

		// Functions and procedures
		{
			name:             "CREATE FUNCTION",
			sql:              "CREATE FUNCTION add_numbers(a INT, b INT) RETURNS INT AS $$ SELECT a + b $$ LANGUAGE SQL",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "CREATE FUNCTION",
		},
		{
			name:             "DROP FUNCTION",
			sql:              "DROP FUNCTION add_numbers(INT, INT)",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "DROP FUNCTION",
		},
		{
			name:             "CREATE PROCEDURE",
			sql:              "CREATE PROCEDURE process_orders() LANGUAGE SQL AS $$ UPDATE orders SET processed = true $$",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "CREATE PROCEDURE",
		},
		{
			name:             "DROP PROCEDURE",
			sql:              "DROP PROCEDURE process_orders()",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "DROP PROCEDURE",
		},

		// Text search objects
		{
			name:             "CREATE TEXT SEARCH CONFIGURATION",
			sql:              "CREATE TEXT SEARCH CONFIGURATION my_config (COPY = english)",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "CREATE TEXT SEARCH CONFIGURATION",
		},
		{
			name:             "DROP TEXT SEARCH CONFIGURATION",
			sql:              "DROP TEXT SEARCH CONFIGURATION my_config",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "DROP TEXT SEARCH CONFIGURATION",
		},
		{
			name:             "CREATE TEXT SEARCH DICTIONARY",
			sql:              "CREATE TEXT SEARCH DICTIONARY my_dict (TEMPLATE = simple, STOPWORDS = english)",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "CREATE TEXT SEARCH DICTIONARY",
		},
		{
			name:             "DROP TEXT SEARCH DICTIONARY",
			sql:              "DROP TEXT SEARCH DICTIONARY my_dict",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "DROP TEXT SEARCH DICTIONARY",
		},
		{
			name:             "CREATE TEXT SEARCH PARSER",
			sql:              "CREATE TEXT SEARCH PARSER my_parser (START = prsd_start, GETTOKEN = prsd_nexttoken, END = prsd_end, LEXTYPES = prsd_lextype)",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "CREATE TEXT SEARCH PARSER",
		},
		{
			name:             "DROP TEXT SEARCH PARSER",
			sql:              "DROP TEXT SEARCH PARSER my_parser",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "DROP TEXT SEARCH PARSER",
		},
		{
			name:             "CREATE TEXT SEARCH TEMPLATE",
			sql:              "CREATE TEXT SEARCH TEMPLATE my_template (INIT = dsimple_init, LEXIZE = dsimple_lexize)",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "CREATE TEXT SEARCH TEMPLATE",
		},
		{
			name:             "DROP TEXT SEARCH TEMPLATE",
			sql:              "DROP TEXT SEARCH TEMPLATE my_template",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "DROP TEXT SEARCH TEMPLATE",
		},

		// Statistics and event triggers
		{
			name:             "CREATE STATISTICS",
			sql:              "CREATE STATISTICS s1 (dependencies) ON a, b FROM t1",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "CREATE STATISTICS",
		},
		{
			name:             "DROP STATISTICS",
			sql:              "DROP STATISTICS s1",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "DROP STATISTICS",
		},
		{
			name:             "CREATE EVENT TRIGGER",
			sql:              "CREATE EVENT TRIGGER my_trigger ON ddl_command_start EXECUTE FUNCTION my_func()",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "CREATE EVENT TRIGGER",
		},
		{
			name:             "DROP EVENT TRIGGER",
			sql:              "DROP EVENT TRIGGER my_trigger",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "DROP EVENT TRIGGER",
		},

		// Other operations
		{
			name:             "CREATE AGGREGATE",
			sql:              "CREATE AGGREGATE myavg(numeric) (SFUNC = numeric_avg_accum, STYPE = internal[])",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "CREATE AGGREGATE",
		},
		{
			name:             "DROP AGGREGATE",
			sql:              "DROP AGGREGATE myavg(numeric)",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "DROP AGGREGATE",
		},
		{
			name:             "CREATE OPERATOR",
			sql:              "CREATE OPERATOR === (LEFTARG = text, RIGHTARG = text, FUNCTION = text_eq)",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "CREATE OPERATOR",
		},
		{
			name:             "DROP OPERATOR",
			sql:              "DROP OPERATOR === (text, text)",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "DROP OPERATOR",
		},
		{
			name:             "CREATE CAST",
			sql:              "CREATE CAST (varchar AS text) WITHOUT FUNCTION AS IMPLICIT",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "CREATE CAST",
		},
		{
			name:             "DROP CAST",
			sql:              "DROP CAST (varchar AS text)",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "DROP CAST",
		},
		{
			name:             "CREATE COLLATION",
			sql:              "CREATE COLLATION french (LOCALE = 'fr_FR.utf8')",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "CREATE COLLATION",
		},
		{
			name:             "DROP COLLATION",
			sql:              "DROP COLLATION french",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "DROP COLLATION",
		},
	}

	runAnalyzerTests(t, tests)
}

func TestAnalyzer_DDL_TriggersRulesAndPolicies(t *testing.T) {
	tests := []struct {
		name             string
		sql              string
		mode             TransactionMode
		expectedSeverity Severity
		expectedOp       string
		expectedLocks    map[string]string
	}{
		// Triggers
		{
			name:             "CREATE TRIGGER",
			sql:              "CREATE TRIGGER audit_trigger AFTER INSERT ON users FOR EACH ROW EXECUTE FUNCTION audit_function()",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "CREATE TRIGGER",
			expectedLocks:    map[string]string{"users": "ShareRowExclusive"},
		},
		{
			name:             "DROP TRIGGER",
			sql:              "DROP TRIGGER audit_trigger ON users",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "DROP TRIGGER",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},

		// Rules
		{
			name:             "CREATE RULE",
			sql:              "CREATE RULE notify_me AS ON INSERT TO users DO NOTIFY user_added",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "CREATE RULE",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},
		{
			name:             "DROP RULE",
			sql:              "DROP RULE notify_me ON users",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "DROP RULE",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},

		// Policies (Row Level Security)
		{
			name:             "CREATE POLICY",
			sql:              "CREATE POLICY user_policy ON users FOR SELECT USING (user_id = current_user_id())",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "CREATE POLICY",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},
		{
			name:             "DROP POLICY",
			sql:              "DROP POLICY user_policy ON users",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "DROP POLICY",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},
	}

	runAnalyzerTests(t, tests)
}

// ===== 3. MAINTENANCE OPERATIONS =====

func TestAnalyzer_MaintenanceOperations(t *testing.T) {
	tests := []struct {
		name             string
		sql              string
		mode             TransactionMode
		expectedSeverity Severity
		expectedOp       string
		expectedLocks    map[string]string
	}{
		// VACUUM operations
		{
			name:             "VACUUM - transaction",
			sql:              "VACUUM users",
			mode:             InTransaction,
			expectedSeverity: SeverityError,
			expectedOp:       "VACUUM",
		},
		{
			name:             "VACUUM - no transaction",
			sql:              "VACUUM users",
			mode:             NoTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "VACUUM",
			expectedLocks:    map[string]string{"users": "ShareUpdateExclusive"},
		},
		{
			name:             "VACUUM FULL - transaction",
			sql:              "VACUUM FULL users",
			mode:             InTransaction,
			expectedSeverity: SeverityError,
			expectedOp:       "VACUUM FULL",
		},
		{
			name:             "VACUUM FULL - no transaction",
			sql:              "VACUUM FULL users",
			mode:             NoTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "VACUUM FULL",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},
		{
			name:             "VACUUM FREEZE - transaction",
			sql:              "VACUUM FREEZE users",
			mode:             InTransaction,
			expectedSeverity: SeverityError,
			expectedOp:       "VACUUM FREEZE",
		},
		{
			name:             "VACUUM FREEZE - no transaction",
			sql:              "VACUUM FREEZE users",
			mode:             NoTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "VACUUM FREEZE",
			expectedLocks:    map[string]string{"users": "ShareUpdateExclusive"},
		},
		{
			name:             "VACUUM ANALYZE - transaction",
			sql:              "VACUUM ANALYZE users",
			mode:             InTransaction,
			expectedSeverity: SeverityError,
			expectedOp:       "VACUUM ANALYZE",
		},
		{
			name:             "VACUUM ANALYZE - no transaction",
			sql:              "VACUUM ANALYZE users",
			mode:             NoTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "VACUUM ANALYZE",
			expectedLocks:    map[string]string{"users": "ShareUpdateExclusive"},
		},

		// ANALYZE
		{
			name:             "ANALYZE",
			sql:              "ANALYZE users",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "ANALYZE",
			expectedLocks:    map[string]string{"users": "ShareUpdateExclusive"},
		},

		// CLUSTER
		{
			name:             "CLUSTER",
			sql:              "CLUSTER users USING users_pkey",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "CLUSTER",
			expectedLocks:    map[string]string{"users": "AccessExclusive"},
		},
	}

	runAnalyzerTests(t, tests)
}

// ===== 4. EXPLICIT LOCKING =====

func TestAnalyzer_ExplicitLocking(t *testing.T) {
	tests := []struct {
		name             string
		sql              string
		mode             TransactionMode
		expectedSeverity Severity
		expectedOp       string
		expectedLocks    map[string]string
		expectedLock     string
	}{
		{
			name:             "LOCK TABLE ACCESS SHARE",
			sql:              "LOCK TABLE users IN ACCESS SHARE MODE",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "LOCK TABLE ACCESS SHARE",
			expectedLock:     "AccessShare",
		},
		{
			name:             "LOCK TABLE ROW SHARE",
			sql:              "LOCK TABLE users IN ROW SHARE MODE",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "LOCK TABLE ROW SHARE",
			expectedLock:     "RowShare",
		},
		{
			name:             "LOCK TABLE ROW EXCLUSIVE",
			sql:              "LOCK TABLE users IN ROW EXCLUSIVE MODE",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "LOCK TABLE ROW EXCLUSIVE",
			expectedLock:     "RowExclusive",
		},
		{
			name:             "LOCK TABLE SHARE UPDATE EXCLUSIVE",
			sql:              "LOCK TABLE users IN SHARE UPDATE EXCLUSIVE MODE",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "LOCK TABLE SHARE UPDATE EXCLUSIVE",
			expectedLock:     "ShareUpdateExclusive",
		},
		{
			name:             "LOCK TABLE SHARE",
			sql:              "LOCK TABLE users IN SHARE MODE",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "LOCK TABLE SHARE",
			expectedLock:     "Share",
		},
		{
			name:             "LOCK TABLE SHARE ROW EXCLUSIVE",
			sql:              "LOCK TABLE users IN SHARE ROW EXCLUSIVE MODE",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "LOCK TABLE SHARE ROW EXCLUSIVE",
			expectedLock:     "ShareRowExclusive",
		},
		{
			name:             "LOCK TABLE EXCLUSIVE",
			sql:              "LOCK TABLE users IN EXCLUSIVE MODE",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "LOCK TABLE EXCLUSIVE",
			expectedLock:     "Exclusive",
		},
		{
			name:             "LOCK TABLE ACCESS EXCLUSIVE",
			sql:              "LOCK TABLE users IN ACCESS EXCLUSIVE MODE",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "LOCK TABLE ACCESS EXCLUSIVE",
			expectedLock:     "AccessExclusive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := New()
			p := parser.NewParser()

			parsed, err := p.ParseSQL(tt.sql)
			if err != nil {
				t.Fatalf("Failed to parse SQL: %v", err)
			}

			result, err := a.AnalyzeStatement(parsed.Statements[0], tt.mode)
			if err != nil {
				t.Fatalf("Failed to analyze: %v", err)
			}

			if result.Severity != tt.expectedSeverity {
				t.Errorf("Expected severity %s, got %s", tt.expectedSeverity, result.Severity)
			}

			if result.Operation() != tt.expectedOp {
				t.Errorf("Expected operation %s, got %s", tt.expectedOp, result.Operation())
			}

			if string(result.LockType()) != tt.expectedLock {
				t.Errorf("Expected lock type %s, got %s", tt.expectedLock, result.LockType())
			}
		})
	}
}

// ===== 5. OWNERSHIP AND PERMISSIONS =====

func TestAnalyzer_OwnershipAndPermissions(t *testing.T) {
	tests := []struct {
		name             string
		sql              string
		mode             TransactionMode
		expectedSeverity Severity
		expectedOp       string
		expectedLocks    map[string]string
	}{
		{
			name:             "GRANT on table",
			sql:              "GRANT SELECT, INSERT ON users TO app_user",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "GRANT",
		},
		{
			name:             "REVOKE on table",
			sql:              "REVOKE DELETE ON users FROM app_user",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "REVOKE",
		},
		{
			name:             "REASSIGN OWNED",
			sql:              "REASSIGN OWNED BY old_user TO new_user",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedOp:       "REASSIGN OWNED",
		},
		{
			name:             "DROP OWNED",
			sql:              "DROP OWNED BY old_user",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedOp:       "DROP OWNED",
		},
		{
			name:             "CREATE ROLE",
			sql:              "CREATE ROLE app_user WITH LOGIN PASSWORD 'secret'",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "CREATE ROLE",
		},
		{
			name:             "DROP ROLE",
			sql:              "DROP ROLE app_user",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "DROP ROLE",
		},
		{
			name:             "ALTER ROLE",
			sql:              "ALTER ROLE app_user SET work_mem = '256MB'",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "ALTER ROLE",
		},
		{
			name:             "ALTER DEFAULT PRIVILEGES",
			sql:              "ALTER DEFAULT PRIVILEGES IN SCHEMA myschema GRANT SELECT ON TABLES TO readonly",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "ALTER DEFAULT PRIVILEGES",
		},
	}

	runAnalyzerTests(t, tests)
}

// ===== 6. SYSTEM AND SESSION OPERATIONS =====

func TestAnalyzer_SystemAndSessionOperations(t *testing.T) {
	tests := []struct {
		name             string
		sql              string
		mode             TransactionMode
		expectedSeverity Severity
		expectedOp       string
		expectedLocks    map[string]string
	}{
		// System operations
		{
			name:             "ALTER SYSTEM - transaction",
			sql:              "ALTER SYSTEM SET work_mem = '256MB'",
			mode:             InTransaction,
			expectedSeverity: SeverityError,
			expectedOp:       "ALTER SYSTEM",
		},
		{
			name:             "ALTER SYSTEM - no transaction",
			sql:              "ALTER SYSTEM SET work_mem = '256MB'",
			mode:             NoTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "ALTER SYSTEM",
		},
		{
			name:             "CHECKPOINT",
			sql:              "CHECKPOINT",
			mode:             NoTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "CHECKPOINT",
		},
		{
			name:             "LOAD",
			sql:              "LOAD 'auto_explain'",
			mode:             NoTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "LOAD",
		},

		// Session settings
		{
			name:             "SET session variable",
			sql:              "SET work_mem = '256MB'",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "SET",
		},
		{
			name:             "SET LOCAL",
			sql:              "SET LOCAL work_mem = '256MB'",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "SET LOCAL",
		},
		{
			name:             "RESET",
			sql:              "RESET work_mem",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "RESET",
		},

		// Comments
		{
			name:             "COMMENT ON TABLE",
			sql:              "COMMENT ON TABLE users IS 'User accounts'",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "COMMENT ON",
		},
	}

	runAnalyzerTests(t, tests)
}

// ===== 7. SUBSCRIPTION OPERATIONS =====

func TestAnalyzer_SubscriptionOperations(t *testing.T) {
	tests := []struct {
		name             string
		sql              string
		mode             TransactionMode
		expectedSeverity Severity
		expectedOp       string
		expectedLocks    map[string]string
	}{
		{
			name:             "CREATE SUBSCRIPTION - transaction",
			sql:              "CREATE SUBSCRIPTION mysub CONNECTION 'host=source dbname=mydb' PUBLICATION mypub",
			mode:             InTransaction,
			expectedSeverity: SeverityError,
			expectedOp:       "CREATE SUBSCRIPTION",
		},
		{
			name:             "CREATE SUBSCRIPTION - no transaction",
			sql:              "CREATE SUBSCRIPTION mysub CONNECTION 'host=source dbname=mydb' PUBLICATION mypub",
			mode:             NoTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "CREATE SUBSCRIPTION",
		},
		{
			name:             "ALTER SUBSCRIPTION - transaction",
			sql:              "ALTER SUBSCRIPTION mysub DISABLE",
			mode:             InTransaction,
			expectedSeverity: SeverityError,
			expectedOp:       "ALTER SUBSCRIPTION",
		},
		{
			name:             "ALTER SUBSCRIPTION - no transaction",
			sql:              "ALTER SUBSCRIPTION mysub DISABLE",
			mode:             NoTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "ALTER SUBSCRIPTION",
		},
		{
			name:             "DROP SUBSCRIPTION - transaction",
			sql:              "DROP SUBSCRIPTION mysub",
			mode:             InTransaction,
			expectedSeverity: SeverityError,
			expectedOp:       "DROP SUBSCRIPTION",
		},
		{
			name:             "DROP SUBSCRIPTION - no transaction",
			sql:              "DROP SUBSCRIPTION mysub",
			mode:             NoTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "DROP SUBSCRIPTION",
		},
		{
			name:             "CREATE PUBLICATION",
			sql:              "CREATE PUBLICATION mypub FOR ALL TABLES",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "CREATE PUBLICATION",
		},
		{
			name:             "DROP PUBLICATION",
			sql:              "DROP PUBLICATION mypub",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "DROP PUBLICATION",
		},
		{
			name:             "ALTER PUBLICATION ADD TABLE",
			sql:              "ALTER PUBLICATION mypub ADD TABLE users, orders",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "ALTER PUBLICATION ADD TABLE",
		},
		{
			name:             "ALTER PUBLICATION DROP TABLE",
			sql:              "ALTER PUBLICATION mypub DROP TABLE users",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "ALTER PUBLICATION DROP TABLE",
		},

		// Foreign data wrappers
		{
			name:             "CREATE FOREIGN DATA WRAPPER",
			sql:              "CREATE FOREIGN DATA WRAPPER postgres_fdw",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "CREATE FOREIGN DATA WRAPPER",
		},
		{
			name:             "DROP FOREIGN DATA WRAPPER",
			sql:              "DROP FOREIGN DATA WRAPPER postgres_fdw",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "DROP FOREIGN DATA WRAPPER",
		},
		{
			name:             "CREATE SERVER",
			sql:              "CREATE SERVER foreign_server FOREIGN DATA WRAPPER postgres_fdw OPTIONS (host 'foo', dbname 'foodb', port '5432')",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "CREATE SERVER",
		},
		{
			name:             "DROP SERVER",
			sql:              "DROP SERVER foreign_server",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "DROP SERVER",
		},
		{
			name:             "CREATE USER MAPPING",
			sql:              "CREATE USER MAPPING FOR bob SERVER foreign_server OPTIONS (user 'bob', password 'secret')",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "CREATE USER MAPPING",
		},
		{
			name:             "DROP USER MAPPING",
			sql:              "DROP USER MAPPING FOR bob SERVER foreign_server",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "DROP USER MAPPING",
		},
	}

	runAnalyzerTests(t, tests)
}

// ===== 8. TRANSACTION CONTROL =====

func TestAnalyzer_TransactionControl(t *testing.T) {
	tests := []struct {
		name             string
		sql              string
		mode             TransactionMode
		expectedSeverity Severity
		expectedOp       string
		expectedLocks    map[string]string
	}{
		{
			name:             "BEGIN",
			sql:              "BEGIN",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "BEGIN",
		},
		{
			name:             "START TRANSACTION",
			sql:              "START TRANSACTION",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "START TRANSACTION",
		},
		{
			name:             "COMMIT",
			sql:              "COMMIT",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "COMMIT",
		},
		{
			name:             "END",
			sql:              "END",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "COMMIT",
		},
		{
			name:             "ROLLBACK",
			sql:              "ROLLBACK",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "ROLLBACK",
		},
		{
			name:             "SAVEPOINT",
			sql:              "SAVEPOINT my_savepoint",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "SAVEPOINT",
		},
		{
			name:             "RELEASE SAVEPOINT",
			sql:              "RELEASE SAVEPOINT my_savepoint",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "RELEASE SAVEPOINT",
		},
		{
			name:             "ROLLBACK TO SAVEPOINT",
			sql:              "ROLLBACK TO SAVEPOINT my_savepoint",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "ROLLBACK TO SAVEPOINT",
		},
		{
			name:             "SET TRANSACTION",
			sql:              "SET TRANSACTION ISOLATION LEVEL SERIALIZABLE",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedOp:       "SET TRANSACTION",
		},
	}

	runAnalyzerTests(t, tests)
}

// ===== 9. COMPLEX QUERIES =====

func TestAnalyzer_ComplexQueries(t *testing.T) {
	tests := []struct {
		name             string
		sql              string
		mode             TransactionMode
		expectedSeverity Severity
		expectedTables   []string
		minTableCount    int
	}{
		// Simple CTEs
		{
			name: "CTE with single DELETE",
			sql: `WITH deleted AS (
                DELETE FROM sessions WHERE expired = true RETURNING user_id
            )
            SELECT * FROM deleted`,
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedTables:   []string{"sessions"},
			minTableCount:    1,
		},

		// Multiple CTEs
		{
			name: "Multiple CTEs with different operations",
			sql: `WITH 
            deleted_sessions AS (
                DELETE FROM sessions WHERE expired = true RETURNING user_id
            ),
            updated_users AS (
                UPDATE users SET last_active = NOW() 
                WHERE id IN (SELECT user_id FROM deleted_sessions) 
                RETURNING id
            )
            INSERT INTO audit_log (user_id, action) 
            SELECT user_id, 'session_cleanup' FROM deleted_sessions`,
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedTables:   []string{"sessions", "users", "audit_log"},
			minTableCount:    3,
		},

		// CTE with full table operations
		{
			name: "CTE with full table DELETE",
			sql: `WITH cleanup AS (
                DELETE FROM temp_data RETURNING *
            )
            INSERT INTO permanent_data SELECT * FROM cleanup`,
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedTables:   []string{"temp_data", "permanent_data"},
			minTableCount:    2,
		},

		// MERGE with CTEs
		{
			name: "Complex MERGE with multiple CTEs",
			sql: `WITH inactive_users AS(
                DELETE FROM user_sessions 
                WHERE last_active < NOW() - INTERVAL '90 days' 
                RETURNING user_id
            ),
            archived AS(
                INSERT INTO archived_users 
                SELECT * FROM users 
                WHERE user_id IN(SELECT user_id FROM inactive_users) 
                RETURNING user_id
            ) 
            MERGE INTO user_statistics us
            USING (SELECT user_id FROM archived) a
            ON us.user_id = a.user_id
            WHEN MATCHED THEN UPDATE SET archived_at = NOW()
            WHEN NOT MATCHED THEN INSERT (user_id, archived_at) VALUES (a.user_id, NOW())`,
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedTables:   []string{"user_sessions", "archived_users", "users", "user_statistics"},
			minTableCount:    4,
		},

		// Recursive CTEs
		{
			name: "Recursive CTE with UPDATE",
			sql: `WITH RECURSIVE subordinates AS (
                SELECT employee_id, manager_id FROM employees WHERE employee_id = 1
                UNION ALL
                SELECT e.employee_id, e.manager_id 
                FROM employees e 
                INNER JOIN subordinates s ON s.employee_id = e.manager_id
            )
            UPDATE employees SET department = 'new_dept' 
            WHERE employee_id IN (SELECT employee_id FROM subordinates)`,
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedTables:   []string{"employees"},
			minTableCount:    1,
		},

		// Data modifying CTEs
		{
			name: "Data modifying CTE chain",
			sql: `WITH moved_rows AS (
                DELETE FROM products 
                WHERE category = 'discontinued' 
                RETURNING *
            ), 
            inserted AS (
                INSERT INTO archived_products 
                SELECT * FROM moved_rows 
                RETURNING id
            )
            UPDATE inventory SET quantity = 0 
            WHERE product_id IN (SELECT id FROM inserted)`,
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedTables:   []string{"products", "archived_products", "inventory"},
			minTableCount:    3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := New()
			p := parser.NewParser()

			parsed, err := p.ParseSQL(tt.sql)
			if err != nil {
				t.Fatalf("Failed to parse SQL: %v", err)
			}

			result, err := a.AnalyzeStatement(parsed.Statements[0], tt.mode)
			if err != nil {
				t.Fatalf("Failed to analyze: %v", err)
			}

			if result.Severity != tt.expectedSeverity {
				t.Errorf("Expected severity %s, got %s", tt.expectedSeverity, result.Severity)
			}

			tableLocks := result.TableLocks()
			if len(tableLocks) < tt.minTableCount {
				t.Errorf("Expected at least %d tables, got %d: %v",
					tt.minTableCount, len(tableLocks), tableLocks)
			}

			for _, expectedTable := range tt.expectedTables {
				found := false
				for _, tl := range tableLocks {
					if strings.Contains(tl, expectedTable) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected table %s not found in %v", expectedTable, tableLocks)
				}
			}
		})
	}
}

// ===== 10. EDGE CASES =====

func TestAnalyzer_EdgeCases(t *testing.T) {
	tests := []struct {
		name             string
		sql              string
		mode             TransactionMode
		expectedSeverity Severity
		expectError      bool
	}{
		{
			name:             "Empty input",
			sql:              "",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectError:      false,
		},
		{
			name:             "Whitespace only",
			sql:              "   \n\t   ",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectError:      false,
		},
		{
			name:             "Comment only",
			sql:              "-- This is just a comment",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectError:      false,
		},
		{
			name:             "Multi-line comment only",
			sql:              "/* This is a\n   multi-line comment */",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectError:      false,
		},
		{
			name:             "Invalid SQL",
			sql:              "SELEKT * FROM users", // Typo in SELECT
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectError:      true,
		},
		{
			name:             "Partial statement",
			sql:              "UPDATE users SET",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := New()
			p := parser.NewParser()

			parsed, err := p.ParseSQL(tt.sql)
			if err != nil {
				if !tt.expectError {
					t.Fatalf("Failed to parse SQL: %v", err)
				}
				return
			}

			if len(parsed.Statements) == 0 {
				// Empty input - analyzer should handle gracefully
				return
			}

			result, err := a.AnalyzeStatement(parsed.Statements[0], tt.mode)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Failed to analyze: %v", err)
			}

			if result.Severity != tt.expectedSeverity {
				t.Errorf("Expected severity %s, got %s", tt.expectedSeverity, result.Severity)
			}
		})
	}
}

// ===== 11. MULTIPLE STATEMENTS =====

func TestAnalyzer_MultipleStatements(t *testing.T) {
	tests := []struct {
		name               string
		sql                string
		mode               TransactionMode
		expectedSeverities []Severity
		expectedOps        []string
	}{
		{
			name: "Mixed severity statements",
			sql: `
                BEGIN;
                UPDATE users SET active = false WHERE id = 1;
                DELETE FROM sessions;
                CREATE INDEX idx_users_email ON users(email);
                COMMIT;
            `,
			mode: NoTransaction, // Start outside transaction, BEGIN will enter transaction
			expectedSeverities: []Severity{
				SeverityInfo,     // BEGIN
				SeverityWarning,  // UPDATE with WHERE (in transaction due to BEGIN)
				SeverityCritical, // DELETE without WHERE (in transaction)
				SeverityCritical, // CREATE INDEX (always CRITICAL)
				SeverityInfo,     // COMMIT
			},
			expectedOps: []string{
				"BEGIN",
				"UPDATE with WHERE",
				"DELETE without WHERE",
				"CREATE INDEX",
				"COMMIT",
			},
		},
		{
			name: "DDL batch in explicit transaction",
			sql: `
                BEGIN;
                ALTER TABLE users ADD COLUMN IF NOT EXISTS age INT;
                ALTER TABLE users ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'active';
                CREATE INDEX CONCURRENTLY idx_users_age ON users(age);
                ANALYZE users;
                COMMIT;
            `,
			mode: NoTransaction, // Start outside, BEGIN enters transaction
			expectedSeverities: []Severity{
				SeverityInfo,    // BEGIN
				SeverityInfo,    // ADD COLUMN without DEFAULT (in transaction)
				SeverityInfo,    // ADD COLUMN with constant DEFAULT (in transaction)
				SeverityError,   // CREATE INDEX CONCURRENTLY in transaction
				SeverityWarning, // ANALYZE (in transaction)
				SeverityInfo,    // COMMIT
			},
			expectedOps: []string{
				"BEGIN",
				"ALTER TABLE ADD COLUMN without DEFAULT",
				"ALTER TABLE ADD COLUMN with constant DEFAULT",
				"CREATE INDEX CONCURRENTLY",
				"ANALYZE",
				"COMMIT",
			},
		},
		{
			name: "Maintenance batch",
			sql: `
                VACUUM users;
                VACUUM FULL sessions;
                REINDEX TABLE products;
                CLUSTER orders USING orders_pkey;
            `,
			mode: NoTransaction,
			expectedSeverities: []Severity{
				SeverityWarning,  // VACUUM
				SeverityCritical, // VACUUM FULL
				SeverityCritical, // REINDEX TABLE
				SeverityCritical, // CLUSTER
			},
		},
		{
			name: "Transaction tracking - operations that cannot run in transaction",
			sql: `
                CREATE INDEX CONCURRENTLY idx1 ON users(id);
                BEGIN;
                CREATE INDEX CONCURRENTLY idx2 ON users(email);
                COMMIT;
                CREATE INDEX CONCURRENTLY idx3 ON users(name);
            `,
			mode: NoTransaction,
			expectedSeverities: []Severity{
				SeverityWarning, // CREATE INDEX CONCURRENTLY (outside transaction)
				SeverityInfo,    // BEGIN
				SeverityError,   // CREATE INDEX CONCURRENTLY (inside transaction - ERROR!)
				SeverityInfo,    // COMMIT
				SeverityWarning, // CREATE INDEX CONCURRENTLY (outside transaction again)
			},
			expectedOps: []string{
				"CREATE INDEX CONCURRENTLY",
				"BEGIN",
				"CREATE INDEX CONCURRENTLY",
				"COMMIT",
				"CREATE INDEX CONCURRENTLY",
			},
		},
		{
			name: "Transaction tracking - nested transaction with savepoints",
			sql: `
                BEGIN;
                UPDATE users SET active = true;
                SAVEPOINT sp1;
                DELETE FROM logs WHERE created_at < '2020-01-01';
                ROLLBACK TO SAVEPOINT sp1;
                DELETE FROM logs WHERE created_at < '2019-01-01';
                COMMIT;
            `,
			mode: NoTransaction,
			expectedSeverities: []Severity{
				SeverityInfo,     // BEGIN
				SeverityCritical, // UPDATE without WHERE (in transaction)
				SeverityInfo,     // SAVEPOINT
				SeverityWarning,  // DELETE with WHERE (in transaction)
				SeverityInfo,     // ROLLBACK TO SAVEPOINT
				SeverityWarning,  // DELETE with WHERE (still in transaction)
				SeverityInfo,     // COMMIT
			},
		},
		{
			name: "Transaction tracking - mode InTransaction by default",
			sql: `
                CREATE INDEX CONCURRENTLY idx_test ON users(id);
                COMMIT;
                CREATE INDEX CONCURRENTLY idx_new ON users(email);
            `,
			mode: InTransaction, // Start in transaction
			expectedSeverities: []Severity{
				SeverityError,   // CREATE INDEX CONCURRENTLY (ERROR in transaction)
				SeverityInfo,    // COMMIT
				SeverityWarning, // CREATE INDEX CONCURRENTLY (WARNING outside transaction)
			},
			expectedOps: []string{
				"CREATE INDEX CONCURRENTLY",
				"COMMIT",
				"CREATE INDEX CONCURRENTLY",
			},
		},
		{
			name: "Transaction tracking - ROLLBACK ends transaction",
			sql: `
                BEGIN;
                DROP TABLE users;
                ROLLBACK;
                DROP TABLE logs;
            `,
			mode: NoTransaction,
			expectedSeverities: []Severity{
				SeverityInfo,     // BEGIN
				SeverityCritical, // DROP TABLE (in transaction)
				SeverityInfo,     // ROLLBACK
				SeverityCritical, // DROP TABLE (outside transaction)
			},
		},
		{
			name: "Transaction tracking - COMMIT without BEGIN",
			sql: `
                COMMIT;
                CREATE INDEX CONCURRENTLY idx_test ON users(id);
            `,
			mode: NoTransaction,
			expectedSeverities: []Severity{
				SeverityInfo,    // COMMIT (harmless when not in transaction)
				SeverityWarning, // CREATE INDEX CONCURRENTLY (outside transaction)
			},
		},
		{
			name: "Transaction tracking - multiple BEGIN statements",
			sql: `
                BEGIN;
                BEGIN;
                VACUUM users;
                COMMIT;
                VACUUM logs;
            `,
			mode: NoTransaction,
			expectedSeverities: []Severity{
				SeverityInfo,    // BEGIN
				SeverityInfo,    // BEGIN (second BEGIN doesn't increase depth in PostgreSQL)
				SeverityError,   // VACUUM (still in transaction - ERROR)
				SeverityInfo,    // COMMIT
				SeverityWarning, // VACUUM (outside transaction - OK)
			},
		},
		{
			name: "Transaction tracking - VACUUM cannot run in transaction",
			sql: `
                VACUUM users;
                BEGIN;
                VACUUM logs;
                COMMIT;
            `,
			mode: NoTransaction,
			expectedSeverities: []Severity{
				SeverityWarning, // VACUUM (outside transaction - OK)
				SeverityInfo,    // BEGIN
				SeverityError,   // VACUUM (inside transaction - ERROR!)
				SeverityInfo,    // COMMIT
			},
			expectedOps: []string{
				"VACUUM",
				"BEGIN",
				"VACUUM",
				"COMMIT",
			},
		},
		{
			name: "Transaction tracking - operations change severity based on transaction state",
			sql: `
                CREATE INDEX CONCURRENTLY idx1 ON users(email);
                CREATE INDEX idx2 ON users(name);
                BEGIN;
                CREATE INDEX idx3 ON users(active);
                CREATE INDEX CONCURRENTLY idx4 ON users(created_at);
                COMMIT;
                CREATE INDEX idx5 ON users(updated_at);
                CREATE INDEX CONCURRENTLY idx6 ON users(deleted_at);
            `,
			mode: NoTransaction,
			expectedSeverities: []Severity{
				SeverityWarning,  // CREATE INDEX CONCURRENTLY (outside - WARNING)
				SeverityCritical, // CREATE INDEX (always CRITICAL)
				SeverityInfo,     // BEGIN
				SeverityCritical, // CREATE INDEX (always CRITICAL)
				SeverityError,    // CREATE INDEX CONCURRENTLY (inside - ERROR!)
				SeverityInfo,     // COMMIT
				SeverityCritical, // CREATE INDEX (always CRITICAL)
				SeverityWarning,  // CREATE INDEX CONCURRENTLY (outside - WARNING)
			},
		},
		{
			name: "Transaction tracking - multiple transaction sequences",
			sql: `
                BEGIN;
                UPDATE users SET active = true WHERE id = 1;
                COMMIT;
                BEGIN;
                DELETE FROM logs WHERE id = 1;
                ROLLBACK;
                INSERT INTO audit_log (message) VALUES ('rolled back');
            `,
			mode: NoTransaction,
			expectedSeverities: []Severity{
				SeverityInfo,    // BEGIN
				SeverityWarning, // UPDATE with WHERE (in transaction)
				SeverityInfo,    // COMMIT
				SeverityInfo,    // BEGIN
				SeverityWarning, // DELETE with WHERE (in transaction)
				SeverityInfo,    // ROLLBACK
				SeverityInfo,    // INSERT (outside transaction)
			},
		},
		{
			name: "Transaction tracking - START TRANSACTION alternative syntax",
			sql: `
                START TRANSACTION;
                TRUNCATE users;
                COMMIT;
            `,
			mode: NoTransaction,
			expectedSeverities: []Severity{
				SeverityInfo,     // START TRANSACTION
				SeverityCritical, // TRUNCATE (in transaction)
				SeverityInfo,     // COMMIT
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := New()
			p := parser.NewParser()

			parsed, err := p.ParseSQL(tt.sql)
			if err != nil {
				t.Fatalf("Failed to parse SQL: %v", err)
			}

			results, err := a.Analyze(parsed, tt.mode)
			if err != nil {
				t.Fatalf("Failed to analyze statements: %v", err)
			}

			if len(results) != len(tt.expectedSeverities) {
				t.Fatalf("Expected %d results, got %d", len(tt.expectedSeverities), len(results))
			}

			for i, expected := range tt.expectedSeverities {
				if results[i].Severity != expected {
					t.Errorf("Statement %d: expected severity %s, got %s",
						i+1, expected, results[i].Severity)
				}

				if tt.expectedOps != nil && i < len(tt.expectedOps) {
					if results[i].Operation() != tt.expectedOps[i] {
						t.Errorf("Statement %d: expected operation %s, got %s",
							i+1, tt.expectedOps[i], results[i].Operation())
					}
				}
			}
		})
	}
}

// ===== HELPER FUNCTIONS =====

func runAnalyzerTests(t *testing.T, tests []struct {
	name             string
	sql              string
	mode             TransactionMode
	expectedSeverity Severity
	expectedOp       string
	expectedLocks    map[string]string
}) {
	a := New()
	p := parser.NewParser()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := p.ParseSQL(tt.sql)
			if err != nil {
				t.Fatalf("Failed to parse SQL: %v", err)
			}

			result, err := a.AnalyzeStatement(parsed.Statements[0], tt.mode)
			if err != nil {
				t.Fatalf("Failed to analyze: %v", err)
			}

			if result.Severity != tt.expectedSeverity {
				t.Errorf("Expected severity %s, got %s", tt.expectedSeverity, result.Severity)
			}

			if tt.expectedOp != "" && result.Operation() != tt.expectedOp {
				t.Errorf("Expected operation %s, got %s", tt.expectedOp, result.Operation())
			}

			if tt.expectedLocks != nil {
				for table, expectedLock := range tt.expectedLocks {
					found := false
					for _, tl := range result.TableLocks() {
						if strings.Contains(tl, table) && strings.Contains(tl, expectedLock) {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected lock %s on table %s, not found in %v",
							expectedLock, table, result.TableLocks())
					}
				}
			}
		})
	}
}


// ===== UNKNOWN OPERATIONS TEST =====

func TestAnalyzer_UnknownOperations(t *testing.T) {
	// Test that analyzer handles unknown/future PostgreSQL operations gracefully
	tests := []struct {
		name             string
		sql              string
		mode             TransactionMode
		expectedSeverity Severity
	}{
		{
			name:             "Future SQL command",
			sql:              "FUTURE_COMMAND table_name", // Hypothetical future command
			mode:             InTransaction,
			expectedSeverity: SeverityInfo, // Should default to INFO for unknown
		},
	}

	// Note: These tests will fail parsing, but we include them to ensure
	// the analyzer would handle unknown AST nodes gracefully if they existed
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This will fail at parse stage, which is expected
			// The test documents our intention to handle unknown operations
			t.Skip("Unknown operations fail at parse stage - documenting intended behavior")
		})
	}
}
