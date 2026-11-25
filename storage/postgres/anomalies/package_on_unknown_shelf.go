package anomalies

import (
	"context"
	"fmt"
	"order-monitoring/models"

	"github.com/jackc/pgx/v4/pgxpool"
)

type PackageOnUnknownShelfRepo struct {
	db *pgxpool.Pool
}

func NewPackageOnUnknownShelfRepo(db *pgxpool.Pool) *PackageOnUnknownShelfRepo {
	return &PackageOnUnknownShelfRepo{
		db: db,
	}
}

func (p *PackageOnUnknownShelfRepo) PackageOnUnknownShelfRule(ctx context.Context) (*models.RuleItem, error) {

	resp := &models.RuleItem{
		Rule:        "package_on_unknown_shelf",
		Description: "Qutilar noma'lum javonlarda joylashgan",
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
		WHERE p.shelf_code IS NULL
		GROUP BY o.id
		ORDER BY o.created_at
	`

	rows, err := p.db.Query(ctx, query)
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
				"noma'lum javonlarda joylashgan qutilar: %s",
				packages,
			),
		})
	}

	resp.Total = len(resp.Items)

	return resp, nil
}
