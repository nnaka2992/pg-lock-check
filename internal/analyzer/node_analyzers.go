package analyzer

import (
	"fmt"
	"strings"

	"github.com/pganalyze/pg_query_go/v6"
)

// operationSeverity represents the severity of an operation for comparison
type operationSeverity int

const (
	severityUnknown operationSeverity = iota
	severitySelect
	severityInfo
	severityWarning
	severityCritical
	severityError
)

// getSeverityLevel returns the severity level for comparison
func getSeverityLevel(operation string) operationSeverity {
	// Check for specific operations that indicate severity
	if strings.Contains(operation, "SELECT") && !strings.Contains(operation, "FOR") {
		return severitySelect
	}
	if strings.Contains(operation, "INSERT") || strings.Contains(operation, "COPY FROM") {
		return severityInfo
	}
	if strings.Contains(operation, "with WHERE") || strings.Contains(operation, "FOR") {
		return severityWarning
	}
	if strings.Contains(operation, "without WHERE") || strings.Contains(operation, "TRUNCATE") {
		return severityCritical
	}
	return severityWarning // Default for UPDATE/DELETE/MERGE
}

// analyzeDMLRecursive recursively analyzes DML statements including CTEs
func (a *analyzer) analyzeDMLRecursive(node *pg_query.Node, mode TransactionMode) *operationInfo {
	if node == nil {
		return nil
	}

	var allOperations []*operationInfo
	var allTables []string
	tableMap := make(map[string]bool)

	// Extract and analyze CTEs first
	withClause := extractWithClause(node)
	if withClause != nil && len(withClause.Ctes) > 0 {
		for _, cte := range withClause.Ctes {
			if cteNode, ok := cte.Node.(*pg_query.Node_CommonTableExpr); ok && cteNode.CommonTableExpr != nil {
				if cteNode.CommonTableExpr.Ctequery != nil {
					cteOp := a.analyzeDMLRecursive(cteNode.CommonTableExpr.Ctequery, mode)
					if cteOp != nil {
						allOperations = append(allOperations, cteOp)
						// Extract tables from CTE operation
						cteTables := extractTablesFromNode(cteNode.CommonTableExpr.Ctequery)
						for _, table := range cteTables {
							if !tableMap[table] {
								tableMap[table] = true
								allTables = append(allTables, table)
							}
						}
					}
				}
			}
		}
	}

	// Analyze the main statement (non-recursively)
	mainOp := a.analyzeMainStatement(node, mode)
	if mainOp != nil {
		allOperations = append(allOperations, mainOp)
		// Extract tables from main statement
		mainTables := extractTablesFromNode(node)
		for _, table := range mainTables {
			if !tableMap[table] {
				tableMap[table] = true
				allTables = append(allTables, table)
			}
		}
	}

	// Find the most severe operation
	mostSevere := determineMostSevereOperation(allOperations)
	if mostSevere != nil {
		// Create a new operation info with all collected tables
		result := &operationInfo{
			operation:            mostSevere.operation,
			tableLock:            mostSevere.tableLock,
			message:              mostSevere.message,
			additionalTableLocks: make(map[string]LockType),
		}

		// For INSERT SELECT, we need to handle table locks differently
		if mostSevere.operation == "INSERT SELECT" {
			// The main statement tables get the operation's lock type
			mainTables := extractTablesFromNode(node)
			for _, table := range mainTables {
				result.additionalTableLocks[table] = mostSevere.tableLock
			}

			// Tables from SELECT get AccessShare
			if insertStmt, ok := node.Node.(*pg_query.Node_InsertStmt); ok && insertStmt.InsertStmt.SelectStmt != nil {
				selectTables := extractTablesFromNode(insertStmt.InsertStmt.SelectStmt)
				for _, table := range selectTables {
					if _, exists := result.additionalTableLocks[table]; !exists {
						result.additionalTableLocks[table] = AccessShare
					}
				}
			}
		} else {
			// For other operations, all tables get the same lock type
			for _, table := range allTables {
				result.additionalTableLocks[table] = mostSevere.tableLock
			}
		}

		return result
	}

	return nil
}

// extractWithClause extracts the WITH clause from various statement types
func extractWithClause(node *pg_query.Node) *pg_query.WithClause {
	if node == nil {
		return nil
	}

	switch n := node.Node.(type) {
	case *pg_query.Node_SelectStmt:
		return n.SelectStmt.WithClause
	case *pg_query.Node_InsertStmt:
		return n.InsertStmt.WithClause
	case *pg_query.Node_UpdateStmt:
		return n.UpdateStmt.WithClause
	case *pg_query.Node_DeleteStmt:
		return n.DeleteStmt.WithClause
	case *pg_query.Node_MergeStmt:
		return n.MergeStmt.WithClause
	}
	return nil
}

// analyzeMainStatement analyzes the main statement without recursion
func (a *analyzer) analyzeMainStatement(node *pg_query.Node, mode TransactionMode) *operationInfo {
	if node == nil {
		return nil
	}

	switch n := node.Node.(type) {
	case *pg_query.Node_UpdateStmt:
		return a.analyzeUpdateMain(n.UpdateStmt)
	case *pg_query.Node_DeleteStmt:
		return a.analyzeDeleteMain(n.DeleteStmt)
	case *pg_query.Node_InsertStmt:
		return a.analyzeInsertMain(n.InsertStmt)
	case *pg_query.Node_SelectStmt:
		return a.analyzeSelectMain(n.SelectStmt)
	case *pg_query.Node_MergeStmt:
		return a.analyzeMergeMain(n.MergeStmt)
	default:
		return nil
	}
}

// determineMostSevereOperation finds the most severe operation from a list
func determineMostSevereOperation(operations []*operationInfo) *operationInfo {
	if len(operations) == 0 {
		return nil
	}

	mostSevere := operations[0]
	mostSevereSeverity := getSeverityLevel(mostSevere.operation)

	for _, op := range operations[1:] {
		opSeverity := getSeverityLevel(op.operation)
		if opSeverity > mostSevereSeverity {
			mostSevere = op
			mostSevereSeverity = opSeverity
		}
	}

	return mostSevere
}

// analyzeUpdateMain analyzes UPDATE without recursion
func (a *analyzer) analyzeUpdateMain(stmt *pg_query.UpdateStmt) *operationInfo {
	hasWhere := stmt.WhereClause != nil
	operation := "UPDATE without WHERE"
	if hasWhere {
		operation = "UPDATE with WHERE"
	}

	return &operationInfo{
		operation: operation,
		tableLock: RowExclusive,
	}
}

// analyzeDeleteMain analyzes DELETE without recursion
func (a *analyzer) analyzeDeleteMain(stmt *pg_query.DeleteStmt) *operationInfo {
	hasWhere := stmt.WhereClause != nil
	operation := "DELETE without WHERE"
	if hasWhere {
		operation = "DELETE with WHERE"
	}

	return &operationInfo{
		operation: operation,
		tableLock: RowExclusive,
	}
}

// analyzeInsertMain analyzes INSERT without recursion
func (a *analyzer) analyzeInsertMain(stmt *pg_query.InsertStmt) *operationInfo {
	operation := "INSERT"

	// Check for INSERT SELECT first
	if stmt.SelectStmt != nil {
		// Only mark as INSERT SELECT if it's actually selecting from another table
		selectStmt := stmt.SelectStmt.GetSelectStmt()
		if selectStmt != nil && len(selectStmt.FromClause) > 0 {
			operation = "INSERT SELECT"
		}
	}

	// Check for ON CONFLICT (overrides INSERT SELECT)
	if stmt.OnConflictClause != nil {
		operation = "INSERT ON CONFLICT"
	}

	return &operationInfo{
		operation: operation,
		tableLock: RowExclusive,
	}
}

// analyzeSelectMain analyzes SELECT without recursion
func (a *analyzer) analyzeSelectMain(stmt *pg_query.SelectStmt) *operationInfo {
	// Check for locking clauses
	if len(stmt.LockingClause) > 0 {
		return a.analyzeLockingClause(stmt)
	}

	// Check for INTO clause (SELECT INTO)
	if stmt.IntoClause != nil {
		return &operationInfo{
			operation: "SELECT INTO",
			tableLock: AccessShare,
		}
	}

	// Regular SELECT
	return &operationInfo{
		operation: "SELECT",
		tableLock: AccessShare,
	}
}

// analyzeMergeMain analyzes MERGE without recursion
func (a *analyzer) analyzeMergeMain(stmt *pg_query.MergeStmt) *operationInfo {
	// Default to "without WHERE" - the main analyzer will check SQL text
	// for "WHEN MATCHED AND" pattern to determine if it has WHERE conditions
	return &operationInfo{
		operation: "MERGE without WHERE",
		tableLock: RowExclusive,
	}
}

// extractTablesFromNode extracts all table names from a node
func extractTablesFromNode(node *pg_query.Node) []string {
	if node == nil {
		return nil
	}

	tables := extractTables(node)
	return tables
}

// analyzeUpdate analyzes UPDATE statements
func (a *analyzer) analyzeUpdate(stmt *pg_query.UpdateStmt) *operationInfo {
	// Use recursive DML analyzer to handle CTEs
	return a.analyzeDMLRecursive(&pg_query.Node{
		Node: &pg_query.Node_UpdateStmt{UpdateStmt: stmt},
	}, InTransaction)
}

// analyzeDelete analyzes DELETE statements
func (a *analyzer) analyzeDelete(stmt *pg_query.DeleteStmt) *operationInfo {
	// Use recursive DML analyzer to handle CTEs
	return a.analyzeDMLRecursive(&pg_query.Node{
		Node: &pg_query.Node_DeleteStmt{DeleteStmt: stmt},
	}, InTransaction)
}

// analyzeInsert analyzes INSERT statements
func (a *analyzer) analyzeInsert(stmt *pg_query.InsertStmt) *operationInfo {
	// Use recursive DML analyzer to handle CTEs
	return a.analyzeDMLRecursive(&pg_query.Node{
		Node: &pg_query.Node_InsertStmt{InsertStmt: stmt},
	}, InTransaction)
}

// analyzeSelect analyzes SELECT statements
func (a *analyzer) analyzeSelect(stmt *pg_query.SelectStmt) *operationInfo {
	// Use recursive DML analyzer to handle CTEs
	opInfo := a.analyzeDMLRecursive(&pg_query.Node{
		Node: &pg_query.Node_SelectStmt{SelectStmt: stmt},
	}, InTransaction)

	// If recursive analysis found a data-modifying operation, return it
	if opInfo != nil && opInfo.operation != "SELECT" {
		return opInfo
	}

	// Check for locking clauses
	if len(stmt.LockingClause) > 0 {
		return a.analyzeLockingClause(stmt)
	}

	// Check for INTO clause (SELECT INTO)
	if stmt.IntoClause != nil {
		return &operationInfo{
			operation: "SELECT INTO",
			tableLock: AccessShare,
		}
	}

	// Regular SELECT
	return &operationInfo{
		operation: "SELECT",
		tableLock: AccessShare,
	}
}

// analyzeLockingClause analyzes SELECT with locking clauses
func (a *analyzer) analyzeLockingClause(stmt *pg_query.SelectStmt) *operationInfo {
	if len(stmt.LockingClause) == 0 {
		return &operationInfo{
			operation: "SELECT",
			tableLock: AccessShare,
		}
	}

	lockingClause := stmt.LockingClause[0]
	hasWhere := stmt.WhereClause != nil
	whereQualifier := ""

	lc := lockingClause.GetLockingClause()
	if lc == nil {
		return &operationInfo{
			operation: "SELECT",
			tableLock: AccessShare,
		}
	}

	// For locking clauses, we need to determine the qualifier
	switch lc.Strength {
	case pg_query.LockClauseStrength_LCS_FORUPDATE:
		if !hasWhere {
			whereQualifier = " without WHERE"
		} else {
			whereQualifier = " with WHERE"
		}
		return &operationInfo{
			operation: "SELECT FOR UPDATE" + whereQualifier,
			tableLock: RowShare,
		}
	case pg_query.LockClauseStrength_LCS_FORNOKEYUPDATE:
		if !hasWhere {
			whereQualifier = " without WHERE"
		} else {
			whereQualifier = " with WHERE"
		}
		return &operationInfo{
			operation: "SELECT FOR NO KEY UPDATE" + whereQualifier,
			tableLock: RowShare,
		}
	case pg_query.LockClauseStrength_LCS_FORSHARE:
		if !hasWhere {
			whereQualifier = " without WHERE"
		} else {
			whereQualifier = " with WHERE"
		}
		return &operationInfo{
			operation: "SELECT FOR SHARE" + whereQualifier,
			tableLock: RowShare,
		}
	case pg_query.LockClauseStrength_LCS_FORKEYSHARE:
		// SELECT FOR KEY SHARE doesn't include WHERE qualifier in tests
		return &operationInfo{
			operation: "SELECT FOR KEY SHARE",
			tableLock: RowShare,
		}
	default:
		return &operationInfo{
			operation: "SELECT",
			tableLock: AccessShare,
		}
	}
}

// analyzeAlterTable analyzes ALTER TABLE statements
func (a *analyzer) analyzeAlterTable(stmt *pg_query.AlterTableStmt) *operationInfo {
	// Check if this is actually an ALTER INDEX
	if stmt.Objtype == pg_query.ObjectType_OBJECT_INDEX {
		// For ALTER INDEX, we'll analyze the command but return ALTER INDEX operation
		for _, cmd := range stmt.Cmds {
			alterCmd := cmd.GetAlterTableCmd()
			if alterCmd != nil && alterCmd.Subtype == pg_query.AlterTableType_AT_SetTableSpace {
				return &operationInfo{
					operation: "ALTER INDEX",
					tableLock: AccessExclusive,
				}
			}
		}
		return &operationInfo{
			operation: "ALTER INDEX",
			tableLock: AccessExclusive,
		}
	}

	// Analyze each command in the ALTER TABLE
	for _, cmd := range stmt.Cmds {
		alterCmd := cmd.GetAlterTableCmd()
		if alterCmd != nil {
			op := a.analyzeAlterTableCmd(alterCmd)
			if op != nil {
				return op
			}
		}
	}

	return &operationInfo{
		operation: "ALTER TABLE",
		tableLock: AccessExclusive,
	}
}

// analyzeAlterTableCmd analyzes individual ALTER TABLE commands
func (a *analyzer) analyzeAlterTableCmd(cmd *pg_query.AlterTableCmd) *operationInfo {
	switch cmd.Subtype {
	case pg_query.AlterTableType_AT_AddColumn:
		return a.analyzeAddColumn(cmd)
	case pg_query.AlterTableType_AT_DropColumn:
		return &operationInfo{
			operation: "ALTER TABLE DROP COLUMN",
			tableLock: AccessExclusive,
		}
	case pg_query.AlterTableType_AT_AlterColumnType:
		return &operationInfo{
			operation: "ALTER TABLE ALTER COLUMN TYPE",
			tableLock: AccessExclusive,
		}
	case pg_query.AlterTableType_AT_SetTableSpace:
		return &operationInfo{
			operation: "ALTER TABLE SET TABLESPACE",
			tableLock: AccessExclusive,
		}
	case pg_query.AlterTableType_AT_SetLogged:
		return &operationInfo{
			operation: "ALTER TABLE SET LOGGED",
			tableLock: AccessExclusive,
		}
	case pg_query.AlterTableType_AT_SetUnLogged:
		return &operationInfo{
			operation: "ALTER TABLE SET UNLOGGED",
			tableLock: AccessExclusive,
		}
	case pg_query.AlterTableType_AT_AddConstraint:
		return a.analyzeAddConstraint(cmd)
	case pg_query.AlterTableType_AT_DropConstraint:
		return &operationInfo{
			operation: "ALTER TABLE DROP CONSTRAINT",
			tableLock: AccessExclusive,
		}
	case pg_query.AlterTableType_AT_ValidateConstraint:
		return &operationInfo{
			operation: "ALTER TABLE VALIDATE CONSTRAINT",
			tableLock: ShareUpdateExclusive,
		}
	case pg_query.AlterTableType_AT_AddIndex:
		return &operationInfo{
			operation: "ALTER TABLE ADD PRIMARY KEY",
			tableLock: AccessExclusive,
		}
	case pg_query.AlterTableType_AT_AttachPartition:
		opInfo := &operationInfo{
			operation:            "ALTER TABLE ATTACH PARTITION",
			tableLock:            ShareUpdateExclusive,
			additionalTableLocks: make(map[string]LockType),
		}
		// The partition table is in cmd.Def as a PartitionCmd
		if cmd.Def != nil {
			if pc := cmd.Def.GetPartitionCmd(); pc != nil && pc.Name != nil {
				partitionName := getQualifiedTableName(pc.Name)
				if partitionName != "" {
					opInfo.additionalTableLocks[partitionName] = ShareUpdateExclusive
				}
			}
		}
		return opInfo
	case pg_query.AlterTableType_AT_DetachPartition:
		// Always return base operation - we'll check for CONCURRENTLY in the main analyzer
		opInfo := &operationInfo{
			operation:            "ALTER TABLE DETACH PARTITION",
			tableLock:            ShareUpdateExclusive,
			additionalTableLocks: make(map[string]LockType),
		}
		// The partition table is in cmd.Def as a PartitionCmd
		if cmd.Def != nil {
			if pc := cmd.Def.GetPartitionCmd(); pc != nil && pc.Name != nil {
				partitionName := getQualifiedTableName(pc.Name)
				if partitionName != "" {
					opInfo.additionalTableLocks[partitionName] = ShareUpdateExclusive
				}
			}
		}
		return opInfo
	case pg_query.AlterTableType_AT_SetRelOptions:
		return &operationInfo{
			operation: "ALTER TABLE SET",
			tableLock: ShareUpdateExclusive,
		}
	case pg_query.AlterTableType_AT_ResetRelOptions:
		return &operationInfo{
			operation: "ALTER TABLE RESET",
			tableLock: ShareUpdateExclusive,
		}
	case pg_query.AlterTableType_AT_ClusterOn:
		return &operationInfo{
			operation: "ALTER TABLE CLUSTER ON",
			tableLock: ShareUpdateExclusive,
		}
	case pg_query.AlterTableType_AT_DropCluster:
		return &operationInfo{
			operation: "ALTER TABLE SET WITHOUT CLUSTER",
			tableLock: ShareUpdateExclusive,
		}
	case pg_query.AlterTableType_AT_DropNotNull:
		return &operationInfo{
			operation: "ALTER TABLE DROP NOT NULL",
			tableLock: AccessExclusive,
		}
	case pg_query.AlterTableType_AT_SetNotNull:
		return &operationInfo{
			operation: "ALTER TABLE SET NOT NULL",
			tableLock: AccessExclusive,
		}
	case pg_query.AlterTableType_AT_EnableTrig:
		return &operationInfo{
			operation: "ALTER TABLE ENABLE TRIGGER",
			tableLock: ShareRowExclusive,
		}
	case pg_query.AlterTableType_AT_DisableTrig:
		return &operationInfo{
			operation: "ALTER TABLE DISABLE TRIGGER",
			tableLock: ShareRowExclusive,
		}
	case pg_query.AlterTableType_AT_EnableRule:
		return &operationInfo{
			operation: "ALTER TABLE ENABLE RULE",
			tableLock: ShareRowExclusive,
		}
	case pg_query.AlterTableType_AT_DisableRule:
		return &operationInfo{
			operation: "ALTER TABLE DISABLE RULE",
			tableLock: ShareRowExclusive,
		}
	case pg_query.AlterTableType_AT_EnableRowSecurity:
		return &operationInfo{
			operation: "ALTER TABLE ENABLE ROW LEVEL SECURITY",
			tableLock: AccessExclusive,
		}
	case pg_query.AlterTableType_AT_DisableRowSecurity:
		return &operationInfo{
			operation: "ALTER TABLE DISABLE ROW LEVEL SECURITY",
			tableLock: AccessExclusive,
		}
	case pg_query.AlterTableType_AT_ForceRowSecurity:
		return &operationInfo{
			operation: "ALTER TABLE FORCE ROW LEVEL SECURITY",
			tableLock: AccessExclusive,
		}
	case pg_query.AlterTableType_AT_NoForceRowSecurity:
		return &operationInfo{
			operation: "ALTER TABLE NO FORCE ROW LEVEL SECURITY",
			tableLock: AccessExclusive,
		}
	case pg_query.AlterTableType_AT_AddInherit:
		opInfo := &operationInfo{
			operation:            "ALTER TABLE INHERIT",
			tableLock:            AccessExclusive,
			additionalTableLocks: make(map[string]LockType),
		}
		// The parent table is in cmd.Def as a RangeVar
		if cmd.Def != nil {
			if rv := cmd.Def.GetRangeVar(); rv != nil {
				parentTableName := getQualifiedTableName(rv)
				if parentTableName != "" {
					opInfo.additionalTableLocks[parentTableName] = ShareUpdateExclusive
				}
			}
		}
		return opInfo
	case pg_query.AlterTableType_AT_DropInherit:
		opInfo := &operationInfo{
			operation:            "ALTER TABLE NO INHERIT",
			tableLock:            AccessExclusive,
			additionalTableLocks: make(map[string]LockType),
		}
		// The parent table is in cmd.Def as a RangeVar
		if cmd.Def != nil {
			if rv := cmd.Def.GetRangeVar(); rv != nil {
				parentTableName := getQualifiedTableName(rv)
				if parentTableName != "" {
					opInfo.additionalTableLocks[parentTableName] = ShareUpdateExclusive
				}
			}
		}
		return opInfo
	case pg_query.AlterTableType_AT_AddOf:
		return &operationInfo{
			operation: "ALTER TABLE OF",
			tableLock: AccessExclusive,
		}
	case pg_query.AlterTableType_AT_DropOf:
		return &operationInfo{
			operation: "ALTER TABLE NOT OF",
			tableLock: AccessExclusive,
		}
	case pg_query.AlterTableType_AT_ReplicaIdentity:
		return &operationInfo{
			operation: "ALTER TABLE REPLICA IDENTITY",
			tableLock: AccessExclusive,
		}
	case pg_query.AlterTableType_AT_ChangeOwner:
		return &operationInfo{
			operation: "ALTER TABLE OWNER TO",
			tableLock: AccessExclusive,
		}
	case pg_query.AlterTableType_AT_SetAccessMethod:
		return &operationInfo{
			operation: "ALTER TABLE SET ACCESS METHOD",
			tableLock: AccessExclusive,
		}
	case pg_query.AlterTableType_AT_ColumnDefault:
		// Check if it's SET DEFAULT (has a def) or DROP DEFAULT (no def)
		if cmd.Def != nil {
			return &operationInfo{
				operation: "ALTER TABLE SET DEFAULT",
				tableLock: AccessExclusive,
			}
		}
		return &operationInfo{
			operation: "ALTER TABLE DROP DEFAULT",
			tableLock: AccessExclusive,
		}
	case pg_query.AlterTableType_AT_SetStatistics:
		return &operationInfo{
			operation: "ALTER TABLE ALTER COLUMN SET STATISTICS",
			tableLock: ShareUpdateExclusive,
		}
	case pg_query.AlterTableType_AT_SetStorage:
		return &operationInfo{
			operation: "ALTER TABLE ALTER COLUMN SET STORAGE",
			tableLock: AccessExclusive,
		}
	case pg_query.AlterTableType_AT_AddIdentity:
		return &operationInfo{
			operation: "ALTER TABLE ALTER COLUMN ADD IDENTITY",
			tableLock: AccessExclusive,
		}
	case pg_query.AlterTableType_AT_DropIdentity:
		return &operationInfo{
			operation: "ALTER TABLE ALTER COLUMN DROP IDENTITY",
			tableLock: AccessExclusive,
		}
	}

	return nil
}

// analyzeAddColumn analyzes ADD COLUMN commands
func (a *analyzer) analyzeAddColumn(cmd *pg_query.AlterTableCmd) *operationInfo {
	if cmd.Def == nil {
		return &operationInfo{
			operation: "ALTER TABLE ADD COLUMN without DEFAULT",
			tableLock: AccessExclusive,
		}
	}

	colDef := cmd.Def.GetColumnDef()
	if colDef == nil {
		return &operationInfo{
			operation: "ALTER TABLE ADD COLUMN without DEFAULT",
			tableLock: AccessExclusive,
		}
	}

	// Check for GENERATED ALWAYS AS
	if colDef.Identity != "" || colDef.Generated != "" {
		return &operationInfo{
			operation: "ALTER TABLE ADD COLUMN GENERATED ALWAYS AS",
			tableLock: AccessExclusive,
		}
	}

	// Check for DEFAULT clause
	for _, constraint := range colDef.Constraints {
		if constr := constraint.GetConstraint(); constr != nil && constr.Contype == pg_query.ConstrType_CONSTR_DEFAULT {
			if constr.RawExpr != nil {
				// Check if default is volatile
				if isVolatileDefault(constr.RawExpr) {
					return &operationInfo{
						operation: "ALTER TABLE ADD COLUMN with volatile DEFAULT",
						tableLock: AccessExclusive,
					}
				}
				return &operationInfo{
					operation: "ALTER TABLE ADD COLUMN with constant DEFAULT",
					tableLock: AccessExclusive,
				}
			}
		}
	}

	return &operationInfo{
		operation: "ALTER TABLE ADD COLUMN without DEFAULT",
		tableLock: AccessExclusive,
	}
}

// isVolatileDefault checks if a default expression is volatile
func isVolatileDefault(expr *pg_query.Node) bool {
	if expr == nil {
		return false
	}

	// Check for function calls
	if funcCall := expr.GetFuncCall(); funcCall != nil {
		funcName := ""
		if len(funcCall.Funcname) > 0 {
			for _, name := range funcCall.Funcname {
				if str := name.GetString_(); str != nil {
					funcName += str.Sval
				}
			}
		}

		// List of known volatile functions
		volatileFuncs := []string{"random", "now", "current_timestamp", "current_date",
			"current_time", "timeofday", "clock_timestamp", "statement_timestamp",
			"transaction_timestamp", "uuid_generate_v4", "gen_random_uuid"}

		for _, vf := range volatileFuncs {
			if strings.Contains(strings.ToLower(funcName), vf) {
				return true
			}
		}
	}

	return false
}

// analyzeAddConstraint analyzes ADD CONSTRAINT commands
func (a *analyzer) analyzeAddConstraint(cmd *pg_query.AlterTableCmd) *operationInfo {
	if cmd.Def == nil {
		return &operationInfo{
			operation: "ALTER TABLE ADD CONSTRAINT",
			tableLock: AccessExclusive,
		}
	}

	constraint := cmd.Def.GetConstraint()
	if constraint == nil {
		return &operationInfo{
			operation: "ALTER TABLE ADD CONSTRAINT",
			tableLock: AccessExclusive,
		}
	}

	// Check for NOT VALID
	if constraint.SkipValidation {
		return &operationInfo{
			operation: "ALTER TABLE ADD CONSTRAINT NOT VALID",
			tableLock: ShareRowExclusive,
		}
	}

	switch constraint.Contype {
	case pg_query.ConstrType_CONSTR_PRIMARY:
		return &operationInfo{
			operation: "ALTER TABLE ADD PRIMARY KEY",
			tableLock: AccessExclusive,
		}
	case pg_query.ConstrType_CONSTR_UNIQUE:
		return &operationInfo{
			operation: "ALTER TABLE ADD CONSTRAINT UNIQUE",
			tableLock: AccessExclusive,
		}
	case pg_query.ConstrType_CONSTR_EXCLUSION:
		return &operationInfo{
			operation: "ALTER TABLE ADD CONSTRAINT EXCLUDE",
			tableLock: AccessExclusive,
		}
	case pg_query.ConstrType_CONSTR_FOREIGN:
		opInfo := &operationInfo{
			operation:            "ALTER TABLE ADD FOREIGN KEY",
			tableLock:            ShareRowExclusive,
			additionalTableLocks: make(map[string]LockType),
		}
		// Extract referenced table from the constraint
		if constraint.Pktable != nil {
			refTableName := getQualifiedTableName(constraint.Pktable)
			if refTableName != "" {
				opInfo.additionalTableLocks[refTableName] = RowShare
			}
		}
		return opInfo
	case pg_query.ConstrType_CONSTR_CHECK:
		return &operationInfo{
			operation: "ALTER TABLE ADD CONSTRAINT CHECK",
			tableLock: AccessExclusive,
		}
	}

	return &operationInfo{
		operation: "ALTER TABLE ADD CONSTRAINT",
		tableLock: AccessExclusive,
	}
}

// analyzeCreate analyzes CREATE TABLE statements
func (a *analyzer) analyzeCreate(stmt *pg_query.CreateStmt) *operationInfo {
	// Check for TEMPORARY
	if stmt.Relation != nil && stmt.Relation.Relpersistence == "t" {
		return &operationInfo{
			operation: "CREATE TEMPORARY TABLE",
			tableLock: AccessExclusive,
		}
	}

	return &operationInfo{
		operation: "CREATE TABLE",
		tableLock: AccessExclusive,
	}
}

// analyzeDrop analyzes DROP statements
func (a *analyzer) analyzeDrop(stmt *pg_query.DropStmt) *operationInfo {
	cascade := ""
	if stmt.Behavior == pg_query.DropBehavior_DROP_CASCADE {
		cascade = " CASCADE"
	}

	switch stmt.RemoveType {
	case pg_query.ObjectType_OBJECT_TABLE:
		return &operationInfo{
			operation: "DROP TABLE",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_INDEX:
		// Check for CONCURRENTLY
		if stmt.Concurrent {
			return &operationInfo{
				operation: "DROP INDEX CONCURRENTLY",
				tableLock: ShareUpdateExclusive,
			}
		}
		return &operationInfo{
			operation: "DROP INDEX",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_SCHEMA:
		return &operationInfo{
			operation: "DROP SCHEMA" + cascade,
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_VIEW:
		return &operationInfo{
			operation: "DROP VIEW",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_MATVIEW:
		return &operationInfo{
			operation: "DROP MATERIALIZED VIEW",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_SEQUENCE:
		return &operationInfo{
			operation: "DROP SEQUENCE",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_TYPE:
		return &operationInfo{
			operation: "DROP TYPE",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_DOMAIN:
		return &operationInfo{
			operation: "DROP DOMAIN",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_EXTENSION:
		return &operationInfo{
			operation: "DROP EXTENSION",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_DATABASE:
		return &operationInfo{
			operation: "DROP DATABASE",
			tableLock: Exclusive,
		}
	case pg_query.ObjectType_OBJECT_TABLESPACE:
		return &operationInfo{
			operation: "DROP TABLESPACE",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_TRIGGER:
		return &operationInfo{
			operation: "DROP TRIGGER",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_RULE:
		return &operationInfo{
			operation: "DROP RULE",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_POLICY:
		return &operationInfo{
			operation: "DROP POLICY",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_FUNCTION:
		return &operationInfo{
			operation: "DROP FUNCTION",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_PROCEDURE:
		return &operationInfo{
			operation: "DROP PROCEDURE",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_AGGREGATE:
		return &operationInfo{
			operation: "DROP AGGREGATE",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_OPERATOR:
		return &operationInfo{
			operation: "DROP OPERATOR",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_CAST:
		return &operationInfo{
			operation: "DROP CAST",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_COLLATION:
		return &operationInfo{
			operation: "DROP COLLATION",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_SUBSCRIPTION:
		return &operationInfo{
			operation: "DROP SUBSCRIPTION",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_PUBLICATION:
		return &operationInfo{
			operation: "DROP PUBLICATION",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_ROLE:
		return &operationInfo{
			operation: "DROP ROLE",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_STATISTIC_EXT:
		return &operationInfo{
			operation: "DROP STATISTICS",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_EVENT_TRIGGER:
		return &operationInfo{
			operation: "DROP EVENT TRIGGER",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_FDW:
		return &operationInfo{
			operation: "DROP FOREIGN DATA WRAPPER",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_FOREIGN_SERVER:
		return &operationInfo{
			operation: "DROP SERVER",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_USER_MAPPING:
		return &operationInfo{
			operation: "DROP USER MAPPING",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_TSPARSER:
		return &operationInfo{
			operation: "DROP TEXT SEARCH PARSER",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_TSDICTIONARY:
		return &operationInfo{
			operation: "DROP TEXT SEARCH DICTIONARY",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_TSTEMPLATE:
		return &operationInfo{
			operation: "DROP TEXT SEARCH TEMPLATE",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_TSCONFIGURATION:
		return &operationInfo{
			operation: "DROP TEXT SEARCH CONFIGURATION",
			tableLock: AccessExclusive,
		}
	}

	return &operationInfo{
		operation: "DROP",
		tableLock: AccessExclusive,
	}
}

// analyzeTruncate analyzes TRUNCATE statements
func (a *analyzer) analyzeTruncate(stmt *pg_query.TruncateStmt) *operationInfo {
	return &operationInfo{
		operation: "TRUNCATE",
		tableLock: AccessExclusive,
	}
}

// analyzeVacuum analyzes VACUUM statements
func (a *analyzer) analyzeVacuum(stmt *pg_query.VacuumStmt) *operationInfo {
	options := []string{}

	// Check for FULL option
	for _, opt := range stmt.Options {
		if defElem := opt.GetDefElem(); defElem != nil {
			if defElem.Defname == "full" {
				options = append(options, "FULL")
			}
			if defElem.Defname == "freeze" {
				options = append(options, "FREEZE")
			}
			if defElem.Defname == "analyze" {
				options = append(options, "ANALYZE")
			}
		}
	}

	operation := "VACUUM"
	if len(options) > 0 {
		operation = "VACUUM " + strings.Join(options, " ")
	}

	lockType := ShareUpdateExclusive
	if contains(operation, "FULL") {
		lockType = AccessExclusive
	}

	return &operationInfo{
		operation: operation,
		tableLock: lockType,
	}
}

// analyzeIndex analyzes INDEX statements
func (a *analyzer) analyzeIndex(stmt *pg_query.IndexStmt) *operationInfo {
	operation := "CREATE INDEX"
	if stmt.Unique {
		operation = "CREATE UNIQUE INDEX"
	}

	if stmt.Concurrent {
		operation = fmt.Sprintf("%s CONCURRENTLY", operation)
	}

	lockType := Share
	if stmt.Concurrent {
		lockType = ShareUpdateExclusive
	}

	return &operationInfo{
		operation: operation,
		tableLock: lockType,
	}
}

// analyzeLock analyzes LOCK statements
func (a *analyzer) analyzeLock(stmt *pg_query.LockStmt) *operationInfo {
	var lockType LockType
	var operation string

	switch stmt.Mode {
	case 1: // ACCESS SHARE
		lockType = AccessShare
		operation = "LOCK TABLE ACCESS SHARE"
	case 2: // ROW SHARE
		lockType = RowShare
		operation = "LOCK TABLE ROW SHARE"
	case 3: // ROW EXCLUSIVE
		lockType = RowExclusive
		operation = "LOCK TABLE ROW EXCLUSIVE"
	case 4: // SHARE UPDATE EXCLUSIVE
		lockType = ShareUpdateExclusive
		operation = "LOCK TABLE SHARE UPDATE EXCLUSIVE"
	case 5: // SHARE
		lockType = Share
		operation = "LOCK TABLE SHARE"
	case 6: // SHARE ROW EXCLUSIVE
		lockType = ShareRowExclusive
		operation = "LOCK TABLE SHARE ROW EXCLUSIVE"
	case 7: // EXCLUSIVE
		lockType = Exclusive
		operation = "LOCK TABLE EXCLUSIVE"
	case 8: // ACCESS EXCLUSIVE
		lockType = AccessExclusive
		operation = "LOCK TABLE ACCESS EXCLUSIVE"
	default:
		lockType = AccessShare
		operation = "LOCK TABLE"
	}

	return &operationInfo{
		operation: operation,
		tableLock: lockType,
	}
}

// analyzeMerge analyzes MERGE statements
func (a *analyzer) analyzeMerge(stmt *pg_query.MergeStmt) *operationInfo {
	// Use recursive DML analyzer to handle CTEs
	return a.analyzeDMLRecursive(&pg_query.Node{
		Node: &pg_query.Node_MergeStmt{MergeStmt: stmt},
	}, InTransaction)
}

// analyzeCopy analyzes COPY statements
func (a *analyzer) analyzeCopy(stmt *pg_query.CopyStmt) *operationInfo {
	if stmt.IsFrom {
		return &operationInfo{
			operation: "COPY FROM",
			tableLock: RowExclusive,
		}
	}
	return &operationInfo{
		operation: "COPY TO",
		tableLock: AccessShare,
	}
}

// analyzeAnalyze analyzes ANALYZE statements
func (a *analyzer) analyzeAnalyze(stmt *pg_query.VacuumStmt) *operationInfo {
	return &operationInfo{
		operation: "ANALYZE",
		tableLock: ShareUpdateExclusive,
	}
}

// analyzeCluster analyzes CLUSTER statements
func (a *analyzer) analyzeCluster(stmt *pg_query.ClusterStmt) *operationInfo {
	return &operationInfo{
		operation: "CLUSTER",
		tableLock: AccessExclusive,
	}
}

// analyzeReindex analyzes REINDEX statements
func (a *analyzer) analyzeReindex(stmt *pg_query.ReindexStmt) *operationInfo {
	switch stmt.Kind {
	case pg_query.ReindexObjectType_REINDEX_OBJECT_DATABASE:
		return &operationInfo{
			operation: "REINDEX DATABASE",
			tableLock: AccessExclusive,
		}
	case pg_query.ReindexObjectType_REINDEX_OBJECT_SCHEMA:
		return &operationInfo{
			operation: "REINDEX SCHEMA",
			tableLock: AccessExclusive,
		}
	case pg_query.ReindexObjectType_REINDEX_OBJECT_SYSTEM:
		return &operationInfo{
			operation: "REINDEX SYSTEM",
			tableLock: AccessExclusive,
		}
	case pg_query.ReindexObjectType_REINDEX_OBJECT_TABLE:
		if stmt.Params != nil {
			for _, defElem := range stmt.Params {
				if de := defElem.GetDefElem(); de != nil && de.Defname == "concurrently" {
					return &operationInfo{
						operation: "REINDEX CONCURRENTLY",
						tableLock: ShareUpdateExclusive,
					}
				}
			}
		}
		return &operationInfo{
			operation: "REINDEX TABLE",
			tableLock: AccessExclusive,
		}
	case pg_query.ReindexObjectType_REINDEX_OBJECT_INDEX:
		if stmt.Params != nil {
			for _, defElem := range stmt.Params {
				if de := defElem.GetDefElem(); de != nil && de.Defname == "concurrently" {
					return &operationInfo{
						operation: "REINDEX CONCURRENTLY",
						tableLock: ShareUpdateExclusive,
					}
				}
			}
		}
		return &operationInfo{
			operation: "REINDEX",
			tableLock: AccessExclusive,
		}
	}

	return &operationInfo{
		operation: "REINDEX",
		tableLock: AccessExclusive,
	}
}

// analyzeCreateView analyzes CREATE VIEW statements
func (a *analyzer) analyzeCreateView(stmt *pg_query.ViewStmt) *operationInfo {
	if stmt.Replace {
		return &operationInfo{
			operation: "CREATE OR REPLACE VIEW",
			tableLock: AccessExclusive,
		}
	}
	return &operationInfo{
		operation: "CREATE VIEW",
		tableLock: AccessShare,
	}
}

// analyzeCreateMatView analyzes CREATE MATERIALIZED VIEW statements
func (a *analyzer) analyzeCreateMatView(stmt *pg_query.CreateTableAsStmt) *operationInfo {
	if stmt.IsSelectInto {
		return &operationInfo{
			operation: "SELECT INTO",
			tableLock: AccessExclusive,
		}
	}

	// Check if it's a materialized view
	if stmt.Objtype == pg_query.ObjectType_OBJECT_MATVIEW {
		return &operationInfo{
			operation: "CREATE MATERIALIZED VIEW",
			tableLock: AccessShare,
		}
	}

	// Default to CREATE TABLE AS
	return &operationInfo{
		operation: "CREATE TABLE AS",
		tableLock: AccessShare,
	}
}

// analyzeRefreshMatView analyzes REFRESH MATERIALIZED VIEW statements
func (a *analyzer) analyzeRefreshMatView(stmt *pg_query.RefreshMatViewStmt) *operationInfo {
	if stmt.Concurrent {
		return &operationInfo{
			operation: "REFRESH MATERIALIZED VIEW CONCURRENTLY",
			tableLock: ShareUpdateExclusive,
		}
	}
	return &operationInfo{
		operation: "REFRESH MATERIALIZED VIEW",
		tableLock: AccessExclusive,
	}
}

// analyzeCreateSchema analyzes CREATE SCHEMA statements
func (a *analyzer) analyzeCreateSchema(stmt *pg_query.CreateSchemaStmt) *operationInfo {
	return &operationInfo{
		operation: "CREATE SCHEMA",
		tableLock: AccessExclusive,
	}
}

// analyzeCreateSequence analyzes CREATE SEQUENCE statements
func (a *analyzer) analyzeCreateSequence(stmt *pg_query.CreateSeqStmt) *operationInfo {
	return &operationInfo{
		operation: "CREATE SEQUENCE",
		tableLock: AccessExclusive,
	}
}

// analyzeAlterSequence analyzes ALTER SEQUENCE statements
func (a *analyzer) analyzeAlterSequence(stmt *pg_query.AlterSeqStmt) *operationInfo {
	return &operationInfo{
		operation: "ALTER SEQUENCE",
		tableLock: AccessExclusive,
	}
}

// analyzeCreateDomain analyzes CREATE DOMAIN statements
func (a *analyzer) analyzeCreateDomain(stmt *pg_query.CreateDomainStmt) *operationInfo {
	return &operationInfo{
		operation: "CREATE DOMAIN",
		tableLock: AccessExclusive,
	}
}

// analyzeCreateExtension analyzes CREATE EXTENSION statements
func (a *analyzer) analyzeCreateExtension(stmt *pg_query.CreateExtensionStmt) *operationInfo {
	return &operationInfo{
		operation: "CREATE EXTENSION",
		tableLock: AccessExclusive,
	}
}

// analyzeAlterExtension analyzes ALTER EXTENSION statements
func (a *analyzer) analyzeAlterExtension(stmt *pg_query.AlterExtensionStmt) *operationInfo {
	return &operationInfo{
		operation: "ALTER EXTENSION ADD/DROP",
		tableLock: AccessExclusive,
	}
}

// analyzeAlterType analyzes ALTER TYPE statements
func (a *analyzer) analyzeAlterType(stmt *pg_query.AlterEnumStmt) *operationInfo {
	return &operationInfo{
		operation: "ALTER TYPE ADD VALUE",
		tableLock: AccessExclusive,
	}
}

// analyzeCreateDatabase analyzes CREATE DATABASE statements
func (a *analyzer) analyzeCreateDatabase(stmt *pg_query.CreatedbStmt) *operationInfo {
	return &operationInfo{
		operation: "CREATE DATABASE",
		tableLock: AccessExclusive,
	}
}

// analyzeAlterDatabase analyzes ALTER DATABASE statements
func (a *analyzer) analyzeAlterDatabase(stmt *pg_query.AlterDatabaseStmt) *operationInfo {
	return &operationInfo{
		operation: "ALTER DATABASE",
		tableLock: Exclusive,
	}
}

// analyzeCreateTablespace analyzes CREATE TABLESPACE statements
func (a *analyzer) analyzeCreateTablespace(stmt *pg_query.CreateTableSpaceStmt) *operationInfo {
	return &operationInfo{
		operation: "CREATE TABLESPACE",
		tableLock: AccessExclusive,
	}
}

// analyzeAlterTablespace analyzes ALTER TABLESPACE statements
func (a *analyzer) analyzeAlterTablespace(stmt *pg_query.AlterTableSpaceOptionsStmt) *operationInfo {
	return &operationInfo{
		operation: "ALTER TABLESPACE",
		tableLock: AccessExclusive,
	}
}

// analyzeGrant analyzes GRANT/REVOKE statements
func (a *analyzer) analyzeGrant(stmt *pg_query.GrantStmt) *operationInfo {
	objType := ""
	switch stmt.Objtype {
	case pg_query.ObjectType_OBJECT_TABLE:
		objType = "" // For tables, just use GRANT/REVOKE without suffix
	case pg_query.ObjectType_OBJECT_SEQUENCE:
		objType = " ON SEQUENCE"
	case pg_query.ObjectType_OBJECT_DATABASE:
		objType = " ON DATABASE"
	case pg_query.ObjectType_OBJECT_SCHEMA:
		objType = " ON SCHEMA"
	case pg_query.ObjectType_OBJECT_FUNCTION:
		objType = " ON FUNCTION"
	case pg_query.ObjectType_OBJECT_PROCEDURE:
		objType = " ON PROCEDURE"
	case pg_query.ObjectType_OBJECT_TYPE:
		objType = " ON TYPE"
	case pg_query.ObjectType_OBJECT_LANGUAGE:
		objType = " ON LANGUAGE"
	case pg_query.ObjectType_OBJECT_FOREIGN_SERVER:
		objType = " ON SERVER"
	case pg_query.ObjectType_OBJECT_FDW:
		objType = " ON FOREIGN DATA WRAPPER"
	}

	if stmt.IsGrant {
		return &operationInfo{
			operation: "GRANT" + objType,
			tableLock: AccessShare,
		}
	}
	return &operationInfo{
		operation: "REVOKE" + objType,
		tableLock: AccessShare,
	}
}

// analyzeCreateRole analyzes CREATE ROLE statements
func (a *analyzer) analyzeCreateRole(stmt *pg_query.CreateRoleStmt) *operationInfo {
	// In v6, check the StmtType field
	if stmt.StmtType == pg_query.RoleStmtType_ROLESTMT_USER {
		return &operationInfo{
			operation: "CREATE USER",
			tableLock: AccessExclusive,
		}
	}
	return &operationInfo{
		operation: "CREATE ROLE",
		tableLock: AccessExclusive,
	}
}

// analyzeAlterRole analyzes ALTER ROLE statements
func (a *analyzer) analyzeAlterRole(stmt *pg_query.AlterRoleStmt) *operationInfo {
	return &operationInfo{
		operation: "ALTER ROLE",
		tableLock: AccessExclusive,
	}
}

// analyzeDropRole analyzes DROP ROLE statements
func (a *analyzer) analyzeDropRole(stmt *pg_query.DropRoleStmt) *operationInfo {
	return &operationInfo{
		operation: "DROP ROLE",
		tableLock: AccessExclusive,
	}
}

// analyzeReassignOwned analyzes REASSIGN OWNED statements
func (a *analyzer) analyzeReassignOwned(stmt *pg_query.ReassignOwnedStmt) *operationInfo {
	return &operationInfo{
		operation: "REASSIGN OWNED",
		tableLock: AccessExclusive,
	}
}

// analyzeAlterDefaultPrivileges analyzes ALTER DEFAULT PRIVILEGES statements
func (a *analyzer) analyzeAlterDefaultPrivileges(stmt *pg_query.AlterDefaultPrivilegesStmt) *operationInfo {
	return &operationInfo{
		operation: "ALTER DEFAULT PRIVILEGES",
		tableLock: AccessExclusive,
	}
}

// analyzeCreateTrigger analyzes CREATE TRIGGER statements
func (a *analyzer) analyzeCreateTrigger(stmt *pg_query.CreateTrigStmt) *operationInfo {
	return &operationInfo{
		operation: "CREATE TRIGGER",
		tableLock: ShareRowExclusive,
	}
}

// analyzeCreateRule analyzes CREATE RULE statements
func (a *analyzer) analyzeCreateRule(stmt *pg_query.RuleStmt) *operationInfo {
	return &operationInfo{
		operation: "CREATE RULE",
		tableLock: AccessExclusive,
	}
}

// analyzeCreatePolicy analyzes CREATE POLICY statements
func (a *analyzer) analyzeCreatePolicy(stmt *pg_query.CreatePolicyStmt) *operationInfo {
	return &operationInfo{
		operation: "CREATE POLICY",
		tableLock: AccessExclusive,
	}
}

// analyzeTransaction analyzes transaction control statements
func (a *analyzer) analyzeTransaction(stmt *pg_query.TransactionStmt) *operationInfo {
	switch stmt.Kind {
	case pg_query.TransactionStmtKind_TRANS_STMT_BEGIN:
		return &operationInfo{
			operation: "BEGIN",
			tableLock: AccessShare,
		}
	case pg_query.TransactionStmtKind_TRANS_STMT_START:
		return &operationInfo{
			operation: "START TRANSACTION",
			tableLock: AccessShare,
		}
	case pg_query.TransactionStmtKind_TRANS_STMT_COMMIT:
		return &operationInfo{
			operation: "COMMIT",
			tableLock: AccessShare,
		}
	case pg_query.TransactionStmtKind_TRANS_STMT_ROLLBACK:
		return &operationInfo{
			operation: "ROLLBACK",
			tableLock: AccessShare,
		}
	case pg_query.TransactionStmtKind_TRANS_STMT_SAVEPOINT:
		return &operationInfo{
			operation: "SAVEPOINT",
			tableLock: AccessShare,
		}
	case pg_query.TransactionStmtKind_TRANS_STMT_RELEASE:
		return &operationInfo{
			operation: "RELEASE SAVEPOINT",
			tableLock: AccessShare,
		}
	case pg_query.TransactionStmtKind_TRANS_STMT_ROLLBACK_TO:
		return &operationInfo{
			operation: "ROLLBACK TO SAVEPOINT",
			tableLock: AccessShare,
		}
	}

	return &operationInfo{
		operation: "TRANSACTION",
		tableLock: AccessShare,
	}
}

// analyzeVariableSet analyzes SET statements
func (a *analyzer) analyzeVariableSet(stmt *pg_query.VariableSetStmt) *operationInfo {
	switch stmt.Kind {
	case pg_query.VariableSetKind_VAR_SET_VALUE:
		// Check if it's SET LOCAL
		if stmt.IsLocal {
			return &operationInfo{
				operation: "SET LOCAL",
				tableLock: AccessShare,
			}
		}
		return &operationInfo{
			operation: "SET",
			tableLock: AccessShare,
		}
	case pg_query.VariableSetKind_VAR_SET_CURRENT:
		return &operationInfo{
			operation: "SET LOCAL",
			tableLock: AccessShare,
		}
	case pg_query.VariableSetKind_VAR_SET_DEFAULT:
		return &operationInfo{
			operation: "RESET",
			tableLock: AccessShare,
		}
	case pg_query.VariableSetKind_VAR_SET_MULTI:
		// VAR_SET_MULTI is used for both SET TRANSACTION and SET CONSTRAINTS
		// Check the name to distinguish between them
		if stmt.Name == "TRANSACTION" {
			return &operationInfo{
				operation: "SET TRANSACTION",
				tableLock: AccessShare,
			}
		}
		return &operationInfo{
			operation: "SET CONSTRAINTS",
			tableLock: AccessShare,
		}
	case pg_query.VariableSetKind_VAR_RESET:
		return &operationInfo{
			operation: "RESET",
			tableLock: AccessShare,
		}
	case pg_query.VariableSetKind_VAR_RESET_ALL:
		return &operationInfo{
			operation: "RESET ALL",
			tableLock: AccessShare,
		}
	}

	return &operationInfo{
		operation: "SET",
		tableLock: AccessShare,
	}
}

// analyzeAlterSystem analyzes ALTER SYSTEM statements
func (a *analyzer) analyzeAlterSystem(stmt *pg_query.AlterSystemStmt) *operationInfo {
	return &operationInfo{
		operation: "ALTER SYSTEM",
		tableLock: AccessExclusive,
	}
}

// analyzeCheckpoint analyzes CHECKPOINT statements
func (a *analyzer) analyzeCheckpoint(stmt *pg_query.CheckPointStmt) *operationInfo {
	return &operationInfo{
		operation: "CHECKPOINT",
		tableLock: AccessShare,
	}
}

// analyzeLoad analyzes LOAD statements
func (a *analyzer) analyzeLoad(stmt *pg_query.LoadStmt) *operationInfo {
	return &operationInfo{
		operation: "LOAD",
		tableLock: AccessShare,
	}
}

// analyzeCreateSubscription analyzes CREATE SUBSCRIPTION statements
func (a *analyzer) analyzeCreateSubscription(stmt *pg_query.CreateSubscriptionStmt) *operationInfo {
	return &operationInfo{
		operation: "CREATE SUBSCRIPTION",
		tableLock: AccessExclusive,
	}
}

// analyzeAlterSubscription analyzes ALTER SUBSCRIPTION statements
func (a *analyzer) analyzeAlterSubscription(stmt *pg_query.AlterSubscriptionStmt) *operationInfo {
	return &operationInfo{
		operation: "ALTER SUBSCRIPTION",
		tableLock: AccessExclusive,
	}
}

// analyzeCreatePublication analyzes CREATE PUBLICATION statements
func (a *analyzer) analyzeCreatePublication(stmt *pg_query.CreatePublicationStmt) *operationInfo {
	return &operationInfo{
		operation: "CREATE PUBLICATION",
		tableLock: AccessExclusive,
	}
}

// analyzeAlterPublication analyzes ALTER PUBLICATION statements
func (a *analyzer) analyzeAlterPublication(stmt *pg_query.AlterPublicationStmt) *operationInfo {
	operation := "ALTER PUBLICATION"

	// Check for specific actions
	switch stmt.Action {
	case pg_query.AlterPublicationAction_AP_AddObjects:
		operation = "ALTER PUBLICATION ADD TABLE"
	case pg_query.AlterPublicationAction_AP_DropObjects:
		operation = "ALTER PUBLICATION DROP TABLE"
	case pg_query.AlterPublicationAction_AP_SetObjects:
		operation = "ALTER PUBLICATION SET TABLE"
	}

	return &operationInfo{
		operation: operation,
		tableLock: AccessExclusive,
	}
}

// analyzeComment analyzes COMMENT statements
func (a *analyzer) analyzeComment(stmt *pg_query.CommentStmt) *operationInfo {
	return &operationInfo{
		operation: "COMMENT ON",
		tableLock: AccessShare,
	}
}

// analyzeRename analyzes RENAME statements (ALTER TABLE RENAME, etc.)
func (a *analyzer) analyzeRename(stmt *pg_query.RenameStmt) *operationInfo {
	switch stmt.RenameType {
	case pg_query.ObjectType_OBJECT_TABLE:
		return &operationInfo{
			operation: "ALTER TABLE RENAME TO",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_COLUMN:
		return &operationInfo{
			operation: "ALTER TABLE RENAME COLUMN",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_TABCONSTRAINT:
		return &operationInfo{
			operation: "ALTER TABLE RENAME CONSTRAINT",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_INDEX:
		return &operationInfo{
			operation: "ALTER INDEX",
			tableLock: ShareUpdateExclusive,
		}
	case pg_query.ObjectType_OBJECT_VIEW:
		return &operationInfo{
			operation: "ALTER VIEW",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_MATVIEW:
		return &operationInfo{
			operation: "ALTER MATERIALIZED VIEW RENAME TO",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_SEQUENCE:
		return &operationInfo{
			operation: "ALTER SEQUENCE RENAME TO",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_TYPE:
		return &operationInfo{
			operation: "ALTER TYPE",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_DATABASE:
		return &operationInfo{
			operation: "ALTER DATABASE RENAME TO",
			tableLock: Exclusive,
		}
	case pg_query.ObjectType_OBJECT_SCHEMA:
		return &operationInfo{
			operation: "ALTER SCHEMA RENAME TO",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_TABLESPACE:
		return &operationInfo{
			operation: "ALTER TABLESPACE",
			tableLock: AccessExclusive,
		}
	default:
		return &operationInfo{
			operation: "ALTER RENAME",
			tableLock: AccessExclusive,
		}
	}
}

// analyzeAlterObjectSchema analyzes ALTER ... SET SCHEMA statements
func (a *analyzer) analyzeAlterObjectSchema(stmt *pg_query.AlterObjectSchemaStmt) *operationInfo {
	switch stmt.ObjectType {
	case pg_query.ObjectType_OBJECT_TABLE:
		return &operationInfo{
			operation: "ALTER TABLE SET SCHEMA",
			tableLock: AccessExclusive,
		}
	default:
		return &operationInfo{
			operation: "ALTER SET SCHEMA",
			tableLock: AccessExclusive,
		}
	}
}

// analyzeDefine analyzes DEFINE statements (CREATE OPERATOR, CREATE AGGREGATE, etc.)
func (a *analyzer) analyzeDefine(stmt *pg_query.DefineStmt) *operationInfo {
	switch stmt.Kind {
	case pg_query.ObjectType_OBJECT_AGGREGATE:
		return &operationInfo{
			operation: "CREATE AGGREGATE",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_OPERATOR:
		return &operationInfo{
			operation: "CREATE OPERATOR",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_COLLATION:
		return &operationInfo{
			operation: "CREATE COLLATION",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_TSCONFIGURATION:
		return &operationInfo{
			operation: "CREATE TEXT SEARCH CONFIGURATION",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_TSDICTIONARY:
		return &operationInfo{
			operation: "CREATE TEXT SEARCH DICTIONARY",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_TSPARSER:
		return &operationInfo{
			operation: "CREATE TEXT SEARCH PARSER",
			tableLock: AccessExclusive,
		}
	case pg_query.ObjectType_OBJECT_TSTEMPLATE:
		return &operationInfo{
			operation: "CREATE TEXT SEARCH TEMPLATE",
			tableLock: AccessExclusive,
		}
	default:
		return &operationInfo{
			operation: "UNKNOWN",
			tableLock: AccessShare,
		}
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
