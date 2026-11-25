package rules

import (
	"context"
	"order-monitoring/models"

	"github.com/jackc/pgx/v4/pgxpool"
)

const (
	issuedOrderPackageReturnedRule        = "issued_order_package_returned"
	issuedOrderPackageReturnedDescription = "Xaridorga topshirilgan orderning paketlari qaytarilgan"
	issuedOrderPackageReturnedSeverity    = "high"
)

type IssuedOrderPackageReturned struct {
	db *pgxpool.Pool
}

func NewIssuedOrderPackageReturnedRepo(db *pgxpool.Pool) *IssuedOrderPackageReturned {
	return &IssuedOrderPackageReturned{
		db: db,
	}
}

func (r *IssuedOrderPackageReturned) Get(ctx context.Context) (*models.RuleItem, error) {
	// query := `
	// 	SELECT
	// 		COALESCE(order_code, group_no, id),
	// 	FROM
	// 		orders o
	// 	INNER JOIN
	// 		packages p ON o.id = p.order_id AND p.status != 'ISSUED_TO_BUYER'
	// 	`

	return nil, nil
}
