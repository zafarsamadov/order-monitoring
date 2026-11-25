package postgres

import (
	"context"
	"errors"

	"order-monitoring/models"
	"order-monitoring/storage"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type orderRepo struct {
	db *pgxpool.Pool
}

func NewOrderRepo(db *pgxpool.Pool) *orderRepo {
	return &orderRepo{db: db}
}

// GetByCode orderni avval order_code, keyin group_no, keyin id bo'yicha qidiradi:
// ko'p orderlarda order_code yo'q.
func (o *orderRepo) GetByCode(ctx context.Context, code string) (*models.OrderTimelineRow, error) {
	query := `
		SELECT
			id,
			COALESCE(order_code, group_no, id),
			COALESCE(lifecycle, ''),
			COALESCE(lifecycle_outcome, ''),
			COALESCE(warehouse_status, ''),
			COALESCE(raw->>'current_step', current_step_code, ''),
			COALESCE(location_kind, ''),
			COALESCE(location_label, ''),
			created_at,
			lifecycle_since,
			location_since,
			last_activity_at
		FROM orders
		WHERE order_code = $1 OR group_no = $1 OR id = $1
		ORDER BY
			CASE
				WHEN order_code = $1 THEN 0
				WHEN group_no = $1 THEN 1
				ELSE 2
			END
		LIMIT 1
	`

	var r models.OrderTimelineRow
	err := o.db.QueryRow(ctx, query, code).Scan(
		&r.ID,
		&r.Code,
		&r.Lifecycle,
		&r.LifecycleOutcome,
		&r.WarehouseStatus,
		&r.CurrentStep,
		&r.LocationKind,
		&r.LocationLabel,
		&r.CreatedAt,
		&r.LifecycleSince,
		&r.LocationSince,
		&r.LastActivityAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, storage.ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}

	return &r, nil
}

// GetPackages orderning qutilari (javondagi tile bilan birga).
func (o *orderRepo) GetPackages(ctx context.Context, orderID string) ([]models.PackageTimelineRow, error) {
	query := `
		SELECT
			p.id,
			COALESCE(p.package_no, ''),
			COALESCE(p.status, ''),
			COALESCE(p.shelf_code, ''),
			COALESCE(p.held_by, ''),
			p.packed_at,
			p.held_since,
			COALESCE(t.shelf_code, ''),
			t.sorted_at
		FROM packages p
		LEFT JOIN shelf_tiles t ON t.package_id = p.id
		WHERE p.order_group_id = $1
		ORDER BY p.package_no, p.id
	`

	rows, err := o.db.Query(ctx, query, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.PackageTimelineRow
	for rows.Next() {
		var p models.PackageTimelineRow
		if err := rows.Scan(
			&p.ID,
			&p.PackageNo,
			&p.Status,
			&p.ShelfCode,
			&p.HeldBy,
			&p.PackedAt,
			&p.HeldSince,
			&p.TileShelf,
			&p.SortedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, p)
	}

	return out, rows.Err()
}

// GetHandovers orderning handover'lari.
func (o *orderRepo) GetHandovers(ctx context.Context, orderID string) ([]models.HandoverTimelineRow, error) {
	query := `
		SELECT
			COALESCE(handover_no, id),
			COALESCE(shop_name, ''),
			COALESCE(item_count, 0),
			COALESCE(expected_qty, 0),
			created_at,
			transit_since,
			received_at
		FROM handovers
		WHERE order_group_id = $1
		ORDER BY created_at, id
	`

	rows, err := o.db.Query(ctx, query, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.HandoverTimelineRow
	for rows.Next() {
		var h models.HandoverTimelineRow
		if err := rows.Scan(
			&h.HandoverNo,
			&h.ShopName,
			&h.ItemCount,
			&h.ExpectedQty,
			&h.CreatedAt,
			&h.TransitSince,
			&h.ReceivedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, h)
	}

	return out, rows.Err()
}
