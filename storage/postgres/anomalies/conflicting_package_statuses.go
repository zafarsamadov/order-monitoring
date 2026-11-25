package anomalies

import (
	"context"
	"fmt"

	"order-monitoring/models"

	"github.com/jackc/pgx/v4/pgxpool"
)

// 3-qoida: bitta order qutilarining statuslari bir-biriga zid bo'lmaydi.
// Bitta orderda bir vaqtda ISSUED_TO_BUYER va RETURNED_TO_SELLERS qutilar bo'lmaydi.
const (
	conflictingPackageStatusesRule        = "conflicting_package_statuses"
	conflictingPackageStatusesDescription = "Bitta orderda ham xaridorga topshirilgan, ham sotuvchiga qaytarilgan qutilar bor"
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
			string_agg(DISTINCT p.status, ', ' ORDER BY p.status)
		FROM orders o
		INNER JOIN packages p ON p.order_group_id = o.id
		GROUP BY o.id
		HAVING bool_or(p.status = $1)
			AND bool_or(p.status = $2)
		ORDER BY o.created_at
	`

	rows, err := c.db.Query(ctx, query, issuedToBuyer, returnedToSellers)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			orderCode string
			statuses  string
		)

		if err := rows.Scan(
			&orderCode,
			&statuses,
		); err != nil {
			return nil, err
		}

		resp.Items = append(resp.Items, models.Item{
			OrderCode: orderCode,
			Reason: fmt.Sprintf(
				"qutilar statuslari: %s",
				statuses,
			),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	resp.Total = len(resp.Items)

	return resp, nil
}
