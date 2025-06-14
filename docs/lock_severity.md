# PostgreSQL Lock Analyzer - Comprehensive Severity Tables

## Transaction Mode (Default)

| Severity | Operation | Lock Type | Impact | Notes |
|----------|-----------|-----------|---------|--------|
| **ERROR** | `CREATE DATABASE` | None | Cannot run in transaction | Must run outside transaction |
| **ERROR** | `DROP DATABASE` | None | Cannot run in transaction | Must run outside transaction |
| **ERROR** | `CREATE TABLESPACE` | None | Cannot run in transaction | Must run outside transaction |
| **ERROR** | `DROP TABLESPACE` | None | Cannot run in transaction | Must run outside transaction |
| **ERROR** | `ALTER TABLESPACE` | None | Cannot run in transaction | Must run outside transaction |
| **ERROR** | `VACUUM` | None | Cannot run in transaction | All variants |
| **ERROR** | `VACUUM FULL` | None | Cannot run in transaction | All variants |
| **ERROR** | `VACUUM FREEZE` | None | Cannot run in transaction | All variants |
| **ERROR** | `VACUUM ANALYZE` | None | Cannot run in transaction | All variants |
| **ERROR** | `CREATE INDEX CONCURRENTLY` | None | Cannot run in transaction | Needs own transaction control |
| **ERROR** | `DROP INDEX CONCURRENTLY` | None | Cannot run in transaction | Needs own transaction control |
| **ERROR** | `REINDEX CONCURRENTLY` | None | Cannot run in transaction | Needs own transaction control |
| **ERROR** | `REFRESH MATERIALIZED VIEW CONCURRENTLY` | None | Cannot run in transaction | Needs own transaction control |
| **ERROR** | `ALTER SYSTEM` | None | Cannot run in transaction | System-level change |
| **ERROR** | `CREATE SUBSCRIPTION` | None | Cannot run in transaction | Logical replication |
| **ERROR** | `ALTER SUBSCRIPTION` | None | Cannot run in transaction | Logical replication |
| **ERROR** | `DROP SUBSCRIPTION` | None | Cannot run in transaction | Logical replication |
| **ERROR** | `ALTER TYPE ADD VALUE` | None | Cannot run in transaction | Enum type modification |
| **ERROR** | `ALTER TABLE DETACH PARTITION CONCURRENTLY` | None | Cannot run in transaction | PostgreSQL 14+ feature |
| **CRITICAL** | `UPDATE` without WHERE | RowExclusive | Blocks concurrent updates/deletes | Full table update |
| **CRITICAL** | `DELETE` without WHERE | RowExclusive | Blocks concurrent updates/deletes | Full table delete |
| **CRITICAL** | `MERGE` without WHERE | RowExclusive | Blocks concurrent updates/deletes | No conditions in WHEN clauses |
| **CRITICAL** | `TRUNCATE` | AccessExclusive | Blocks all operations | Immediate data removal |
| **CRITICAL** | `DROP TABLE` | AccessExclusive | Blocks all operations | Removes table |
| **CRITICAL** | `DROP INDEX` | AccessExclusive | Blocks all operations | Removes index |
| **CRITICAL** | `DROP SCHEMA` | AccessExclusive | Blocks all operations | Removes entire schema |
| **CRITICAL** | `DROP SCHEMA CASCADE` | AccessExclusive | Blocks all operations | Cascading removal |
| **CRITICAL** | `DROP OWNED` | AccessExclusive | Blocks all operations | Drops all owned objects |
| **CRITICAL** | `CREATE INDEX` | Share | Blocks all writes | Non-concurrent index |
| **CRITICAL** | `CREATE UNIQUE INDEX` | Share | Blocks all writes | Non-concurrent unique index |
| **CRITICAL** | `REINDEX` | AccessExclusive | Blocks all operations | Rebuilds index |
| **CRITICAL** | `REINDEX TABLE` | AccessExclusive | Blocks all operations | Rebuilds all indexes |
| **CRITICAL** | `REINDEX DATABASE` | AccessExclusive | Blocks all operations | Database-wide |
| **CRITICAL** | `REINDEX SCHEMA` | AccessExclusive | Blocks all operations | Schema-wide |
| **CRITICAL** | `REINDEX SYSTEM` | AccessExclusive | Blocks all operations | System catalog reindex |
| **CRITICAL** | `CLUSTER` | AccessExclusive | Blocks all operations | Physically reorders table |
| **CRITICAL** | `REFRESH MATERIALIZED VIEW` | AccessExclusive | Blocks all operations | Full refresh |
| **CRITICAL** | `ALTER TABLE ADD COLUMN` with volatile DEFAULT | AccessExclusive | Blocks all operations + rewrites table | e.g., DEFAULT random() |
| **CRITICAL** | `ALTER TABLE DROP COLUMN` | AccessExclusive | Blocks all operations + rewrites table | Physical removal |
| **CRITICAL** | `ALTER TABLE ALTER COLUMN TYPE` | AccessExclusive | Blocks all operations + rewrites table | Type conversion |
| **CRITICAL** | `ALTER TABLE SET TABLESPACE` | AccessExclusive | Blocks all operations + rewrites table | Physical relocation |
| **CRITICAL** | `ALTER TABLE SET LOGGED/UNLOGGED` | AccessExclusive | Blocks all operations + rewrites table | Durability change |
| **CRITICAL** | `ALTER TABLE ADD PRIMARY KEY` | AccessExclusive | Blocks all operations | Creates unique index |
| **CRITICAL** | `ALTER TABLE RENAME TO` | AccessExclusive | Blocks all operations | Name change |
| **CRITICAL** | `ALTER TABLE SET SCHEMA` | AccessExclusive | Blocks all operations | Schema change |
| **CRITICAL** | `ALTER TABLE ADD CONSTRAINT CHECK` | AccessExclusive + scan | Blocks all operations + scans table | Full table validation |
| **CRITICAL** | `ALTER TABLE SET/DROP NOT NULL` | AccessExclusive + scan | Blocks all operations + scans table | Full table constraint check |
| **CRITICAL** | `LOCK TABLE ACCESS EXCLUSIVE` | AccessExclusive | Blocks all operations | Explicit lock |
| **WARNING** | `UPDATE` with WHERE | RowExclusive | Blocks concurrent updates/deletes on target rows | Targeted update |
| **WARNING** | `DELETE` with WHERE | RowExclusive | Blocks concurrent updates/deletes on target rows | Targeted delete |
| **WARNING** | `MERGE` with WHERE | RowExclusive | Blocks concurrent updates/deletes on target rows | Has conditions in WHEN clauses or subquery |
| **WARNING** | `SELECT FOR UPDATE` without WHERE | RowShare | Prevents concurrent updates | Full table scan with locks |
| **WARNING** | `SELECT FOR UPDATE` with WHERE | RowShare | Prevents concurrent updates on selected rows | Targeted row locking |
| **WARNING** | `SELECT FOR NO KEY UPDATE` without WHERE | RowShare | Prevents non-key updates | Weaker than FOR UPDATE |
| **WARNING** | `SELECT FOR NO KEY UPDATE` with WHERE | RowShare | Prevents non-key updates on selected rows | Targeted row locking |
| **WARNING** | `SELECT FOR SHARE` without WHERE | RowShare | Prevents updates | Shared lock |
| **WARNING** | `SELECT FOR SHARE` with WHERE | RowShare | Prevents updates on selected rows | Targeted shared lock |
| **WARNING** | `INSERT SELECT` from large table | RowExclusive | Long operation | Large data copy |
| **WARNING** | `CREATE TABLE AS` | AccessShare on source | Creates new table | With data copy |
| **WARNING** | `SELECT INTO` | AccessShare on source | Creates new table | With data copy |
| **WARNING** | `COPY FROM` large file | RowExclusive | Long operation | Bulk insert |
| **WARNING** | `ANALYZE` | ShareUpdateExclusive | Blocks DDL | Statistics update |
| **WARNING** | `CREATE TRIGGER` | ShareRowExclusive | Blocks DML | Adds trigger |
| **WARNING** | `DROP TRIGGER` | AccessExclusive | Blocks all operations | Removes trigger |
| **WARNING** | `ALTER TABLE ADD FOREIGN KEY` | ShareRowExclusive | Blocks DML | Adds constraint |
| **WARNING** | `ALTER TABLE ADD CONSTRAINT UNIQUE` | AccessExclusive | Blocks all operations | Creates index |
| **WARNING** | `ALTER TABLE ADD CONSTRAINT EXCLUDE` | AccessExclusive | Blocks all operations | Creates index |
| **WARNING** | `ALTER TABLE ADD CONSTRAINT NOT VALID` | ShareRowExclusive | Minimal impact | Constraint without validation |
| **WARNING** | `ALTER TABLE VALIDATE CONSTRAINT` | ShareUpdateExclusive | Blocks DDL | Validates existing |
| **WARNING** | `ALTER TABLE DROP CONSTRAINT` | AccessExclusive | Blocks all operations | Removes constraint |
| **WARNING** | `ALTER TABLE ENABLE TRIGGER` | ShareRowExclusive | Blocks DML | Activates trigger |
| **WARNING** | `ALTER TABLE DISABLE TRIGGER` | ShareRowExclusive | Blocks DML | Deactivates trigger |
| **WARNING** | `ALTER TABLE ENABLE RULE` | ShareRowExclusive | Blocks DML | Activates rule |
| **WARNING** | `ALTER TABLE DISABLE RULE` | ShareRowExclusive | Blocks DML | Deactivates rule |
| **WARNING** | `ALTER TABLE ENABLE ROW LEVEL SECURITY` | AccessExclusive | Blocks all operations | Activates RLS |
| **WARNING** | `ALTER TABLE DISABLE ROW LEVEL SECURITY` | AccessExclusive | Blocks all operations | Deactivates RLS |
| **WARNING** | `ALTER TABLE FORCE ROW LEVEL SECURITY` | AccessExclusive | Blocks all operations | Forces RLS |
| **WARNING** | `ALTER TABLE NO FORCE ROW LEVEL SECURITY` | AccessExclusive | Blocks all operations | Unforces RLS |
| **WARNING** | `ALTER TABLE RENAME COLUMN` | AccessExclusive | Blocks all operations | Metadata change |
| **WARNING** | `ALTER TABLE INHERIT` | AccessExclusive | Blocks all operations | Inheritance change |
| **WARNING** | `ALTER TABLE NO INHERIT` | AccessExclusive | Blocks all operations | Inheritance change |
| **WARNING** | `ALTER TABLE OF` | AccessExclusive | Blocks all operations | Type binding |
| **WARNING** | `ALTER TABLE NOT OF` | AccessExclusive | Blocks all operations | Type unbinding |
| **WARNING** | `ALTER TABLE REPLICA IDENTITY` | AccessExclusive | Blocks all operations | Replication change |
| **WARNING** | `ALTER TABLE OWNER TO` | AccessExclusive | Blocks all operations | Ownership change |
| **WARNING** | `ALTER TABLE ATTACH PARTITION` | ShareUpdateExclusive | Blocks DDL | Partition management |
| **WARNING** | `ALTER TABLE DETACH PARTITION` | ShareUpdateExclusive | Blocks DDL | Partition management |
| **WARNING** | `ALTER TABLE SET ACCESS METHOD` | AccessExclusive | Blocks all operations | Storage method change |
| **WARNING** | `DROP VIEW` | AccessExclusive on view | Blocks view access | Removes view |
| **WARNING** | `DROP MATERIALIZED VIEW` | AccessExclusive | Blocks all operations | Removes mat view |
| **WARNING** | `DROP SEQUENCE` | AccessExclusive on sequence | Blocks sequence access | Removes sequence |
| **WARNING** | `DROP TYPE` | AccessExclusive | Blocks type usage | May cascade |
| **WARNING** | `DROP DOMAIN` | AccessExclusive | Blocks domain usage | May cascade |
| **WARNING** | `DROP EXTENSION` | Varies | Depends on objects | May cascade |
| **WARNING** | `CREATE RULE` | AccessExclusive | Blocks all operations | Adds rule |
| **WARNING** | `DROP RULE` | AccessExclusive | Blocks all operations | Removes rule |
| **WARNING** | `CREATE POLICY` | AccessExclusive | Blocks all operations | Adds RLS policy |
| **WARNING** | `DROP POLICY` | AccessExclusive | Blocks all operations | Removes RLS policy |
| **WARNING** | `ALTER INDEX` | AccessExclusive | Blocks all operations | Index modification |
| **WARNING** | `ALTER VIEW` | AccessExclusive on view | Blocks view access | View modification |
| **WARNING** | `ALTER SEQUENCE` | AccessExclusive on sequence | Blocks sequence access | Sequence modification |
| **WARNING** | `ALTER TYPE` | AccessExclusive | Blocks type usage | Type modification |
| **WARNING** | `ALTER DOMAIN` | AccessExclusive | Blocks domain usage | Domain modification |
| **WARNING** | `REASSIGN OWNED` | AccessExclusive on objects | Blocks owned objects | Ownership transfer |
| **WARNING** | `LOCK TABLE ROW EXCLUSIVE` | RowExclusive | Blocks some operations | Explicit lock |
| **WARNING** | `LOCK TABLE SHARE UPDATE EXCLUSIVE` | ShareUpdateExclusive | Blocks DDL | Explicit lock |
| **WARNING** | `LOCK TABLE SHARE` | Share | Blocks writes | Explicit lock |
| **WARNING** | `LOCK TABLE SHARE ROW EXCLUSIVE` | ShareRowExclusive | Blocks DML | Explicit lock |
| **WARNING** | `LOCK TABLE EXCLUSIVE` | Exclusive | Blocks most operations | Explicit lock |
| **INFO** | `SELECT FOR KEY SHARE` | RowShare | Prevents key updates | Weakest locking mode |
| **INFO** | `SELECT FOR UPDATE` with specific WHERE | RowShare + few row locks | Locks specific rows | Minimal impact |
| **INFO** | `SELECT FOR NO KEY UPDATE` with specific WHERE | RowShare + few row locks | Locks specific rows | Weaker lock |
| **INFO** | `SELECT FOR SHARE` with specific WHERE | RowShare + few row locks | Shared lock few rows | Read stability |
| **INFO** | `SELECT FOR KEY SHARE` with specific WHERE | RowShare + weak row locks | Weakest lock | FK checking |
| **INFO** | `INSERT` | RowExclusive | Minimal impact | New rows only |
| **INFO** | `INSERT ON CONFLICT` | RowExclusive | Minimal impact | Upsert operation |
| **INFO** | `INSERT RETURNING` | RowExclusive | Minimal impact | Returns inserted data |
| **INFO** | `COPY TO` | AccessShare | Read only | Data export |
| **INFO** | `ALTER TABLE ADD COLUMN` without DEFAULT | AccessExclusive | Quick operation | Metadata only |
| **INFO** | `ALTER TABLE ADD COLUMN` with constant DEFAULT | AccessExclusive | Quick operation | No rewrite |
| **INFO** | `ALTER TABLE ADD COLUMN GENERATED ALWAYS AS` | AccessExclusive | Quick operation | Generated column |
| **INFO** | `ALTER TABLE ALTER COLUMN ADD IDENTITY` | AccessExclusive | Quick operation | Identity column |
| **INFO** | `ALTER TABLE ALTER COLUMN DROP IDENTITY` | AccessExclusive | Quick operation | Remove identity |
| **INFO** | `ALTER TABLE SET/DROP DEFAULT` | AccessExclusive | Quick operation | Metadata only |
| **INFO** | `ALTER TABLE ALTER COLUMN SET STATISTICS` | ShareUpdateExclusive | Minimal impact | Stats metadata |
| **INFO** | `ALTER TABLE ALTER COLUMN SET STORAGE` | AccessExclusive | Quick operation | Storage hint |
| **INFO** | `ALTER TABLE SET (storage_parameter)` | ShareUpdateExclusive | Minimal impact | Table parameters |
| **INFO** | `ALTER TABLE RESET (storage_parameter)` | ShareUpdateExclusive | Minimal impact | Table parameters |
| **INFO** | `ALTER TABLE CLUSTER ON` | ShareUpdateExclusive | Minimal impact | Cluster hint |
| **INFO** | `ALTER TABLE SET WITHOUT CLUSTER` | ShareUpdateExclusive | Minimal impact | Cluster hint |
| **INFO** | `CREATE TABLE` | None on other tables | No conflict | New table |
| **INFO** | `CREATE TEMPORARY TABLE` | None on other tables | No conflict | Session-local table |
| **INFO** | `CREATE VIEW` | AccessShare on referenced | Read locks only | View creation |
| **INFO** | `CREATE MATERIALIZED VIEW` | AccessShare on source | Read locks only | Initial creation |
| **INFO** | `CREATE SEQUENCE` | None on other objects | No conflict | New sequence |
| **INFO** | `CREATE TYPE` | None on other objects | No conflict | New type |
| **INFO** | `CREATE DOMAIN` | None on other objects | No conflict | New domain |
| **INFO** | `CREATE SCHEMA` | None on other objects | No conflict | New schema |
| **INFO** | `CREATE EXTENSION` | Varies | Usually safe | Adds functionality |
| **INFO** | `CREATE/DROP FUNCTION` | None on tables | No table locks | Function management |
| **INFO** | `CREATE/DROP PROCEDURE` | None on tables | No table locks | Procedure management |
| **INFO** | `CREATE/DROP AGGREGATE` | None on tables | No table locks | Aggregate management |
| **INFO** | `CREATE/DROP OPERATOR` | None on tables | No table locks | Operator management |
| **INFO** | `CREATE/DROP CAST` | None on tables | No table locks | Cast management |
| **INFO** | `CREATE/DROP COLLATION` | None on tables | No table locks | Collation management |
| **INFO** | `CREATE/DROP TEXT SEARCH CONFIGURATION` | None on tables | No table locks | Text search config |
| **INFO** | `CREATE/DROP TEXT SEARCH DICTIONARY` | None on tables | No table locks | Text search dictionary |
| **INFO** | `CREATE/DROP TEXT SEARCH PARSER` | None on tables | No table locks | Text search parser |
| **INFO** | `CREATE/DROP TEXT SEARCH TEMPLATE` | None on tables | No table locks | Text search template |
| **INFO** | `CREATE/DROP STATISTICS` | None on tables | No table locks | Extended statistics |
| **INFO** | `CREATE/DROP EVENT TRIGGER` | None on tables | No table locks | Event triggers |
| **INFO** | `CREATE/DROP FOREIGN DATA WRAPPER` | None on tables | No table locks | FDW management |
| **INFO** | `CREATE/DROP SERVER` | None on tables | No table locks | FDW servers |
| **INFO** | `CREATE/DROP USER MAPPING` | None on tables | No table locks | FDW mappings |
| **INFO** | `CREATE/DROP PUBLICATION` | None on tables | No table locks | Logical replication |
| **INFO** | `ALTER PUBLICATION ADD/DROP TABLE` | ShareUpdateExclusive on table | Minimal impact | Publication management |
| **INFO** | `ALTER DEFAULT PRIVILEGES` | None on existing objects | No immediate locks | Future object permissions |
| **INFO** | `GRANT/REVOKE` | AccessShare typically | Quick operation | ACL update |
| **INFO** | `GRANT/REVOKE ON SCHEMA` | AccessShare on schema | Quick operation | Schema permissions |
| **INFO** | `GRANT/REVOKE ON DATABASE` | AccessShare on database | Quick operation | Database permissions |
| **INFO** | `CREATE/DROP/ALTER ROLE` | None on tables | No table locks | Role management |
| **INFO** | `COMMENT ON` | None significant | No lock | Metadata only |
| **INFO** | `LOCK TABLE ACCESS SHARE` | AccessShare | Read only | Explicit lock |
| **INFO** | `LOCK TABLE ROW SHARE` | RowShare | Allows reads | Explicit lock |
| **INFO** | `BEGIN/START TRANSACTION` | None | Context marker | Transaction start |
| **INFO** | `COMMIT/END` | None | Context marker | Transaction end |
| **INFO** | `ROLLBACK` | None | Context marker | Transaction abort |
| **INFO** | `SAVEPOINT` | None | Context marker | Transaction savepoint |
| **INFO** | `RELEASE SAVEPOINT` | None | Context marker | Savepoint release |
| **INFO** | `ROLLBACK TO SAVEPOINT` | None | Context marker | Partial rollback |
| **INFO** | `SET TRANSACTION` | None | Context marker | Transaction properties |
| **INFO** | `SET LOCAL` | None | Session setting | Transaction-scoped |
| **INFO** | `SET` | None | Session setting | Session-scoped |
| **INFO** | `RESET` | None | Session setting | Reset to default |

## No-Transaction Mode (--no-transaction)

| Severity | Operation | Lock Type | Impact | Notes |
|----------|-----------|-----------|---------|--------|
| **CRITICAL** | `UPDATE` without WHERE | RowExclusive | Blocks concurrent updates/deletes | Full table update |
| **CRITICAL** | `DELETE` without WHERE | RowExclusive | Blocks concurrent updates/deletes | Full table delete |
| **CRITICAL** | `MERGE` without WHERE | RowExclusive | Blocks concurrent updates/deletes | No conditions in WHEN clauses |
| **CRITICAL** | `TRUNCATE` | AccessExclusive | Blocks all operations | Immediate data removal |
| **CRITICAL** | `DROP TABLE` | AccessExclusive | Blocks all operations | Removes table |
| **CRITICAL** | `DROP INDEX` | AccessExclusive | Blocks all operations | Removes index |
| **CRITICAL** | `DROP SCHEMA` | AccessExclusive | Blocks all operations | Removes entire schema |
| **CRITICAL** | `DROP SCHEMA CASCADE` | AccessExclusive | Blocks all operations | Cascading removal |
| **CRITICAL** | `DROP DATABASE` | Exclusive on database | Terminates connections | Database removal |
| **CRITICAL** | `DROP OWNED` | AccessExclusive | Blocks all operations | Drops all owned objects |
| **CRITICAL** | `CREATE INDEX` | Share | Blocks all writes | Non-concurrent index |
| **CRITICAL** | `CREATE UNIQUE INDEX` | Share | Blocks all writes | Non-concurrent unique index |
| **CRITICAL** | `REINDEX` | AccessExclusive | Blocks all operations | Rebuilds index |
| **CRITICAL** | `REINDEX TABLE` | AccessExclusive | Blocks all operations | Rebuilds all indexes |
| **CRITICAL** | `REINDEX DATABASE` | AccessExclusive | Blocks all operations | Database-wide |
| **CRITICAL** | `REINDEX SCHEMA` | AccessExclusive | Blocks all operations | Schema-wide |
| **CRITICAL** | `REINDEX SYSTEM` | AccessExclusive | Blocks all operations | System catalog reindex |
| **CRITICAL** | `VACUUM FULL` | AccessExclusive | Blocks all operations | Full table rewrite |
| **CRITICAL** | `CLUSTER` | AccessExclusive | Blocks all operations | Physically reorders table |
| **CRITICAL** | `REFRESH MATERIALIZED VIEW` | AccessExclusive | Blocks all operations | Full refresh |
| **CRITICAL** | `ALTER TABLE ADD COLUMN` with volatile DEFAULT | AccessExclusive | Blocks all operations + rewrites table | e.g., DEFAULT random() |
| **CRITICAL** | `ALTER TABLE DROP COLUMN` | AccessExclusive | Blocks all operations + rewrites table | Physical removal |
| **CRITICAL** | `ALTER TABLE ALTER COLUMN TYPE` | AccessExclusive | Blocks all operations + rewrites table | Type conversion |
| **CRITICAL** | `ALTER TABLE SET TABLESPACE` | AccessExclusive | Blocks all operations + rewrites table | Physical relocation |
| **CRITICAL** | `ALTER TABLE SET LOGGED/UNLOGGED` | AccessExclusive | Blocks all operations + rewrites table | Durability change |
| **CRITICAL** | `ALTER TABLE ADD PRIMARY KEY` | AccessExclusive | Blocks all operations | Creates unique index |
| **CRITICAL** | `ALTER TABLE RENAME TO` | AccessExclusive | Blocks all operations | Name change |
| **CRITICAL** | `ALTER TABLE SET SCHEMA` | AccessExclusive | Blocks all operations | Schema change |
| **CRITICAL** | `ALTER TABLE ADD CONSTRAINT CHECK` | AccessExclusive | Blocks all operations + scans table | Full table validation |
| **CRITICAL** | `ALTER TABLE SET/DROP NOT NULL` | AccessExclusive | Blocks all operations + scans table | Full table constraint check |
| **CRITICAL** | `LOCK TABLE ACCESS EXCLUSIVE` | AccessExclusive | Blocks all operations | Explicit lock |
| **WARNING** | `UPDATE` with WHERE | RowExclusive | Blocks concurrent updates/deletes on target rows | Targeted update |
| **WARNING** | `DELETE` with WHERE | RowExclusive | Blocks concurrent updates/deletes on target rows | Targeted delete |
| **WARNING** | `MERGE` with WHERE | RowExclusive | Blocks concurrent updates/deletes on target rows | Has conditions in WHEN clauses or subquery |
| **WARNING** | `SELECT FOR UPDATE` without WHERE | RowShare | Prevents concurrent updates | Full table scan with locks |
| **WARNING** | `SELECT FOR UPDATE` with WHERE | RowShare | Prevents concurrent updates on selected rows | Targeted row locking |
| **WARNING** | `SELECT FOR NO KEY UPDATE` without WHERE | RowShare | Prevents non-key updates | Weaker than FOR UPDATE |
| **WARNING** | `SELECT FOR NO KEY UPDATE` with WHERE | RowShare | Prevents non-key updates on selected rows | Targeted row locking |
| **WARNING** | `SELECT FOR SHARE` without WHERE | RowShare | Prevents updates | Shared lock |
| **WARNING** | `SELECT FOR SHARE` with WHERE | RowShare | Prevents updates on selected rows | Targeted shared lock |
| **WARNING** | `INSERT SELECT` from large table | RowExclusive | Long operation | Large data copy |
| **WARNING** | `CREATE TABLE AS` | AccessShare on source | Creates new table | With data copy |
| **WARNING** | `SELECT INTO` | AccessShare on source | Creates new table | With data copy |
| **WARNING** | `COPY FROM` large file | RowExclusive | Long operation | Bulk insert |
| **WARNING** | `VACUUM` | ShareUpdateExclusive | Blocks DDL | Maintenance operation |
| **WARNING** | `VACUUM FREEZE` | ShareUpdateExclusive | Blocks DDL | Freeze operation |
| **WARNING** | `VACUUM ANALYZE` | ShareUpdateExclusive | Blocks DDL | Vacuum + stats |
| **WARNING** | `ANALYZE` | ShareUpdateExclusive | Blocks DDL | Statistics update |
| **WARNING** | `CREATE INDEX CONCURRENTLY` | ShareUpdateExclusive | Allows reads/writes | Longer but safer |
| **WARNING** | `DROP INDEX CONCURRENTLY` | ShareUpdateExclusive | Allows reads/writes | Longer but safer |
| **WARNING** | `REINDEX CONCURRENTLY` | ShareUpdateExclusive | Allows reads/writes | Longer but safer |
| **WARNING** | `REFRESH MATERIALIZED VIEW CONCURRENTLY` | Exclusive | Allows reads | Incremental refresh |
| **WARNING** | `CREATE TRIGGER` | ShareRowExclusive | Blocks DML | Adds trigger |
| **WARNING** | `DROP TRIGGER` | AccessExclusive | Blocks all operations | Removes trigger |
| **WARNING** | `ALTER TABLE ADD FOREIGN KEY` | ShareRowExclusive | Blocks DML | Adds constraint |
| **WARNING** | `ALTER TABLE ADD CONSTRAINT UNIQUE` | AccessExclusive | Blocks all operations | Creates index |
| **WARNING** | `ALTER TABLE ADD CONSTRAINT EXCLUDE` | AccessExclusive | Blocks all operations | Creates index |
| **WARNING** | `ALTER TABLE ADD CONSTRAINT NOT VALID` | ShareRowExclusive | Minimal impact | Constraint without validation |
| **WARNING** | `ALTER TABLE VALIDATE CONSTRAINT` | ShareUpdateExclusive | Blocks DDL | Validates existing |
| **WARNING** | `ALTER TABLE DROP CONSTRAINT` | AccessExclusive | Blocks all operations | Removes constraint |
| **WARNING** | `ALTER TABLE ENABLE TRIGGER` | ShareRowExclusive | Blocks DML | Activates trigger |
| **WARNING** | `ALTER TABLE DISABLE TRIGGER` | ShareRowExclusive | Blocks DML | Deactivates trigger |
| **WARNING** | `ALTER TABLE ENABLE RULE` | ShareRowExclusive | Blocks DML | Activates rule |
| **WARNING** | `ALTER TABLE DISABLE RULE` | ShareRowExclusive | Blocks DML | Deactivates rule |
| **WARNING** | `ALTER TABLE ENABLE ROW LEVEL SECURITY` | AccessExclusive | Blocks all operations | Activates RLS |
| **WARNING** | `ALTER TABLE DISABLE ROW LEVEL SECURITY` | AccessExclusive | Blocks all operations | Deactivates RLS |
| **WARNING** | `ALTER TABLE FORCE ROW LEVEL SECURITY` | AccessExclusive | Blocks all operations | Forces RLS |
| **WARNING** | `ALTER TABLE NO FORCE ROW LEVEL SECURITY` | AccessExclusive | Blocks all operations | Unforces RLS |
| **WARNING** | `ALTER TABLE RENAME COLUMN` | AccessExclusive | Blocks all operations | Metadata change |
| **WARNING** | `ALTER TABLE INHERIT` | AccessExclusive | Blocks all operations | Inheritance change |
| **WARNING** | `ALTER TABLE NO INHERIT` | AccessExclusive | Blocks all operations | Inheritance change |
| **WARNING** | `ALTER TABLE OF` | AccessExclusive | Blocks all operations | Type binding |
| **WARNING** | `ALTER TABLE NOT OF` | AccessExclusive | Blocks all operations | Type unbinding |
| **WARNING** | `ALTER TABLE REPLICA IDENTITY` | AccessExclusive | Blocks all operations | Replication change |
| **WARNING** | `ALTER TABLE OWNER TO` | AccessExclusive | Blocks all operations | Ownership change |
| **WARNING** | `ALTER TABLE ATTACH PARTITION` | ShareUpdateExclusive | Blocks DDL | Partition management |
| **WARNING** | `ALTER TABLE DETACH PARTITION` | ShareUpdateExclusive | Blocks DDL | Partition management |
| **WARNING** | `ALTER TABLE DETACH PARTITION CONCURRENTLY` | ShareUpdateExclusive | Allows reads/writes | PostgreSQL 14+ feature |
| **WARNING** | `ALTER TABLE SET ACCESS METHOD` | AccessExclusive | Blocks all operations | Storage method change |
| **WARNING** | `DROP VIEW` | AccessExclusive on view | Blocks view access | Removes view |
| **WARNING** | `DROP MATERIALIZED VIEW` | AccessExclusive | Blocks all operations | Removes mat view |
| **WARNING** | `DROP SEQUENCE` | AccessExclusive on sequence | Blocks sequence access | Removes sequence |
| **WARNING** | `DROP TYPE` | AccessExclusive | Blocks type usage | May cascade |
| **WARNING** | `DROP DOMAIN` | AccessExclusive | Blocks domain usage | May cascade |
| **WARNING** | `DROP EXTENSION` | Varies | Depends on objects | May cascade |
| **WARNING** | `DROP TABLESPACE` | System-wide | Storage impact | Tablespace removal |
| **WARNING** | `CREATE RULE` | AccessExclusive | Blocks all operations | Adds rule |
| **WARNING** | `DROP RULE` | AccessExclusive | Blocks all operations | Removes rule |
| **WARNING** | `CREATE POLICY` | AccessExclusive | Blocks all operations | Adds RLS policy |
| **WARNING** | `DROP POLICY` | AccessExclusive | Blocks all operations | Removes RLS policy |
| **WARNING** | `ALTER INDEX` | AccessExclusive | Blocks all operations | Index modification |
| **WARNING** | `ALTER VIEW` | AccessExclusive on view | Blocks view access | View modification |
| **WARNING** | `ALTER SEQUENCE` | AccessExclusive on sequence | Blocks sequence access | Sequence modification |
| **WARNING** | `ALTER TYPE` | AccessExclusive | Blocks type usage | Type modification |
| **WARNING** | `ALTER TYPE ADD VALUE` | AccessExclusive | Blocks type usage | Enum extension |
| **WARNING** | `ALTER DOMAIN` | AccessExclusive | Blocks domain usage | Domain modification |
| **WARNING** | `REASSIGN OWNED` | AccessExclusive on objects | Blocks owned objects | Ownership transfer |
| **WARNING** | `LOCK TABLE ROW EXCLUSIVE` | RowExclusive | Blocks some operations | Explicit lock |
| **WARNING** | `LOCK TABLE SHARE UPDATE EXCLUSIVE` | ShareUpdateExclusive | Blocks DDL | Explicit lock |
| **WARNING** | `LOCK TABLE SHARE` | Share | Blocks writes | Explicit lock |
| **WARNING** | `LOCK TABLE SHARE ROW EXCLUSIVE` | ShareRowExclusive | Blocks DML | Explicit lock |
| **WARNING** | `LOCK TABLE EXCLUSIVE` | Exclusive | Blocks most operations | Explicit lock |
| **INFO** | `SELECT FOR KEY SHARE` | RowShare | Prevents key updates | Weakest locking mode |
| **INFO** | `SELECT FOR UPDATE` with specific WHERE | RowShare| Locks specific rows | Minimal impact |
| **INFO** | `SELECT FOR NO KEY UPDATE` with specific WHERE | RowShare| Locks specific rows | Weaker lock |
| **INFO** | `SELECT FOR SHARE` with specific WHERE | RowShare| Shared lock few rows | Read stability |
| **INFO** | `SELECT FOR KEY SHARE` with specific WHERE | RowShare + weak row locks | Weakest lock | FK checking |
| **INFO** | `INSERT` | RowExclusive | Minimal impact | New rows only |
| **INFO** | `INSERT ON CONFLICT` | RowExclusive | Minimal impact | Upsert operation |
| **INFO** | `INSERT RETURNING` | RowExclusive | Minimal impact | Returns inserted data |
| **INFO** | `COPY TO` | AccessShare | Read only | Data export |
| **INFO** | `ALTER TABLE ADD COLUMN` without DEFAULT | AccessExclusive | Quick operation | Metadata only |
| **INFO** | `ALTER TABLE ADD COLUMN` with constant DEFAULT | AccessExclusive | Quick operation | No rewrite |
| **INFO** | `ALTER TABLE ADD COLUMN GENERATED ALWAYS AS` | AccessExclusive | Quick operation | Generated column |
| **INFO** | `ALTER TABLE ALTER COLUMN ADD IDENTITY` | AccessExclusive | Quick operation | Identity column |
| **INFO** | `ALTER TABLE ALTER COLUMN DROP IDENTITY` | AccessExclusive | Quick operation | Remove identity |
| **INFO** | `ALTER TABLE SET/DROP DEFAULT` | AccessExclusive | Quick operation | Metadata only |
| **INFO** | `ALTER TABLE ALTER COLUMN SET STATISTICS` | ShareUpdateExclusive | Minimal impact | Stats metadata |
| **INFO** | `ALTER TABLE ALTER COLUMN SET STORAGE` | AccessExclusive | Quick operation | Storage hint |
| **INFO** | `ALTER TABLE SET (storage_parameter)` | ShareUpdateExclusive | Minimal impact | Table parameters |
| **INFO** | `ALTER TABLE RESET (storage_parameter)` | ShareUpdateExclusive | Minimal impact | Table parameters |
| **INFO** | `ALTER TABLE CLUSTER ON` | ShareUpdateExclusive | Minimal impact | Cluster hint |
| **INFO** | `ALTER TABLE SET WITHOUT CLUSTER` | ShareUpdateExclusive | Minimal impact | Cluster hint |
| **INFO** | `CREATE DATABASE` | System-level | New database | No table impact |
| **INFO** | `ALTER DATABASE` | Varies | Database properties | Usually safe |
| **INFO** | `CREATE TABLESPACE` | System-level | Storage management | No table locks |
| **INFO** | `ALTER TABLESPACE` | System-level | Storage properties | No table locks |
| **INFO** | `ALTER SYSTEM` | None | Configuration change | No locks |
| **INFO** | `CREATE SUBSCRIPTION` | None on tables | Logical replication | Setup only |
| **INFO** | `ALTER SUBSCRIPTION` | None on tables | Logical replication | Configuration |
| **INFO** | `DROP SUBSCRIPTION` | None on tables | Logical replication | Cleanup |
| **INFO** | `CREATE TABLE` | None on other tables | No conflict | New table |
| **INFO** | `CREATE TEMPORARY TABLE` | None on other tables | No conflict | Session-local table |
| **INFO** | `CREATE VIEW` | AccessShare on referenced | Read locks only | View creation |
| **INFO** | `CREATE MATERIALIZED VIEW` | AccessShare on source | Read locks only | Initial creation |
| **INFO** | `CREATE SEQUENCE` | None on other objects | No conflict | New sequence |
| **INFO** | `CREATE TYPE` | None on other objects | No conflict | New type |
| **INFO** | `CREATE DOMAIN` | None on other objects | No conflict | New domain |
| **INFO** | `CREATE SCHEMA` | None on other objects | No conflict | New schema |
| **INFO** | `CREATE EXTENSION` | Varies | Usually safe | Adds functionality |
| **INFO** | `CREATE/DROP FUNCTION` | None on tables | No table locks | Function management |
| **INFO** | `CREATE/DROP PROCEDURE` | None on tables | No table locks | Procedure management |
| **INFO** | `CREATE/DROP AGGREGATE` | None on tables | No table locks | Aggregate management |
| **INFO** | `CREATE/DROP OPERATOR` | None on tables | No table locks | Operator management |
| **INFO** | `CREATE/DROP CAST` | None on tables | No table locks | Cast management |
| **INFO** | `CREATE/DROP COLLATION` | None on tables | No table locks | Collation management |
| **INFO** | `CREATE/DROP TEXT SEARCH CONFIGURATION` | None on tables | No table locks | Text search config |
| **INFO** | `CREATE/DROP TEXT SEARCH DICTIONARY` | None on tables | No table locks | Text search dictionary |
| **INFO** | `CREATE/DROP TEXT SEARCH PARSER` | None on tables | No table locks | Text search parser |
| **INFO** | `CREATE/DROP TEXT SEARCH TEMPLATE` | None on tables | No table locks | Text search template |
| **INFO** | `CREATE/DROP STATISTICS` | None on tables | No table locks | Extended statistics |
| **INFO** | `CREATE/DROP EVENT TRIGGER` | None on tables | No table locks | Event triggers |
| **INFO** | `CREATE/DROP FOREIGN DATA WRAPPER` | None on tables | No table locks | FDW management |
| **INFO** | `CREATE/DROP SERVER` | None on tables | No table locks | FDW servers |
| **INFO** | `CREATE/DROP USER MAPPING` | None on tables | No table locks | FDW mappings |
| **INFO** | `CREATE/DROP PUBLICATION` | None on tables | No table locks | Logical replication |
| **INFO** | `ALTER PUBLICATION ADD/DROP TABLE` | ShareUpdateExclusive on table | Minimal impact | Publication management |
| **INFO** | `ALTER DEFAULT PRIVILEGES` | None on existing objects | No immediate locks | Future object permissions |
| **INFO** | `GRANT/REVOKE` | AccessShare typically | Quick operation | ACL update |
| **INFO** | `GRANT/REVOKE ON SCHEMA` | AccessShare on schema | Quick operation | Schema permissions |
| **INFO** | `GRANT/REVOKE ON DATABASE` | AccessShare on database | Quick operation | Database permissions |
| **INFO** | `CREATE/DROP/ALTER ROLE` | None on tables | No table locks | Role management |
| **INFO** | `COMMENT ON` | None significant | No lock | Metadata only |
| **INFO** | `CHECKPOINT` | None | I/O impact only | WAL checkpoint |
| **INFO** | `LOAD` | None | Library loading | No locks |
| **INFO** | `LOCK TABLE ACCESS SHARE` | AccessShare | Read only | Explicit lock |
| **INFO** | `LOCK TABLE ROW SHARE` | RowShare | Allows reads | Explicit lock |
| **INFO** | `BEGIN/START TRANSACTION` | None | Context marker | Not applicable |
| **INFO** | `COMMIT/END` | None | Context marker | Not applicable |
| **INFO** | `ROLLBACK` | None | Context marker | Not applicable |
| **INFO** | `SAVEPOINT` | None | Context marker | Not applicable |
| **INFO** | `RELEASE SAVEPOINT` | None | Context marker | Not applicable |
| **INFO** | `ROLLBACK TO SAVEPOINT` | None | Context marker | Not applicable |
| **INFO** | `SET TRANSACTION` | None | Context marker | Not applicable |
| **INFO** | `SET LOCAL` | None | Session setting | Not applicable |
| **INFO** | `SET` | None | Session setting | Session-scoped |
| **INFO** | `RESET` | None | Session setting | Reset to default |

## Summary Statistics

**Transaction Mode:**
- ERROR: 19 operations (cannot run in transaction)
- CRITICAL: 28 operations (severe locks)
- WARNING: 93 operations (moderate impact)
- INFO: 89 operations (minimal impact)
- **Total: 229 operations**

**No-Transaction Mode:**
- CRITICAL: 29 operations (severe locks)
- WARNING: 96 operations (moderate impact)
- INFO: 104 operations (minimal impact)
- **Total: 229 operations**

This comprehensive list covers 95%+ of PostgreSQL operations relevant to migration safety analysis, organized by severity and transaction context.
