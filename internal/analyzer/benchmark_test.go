package analyzer

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nnaka2992/pg-lock-check/internal/parser"
)

func BenchmarkAnalyzeSingleStatement(b *testing.B) {
	p := parser.NewParser()
	parsed, err := p.ParseSQL("SELECT * FROM users WHERE id = 1")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a := New()
		_, err := a.AnalyzeStatement(parsed.Statements[0], InTransaction)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAnalyzeMultipleStatements(b *testing.B) {
	p := parser.NewParser()

	// Build a realistic multi-statement SQL
	var stmts []string
	for i := 0; i < 100; i++ {
		switch i % 5 {
		case 0:
			stmts = append(stmts, fmt.Sprintf("SELECT * FROM table_%d WHERE id = %d", i, i))
		case 1:
			stmts = append(stmts, fmt.Sprintf("UPDATE table_%d SET col = %d WHERE id = %d", i, i, i))
		case 2:
			stmts = append(stmts, fmt.Sprintf("INSERT INTO table_%d (col) VALUES (%d)", i, i))
		case 3:
			stmts = append(stmts, fmt.Sprintf("DELETE FROM table_%d WHERE id = %d", i, i))
		case 4:
			stmts = append(stmts, fmt.Sprintf("CREATE INDEX idx_%d ON table_%d(col)", i, i))
		}
	}
	sql := strings.Join(stmts, ";\n") + ";"

	parsed, err := p.ParseSQL(sql)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a := New()
		_, err := a.Analyze(parsed, InTransaction)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRegistryLookup(b *testing.B) {
	registry := newOperationRegistry()

	operations := []string{
		"SELECT", "INSERT", "UPDATE with WHERE", "DELETE without WHERE",
		"CREATE INDEX", "ALTER TABLE ADD COLUMN without DEFAULT",
		"VACUUM", "TRUNCATE", "DROP TABLE",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		op := operations[i%len(operations)]
		registry.getSeverityAndLock(op, InTransaction)
	}
}

func BenchmarkNewRegistry(b *testing.B) {
	for i := 0; i < b.N; i++ {
		newOperationRegistry()
	}
}
