package syncer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"

	"order-monitoring/models"
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
		"order_code":        text(o.OrderCode),
		"group_no":          text(o.GroupNo),
		"warehouse_status":  text(o.WarehouseStatus),
		"lifecycle":         text(o.Lifecycle),
		"lifecycle_outcome": text(o.LifecycleOutcome),
		"lifecycle_clock":   text(o.LifecycleClock),
		"lifecycle_since":   o.LifecycleSince.Ptr(),
		"current_step_code": text(o.CurrentStepCode),
		"delivery_status":   text(o.DeliveryStatus),
		"status_summary":    text(o.StatusSummary),
		"payment_status":    text(o.PaymentStatus),
		"item_count":        o.ItemCount,
		"seller_count":      o.SellerCount,
		"total_uzs":         o.TotalUZS,
		"open_exceptions":   o.OpenExceptions,
		"is_dev":            o.IsDev,
		"is_backorder":      o.IsBackorder,
		"location_kind":     text(locKind),
		"location_label":    text(locLabel),
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
		"package_no":     text(p.PackageNo),
		"order_group_id": text(p.OrderGroupID),
		"order_code":     text(p.OrderCode),
		"group_no":       text(p.GroupNo),
		"status":         text(p.Status),
		"shelf_code":     text(p.ShelfCode),
		"packed_at":      p.PackedAt.Ptr(),
		"held_since":     p.HeldSince.Ptr(),
		"held_by":        text(p.HeldBy),
		"item_count":     p.ItemCount,
		"label_printed":  p.LabelPrinted,
		"return_id":      text(p.ReturnID),
	})
}

func handoverRecord(raw json.RawMessage) (models.Record, error) {
	var h models.DonaHandover
	if err := json.Unmarshal(raw, &h); err != nil {
		return models.Record{}, err
	}
	return newRecord("handovers", "id", h.ID, raw, nil, map[string]any{
		"handover_no":        text(h.HandoverNo),
		"order_group_id":     text(h.OrderGroupID),
		"order_code":         text(h.OrderCode),
		"group_no":           text(h.GroupNo),
		"seller_id":          text(h.SellerID),
		"shop_name":          text(h.ShopName),
		"status":             text(h.Status),
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
		"code":            text(s.Code),
		"kind":            text(s.Kind),
		"name":            text(s.Name),
		"active":          s.Active,
		"status":          text(s.Status),
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
		"shelf_code":     text(shelfCode),
		"package_no":     text(t.PackageNo),
		"order_group_id": text(t.OrderGroupID),
		"area_code":      text(t.AreaCode),
		"cancelled":      t.Cancelled,
		"packed_at":      t.PackedAt.Ptr(),
		"sorted_at":      t.SortedAt.Ptr(),
	})
	if err != nil {
		return rec, err
	}
	rec.Hash = hashOf(append([]byte(shelfCode+"|"), rec.Raw...))
	return rec, nil
}

// newRecord kanonik JSON va uning hash'ini hisoblaydi. Ustunlar map'da beriladi,
// tartib Record ichida barqaror bo'lishi uchun saralanadi.
func newRecord(table, keyCol, key string, raw json.RawMessage, ignoreInHash []string, cols map[string]any) (models.Record, error) {
	if key == "" {
		return models.Record{}, errNoKey
	}

	canonical, hashed, err := canonicalJSON(raw, ignoreInHash)
	if err != nil {
		return models.Record{}, err
	}

	rec := models.Record{Table: table, KeyCol: keyCol, Key: key, Raw: canonical, Hash: hashOf(hashed)}
	for _, c := range sortedKeys(cols) {
		rec.Cols = append(rec.Cols, c)
		rec.Vals = append(rec.Vals, cols[c])
	}
	return rec, nil
}

// canonicalJSON kalitlari saralangan JSON qaytaradi: bo'sh joy yoki kalit tartibi
// o'zgarsa ham hash bir xil qoladi. Ikkinchi natija — hash uchun (volatile kalitlarsiz).
func canonicalJSON(raw json.RawMessage, ignore []string) (full, forHash []byte, err error) {
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, nil, err
	}
	full, err = json.Marshal(v) // encoding/json map kalitlarini saralaydi
	if err != nil {
		return nil, nil, err
	}
	if len(ignore) == 0 {
		return full, full, nil
	}
	for _, k := range ignore {
		delete(v, k)
	}
	forHash, err = json.Marshal(v)
	return full, forHash, err
}

func hashOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// text bo'sh satrni NULL qiladi: SQL'da "yo'q" holatini bitta usulda tekshirish uchun.
func text(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
