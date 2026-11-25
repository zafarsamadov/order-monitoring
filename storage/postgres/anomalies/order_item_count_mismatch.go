package anomalies

import (
	"context"
	"fmt"

	"order-monitoring/models"

	"github.com/jackc/pgx/v4/pgxpool"
)

// 10-qoida: order tovar soni qutilardagi tovarlar yig'indisiga teng.
// orders.item_count = SUM(packages.item_count).
// item_count = 0 "noma'lum" degani (638 orderdan faqat 39 tasida to'ldirilgan), shuning uchun tekshirilmaydi.
const (
	orderItemCountMismatchRule        = "order_item_count_mismatch"
	orderItemCountMismatchDescription = "Order tovar soni qutilardagi tovarlar yig'indisiga teng emas"
	orderItemCountMismatchSeverity    = severityLow
)

type OrderItemCountMismatch struct {
	db *pgxpool.Pool
}

func NewOrderItemCountMismatchRepo(db *pgxpool.Pool) *OrderItemCountMismatch {
	return &OrderItemCountMismatch{
		db: db,
	}
}

func (o *OrderItemCountMismatch) OrderItemCountMismatchRule(ctx context.Context) (*models.RuleItem, error) {
	resp := &models.RuleItem{
		Rule:        orderItemCountMismatchRule,
		Description: orderItemCountMismatchDescription,
		Severity:    orderItemCountMismatchSeverity,
		Items:       []models.Item{},
	}

	query := `
		SELECT
			COALESCE(o.order_code, o.group_no, o.id),
			COALESCE(o.item_count, 0),
			COALESCE(SUM(p.item_count), 0),
			COUNT(p.id)
		FROM orders o
		INNER JOIN packages p ON p.order_group_id = o.id
		WHERE o.item_count > 0
		GROUP BY o.id
		HAVING o.item_count IS DISTINCT FROM SUM(p.item_count)
		ORDER BY o.created_at
	`

	rows, err := o.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			orderCode     string
			orderItems    int
			packageItems  int
			packagesCount int
		)

		if err := rows.Scan(
			&orderCode,
			&orderItems,
			&packageItems,
			&packagesCount,
		); err != nil {
			return nil, err
		}

		resp.Items = append(resp.Items, models.Item{
			OrderCode: orderCode,
			Reason: fmt.Sprintf(
				"orderda %d ta tovar, %d ta qutida jami %d ta tovar",
				orderItems,
				packagesCount,
				packageItems,
			),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	resp.Total = len(resp.Items)

	return resp, nil
}
