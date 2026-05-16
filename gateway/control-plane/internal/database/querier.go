package database

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Row interface {
	Scan(dest ...interface{}) error
}

type Rows interface {
	Next() bool
	Scan(dest ...interface{}) error
	Close()
}

type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...interface{}) Row
	Query(ctx context.Context, sql string, args ...interface{}) (Rows, error)
	Exec(ctx context.Context, sql string, args ...interface{}) (CommandTag, error)
}

type CommandTag struct {
	RowsAffected int64
}

type pgxRowWrapper struct {
	row pgx.Row
}

func (r *pgxRowWrapper) Scan(dest ...interface{}) error {
	return r.row.Scan(dest...)
}

type pgxRowsWrapper struct {
	rows pgx.Rows
}

func (r *pgxRowsWrapper) Next() bool {
	return r.rows.Next()
}

func (r *pgxRowsWrapper) Scan(dest ...interface{}) error {
	return r.rows.Scan(dest...)
}

func (r *pgxRowsWrapper) Close() {
	r.rows.Close()
}

type PoolQuerier struct {
	Pool *pgxpool.Pool
}

func (q *PoolQuerier) QueryRow(ctx context.Context, sql string, args ...interface{}) Row {
	return &pgxRowWrapper{row: q.Pool.QueryRow(ctx, sql, args...)}
}

func (q *PoolQuerier) Query(ctx context.Context, sql string, args ...interface{}) (Rows, error) {
	rows, err := q.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return &pgxRowsWrapper{rows: rows}, nil
}

func (q *PoolQuerier) Exec(ctx context.Context, sql string, args ...interface{}) (CommandTag, error) {
	tag, err := q.Pool.Exec(ctx, sql, args...)
	return CommandTag{RowsAffected: tag.RowsAffected()}, err
}