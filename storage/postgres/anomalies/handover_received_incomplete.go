package anomalies

import (
	"context"
	"fmt"

	"order-monitoring/models"

	"github.com/jackc/pgx/v4/pgxpool"
)

// 10-qoida: qabul qilingan handover to'liq bo'ladi.
// status = RECEIVED bo'lsa, item_count expected_qty dan kam bo'lmaydi.
const (
	handoverReceivedIncompleteRule        = "handover_received_incomplete"
	handoverReceivedIncompleteDescription = "Handover qabul qilingan, lekin kutilganidan kam tovar kelgan (item_count < expected_qty)"
	handoverReceivedIncompleteSeverity    = severityMedium
)

type HandoverReceivedIncomplete struct {
	db *pgxpool.Pool
}

func NewHandoverReceivedIncompleteRepo(db *pgxpool.Pool) *HandoverReceivedIncomplete {
	return &HandoverReceivedIncomplete{
		db: db,
	}
}

func (h *HandoverReceivedIncomplete) HandoverReceivedIncompleteRule(ctx context.Context) (*models.RuleItem, error) {
	resp := &models.RuleItem{
		Rule:        handoverReceivedIncompleteRule,
		Description: handoverReceivedIncompleteDescription,
		Severity:    handoverReceivedIncompleteSeverity,
		Items:       []models.Item{},
	}

	query := `
		SELECT
			COALESCE(o.order_code, o.group_no, h.order_group_id, ''),
			COALESCE(h.handover_no, h.id),
			h.item_count,
			h.expected_qty
		FROM handovers h
		LEFT JOIN orders o ON o.id = h.order_group_id
		WHERE h.status = $1
			AND h.item_count < h.expected_qty
		ORDER BY h.created_at
	`

	rows, err := h.db.Query(ctx, query, received)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			orderCode   string
			handoverNo  string
			itemCount   int
			expectedQty int
		)

		if err := rows.Scan(
			&orderCode,
			&handoverNo,
			&itemCount,
			&expectedQty,
		); err != nil {
			return nil, err
		}

		resp.Items = append(resp.Items, models.Item{
			OrderCode: orderCode,
			Reason: fmt.Sprintf(
				"%s: status=%s, kelgan %d / kutilgan %d",
				handoverNo,
				received,
				itemCount,
				expectedQty,
			),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	resp.Total = len(resp.Items)

	return resp, nil
}
