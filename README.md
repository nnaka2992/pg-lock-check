<div align="center">

# 🔒 pg-lock-check

### Stop PostgreSQL Locks Before They Stop You

[English](README.md) | [日本語](README.ja.md) | [中文](README.zh.md)

[![CI](https://github.com/nnaka2992/pg-lock-check/actions/workflows/ci.yml/badge.svg)](https://github.com/nnaka2992/pg-lock-check/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/nnaka2992/pg-lock-check)](https://goreportcard.com/report/github.com/nnaka2992/pg-lock-check)
[![Go Reference](https://pkg.go.dev/badge/github.com/nnaka2992/pg-lock-check.svg)](https://pkg.go.dev/github.com/nnaka2992/pg-lock-check)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Catch dangerous PostgreSQL migrations before they bring down production** 🚨

![pg-lock-check demo](docs/assets/demo.gif)

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

Summary: 1 statements analyzed

# Check your migration files
$ pg-lock-check -f migrations/*.sql
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
- 📊 **Multiple Output Formats** - Human-readable, JSON for your tools, YAML because why not
- 🚪 **Exit Codes That Make Sense** - Perfect for CI/CD pipelines
- 📁 **Bulk Analysis** - Check entire migration directories at once
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
# Single file
pg-lock-check -f migrations/20240114_add_index.sql

# All migrations at once
pg-lock-check -f migrations/*.sql

# From your CI/CD pipeline
pg-lock-check -f migrations/*.sql || exit 1
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
          pg-lock-check -f migrations/*.sql -o json | \
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
- **CLI**: Clean interface with multiple output formats

## 🤝 Contributing

Found a bug? Want a feature? PRs welcome!

## 🔮 Future Work

- **Real-world severity**: Base severity on actual production impact, not just lock types
- **Safe migration suggestions**: Automatically suggest safer alternatives for dangerous operations

## License

MIT License - see LICENSE file for details