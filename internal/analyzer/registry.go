package analyzer

// operationRegistry holds the mapping of operations to their severity and lock types
type operationRegistry struct {
	operations map[string]map[TransactionMode]*registryOperationInfo
}

// registryOperationInfo stores severity and lock type for an operation
type registryOperationInfo struct {
	severity Severity
	lockType LockType
}

// newOperationRegistry creates and initializes the operation registry
func newOperationRegistry() *operationRegistry {
	r := &operationRegistry{
		operations: make(map[string]map[TransactionMode]*registryOperationInfo),
	}
	r.initializeOperations()
	return r
}

// getSeverityAndLock returns the severity and lock type for an operation
func (r *operationRegistry) getSeverityAndLock(operation string, mode TransactionMode) (Severity, LockType) {
	if modeMap, exists := r.operations[operation]; exists {
		if info, exists := modeMap[mode]; exists {
			return info.severity, info.lockType
		}
	}
	// Default to INFO and AccessShare for unknown operations
	return SeverityInfo, AccessShare
}

// register adds an operation to the registry
func (r *operationRegistry) register(operation string, inTxn, noTxn *registryOperationInfo) {
	r.operations[operation] = map[TransactionMode]*registryOperationInfo{
		InTransaction: inTxn,
		NoTransaction: noTxn,
	}
}

// initializeOperations populates the registry with all operations from lock_severity.md
func (r *operationRegistry) initializeOperations() {
	// ERROR operations (only in transaction mode)
	r.register("CREATE DATABASE",
		&registryOperationInfo{SeverityError, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("DROP DATABASE",
		&registryOperationInfo{SeverityError, AccessExclusive},
		&registryOperationInfo{SeverityCritical, Exclusive})
	r.register("CREATE TABLESPACE",
		&registryOperationInfo{SeverityError, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("DROP TABLESPACE",
		&registryOperationInfo{SeverityError, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("ALTER TABLESPACE",
		&registryOperationInfo{SeverityError, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("VACUUM",
		&registryOperationInfo{SeverityError, ShareUpdateExclusive},
		&registryOperationInfo{SeverityWarning, ShareUpdateExclusive})
	r.register("VACUUM FULL",
		&registryOperationInfo{SeverityError, AccessExclusive},
		&registryOperationInfo{SeverityCritical, AccessExclusive})
	r.register("VACUUM FREEZE",
		&registryOperationInfo{SeverityError, ShareUpdateExclusive},
		&registryOperationInfo{SeverityWarning, ShareUpdateExclusive})
	r.register("VACUUM ANALYZE",
		&registryOperationInfo{SeverityError, ShareUpdateExclusive},
		&registryOperationInfo{SeverityWarning, ShareUpdateExclusive})
	r.register("CREATE INDEX CONCURRENTLY",
		&registryOperationInfo{SeverityError, ShareUpdateExclusive},
		&registryOperationInfo{SeverityWarning, ShareUpdateExclusive})
	r.register("DROP INDEX CONCURRENTLY",
		&registryOperationInfo{SeverityError, ShareUpdateExclusive},
		&registryOperationInfo{SeverityWarning, ShareUpdateExclusive})
	r.register("REINDEX CONCURRENTLY",
		&registryOperationInfo{SeverityError, ShareUpdateExclusive},
		&registryOperationInfo{SeverityWarning, ShareUpdateExclusive})
	r.register("REFRESH MATERIALIZED VIEW CONCURRENTLY",
		&registryOperationInfo{SeverityError, Exclusive},
		&registryOperationInfo{SeverityWarning, Exclusive})
	r.register("ALTER SYSTEM",
		&registryOperationInfo{SeverityError, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("CREATE SUBSCRIPTION",
		&registryOperationInfo{SeverityError, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("ALTER SUBSCRIPTION",
		&registryOperationInfo{SeverityError, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("DROP SUBSCRIPTION",
		&registryOperationInfo{SeverityError, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("ALTER TYPE ADD VALUE",
		&registryOperationInfo{SeverityError, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("ALTER TABLE DETACH PARTITION CONCURRENTLY",
		&registryOperationInfo{SeverityError, ShareUpdateExclusive},
		&registryOperationInfo{SeverityWarning, ShareUpdateExclusive})

	// CRITICAL operations
	r.register("UPDATE without WHERE",
		&registryOperationInfo{SeverityCritical, RowExclusive},
		&registryOperationInfo{SeverityCritical, RowExclusive})
	r.register("DELETE without WHERE",
		&registryOperationInfo{SeverityCritical, RowExclusive},
		&registryOperationInfo{SeverityCritical, RowExclusive})
	r.register("MERGE without WHERE",
		&registryOperationInfo{SeverityCritical, RowExclusive},
		&registryOperationInfo{SeverityCritical, RowExclusive})
	r.register("TRUNCATE",
		&registryOperationInfo{SeverityCritical, AccessExclusive},
		&registryOperationInfo{SeverityCritical, AccessExclusive})
	r.register("DROP TABLE",
		&registryOperationInfo{SeverityCritical, AccessExclusive},
		&registryOperationInfo{SeverityCritical, AccessExclusive})
	r.register("DROP INDEX",
		&registryOperationInfo{SeverityCritical, AccessExclusive},
		&registryOperationInfo{SeverityCritical, AccessExclusive})
	r.register("DROP SCHEMA",
		&registryOperationInfo{SeverityCritical, AccessExclusive},
		&registryOperationInfo{SeverityCritical, AccessExclusive})
	r.register("DROP SCHEMA CASCADE",
		&registryOperationInfo{SeverityCritical, AccessExclusive},
		&registryOperationInfo{SeverityCritical, AccessExclusive})
	r.register("DROP OWNED",
		&registryOperationInfo{SeverityCritical, AccessExclusive},
		&registryOperationInfo{SeverityCritical, AccessExclusive})
	r.register("CREATE INDEX",
		&registryOperationInfo{SeverityCritical, Share},
		&registryOperationInfo{SeverityCritical, Share})
	r.register("CREATE UNIQUE INDEX",
		&registryOperationInfo{SeverityCritical, Share},
		&registryOperationInfo{SeverityCritical, Share})
	r.register("REINDEX",
		&registryOperationInfo{SeverityCritical, AccessExclusive},
		&registryOperationInfo{SeverityCritical, AccessExclusive})
	r.register("REINDEX TABLE",
		&registryOperationInfo{SeverityCritical, AccessExclusive},
		&registryOperationInfo{SeverityCritical, AccessExclusive})
	r.register("REINDEX DATABASE",
		&registryOperationInfo{SeverityCritical, AccessExclusive},
		&registryOperationInfo{SeverityCritical, AccessExclusive})
	r.register("REINDEX SCHEMA",
		&registryOperationInfo{SeverityCritical, AccessExclusive},
		&registryOperationInfo{SeverityCritical, AccessExclusive})
	r.register("REINDEX SYSTEM",
		&registryOperationInfo{SeverityCritical, AccessExclusive},
		&registryOperationInfo{SeverityCritical, AccessExclusive})
	r.register("CLUSTER",
		&registryOperationInfo{SeverityCritical, AccessExclusive},
		&registryOperationInfo{SeverityCritical, AccessExclusive})
	r.register("REFRESH MATERIALIZED VIEW",
		&registryOperationInfo{SeverityCritical, AccessExclusive},
		&registryOperationInfo{SeverityCritical, AccessExclusive})
	r.register("ALTER TABLE ADD COLUMN with volatile DEFAULT",
		&registryOperationInfo{SeverityCritical, AccessExclusive},
		&registryOperationInfo{SeverityCritical, AccessExclusive})
	r.register("ALTER TABLE DROP COLUMN",
		&registryOperationInfo{SeverityCritical, AccessExclusive},
		&registryOperationInfo{SeverityCritical, AccessExclusive})
	r.register("ALTER TABLE ALTER COLUMN TYPE",
		&registryOperationInfo{SeverityCritical, AccessExclusive},
		&registryOperationInfo{SeverityCritical, AccessExclusive})
	r.register("ALTER TABLE SET TABLESPACE",
		&registryOperationInfo{SeverityCritical, AccessExclusive},
		&registryOperationInfo{SeverityCritical, AccessExclusive})
	r.register("ALTER TABLE SET LOGGED",
		&registryOperationInfo{SeverityCritical, AccessExclusive},
		&registryOperationInfo{SeverityCritical, AccessExclusive})
	r.register("ALTER TABLE SET UNLOGGED",
		&registryOperationInfo{SeverityCritical, AccessExclusive},
		&registryOperationInfo{SeverityCritical, AccessExclusive})
	r.register("ALTER TABLE ADD PRIMARY KEY",
		&registryOperationInfo{SeverityCritical, AccessExclusive},
		&registryOperationInfo{SeverityCritical, AccessExclusive})
	r.register("ALTER TABLE RENAME TO",
		&registryOperationInfo{SeverityCritical, AccessExclusive},
		&registryOperationInfo{SeverityCritical, AccessExclusive})
	r.register("ALTER TABLE SET SCHEMA",
		&registryOperationInfo{SeverityCritical, AccessExclusive},
		&registryOperationInfo{SeverityCritical, AccessExclusive})
	r.register("ALTER TABLE ADD CONSTRAINT CHECK",
		&registryOperationInfo{SeverityCritical, AccessExclusive},
		&registryOperationInfo{SeverityCritical, AccessExclusive})
	r.register("ALTER TABLE SET NOT NULL",
		&registryOperationInfo{SeverityCritical, AccessExclusive},
		&registryOperationInfo{SeverityCritical, AccessExclusive})
	r.register("ALTER TABLE DROP NOT NULL",
		&registryOperationInfo{SeverityCritical, AccessExclusive},
		&registryOperationInfo{SeverityCritical, AccessExclusive})
	r.register("LOCK TABLE ACCESS EXCLUSIVE",
		&registryOperationInfo{SeverityCritical, AccessExclusive},
		&registryOperationInfo{SeverityCritical, AccessExclusive})

	// WARNING operations
	r.register("UPDATE with WHERE",
		&registryOperationInfo{SeverityWarning, RowExclusive},
		&registryOperationInfo{SeverityWarning, RowExclusive})
	r.register("DELETE with WHERE",
		&registryOperationInfo{SeverityWarning, RowExclusive},
		&registryOperationInfo{SeverityWarning, RowExclusive})
	r.register("MERGE with WHERE",
		&registryOperationInfo{SeverityWarning, RowExclusive},
		&registryOperationInfo{SeverityWarning, RowExclusive})
	r.register("SELECT FOR UPDATE without WHERE",
		&registryOperationInfo{SeverityWarning, RowShare},
		&registryOperationInfo{SeverityWarning, RowShare})
	r.register("SELECT FOR UPDATE with WHERE",
		&registryOperationInfo{SeverityInfo, RowShare},
		&registryOperationInfo{SeverityInfo, RowShare})
	r.register("SELECT FOR NO KEY UPDATE without WHERE",
		&registryOperationInfo{SeverityWarning, RowShare},
		&registryOperationInfo{SeverityWarning, RowShare})
	r.register("SELECT FOR NO KEY UPDATE with WHERE",
		&registryOperationInfo{SeverityInfo, RowShare},
		&registryOperationInfo{SeverityInfo, RowShare})
	r.register("SELECT FOR SHARE without WHERE",
		&registryOperationInfo{SeverityWarning, RowShare},
		&registryOperationInfo{SeverityWarning, RowShare})
	r.register("SELECT FOR SHARE with WHERE",
		&registryOperationInfo{SeverityInfo, RowShare},
		&registryOperationInfo{SeverityInfo, RowShare})
	r.register("INSERT SELECT",
		&registryOperationInfo{SeverityWarning, RowExclusive},
		&registryOperationInfo{SeverityWarning, RowExclusive})
	r.register("CREATE TABLE AS",
		&registryOperationInfo{SeverityWarning, AccessShare},
		&registryOperationInfo{SeverityWarning, AccessShare})
	r.register("SELECT INTO",
		&registryOperationInfo{SeverityWarning, AccessShare},
		&registryOperationInfo{SeverityWarning, AccessShare})
	r.register("COPY FROM",
		&registryOperationInfo{SeverityWarning, RowExclusive},
		&registryOperationInfo{SeverityWarning, RowExclusive})
	r.register("ANALYZE",
		&registryOperationInfo{SeverityWarning, ShareUpdateExclusive},
		&registryOperationInfo{SeverityWarning, ShareUpdateExclusive})
	r.register("CREATE TRIGGER",
		&registryOperationInfo{SeverityWarning, ShareRowExclusive},
		&registryOperationInfo{SeverityWarning, ShareRowExclusive})
	r.register("DROP TRIGGER",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("ALTER TABLE ADD FOREIGN KEY",
		&registryOperationInfo{SeverityWarning, ShareRowExclusive},
		&registryOperationInfo{SeverityWarning, ShareRowExclusive})
	r.register("ALTER TABLE ADD CONSTRAINT UNIQUE",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("ALTER TABLE ADD CONSTRAINT EXCLUDE",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("ALTER TABLE ADD CONSTRAINT NOT VALID",
		&registryOperationInfo{SeverityWarning, ShareRowExclusive},
		&registryOperationInfo{SeverityWarning, ShareRowExclusive})
	r.register("ALTER TABLE VALIDATE CONSTRAINT",
		&registryOperationInfo{SeverityWarning, ShareUpdateExclusive},
		&registryOperationInfo{SeverityWarning, ShareUpdateExclusive})
	r.register("ALTER TABLE DROP CONSTRAINT",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("ALTER TABLE ENABLE TRIGGER",
		&registryOperationInfo{SeverityWarning, ShareRowExclusive},
		&registryOperationInfo{SeverityWarning, ShareRowExclusive})
	r.register("ALTER TABLE DISABLE TRIGGER",
		&registryOperationInfo{SeverityWarning, ShareRowExclusive},
		&registryOperationInfo{SeverityWarning, ShareRowExclusive})
	r.register("ALTER TABLE ENABLE RULE",
		&registryOperationInfo{SeverityWarning, ShareRowExclusive},
		&registryOperationInfo{SeverityWarning, ShareRowExclusive})
	r.register("ALTER TABLE DISABLE RULE",
		&registryOperationInfo{SeverityWarning, ShareRowExclusive},
		&registryOperationInfo{SeverityWarning, ShareRowExclusive})
	r.register("ALTER TABLE ENABLE ROW LEVEL SECURITY",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("ALTER TABLE DISABLE ROW LEVEL SECURITY",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("ALTER TABLE FORCE ROW LEVEL SECURITY",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("ALTER TABLE NO FORCE ROW LEVEL SECURITY",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("ALTER TABLE RENAME COLUMN",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("ALTER TABLE INHERIT",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("ALTER TABLE NO INHERIT",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("ALTER TABLE OF",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("ALTER TABLE NOT OF",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("ALTER TABLE REPLICA IDENTITY",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("ALTER TABLE OWNER TO",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("ALTER TABLE ATTACH PARTITION",
		&registryOperationInfo{SeverityWarning, ShareUpdateExclusive},
		&registryOperationInfo{SeverityWarning, ShareUpdateExclusive})
	r.register("ALTER TABLE DETACH PARTITION",
		&registryOperationInfo{SeverityWarning, ShareUpdateExclusive},
		&registryOperationInfo{SeverityWarning, ShareUpdateExclusive})
	r.register("ALTER TABLE SET ACCESS METHOD",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("DROP VIEW",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("DROP MATERIALIZED VIEW",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("DROP SEQUENCE",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("DROP TYPE",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("DROP DOMAIN",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("DROP EXTENSION",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("CREATE RULE",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("DROP RULE",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("CREATE POLICY",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("DROP POLICY",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("ALTER INDEX",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("ALTER VIEW",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("ALTER VIEW RENAME TO",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("ALTER SEQUENCE",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("ALTER TYPE",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("ALTER DOMAIN",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("REASSIGN OWNED",
		&registryOperationInfo{SeverityWarning, AccessExclusive},
		&registryOperationInfo{SeverityWarning, AccessExclusive})
	r.register("LOCK TABLE ROW EXCLUSIVE",
		&registryOperationInfo{SeverityWarning, RowExclusive},
		&registryOperationInfo{SeverityWarning, RowExclusive})
	r.register("LOCK TABLE SHARE UPDATE EXCLUSIVE",
		&registryOperationInfo{SeverityWarning, ShareUpdateExclusive},
		&registryOperationInfo{SeverityWarning, ShareUpdateExclusive})
	r.register("LOCK TABLE SHARE",
		&registryOperationInfo{SeverityWarning, Share},
		&registryOperationInfo{SeverityWarning, Share})
	r.register("LOCK TABLE SHARE ROW EXCLUSIVE",
		&registryOperationInfo{SeverityWarning, ShareRowExclusive},
		&registryOperationInfo{SeverityWarning, ShareRowExclusive})
	r.register("LOCK TABLE EXCLUSIVE",
		&registryOperationInfo{SeverityWarning, Exclusive},
		&registryOperationInfo{SeverityWarning, Exclusive})

	// INFO operations
	r.register("SELECT FOR UPDATE",
		&registryOperationInfo{SeverityInfo, RowShare},
		&registryOperationInfo{SeverityInfo, RowShare})
	r.register("SELECT FOR NO KEY UPDATE",
		&registryOperationInfo{SeverityInfo, RowShare},
		&registryOperationInfo{SeverityInfo, RowShare})
	r.register("SELECT FOR SHARE",
		&registryOperationInfo{SeverityInfo, RowShare},
		&registryOperationInfo{SeverityInfo, RowShare})
	r.register("SELECT FOR KEY SHARE",
		&registryOperationInfo{SeverityInfo, RowShare},
		&registryOperationInfo{SeverityInfo, RowShare})
	r.register("INSERT",
		&registryOperationInfo{SeverityInfo, RowExclusive},
		&registryOperationInfo{SeverityInfo, RowExclusive})
	r.register("INSERT ON CONFLICT",
		&registryOperationInfo{SeverityInfo, RowExclusive},
		&registryOperationInfo{SeverityInfo, RowExclusive})
	r.register("INSERT RETURNING",
		&registryOperationInfo{SeverityInfo, RowExclusive},
		&registryOperationInfo{SeverityInfo, RowExclusive})
	r.register("COPY TO",
		&registryOperationInfo{SeverityInfo, AccessShare},
		&registryOperationInfo{SeverityInfo, AccessShare})
	r.register("ALTER TABLE ADD COLUMN without DEFAULT",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("ALTER TABLE ADD COLUMN with constant DEFAULT",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("ALTER TABLE ADD COLUMN GENERATED ALWAYS AS",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("ALTER TABLE ALTER COLUMN ADD IDENTITY",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("ALTER TABLE ALTER COLUMN DROP IDENTITY",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("ALTER TABLE SET DEFAULT",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("ALTER TABLE DROP DEFAULT",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("ALTER TABLE ALTER COLUMN SET STATISTICS",
		&registryOperationInfo{SeverityInfo, ShareUpdateExclusive},
		&registryOperationInfo{SeverityInfo, ShareUpdateExclusive})
	r.register("ALTER TABLE ALTER COLUMN SET STORAGE",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("ALTER TABLE SET",
		&registryOperationInfo{SeverityInfo, ShareUpdateExclusive},
		&registryOperationInfo{SeverityInfo, ShareUpdateExclusive})
	r.register("ALTER TABLE RESET",
		&registryOperationInfo{SeverityInfo, ShareUpdateExclusive},
		&registryOperationInfo{SeverityInfo, ShareUpdateExclusive})
	r.register("ALTER TABLE CLUSTER ON",
		&registryOperationInfo{SeverityInfo, ShareUpdateExclusive},
		&registryOperationInfo{SeverityInfo, ShareUpdateExclusive})
	r.register("ALTER TABLE SET WITHOUT CLUSTER",
		&registryOperationInfo{SeverityInfo, ShareUpdateExclusive},
		&registryOperationInfo{SeverityInfo, ShareUpdateExclusive})
	r.register("CREATE TABLE",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("CREATE TEMPORARY TABLE",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("CREATE VIEW",
		&registryOperationInfo{SeverityInfo, AccessShare},
		&registryOperationInfo{SeverityInfo, AccessShare})
	r.register("CREATE MATERIALIZED VIEW",
		&registryOperationInfo{SeverityInfo, AccessShare},
		&registryOperationInfo{SeverityInfo, AccessShare})
	r.register("CREATE SEQUENCE",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("CREATE TYPE",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("CREATE DOMAIN",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("CREATE SCHEMA",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("CREATE EXTENSION",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("CREATE FUNCTION",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("DROP FUNCTION",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("CREATE PROCEDURE",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("DROP PROCEDURE",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("CREATE AGGREGATE",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("DROP AGGREGATE",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("CREATE OPERATOR",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("DROP OPERATOR",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("CREATE CAST",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("DROP CAST",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("CREATE COLLATION",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("DROP COLLATION",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("CREATE TEXT SEARCH CONFIGURATION",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("DROP TEXT SEARCH CONFIGURATION",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("CREATE TEXT SEARCH DICTIONARY",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("DROP TEXT SEARCH DICTIONARY",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("CREATE TEXT SEARCH PARSER",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("DROP TEXT SEARCH PARSER",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("CREATE TEXT SEARCH TEMPLATE",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("DROP TEXT SEARCH TEMPLATE",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("CREATE STATISTICS",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("DROP STATISTICS",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("CREATE EVENT TRIGGER",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("DROP EVENT TRIGGER",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("CREATE FOREIGN DATA WRAPPER",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("DROP FOREIGN DATA WRAPPER",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("CREATE SERVER",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("DROP SERVER",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("CREATE USER MAPPING",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("DROP USER MAPPING",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("CREATE PUBLICATION",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("DROP PUBLICATION",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("ALTER PUBLICATION ADD/DROP TABLE",
		&registryOperationInfo{SeverityInfo, ShareUpdateExclusive},
		&registryOperationInfo{SeverityInfo, ShareUpdateExclusive})
	r.register("ALTER DEFAULT PRIVILEGES",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("GRANT",
		&registryOperationInfo{SeverityInfo, AccessShare},
		&registryOperationInfo{SeverityInfo, AccessShare})
	r.register("REVOKE",
		&registryOperationInfo{SeverityInfo, AccessShare},
		&registryOperationInfo{SeverityInfo, AccessShare})
	r.register("GRANT ON TABLE",
		&registryOperationInfo{SeverityInfo, AccessShare},
		&registryOperationInfo{SeverityInfo, AccessShare})
	r.register("REVOKE ON TABLE",
		&registryOperationInfo{SeverityInfo, AccessShare},
		&registryOperationInfo{SeverityInfo, AccessShare})
	r.register("GRANT ON SCHEMA",
		&registryOperationInfo{SeverityInfo, AccessShare},
		&registryOperationInfo{SeverityInfo, AccessShare})
	r.register("REVOKE ON SCHEMA",
		&registryOperationInfo{SeverityInfo, AccessShare},
		&registryOperationInfo{SeverityInfo, AccessShare})
	r.register("GRANT ON DATABASE",
		&registryOperationInfo{SeverityInfo, AccessShare},
		&registryOperationInfo{SeverityInfo, AccessShare})
	r.register("REVOKE ON DATABASE",
		&registryOperationInfo{SeverityInfo, AccessShare},
		&registryOperationInfo{SeverityInfo, AccessShare})
	r.register("CREATE ROLE",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("CREATE USER",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("DROP ROLE",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("ALTER ROLE",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("COMMENT ON",
		&registryOperationInfo{SeverityInfo, AccessShare},
		&registryOperationInfo{SeverityInfo, AccessShare})
	r.register("LOCK TABLE ACCESS SHARE",
		&registryOperationInfo{SeverityInfo, AccessShare},
		&registryOperationInfo{SeverityInfo, AccessShare})
	r.register("LOCK TABLE ROW SHARE",
		&registryOperationInfo{SeverityInfo, RowShare},
		&registryOperationInfo{SeverityInfo, RowShare})
	r.register("BEGIN",
		&registryOperationInfo{SeverityInfo, AccessShare},
		&registryOperationInfo{SeverityInfo, AccessShare})
	r.register("START TRANSACTION",
		&registryOperationInfo{SeverityInfo, AccessShare},
		&registryOperationInfo{SeverityInfo, AccessShare})
	r.register("COMMIT",
		&registryOperationInfo{SeverityInfo, AccessShare},
		&registryOperationInfo{SeverityInfo, AccessShare})
	r.register("END",
		&registryOperationInfo{SeverityInfo, AccessShare},
		&registryOperationInfo{SeverityInfo, AccessShare})
	r.register("ROLLBACK",
		&registryOperationInfo{SeverityInfo, AccessShare},
		&registryOperationInfo{SeverityInfo, AccessShare})
	r.register("SAVEPOINT",
		&registryOperationInfo{SeverityInfo, AccessShare},
		&registryOperationInfo{SeverityInfo, AccessShare})
	r.register("RELEASE SAVEPOINT",
		&registryOperationInfo{SeverityInfo, AccessShare},
		&registryOperationInfo{SeverityInfo, AccessShare})
	r.register("ROLLBACK TO SAVEPOINT",
		&registryOperationInfo{SeverityInfo, AccessShare},
		&registryOperationInfo{SeverityInfo, AccessShare})
	r.register("SET TRANSACTION",
		&registryOperationInfo{SeverityInfo, AccessShare},
		&registryOperationInfo{SeverityInfo, AccessShare})
	r.register("SET LOCAL",
		&registryOperationInfo{SeverityInfo, AccessShare},
		&registryOperationInfo{SeverityInfo, AccessShare})
	r.register("SET",
		&registryOperationInfo{SeverityInfo, AccessShare},
		&registryOperationInfo{SeverityInfo, AccessShare})
	r.register("RESET",
		&registryOperationInfo{SeverityInfo, AccessShare},
		&registryOperationInfo{SeverityInfo, AccessShare})

	// No-transaction mode specific
	r.register("ALTER DATABASE",
		&registryOperationInfo{SeverityInfo, AccessExclusive},
		&registryOperationInfo{SeverityInfo, AccessExclusive})
	r.register("CHECKPOINT",
		&registryOperationInfo{SeverityInfo, AccessShare},
		&registryOperationInfo{SeverityInfo, AccessShare})
	r.register("LOAD",
		&registryOperationInfo{SeverityInfo, AccessShare},
		&registryOperationInfo{SeverityInfo, AccessShare})
}
