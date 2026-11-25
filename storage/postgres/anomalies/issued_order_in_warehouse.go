package rules

import (
	"context"
	"fmt"

	"order-monitoring/models"

	"github.com/jackc/pgx/v4/pgxpool"
)

// 1-qoida: xaridorga topshirilgan order omborda turmaydi.
// lifecycle_outcome = ISSUED_TO_BUYER bo'lsa, warehouse_status READY_TO_SHIP bo'lmaydi
// va omborda joylashuvi (location) bo'lmaydi.
const (
	issuedOrderInWarehouseRule        = "issued_order_in_warehouse"
	issuedOrderInWarehouseDescription = "Xaridorga topshirilgan order hali jo'natishga tayyor holatda yoki omborda turibdi"
	issuedOrderInWarehouseSeverity    = "high"

	issuedOutcome     = "ISSUED_TO_BUYER"
	readyToShipStatus = "READY_TO_SHIP"
)

type IssuedOrderInWarehouse struct {
	db *pgxpool.Pool
}

func NewIssuedOrderInWarehouseRepo(db *pgxpool.Pool) *IssuedOrderInWarehouse {
	return &IssuedOrderInWarehouse{
		db: db,
	}
}

func (i *IssuedOrderInWarehouse) IssuedOrderInWarehouseRule(ctx context.Context) (*models.RuleItem, error) {
	result := &models.RuleItem{
		Rule:        issuedOrderInWarehouseRule,
		Description: issuedOrderInWarehouseDescription,
		Severity:    issuedOrderInWarehouseSeverity,
		Items:       []models.Item{},
	}

	query := `
		SELECT
			COALESCE(order_code, group_no, id),
			COALESCE(warehouse_status, ''),
			COALESCE(location_kind, ''),
			COALESCE(location_label, '')
		FROM orders
		WHERE lifecycle_outcome = $1
			AND (warehouse_status = $2 OR location_kind IS NOT NULL)
		ORDER BY created_at
	`

	rows, err := i.db.Query(ctx, query, issuedOutcome, readyToShipStatus)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			orderCode       string
			warehouseStatus string
			locationKind    string
			locationLabel   string
		)

		if err := rows.Scan(
			&orderCode,
			&warehouseStatus,
			&locationKind,
			&locationLabel,
		); err != nil {
			return nil, err
		}

		location := "yo'q"
		if locationKind != "" {
			location = locationKind + " " + locationLabel
		}

		result.Items = append(result.Items, models.Item{
			OrderCode: orderCode,
			Reason: fmt.Sprintf(
				"lifecycle_outcome=%s, warehouse_status=%s, location=%s",
				issuedOutcome,
				warehouseStatus,
				location,
			),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	result.Total = len(result.Items)

	return result, nil
}
