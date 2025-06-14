package analyzer

import (
	"fmt"
	"strings"

	"github.com/nnaka2992/pg-lock-check/internal/parser"
	"github.com/pganalyze/pg_query_go/v6"
)

// Analyzer analyzes SQL statements for lock severity
type Analyzer interface {
	// AnalyzeStatement analyzes a single parsed statement
	AnalyzeStatement(stmt parser.ParsedStatement, mode TransactionMode) (*Result, error)

	// Analyze analyzes all statements in a parsed result
	Analyze(parsed *parser.ParseResult, mode TransactionMode) ([]*Result, error)
}

// analyzer is the main implementation of the Analyzer interface
type analyzer struct {
	registry         *operationRegistry
	transactionDepth int // Track nesting level of transactions
}

// New creates a new analyzer instance
func New() Analyzer {
	return &analyzer{
		registry: newOperationRegistry(),
	}
}

// AnalyzeStatement analyzes a single parsed statement
func (a *analyzer) AnalyzeStatement(stmt parser.ParsedStatement, mode TransactionMode) (*Result, error) {
	if stmt.AST == nil || len(stmt.AST.Stmts) == 0 {
		return &Result{
			Severity:  SeverityInfo,
			operation: "UNKNOWN",
			lockType:  AccessShare,
		}, nil
	}

	// Get the first statement node from the AST
	stmtNode := stmt.AST.Stmts[0].Stmt

	// Analyze the AST node to determine operation type and details
	opInfo := a.analyzeNode(stmtNode, mode)
	if opInfo == nil {
		return nil, fmt.Errorf("unsupported SQL operation")
	}

	// Special handling for MERGE to detect WHERE conditions
	if opInfo.operation == "MERGE without WHERE" {
		upperSQL := strings.ToUpper(stmt.SQL)
		// MERGE is considered "with WHERE" if:
		// 1. It has additional conditions in WHEN clause (AND after MATCHED)
		// 2. It uses a subquery/CTE in USING clause (targeted merge)
		if strings.Contains(upperSQL, "WHEN MATCHED AND") ||
			strings.Contains(upperSQL, "USING (SELECT") {
			opInfo.operation = "MERGE with WHERE"
		}
	}

	// Special handling for DETACH PARTITION CONCURRENTLY
	if opInfo.operation == "ALTER TABLE DETACH PARTITION" && strings.Contains(strings.ToUpper(stmt.SQL), "CONCURRENTLY") {
		opInfo.operation = "ALTER TABLE DETACH PARTITION CONCURRENTLY"
	}

	// Get severity and lock information from the registry
	severity, lockType := a.registry.getSeverityAndLock(opInfo.operation, mode)

	// Extract table information with context-aware locks
	tableLocksMap := extractTablesWithContext(stmtNode)

	// If no tables were found with context, fall back to simple extraction
	if len(tableLocksMap) == 0 {
		tables := extractTables(stmtNode)
		tableLocksMap = make(map[string]LockType)
		for _, table := range tables {
			tableLocksMap[table] = lockType
		}
	}

	if opInfo.additionalTableLocks != nil {
		for table, lock := range opInfo.additionalTableLocks {
			if _, exists := tableLocksMap[table]; !exists {
				tableLocksMap[table] = lock
			}
		}
	}

	// For SELECT statements with locking, override the lock type
	if _, ok := stmtNode.Node.(*pg_query.Node_SelectStmt); ok {
		// If it's a SELECT with locking clause, use the operation's lock type
		if strings.Contains(opInfo.operation, "FOR") {
			for table := range tableLocksMap {
				tableLocksMap[table] = lockType
			}
		}
	}

	// Format table locks
	tableLocks := make([]string, 0, len(tableLocksMap))
	for table, lockType := range tableLocksMap {
		tableLocks = append(tableLocks, formatTableLock(table, lockType))
	}

	return &Result{
		Severity:   severity,
		operation:  opInfo.operation,
		lockType:   lockType,
		tableLocks: tableLocks,
		message:    opInfo.message,
	}, nil
}

// Analyze analyzes all statements in a parsed result
func (a *analyzer) Analyze(parsed *parser.ParseResult, mode TransactionMode) ([]*Result, error) {
	results := make([]*Result, 0, len(parsed.Statements))

	// Reset transaction depth for each analysis
	a.transactionDepth = 0

	// If the default mode is InTransaction, start with depth 1
	if mode == InTransaction {
		a.transactionDepth = 1
	}

	for _, stmt := range parsed.Statements {
		// Determine the effective mode based on transaction depth
		effectiveMode := NoTransaction
		if a.transactionDepth > 0 {
			effectiveMode = InTransaction
		}

		result, err := a.AnalyzeStatement(stmt, effectiveMode)
		if err != nil {
			return nil, err
		}

		// Update transaction depth based on the operation
		a.updateTransactionDepth(result.Operation())

		results = append(results, result)
	}

	return results, nil
}

// operationInfo holds information about an analyzed operation
type operationInfo struct {
	operation string
	tableLock LockType
	message   string
	// Additional table locks for multi-table operations
	additionalTableLocks map[string]LockType
}

// analyzeNode analyzes an AST node to determine the operation type
func (a *analyzer) analyzeNode(node *pg_query.Node, mode TransactionMode) *operationInfo {
	if node == nil {
		return &operationInfo{
			operation: "UNKNOWN",
			tableLock: AccessShare,
		}
	}

	// Check all possible statement types in the node
	switch n := node.Node.(type) {
	// DML Operations
	case *pg_query.Node_UpdateStmt:
		return a.analyzeUpdate(n.UpdateStmt)
	case *pg_query.Node_DeleteStmt:
		return a.analyzeDelete(n.DeleteStmt)
	case *pg_query.Node_InsertStmt:
		return a.analyzeInsert(n.InsertStmt)
	case *pg_query.Node_SelectStmt:
		return a.analyzeSelect(n.SelectStmt)
	case *pg_query.Node_MergeStmt:
		return a.analyzeMerge(n.MergeStmt)

	// DDL Operations - Tables
	case *pg_query.Node_CreateStmt:
		return a.analyzeCreate(n.CreateStmt)
	case *pg_query.Node_AlterTableStmt:
		return a.analyzeAlterTable(n.AlterTableStmt)
	case *pg_query.Node_AlterObjectSchemaStmt:
		return a.analyzeAlterObjectSchema(n.AlterObjectSchemaStmt)
	case *pg_query.Node_RenameStmt:
		return a.analyzeRename(n.RenameStmt)
	case *pg_query.Node_DropStmt:
		return a.analyzeDrop(n.DropStmt)
	case *pg_query.Node_TruncateStmt:
		return a.analyzeTruncate(n.TruncateStmt)

	// DDL Operations - Indexes
	case *pg_query.Node_IndexStmt:
		return a.analyzeIndex(n.IndexStmt)
	case *pg_query.Node_ReindexStmt:
		return a.analyzeReindex(n.ReindexStmt)

	// DDL Operations - Views
	case *pg_query.Node_ViewStmt:
		return a.analyzeCreateView(n.ViewStmt)
	case *pg_query.Node_CreateTableAsStmt:
		return a.analyzeCreateMatView(n.CreateTableAsStmt)
	case *pg_query.Node_RefreshMatViewStmt:
		return a.analyzeRefreshMatView(n.RefreshMatViewStmt)

	// DDL Operations - Other Objects
	case *pg_query.Node_CreateSchemaStmt:
		return a.analyzeCreateSchema(n.CreateSchemaStmt)
	case *pg_query.Node_CreateSeqStmt:
		return a.analyzeCreateSequence(n.CreateSeqStmt)
	case *pg_query.Node_AlterSeqStmt:
		return a.analyzeAlterSequence(n.AlterSeqStmt)
	case *pg_query.Node_CreateDomainStmt:
		return a.analyzeCreateDomain(n.CreateDomainStmt)
	case *pg_query.Node_CreateExtensionStmt:
		return a.analyzeCreateExtension(n.CreateExtensionStmt)
	case *pg_query.Node_AlterExtensionStmt:
		return a.analyzeAlterExtension(n.AlterExtensionStmt)
	case *pg_query.Node_AlterEnumStmt:
		return a.analyzeAlterType(n.AlterEnumStmt)

	// Database/Tablespace Operations
	case *pg_query.Node_CreatedbStmt:
		return a.analyzeCreateDatabase(n.CreatedbStmt)
	case *pg_query.Node_DropdbStmt:
		return &operationInfo{
			operation: "DROP DATABASE",
			tableLock: Exclusive,
		}
	case *pg_query.Node_AlterDatabaseStmt:
		return a.analyzeAlterDatabase(n.AlterDatabaseStmt)
	case *pg_query.Node_AlterDatabaseSetStmt:
		return &operationInfo{
			operation: "ALTER DATABASE",
			tableLock: Exclusive,
		}
	case *pg_query.Node_CreateTableSpaceStmt:
		return a.analyzeCreateTablespace(n.CreateTableSpaceStmt)
	case *pg_query.Node_DropTableSpaceStmt:
		return &operationInfo{
			operation: "DROP TABLESPACE",
			tableLock: AccessExclusive,
		}
	case *pg_query.Node_AlterTableSpaceOptionsStmt:
		return a.analyzeAlterTablespace(n.AlterTableSpaceOptionsStmt)

	// Maintenance Operations
	case *pg_query.Node_VacuumStmt:
		if !n.VacuumStmt.IsVacuumcmd {
			return a.analyzeAnalyze(n.VacuumStmt)
		}
		return a.analyzeVacuum(n.VacuumStmt)
	case *pg_query.Node_ClusterStmt:
		return a.analyzeCluster(n.ClusterStmt)
	case *pg_query.Node_CopyStmt:
		return a.analyzeCopy(n.CopyStmt)

	// Access Control
	case *pg_query.Node_GrantStmt:
		return a.analyzeGrant(n.GrantStmt)
	case *pg_query.Node_CreateRoleStmt:
		return a.analyzeCreateRole(n.CreateRoleStmt)
	case *pg_query.Node_AlterRoleStmt:
		return a.analyzeAlterRole(n.AlterRoleStmt)
	case *pg_query.Node_DropRoleStmt:
		return a.analyzeDropRole(n.DropRoleStmt)
	case *pg_query.Node_ReassignOwnedStmt:
		return a.analyzeReassignOwned(n.ReassignOwnedStmt)
	case *pg_query.Node_DropOwnedStmt:
		return &operationInfo{
			operation: "DROP OWNED",
			tableLock: AccessExclusive,
		}
	case *pg_query.Node_AlterDefaultPrivilegesStmt:
		return a.analyzeAlterDefaultPrivileges(n.AlterDefaultPrivilegesStmt)

	// Rules/Policies/Triggers
	case *pg_query.Node_CreateTrigStmt:
		return a.analyzeCreateTrigger(n.CreateTrigStmt)
	case *pg_query.Node_RuleStmt:
		return a.analyzeCreateRule(n.RuleStmt)
	case *pg_query.Node_CreatePolicyStmt:
		return a.analyzeCreatePolicy(n.CreatePolicyStmt)

	// Locking
	case *pg_query.Node_LockStmt:
		return a.analyzeLock(n.LockStmt)

	// Transaction Control
	case *pg_query.Node_TransactionStmt:
		return a.analyzeTransaction(n.TransactionStmt)

	// System Operations
	case *pg_query.Node_VariableSetStmt:
		return a.analyzeVariableSet(n.VariableSetStmt)
	case *pg_query.Node_AlterSystemStmt:
		return a.analyzeAlterSystem(n.AlterSystemStmt)
	case *pg_query.Node_CheckPointStmt:
		return a.analyzeCheckpoint(n.CheckPointStmt)
	case *pg_query.Node_LoadStmt:
		return a.analyzeLoad(n.LoadStmt)

	// Replication
	case *pg_query.Node_CreateSubscriptionStmt:
		return a.analyzeCreateSubscription(n.CreateSubscriptionStmt)
	case *pg_query.Node_AlterSubscriptionStmt:
		return a.analyzeAlterSubscription(n.AlterSubscriptionStmt)
	case *pg_query.Node_CreatePublicationStmt:
		return a.analyzeCreatePublication(n.CreatePublicationStmt)
	case *pg_query.Node_AlterPublicationStmt:
		return a.analyzeAlterPublication(n.AlterPublicationStmt)

	// Comments
	case *pg_query.Node_CommentStmt:
		return a.analyzeComment(n.CommentStmt)

	// Additional DDL Operations
	case *pg_query.Node_CreateEnumStmt:
		return &operationInfo{
			operation: "CREATE TYPE",
			tableLock: AccessExclusive,
		}
	case *pg_query.Node_AlterDomainStmt:
		return &operationInfo{
			operation: "ALTER DOMAIN",
			tableLock: AccessExclusive,
		}
	case *pg_query.Node_CreateFunctionStmt:
		// Check if it's a procedure
		if n.CreateFunctionStmt.IsProcedure {
			return &operationInfo{
				operation: "CREATE PROCEDURE",
				tableLock: AccessExclusive,
			}
		}
		return &operationInfo{
			operation: "CREATE FUNCTION",
			tableLock: AccessExclusive,
		}
	case *pg_query.Node_DefineStmt:
		return a.analyzeDefine(n.DefineStmt)
	case *pg_query.Node_CreateStatsStmt:
		return &operationInfo{
			operation: "CREATE STATISTICS",
			tableLock: AccessExclusive,
		}
	case *pg_query.Node_CreateEventTrigStmt:
		return &operationInfo{
			operation: "CREATE EVENT TRIGGER",
			tableLock: AccessExclusive,
		}
	case *pg_query.Node_CreateCastStmt:
		return &operationInfo{
			operation: "CREATE CAST",
			tableLock: AccessExclusive,
		}
	case *pg_query.Node_CreateFdwStmt:
		return &operationInfo{
			operation: "CREATE FOREIGN DATA WRAPPER",
			tableLock: AccessExclusive,
		}
	case *pg_query.Node_CreateForeignServerStmt:
		return &operationInfo{
			operation: "CREATE SERVER",
			tableLock: AccessExclusive,
		}
	case *pg_query.Node_CreateUserMappingStmt:
		return &operationInfo{
			operation: "CREATE USER MAPPING",
			tableLock: AccessExclusive,
		}
	case *pg_query.Node_DropUserMappingStmt:
		return &operationInfo{
			operation: "DROP USER MAPPING",
			tableLock: AccessExclusive,
		}
	case *pg_query.Node_DropSubscriptionStmt:
		return &operationInfo{
			operation: "DROP SUBSCRIPTION",
			tableLock: AccessExclusive,
		}
	case *pg_query.Node_AlterRoleSetStmt:
		return &operationInfo{
			operation: "ALTER ROLE",
			tableLock: AccessExclusive,
		}

	// If we have a RawStmt, recursively analyze its stmt
	case *pg_query.Node_RawStmt:
		if n.RawStmt != nil && n.RawStmt.Stmt != nil {
			return a.analyzeNode(n.RawStmt.Stmt, mode)
		}
		// Malformed RawStmt without inner statement
		return nil

	// Default for unknown nodes
	default:
		return nil
	}
}

// updateTransactionDepth updates the transaction depth based on the operation
func (a *analyzer) updateTransactionDepth(operation string) {
	switch operation {
	case "BEGIN", "START TRANSACTION":
		// In PostgreSQL, nested BEGIN doesn't actually create nested transactions
		// but we'll track it to maintain consistency
		if a.transactionDepth == 0 {
			a.transactionDepth = 1
		}
	case "COMMIT", "ROLLBACK":
		if a.transactionDepth > 0 {
			a.transactionDepth--
		}
	case "ROLLBACK TO SAVEPOINT":
		// ROLLBACK TO SAVEPOINT doesn't end the transaction
		// Transaction depth remains the same
	}
}

// formatTableLock formats a table lock for display
func formatTableLock(tableName string, lockType LockType) string {
	return tableName + ": " + string(lockType)
}
