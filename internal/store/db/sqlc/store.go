package db

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store interface {
	CreateUser(ctx context.Context, arg CreateUserParams) (User, error)
	GetUserById(ctx context.Context, id uuid.UUID) (User, error)
	GetUserByUsername(ctx context.Context, username string) (User, error)
	ListUsers(ctx context.Context) ([]User, error)
	UpdateUser(ctx context.Context, arg UpdateUserParams) (User, error)
	UpdatePassword(ctx context.Context, id uuid.UUID, password string) error
	CreateRefreshToken(ctx context.Context, arg CreateRefreshTokenParams) (RefreshToken, error)
	GetRefreshToken(ctx context.Context, token string) (RefreshToken, error)
	DeleteRefreshToken(ctx context.Context, token string) error
	DeleteRefreshTokensByUsername(ctx context.Context, username string) error

	InsertOutboxEvent(ctx context.Context, arg InsertOutboxEventParams) (OutboxEvent, error)
	GetOutboxEventById(ctx context.Context, id uuid.UUID) (OutboxEvent, error)
	ListUnpublishedOutboxEvents(ctx context.Context, limit int32) ([]OutboxEvent, error)
	CountUnpublishedOutboxEvents(ctx context.Context) (int64, error)
	MarkOutboxEventPublished(ctx context.Context, id uuid.UUID) error
	DeletePublishedOutboxEvents(ctx context.Context, interval pgtype.Interval) error

	ExecTx(ctx context.Context, fn func(q *Queries) error) error
}

type SQLStore struct {
	connPool *pgxpool.Pool
	*Queries
}

func NewStore(connPool *pgxpool.Pool) Store {
	return &SQLStore{
		connPool: connPool,
		Queries:  New(connPool),
	}
}

func (s *SQLStore) UpdatePassword(ctx context.Context, id uuid.UUID, password string) error {
	_, err := s.connPool.Exec(ctx, "UPDATE users SET password = $2 WHERE id = $1", id, password)
	return err
}

func (s *SQLStore) ExecTx(ctx context.Context, fn func(q *Queries) error) error {
	tx, err := s.connPool.Begin(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback(ctx)
			panic(r)
		}
	}()

	err = fn(s.Queries.WithTx(tx))
	if err != nil {
		rollbackErr := tx.Rollback(ctx)
		if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			return errors.Join(err, rollbackErr)
		}
		return err
	}

	return tx.Commit(ctx)
}
