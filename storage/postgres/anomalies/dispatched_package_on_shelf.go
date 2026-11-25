package anomalies

import (
	"context"
	"fmt"
	"order-monitoring/models"

	"github.com/jackc/pgx/v4/pgxpool"
)

const (
	dispatchedOutcome = "DISPATCHED"
)

type DispatchedPackageOnShelf struct {
	db *pgxpool.Pool
}

func NewDispatchedPackageOnShelfRepo(db *pgxpool.Pool) *DispatchedPackageOnShelf {
	return &DispatchedPackageOnShelf{
		db: db,
	}
}

func (d *DispatchedPackageOnShelf) DispatchedPackageOnShelfRule(ctx context.Context) (*models.RuleItem, error) {

	resp := &models.RuleItem{
		Rule:        "dispatched_package_on_shelf",
		Description: "Qutilar javonlarda joylashgan bo'lsa ham, ularni xaridorga topshirish uchun yetkazib beruvchi tomonidan olib ketilmagan",
		Severity:    "medium",
		Items:       []models.Item{},
	}

	query := `
		SELECT
			COALESCE(o.order_code, o.group_no, o.id),
			string_agg(
				COALESCE(p.package_no, p.id) || ' ' || COALESCE(p.shelf_code, 'NULL'),
				', ' ORDER BY p.package_no
			)
		FROM orders o
		INNER JOIN packages p ON p.order_group_id = o.id
		WHERE p.status = $1
			AND p.shelf_code IS NOT NULL
		GROUP BY o.id
		ORDER BY o.created_at
	`

	rows, err := d.db.Query(ctx, query, dispatchedOutcome)
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
				"lifecycle_outcome=%s, javonlardagi qutilar: %s",
				dispatchedOutcome,
				packages,
			),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	resp.Total = len(resp.Items)

	return resp, nil
}
