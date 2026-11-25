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

// migrationLockID — pg_advisory_lock kaliti (ixtiyoriy, loyiha uchun o'zgarmas son).
const migrationLockID = 7_310_001

// Up hali qo'llanmagan up/*.sql fayllarni tartib bilan qo'llaydi.
// rider-backend Makefile'idagi mantiq, faqat ilova start'ida Go ichida ishlaydi.
func Up(ctx context.Context, db *pgxpool.Pool) error {
	// Bir vaqtda bir nechta jarayon (ilova nusxalari, parallel testlar) migratsiya qilsa,
	// CREATE TABLE to'qnashadi. Advisory lock ularni navbatga qo'yadi. Lock ulanishga bog'liq,
	// shuning uchun hammasi bitta ulanishda bajariladi.
	conn, err := db.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrationLockID); err != nil {
		return fmt.Errorf("migratsiya lock: %w", err)
	}
	defer conn.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, migrationLockID) //nolint:errcheck

	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
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
		if err := conn.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, version).Scan(&applied); err != nil {
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
		tx, err := conn.Begin(ctx)
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
