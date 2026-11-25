package models

import "time"

// DonaOptions — stend klienti (dona.Client) sozlamalari.
type DonaOptions struct {
	BaseURL       string // masalan http://host:port/api/v1
	Email         string
	Password      string
	WarehouseCode string // X-Warehouse-Code header, bo'sh bo'lsa yuborilmaydi
	WarehouseID   string // X-Warehouse-Id header, bo'sh bo'lsa yuborilmaydi
	Timeout       time.Duration
	MaxRetries    int
}

// SyncOptions — syncer sozlamalari.
type SyncOptions struct {
	PageSize     int
	RequestDelay time.Duration
	Warehouse    string // /packages va /shelves uchun ?warehouse=
}
