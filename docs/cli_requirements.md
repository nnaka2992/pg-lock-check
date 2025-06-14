# CLI Requirements for pg-lock-check

## Command Structure
```bash
pg-lock-check [OPTIONS] [SQL_STATEMENT | -f FILE...]
```

## Input Methods
1. **Direct SQL input**: Pass SQL statement as argument
2. **File input**: Use `-f` or `--file` flag for file(s)
3. **Multiple files**: Support glob patterns and multiple file arguments

## Options/Flags

### Required (one of):
- `SQL_STATEMENT` - Direct SQL input as argument
- `-f, --file FILE` - Read SQL from file(s)

### Transaction Mode:
- `--transaction` - Analyze assuming wrapped in transaction (default)
- `--no-transaction` - Analyze assuming no transaction wrapper

### Output Control:
- `-o, --output FORMAT` - Output format: `text` (default), `json`, `yaml`
- `-v, --verbose` - Show detailed analysis including safe operations
- `-q, --quiet` - Only show errors and critical issues
- `--no-color` - Disable colored output

### Filtering:
- `-s, --severity LEVEL` - Minimum severity to report: `error`, `critical`, `warning`, `info`
- `--ignore-info` - Don't show INFO level issues (shorthand for `-s warning`)

### Help/Version:
- `-h, --help` - Show help message
- `-V, --version` - Show version information

## Output Format

### Default (text):
```
[ERROR] line 5: CREATE INDEX CONCURRENTLY idx_users_email ON users(email)
  Cannot run inside a transaction block
  Suggestion: Use --no-transaction flag or remove from transaction

[CRITICAL] line 12: UPDATE users SET status = 'active'
  Missing WHERE clause locks entire table (RowExclusive + all row locks)
  Lock type: RowExclusive
  Impact: Blocks all concurrent updates/deletes on users table

Summary: 1 error, 1 critical, 0 warnings, 2 info
```

### JSON format:
```json
{
  "results": [
    {
      "severity": "ERROR",
      "line": 5,
      "statement": "CREATE INDEX CONCURRENTLY idx_users_email ON users(email)",
      "message": "Cannot run inside a transaction block",
      "suggestion": "Use --no-transaction flag or remove from transaction",
      "lock_type": null
    }
  ],
  "summary": {
    "error": 1,
    "critical": 1,
    "warning": 0,
    "info": 2
  }
}
```

### YAML format:
```yaml
results:
  - severity: ERROR
    line: 5
    statement: CREATE INDEX CONCURRENTLY idx_users_email ON users(email)
    message: Cannot run inside a transaction block
    suggestion: Use --no-transaction flag or remove from transaction
    lock_type: null
summary:
  error: 1
  critical: 1
  warning: 0
  info: 2
```

## Exit Codes
- `0` - No issues or only INFO level
- `1` - Has WARNING level issues
- `2` - Has CRITICAL level issues  
- `3` - Has ERROR level issues
- `4` - Invalid input/arguments

## Examples

```bash
# Analyze direct SQL
pg-lock-check "ALTER TABLE users ADD COLUMN age INT DEFAULT 0"

# Analyze file
pg-lock-check -f migration.sql

# Analyze multiple files
pg-lock-check -f migrations/*.sql

# Non-transaction mode
pg-lock-check --no-transaction -f concurrent_index.sql

# JSON output with minimum severity
pg-lock-check -f migration.sql -o json -s warning

# Verbose mode shows all operations
pg-lock-check -v -f migration.sql

# Quiet mode for CI/CD
pg-lock-check -q -f migration.sql
```

## Error Handling
- Clear error messages for invalid SQL syntax
- Handle empty files gracefully
- Support comments in SQL files
- Handle multi-statement files
- Detect and report if file doesn't exist

## Performance Requirements
- Process files up to 10MB efficiently
- Support analyzing 100+ migration files in batch
- Stream processing for large files

## Implementation Notes
- Use cobra or similar for CLI framework
- Support both short and long flag formats
- Implement proper signal handling (Ctrl+C)
- Provide helpful error messages with context
- Support reading from stdin when no arguments provided