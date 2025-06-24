<div align="center">

# 🔒 pg-lock-check

### Stop PostgreSQL Locks Before They Stop You

[English](README.md) | [日本語](README.ja.md) | [中文](README.zh.md)

[![CI](https://github.com/nnaka2992/pg-lock-check/actions/workflows/ci.yml/badge.svg)](https://github.com/nnaka2992/pg-lock-check/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/nnaka2992/pg-lock-check)](https://goreportcard.com/report/github.com/nnaka2992/pg-lock-check)
[![Go Reference](https://pkg.go.dev/badge/github.com/nnaka2992/pg-lock-check.svg)](https://pkg.go.dev/github.com/nnaka2992/pg-lock-check)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Catch dangerous PostgreSQL migrations before they bring down production** 🚨

![pg-lock-check demo](docs/sample/demo.gif)

[**Quick Start**](#-quick-start) • [**Why You Need This**](#-why-you-need-this) • [**Installation**](#-installation) • [**Usage**](#-usage) • [**CI/CD Integration**](#-cicd-integration)

</div>

---

## 🚀 Quick Start

```bash
# Install in 5 seconds
go install github.com/nnaka2992/pg-lock-check/cmd/pg-lock-check@latest

# Catch that dangerous migration
$ pg-lock-check "UPDATE users SET active = false"
[CRITICAL] UPDATE users SET active = false
Suggestion for safe migration:
  Step: Export target row IDs to file
    Can run in transaction: Yes
    SQL:
      \COPY (SELECT id FROM users ORDER BY id) TO '/path/to/target_ids.csv' CSV
  Step: Process file in batches with progress tracking
    Can run in transaction: No
    Instructions:
      1. Read ID file in chunks (e.g., 1000-5000 rows)
      2. For each chunk:
         - Build explicit ID list
         - Execute UPDATE users SET active = false WHERE id IN (chunk_ids)
         - Commit transaction
         - Log progress (line number or ID range)
         - Sleep 100-500ms between batches
         - Monitor replication lag
      3. Handle failures with resume capability

Summary: 1 statements analyzed

# Check your migration file
$ pg-lock-check -f migration.sql
```

## 💡 Why You Need This

Ever had a "quick" database migration take down your entire app? Yeah, we've been there too.

```sql
-- Looks innocent, right? WRONG! 💀
UPDATE users SET last_login = NOW();
-- ☠️ LOCKS ENTIRE TABLE - RIP your app
```

**pg-lock-check** catches these disasters **before** they happen:

- 🎯 **229 PostgreSQL operations analyzed** - We know what locks and what doesn't
- ⚡ **Instant feedback** - Know in milliseconds, not after your pager goes off
- 🔄 **Transaction-aware** - Different rules for inside/outside transactions
- 🚦 **CI/CD ready** - Block dangerous migrations automatically

## ✨ Features

- 🧠 **Smart Analysis** - Knows the difference between `UPDATE` with and without `WHERE`
- 🎭 **Transaction Context** - `CREATE INDEX CONCURRENTLY` works outside transactions, fails inside
- 💡 **Safe Migration Suggestions** - Get actionable alternatives for dangerous operations
- 📊 **Multiple Output Formats** - Human-readable, JSON for your tools, YAML because why not
- 🚪 **Exit Codes That Make Sense** - Perfect for CI/CD pipelines
- 📁 **File Analysis** - Check SQL files directly
- ⚡ **Lightning Fast** - Won't slow down your CI/CD pipeline

## 📦 Installation

<details>
<summary><b>Option 1: Install with Go</b> (Recommended)</summary>

```bash
go install github.com/nnaka2992/pg-lock-check/cmd/pg-lock-check@latest
```
</details>

<details>
<summary><b>Option 2: Download Binary</b></summary>

Grab the latest from the [releases page](https://github.com/nnaka2992/pg-lock-check/releases).

</details>

<details>
<summary><b>Option 3: Build from Source</b></summary>

```bash
git clone https://github.com/nnaka2992/pg-lock-check.git
cd pg-lock-check
go build -o pg-lock-check ./cmd/pg-lock-check
```
</details>

## 🎯 Usage

### Real-World Examples

#### 😱 The Horror Story
```bash
# This innocent-looking query...
$ pg-lock-check "UPDATE users SET preferences = '{}'"
[CRITICAL] UPDATE users SET preferences = '{}'
Suggestion for safe migration:
  Step: Export target row IDs to file
    Can run in transaction: Yes
    SQL:
      \COPY (SELECT id FROM users ORDER BY id) TO '/path/to/target_ids.csv' CSV
  Step: Process file in batches with progress tracking
    Can run in transaction: No
    Instructions:
      1. Read ID file in chunks (e.g., 1000-5000 rows)
      2. For each chunk:
         - Build explicit ID list
         - Execute UPDATE users SET preferences = '{}' WHERE id IN (chunk_ids)
         - Commit transaction
         - Log progress (line number or ID range)
         - Sleep 100-500ms between batches
         - Monitor replication lag
      3. Handle failures with resume capability

Summary: 1 statements analyzed
```

#### 🎉 The Happy Path
```bash
# Add a WHERE clause, save your weekend
$ pg-lock-check "UPDATE users SET preferences = '{}' WHERE id = 123"
[WARNING] UPDATE users SET preferences = '{}' WHERE id = 123

Summary: 1 statements analyzed
```

### 🔧 Common Scenarios

<details>
<summary><b>Check Your Migration Files</b></summary>

```bash
# Check a migration file
pg-lock-check -f migrations/20240114_add_index.sql

# From your CI/CD pipeline
pg-lock-check -f migration.sql || exit 1
```
</details>

<details>
<summary><b>Handle CREATE INDEX CONCURRENTLY</b></summary>

```bash
# ❌ Inside a transaction - FAILS
$ pg-lock-check "CREATE INDEX CONCURRENTLY idx_users_email ON users(email)"
[ERROR] CREATE INDEX CONCURRENTLY idx_users_email ON users(email)

Summary: 1 statements analyzed

# ✅ Outside a transaction - WORKS
$ pg-lock-check --no-transaction "CREATE INDEX CONCURRENTLY idx_users_email ON users(email)"
[WARNING] CREATE INDEX CONCURRENTLY idx_users_email ON users(email)

Summary: 1 statements analyzed
```
</details>

<details>
<summary><b>JSON Output for Your Tools</b></summary>

```bash
pg-lock-check -o json "TRUNCATE users" | jq '.severity'
# "CRITICAL"

# Use in scripts
SEVERITY=$(pg-lock-check -o json "$SQL" | jq -r '.results[0].severity')
if [ "$SEVERITY" = "CRITICAL" ]; then
  echo "🚨 DANGER! Don't run this in production!"
  exit 1
fi
```
</details>

## 💡 Safe Migration Suggestions

pg-lock-check doesn't just warn you - it shows you how to fix dangerous operations! Get step-by-step migration patterns that avoid long-running locks.

- ✅ **18 CRITICAL operations** have safe alternatives
- 🎯 **Smart suggestions** for batching, CONCURRENTLY operations, and more
- 📊 **Transaction safety** indicators for each step

See [Safe Migration Patterns](docs/design/suggestions.md) for all available suggestions.

### Quick Example

```bash
$ pg-lock-check "CREATE INDEX idx_users_email ON users(email)"
[CRITICAL] CREATE INDEX idx_users_email ON users(email)
Suggestion for safe migration:
  Step: Use `CREATE INDEX CONCURRENTLY` outside transaction
    Can run in transaction: No
    SQL:
      CREATE INDEX CONCURRENTLY idx_users_email ON users (email);
```

Disable suggestions with `--no-suggestion` flag.

## 🚦 Severity Levels

| Level | What It Means | Example | Should You Run It? |
|-------|--------------|---------|-------------------|
| 🔴 **ERROR** | Can't run in this mode | `VACUUM` inside transaction | ❌ Fix your code |
| 🟠 **CRITICAL** | Table-wide locks | `UPDATE users SET active = true` | ⚠️ Only at 3 AM |
| 🟡 **WARNING** | Row/page locks | `UPDATE users SET ... WHERE id = 1` | ✅ Probably fine |
| 🟢 **INFO** | You're good | `SELECT * FROM users` | ✅ Ship it! |

### Exit Codes

- `0`: Success - Analysis completed
- `1`: Runtime error - File not found, read errors, etc.
- `2`: Parse error - Invalid SQL syntax

## 🚀 CI/CD Integration

### GitHub Actions
```yaml
name: Check Migrations
on: [pull_request]

jobs:
  check-locks:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
      - run: go install github.com/nnaka2992/pg-lock-check/cmd/pg-lock-check@latest
      - name: Check for dangerous locks
        run: |
          pg-lock-check -f migration.sql -o json | \
          jq -e '.results[] | select(.severity == "CRITICAL" or .severity == "ERROR")' && \
          echo "🚨 Dangerous operations detected!" && exit 1 || \
          echo "✅ Migrations look safe!"
```

### Pre-commit Hook
```bash
#!/bin/bash
# .git/hooks/pre-commit
files=$(git diff --cached --name-only --diff-filter=ACM | grep '\.sql$')
if [ -n "$files" ]; then
    echo "🔍 Checking SQL files for lock issues..."
    pg-lock-check -f $files || exit 1
fi
```

## 🛠️ Development

```bash
# Clone & test
git clone https://github.com/nnaka2992/pg-lock-check.git
cd pg-lock-check
go test ./...

# Build
go build -o pg-lock-check ./cmd/pg-lock-check
```

## 🏗️ Architecture

- **Parser**: Wraps `pg_query_go` for PostgreSQL AST parsing
- **Analyzer**: Maps 229 operations to lock severity levels
- **Suggester**: Provides safe migration patterns for CRITICAL operations
- **Metadata**: Extracts SQL metadata for suggestion generation
- **CLI**: Clean interface with multiple output formats

## 🤝 Contributing

Found a bug? Want a feature? PRs welcome!

## 🔮 Future Work

- **Enhanced CLI output**: Add detailed lock information and impact descriptions
- **Parallel analysis**: Analyze multiple files concurrently for faster CI/CD
- **Custom rules**: Define your own severity levels for specific operations
- **Long transaction handling**: Some WARNING level operations can escalate to CRITICAL with long-running transactions

## License

MIT License - see LICENSE file for details
