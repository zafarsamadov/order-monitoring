package anomalies

import (
	"context"
	"fmt"

	"order-monitoring/models"

	"github.com/jackc/pgx/v4/pgxpool"
)

// 4-qoida: omborda turgan quti faqat mavjud javonda turadi.
// packages.shelf_code bo'lsa, u shelves.code ro'yxatida bo'lishi kerak.
// Ombordan chiqib ketgan qutilar (sotuvchiga, xaridorga, jo'natilgan) bu yerda tekshirilmaydi —
// ularni 3- va 5-qoidalar ko'radi, ikki marta sanalmasin.
const (
	packageOnUnknownShelfRule        = "package_on_unknown_shelf"
	packageOnUnknownShelfDescription = "Omborda turgan quti javonlar ro'yxatida yo'q javonda"
	packageOnUnknownShelfSeverity    = severityHigh
)

type PackageOnUnknownShelf struct {
	db *pgxpool.Pool
}

func NewPackageOnUnknownShelfRepo(db *pgxpool.Pool) *PackageOnUnknownShelf {
	return &PackageOnUnknownShelf{
		db: db,
	}
}

func (p *PackageOnUnknownShelf) PackageOnUnknownShelfRule(ctx context.Context) (*models.RuleItem, error) {
	resp := &models.RuleItem{
		Rule:        packageOnUnknownShelfRule,
		Description: packageOnUnknownShelfDescription,
		Severity:    packageOnUnknownShelfSeverity,
		Items:       []models.Item{},
	}

	query := `
		SELECT
			COALESCE(o.order_code, o.group_no, o.id),
			string_agg(
				COALESCE(p.package_no, p.id) || ' ' || p.shelf_code || ' (' || COALESCE(p.status, 'NULL') || ')',
				', ' ORDER BY p.package_no
			)
		FROM orders o
		INNER JOIN packages p ON p.order_group_id = o.id
		WHERE p.shelf_code IS NOT NULL
			AND p.status NOT IN ($1, $2, $3)
			AND NOT EXISTS (SELECT 1 FROM shelves s WHERE s.code = p.shelf_code)
		GROUP BY o.id
		ORDER BY o.created_at
	`

	rows, err := p.db.Query(ctx, query, returnedToSellers, issuedToBuyer, dispatched)
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
				"noma'lum javondagi qutilar: %s",
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
