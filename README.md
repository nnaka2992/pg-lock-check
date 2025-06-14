# pg-lock-check

A PostgreSQL lock analyzer that examines SQL statements for potential locking issues and provides severity-based warnings about operations that could cause database locks or blocking.

## Overview

`pg-lock-check` analyzes SQL statements and warns about operations that could cause table locks, helping developers and DBAs identify potentially problematic queries before they impact production databases.

## Features

- **Transaction-aware analysis**: Different severity levels for operations inside vs outside transactions
- **229 PostgreSQL operations mapped**: Comprehensive coverage of DDL, DML, and maintenance operations
- **Multiple severity levels**: ERROR, CRITICAL, WARNING, INFO
- **Flexible input methods**: Direct SQL, files, or stdin
- **Multiple output formats**: Text (default), JSON, YAML
- **Exit codes for CI/CD**: Different exit codes based on highest severity found

## Installation

```bash
go install github.com/nnaka2992/pg-lock-check/cmd/pg-lock-check@latest
```

Or build from source:

```bash
git clone https://github.com/nnaka2992/pg-lock-check.git
cd pg-lock-check
go build -o pg-lock-check ./cmd/pg-lock-check
```

## Usage

### Basic usage

```bash
# Analyze a single SQL statement
pg-lock-check "ALTER TABLE users ADD COLUMN age INT DEFAULT 0"

# Analyze SQL from a file
pg-lock-check -f migration.sql

# Analyze multiple files
pg-lock-check -f migrations/*.sql

# Read from stdin
echo "TRUNCATE TABLE users" | pg-lock-check
```

### Transaction modes

By default, pg-lock-check assumes SQL runs inside a transaction:

```bash
# Default: analyze as if wrapped in BEGIN/COMMIT
pg-lock-check "CREATE INDEX CONCURRENTLY idx ON users(email)"
# Output: [ERROR] Cannot run inside a transaction block

# Analyze without transaction wrapper
pg-lock-check --no-transaction "CREATE INDEX CONCURRENTLY idx ON users(email)"
# Output: [WARNING] Creates ShareUpdateExclusiveLock on users
```

### Output formats

```bash
# Default text output
pg-lock-check "UPDATE users SET active = true"

# JSON output for programmatic use
pg-lock-check -o json "UPDATE users SET active = true"

# YAML output
pg-lock-check -o yaml "UPDATE users SET active = true"
```

### Filtering by severity

```bash
# Only show warnings and above (ignore INFO)
pg-lock-check --ignore-info -f migration.sql

# Set minimum severity level
pg-lock-check -s critical -f migration.sql
```

## Severity Levels

- **ERROR**: Operations that cannot run in the current mode (e.g., VACUUM in transaction)
- **CRITICAL**: Operations causing severe locks (e.g., TRUNCATE, DROP TABLE, UPDATE without WHERE)
- **WARNING**: Operations with moderate impact (e.g., CREATE INDEX, targeted UPDATE/DELETE)
- **INFO**: Operations with minimal impact (e.g., simple INSERT, SELECT)

## Exit Codes

- `0`: No issues or only INFO level
- `1`: Has WARNING level issues
- `2`: Has CRITICAL level issues
- `3`: Has ERROR level issues
- `4`: Invalid input/arguments

## Examples

### CI/CD Integration

```bash
#!/bin/bash
# Check migration files before deployment
pg-lock-check -f migrations/*.sql
if [ $? -ge 2 ]; then
    echo "Critical locking issues found!"
    exit 1
fi
```

### Pre-commit Hook

```bash
#!/bin/bash
# .git/hooks/pre-commit
files=$(git diff --cached --name-only --diff-filter=ACM | grep '\.sql$')
if [ -n "$files" ]; then
    pg-lock-check -f $files || exit 1
fi
```

## Development

```bash
# Run tests
go test ./...

# Run with coverage
go test -cover ./...

# Build
go build -o pg-lock-check ./cmd/pg-lock-check
```

## Architecture

The tool consists of three main components:

1. **Parser**: Wraps `pg_query_go` to parse SQL into AST
2. **Analyzer**: Maps PostgreSQL operations to lock severity levels
3. **CLI**: Command-line interface with multiple output formats

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see LICENSE file for details