package anomalies

import (
	"context"
	"fmt"

	"order-monitoring/models"

	"github.com/jackc/pgx/v4/pgxpool"
)

// 1-qoida: xaridorga topshirilgan orderning hamma qutilari ham topshirilgan bo'ladi.
// Quti qaytarilsa (return_id), order lifecycle RETURN ga o'tadi — RETURNED_TO_WAREHOUSE orderlarda shunday.
// Bu yerda esa order hali OUT / ISSUED_TO_BUYER, qaytgan quti esa omborda (location STAGING).
const (
	issuedOrderPackageReturnedRule        = "issued_order_package_returned"
	issuedOrderPackageReturnedDescription = "Xaridorga topshirilgan orderning qutisi omborga qaytgan, lekin order hali 'topshirilgan' holatida"
	issuedOrderPackageReturnedSeverity    = severityHigh
)

type IssuedOrderPackageReturned struct {
	db *pgxpool.Pool
}

func NewIssuedOrderPackageReturnedRepo(db *pgxpool.Pool) *IssuedOrderPackageReturned {
	return &IssuedOrderPackageReturned{
		db: db,
	}
}

func (i *IssuedOrderPackageReturned) IssuedOrderPackageReturnedRule(ctx context.Context) (*models.RuleItem, error) {
	resp := &models.RuleItem{
		Rule:        issuedOrderPackageReturnedRule,
		Description: issuedOrderPackageReturnedDescription,
		Severity:    issuedOrderPackageReturnedSeverity,
		Items:       []models.Item{},
	}

	query := `
		SELECT
			COALESCE(o.order_code, o.group_no, o.id),
			COALESCE(o.warehouse_status, ''),
			COALESCE(o.location_kind, 'yo''q'),
			string_agg(
				COALESCE(p.package_no, p.id) || ' ' || COALESCE(p.status, 'NULL')
					|| CASE WHEN p.return_id IS NOT NULL THEN ' (return)' ELSE '' END,
				', ' ORDER BY p.package_no
			)
		FROM orders o
		INNER JOIN packages p ON p.order_group_id = o.id
		WHERE o.lifecycle_outcome = $1
			AND p.status IS DISTINCT FROM $1
		GROUP BY o.id
		ORDER BY o.created_at
	`

	rows, err := i.db.Query(ctx, query, issuedToBuyer)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			orderCode       string
			warehouseStatus string
			location        string
			packages        string
		)

		if err := rows.Scan(
			&orderCode,
			&warehouseStatus,
			&location,
			&packages,
		); err != nil {
			return nil, err
		}

		resp.Items = append(resp.Items, models.Item{
			OrderCode: orderCode,
			Reason: fmt.Sprintf(
				"lifecycle_outcome=%s, lekin topshirilmagan qutilar: %s; order warehouse_status=%s, location=%s",
				issuedToBuyer,
				packages,
				warehouseStatus,
				location,
			),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	resp.Total = len(resp.Items)

	return resp, nil
}
