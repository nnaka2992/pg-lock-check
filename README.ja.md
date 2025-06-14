<div align="center">

# 🔒 pg-lock-check

### PostgreSQLのロックを事前に検出

[English](README.md) | [日本語](README.ja.md) | [中文](README.zh.md)

[![CI](https://github.com/nnaka2992/pg-lock-check/actions/workflows/ci.yml/badge.svg)](https://github.com/nnaka2992/pg-lock-check/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/nnaka2992/pg-lock-check)](https://goreportcard.com/report/github.com/nnaka2992/pg-lock-check)
[![Go Reference](https://pkg.go.dev/badge/github.com/nnaka2992/pg-lock-check.svg)](https://pkg.go.dev/github.com/nnaka2992/pg-lock-check)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**本番環境を停止させる危険なPostgreSQLマイグレーションを事前に検出** 🚨

![pg-lock-check demo](docs/assets/demo.gif)

[**クイックスタート**](#-クイックスタート) • [**なぜ必要か**](#-なぜ必要か) • [**インストール**](#-インストール) • [**使い方**](#-使い方) • [**CI/CD連携**](#-cicd連携)

</div>

---

## 🚀 クイックスタート

```bash
# 5秒でインストール
go install github.com/nnaka2992/pg-lock-check/cmd/pg-lock-check@latest

# 危険なマイグレーションを検出
$ pg-lock-check "UPDATE users SET active = false"
[CRITICAL] UPDATE users SET active = false

Summary: 1 statements analyzed

# マイグレーションファイルをチェック
$ pg-lock-check -f migrations/*.sql
```

## 💡 なぜ必要か

「ちょっとした」データベースマイグレーションでアプリ全体が停止したことはありませんか？

```sql
-- 無害に見えますよね？ 違います！ 💀
UPDATE users SET last_login = NOW();
-- ☠️ テーブル全体をロック - アプリが停止します
```

**pg-lock-check**は災害が起こる**前に**検出します：

- 🎯 **229種類のPostgreSQL操作を分析** - 何がロックするか把握
- ⚡ **即座にフィードバック** - ポケベルが鳴る前に検出
- 🔄 **トランザクション対応** - トランザクション内外で異なるルール
- 🚦 **CI/CD対応** - 危険なマイグレーションを自動ブロック

## ✨ 機能

- 🧠 **スマート分析** - WHERE句ありなしの`UPDATE`の違いを理解
- 🎭 **トランザクションコンテキスト** - `CREATE INDEX CONCURRENTLY`はトランザクション外でのみ動作
- 📊 **複数の出力形式** - 人間が読める形式、ツール用のJSON、YAML
- 🚪 **意味のある終了コード** - CI/CDパイプラインに最適
- 📁 **一括分析** - マイグレーションディレクトリ全体を一度にチェック
- ⚡ **超高速** - CI/CDパイプラインを遅延させません

## 📦 インストール

<details>
<summary><b>オプション1: Goでインストール</b>（推奨）</summary>

```bash
go install github.com/nnaka2992/pg-lock-check/cmd/pg-lock-check@latest
```
</details>

<details>
<summary><b>オプション2: バイナリをダウンロード</b></summary>

[リリースページ](https://github.com/nnaka2992/pg-lock-check/releases)から最新版を取得。

</details>

<details>
<summary><b>オプション3: ソースからビルド</b></summary>

```bash
git clone https://github.com/nnaka2992/pg-lock-check.git
cd pg-lock-check
go build -o pg-lock-check ./cmd/pg-lock-check
```
</details>

## 🎯 使い方

### 実例

#### 😱 恐怖の例
```bash
# この無害に見えるクエリ...
$ pg-lock-check "UPDATE users SET preferences = '{}'"
[CRITICAL] UPDATE users SET preferences = '{}'

Summary: 1 statements analyzed
```

#### 🎉 正しい例
```bash
# WHERE句を追加して、週末を守る
$ pg-lock-check "UPDATE users SET preferences = '{}' WHERE id = 123"
[WARNING] UPDATE users SET preferences = '{}' WHERE id = 123

Summary: 1 statements analyzed
```

### 🔧 一般的なシナリオ

<details>
<summary><b>CREATE INDEX CONCURRENTLYの処理</b></summary>

```bash
# ❌ トランザクション内 - 失敗
$ pg-lock-check "CREATE INDEX CONCURRENTLY idx_users_email ON users(email)"
[ERROR] CREATE INDEX CONCURRENTLY idx_users_email ON users(email)

Summary: 1 statements analyzed

# ✅ トランザクション外 - 動作
$ pg-lock-check --no-transaction "CREATE INDEX CONCURRENTLY idx_users_email ON users(email)"
[WARNING] CREATE INDEX CONCURRENTLY idx_users_email ON users(email)

Summary: 1 statements analyzed
```
</details>

## 🚦 重要度レベル

| レベル | 意味 | 例 | 実行すべき？ |
|-------|------|-----|------------|
| 🔴 **ERROR** | このモードでは実行不可 | `VACUUM` inside transaction | ❌ コードを修正 |
| 🟠 **CRITICAL** | テーブル全体のロック | `UPDATE users SET active = true` | ⚠️ 午前3時のみ |
| 🟡 **WARNING** | 行/ページのロック | `UPDATE users SET ... WHERE id = 1` | ✅ おそらく大丈夫 |
| 🟢 **INFO** | 問題なし | `SELECT * FROM users` | ✅ デプロイ！ |

### 終了コード

- `0`: 成功 - 分析完了
- `1`: 実行時エラー - ファイルが見つからない、読み取りエラーなど
- `2`: パースエラー - 無効なSQL構文

## 🚀 CI/CD連携

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

## 🔮 今後の予定

- **実世界の重要度**: ロックタイプだけでなく、実際の本番環境への影響に基づく重要度
- **安全なマイグレーション提案**: 危険な操作に対して自動的により安全な代替案を提案

## ライセンス

MIT License - 詳細は LICENSE ファイルを参照