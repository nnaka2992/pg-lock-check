---
title: CRITICAL Operations - Safe Migration Patterns
version: 1.0
generated_from: internal/suggester/suggestions.yaml
generated_at: 2025-06-26
---
# CRITICAL Operations - Safe Migration Patterns

Based on lock_severity.md, this document maps all CRITICAL severity operations to their safe migration suggestions.

## Available Suggestions Summary

### Operations with Safe Alternatives

| Operation | Category | Migration Steps | Transaction Safe |
|-----------|----------|-------|------------------|
| UPDATE without WHERE | DML Operations | Export target row IDs to file;Process file in batches with progress tracking; | ⚠️ Mixed |
| DELETE without WHERE | DML Operations | Export target row IDs to file;Process file in batches; | ⚠️ Mixed |
| MERGE without WHERE | DML Operations | Export source data IDs to file;Process MERGE in batches; | ⚠️ Mixed |
| DROP INDEX | Index Operations | Use `DROP INDEX CONCURRENTLY` outside transaction; | ❌ No |
| CREATE INDEX | Index Operations | Use `CREATE INDEX CONCURRENTLY` outside transaction; | ❌ No |
| CREATE UNIQUE INDEX | Index Operations | Use `CREATE UNIQUE INDEX CONCURRENTLY` outside transaction; | ❌ No |
| REINDEX | Index Operations | Use `REINDEX CONCURRENTLY` or CREATE new index + DROP old pattern; | ❌ No |
| REINDEX TABLE | Index Operations | Export all index names for the table;Reindex each index individually; | ⚠️ Mixed |
| REINDEX DATABASE | Index Operations | Export all index names in the database;Reindex each index individually; | ⚠️ Mixed |
| REINDEX SCHEMA | Index Operations | Export all index names in the schema;Reindex each index individually; | ⚠️ Mixed |
| ALTER TABLE ADD COLUMN with volatile DEFAULT | ALTER TABLE Operations | `ADD COLUMN` without default;Batch update with default values (separate transactions per batch);`ALTER COLUMN SET DEFAULT`; | ⚠️ Mixed |
| ALTER TABLE ALTER COLUMN TYPE | ALTER TABLE Operations | Add new column;Add sync trigger;Backfill script;Atomic swap; | ⚠️ Mixed |
| ALTER TABLE ADD PRIMARY KEY | ALTER TABLE Operations | First `CREATE UNIQUE INDEX CONCURRENTLY`;Then `ALTER TABLE ADD CONSTRAINT pkey PRIMARY KEY USING INDEX`; | ⚠️ Mixed |
| ALTER TABLE ADD CONSTRAINT CHECK | ALTER TABLE Operations | Use `ADD CONSTRAINT NOT VALID`;Then `VALIDATE CONSTRAINT`; | ✅ Yes |
| ALTER TABLE SET NOT NULL | ALTER TABLE Operations | `ADD CONSTRAINT CHECK (col IS NOT NULL) NOT VALID`;`VALIDATE CONSTRAINT`;`SET NOT NULL`;Drop constraint; | ✅ Yes |
| CLUSTER | Maintenance Operations | Consider `pg_repack` extension for online reorganization; | ❌ No |
| REFRESH MATERIALIZED VIEW | Maintenance Operations | Use `REFRESH MATERIALIZED VIEW CONCURRENTLY` (requires unique index); | ❌ No |
| VACUUM FULL | Maintenance Operations | Use `pg_repack` extension instead; | ❌ No |

### Operations Without Safe Alternatives

The following CRITICAL operations do not have safe alternatives:

| Operation | Category | Reason |
|-----------|----------|--------|
| TRUNCATE | DML Operations | Requires AccessExclusive lock, no workaround |
| DROP TABLE | DDL Operations | Destructive operation |
| DROP SCHEMA | DDL Operations | Destructive operation |
| DROP SCHEMA CASCADE | DDL Operations | Destructive operation |
| DROP OWNED | DDL Operations | Destructive operation |
| REINDEX SYSTEM | Index Operations | System catalogs require exclusive access |
| ALTER TABLE DROP COLUMN | ALTER TABLE Operations | Requires table rewrite |
| ALTER TABLE SET TABLESPACE | ALTER TABLE Operations | Requires full table lock |
| ALTER TABLE SET LOGGED/UNLOGGED | ALTER TABLE Operations | Requires table rewrite |
| ALTER TABLE RENAME TO | ALTER TABLE Operations | Requires exclusive lock |
| ALTER TABLE SET SCHEMA | ALTER TABLE Operations | Requires exclusive lock |
| LOCK TABLE ACCESS EXCLUSIVE | Explicit Locking | Explicitly requests exclusive lock |
| DROP DATABASE | DDL Operations | Requires exclusive database access |

## Summary Statistics

- **Total CRITICAL operations**: 31
- **Operations with safe alternatives**: 18 (58%)
- **Operations without safe alternatives**: 13 (41%)

## Prerequisites

- PostgreSQL 13+ (minimum supported version)
- All CONCURRENTLY operations must run outside transactions
- Set `lock_timeout` before DDL operations: `SET lock_timeout = '5s';`

## Guidelines

### General
- All CONCURRENTLY operations **must** run outside transactions (will ERROR in transaction mode)
- Always set `lock_timeout` for DDL operations: `SET lock_timeout = '5s';`
- Monitor locks during migration: `SELECT * FROM pg_stat_activity WHERE wait_event_type = 'Lock';`

### Batching
- **Common pattern**: `SELECT id FROM table WHERE condition \COPY TO '/tmp/ids.csv'` → Process file in script
- Default batch size: 1000-5000 rows (use `WHERE id IN (...)` with explicit lists)
- Add sleep delays (100-500ms) between batches
- Track progress: Line number in file or separate progress table
- Monitor: Replication lag, lock waits, transaction duration
- Resume capability: Essential for large tables (millions of rows)

## Version Features Available in PostgreSQL 13+

- CREATE/DROP INDEX CONCURRENTLY
- REFRESH MATERIALIZED VIEW CONCURRENTLY
- REINDEX CONCURRENTLY
- ALTER TABLE ... ADD CONSTRAINT ... NOT VALID
- Generated columns
- Partitioned table improvements

## Risk Assessment

| Pattern | Risk Level | Performance Impact | Best For |
|---------|------------|-------------------|----------|
| Batched DML | Low | 2-5x slower | Tables > 1M rows |
| CONCURRENTLY operations | Low | 2-3x slower | Production systems |
| Blue-green migrations | Low | Requires 2x storage | Complex type changes |
| NOT VALID + VALIDATE | Low | Minimal | Large tables |
| pg_repack | Medium | CPU intensive | Bloated tables |

## Real-World Batch Processing Example

```bash
# 1. Export IDs
psql -h db -U user -d mydb -c "\COPY (SELECT id FROM users WHERE active = false ORDER BY id) TO '/tmp/inactive_users.csv' CSV"

# 2. Split into smaller files (optional for very large datasets)
split -l 5000 /tmp/inactive_users.csv /tmp/batch_

# 3. Process with script (Python example pattern)
# - Read file/chunk
# - Build explicit ID lists: UPDATE users SET deleted_at = NOW() WHERE id IN (1,2,3,4,5...)
# - Commit every batch
# - Log progress to file/table
# - Sleep between batches
# - Handle connection failures with retry
```
