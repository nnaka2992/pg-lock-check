package suggester

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

//go:embed suggestions.yaml
var suggestionsYAML []byte

// Suggester provides safe migration suggestions for CRITICAL operations
type Suggester interface {
	// HasSuggestion checks if a suggestion exists for the given operation
	HasSuggestion(operation string) bool

	// GetSuggestion returns a safe migration suggestion for the given operation
	GetSuggestion(operation string, metadata OperationMetadata) (*Suggestion, error)
}

// OperationMetadata is a flexible map for template data
type OperationMetadata map[string]interface{}

// Suggestion represents a safe migration suggestion
type Suggestion struct {
	Operation   string // Operation name (for tests)
	Category    string // Category (for tests)
	Description string
	Steps       []Step
	IsPartial   bool // True if this is only a partial alternative
}

// Step represents a single step in a migration suggestion
type Step struct {
	Description         string
	CanRunInTransaction bool
	Type                string // "sql", "command", "procedural"
	SQL                 string // SQL to execute
	Command             string // External command to run
	Notes               string // Procedural instructions
	SQLTemplate         string // Original template (for tests)
	CommandTemplate     string // Original template (for tests)
}

// ErrNoSuggestion is returned when no suggestion exists for an operation
var ErrNoSuggestion = fmt.Errorf("no suggestion available for this operation")

// yamlRoot represents the root structure of operations.yaml
type yamlRoot struct {
	OperationsWithAlternatives []operationDef `yaml:"operations_with_alternatives"`
}

// operationDef represents a single operation definition
type operationDef struct {
	Operation   string `yaml:"operation"`
	Category    string `yaml:"category"`
	Description string `yaml:"description"`
	IsPartial   bool   `yaml:"partial_alternative,omitempty"`
	Steps       []struct {
		Type                string `yaml:"type"`
		Description         string `yaml:"description"`
		SQL                 string `yaml:"sql,omitempty"`
		SQLTemplate         string `yaml:"sql_template,omitempty"`
		Command             string `yaml:"command,omitempty"`
		CommandTemplate     string `yaml:"command_template,omitempty"`
		Notes               string `yaml:"notes,omitempty"`
		CanRunInTransaction bool   `yaml:"can_run_in_transaction"`
	} `yaml:"steps"`
}

// operations holds all parsed operations from YAML, keyed by operation name
var operations map[string]operationDef

func init() {
	// Parse suggestions.yaml at startup
	var root yamlRoot
	if err := yaml.Unmarshal(suggestionsYAML, &root); err != nil {
		panic(fmt.Sprintf("failed to parse suggestions.yaml: %v", err))
	}

	// Build operations map
	operations = make(map[string]operationDef)
	for _, op := range root.OperationsWithAlternatives {
		operations[op.Operation] = op
	}
}

// suggester implements the Suggester interface
type suggester struct{}

// NewSuggester creates a new suggester instance
func NewSuggester() Suggester {
	return &suggester{}
}

// GetSuggestion returns a safe migration suggestion for the given operation
func (s *suggester) GetSuggestion(operation string, metadata OperationMetadata) (*Suggestion, error) {
	def, exists := operations[operation]
	if !exists {
		return nil, ErrNoSuggestion
	}

	// Validate critical fields that would produce invalid SQL
	if err := s.validateCriticalFields(operation, metadata); err != nil {
		return nil, err
	}

	suggestion := &Suggestion{
		Operation:   operation,
		Category:    def.Category,
		Description: def.Description,
		IsPartial:   def.IsPartial,
		Steps:       make([]Step, 0, len(def.Steps)),
	}

	for _, stepDef := range def.Steps {
		step := Step{
			Description:         stepDef.Description,
			CanRunInTransaction: stepDef.CanRunInTransaction,
			Type:                stepDef.Type,
		}

		// Store original templates for tests
		switch stepDef.Type {
		case "sql":
			if stepDef.SQLTemplate != "" {
				step.SQLTemplate = stepDef.SQLTemplate
			} else {
				step.SQLTemplate = stepDef.SQL
			}
		case "command", "external":
			if stepDef.CommandTemplate != "" {
				step.CommandTemplate = stepDef.CommandTemplate
			} else {
				step.CommandTemplate = stepDef.Command
			}
		}

		// Get the content based on type
		var content string
		switch stepDef.Type {
		case "sql":
			// Use sql_template if available, otherwise sql
			if stepDef.SQLTemplate != "" {
				content = stepDef.SQLTemplate
			} else {
				content = stepDef.SQL
			}
		case "command", "external":
			// Use command_template if available, otherwise command
			if stepDef.CommandTemplate != "" {
				content = stepDef.CommandTemplate
			} else {
				content = stepDef.Command
			}
		case "procedural":
			content = stepDef.Notes
		}

		// Simple template substitution
		content = s.substituteTemplate(content, metadata)

		// Assign content to appropriate field
		switch stepDef.Type {
		case "sql":
			step.SQL = content
		case "command", "external":
			step.Command = content
		case "procedural":
			step.Notes = content
		}

		suggestion.Steps = append(suggestion.Steps, step)
	}

	return suggestion, nil
}

// HasSuggestion returns true if a suggestion exists for the given operation
func (s *suggester) HasSuggestion(operation string) bool {
	_, exists := operations[operation]
	return exists
}

// substituteTemplate renders a template with the given metadata
func (s *suggester) substituteTemplate(tmplStr string, metadata OperationMetadata) string {
	// Handle empty template
	if tmplStr == "" {
		return ""
	}

	// Define template functions
	funcMap := template.FuncMap{
		"join":   strings.Join,
		"printf": fmt.Sprintf,
		"required": func(value interface{}, fieldName string) (interface{}, error) {
			if value == nil || value == "" {
				return nil, fmt.Errorf("missing required field: %s", s.fieldDisplayName(fieldName))
			}
			// Check for empty slices
			if slice, ok := value.([]string); ok && len(slice) == 0 {
				return nil, fmt.Errorf("field '%s' cannot be empty", s.fieldDisplayName(fieldName))
			}
			return value, nil
		},
		"error": func(msg string) (string, error) {
			return "", fmt.Errorf("%s", msg)
		},
	}

	// Parse and execute template
	tmpl, err := template.New("suggestion").Funcs(funcMap).Parse(tmplStr)
	if err != nil {
		// If template parsing fails, return the original string
		// This maintains backward compatibility and prevents crashes
		return tmplStr
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, metadata); err != nil {
		// If template execution fails, return the original string
		return tmplStr
	}

	return buf.String()
}

// fieldDisplayName converts field names to display format
func (s *suggester) fieldDisplayName(field string) string {
	displayNames := map[string]string{
		"tableName":        "TableName",
		"idColumn":         "IDColumn",
		"columnsValues":    "ColumnsValues",
		"indexName":        "IndexName",
		"columns":          "Columns",
		"targetTable":      "TargetTable",
		"sourceTable":      "SourceTable",
		"mergeCondition":   "MergeCondition",
		"matchedAction":    "MatchedAction",
		"notMatchedAction": "NotMatchedAction",
		"columnName":       "ColumnName",
		"columnType":       "ColumnType",
		"defaultValue":     "DefaultValue",
		"newType":          "NewType",
		"constraintName":   "ConstraintName",
		"column":           "Column",
		"viewName":         "ViewName",
		"schema":           "Schema",
	}

	if display, ok := displayNames[field]; ok {
		return display
	}
	// Default: capitalize first letter
	if len(field) > 0 {
		return strings.ToUpper(field[:1]) + field[1:]
	}
	return field
}

// validateCriticalFields checks for fields that would produce invalid SQL if missing
func (s *suggester) validateCriticalFields(operation string, metadata OperationMetadata) error {
	// Only validate fields that would cause invalid SQL
	criticalFields := map[string][]string{
		"CREATE INDEX":        {"tableName", "columns"},
		"CREATE UNIQUE INDEX": {"tableName", "columns"},
		"DROP INDEX":          {"indexName"},
		"REINDEX":             {"indexName"},
		"REINDEX TABLE":       {"tableName"},
		"REINDEX SCHEMA":      {"schema"},
		// DML operations can use defaults, so less critical
		"UPDATE without WHERE": {"tableName", "idColumn", "columnsValues"},
		"DELETE without WHERE": {"tableName", "idColumn"},
		"MERGE without WHERE":  {"targetTable", "sourceTable"},
		// DDL operations
		"ALTER TABLE ADD COLUMN with volatile DEFAULT": {"tableName", "columnName", "dataType"},
		"ALTER TABLE ALTER COLUMN TYPE":                {"tableName", "columnName", "newType"},
		"ALTER TABLE ADD PRIMARY KEY":                  {"tableName", "columns"},
		"ALTER TABLE ADD CONSTRAINT CHECK":             {"tableName", "constraintName"},
		"ALTER TABLE SET NOT NULL":                     {"tableName", "column"},
		"CLUSTER":                                      {"tableName", "indexName"},
		"REFRESH MATERIALIZED VIEW":                    {"viewName"},
		"VACUUM FULL":                                  {"tableName"},
	}

	fields, ok := criticalFields[operation]
	if !ok {
		return nil // No validation for unknown operations
	}

	for _, field := range fields {
		value, exists := metadata[field]
		if !exists || value == nil {
			return fmt.Errorf("missing required field: %s", s.fieldDisplayName(field))
		}

		// Check for empty strings
		if str, ok := value.(string); ok && str == "" {
			return fmt.Errorf("field '%s' cannot be empty", s.fieldDisplayName(field))
		}

		// Check for empty arrays
		if arr, ok := value.([]string); ok && len(arr) == 0 {
			return fmt.Errorf("field '%s' cannot be empty", s.fieldDisplayName(field))
		}
	}

	return nil
}
