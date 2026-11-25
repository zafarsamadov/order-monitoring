package anomalies

import (
	"context"
	"fmt"

	"order-monitoring/models"

	"github.com/jackc/pgx/v4/pgxpool"
)

// 3-qoida: sotuvchiga qaytarilgan quti javonda turmaydi.
// packages.status = RETURNED_TO_SELLERS bo'lsa, shelf_code bo'sh bo'ladi.
// Dalil: RET-01 javonining box_count'i faqat RETURNED_TO_WAREHOUSE qutilarni sanaydi.
// Eslatma: order location (STAGING) bu yerda tekshirilmaydi — u barcha RETURN orderlarda bor, stend shunday ishlaydi.
const (
	returnedToSellerInWarehouseRule        = "returned_to_seller_in_warehouse"
	returnedToSellerInWarehouseDescription = "Sotuvchiga qaytarilgan quti hali javonda turibdi deb yozilgan"
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
			string_agg(
				COALESCE(p.package_no, p.id) || ' ' || p.shelf_code,
				', ' ORDER BY p.package_no
			)
		FROM orders o
		INNER JOIN packages p ON p.order_group_id = o.id
		WHERE p.status = $1
			AND p.shelf_code IS NOT NULL
		GROUP BY o.id
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
				"status=%s, lekin javonda: %s",
				returnedToSellers,
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
