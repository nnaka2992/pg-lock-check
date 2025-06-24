<div align="center">

# 🔒 pg-lock-check

### 在 PostgreSQL 锁定发生之前阻止它

[English](README.md) | [日本語](README.ja.md) | [中文](README.zh.md)

[![CI](https://github.com/nnaka2992/pg-lock-check/actions/workflows/ci.yml/badge.svg)](https://github.com/nnaka2992/pg-lock-check/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/nnaka2992/pg-lock-check)](https://goreportcard.com/report/github.com/nnaka2992/pg-lock-check)
[![Go Reference](https://pkg.go.dev/badge/github.com/nnaka2992/pg-lock-check.svg)](https://pkg.go.dev/github.com/nnaka2992/pg-lock-check)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**在危险的 PostgreSQL 迁移导致生产环境宕机之前捕获它们** 🚨

![pg-lock-check demo](docs/sample/demo.gif)

[**快速开始**](#-快速开始) • [**为什么需要它**](#-为什么需要它) • [**安装**](#-安装) • [**使用方法**](#-使用方法) • [**CI/CD 集成**](#-cicd-集成)

</div>

---

## 🚀 快速开始

```bash
# 5秒安装完成
go install github.com/nnaka2992/pg-lock-check/cmd/pg-lock-check@latest

# 捕获危险的迁移
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

# 检查迁移文件
$ pg-lock-check -f migration.sql
```

## 💡 为什么需要它

是否遇到过一个"快速"的数据库迁移导致整个应用崩溃？我们也经历过。

```sql
-- 看起来无害，对吧？错了！ 💀
UPDATE users SET last_login = NOW();
-- ☠️ 锁定整个表 - 你的应用会崩溃
```

**pg-lock-check** 在灾难发生**之前**捕获它们：

- 🎯 **分析 229 种 PostgreSQL 操作** - 我们知道什么会锁定，什么不会
- ⚡ **即时反馈** - 在你的呼叫器响起之前就知道问题
- 🔄 **事务感知** - 事务内外有不同的规则
- 🚦 **CI/CD 就绪** - 自动阻止危险的迁移

## ✨ 功能

- 🧠 **智能分析** - 知道有 WHERE 和没有 WHERE 的 `UPDATE` 之间的区别
- 🎭 **事务上下文** - `CREATE INDEX CONCURRENTLY` 只在事务外工作
- 💡 **安全迁移建议** - 为危险操作提供可执行的替代方案
- 📊 **多种输出格式** - 人类可读格式、工具用的 JSON、YAML
- 🚪 **有意义的退出码** - 完美适配 CI/CD 流水线
- 📁 **文件分析** - 直接检查 SQL 文件
- ⚡ **闪电般快速** - 不会拖慢你的 CI/CD 流水线

## 📦 安装

<details>
<summary><b>选项 1：使用 Go 安装</b>（推荐）</summary>

```bash
go install github.com/nnaka2992/pg-lock-check/cmd/pg-lock-check@latest
```
</details>

<details>
<summary><b>选项 2：下载二进制文件</b></summary>

从[发布页面](https://github.com/nnaka2992/pg-lock-check/releases)获取最新版本。

</details>

<details>
<summary><b>选项 3：从源码构建</b></summary>

```bash
git clone https://github.com/nnaka2992/pg-lock-check.git
cd pg-lock-check
go build -o pg-lock-check ./cmd/pg-lock-check
```
</details>

## 🎯 使用方法

### 真实示例

#### 😱 恐怖故事
```bash
# 这个看起来无害的查询...
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

#### 🎉 正确做法
```bash
# 添加 WHERE 子句，拯救你的周末
$ pg-lock-check "UPDATE users SET preferences = '{}' WHERE id = 123"
[WARNING] UPDATE users SET preferences = '{}' WHERE id = 123

Summary: 1 statements analyzed
```

### 🔧 常见场景

<details>
<summary><b>检查迁移文件</b></summary>

```bash
# 单个文件
pg-lock-check -f migrations/20240114_add_index.sql

# 从 CI/CD 流水线
pg-lock-check -f migration.sql || exit 1
```
</details>

<details>
<summary><b>处理 CREATE INDEX CONCURRENTLY</b></summary>

```bash
# ❌ 在事务内 - 失败
$ pg-lock-check "CREATE INDEX CONCURRENTLY idx_users_email ON users(email)"
[ERROR] CREATE INDEX CONCURRENTLY idx_users_email ON users(email)

Summary: 1 statements analyzed

# ✅ 在事务外 - 正常工作
$ pg-lock-check --no-transaction "CREATE INDEX CONCURRENTLY idx_users_email ON users(email)"
[WARNING] CREATE INDEX CONCURRENTLY idx_users_email ON users(email)

Summary: 1 statements analyzed
```
</details>

<details>
<summary><b>工具用 JSON 输出</b></summary>

```bash
pg-lock-check -o json "TRUNCATE users" | jq '.severity'
# "CRITICAL"

# 在脚本中使用
SEVERITY=$(pg-lock-check -o json "$SQL" | jq -r '.results[0].severity')
if [ "$SEVERITY" = "CRITICAL" ]; then
  echo "🚨 危险！不要在生产环境运行！"
  exit 1
fi
```
</details>

## 💡 安全迁移建议

pg-lock-check 不仅仅是警告您 - 它还会展示如何修复危险操作！获取避免长时间锁定的逐步迁移模式。

- ✅ **18 个 CRITICAL 操作**有安全替代方案
- 🎯 **智能建议**：批处理、CONCURRENTLY 操作等
- 📊 **事务安全指示器**：每个步骤的指示

参见[安全迁移模式](docs/design/suggestions.md)了解所有可用建议。

### 快速示例

```bash
$ pg-lock-check "CREATE INDEX idx_users_email ON users(email)"
[CRITICAL] CREATE INDEX idx_users_email ON users(email)
Suggestion for safe migration:
  Step: Use `CREATE INDEX CONCURRENTLY` outside transaction
    Can run in transaction: No
    SQL:
      CREATE INDEX CONCURRENTLY idx_users_email ON users (email);
```

使用 `--no-suggestion` 标志禁用建议。

## 🚦 严重级别

| 级别 | 含义 | 示例 | 应该运行吗？ |
|------|------|------|------------|
| 🔴 **ERROR** | 在此模式下无法运行 | `VACUUM` inside transaction | ❌ 修复代码 |
| 🟠 **CRITICAL** | 全表锁定 | `UPDATE users SET active = true` | ⚠️ 仅在凌晨3点 |
| 🟡 **WARNING** | 行/页锁定 | `UPDATE users SET ... WHERE id = 1` | ✅ 可能没问题 |
| 🟢 **INFO** | 没问题 | `SELECT * FROM users` | ✅ 发布吧！ |

### 退出码

- `0`: 成功 - 分析完成
- `1`: 运行时错误 - 文件未找到、读取错误等
- `2`: 解析错误 - 无效的 SQL 语法

## 🚀 CI/CD 集成

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
    echo "🔍 检查 SQL 文件的锁定问题..."
    pg-lock-check -f $files || exit 1
fi
```

## 🛠️ 开发

```bash
# 克隆和测试
git clone https://github.com/nnaka2992/pg-lock-check.git
cd pg-lock-check
go test ./...

# 构建
go build -o pg-lock-check ./cmd/pg-lock-check
```

## 🏗️ 架构

- **Parser**: 封装 `pg_query_go` 用于 PostgreSQL AST 解析
- **Analyzer**: 将 229 种操作映射到锁严重级别
- **Suggester**: 为 CRITICAL 操作提供安全迁移模式
- **Metadata**: 提取 SQL 元数据用于建议生成
- **CLI**: 具有多种输出格式的清洁接口

## 🤝 贡献

发现 bug？需要新功能？欢迎 PR！

## 🔮 未来计划

- **增强 CLI 输出**: 添加详细的锁信息和影响描述
- **并行分析**: 同时分析多个文件以加快 CI/CD
- **自定义规则**: 为特定操作定义自己的严重级别
- **长事务处理**: 一些 WARNING 级别操作在长时间运行的事务中可能升级为 CRITICAL

## 许可证

MIT License - 详见 LICENSE 文件