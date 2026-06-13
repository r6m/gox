// Package pgxutil provides small pgx pool and transaction helpers.
package pgxutil

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OpenPool creates and verifies a pgx connection pool.
func OpenPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

// Ping verifies that a pool can reach Postgres.
func Ping(ctx context.Context, pool *pgxpool.Pool) error {
	return pool.Ping(ctx)
}

// Tx runs fn in a transaction, committing on success and rolling back on error.
func Tx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	_, err := TxValue(ctx, pool, func(tx pgx.Tx) (struct{}, error) {
		return struct{}{}, fn(tx)
	})
	return err
}

// TxValue runs fn in a transaction and returns its value after a successful
// commit. Callback errors trigger rollback and commit errors are returned.
func TxValue[T any](ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) (T, error)) (T, error) {
	var zero T
	tx, err := pool.Begin(ctx)
	if err != nil {
		return zero, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	value, err := fn(tx)
	if err != nil {
		return zero, err
	}
	if err := tx.Commit(ctx); err != nil {
		return zero, err
	}
	return value, nil
}

// Text converts a nullable string pointer to pgtype.Text.
func Text(value *string) pgtype.Text {
	if value == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *value, Valid: true}
}

// TextPtr converts pgtype.Text to a nullable string pointer.
func TextPtr(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

// Int4 converts a nullable int32 pointer to pgtype.Int4.
func Int4(value *int32) pgtype.Int4 {
	if value == nil {
		return pgtype.Int4{}
	}
	return pgtype.Int4{Int32: *value, Valid: true}
}

// Int4Ptr converts pgtype.Int4 to a nullable int32 pointer.
func Int4Ptr(value pgtype.Int4) *int32 {
	if !value.Valid {
		return nil
	}
	return &value.Int32
}

// Int8 converts a nullable int64 pointer to pgtype.Int8.
func Int8(value *int64) pgtype.Int8 {
	if value == nil {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: *value, Valid: true}
}

// Int8Ptr converts pgtype.Int8 to a nullable int64 pointer.
func Int8Ptr(value pgtype.Int8) *int64 {
	if !value.Valid {
		return nil
	}
	return &value.Int64
}

// Bool converts a nullable bool pointer to pgtype.Bool.
func Bool(value *bool) pgtype.Bool {
	if value == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *value, Valid: true}
}

// BoolPtr converts pgtype.Bool to a nullable bool pointer.
func BoolPtr(value pgtype.Bool) *bool {
	if !value.Valid {
		return nil
	}
	return &value.Bool
}

// Timestamptz converts a nullable time pointer to pgtype.Timestamptz.
func Timestamptz(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *value, Valid: true}
}

// TimestamptzPtr converts pgtype.Timestamptz to a nullable time pointer.
func TimestamptzPtr(value pgtype.Timestamptz) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}

// Date converts a nullable time pointer to pgtype.Date.
func Date(value *time.Time) pgtype.Date {
	if value == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: *value, Valid: true}
}

// DatePtr converts pgtype.Date to a nullable time pointer.
func DatePtr(value pgtype.Date) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}

// UUID converts a nullable 16-byte UUID pointer to pgtype.UUID.
func UUID(value *[16]byte) pgtype.UUID {
	if value == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *value, Valid: true}
}

// UUIDPtr converts pgtype.UUID to a nullable 16-byte UUID pointer.
func UUIDPtr(value pgtype.UUID) *[16]byte {
	if !value.Valid {
		return nil
	}
	return &value.Bytes
}
