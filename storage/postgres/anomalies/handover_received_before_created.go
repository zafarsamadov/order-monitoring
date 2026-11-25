package anomalies

import (
	"context"
	"fmt"
	"time"

	"order-monitoring/models"

	"github.com/jackc/pgx/v4/pgxpool"
)

// 7-qoida: handover yaratilishidan oldin qabul qilinmaydi.
// received_at >= created_at bo'ladi.
const (
	handoverReceivedBeforeCreatedRule        = "handover_received_before_created"
	handoverReceivedBeforeCreatedDescription = "Handover yaratilishidan oldin qabul qilingan (received_at < created_at)"
	handoverReceivedBeforeCreatedSeverity    = severityHigh
)

type HandoverReceivedBeforeCreated struct {
	db *pgxpool.Pool
}

func NewHandoverReceivedBeforeCreatedRepo(db *pgxpool.Pool) *HandoverReceivedBeforeCreated {
	return &HandoverReceivedBeforeCreated{
		db: db,
	}
}

func (h *HandoverReceivedBeforeCreated) HandoverReceivedBeforeCreatedRule(ctx context.Context) (*models.RuleItem, error) {
	resp := &models.RuleItem{
		Rule:        handoverReceivedBeforeCreatedRule,
		Description: handoverReceivedBeforeCreatedDescription,
		Severity:    handoverReceivedBeforeCreatedSeverity,
		Items:       []models.Item{},
	}

	query := `
		SELECT
			COALESCE(o.order_code, o.group_no, h.order_group_id, ''),
			COALESCE(h.handover_no, h.id),
			h.created_at,
			h.received_at
		FROM handovers h
		LEFT JOIN orders o ON o.id = h.order_group_id
		WHERE h.received_at < h.created_at
		ORDER BY h.created_at
	`

	rows, err := h.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			orderCode  string
			handoverNo string
			createdAt  time.Time
			receivedAt time.Time
		)

		if err := rows.Scan(
			&orderCode,
			&handoverNo,
			&createdAt,
			&receivedAt,
		); err != nil {
			return nil, err
		}

		hours := int(createdAt.Sub(receivedAt).Hours())

		resp.Items = append(resp.Items, models.Item{
			OrderCode: orderCode,
			Reason: fmt.Sprintf(
				"%s: received_at=%s, created_at=%s (%d kun %d soat oldin qabul qilingan)",
				handoverNo,
				receivedAt.Format(time.DateTime),
				createdAt.Format(time.DateTime),
				hours/24,
				hours%24,
			),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	resp.Total = len(resp.Items)

	return resp, nil
}
