package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"order-monitoring/models"
	"order-monitoring/pkg"
	"order-monitoring/storage"
)

type orderService struct {
	repo    storage.OrderRepoI
	anomaly AnomalyServiceI
}

func NewOrderService(repo storage.OrderRepoI, anomaly AnomalyServiceI) *orderService {
	return &orderService{repo: repo, anomaly: anomaly}
}

// Timeline order kodi (order_code, group_no yoki id) bo'yicha orderni, uning qutilari va
// handover'larini o'qib, hodisalarni vaqt bo'yicha yig'adi. Faqat bazadan o'qiydi.
// Order topilmasa storage.ErrOrderNotFound.
func (s *orderService) Timeline(ctx context.Context, code string) (*models.Timeline, error) {
	order, err := s.repo.GetByCode(ctx, code)
	if err != nil {
		return nil, err
	}

	packages, err := s.repo.GetPackages(ctx, order.ID)
	if err != nil {
		return nil, err
	}

	handovers, err := s.repo.GetHandovers(ctx, order.ID)
	if err != nil {
		return nil, err
	}

	// Qoidalar natijasidan faqat shu orderniki. Kod ikkalasida ham order_code → group_no → id.
	rules, err := s.anomaly.GetList(ctx, "", "")
	if err != nil {
		return nil, err
	}
	anomalies := []models.Anomaly{}
	for _, rule := range rules.Items {
		for _, item := range rule.Items {
			if item.OrderCode == order.Code {
				anomalies = append(anomalies, models.Anomaly{
					Rule:      rule.Rule,
					Severity:  rule.Severity,
					OrderCode: item.OrderCode,
					Reason:    item.Reason,
				})
			}
		}
	}

	return &models.Timeline{
		Order:     timelineOrder(order, packages),
		Events:    buildEvents(order, packages, handovers),
		Anomalies: anomalies,
	}, nil
}

func timelineOrder(o *models.OrderTimelineRow, packages []models.PackageTimelineRow) models.TimelineOrder {
	out := models.TimelineOrder{
		ID:               o.ID,
		Code:             o.Code,
		Lifecycle:        o.Lifecycle,
		LifecycleOutcome: o.LifecycleOutcome,
		WarehouseStatus:  o.WarehouseStatus,
		Packages:         []models.TimelinePackage{},
	}
	if o.LocationKind != "" {
		out.Shelf = &models.OrderShelf{Kind: o.LocationKind, Label: o.LocationLabel}
	}
	for _, p := range packages {
		out.Packages = append(out.Packages, models.TimelinePackage{
			ID:        p.ID,
			PackageNo: p.PackageNo,
			Status:    p.Status,
			ShelfCode: p.ShelfCode,
		})
	}

	return out
}

// sourceRank: vaqtlari teng hodisalar tartibi — order → package → handover.
var sourceRank = map[string]int{
	models.SourceOrder:    0,
	models.SourcePackage:  1,
	models.SourceHandover: 2,
}

// buildEvents real vaqt maydonlaridan hodisalar yasaydi. Vaqti bo'sh maydon hodisa bermaydi.
// Stend har maydonning faqat oxirgi qiymatini beradi, shuning uchun bu to'liq tarix emas.
func buildEvents(o *models.OrderTimelineRow, packages []models.PackageTimelineRow, handovers []models.HandoverTimelineRow) []models.TimelineEvent {
	events := []models.TimelineEvent{}
	add := func(t *time.Time, typ, source, description string) {
		if t == nil {
			return
		}
		events = append(events, models.TimelineEvent{Time: *t, Type: typ, Description: description, Source: source})
	}

	// order
	add(o.CreatedAt, "order_created", models.SourceOrder, "Buyurtma yaratildi")
	add(o.LifecycleSince, "lifecycle_entered", models.SourceOrder, "Joriy bosqich: "+pkg.JoinNonEmpty(" / ", o.Lifecycle, o.LifecycleOutcome))
	add(o.LocationSince, "order_located", models.SourceOrder, "Joylashuv: "+pkg.JoinNonEmpty(" ", o.LocationKind, o.LocationLabel))
	add(o.LastActivityAt, "last_activity", models.SourceOrder, "Oxirgi faollik: "+o.CurrentStep)

	// package
	for _, p := range packages {
		add(p.PackedAt, "package_packed", models.SourcePackage, p.PackageNo+" qadoqlandi")

		held := p.PackageNo + " saqlanmoqda"
		if p.ShelfCode != "" {
			held = p.PackageNo + " " + p.ShelfCode + " javonida saqlanmoqda"
		}
		if p.HeldBy != "" {
			held += " (" + p.HeldBy + ")"
		}
		add(p.HeldSince, "package_held", models.SourcePackage, held)

		add(p.SortedAt, "package_sorted", models.SourcePackage, p.PackageNo+" javon "+p.TileShelf+" ga joylandi")
	}

	// handover
	for _, h := range handovers {
		created := h.HandoverNo + " yaratildi"
		if h.ShopName != "" {
			created += ": " + h.ShopName
		}
		add(h.CreatedAt, "handover_created", models.SourceHandover, fmt.Sprintf("%s, %d dona kutilmoqda", created, h.ExpectedQty))
		add(h.TransitSince, "handover_in_transit", models.SourceHandover, h.HandoverNo+" yo'lga chiqdi")
		add(h.ReceivedAt, "handover_received", models.SourceHandover,
			fmt.Sprintf("%s qabul qilindi: %d/%d dona", h.HandoverNo, h.ItemCount, h.ExpectedQty))
	}

	sort.SliceStable(events, func(i, j int) bool {
		if !events[i].Time.Equal(events[j].Time) {
			return events[i].Time.Before(events[j].Time)
		}
		return sourceRank[events[i].Source] < sourceRank[events[j].Source]
	})

	return events
}
