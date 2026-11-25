package anomalies

import (
	"context"
	"fmt"
	"strings"
	"time"

	"order-monitoring/models"
	"order-monitoring/pkg"

	"github.com/jackc/pgx/v4/pgxpool"
)

// 6-qoida: handover vaqtlari tartib bilan bo'ladi: created_at ≤ transit_since ≤ received_at.
// Bo'sh (NULL) vaqtlar solishtirilmaydi.
const (
	handoverTimestampsOutOfOrderRule        = "handover_timestamps_out_of_order"
	handoverTimestampsOutOfOrderDescription = "Handover vaqtlari tartibi buzilgan (yaratilishidan oldin yo'lga chiqqan yoki qabul qilingan)"
	handoverTimestampsOutOfOrderSeverity    = severityHigh
)

type HandoverTimestampsOutOfOrder struct {
	db *pgxpool.Pool
}

func NewHandoverTimestampsOutOfOrderRepo(db *pgxpool.Pool) *HandoverTimestampsOutOfOrder {
	return &HandoverTimestampsOutOfOrder{
		db: db,
	}
}

func (h *HandoverTimestampsOutOfOrder) HandoverTimestampsOutOfOrderRule(ctx context.Context) (*models.RuleItem, error) {
	resp := &models.RuleItem{
		Rule:        handoverTimestampsOutOfOrderRule,
		Description: handoverTimestampsOutOfOrderDescription,
		Severity:    handoverTimestampsOutOfOrderSeverity,
		Items:       []models.Item{},
	}

	query := `
		SELECT
			COALESCE(o.order_code, o.group_no, h.order_group_id, ''),
			COALESCE(h.handover_no, h.id),
			h.created_at,
			h.transit_since,
			h.received_at
		FROM handovers h
		LEFT JOIN orders o ON o.id = h.order_group_id
		WHERE h.transit_since < h.created_at
			OR h.received_at < h.created_at
			OR h.received_at < h.transit_since
		ORDER BY h.created_at
	`

	rows, err := h.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			orderCode    string
			handoverNo   string
			createdAt    time.Time
			transitSince *time.Time
			receivedAt   *time.Time
		)

		if err := rows.Scan(
			&orderCode,
			&handoverNo,
			&createdAt,
			&transitSince,
			&receivedAt,
		); err != nil {
			return nil, err
		}

		// Qaysi juftlik buzilganini yozamiz: "transit_since created_at dan 2 soat 0 daq oldin".
		var broken []string
		if transitSince != nil && transitSince.Before(createdAt) {
			broken = append(broken, "transit_since created_at dan "+pkg.HumanDuration(createdAt.Sub(*transitSince))+" oldin")
		}
		if receivedAt != nil && receivedAt.Before(createdAt) {
			broken = append(broken, "received_at created_at dan "+pkg.HumanDuration(createdAt.Sub(*receivedAt))+" oldin")
		}
		if receivedAt != nil && transitSince != nil && receivedAt.Before(*transitSince) {
			broken = append(broken, "received_at transit_since dan "+pkg.HumanDuration(transitSince.Sub(*receivedAt))+" oldin")
		}

		resp.Items = append(resp.Items, models.Item{
			OrderCode: orderCode,
			Reason: fmt.Sprintf(
				"%s: %s",
				handoverNo,
				strings.Join(broken, "; "),
			),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	resp.Total = len(resp.Items)

	return resp, nil
}
