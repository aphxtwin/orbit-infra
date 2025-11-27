package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Config holds database connection configuration
// These values will come from environment variables
type Config struct {
	Host            string        // PostgreSQL host (e.g., "localhost" or RDS endpoint)
	Port            int           // PostgreSQL port (default: 5432)
	User            string        // Database user
	Password        string        // Database password
	Database        string        // Database name
	SSLMode         string        // SSL mode: disable, require, verify-ca, verify-full
	MaxConns        int32         // Maximum number of connections in pool
	MinConns        int32         // Minimum number of connections in pool
	MaxConnLifetime time.Duration // Maximum lifetime of a connection
	MaxConnIdleTime time.Duration // Maximum idle time before connection is closed
}

// DB wraps the pgxpool.Pool for dependency injection
// This allows us to pass around a DB instance to repositories
type DB struct {
	Pool *pgxpool.Pool
}

// NewDB creates a new database connection pool
// It takes a context for cancellation and a Config struct
// Returns *DB or an error if connection fails
func NewDB(ctx context.Context, cfg Config) (*DB, error) {
	// Build PostgreSQL connection string from config
	// Format: postgres://user:password@host:port/database?options
	connString := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode,
	)

	// Parse the connection string and create pool configuration
	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	// Configure pool settings for optimal performance
	poolConfig.MaxConns = cfg.MaxConns               // Max concurrent connections (default: 25)
	poolConfig.MinConns = cfg.MinConns               // Min idle connections (default: 5)
	poolConfig.MaxConnLifetime = cfg.MaxConnLifetime // Recycle connections after this time (default: 30m)
	poolConfig.MaxConnIdleTime = cfg.MaxConnIdleTime // Close idle connections after this time (default: 5m)

	// Create the connection pool
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify the connection works by pinging the database
	if err := pool.Ping(ctx); err != nil {
		pool.Close() // Clean up pool if ping fails
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Return wrapped pool
	return &DB{Pool: pool}, nil
}

// Close gracefully closes the database connection pool
// Should be called when shutting down the application
func (db *DB) Close() {
	db.Pool.Close()
}

// Health checks if the database connection is healthy
// Used for health check endpoints
func (db *DB) Health(ctx context.Context) error {
	return db.Pool.Ping(ctx)
}
