package anomalies

import (
	"context"
	"fmt"
	"time"

	"order-monitoring/models"
	"order-monitoring/pkg"

	"github.com/jackc/pgx/v4/pgxpool"
)

// 8-qoida: handover yo'lda uzoq qolmaydi.
// status = IN_TRANSIT bo'lsa, maxTransitDays kundan oshmaydi. transit_since bo'sh bo'lsa created_at dan hisoblanadi.
const (
	handoverStuckInTransitRule        = "handover_stuck_in_transit"
	handoverStuckInTransitDescription = "Handover yo'lda juda uzoq turibdi"
	handoverStuckInTransitSeverity    = severityLow

	maxTransitDays = 3
)

type HandoverStuckInTransit struct {
	db *pgxpool.Pool
}

func NewHandoverStuckInTransitRepo(db *pgxpool.Pool) *HandoverStuckInTransit {
	return &HandoverStuckInTransit{
		db: db,
	}
}

func (h *HandoverStuckInTransit) HandoverStuckInTransitRule(ctx context.Context) (*models.RuleItem, error) {
	resp := &models.RuleItem{
		Rule:        handoverStuckInTransitRule,
		Description: handoverStuckInTransitDescription,
		Severity:    handoverStuckInTransitSeverity,
		Items:       []models.Item{},
	}

	query := `
		SELECT
			COALESCE(o.order_code, o.group_no, h.order_group_id, ''),
			COALESCE(h.handover_no, h.id),
			COALESCE(h.transit_since, h.created_at),
			h.transit_since IS NULL
		FROM handovers h
		LEFT JOIN orders o ON o.id = h.order_group_id
		WHERE h.status = $1
			AND COALESCE(h.transit_since, h.created_at) < now() - make_interval(days => $2)
		ORDER BY COALESCE(h.transit_since, h.created_at)
	`

	rows, err := h.db.Query(ctx, query, inTransit, maxTransitDays)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			orderCode     string
			handoverNo    string
			since         time.Time
			noTransitTime bool
		)

		if err := rows.Scan(
			&orderCode,
			&handoverNo,
			&since,
			&noTransitTime,
		); err != nil {
			return nil, err
		}

		reason := fmt.Sprintf(
			"%s: %s dan beri %s (%s, chegara %d kun)",
			handoverNo,
			since.Format(time.DateTime),
			inTransit,
			pkg.HumanDuration(time.Since(since)),
			maxTransitDays,
		)
		if noTransitTime {
			reason += "; transit_since yo'q, created_at dan hisoblandi"
		}

		resp.Items = append(resp.Items, models.Item{
			OrderCode: orderCode,
			Reason:    reason,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	resp.Total = len(resp.Items)

	return resp, nil
}
