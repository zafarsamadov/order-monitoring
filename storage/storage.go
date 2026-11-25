package storage

import (
	"context"
	"errors"

	"order-monitoring/models"
)

var ErrOrderNotFound = errors.New("order topilmadi")

type StorageI interface {
	CloseDB()
	Sync() SyncRepoI
	Order() OrderRepoI
	Anomalies() AnomaliesRepoI
}

type OrderRepoI interface {
	// GetByCode orderni order_code, group_no yoki id bo'yicha topadi; yo'q bo'lsa ErrOrderNotFound.
	GetByCode(ctx context.Context, code string) (*models.OrderTimelineRow, error)
	GetPackages(ctx context.Context, orderID string) ([]models.PackageTimelineRow, error)
	GetHandovers(ctx context.Context, orderID string) ([]models.HandoverTimelineRow, error)
}

type SyncRepoI interface {
	MarkInterrupted(ctx context.Context) error
	StartRun(ctx context.Context) (*models.SyncRun, error)
	SavePage(ctx context.Context, run *models.SyncRun, records []models.Record) (models.UpsertStats, error)
	FinishRun(ctx context.Context, run *models.SyncRun) error
	LastRun(ctx context.Context) (*models.SyncRun, error)
}

type AnomaliesRepoI interface {
	IssuedOrderInWarehouseRule(ctx context.Context) (*models.RuleItem, error)
	IssuedOrderPackageReturnedRule(ctx context.Context) (*models.RuleItem, error)
	ConflictingPackageStatusesRule(ctx context.Context) (*models.RuleItem, error)
	ReturnedToSellerInWarehouseRule(ctx context.Context) (*models.RuleItem, error)
	PackageOnUnknownShelfRule(ctx context.Context) (*models.RuleItem, error)
	DispatchedPackageOnShelfRule(ctx context.Context) (*models.RuleItem, error)
}
