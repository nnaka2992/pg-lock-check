package analyzer

import "strings"

// PostgreSQL reserved words that require quoting when used as identifiers
// Based on PostgreSQL documentation: https://www.postgresql.org/docs/current/sql-keywords-appendix.html
var postgresReservedWords = map[string]bool{
	"all": true, "analyse": true, "analyze": true, "and": true, "any": true,
	"array": true, "as": true, "asc": true, "asymmetric": true, "both": true,
	"case": true, "cast": true, "check": true, "collate": true, "column": true,
	"constraint": true, "create": true, "current_catalog": true, "current_date": true,
	"current_role": true, "current_schema": true, "current_time": true,
	"current_timestamp": true, "current_user": true, "default": true,
	"deferrable": true, "desc": true, "distinct": true, "do": true,
	"else": true, "end": true, "except": true, "false": true, "fetch": true,
	"for": true, "foreign": true, "from": true, "grant": true, "group": true,
	"having": true, "in": true, "initially": true, "intersect": true,
	"into": true, "lateral": true, "leading": true, "limit": true,
	"localtime": true, "localtimestamp": true, "not": true, "null": true,
	"offset": true, "on": true, "only": true, "or": true, "order": true,
	"placing": true, "primary": true, "references": true, "returning": true,
	"select": true, "session_user": true, "some": true, "symmetric": true,
	"table": true, "then": true, "to": true, "trailing": true, "true": true,
	"union": true, "unique": true, "user": true, "using": true, "variadic": true,
	"when": true, "where": true, "window": true, "with": true,
	// Additional commonly problematic keywords
	"authorization": true, "between": true, "binary": true, "cross": true,
	"freeze": true, "full": true, "ilike": true, "inner": true, "is": true,
	"isnull": true, "join": true, "left": true, "like": true, "natural": true,
	"notnull": true, "outer": true, "overlaps": true, "right": true,
	"similar": true, "verbose": true,
}

// needsQuoting checks if a PostgreSQL identifier needs quoting
func needsQuoting(identifier string) bool {
	if len(identifier) == 0 {
		return false
	}

	// Check if it's a reserved word (case-insensitive)
	if postgresReservedWords[strings.ToLower(identifier)] {
		return true
	}

	// Check if it contains any uppercase letters
	hasUpper := false
	for _, ch := range identifier {
		if ch >= 'A' && ch <= 'Z' {
			hasUpper = true
			break
		}
	}
	if hasUpper {
		return true
	}

	// Check if it starts with lowercase letter or underscore
	firstChar := identifier[0]
	if (firstChar < 'a' || firstChar > 'z') && firstChar != '_' {
		return true
	}

	// Check remaining characters - must be lowercase letters, digits, or underscores
	for i := 1; i < len(identifier); i++ {
		ch := identifier[i]
		if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') && ch != '_' {
			return true
		}
	}

	return false
}

// quoteIdentifier quotes an identifier if it needs quoting
func quoteIdentifier(identifier string) string {
	if needsQuoting(identifier) {
		// Escape any embedded quotes by doubling them
		escaped := strings.ReplaceAll(identifier, `"`, `""`)
		return `"` + escaped + `"`
	}
	return identifier
}

// quoteQualifiedIdentifier quotes a schema-qualified identifier if needed
func quoteQualifiedIdentifier(schema, identifier string) string {
	if schema != "" {
		return quoteIdentifier(schema) + "." + quoteIdentifier(identifier)
	}
	return quoteIdentifier(identifier)
}

// unquoteIdentifier removes quotes from an identifier if present
func unquoteIdentifier(identifier string) string {
	if len(identifier) >= 2 && identifier[0] == '"' && identifier[len(identifier)-1] == '"' {
		// Remove outer quotes and unescape any doubled quotes
		unquoted := identifier[1 : len(identifier)-1]
		return strings.ReplaceAll(unquoted, `""`, `"`)
	}
	return identifier
}

// isQuoted checks if an identifier is already quoted
func isQuoted(identifier string) bool {
	return len(identifier) >= 2 && identifier[0] == '"' && identifier[len(identifier)-1] == '"'
}
