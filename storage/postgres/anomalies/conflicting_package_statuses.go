package anomalies

import (
	"context"
	"fmt"

	"order-monitoring/models"

	"github.com/jackc/pgx/v4/pgxpool"
)

// 2-qoida: qaytarish hodisalari tartib bilan bo'ladi: xaridor → hub → sotuvchi.
// Order RETURN_TO_HUB ("kuryer qaytarishni olib kelyapti") bo'lsa, qaytarilgan quti (return_id)
// hali sotuvchiga yetib bormagan bo'ladi.
// Eslatma: bitta orderda ISSUED_TO_BUYER va RETURNED_TO_SELLERS qutilar bo'lishi o'zi normal (qisman qaytish).
const (
	conflictingPackageStatusesRule        = "conflicting_package_statuses"
	conflictingPackageStatusesDescription = "Kuryer qaytarishni hali hub'ga olib kelyapti, lekin qaytarilgan quti allaqachon sotuvchida"
	conflictingPackageStatusesSeverity    = severityMedium
)

type ConflictingPackageStatuses struct {
	db *pgxpool.Pool
}

func NewConflictingPackageStatusesRepo(db *pgxpool.Pool) *ConflictingPackageStatuses {
	return &ConflictingPackageStatuses{
		db: db,
	}
}

func (c *ConflictingPackageStatuses) ConflictingPackageStatusesRule(ctx context.Context) (*models.RuleItem, error) {
	resp := &models.RuleItem{
		Rule:        conflictingPackageStatusesRule,
		Description: conflictingPackageStatusesDescription,
		Severity:    conflictingPackageStatusesSeverity,
		Items:       []models.Item{},
	}

	query := `
		SELECT
			COALESCE(o.order_code, o.group_no, o.id),
			string_agg(
				COALESCE(p.package_no, p.id) || ' ' || p.status,
				', ' ORDER BY p.package_no
			)
		FROM orders o
		INNER JOIN packages p ON p.order_group_id = o.id
		WHERE o.lifecycle_outcome = $1
			AND p.return_id IS NOT NULL
			AND p.status = $2
		GROUP BY o.id
		ORDER BY o.created_at
	`

	rows, err := c.db.Query(ctx, query, returnToHub, returnedToSellers)
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
				"lifecycle_outcome=%s, lekin qaytarilgan quti allaqachon sotuvchida: %s",
				returnToHub,
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
