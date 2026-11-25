package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"order-monitoring/models"
	"order-monitoring/storage"
)

// ---- buildEvents --------------------------------------------------------

func at(min int) *time.Time {
	t := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC).Add(time.Duration(min) * time.Minute)
	return &t
}

func eventTypes(events []models.TimelineEvent) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = e.Type
	}
	return out
}

// Har bir vaqt maydoni o'z hodisasiga aylanadi va hodisalar vaqt bo'yicha saralanadi.
func TestBuildEventsAllFieldsSorted(t *testing.T) {
	order := &models.OrderTimelineRow{
		Lifecycle: "RETURN", LifecycleOutcome: "RETURNED_TO_SELLERS", CurrentStep: "Returned to sellers",
		LocationKind: "SHELF", LocationLabel: "1",
		CreatedAt: at(0), LifecycleSince: at(50), LocationSince: at(40), LastActivityAt: at(60),
	}
	packages := []models.PackageTimelineRow{{
		PackageNo: "PKG-1", ShelfCode: "RET-01", HeldBy: "Seed Bot", TileShelf: "1",
		PackedAt: at(20), HeldSince: at(45), SortedAt: at(30),
	}}
	handovers := []models.HandoverTimelineRow{{
		HandoverNo: "HND-1", ShopName: "Oasis Mart", ItemCount: 2, ExpectedQty: 3,
		CreatedAt: at(5), TransitSince: at(10), ReceivedAt: at(15),
	}}

	events := buildEvents(order, packages, handovers)

	want := []string{
		"order_created",       // 0
		"handover_created",    // 5
		"handover_in_transit", // 10
		"handover_received",   // 15
		"package_packed",      // 20
		"package_sorted",      // 30
		"order_located",       // 40
		"package_held",        // 45
		"lifecycle_entered",   // 50
		"last_activity",       // 60
	}
	if got := eventTypes(events); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("hodisalar tartibi:\n bor:     %v\n kutilgan: %v", got, want)
	}

	descriptions := map[string]string{
		"lifecycle_entered": "Joriy bosqich: RETURN / RETURNED_TO_SELLERS",
		"order_located":     "Joylashuv: SHELF 1",
		"last_activity":     "Oxirgi faollik: Returned to sellers",
		"package_held":      "PKG-1 RET-01 javonida saqlanmoqda (Seed Bot)",
		"package_sorted":    "PKG-1 javon 1 ga joylandi",
		"handover_created":  "HND-1 yaratildi: Oasis Mart, 3 dona kutilmoqda",
		"handover_received": "HND-1 qabul qilindi: 2/3 dona",
	}
	sources := map[string]string{"order_created": models.SourceOrder, "package_packed": models.SourcePackage, "handover_created": models.SourceHandover}
	for _, e := range events {
		if want, ok := descriptions[e.Type]; ok && e.Description != want {
			t.Errorf("%s: %q, kutilgan %q", e.Type, e.Description, want)
		}
		if want, ok := sources[e.Type]; ok && e.Source != want {
			t.Errorf("%s: source %q, kutilgan %q", e.Type, e.Source, want)
		}
	}
}

// Vaqti bo'sh maydon hodisa bermaydi (o'ylab topilmaydi).
func TestBuildEventsSkipsNullTimes(t *testing.T) {
	order := &models.OrderTimelineRow{CreatedAt: at(0)}
	packages := []models.PackageTimelineRow{{PackageNo: "PKG-1"}}
	handovers := []models.HandoverTimelineRow{{HandoverNo: "HND-1", CreatedAt: at(1)}}

	got := eventTypes(buildEvents(order, packages, handovers))
	if strings.Join(got, ",") != "order_created,handover_created" {
		t.Fatalf("faqat to'ldirilgan vaqtlar hodisa bo'lishi kerak, bor: %v", got)
	}
}

// Vaqtlari teng bo'lsa tartib: order → package → handover.
func TestBuildEventsTieOrder(t *testing.T) {
	order := &models.OrderTimelineRow{CreatedAt: at(0)}
	packages := []models.PackageTimelineRow{{PackageNo: "PKG-1", PackedAt: at(0)}}
	handovers := []models.HandoverTimelineRow{{HandoverNo: "HND-1", CreatedAt: at(0)}}

	events := buildEvents(order, packages, handovers)
	got := []string{events[0].Source, events[1].Source, events[2].Source}
	if strings.Join(got, ",") != "order,package,handover" {
		t.Fatalf("teng vaqtda tartib order,package,handover bo'lishi kerak, bor: %v", got)
	}
}

// Qutisi va handover'i yo'q order: hodisalar faqat orderdan, ro'yxatlar null emas.
func TestBuildEventsOrderOnly(t *testing.T) {
	events := buildEvents(&models.OrderTimelineRow{CreatedAt: at(0), LastActivityAt: at(1)}, nil, nil)
	if len(events) != 2 {
		t.Fatalf("2 ta hodisa kutilgan, bor %d", len(events))
	}
	if buildEvents(&models.OrderTimelineRow{}, nil, nil) == nil {
		t.Fatal("hodisa bo'lmasa ham [] (nil emas) qaytishi kerak")
	}
}

// ---- Timeline (soxta repo bilan) ------------------------------------------

type fakeOrderRepo struct {
	order     *models.OrderTimelineRow
	packages  []models.PackageTimelineRow
	handovers []models.HandoverTimelineRow
	err       error
}

func (f fakeOrderRepo) GetByCode(context.Context, string) (*models.OrderTimelineRow, error) {
	return f.order, f.err
}

func (f fakeOrderRepo) GetPackages(context.Context, string) ([]models.PackageTimelineRow, error) {
	return f.packages, nil
}

func (f fakeOrderRepo) GetHandovers(context.Context, string) ([]models.HandoverTimelineRow, error) {
	return f.handovers, nil
}

// fakeAnomalyService — GetList berilgan javobni qaytaradi (nil bo'lsa bo'sh).
type fakeAnomalyService struct {
	resp *models.RuleResponse
}

func (f fakeAnomalyService) GetList(context.Context, string, string) (*models.RuleResponse, error) {
	if f.resp == nil {
		return &models.RuleResponse{Items: []models.RuleItem{}}, nil
	}
	return f.resp, nil
}

func TestTimelineAssemblesOrder(t *testing.T) {
	repo := fakeOrderRepo{
		order: &models.OrderTimelineRow{
			ID: "o1", Code: "AB123", Lifecycle: "SORTING", WarehouseStatus: "READY_TO_SHIP",
			LocationKind: "SHELF", LocationLabel: "1", CreatedAt: at(0),
		},
		packages:  []models.PackageTimelineRow{{ID: "p1", PackageNo: "PKG-1", Status: "SORTED", ShelfCode: "1", PackedAt: at(10)}},
		handovers: []models.HandoverTimelineRow{{HandoverNo: "HND-1", ExpectedQty: 1, CreatedAt: at(5)}},
	}

	tl, err := NewOrderService(repo, fakeAnomalyService{}).Timeline(context.Background(), "AB123")
	if err != nil {
		t.Fatal(err)
	}

	if tl.Order.Code != "AB123" || tl.Order.Shelf == nil || tl.Order.Shelf.Label != "1" {
		t.Fatalf("order/javon noto'g'ri: %+v", tl.Order)
	}
	if len(tl.Order.Packages) != 1 || tl.Order.Packages[0].PackageNo != "PKG-1" {
		t.Fatalf("qutilar noto'g'ri: %+v", tl.Order.Packages)
	}
	if got := strings.Join(eventTypes(tl.Events), ","); got != "order_created,handover_created,package_packed" {
		t.Fatalf("hodisalar: %s", got)
	}
	if tl.Anomalies == nil || len(tl.Anomalies) != 0 {
		t.Fatalf("anomalies [] bo'lishi kerak, bor %#v", tl.Anomalies)
	}
}

func TestTimelineNoLocation(t *testing.T) {
	repo := fakeOrderRepo{order: &models.OrderTimelineRow{ID: "o2", Code: "GRP-2"}}

	tl, err := NewOrderService(repo, fakeAnomalyService{}).Timeline(context.Background(), "GRP-2")
	if err != nil {
		t.Fatal(err)
	}
	if tl.Order.Shelf != nil || tl.Order.Packages == nil || tl.Events == nil {
		t.Fatalf("joylashuvsiz orderda shelf null, packages va events [] bo'lishi kerak: %+v", tl)
	}
}

func TestTimelineNotFound(t *testing.T) {
	repo := fakeOrderRepo{err: storage.ErrOrderNotFound}

	_, err := NewOrderService(repo, fakeAnomalyService{}).Timeline(context.Background(), "NOPE")
	if !errors.Is(err, storage.ErrOrderNotFound) {
		t.Fatalf("ErrOrderNotFound kutilgan, bor %v", err)
	}
}

// Timeline'ga faqat shu orderning anomaliyalari tushadi.
func TestTimelineAnomaliesOnlyForThisOrder(t *testing.T) {
	repo := fakeOrderRepo{order: &models.OrderTimelineRow{ID: "o1", Code: "AB123"}}
	anomaly := fakeAnomalyService{resp: &models.RuleResponse{Items: []models.RuleItem{
		{Rule: "rule_a", Severity: "high", Items: []models.Item{
			{OrderCode: "AB123", Reason: "a sabab"},
			{OrderCode: "OTHER", Reason: "boshqa order"},
		}},
		{Rule: "rule_b", Severity: "low", Items: []models.Item{{OrderCode: "OTHER", Reason: "boshqa order"}}},
	}}}

	tl, err := NewOrderService(repo, anomaly).Timeline(context.Background(), "AB123")
	if err != nil {
		t.Fatal(err)
	}
	want := []models.Anomaly{{Rule: "rule_a", Severity: "high", OrderCode: "AB123", Reason: "a sabab"}}
	if !reflect.DeepEqual(tl.Anomalies, want) {
		t.Fatalf("faqat AB123 ning rule_a anomaliyasi kutilgan, bor %+v", tl.Anomalies)
	}
}
