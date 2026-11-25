package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"order-monitoring/models"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type syncRepo struct {
	db *pgxpool.Pool
}

func NewSyncRepo(db *pgxpool.Pool) *syncRepo {
	return &syncRepo{db: db}
}

// execer — pool ham, tranzaksiya ham.
type execer interface {
	Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
}

func (r *syncRepo) MarkInterrupted(ctx context.Context) error {
	query := `
		UPDATE sync_runs
		SET
			status      = $1,
			finished_at = now(),
			error       = 'ilova qayta ishga tushdi, sync uzilib qolgan'
		WHERE status = $2
	`

	_, err := r.db.Exec(
		ctx,
		query,
		models.SyncFailed,
		models.SyncRunning,
	)

	return err
}

func (r *syncRepo) StartRun(ctx context.Context) (*models.SyncRun, error) {
	run := &models.SyncRun{
		Status:   models.SyncRunning,
		Entities: []models.EntityResult{},
	}

	query := `
		INSERT INTO
			sync_runs(status)
		VALUES($1)
		RETURNING
			id,
			started_at
	`

	err := r.db.QueryRow(ctx, query, models.SyncRunning).Scan(
		&run.ID,
		&run.StartedAt,
	)
	if err != nil {
		return nil, err
	}

	return run, nil
}

func (r *syncRepo) SavePage(ctx context.Context, run *models.SyncRun, records []models.Record) (models.UpsertStats, error) {
	var stats models.UpsertStats

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return stats, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // commit bo'lgach rollback hech narsa qilmaydi

	for _, rec := range records {
		result, err := upsert(ctx, tx, rec)
		if err != nil {
			return models.UpsertStats{}, fmt.Errorf("%s %s: %w", rec.Table, rec.Key, err)
		}

		switch result {
		case models.UpsertInserted:
			stats.Inserted++
		case models.UpsertUpdated:
			stats.Updated++
		default:
			stats.Unchanged++
		}
	}

	// Run holati sahifa bilan bitta tranzaksiyada saqlanadi: uzilish bo'lsa ikkalasi mos qoladi.
	// Nusxada hisoblaymiz va faqat commit muvaffaqiyatli bo'lsa run'ga ko'chiramiz.
	snapshot := *run
	snapshot.Entities = append([]models.EntityResult(nil), run.Entities...)
	if n := len(snapshot.Entities); n > 0 {
		snapshot.Entities[n-1].UpsertStats.Add(stats)
	}
	snapshot.Recount()

	if err := saveRun(ctx, tx, &snapshot); err != nil {
		return models.UpsertStats{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return models.UpsertStats{}, err
	}

	*run = snapshot
	return stats, nil
}

func (r *syncRepo) FinishRun(ctx context.Context, run *models.SyncRun) error {
	return saveRun(ctx, r.db, run)
}

func (r *syncRepo) LastRun(ctx context.Context) (*models.SyncRun, error) {
	var (
		run      models.SyncRun
		entities []byte
	)

	query := `
		SELECT
			id,
			status,
			started_at,
			finished_at,
			inserted,
			updated,
			unchanged,
			last_entity,
			last_offset,
			entities,
			error
		FROM sync_runs
		ORDER BY id DESC
		LIMIT 1
	`

	err := r.db.QueryRow(ctx, query).Scan(
		&run.ID,
		&run.Status,
		&run.StartedAt,
		&run.FinishedAt,
		&run.Inserted,
		&run.Updated,
		&run.Unchanged,
		&run.LastEntity,
		&run.LastOffset,
		&entities,
		&run.Error,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(entities, &run.Entities); err != nil {
		return nil, err
	}

	return &run, nil
}

func saveRun(ctx context.Context, db execer, run *models.SyncRun) error {
	entities, err := json.Marshal(run.Entities)
	if err != nil {
		return err
	}

	query := `
		UPDATE sync_runs
		SET
			status      = $2,
			finished_at = $3,
			inserted    = $4,
			updated     = $5,
			unchanged   = $6,
			last_entity = $7,
			last_offset = $8,
			entities    = $9,
			error       = $10
		WHERE id = $1
	`

	_, err = db.Exec(
		ctx,
		query,
		run.ID,
		run.Status,
		run.FinishedAt,
		run.Inserted,
		run.Updated,
		run.Unchanged,
		run.LastEntity,
		run.LastOffset,
		entities,
		run.Error,
	)

	return err
}

// upsertQuery shabloni. Jadval va ustun nomlari kod ichidagi konstantalardan keladi
// (service/sync_records.go), foydalanuvchi kiritgan matn emas; qiymatlar $n parametr bo'lib boradi.
//
// xmax = 0 bo'lsa qator yangi qo'shilgan, aks holda yangilangan.
// WHERE sharti hash teng bo'lsa UPDATE'ni o'tkazib yuboradi va hech qator qaytmaydi.
const upsertQuery = `
		INSERT INTO
			%[1]s(
				%[2]s
			)
		VALUES(%[3]s)
		ON CONFLICT (%[4]s) DO UPDATE
		SET
			%[5]s
		WHERE %[1]s.raw_hash <> EXCLUDED.raw_hash
		RETURNING (xmax = 0)
	`

const touchQuery = `
		UPDATE %[1]s
		SET last_synced_at = now()
		WHERE %[2]s = $1
	`

// upsert yozuvni tabiiy kalit bo'yicha qo'shadi yoki yangilaydi.
// raw_hash o'zgarmagan bo'lsa ma'lumot ustunlariga tegilmaydi, faqat last_synced_at yangilanadi.
func upsert(ctx context.Context, tx pgx.Tx, rec models.Record) (models.UpsertResult, error) {
	cols := append([]string{rec.KeyCol}, rec.Cols...)
	cols = append(cols, "raw", "raw_hash")

	args := append([]any{rec.Key}, rec.Vals...)
	args = append(args, rec.Raw, rec.Hash)

	placeholders := make([]string, len(cols))
	for i := range cols {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}

	sets := make([]string, 0, len(cols))
	for _, c := range cols[1:] {
		sets = append(sets, fmt.Sprintf("%s = EXCLUDED.%s", c, c))
	}
	sets = append(sets, "last_synced_at = now()")

	query := fmt.Sprintf(
		upsertQuery,
		rec.Table,
		strings.Join(cols, ",\n\t\t\t\t"),
		strings.Join(placeholders, ", "),
		rec.KeyCol,
		strings.Join(sets, ",\n\t\t\t"),
	)

	var isNew bool
	err := tx.QueryRow(ctx, query, args...).Scan(&isNew)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		_, err = tx.Exec(ctx, fmt.Sprintf(touchQuery, rec.Table, rec.KeyCol), rec.Key)
		return models.UpsertUnchanged, err
	case err != nil:
		return models.UpsertUnchanged, err
	case isNew:
		return models.UpsertInserted, nil
	default:
		return models.UpsertUpdated, nil
	}
}
