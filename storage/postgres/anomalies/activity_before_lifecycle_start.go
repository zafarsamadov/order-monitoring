package anomalies

import (
	"context"
	"fmt"
	"time"

	"order-monitoring/models"
	"order-monitoring/pkg"

	"github.com/jackc/pgx/v4/pgxpool"
)

// 7-qoida: oxirgi faollik joriy bosqich boshlanishidan oldin bo'lmaydi.
// Bosqich o'zgarishi ham faollik, shuning uchun last_activity_at >= lifecycle_since.
// Soniyaning bir qismiga teng farqlar — yozish kechikishi, ular tolerans bilan o'tkaziladi.
const (
	activityBeforeLifecycleStartRule        = "activity_before_lifecycle_start"
	activityBeforeLifecycleStartDescription = "Orderning oxirgi faolligi joriy bosqich boshlanishidan oldin"
	activityBeforeLifecycleStartSeverity    = severityMedium

	activityToleranceMinutes = 1
)

type ActivityBeforeLifecycleStart struct {
	db *pgxpool.Pool
}

func NewActivityBeforeLifecycleStartRepo(db *pgxpool.Pool) *ActivityBeforeLifecycleStart {
	return &ActivityBeforeLifecycleStart{
		db: db,
	}
}

func (a *ActivityBeforeLifecycleStart) ActivityBeforeLifecycleStartRule(ctx context.Context) (*models.RuleItem, error) {
	resp := &models.RuleItem{
		Rule:        activityBeforeLifecycleStartRule,
		Description: activityBeforeLifecycleStartDescription,
		Severity:    activityBeforeLifecycleStartSeverity,
		Items:       []models.Item{},
	}

	query := `
		SELECT
			COALESCE(order_code, group_no, id),
			COALESCE(lifecycle, ''),
			lifecycle_since,
			last_activity_at
		FROM orders
		WHERE last_activity_at < lifecycle_since - make_interval(mins => $1)
		ORDER BY created_at
	`

	rows, err := a.db.Query(ctx, query, activityToleranceMinutes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			orderCode      string
			lifecycle      string
			lifecycleSince time.Time
			lastActivityAt time.Time
		)

		if err := rows.Scan(
			&orderCode,
			&lifecycle,
			&lifecycleSince,
			&lastActivityAt,
		); err != nil {
			return nil, err
		}

		resp.Items = append(resp.Items, models.Item{
			OrderCode: orderCode,
			Reason: fmt.Sprintf(
				"lifecycle=%s, lifecycle_since=%s, last_activity_at=%s (%s oldin)",
				lifecycle,
				lifecycleSince.Format(time.DateTime),
				lastActivityAt.Format(time.DateTime),
				pkg.HumanDuration(lifecycleSince.Sub(lastActivityAt)),
			),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	resp.Total = len(resp.Items)

	return resp, nil
}
