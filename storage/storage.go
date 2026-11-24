package storage

import (
	"context"

	"order-monitoring/models"
)

type StorageI interface {
	CloseDB()
	Sync() SyncRepoI
}

// SyncRepoI — bazaga yozadigan yagona joy. Faqat sync ishlatadi; hech narsa o'chirilmaydi.
type SyncRepoI interface {
	// MarkInterrupted oldingi jarayonda uzilib qolgan "running" yozuvlarini failed qiladi.
	MarkInterrupted(ctx context.Context) error
	StartRun(ctx context.Context) (*models.SyncRun, error)
	// SavePage sahifadagi yozuvlarni upsert qiladi va run holatini bitta tranzaksiyada saqlaydi.
	// Muvaffaqiyatli bo'lsa, sahifa natijasi run'ning oxirgi obyekt turiga qo'shiladi.
	SavePage(ctx context.Context, run *models.SyncRun, records []models.Record) (models.UpsertStats, error)
	FinishRun(ctx context.Context, run *models.SyncRun) error
	LastRun(ctx context.Context) (*models.SyncRun, error)
}
