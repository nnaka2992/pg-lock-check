---
title: CRITICAL Operations - Safe Migration Patterns
version: ${VERSION}
generated_from: internal/suggester/suggestions.yaml
generated_at: ${GENERATED_AT}
---
# CRITICAL Operations - Safe Migration Patterns

Based on lock_severity.md, this document maps all CRITICAL severity operations to their safe migration suggestions.

## Available Suggestions Summary

### Operations with Safe Alternatives

| Operation | Category | Migration Steps | Transaction Safe |
|-----------|----------|-------|------------------|
${OPERATIONS_WITH_ALTERNATIVES_TABLE}

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

- **Total CRITICAL operations**: ${TOTAL_OPERATIONS}
- **Operations with safe alternatives**: ${WITH_ALTERNATIVES} (${WITH_ALTERNATIVES_PERCENT}%)
- **Operations without safe alternatives**: ${WITHOUT_ALTERNATIVES} (${WITHOUT_ALTERNATIVES_PERCENT}%)

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
