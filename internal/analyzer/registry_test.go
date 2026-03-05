package analyzer

import (
	"testing"
)

func TestRegistryOperations(t *testing.T) {
	registry := newOperationRegistry()

	tests := []struct {
		name             string
		operation        string
		mode             TransactionMode
		expectedSeverity Severity
		expectedLock     LockType
	}{
		// ERROR operations (in transaction)
		{
			name:             "CREATE DATABASE in transaction",
			operation:        "CREATE DATABASE",
			mode:             InTransaction,
			expectedSeverity: SeverityError,
			expectedLock:     AccessExclusive,
		},
		{
			name:             "CREATE DATABASE no transaction",
			operation:        "CREATE DATABASE",
			mode:             NoTransaction,
			expectedSeverity: SeverityInfo,
			expectedLock:     AccessExclusive,
		},
		{
			name:             "DROP DATABASE in transaction",
			operation:        "DROP DATABASE",
			mode:             InTransaction,
			expectedSeverity: SeverityError,
			expectedLock:     AccessExclusive,
		},
		{
			name:             "DROP DATABASE no transaction",
			operation:        "DROP DATABASE",
			mode:             NoTransaction,
			expectedSeverity: SeverityCritical,
			expectedLock:     Exclusive,
		},
		{
			name:             "VACUUM in transaction",
			operation:        "VACUUM",
			mode:             InTransaction,
			expectedSeverity: SeverityError,
			expectedLock:     ShareUpdateExclusive,
		},
		{
			name:             "VACUUM no transaction",
			operation:        "VACUUM",
			mode:             NoTransaction,
			expectedSeverity: SeverityWarning,
			expectedLock:     ShareUpdateExclusive,
		},
		{
			name:             "VACUUM FULL in transaction",
			operation:        "VACUUM FULL",
			mode:             InTransaction,
			expectedSeverity: SeverityError,
			expectedLock:     AccessExclusive,
		},
		{
			name:             "VACUUM FULL no transaction",
			operation:        "VACUUM FULL",
			mode:             NoTransaction,
			expectedSeverity: SeverityCritical,
			expectedLock:     AccessExclusive,
		},
		{
			name:             "CREATE INDEX CONCURRENTLY in transaction",
			operation:        "CREATE INDEX CONCURRENTLY",
			mode:             InTransaction,
			expectedSeverity: SeverityError,
			expectedLock:     ShareUpdateExclusive,
		},
		{
			name:             "CREATE INDEX CONCURRENTLY no transaction",
			operation:        "CREATE INDEX CONCURRENTLY",
			mode:             NoTransaction,
			expectedSeverity: SeverityWarning,
			expectedLock:     ShareUpdateExclusive,
		},
		{
			name:             "ALTER TYPE ADD VALUE in transaction",
			operation:        "ALTER TYPE ADD VALUE",
			mode:             InTransaction,
			expectedSeverity: SeverityError,
			expectedLock:     AccessExclusive,
		},

		// CRITICAL operations
		{
			name:             "UPDATE without WHERE in transaction",
			operation:        "UPDATE without WHERE",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedLock:     RowExclusive,
		},
		{
			name:             "UPDATE without WHERE no transaction",
			operation:        "UPDATE without WHERE",
			mode:             NoTransaction,
			expectedSeverity: SeverityCritical,
			expectedLock:     RowExclusive,
		},
		{
			name:             "DELETE without WHERE",
			operation:        "DELETE without WHERE",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedLock:     RowExclusive,
		},
		{
			name:             "MERGE without WHERE",
			operation:        "MERGE without WHERE",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedLock:     RowExclusive,
		},
		{
			name:             "TRUNCATE",
			operation:        "TRUNCATE",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedLock:     AccessExclusive,
		},
		{
			name:             "DROP TABLE",
			operation:        "DROP TABLE",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedLock:     AccessExclusive,
		},
		{
			name:             "CREATE INDEX in transaction",
			operation:        "CREATE INDEX",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedLock:     Share,
		},
		{
			name:             "CREATE INDEX no transaction",
			operation:        "CREATE INDEX",
			mode:             NoTransaction,
			expectedSeverity: SeverityCritical,
			expectedLock:     Share,
		},
		{
			name:             "REINDEX",
			operation:        "REINDEX",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedLock:     AccessExclusive,
		},
		{
			name:             "CLUSTER",
			operation:        "CLUSTER",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedLock:     AccessExclusive,
		},
		{
			name:             "REFRESH MATERIALIZED VIEW",
			operation:        "REFRESH MATERIALIZED VIEW",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedLock:     AccessExclusive,
		},
		{
			name:             "ALTER TABLE ADD COLUMN with volatile DEFAULT",
			operation:        "ALTER TABLE ADD COLUMN with volatile DEFAULT",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedLock:     AccessExclusive,
		},
		{
			name:             "ALTER TABLE ALTER COLUMN TYPE",
			operation:        "ALTER TABLE ALTER COLUMN TYPE",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedLock:     AccessExclusive,
		},
		{
			name:             "ALTER TABLE ADD PRIMARY KEY",
			operation:        "ALTER TABLE ADD PRIMARY KEY",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedLock:     AccessExclusive,
		},
		{
			name:             "LOCK TABLE ACCESS EXCLUSIVE",
			operation:        "LOCK TABLE ACCESS EXCLUSIVE",
			mode:             InTransaction,
			expectedSeverity: SeverityCritical,
			expectedLock:     AccessExclusive,
		},

		// WARNING operations
		{
			name:             "UPDATE with WHERE",
			operation:        "UPDATE with WHERE",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedLock:     RowExclusive,
		},
		{
			name:             "DELETE with WHERE",
			operation:        "DELETE with WHERE",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedLock:     RowExclusive,
		},
		{
			name:             "ANALYZE",
			operation:        "ANALYZE",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedLock:     ShareUpdateExclusive,
		},
		{
			name:             "CREATE TRIGGER",
			operation:        "CREATE TRIGGER",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedLock:     ShareRowExclusive,
		},
		{
			name:             "ALTER TABLE ADD FOREIGN KEY",
			operation:        "ALTER TABLE ADD FOREIGN KEY",
			mode:             InTransaction,
			expectedSeverity: SeverityWarning,
			expectedLock:     ShareRowExclusive,
		},

		// INFO operations
		{
			name:             "SELECT",
			operation:        "SELECT",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedLock:     AccessShare,
		},
		{
			name:             "INSERT",
			operation:        "INSERT",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedLock:     RowExclusive,
		},
		{
			name:             "CREATE TABLE",
			operation:        "CREATE TABLE",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedLock:     AccessExclusive,
		},
		{
			name:             "CREATE VIEW",
			operation:        "CREATE VIEW",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedLock:     AccessShare,
		},
		{
			name:             "BEGIN",
			operation:        "BEGIN",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedLock:     AccessShare,
		},
		{
			name:             "COMMIT",
			operation:        "COMMIT",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedLock:     AccessShare,
		},
		{
			name:             "GRANT",
			operation:        "GRANT",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedLock:     AccessShare,
		},
		{
			name:             "ALTER TABLE ADD COLUMN without DEFAULT",
			operation:        "ALTER TABLE ADD COLUMN without DEFAULT",
			mode:             InTransaction,
			expectedSeverity: SeverityInfo,
			expectedLock:     AccessExclusive,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			severity, lockType := registry.getSeverityAndLock(tt.operation, tt.mode)
			if severity != tt.expectedSeverity {
				t.Errorf("getSeverityAndLock(%q, %v): severity = %v, want %v",
					tt.operation, tt.mode, severity, tt.expectedSeverity)
			}
			if lockType != tt.expectedLock {
				t.Errorf("getSeverityAndLock(%q, %v): lockType = %v, want %v",
					tt.operation, tt.mode, lockType, tt.expectedLock)
			}
		})
	}
}

func TestRegistryUnknownOperation(t *testing.T) {
	registry := newOperationRegistry()

	severity, lockType := registry.getSeverityAndLock("UNKNOWN_OPERATION", InTransaction)
	if severity != SeverityInfo {
		t.Errorf("Unknown operation severity = %v, want %v", severity, SeverityInfo)
	}
	if lockType != AccessShare {
		t.Errorf("Unknown operation lockType = %v, want %v", lockType, AccessShare)
	}

	severity, lockType = registry.getSeverityAndLock("UNKNOWN_OPERATION", NoTransaction)
	if severity != SeverityInfo {
		t.Errorf("Unknown operation (NoTransaction) severity = %v, want %v", severity, SeverityInfo)
	}
	if lockType != AccessShare {
		t.Errorf("Unknown operation (NoTransaction) lockType = %v, want %v", lockType, AccessShare)
	}
}

func TestRegistryCompleteness(t *testing.T) {
	registry := newOperationRegistry()

	// Verify the registry has a substantial number of operations registered
	operationCount := len(registry.operations)
	if operationCount < 100 {
		t.Errorf("Registry has %d operations, expected at least 100", operationCount)
	}

	// Verify each registered operation has both transaction modes
	for op, modeMap := range registry.operations {
		if _, hasInTxn := modeMap[InTransaction]; !hasInTxn {
			t.Errorf("Operation %q missing InTransaction mode", op)
		}
		if _, hasNoTxn := modeMap[NoTransaction]; !hasNoTxn {
			t.Errorf("Operation %q missing NoTransaction mode", op)
		}
	}
}

func TestSeverityString(t *testing.T) {
	tests := []struct {
		severity Severity
		expected string
	}{
		{SeverityInfo, "INFO"},
		{SeverityWarning, "WARNING"},
		{SeverityCritical, "CRITICAL"},
		{SeverityError, "ERROR"},
		{Severity(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		if got := tt.severity.String(); got != tt.expected {
			t.Errorf("Severity(%d).String() = %q, want %q", tt.severity, got, tt.expected)
		}
	}
}

func TestTransactionModeString(t *testing.T) {
	tests := []struct {
		mode     TransactionMode
		expected string
	}{
		{InTransaction, "IN_TRANSACTION"},
		{NoTransaction, "NO_TRANSACTION"},
		{TransactionMode(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		if got := tt.mode.String(); got != tt.expected {
			t.Errorf("TransactionMode(%d).String() = %q, want %q", tt.mode, got, tt.expected)
		}
	}
}
