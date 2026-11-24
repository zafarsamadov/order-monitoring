package migration

import (
	"context"
	"embed"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	"order-monitoring/logger"

	"github.com/jackc/pgx/v4/pgxpool"
)

//go:embed up/*.sql
var upFS embed.FS

// Up hali qo'llanmagan up/*.sql fayllarni tartib bilan qo'llaydi.
// rider-backend Makefile'idagi mantiq, faqat ilova start'ida Go ichida ishlaydi.
func Up(ctx context.Context, db *pgxpool.Pool) error {
	if _, err := db.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    int PRIMARY KEY,
		applied_at timestamptz NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("schema_migrations: %w", err)
	}

	entries, err := upFS.ReadDir("up")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		version, err := strconv.Atoi(strings.SplitN(name, ".", 2)[0])
		if err != nil {
			return fmt.Errorf("migratsiya nomi noto'g'ri: %s", name)
		}

		var applied bool
		if err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, version).Scan(&applied); err != nil {
			return err
		}
		if applied {
			continue
		}

		sql, err := upFS.ReadFile(path.Join("up", name))
		if err != nil {
			return err
		}

		// Migratsiya va uning qaydi bitta tranzaksiyada: yarim qo'llangan holat qolmaydi.
		tx, err := db.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(sql)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("%s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		logger.Infof("migratsiya qo'llandi: %s", name)
	}

	return nil
}
