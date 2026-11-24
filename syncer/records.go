package syncer

import (
	"encoding/json"
	"errors"

	"order-monitoring/models"
	"order-monitoring/pkg"
)

// Bu fayl API yozuvlarini bazaga upsert qilinadigan models.Record ko'rinishiga o'tkazadi.
// Har obyekt turi uchun: qaysi maydonlar alohida ustunga chiqadi. To'liq JSON raw'da qoladi.

var errNoKey = errors.New("tabiiy kalit (id) bo'sh")

func orderRecord(raw json.RawMessage) (models.Record, error) {
	var o models.DonaOrder
	if err := json.Unmarshal(raw, &o); err != nil {
		return models.Record{}, err
	}
	var locKind, locLabel string
	var locSince models.NullTime
	if o.Location != nil {
		locKind, locLabel, locSince = o.Location.Kind, o.Location.Label, o.Location.Since
	}
	return newRecord("orders", "id", o.ID, raw, nil, map[string]any{
		"order_code":        pkg.EmptyToNil(o.OrderCode),
		"group_no":          pkg.EmptyToNil(o.GroupNo),
		"warehouse_status":  pkg.EmptyToNil(o.WarehouseStatus),
		"lifecycle":         pkg.EmptyToNil(o.Lifecycle),
		"lifecycle_outcome": pkg.EmptyToNil(o.LifecycleOutcome),
		"lifecycle_clock":   pkg.EmptyToNil(o.LifecycleClock),
		"lifecycle_since":   o.LifecycleSince.Ptr(),
		"current_step_code": pkg.EmptyToNil(o.CurrentStepCode),
		"delivery_status":   pkg.EmptyToNil(o.DeliveryStatus),
		"status_summary":    pkg.EmptyToNil(o.StatusSummary),
		"payment_status":    pkg.EmptyToNil(o.PaymentStatus),
		"item_count":        o.ItemCount,
		"seller_count":      o.SellerCount,
		"total_uzs":         o.TotalUZS,
		"open_exceptions":   o.OpenExceptions,
		"is_dev":            o.IsDev,
		"is_backorder":      o.IsBackorder,
		"location_kind":     pkg.EmptyToNil(locKind),
		"location_label":    pkg.EmptyToNil(locLabel),
		"location_since":    locSince.Ptr(),
		"created_at":        o.CreatedAt.Ptr(),
		"last_activity_at":  o.LastActivityAt.Ptr(),
	})
}

func packageRecord(raw json.RawMessage) (models.Record, error) {
	var p models.DonaPackage
	if err := json.Unmarshal(raw, &p); err != nil {
		return models.Record{}, err
	}
	return newRecord("packages", "id", p.ID, raw, nil, map[string]any{
		"package_no":     pkg.EmptyToNil(p.PackageNo),
		"order_group_id": pkg.EmptyToNil(p.OrderGroupID),
		"order_code":     pkg.EmptyToNil(p.OrderCode),
		"group_no":       pkg.EmptyToNil(p.GroupNo),
		"status":         pkg.EmptyToNil(p.Status),
		"shelf_code":     pkg.EmptyToNil(p.ShelfCode),
		"packed_at":      p.PackedAt.Ptr(),
		"held_since":     p.HeldSince.Ptr(),
		"held_by":        pkg.EmptyToNil(p.HeldBy),
		"item_count":     p.ItemCount,
		"label_printed":  p.LabelPrinted,
		"return_id":      pkg.EmptyToNil(p.ReturnID),
	})
}

func handoverRecord(raw json.RawMessage) (models.Record, error) {
	var h models.DonaHandover
	if err := json.Unmarshal(raw, &h); err != nil {
		return models.Record{}, err
	}
	return newRecord("handovers", "id", h.ID, raw, nil, map[string]any{
		"handover_no":        pkg.EmptyToNil(h.HandoverNo),
		"order_group_id":     pkg.EmptyToNil(h.OrderGroupID),
		"order_code":         pkg.EmptyToNil(h.OrderCode),
		"group_no":           pkg.EmptyToNil(h.GroupNo),
		"seller_id":          pkg.EmptyToNil(h.SellerID),
		"shop_name":          pkg.EmptyToNil(h.ShopName),
		"status":             pkg.EmptyToNil(h.Status),
		"item_count":         h.ItemCount,
		"expected_qty":       h.ExpectedQty,
		"location_confirmed": h.LocationConfirmed,
		"created_at":         h.CreatedAt.Ptr(),
		"transit_since":      h.TransitSince.Ptr(),
		"received_at":        h.ReceivedAt.Ptr(),
	})
}

// shelfVolatileKeys har so'rovda o'zgaradigan hisoblangan maydonlar (javon "yoshi").
// Hash'ga kirmaydi, aks holda har sync'da hamma javon "updated" bo'lib qoladi.
var shelfVolatileKeys = []string{"oldest_age_seconds", "progress"}

func shelfRecord(raw json.RawMessage, s models.DonaShelf) (models.Record, error) {
	return newRecord("shelves", "location_id", s.LocationID, raw, shelfVolatileKeys, map[string]any{
		"code":            pkg.EmptyToNil(s.Code),
		"kind":            pkg.EmptyToNil(s.Kind),
		"name":            pkg.EmptyToNil(s.Name),
		"active":          s.Active,
		"status":          pkg.EmptyToNil(s.Status),
		"box_count":       s.BoxCount,
		"shippable_count": s.ShippableCount,
		"unpacked_count":  s.UnpackedCount,
		"cancelled_count": s.CancelledCount,
	})
}

// tileRecord: tile'da javon kodi yo'q, u ota javondan olinadi. Quti boshqa javonga
// ko'chsa ham "updated" bo'lishi uchun shelf_code hash'ga qo'shiladi.
func tileRecord(raw json.RawMessage, shelfCode string) (models.Record, error) {
	var t models.DonaShelfTile
	if err := json.Unmarshal(raw, &t); err != nil {
		return models.Record{}, err
	}
	rec, err := newRecord("shelf_tiles", "package_id", t.PackageID, raw, nil, map[string]any{
		"shelf_code":     pkg.EmptyToNil(shelfCode),
		"package_no":     pkg.EmptyToNil(t.PackageNo),
		"order_group_id": pkg.EmptyToNil(t.OrderGroupID),
		"area_code":      pkg.EmptyToNil(t.AreaCode),
		"cancelled":      t.Cancelled,
		"packed_at":      t.PackedAt.Ptr(),
		"sorted_at":      t.SortedAt.Ptr(),
	})
	if err != nil {
		return rec, err
	}
	rec.Hash = pkg.SHA256Hex(append([]byte(shelfCode+"|"), rec.Raw...))
	return rec, nil
}

// newRecord kanonik JSON va uning hash'ini hisoblaydi. Ustunlar map'da beriladi,
// tartib Record ichida barqaror bo'lishi uchun saralanadi.
func newRecord(table, keyCol, key string, raw json.RawMessage, ignoreInHash []string, cols map[string]any) (models.Record, error) {
	if key == "" {
		return models.Record{}, errNoKey
	}

	canonical, hashed, err := pkg.CanonicalJSON(raw, ignoreInHash)
	if err != nil {
		return models.Record{}, err
	}

	rec := models.Record{Table: table, KeyCol: keyCol, Key: key, Raw: canonical, Hash: pkg.SHA256Hex(hashed)}
	for _, c := range pkg.SortedKeys(cols) {
		rec.Cols = append(rec.Cols, c)
		rec.Vals = append(rec.Vals, cols[c])
	}
	return rec, nil
}
