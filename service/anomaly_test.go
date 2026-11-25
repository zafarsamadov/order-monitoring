package service

import (
	"context"
	"errors"
	"testing"

	"order-monitoring/models"
)

// fakeAnomaliesRepo — har bir qoida metodi rules map'idan o'z natijasini qaytaradi.
type fakeAnomaliesRepo struct {
	rules map[string]models.RuleItem
}

func (f fakeAnomaliesRepo) get(name, severity string) (*models.RuleItem, error) {
	item, ok := f.rules[name]
	if !ok {
		item = models.RuleItem{Items: []models.Item{}}
	}
	item.Rule, item.Severity = name, severity
	return &item, nil
}

func (f fakeAnomaliesRepo) IssuedOrderPackageReturnedRule(context.Context) (*models.RuleItem, error) {
	return f.get("issued_order_package_returned", "high")
}
func (f fakeAnomaliesRepo) ConflictingPackageStatusesRule(context.Context) (*models.RuleItem, error) {
	return f.get("conflicting_package_statuses", "medium")
}
func (f fakeAnomaliesRepo) ReturnedToSellerInWarehouseRule(context.Context) (*models.RuleItem, error) {
	return f.get("returned_to_seller_in_warehouse", "medium")
}
func (f fakeAnomaliesRepo) PackageOnUnknownShelfRule(context.Context) (*models.RuleItem, error) {
	return f.get("package_on_unknown_shelf", "high")
}
func (f fakeAnomaliesRepo) DispatchedPackageOnShelfRule(context.Context) (*models.RuleItem, error) {
	return f.get("dispatched_package_on_shelf", "medium")
}
func (f fakeAnomaliesRepo) HandoverTimestampsOutOfOrderRule(context.Context) (*models.RuleItem, error) {
	return f.get("handover_timestamps_out_of_order", "high")
}
func (f fakeAnomaliesRepo) ActivityBeforeLifecycleStartRule(context.Context) (*models.RuleItem, error) {
	return f.get("activity_before_lifecycle_start", "medium")
}
func (f fakeAnomaliesRepo) HandoverReceivedIncompleteRule(context.Context) (*models.RuleItem, error) {
	return f.get("handover_received_incomplete", "medium")
}
func (f fakeAnomaliesRepo) OrderItemCountMismatchRule(context.Context) (*models.RuleItem, error) {
	return f.get("order_item_count_mismatch", "low")
}

// Total — noyob orderlar; misollar — 3 tagacha, har biri boshqa order; total_orders qoidalar bo'yicha noyob.
func TestGetListCountsUniqueOrdersAndDistinctExamples(t *testing.T) {
	repo := fakeAnomaliesRepo{rules: map[string]models.RuleItem{
		"handover_timestamps_out_of_order": {Items: []models.Item{
			{OrderCode: "A", Reason: "h1"},
			{OrderCode: "A", Reason: "h2"}, // bitta orderning ikkinchi handover'i
			{OrderCode: "B", Reason: "h3"},
			{OrderCode: "C", Reason: "h4"},
			{OrderCode: "D", Reason: "h5"},
		}},
		"dispatched_package_on_shelf": {Items: []models.Item{{OrderCode: "A", Reason: "p1"}}},
	}}

	resp, err := NewAnomalyService(repo).GetList(context.Background(), "", "")
	if err != nil {
		t.Fatal(err)
	}

	var handover models.RuleItem
	for _, item := range resp.Items {
		if item.Rule == "handover_timestamps_out_of_order" {
			handover = item
		}
	}
	if handover.Total != 4 {
		t.Fatalf("4 ta noyob order kutilgan (A,B,C,D), bor %d", handover.Total)
	}
	var codes []string
	for _, ex := range handover.Examples {
		codes = append(codes, ex.OrderCode)
	}
	if len(codes) != 3 || codes[0] != "A" || codes[1] != "B" || codes[2] != "C" {
		t.Fatalf("misollar 3 ta turli order bo'lishi kerak (A,B,C), bor %v", codes)
	}
	if resp.TotalOrders != 4 { // A ikki qoidada, lekin bir marta sanaladi
		t.Fatalf("total_orders 4 kutilgan, bor %d", resp.TotalOrders)
	}
}

func TestGetListUnknownRule(t *testing.T) {
	_, err := NewAnomalyService(fakeAnomaliesRepo{}).GetList(context.Background(), "nope", "")
	if !errors.Is(err, ErrUnknownRule) {
		t.Fatalf("ErrUnknownRule kutilgan, bor %v", err)
	}
}

// Mavjud rule + boshqa severity — xato emas, bo'sh ro'yxat.
func TestGetListKnownRuleOtherSeverity(t *testing.T) {
	resp, err := NewAnomalyService(fakeAnomaliesRepo{}).GetList(context.Background(), "dispatched_package_on_shelf", "high")
	if err != nil {
		t.Fatalf("xato kutilmagan: %v", err)
	}
	if len(resp.Items) != 0 || resp.TotalOrders != 0 {
		t.Fatalf("bo'sh ro'yxat kutilgan, bor %+v", resp)
	}
}
