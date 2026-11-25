package postgres

import (
	"context"
	"fmt"

	"order-monitoring/migration"
	"order-monitoring/storage"
	"order-monitoring/storage/postgres/anomalies"

	"github.com/jackc/pgx/v4/pgxpool"
)

type Store struct {
	db        *pgxpool.Pool
	sync      storage.SyncRepoI
	order     storage.OrderRepoI
	anomalies storage.AnomaliesRepoI
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

	// Repolar shu yerda yaratiladi: lazy init parallel so'rovlarda data race berardi.
	return &Store{
		db:        pool,
		sync:      NewSyncRepo(pool),
		order:     NewOrderRepo(pool),
		anomalies: anomalies.NewRepo(pool),
	}, nil
}

func (s *Store) CloseDB() {
	s.db.Close()
}

// Pool testlar va read-only so'rovlar uchun.
func (s *Store) Pool() *pgxpool.Pool {
	return s.db
}

func (s *Store) Sync() storage.SyncRepoI {
	return s.sync
}

func (s *Store) Order() storage.OrderRepoI {
	return s.order
}

func (s *Store) Anomalies() storage.AnomaliesRepoI {
	return s.anomalies
}
