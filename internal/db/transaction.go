package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TxFunc is a function that executes within a database transaction
// If the function returns an error, the transaction will be rolled back
// If the function succeeds (returns nil), the transaction will be committed
type TxFunc func(tx pgx.Tx) error

// WithTransaction executes a function within a database transaction
// Automatically handles commit on success and rollback on error or panic
//
// Example usage:
//   err := db.WithTransaction(ctx, func(tx pgx.Tx) error {
//       // Create tenant
//       _, err := tx.Exec(ctx, "INSERT INTO tenants ...")
//       if err != nil {
//           return err  // Will trigger rollback
//       }
//       // Create environment vars
//       _, err = tx.Exec(ctx, "INSERT INTO tenant_environment ...")
//       return err  // nil = commit, error = rollback
//   })
func (db *DB) WithTransaction(ctx context.Context, fn TxFunc) error {
	// Begin transaction
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Ensure rollback on panic
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p) // Re-throw panic after rollback
		}
	}()

	// Execute function within transaction
	if err := fn(tx); err != nil {
		// Rollback on error
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("transaction error: %w, rollback error: %v", err, rbErr)
		}
		return err
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// TransactionManager provides manual transaction management
// Use this when you need more control over transaction lifecycle
type TransactionManager struct {
	pool *pgxpool.Pool
}

// NewTransactionManager creates a new transaction manager
func NewTransactionManager(pool *pgxpool.Pool) *TransactionManager {
	return &TransactionManager{pool: pool}
}

// BeginTx starts a new transaction
func (tm *TransactionManager) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return tm.pool.Begin(ctx)
}

// CommitTx commits a transaction
func (tm *TransactionManager) CommitTx(ctx context.Context, tx pgx.Tx) error {
	return tx.Commit(ctx)
}

// RollbackTx rolls back a transaction
func (tm *TransactionManager) RollbackTx(ctx context.Context, tx pgx.Tx) error {
	return tx.Rollback(ctx)
}
