package postgres

import (
	"context"
	"fmt"

	"order-monitoring/migration"
	"order-monitoring/storage"

	"github.com/jackc/pgx/v4/pgxpool"
)

type Store struct {
	db   *pgxpool.Pool
	sync storage.SyncRepoI
}

// NewPostgres bazaga ulanadi va migratsiyalarni qo'llaydi.
func NewPostgres(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.Connect(ctx, databaseURL)
	if err != nil {
		// pgx xato matniga host/user qo'shadi, lekin parolni emas.
		return nil, fmt.Errorf("postgres ulanish: %w", err)
	}

	if err := migration.Up(ctx, pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migratsiya: %w", err)
	}

	return &Store{db: pool}, nil
}

func (s *Store) CloseDB() {
	s.db.Close()
}

// Pool testlar va read-only so'rovlar uchun.
func (s *Store) Pool() *pgxpool.Pool {
	return s.db
}

func (s *Store) Sync() storage.SyncRepoI {
	if s.sync == nil {
		s.sync = NewSyncRepo(s.db)
	}

	return s.sync
}
