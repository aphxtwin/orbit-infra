package db

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Common database errors
var (
	// ErrNotFound indicates a record was not found
	ErrNotFound = errors.New("record not found")

	// ErrDuplicateKey indicates a unique constraint violation
	ErrDuplicateKey = errors.New("duplicate key violation")

	// ErrForeignKeyViolation indicates a foreign key constraint violation
	ErrForeignKeyViolation = errors.New("foreign key violation")

	// ErrConnectionFailed indicates database connection failure
	ErrConnectionFailed = errors.New("database connection failed")
)

// PostgreSQL error codes
// See: https://www.postgresql.org/docs/current/errcodes-appendix.html
const (
	PgErrCodeUniqueViolation     = "23505"
	PgErrCodeForeignKeyViolation = "23503"
	PgErrCodeCheckViolation      = "23514"
)

// WrapError converts pgx errors to domain-specific errors
// This makes error handling more idiomatic in Go
//
// Example usage:
//   err := repo.GetByID(ctx, id)
//   if db.IsNotFound(err) {
//       return nil, fmt.Errorf("tenant not found")
//   }
func WrapError(err error) error {
	if err == nil {
		return nil
	}

	// Handle pgx.ErrNoRows (SELECT returned no rows)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}

	// Handle PostgreSQL-specific errors
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case PgErrCodeUniqueViolation:
			return fmt.Errorf("%w: %s", ErrDuplicateKey, pgErr.Detail)
		case PgErrCodeForeignKeyViolation:
			return fmt.Errorf("%w: %s", ErrForeignKeyViolation, pgErr.Detail)
		case PgErrCodeCheckViolation:
			return fmt.Errorf("check constraint violation: %s", pgErr.Detail)
		}
	}

	return err
}

// IsNotFound checks if error is ErrNotFound
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound) || errors.Is(err, pgx.ErrNoRows)
}

// IsDuplicateKey checks if error is ErrDuplicateKey
func IsDuplicateKey(err error) bool {
	return errors.Is(err, ErrDuplicateKey)
}

// IsForeignKeyViolation checks if error is ErrForeignKeyViolation
func IsForeignKeyViolation(err error) bool {
	return errors.Is(err, ErrForeignKeyViolation)
}
