# CLI Requirements for pg-lock-check

## Command Structure
```bash
pg-lock-check [OPTIONS] [SQL_STATEMENT]
```

## Input Methods (Priority Order)
1. **File input**: Use `-f` or `--file` flag (highest priority)
2. **Direct SQL input**: Pass SQL statement as argument
3. **Stdin**: Read from stdin when piped data is detected

## Options/Flags

### Input:
- `SQL_STATEMENT` - Direct SQL input as argument
- `-f, --file FILE` - Read SQL from file (takes precedence over other inputs)

### Transaction Mode:
- `--no-transaction` - Analyze assuming no transaction wrapper
- Default behavior: Analyze assuming wrapped in transaction

### Suggestion Control:
- `--no-suggestion` - Disable safe migration suggestions for CRITICAL operations
- Default behavior: Show suggestions for CRITICAL operations

### Output Control:
- `-o, --output FORMAT` - Output format: `text` (default), `json`, `yaml`
- `--no-color` - Disable colored output
- `-q, --quiet` - Quiet mode (flag exists but implementation limited)
- `--verbose` - Verbose output (flag exists but implementation limited)

### Help/Version:
- `-h, --help` - Show help message
- `-v, --version` - Show version information

## Output Format

### Default (text):
```
[CRITICAL] UPDATE users SET status = 'active'
Suggestion for safe migration:
  Step: Add a WHERE clause to target specific rows
    Can run in transaction: Yes
    SQL:
      UPDATE users SET status = 'active' WHERE id IN (
        SELECT id FROM users 
        WHERE <condition> 
        LIMIT 1000
      );

  Step: Repeat the batched UPDATE until all rows are processed
    Can run in transaction: No
    Instructions:
      Monitor pg_stat_activity between batches.
      Add delays if needed to reduce lock contention.

Summary: 2 statements analyzed
```

### JSON format:
```json
{
  "summary": {
    "total_statements": 2,
    "by_severity": {
      "ERROR": 0,
      "CRITICAL": 1,
      "WARNING": 0,
      "INFO": 1
    }
  },
  "results": [
    {
      "index": 0,
      "sql": "UPDATE users SET status = 'active'",
      "line_number": 1,
      "severity": "CRITICAL",
      "operation": "UPDATE without WHERE",
      "lock_type": "RowExclusive",
      "tables": [
        {
          "name": "users",
          "lock_type": "RowExclusive"
        }
      ],
      "suggestion": {
        "steps": [
          {
            "description": "Add a WHERE clause to target specific rows",
            "can_run_in_transaction": true,
            "output": "UPDATE users SET status = 'active' WHERE id IN (\n  SELECT id FROM users \n  WHERE <condition> \n  LIMIT 1000\n);"
          },
          {
            "description": "Repeat the batched UPDATE until all rows are processed",
            "can_run_in_transaction": false,
            "output": "Monitor pg_stat_activity between batches.\nAdd delays if needed to reduce lock contention."
          }
        ]
      }
    }
  ]
}
```

### YAML format:
```yaml
summary:
  total_statements: 2
  by_severity:
    ERROR: 0
    CRITICAL: 1
    WARNING: 0
    INFO: 1
results:
  - index: 0
    sql: "UPDATE users SET status = 'active'"
    line_number: 1
    severity: CRITICAL
    operation: "UPDATE without WHERE"
    lock_type: RowExclusive
    tables:
      - name: users
        lock_type: RowExclusive
    suggestion:
      steps:
        - description: "Add a WHERE clause to target specific rows"
          can_run_in_transaction: true
          output: |
            UPDATE users SET status = 'active' WHERE id IN (
              SELECT id FROM users 
              WHERE <condition> 
              LIMIT 1000
            );
        - description: "Repeat the batched UPDATE until all rows are processed"
          can_run_in_transaction: false
          output: |
            Monitor pg_stat_activity between batches.
            Add delays if needed to reduce lock contention.
```

## Exit Codes
- `0` - Success - Analysis completed
- `1` - Runtime error - File not found, read errors, flag parsing errors, no SQL provided
- `2` - Parse error - Invalid SQL syntax

## Examples

```bash
# Analyze direct SQL
pg-lock-check "ALTER TABLE users ADD COLUMN age INT DEFAULT 0"

# Analyze file
pg-lock-check -f migration.sql

# Pipe from stdin
echo "TRUNCATE users;" | pg-lock-check

# Non-transaction mode
pg-lock-check --no-transaction -f concurrent_index.sql

# JSON output
pg-lock-check -f migration.sql -o json

# YAML output
pg-lock-check -o yaml "DROP TABLE users"

# Disable suggestions
pg-lock-check --no-suggestion "UPDATE users SET deleted = true"

# Non-transaction mode with JSON output
pg-lock-check --no-transaction -o json "VACUUM FULL users"
```

## Key Features

### Safe Migration Suggestions
- **Enabled by default** for CRITICAL severity operations
- Provides step-by-step migration patterns to avoid long locks
- Shows whether each step can run in a transaction
- Can be disabled with `--no-suggestion` flag
- Covers 18 CRITICAL operations with safe alternatives (100% coverage)

### Transaction Mode Analysis
- **Default**: Assumes SQL runs inside a transaction
- `--no-transaction`: Analyzes as if SQL runs outside a transaction
- Affects severity levels (e.g., VACUUM is ERROR in transaction, WARNING outside)
- Some operations like CREATE INDEX CONCURRENTLY require no-transaction mode

### Multi-Statement Support
- Handles multiple SQL statements in a single input
- Tracks line numbers for each statement
- Analyzes each statement independently
- Provides aggregated summary

### Table Lock Tracking
- Extracts affected tables from each statement
- Shows lock type for each table
- Handles complex queries with joins, CTEs, and subqueries

## Implementation Details

### Core Components Integration
- **Parser** (`internal/parser/`): Wraps pg_query_go for SQL parsing
- **Analyzer** (`internal/analyzer/`): Analyzes lock severity based on operation
- **Suggester** (`internal/suggester/`): Generates safe migration suggestions
- **Metadata** (`internal/metadata/`): Extracts SQL metadata for suggestions

### Severity Levels
- **ERROR**: Operations that cannot run in the specified mode
- **CRITICAL**: Operations causing severe locks (e.g., TRUNCATE, DROP TABLE)
- **WARNING**: Operations with moderate impact (e.g., targeted UPDATE/DELETE)
- **INFO**: Operations with minimal impact (e.g., simple INSERT, SELECT)

### Notable Implementation Differences from Original Design
1. **Removed Features**:
   - No `-s/--severity` filter flag
   - No `--ignore-info` flag
   - No multiple file support with glob patterns
   - No `-V` for version (uses `-v` instead)

2. **Added Features**:
   - `--no-suggestion` flag for disabling suggestions
   - Stdin support for piped input
   - Comprehensive suggestion system with transaction safety info
   - More structured JSON/YAML output with suggestion details

## Implementation Status
- ✅ CLI fully implemented with cobra framework
- ✅ All core features operational
- ✅ Safe migration suggestions integrated
- ✅ All output formats working
- ⚠️ `--quiet` and `--verbose` flags exist but have limited effect
- ⚠️ `--no-color` flag exists but color implementation not visible