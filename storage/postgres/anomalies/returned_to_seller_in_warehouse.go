package anomalies

import (
	"context"
	"fmt"

	"order-monitoring/models"

	"github.com/jackc/pgx/v4/pgxpool"
)

// 4-qoida: sotuvchiga qaytarilgan tovar omborda turmaydi.
// lifecycle_outcome = RETURNED_TO_SELLERS bo'lsa, orderda location bo'lmaydi va qutisida shelf_code bo'lmaydi.
const (
	returnedToSellerInWarehouseRule        = "returned_to_seller_in_warehouse"
	returnedToSellerInWarehouseDescription = "Sotuvchiga qaytarilgan order hali omborda joylashuvga ega yoki qutisi javonda turibdi"
	returnedToSellerInWarehouseSeverity    = severityMedium
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
		Rule:        returnedToSellerInWarehouseRule,
		Description: returnedToSellerInWarehouseDescription,
		Severity:    returnedToSellerInWarehouseSeverity,
		Items:       []models.Item{},
	}

	query := `
		SELECT
			COALESCE(o.order_code, o.group_no, o.id),
			COALESCE(o.location_kind || ' ' || COALESCE(o.location_label, ''), 'yo''q'),
			COALESCE(
				string_agg(
					COALESCE(p.package_no, p.id) || ' ' || p.shelf_code,
					', ' ORDER BY p.package_no
				),
				'yo''q'
			)
		FROM orders o
		LEFT JOIN packages p ON p.order_group_id = o.id AND p.shelf_code IS NOT NULL
		WHERE o.lifecycle_outcome = $1
		GROUP BY o.id
		HAVING o.location_kind IS NOT NULL
			OR count(p.id) > 0
		ORDER BY o.created_at
	`

	rows, err := r.db.Query(ctx, query, returnedToSellers)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			orderCode string
			location  string
			packages  string
		)

		if err := rows.Scan(
			&orderCode,
			&location,
			&packages,
		); err != nil {
			return nil, err
		}

		resp.Items = append(resp.Items, models.Item{
			OrderCode: orderCode,
			Reason: fmt.Sprintf(
				"lifecycle_outcome=%s, location=%s, javondagi qutilar: %s",
				returnedToSellers,
				location,
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
