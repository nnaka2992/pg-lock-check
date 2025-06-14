package analyzer

// Severity represents the severity level of a database operation
type Severity int

const (
	SeverityInfo Severity = iota
	SeverityWarning
	SeverityCritical
	SeverityError
)

func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "INFO"
	case SeverityWarning:
		return "WARNING"
	case SeverityCritical:
		return "CRITICAL"
	case SeverityError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// TransactionMode indicates whether the operation is executed within a transaction
type TransactionMode int

const (
	InTransaction TransactionMode = iota
	NoTransaction
)

func (m TransactionMode) String() string {
	switch m {
	case InTransaction:
		return "IN_TRANSACTION"
	case NoTransaction:
		return "NO_TRANSACTION"
	default:
		return "UNKNOWN"
	}
}

// LockType represents PostgreSQL lock types
type LockType string

const (
	AccessShare          LockType = "AccessShare"
	RowShare             LockType = "RowShare"
	RowExclusive         LockType = "RowExclusive"
	ShareUpdateExclusive LockType = "ShareUpdateExclusive"
	Share                LockType = "Share"
	ShareRowExclusive    LockType = "ShareRowExclusive"
	Exclusive            LockType = "Exclusive"
	AccessExclusive      LockType = "AccessExclusive"
)

// Result represents the analysis result of a SQL statement
type Result struct {
	Severity      Severity
	operation     string
	lockType      LockType
	tableLocks    []string
	message       string
}

// Operation returns the operation type
func (r *Result) Operation() string {
	return r.operation
}

// LockType returns the lock type for the operation
func (r *Result) LockType() LockType {
	return r.lockType
}

// TableLocks returns formatted table lock information
func (r *Result) TableLocks() []string {
	return r.tableLocks
}

// Message returns any additional message about the operation
func (r *Result) Message() string {
	return r.message
}