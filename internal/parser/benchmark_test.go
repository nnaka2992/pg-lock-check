package parser

import (
	"fmt"
	"strings"
	"testing"
)

func BenchmarkParseSQL(b *testing.B) {
	sql := "SELECT id, name, email FROM users WHERE active = true AND created_at > '2024-01-01'"
	p := NewParser()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.ParseSQL(sql)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseLargeFile(b *testing.B) {
	var stmts []string
	for i := 0; i < 100; i++ {
		stmts = append(stmts, fmt.Sprintf("INSERT INTO test_table (id, name, value) VALUES (%d, 'name_%d', %d)", i, i, i*10))
	}
	sql := strings.Join(stmts, ";\n") + ";"
	p := NewParser()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.ParseSQL(sql)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseMixedStatements(b *testing.B) {
	sql := `
BEGIN;
CREATE TABLE users (id SERIAL PRIMARY KEY, name TEXT NOT NULL, email TEXT);
CREATE INDEX idx_users_email ON users(email);
INSERT INTO users (name, email) VALUES ('test', 'test@example.com');
UPDATE users SET name = 'updated' WHERE id = 1;
DELETE FROM users WHERE id = 2;
SELECT * FROM users WHERE email LIKE '%@example.com';
COMMIT;`
	p := NewParser()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.ParseSQL(sql)
		if err != nil {
			b.Fatal(err)
		}
	}
}
