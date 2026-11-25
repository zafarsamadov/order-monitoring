package anomalies

import (
	"context"
	"fmt"
	"order-monitoring/models"

	"github.com/jackc/pgx/v4/pgxpool"
)

type ReturnedToSellerInWarehouse struct {
	db *pgxpool.Pool
}

func NewReturnedToSellerInWarehouseRepo(db *pgxpool.Pool) *ReturnedToSellerInWarehouse {
	return &ReturnedToSellerInWarehouse{
		db: db,
	}
}

func (r *ReturnedToSellerInWarehouse) ReturnedToSellerInWarehouseRule(ctx context.Context) (*models.RuleItem, error) {
	resp := &models.RuleItem{
		Rule:        "returned_to_seller_in_warehouse",
		Description: "Xaridorga topshirilgan orderning ba'zi qutilari topshirilmagan (masalan omborga qaytgan)",
		Severity:    "high",
		Items:       []models.Item{},
	}

	query := `
		SELECT
			COALESCE(o.order_code, o.group_no, o.id),
			string_agg(
				COALESCE(p.package_no, p.id) || ' ' || COALESCE(p.status, 'NULL'),
				', ' ORDER BY p.package_no
			)
		FROM orders o
		INNER JOIN packages p ON p.order_group_id = o.id
		WHERE o.lifecycle_outcome = $1
			AND p.status IS DISTINCT FROM $1
		GROUP BY o.id
		ORDER BY o.created_at
	`

	rows, err := r.db.Query(ctx, query, issuedOutcome)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			orderCode string
			packages  string
		)

		if err := rows.Scan(
			&orderCode,
			&packages,
		); err != nil {
			return nil, err
		}

		resp.Items = append(resp.Items, models.Item{
			OrderCode: orderCode,
			Reason: fmt.Sprintf(
				"lifecycle_outcome=%s, topshirilmagan qutilar: %s",
				issuedOutcome,
				packages,
			),
		})
	}

	resp.Total = len(resp.Items)

	return resp, nil
}
