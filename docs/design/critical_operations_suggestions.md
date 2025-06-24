# CRITICAL Operations - Suggestion Mapping

Based on `lock_severity.md`, this document maps all CRITICAL severity operations to their safe migration suggestions.

## Prerequisites

- PostgreSQL 13+ (minimum supported version)
- All CONCURRENTLY operations must run outside transactions
- Set `lock_timeout` before DDL operations: `SET lock_timeout = '5s';`

## CRITICAL Operations in Transaction Mode

| Operation | Safe Alternative |
|-----------|------------------|
| ✅ `UPDATE` without WHERE | 1) Export target row IDs to file, 2) Process file in batches with progress tracking, 3) Update by explicit ID lists |
| ✅ `DELETE` without WHERE | 1) Export target row IDs to file, 2) Process file in batches, 3) DELETE by explicit ID lists with monitoring |
| ✅ `MERGE` without WHERE | Add conditions to WHEN clauses or batch with subqueries |
| ❌ `TRUNCATE` | |
| ❌ `DROP TABLE` | |
| ✅ `DROP INDEX` | Use `DROP INDEX CONCURRENTLY` outside transaction |
| ❌ `DROP SCHEMA` | |
| ❌ `DROP SCHEMA CASCADE` | |
| ❌ `DROP OWNED` | |
| ✅ `CREATE INDEX` | Use `CREATE INDEX CONCURRENTLY` outside transaction |
| ✅ `CREATE UNIQUE INDEX` | Use `CREATE UNIQUE INDEX CONCURRENTLY` outside transaction |
| ✅ `REINDEX` | Use `REINDEX CONCURRENTLY` or CREATE new index + DROP old pattern |
| ✅ `REINDEX TABLE` | Script to list all indexes on table, then REINDEX CONCURRENTLY each one |
| ✅ `REINDEX DATABASE` | Script to reindex each index individually with CONCURRENTLY |
| ✅ `REINDEX SCHEMA` | Script to reindex each index individually with CONCURRENTLY |
| ❌ `REINDEX SYSTEM` | |
| ⚠️ `CLUSTER` | Consider `pg_repack` extension for online reorganization |
| ✅ `REFRESH MATERIALIZED VIEW` | Use `REFRESH MATERIALIZED VIEW CONCURRENTLY` (requires unique index) |
| ✅ `ALTER TABLE ADD COLUMN` with volatile DEFAULT | Split: 1) `ADD COLUMN` without default, 2) Batch update script for default values, 3) `ALTER COLUMN SET DEFAULT` |
| ❌ `ALTER TABLE DROP COLUMN` | |
| ✅ `ALTER TABLE ALTER COLUMN TYPE` | Blue-green: 1) Add new column, 2) Add sync trigger, 3) Backfill script, 4) Atomic swap |
| ❌ `ALTER TABLE SET TABLESPACE` | |
| ❌ `ALTER TABLE SET LOGGED/UNLOGGED` | |
| ✅ `ALTER TABLE ADD PRIMARY KEY` | First `CREATE UNIQUE INDEX CONCURRENTLY`, then `ALTER TABLE ADD CONSTRAINT pkey PRIMARY KEY USING INDEX` |
| ❌ `ALTER TABLE RENAME TO` | |
| ❌ `ALTER TABLE SET SCHEMA` | |
| ✅ `ALTER TABLE ADD CONSTRAINT CHECK` | Use `ADD CONSTRAINT NOT VALID` then `VALIDATE CONSTRAINT` |
| ✅ `ALTER TABLE SET NOT NULL` | `ADD CONSTRAINT CHECK (col IS NOT NULL) NOT VALID` → `VALIDATE CONSTRAINT` → `SET NOT NULL` → Drop constraint |
| ✅ `ALTER TABLE DROP NOT NULL` | Direct operation is safe: `ALTER TABLE ... ALTER COLUMN ... DROP NOT NULL` |
| ❌ `LOCK TABLE ACCESS EXCLUSIVE` | |

## Additional CRITICAL Operations (No-Transaction Mode Only)

| Operation | Safe Alternative |
|-----------|------------------|
| ❌ `DROP DATABASE` | No safe alternative - requires exclusive database access |
| ❌ `VACUUM FULL` | Use regular `VACUUM` or `pg_repack` extension instead |

## Summary

- **Total CRITICAL operations**: 31 (29 in transaction mode, 2 additional in no-transaction mode)
- ✅ **With safe alternatives**: 18 (58%)
- ❌ **No safe alternatives**: 12 (39%)
- ⚠️ **Partial alternatives**: 1 (3%)

## Implementation Priority

### High Priority (Common & High Impact)
1. ✅ `UPDATE` without WHERE
2. ✅ `DELETE` without WHERE
3. ✅ `CREATE INDEX` / `CREATE UNIQUE INDEX`
4. ✅ `ALTER TABLE ADD COLUMN` with volatile DEFAULT
5. ✅ `ALTER TABLE ALTER COLUMN TYPE`

### Medium Priority (Less Common)
6. ✅ `REINDEX` variants
7. ✅ `REFRESH MATERIALIZED VIEW`
8. ✅ `ALTER TABLE ADD CONSTRAINT CHECK`
9. ✅ `ALTER TABLE SET NOT NULL`
10. ✅ `ALTER TABLE ADD PRIMARY KEY`

### Low Priority (Rare or Complex)
11. ✅ `MERGE` without WHERE
12. ✅ `ALTER TABLE DROP NOT NULL`
13. ✅ `DROP INDEX`

## Important Notes

### General Guidelines
- All CONCURRENTLY operations **must** run outside transactions (will ERROR in transaction mode)
- Always set `lock_timeout` for DDL operations: `SET lock_timeout = '5s';`
- Monitor locks during migration: `SELECT * FROM pg_stat_activity WHERE wait_event_type = 'Lock';`
- For batching operations:
  - **Common pattern**: `SELECT id FROM table WHERE condition \COPY TO '/tmp/ids.csv'` → Process file in script
  - Default batch size: 1000-5000 rows (use `WHERE id IN (...)` with explicit lists)
  - Add sleep delays (100-500ms) between batches
  - Track progress: Line number in file or separate progress table
  - Monitor: Replication lag, lock waits, transaction duration
  - Resume capability: Essential for large tables (millions of rows)

### Version Features Available in PostgreSQL 13+
- CREATE/DROP INDEX CONCURRENTLY
- REFRESH MATERIALIZED VIEW CONCURRENTLY
- REINDEX CONCURRENTLY
- ALTER TABLE ... ADD CONSTRAINT ... NOT VALID
- Generated columns
- Partitioned table improvements

### Risk Assessment
| Pattern | Risk Level | Performance Impact | Best For |
|---------|------------|-------------------|----------|
| Batched DML | Low | 2-5x slower | Tables > 1M rows |
| CONCURRENTLY operations | Low | 2-3x slower | Production systems |
| Blue-green migrations | Low | Requires 2x storage | Complex type changes |
| NOT VALID + VALIDATE | Low | Minimal | Large tables |
| pg_repack | Medium | CPU intensive | Bloated tables |

### Connection Pooler Considerations
- PgBouncer transaction mode: CONCURRENTLY operations need direct database connections
- Consider connection limits when batching
- May need to disable prepared statements for some DDL operations

### Real-World Batch Processing Example
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