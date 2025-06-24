package analyzer

import "testing"

func TestNeedsQuoting(t *testing.T) {
	tests := []struct {
		name       string
		identifier string
		want       bool
	}{
		// Basic cases
		{"empty string", "", false},
		{"simple lowercase", "users", false},
		{"with underscore", "user_accounts", false},
		{"with numbers", "table123", false},
		{"underscore start", "_private", false},

		// Cases that need quoting
		{"uppercase letters", "Users", true},
		{"mixed case", "UserAccounts", true},
		{"starts with number", "123table", true},
		{"contains hyphen", "user-accounts", true},
		{"contains space", "user accounts", true},
		{"contains dot", "my.table", true},
		{"special characters", "user$data", true},

		// Reserved words
		{"reserved USER", "user", true},
		{"reserved ORDER", "order", true},
		{"reserved GROUP", "group", true},
		{"reserved TABLE", "table", true},
		{"reserved SELECT", "select", true},
		{"reserved WHERE", "where", true},
		{"reserved FROM", "from", true},
		{"reserved AS", "as", true},
		{"reserved COLUMN", "column", true},
		{"reserved CREATE", "create", true},

		// Reserved words in uppercase (still need quoting)
		{"reserved ORDER uppercase", "ORDER", true},
		{"reserved GROUP uppercase", "GROUP", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := needsQuoting(tt.identifier); got != tt.want {
				t.Errorf("needsQuoting(%q) = %v, want %v", tt.identifier, got, tt.want)
			}
		})
	}
}

func TestQuoteIdentifier(t *testing.T) {
	tests := []struct {
		name       string
		identifier string
		want       string
	}{
		// No quoting needed
		{"simple lowercase", "users", "users"},
		{"with underscore", "user_accounts", "user_accounts"},
		{"with numbers", "table123", "table123"},

		// Quoting needed
		{"uppercase", "Users", `"Users"`},
		{"mixed case", "UserAccounts", `"UserAccounts"`},
		{"with hyphen", "user-accounts", `"user-accounts"`},
		{"with space", "user accounts", `"user accounts"`},
		{"reserved word", "user", `"user"`},
		{"reserved word uppercase", "ORDER", `"ORDER"`},

		// Special cases
		{"embedded quotes", `table"with"quotes`, `"table""with""quotes"`},
		{"multiple embedded quotes", `a"b"c"d`, `"a""b""c""d"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := quoteIdentifier(tt.identifier); got != tt.want {
				t.Errorf("quoteIdentifier(%q) = %q, want %q", tt.identifier, got, tt.want)
			}
		})
	}
}

func TestQuoteQualifiedIdentifier(t *testing.T) {
	tests := []struct {
		name       string
		schema     string
		identifier string
		want       string
	}{
		// Simple cases
		{"no schema", "", "users", "users"},
		{"with schema", "public", "users", "public.users"},

		// Quoting needed
		{"quoted table", "public", "Users", `public."Users"`},
		{"quoted schema", "MySchema", "users", `"MySchema".users`},
		{"both quoted", "MySchema", "Users", `"MySchema"."Users"`},

		// Reserved words
		{"reserved table", "public", "user", `public."user"`},
		{"reserved schema", "order", "items", `"order".items`},

		// Special characters
		{"hyphen in table", "public", "user-accounts", `public."user-accounts"`},
		{"space in schema", "my schema", "users", `"my schema".users`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := quoteQualifiedIdentifier(tt.schema, tt.identifier); got != tt.want {
				t.Errorf("quoteQualifiedIdentifier(%q, %q) = %q, want %q",
					tt.schema, tt.identifier, got, tt.want)
			}
		})
	}
}

func TestUnquoteIdentifier(t *testing.T) {
	tests := []struct {
		name       string
		identifier string
		want       string
	}{
		// Not quoted
		{"simple", "users", "users"},
		{"underscore", "user_accounts", "user_accounts"},

		// Quoted
		{"quoted simple", `"users"`, "users"},
		{"quoted uppercase", `"Users"`, "Users"},
		{"quoted with space", `"user accounts"`, "user accounts"},

		// Escaped quotes
		{"escaped quotes", `"table""with""quotes"`, `table"with"quotes`},
		{"single escaped", `"a""b"`, `a"b`},

		// Edge cases
		{"single quote", `"`, `"`},
		{"just quotes", `""`, ""},
		{"unmatched quotes", `"users`, `"users`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := unquoteIdentifier(tt.identifier); got != tt.want {
				t.Errorf("unquoteIdentifier(%q) = %q, want %q", tt.identifier, got, tt.want)
			}
		})
	}
}

func TestIsQuoted(t *testing.T) {
	tests := []struct {
		name       string
		identifier string
		want       bool
	}{
		{"not quoted", "users", false},
		{"quoted", `"users"`, true},
		{"single quote start", `"users`, false},
		{"single quote end", `users"`, false},
		{"empty", "", false},
		{"just quotes", `""`, true},
		{"single char", `"`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isQuoted(tt.identifier); got != tt.want {
				t.Errorf("isQuoted(%q) = %v, want %v", tt.identifier, got, tt.want)
			}
		})
	}
}
