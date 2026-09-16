package store

import (
	"errors"

	"modernc.org/sqlite"
)

// Sentinel errors the HTTP layer maps onto status codes.
var (
	// ErrNotFound covers a missing row and, for the nested item routes, an item
	// that exists but belongs to a different meeting.
	ErrNotFound = errors.New("record not found")

	// ErrConflict is a problem already on the agenda it is being added to.
	ErrConflict = errors.New("already on agenda")

	// ErrInvalidOrder is a reorder whose id list does not match the meeting's
	// items exactly.
	ErrInvalidOrder = errors.New("order list does not match the meeting's items")

	// ErrUnknownProblem is a referenced problem id that does not exist.
	ErrUnknownProblem = errors.New("unknown problem id")
)

// SQLite extended result codes. Declared here rather than imported from
// modernc.org/sqlite/lib, which is an enormous generated package to pull in for
// two integers.
const (
	sqliteConstraintUnique     = 2067
	sqliteConstraintPrimaryKey = 1555
)

// isUniqueViolation reports whether err is a UNIQUE/PRIMARY KEY constraint
// failure. It inspects the driver's error code — never the message text, which
// is not a stable interface.
func isUniqueViolation(err error) bool {
	var se *sqlite.Error
	if !errors.As(err, &se) {
		return false
	}
	code := se.Code()
	return code == sqliteConstraintUnique || code == sqliteConstraintPrimaryKey
}
