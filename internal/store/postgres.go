package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

func openPostgres(ctx context.Context, databaseURL string) (Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	s := &sqlStore{
		db:     stdlib.OpenDBFromPool(pool),
		driver: DriverPostgres,
		rebind: identity,
	}
	if err := s.Ping(ctx); err != nil {
		if closeErr := s.Close(); closeErr != nil {
			return nil, errors.Join(err, fmt.Errorf("close pool: %w", closeErr))
		}
		return nil, err
	}
	return s, nil
}
