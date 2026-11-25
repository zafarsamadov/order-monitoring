package models

import "time"

// Timeline manbalari.
const (
	SourceOrder    = "order"
	SourcePackage  = "package"
	SourceHandover = "handover"
)

// Timeline — GET /orders/{code}/timeline javobi.
type Timeline struct {
	Order     TimelineOrder   `json:"order"`
	Events    []TimelineEvent `json:"events"`
	Anomalies []Anomaly       `json:"anomalies"`
}

type TimelineOrder struct {
	ID               string            `json:"id"`
	Code             string            `json:"code"` // order_code → group_no → id
	Lifecycle        string            `json:"lifecycle"`
	LifecycleOutcome string            `json:"lifecycle_outcome"`
	WarehouseStatus  string            `json:"warehouse_status"`
	Shelf            *OrderShelf       `json:"shelf"` // joylashuv bo'lmasa null
	Packages         []TimelinePackage `json:"packages"`
}

type OrderShelf struct {
	Kind  string `json:"kind"`  // SHELF, STAGING, ...
	Label string `json:"label"` // javon kodi, STAGING'da bo'sh
}

type TimelinePackage struct {
	ID        string `json:"id"`
	PackageNo string `json:"package_no"`
	Status    string `json:"status"`
	ShelfCode string `json:"shelf_code"`
}

type TimelineEvent struct {
	Time        time.Time `json:"time"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Source      string    `json:"source"` // order | package | handover
}

// OrderTimelineRow — timeline uchun bazadagi orderdan o'qiladigan maydonlar.
type OrderTimelineRow struct {
	ID               string
	Code             string
	Lifecycle        string
	LifecycleOutcome string
	WarehouseStatus  string
	CurrentStep      string
	LocationKind     string
	LocationLabel    string
	CreatedAt        *time.Time
	LifecycleSince   *time.Time
	LocationSince    *time.Time
	LastActivityAt   *time.Time
}

// PackageTimelineRow — orderning qutisi (javondagi tile bilan birga).
type PackageTimelineRow struct {
	ID        string
	PackageNo string
	Status    string
	ShelfCode string
	HeldBy    string
	PackedAt  *time.Time
	HeldSince *time.Time
	TileShelf string // /shelves dagi javon (tile bo'lsa)
	SortedAt  *time.Time
}

// HandoverTimelineRow — orderning handover'i.
type HandoverTimelineRow struct {
	HandoverNo   string
	ShopName     string
	ItemCount    int
	ExpectedQty  int
	CreatedAt    *time.Time
	TransitSince *time.Time
	ReceivedAt   *time.Time
}
