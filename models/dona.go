package models

import (
	"bytes"
	"encoding/json"
	"time"
)

// DONA stendi javoblaridagi yozuvlar. Faqat alohida ustunga chiqariladigan maydonlar
// ko'rsatilgan; to'liq JSON baribir `raw` ustunida saqlanadi.
// Maydon nomlari docs/api-samples/ dagi namunalardan olingan.

type DonaOrder struct {
	ID               string        `json:"id"`
	OrderCode        string        `json:"order_code"`
	GroupNo          string        `json:"group_no"`
	WarehouseStatus  string        `json:"warehouse_status"`
	Lifecycle        string        `json:"lifecycle"`
	LifecycleOutcome string        `json:"lifecycle_outcome"`
	LifecycleClock   string        `json:"lifecycle_clock"`
	LifecycleSince   NullTime      `json:"lifecycle_since"`
	CurrentStepCode  string        `json:"current_step_code"`
	DeliveryStatus   string        `json:"delivery_status"`
	StatusSummary    string        `json:"status_summary"`
	PaymentStatus    string        `json:"payment_status"`
	ItemCount        int           `json:"item_count"`
	SellerCount      int           `json:"seller_count"`
	TotalUZS         int64         `json:"total_uzs"`
	OpenExceptions   int           `json:"open_exceptions"`
	IsDev            bool          `json:"is_dev"`
	IsBackorder      bool          `json:"is_backorder"`
	CreatedAt        NullTime      `json:"created_at"`
	LastActivityAt   NullTime      `json:"last_activity_at"`
	Location         *DonaLocation `json:"location"`
}

type DonaLocation struct {
	Kind  string   `json:"kind"`
	Label string   `json:"label"`
	Since NullTime `json:"since"`
}

type DonaPackage struct {
	ID           string   `json:"id"`
	PackageNo    string   `json:"package_no"`
	OrderGroupID string   `json:"order_group_id"`
	OrderCode    string   `json:"order_code"`
	GroupNo      string   `json:"group_no"`
	Status       string   `json:"status"`
	ShelfCode    string   `json:"shelf_code"`
	PackedAt     NullTime `json:"packed_at"`
	HeldSince    NullTime `json:"held_since"`
	HeldBy       string   `json:"held_by"`
	ItemCount    int      `json:"item_count"`
	LabelPrinted bool     `json:"label_printed"`
	ReturnID     string   `json:"return_id"`
}

type DonaHandover struct {
	ID                string   `json:"id"`
	HandoverNo        string   `json:"handover_no"`
	OrderGroupID      string   `json:"order_group_id"`
	OrderCode         string   `json:"order_code"`
	GroupNo           string   `json:"group_no"`
	SellerID          string   `json:"seller_id"`
	ShopName          string   `json:"shop_name"`
	Status            string   `json:"status"`
	ItemCount         int      `json:"item_count"`
	ExpectedQty       int      `json:"expected_qty"`
	LocationConfirmed bool     `json:"location_confirmed"`
	CreatedAt         NullTime `json:"created_at"`
	TransitSince      NullTime `json:"transit_since"`
	ReceivedAt        NullTime `json:"received_at"`
}

type DonaShelf struct {
	LocationID     string            `json:"location_id"`
	Code           string            `json:"code"`
	Kind           string            `json:"kind"`
	Name           string            `json:"name"`
	Active         bool              `json:"active"`
	Status         string            `json:"status"`
	BoxCount       int               `json:"box_count"`
	ShippableCount int               `json:"shippable_count"`
	UnpackedCount  int               `json:"unpacked_count"`
	CancelledCount int               `json:"cancelled_count"`
	Tiles          []json.RawMessage `json:"tiles"`
}

type DonaShelfTile struct {
	PackageID    string   `json:"package_id"`
	PackageNo    string   `json:"package_no"`
	OrderGroupID string   `json:"order_group_id"`
	AreaCode     string   `json:"area_code"`
	Cancelled    bool     `json:"cancelled"`
	PackedAt     NullTime `json:"packed_at"`
	SortedAt     NullTime `json:"sorted_at"`
}

// NullTime — API'dagi vaqt maydoni: null, "" yoki RFC3339 bo'lishi mumkin.
type NullTime struct {
	Time  time.Time
	Valid bool
}

func (t *NullTime) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, []byte("null")) || bytes.Equal(b, []byte(`""`)) {
		*t = NullTime{}
		return nil
	}
	var parsed time.Time
	if err := json.Unmarshal(b, &parsed); err != nil {
		return err
	}
	*t = NullTime{Time: parsed, Valid: true}
	return nil
}

// Ptr bazaga yozish uchun: bo'sh vaqt NULL bo'ladi.
func (t NullTime) Ptr() *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}
