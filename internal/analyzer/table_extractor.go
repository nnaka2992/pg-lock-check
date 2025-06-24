package analyzer

import (
	"github.com/pganalyze/pg_query_go/v6"
)

// extractTables extracts all table names from an AST node
func extractTables(node *pg_query.Node) []string {
	if node == nil {
		return nil
	}

	extractor := &tableExtractor{
		tables: make(map[string]bool),
	}

	extractor.extractFromNode(node)

	// Convert map to slice
	result := make([]string, 0, len(extractor.tables))
	for table := range extractor.tables {
		result = append(result, table)
	}

	return result
}

// extractTablesWithContext extracts tables with their usage context (read vs write)
func extractTablesWithContext(node *pg_query.Node) map[string]LockType {
	if node == nil {
		return nil
	}

	result := make(map[string]LockType)

	// Handle specific statement types that have different locks for different tables
	switch n := node.Node.(type) {
	case *pg_query.Node_UpdateStmt:
		if n.UpdateStmt != nil {
			// Target table gets RowExclusive
			if n.UpdateStmt.Relation != nil {
				tableName := getQualifiedTableName(n.UpdateStmt.Relation)
				if tableName != "" {
					result[tableName] = RowExclusive
				}
			}
			// Tables in FROM clause get AccessShare
			for _, from := range n.UpdateStmt.FromClause {
				extractReadTables(from, result)
			}
			// Tables in WHERE clause subqueries get AccessShare
			if n.UpdateStmt.WhereClause != nil {
				extractReadTables(n.UpdateStmt.WhereClause, result)
			}
		}
	case *pg_query.Node_DeleteStmt:
		if n.DeleteStmt != nil {
			// Target table gets RowExclusive
			if n.DeleteStmt.Relation != nil {
				tableName := getQualifiedTableName(n.DeleteStmt.Relation)
				if tableName != "" {
					result[tableName] = RowExclusive
				}
			}
			// Tables in USING clause get AccessShare
			for _, using := range n.DeleteStmt.UsingClause {
				extractReadTables(using, result)
			}
		}
	case *pg_query.Node_InsertStmt:
		if n.InsertStmt != nil {
			// Target table gets RowExclusive
			if n.InsertStmt.Relation != nil {
				tableName := getQualifiedTableName(n.InsertStmt.Relation)
				if tableName != "" {
					result[tableName] = RowExclusive
				}
			}
			// Tables in SELECT get AccessShare
			if n.InsertStmt.SelectStmt != nil {
				extractReadTables(n.InsertStmt.SelectStmt, result)
			}
		}
	case *pg_query.Node_MergeStmt:
		if n.MergeStmt != nil {
			// Target table gets RowExclusive
			if n.MergeStmt.Relation != nil {
				tableName := getQualifiedTableName(n.MergeStmt.Relation)
				if tableName != "" {
					result[tableName] = RowExclusive
				}
			}
			// Source table/query gets AccessShare
			if n.MergeStmt.SourceRelation != nil {
				extractReadTables(n.MergeStmt.SourceRelation, result)
			}
		}
	case *pg_query.Node_CopyStmt:
		if n.CopyStmt != nil {
			if n.CopyStmt.Relation != nil {
				tableName := getQualifiedTableName(n.CopyStmt.Relation)
				if tableName != "" {
					// COPY FROM writes to the table, COPY TO reads from it
					if n.CopyStmt.IsFrom {
						result[tableName] = RowExclusive
					} else {
						result[tableName] = AccessShare
					}
				}
			}
		}
	case *pg_query.Node_DropStmt:
		if n.DropStmt != nil {
			// DROP operations get AccessExclusive lock
			switch n.DropStmt.RemoveType {
			case pg_query.ObjectType_OBJECT_TABLE,
				pg_query.ObjectType_OBJECT_VIEW,
				pg_query.ObjectType_OBJECT_MATVIEW,
				pg_query.ObjectType_OBJECT_SEQUENCE,
				pg_query.ObjectType_OBJECT_INDEX,
				pg_query.ObjectType_OBJECT_TRIGGER,
				pg_query.ObjectType_OBJECT_RULE,
				pg_query.ObjectType_OBJECT_POLICY:
				tables := extractTables(node)
				lockType := AccessExclusive

				// Special case for DROP INDEX CONCURRENTLY
				if n.DropStmt.RemoveType == pg_query.ObjectType_OBJECT_INDEX && n.DropStmt.Concurrent {
					lockType = ShareUpdateExclusive
				}

				for _, table := range tables {
					result[table] = lockType
				}
			}
		}
	case *pg_query.Node_TruncateStmt:
		if n.TruncateStmt != nil {
			// TRUNCATE gets AccessExclusive lock
			tables := extractTables(node)
			for _, table := range tables {
				result[table] = AccessExclusive
			}
		}
	case *pg_query.Node_ReindexStmt:
		if n.ReindexStmt != nil {
			// REINDEX gets AccessExclusive lock (except CONCURRENTLY)
			tables := extractTables(node)
			lockType := AccessExclusive

			// Check for CONCURRENTLY
			if n.ReindexStmt.Params != nil {
				for _, defElem := range n.ReindexStmt.Params {
					if de := defElem.GetDefElem(); de != nil && de.Defname == "concurrently" {
						lockType = ShareUpdateExclusive
						break
					}
				}
			}

			for _, table := range tables {
				result[table] = lockType
			}
		}
	case *pg_query.Node_AlterTableStmt:
		// ALTER TABLE operations - don't set lock type here, let the main analyzer handle it
		return nil
	default:
		// For other statements, don't set a default lock type - let the main analyzer handle it
		return nil
	}

	return result
}

// extractReadTables extracts tables that are being read (AccessShare lock)
func extractReadTables(node *pg_query.Node, result map[string]LockType) {
	if node == nil {
		return
	}

	extractor := &tableExtractor{
		tables: make(map[string]bool),
	}

	extractor.extractFromNode(node)

	for table := range extractor.tables {
		// Only add if not already present (write locks take precedence)
		if _, exists := result[table]; !exists {
			result[table] = AccessShare
		}
	}
}

// tableExtractor extracts table names from AST nodes
type tableExtractor struct {
	tables map[string]bool
}

// extractFromNode recursively extracts tables from a node
func (e *tableExtractor) extractFromNode(node *pg_query.Node) {
	if node == nil {
		return
	}

	// Handle different node types
	switch n := node.Node.(type) {
	case *pg_query.Node_RangeVar:
		e.extractFromRangeVar(n.RangeVar)
	case *pg_query.Node_UpdateStmt:
		e.extractFromUpdateStmt(n.UpdateStmt)
	case *pg_query.Node_DeleteStmt:
		e.extractFromDeleteStmt(n.DeleteStmt)
	case *pg_query.Node_InsertStmt:
		e.extractFromInsertStmt(n.InsertStmt)
	case *pg_query.Node_SelectStmt:
		e.extractFromSelectStmt(n.SelectStmt)
	case *pg_query.Node_AlterTableStmt:
		e.extractFromAlterTableStmt(n.AlterTableStmt)
	case *pg_query.Node_CreateStmt:
		e.extractFromCreateStmt(n.CreateStmt)
	case *pg_query.Node_DropStmt:
		e.extractFromDropStmt(n.DropStmt)
	case *pg_query.Node_TruncateStmt:
		e.extractFromTruncateStmt(n.TruncateStmt)
	case *pg_query.Node_IndexStmt:
		e.extractFromIndexStmt(n.IndexStmt)
	case *pg_query.Node_VacuumStmt:
		e.extractFromVacuumStmt(n.VacuumStmt)
	case *pg_query.Node_ClusterStmt:
		e.extractFromClusterStmt(n.ClusterStmt)
	case *pg_query.Node_CopyStmt:
		e.extractFromCopyStmt(n.CopyStmt)
	case *pg_query.Node_CreateTableAsStmt:
		e.extractFromCreateTableAsStmt(n.CreateTableAsStmt)
	case *pg_query.Node_ViewStmt:
		e.extractFromViewStmt(n.ViewStmt)
	case *pg_query.Node_RefreshMatViewStmt:
		e.extractFromRefreshMatViewStmt(n.RefreshMatViewStmt)
	case *pg_query.Node_LockStmt:
		e.extractFromLockStmt(n.LockStmt)
	case *pg_query.Node_CreateTrigStmt:
		e.extractFromCreateTrigStmt(n.CreateTrigStmt)
	case *pg_query.Node_RuleStmt:
		e.extractFromRuleStmt(n.RuleStmt)
	case *pg_query.Node_CreatePolicyStmt:
		e.extractFromCreatePolicyStmt(n.CreatePolicyStmt)
	case *pg_query.Node_GrantStmt:
		e.extractFromGrantStmt(n.GrantStmt)
	case *pg_query.Node_MergeStmt:
		e.extractFromMergeStmt(n.MergeStmt)
	case *pg_query.Node_ReindexStmt:
		e.extractFromReindexStmt(n.ReindexStmt)
	case *pg_query.Node_AlterPublicationStmt:
		e.extractFromAlterPublicationStmt(n.AlterPublicationStmt)
	case *pg_query.Node_RenameStmt:
		e.extractFromRenameStmt(n.RenameStmt)
	case *pg_query.Node_AlterObjectSchemaStmt:
		e.extractFromAlterObjectSchemaStmt(n.AlterObjectSchemaStmt)
	case *pg_query.Node_VacuumRelation:
		if n.VacuumRelation != nil && n.VacuumRelation.Relation != nil {
			e.extractFromRangeVar(n.VacuumRelation.Relation)
		}
	case *pg_query.Node_CreateSeqStmt:
		if n.CreateSeqStmt != nil && n.CreateSeqStmt.Sequence != nil {
			e.extractFromRangeVar(n.CreateSeqStmt.Sequence)
		}
	case *pg_query.Node_AlterSeqStmt:
		if n.AlterSeqStmt != nil && n.AlterSeqStmt.Sequence != nil {
			e.extractFromRangeVar(n.AlterSeqStmt.Sequence)
		}
	case *pg_query.Node_JoinExpr:
		e.extractFromJoinExpr(n.JoinExpr)
	case *pg_query.Node_FromExpr:
		e.extractFromFromExpr(n.FromExpr)
	case *pg_query.Node_SubLink:
		e.extractFromSubLink(n.SubLink)
	case *pg_query.Node_CommonTableExpr:
		e.extractFromCommonTableExpr(n.CommonTableExpr)
	case *pg_query.Node_RawStmt:
		if n.RawStmt != nil && n.RawStmt.Stmt != nil {
			e.extractFromNode(n.RawStmt.Stmt)
		}
	case *pg_query.Node_List:
		e.extractFromList(n.List)
	}
}

// extractFromRangeVar extracts table name from a RangeVar
func (e *tableExtractor) extractFromRangeVar(rv *pg_query.RangeVar) {
	if rv == nil {
		return
	}

	tableName := getQualifiedTableName(rv)
	if tableName != "" {
		e.tables[tableName] = true
	}
}

// extractFromUpdateStmt extracts tables from UPDATE statements
func (e *tableExtractor) extractFromUpdateStmt(stmt *pg_query.UpdateStmt) {
	if stmt == nil {
		return
	}

	// Extract target table
	if stmt.Relation != nil {
		e.extractFromRangeVar(stmt.Relation)
	}

	// Extract from FROM clause
	for _, from := range stmt.FromClause {
		e.extractFromNode(from)
	}

	// Extract from WHERE clause
	if stmt.WhereClause != nil {
		e.extractFromNode(stmt.WhereClause)
	}

	// Extract from RETURNING clause
	for _, ret := range stmt.ReturningList {
		e.extractFromNode(ret)
	}
}

// extractFromDeleteStmt extracts tables from DELETE statements
func (e *tableExtractor) extractFromDeleteStmt(stmt *pg_query.DeleteStmt) {
	if stmt == nil {
		return
	}

	// Extract target table
	if stmt.Relation != nil {
		e.extractFromRangeVar(stmt.Relation)
	}

	// Extract from USING clause
	for _, using := range stmt.UsingClause {
		e.extractFromNode(using)
	}

	// Extract from WHERE clause
	if stmt.WhereClause != nil {
		e.extractFromNode(stmt.WhereClause)
	}
}

// extractFromInsertStmt extracts tables from INSERT statements
func (e *tableExtractor) extractFromInsertStmt(stmt *pg_query.InsertStmt) {
	if stmt == nil {
		return
	}

	// Extract target table
	if stmt.Relation != nil {
		e.extractFromRangeVar(stmt.Relation)
	}

	// Extract from SELECT statement
	if stmt.SelectStmt != nil {
		e.extractFromNode(stmt.SelectStmt)
	}
}

// extractFromSelectStmt extracts tables from SELECT statements
func (e *tableExtractor) extractFromSelectStmt(stmt *pg_query.SelectStmt) {
	if stmt == nil {
		return
	}

	// Extract from FROM clause
	for _, from := range stmt.FromClause {
		e.extractFromNode(from)
	}

	// Extract from WHERE clause
	if stmt.WhereClause != nil {
		e.extractFromNode(stmt.WhereClause)
	}

	// Extract from WITH clause (CTEs)
	if stmt.WithClause != nil {
		for _, cte := range stmt.WithClause.Ctes {
			e.extractFromNode(cte)
		}
	}

	// Extract from subqueries in target list
	for _, target := range stmt.TargetList {
		e.extractFromNode(target)
	}

	// Handle set operations (UNION, INTERSECT, EXCEPT)
	if stmt.Larg != nil {
		e.extractFromSelectStmt(stmt.Larg)
	}
	if stmt.Rarg != nil {
		e.extractFromSelectStmt(stmt.Rarg)
	}
}

// extractFromAlterTableStmt extracts tables from ALTER TABLE statements
func (e *tableExtractor) extractFromAlterTableStmt(stmt *pg_query.AlterTableStmt) {
	if stmt == nil {
		return
	}

	if stmt.Relation != nil {
		e.extractFromRangeVar(stmt.Relation)
	}
}

// extractFromCreateStmt extracts tables from CREATE TABLE statements
func (e *tableExtractor) extractFromCreateStmt(stmt *pg_query.CreateStmt) {
	if stmt == nil {
		return
	}

	// Don't extract the table being created, only referenced tables
	// Extract from INHERITS clause
	for _, inherit := range stmt.InhRelations {
		e.extractFromNode(inherit)
	}

	// Extract from table constraints (foreign keys, etc.)
	for _, constraint := range stmt.Constraints {
		e.extractFromNode(constraint)
	}

	// Extract from LIKE clause
	for _, tableElt := range stmt.TableElts {
		e.extractFromNode(tableElt)
	}
}

// extractFromDropStmt extracts tables from DROP statements
func (e *tableExtractor) extractFromDropStmt(stmt *pg_query.DropStmt) {
	if stmt == nil {
		return
	}

	// Extract object names for all DROP types that we track
	switch stmt.RemoveType {
	case pg_query.ObjectType_OBJECT_TABLE,
		pg_query.ObjectType_OBJECT_VIEW,
		pg_query.ObjectType_OBJECT_MATVIEW,
		pg_query.ObjectType_OBJECT_SEQUENCE,
		pg_query.ObjectType_OBJECT_INDEX,
		pg_query.ObjectType_OBJECT_TRIGGER,
		pg_query.ObjectType_OBJECT_RULE,
		pg_query.ObjectType_OBJECT_POLICY:
		for _, obj := range stmt.Objects {
			// Each object is a List containing String_ nodes
			if listNode, ok := obj.Node.(*pg_query.Node_List); ok && listNode.List != nil {
				parts := []string{}
				for _, item := range listNode.List.Items {
					if strNode, ok := item.Node.(*pg_query.Node_String_); ok && strNode.String_ != nil {
						parts = append(parts, strNode.String_.Sval)
					}
				}

				// Handle different DROP types
				var objName string
				switch stmt.RemoveType {
				case pg_query.ObjectType_OBJECT_TRIGGER,
					pg_query.ObjectType_OBJECT_RULE,
					pg_query.ObjectType_OBJECT_POLICY:
					// For TRIGGER/RULE/POLICY: first item is table name, second is object name
					if len(parts) >= 1 {
						objName = quoteIdentifier(parts[0]) // Table name is first
					}
				case pg_query.ObjectType_OBJECT_INDEX:
					// For INDEX: treat the index name as the object being locked
					if len(parts) == 1 {
						objName = quoteIdentifier(parts[0])
					} else if len(parts) >= 2 {
						// Schema.index
						objName = quoteQualifiedIdentifier(parts[0], parts[1])
					}
				default:
					// For other object types (TABLE, VIEW, etc.)
					if len(parts) == 1 {
						// Simple object name
						objName = quoteIdentifier(parts[0])
					} else if len(parts) == 2 {
						// Schema.object
						objName = quoteQualifiedIdentifier(parts[0], parts[1])
					} else if len(parts) > 2 {
						// Could be database.schema.object, but usually just schema.object
						objName = quoteQualifiedIdentifier(parts[len(parts)-2], parts[len(parts)-1])
					}
				}

				if objName != "" {
					e.tables[objName] = true
				}
			}
		}
	}
}

// extractFromTruncateStmt extracts tables from TRUNCATE statements
func (e *tableExtractor) extractFromTruncateStmt(stmt *pg_query.TruncateStmt) {
	if stmt == nil {
		return
	}

	for _, rel := range stmt.Relations {
		e.extractFromNode(rel)
	}
}

// extractFromIndexStmt extracts tables from CREATE INDEX statements
func (e *tableExtractor) extractFromIndexStmt(stmt *pg_query.IndexStmt) {
	if stmt == nil {
		return
	}

	if stmt.Relation != nil {
		e.extractFromRangeVar(stmt.Relation)
	}
}

// extractFromVacuumStmt extracts tables from VACUUM statements
func (e *tableExtractor) extractFromVacuumStmt(stmt *pg_query.VacuumStmt) {
	if stmt == nil {
		return
	}

	for _, rel := range stmt.Rels {
		e.extractFromNode(rel)
	}
}

// extractFromClusterStmt extracts tables from CLUSTER statements
func (e *tableExtractor) extractFromClusterStmt(stmt *pg_query.ClusterStmt) {
	if stmt == nil {
		return
	}

	if stmt.Relation != nil {
		e.extractFromRangeVar(stmt.Relation)
	}
}

// extractFromCopyStmt extracts tables from COPY statements
func (e *tableExtractor) extractFromCopyStmt(stmt *pg_query.CopyStmt) {
	if stmt == nil {
		return
	}

	if stmt.Relation != nil {
		e.extractFromRangeVar(stmt.Relation)
	}

	// Extract from query
	if stmt.Query != nil {
		e.extractFromNode(stmt.Query)
	}
}

// extractFromCreateTableAsStmt extracts tables from CREATE TABLE AS statements
func (e *tableExtractor) extractFromCreateTableAsStmt(stmt *pg_query.CreateTableAsStmt) {
	if stmt == nil {
		return
	}

	// Extract from query (source tables)
	if stmt.Query != nil {
		e.extractFromNode(stmt.Query)
	}
}

// extractFromViewStmt extracts tables from CREATE VIEW statements
func (e *tableExtractor) extractFromViewStmt(stmt *pg_query.ViewStmt) {
	if stmt == nil {
		return
	}

	// Extract from query
	if stmt.Query != nil {
		e.extractFromNode(stmt.Query)
	}
}

// extractFromRefreshMatViewStmt extracts tables from REFRESH MATERIALIZED VIEW
func (e *tableExtractor) extractFromRefreshMatViewStmt(stmt *pg_query.RefreshMatViewStmt) {
	if stmt == nil {
		return
	}

	if stmt.Relation != nil {
		e.extractFromRangeVar(stmt.Relation)
	}
}

// extractFromLockStmt extracts tables from LOCK statements
func (e *tableExtractor) extractFromLockStmt(stmt *pg_query.LockStmt) {
	if stmt == nil {
		return
	}

	for _, rel := range stmt.Relations {
		e.extractFromNode(rel)
	}
}

// extractFromCreateTrigStmt extracts tables from CREATE TRIGGER statements
func (e *tableExtractor) extractFromCreateTrigStmt(stmt *pg_query.CreateTrigStmt) {
	if stmt == nil {
		return
	}

	if stmt.Relation != nil {
		e.extractFromRangeVar(stmt.Relation)
	}
}

// extractFromRuleStmt extracts tables from CREATE RULE statements
func (e *tableExtractor) extractFromRuleStmt(stmt *pg_query.RuleStmt) {
	if stmt == nil {
		return
	}

	if stmt.Relation != nil {
		e.extractFromRangeVar(stmt.Relation)
	}
}

// extractFromCreatePolicyStmt extracts tables from CREATE POLICY statements
func (e *tableExtractor) extractFromCreatePolicyStmt(stmt *pg_query.CreatePolicyStmt) {
	if stmt == nil {
		return
	}

	if stmt.Table != nil {
		e.extractFromRangeVar(stmt.Table)
	}
}

// extractFromGrantStmt extracts tables from GRANT/REVOKE statements
func (e *tableExtractor) extractFromGrantStmt(stmt *pg_query.GrantStmt) {
	if stmt == nil {
		return
	}

	// Only extract for table-related grants
	if stmt.Objtype == pg_query.ObjectType_OBJECT_TABLE {
		for _, obj := range stmt.Objects {
			e.extractFromNode(obj)
		}
	}
}

// extractFromMergeStmt extracts tables from MERGE statements
func (e *tableExtractor) extractFromMergeStmt(stmt *pg_query.MergeStmt) {
	if stmt == nil {
		return
	}

	// Extract target table
	if stmt.Relation != nil {
		e.extractFromRangeVar(stmt.Relation)
	}

	// Extract source table/query
	if stmt.SourceRelation != nil {
		e.extractFromNode(stmt.SourceRelation)
	}

	// Extract from join condition
	if stmt.JoinCondition != nil {
		e.extractFromNode(stmt.JoinCondition)
	}
}

// extractFromReindexStmt extracts tables from REINDEX statements
func (e *tableExtractor) extractFromReindexStmt(stmt *pg_query.ReindexStmt) {
	if stmt == nil {
		return
	}

	// Only extract for table/index reindexing
	if stmt.Kind == pg_query.ReindexObjectType_REINDEX_OBJECT_TABLE ||
		stmt.Kind == pg_query.ReindexObjectType_REINDEX_OBJECT_INDEX {
		if stmt.Relation != nil {
			e.extractFromRangeVar(stmt.Relation)
		}
	}
}

// extractFromAlterPublicationStmt extracts tables from ALTER PUBLICATION statements
func (e *tableExtractor) extractFromAlterPublicationStmt(stmt *pg_query.AlterPublicationStmt) {
	if stmt == nil {
		return
	}

	// Extract from publication object specs
	for _, pubobj := range stmt.Pubobjects {
		e.extractFromNode(pubobj)
	}
}

// extractFromRenameStmt extracts tables from RENAME statements
func (e *tableExtractor) extractFromRenameStmt(stmt *pg_query.RenameStmt) {
	if stmt == nil {
		return
	}

	// For table/view/sequence/matview renames, extract the source table
	switch stmt.RenameType {
	case pg_query.ObjectType_OBJECT_TABLE,
		pg_query.ObjectType_OBJECT_VIEW,
		pg_query.ObjectType_OBJECT_MATVIEW,
		pg_query.ObjectType_OBJECT_SEQUENCE,
		pg_query.ObjectType_OBJECT_INDEX:
		if stmt.Relation != nil {
			tableName := getQualifiedTableName(stmt.Relation)
			if tableName != "" {
				e.tables[tableName] = true
			}
		}
	case pg_query.ObjectType_OBJECT_COLUMN,
		pg_query.ObjectType_OBJECT_TABCONSTRAINT:
		// For column/constraint renames, the table is in relation
		if stmt.Relation != nil {
			tableName := getQualifiedTableName(stmt.Relation)
			if tableName != "" {
				e.tables[tableName] = true
			}
		}
	}
}

// extractFromAlterObjectSchemaStmt extracts tables from ALTER ... SET SCHEMA statements
func (e *tableExtractor) extractFromAlterObjectSchemaStmt(stmt *pg_query.AlterObjectSchemaStmt) {
	if stmt == nil {
		return
	}

	// For table/view/sequence/matview schema changes, extract the table
	switch stmt.ObjectType {
	case pg_query.ObjectType_OBJECT_TABLE,
		pg_query.ObjectType_OBJECT_VIEW,
		pg_query.ObjectType_OBJECT_MATVIEW,
		pg_query.ObjectType_OBJECT_SEQUENCE:
		if stmt.Relation != nil {
			tableName := getQualifiedTableName(stmt.Relation)
			if tableName != "" {
				e.tables[tableName] = true
			}
		}
	}
}

// extractFromJoinExpr extracts tables from JOIN expressions
func (e *tableExtractor) extractFromJoinExpr(join *pg_query.JoinExpr) {
	if join == nil {
		return
	}

	// Extract from left and right sides
	if join.Larg != nil {
		e.extractFromNode(join.Larg)
	}
	if join.Rarg != nil {
		e.extractFromNode(join.Rarg)
	}

	// Extract from join condition
	if join.Quals != nil {
		e.extractFromNode(join.Quals)
	}
}

// extractFromFromExpr extracts tables from FROM expressions
func (e *tableExtractor) extractFromFromExpr(from *pg_query.FromExpr) {
	if from == nil {
		return
	}

	// Extract from FROM list
	for _, item := range from.Fromlist {
		e.extractFromNode(item)
	}

	// Extract from WHERE clause
	if from.Quals != nil {
		e.extractFromNode(from.Quals)
	}
}

// extractFromSubLink extracts tables from subquery links
func (e *tableExtractor) extractFromSubLink(sublink *pg_query.SubLink) {
	if sublink == nil {
		return
	}

	// Extract from subselect
	if sublink.Subselect != nil {
		e.extractFromNode(sublink.Subselect)
	}
}

// extractFromCommonTableExpr extracts tables from CTEs
func (e *tableExtractor) extractFromCommonTableExpr(cte *pg_query.CommonTableExpr) {
	if cte == nil {
		return
	}

	// Extract from CTE query
	if cte.Ctequery != nil {
		e.extractFromNode(cte.Ctequery)
	}
}

// extractFromList extracts tables from lists
func (e *tableExtractor) extractFromList(list *pg_query.List) {
	if list == nil {
		return
	}

	for _, item := range list.Items {
		e.extractFromNode(item)
	}
}

// Helper function to get table name with schema
func getQualifiedTableName(rv *pg_query.RangeVar) string {
	if rv == nil {
		return ""
	}

	return quoteQualifiedIdentifier(rv.Schemaname, rv.Relname)
}
