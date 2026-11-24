package main

import (
	"context"
	"os"

	"order-monitoring/api"
	"order-monitoring/config"
	"order-monitoring/dona"
	"order-monitoring/handler"
	"order-monitoring/logger"
	"order-monitoring/storage/postgres"
	"order-monitoring/syncer"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		logger.Errorf("konfiguratsiya: %v", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// postgres connection + migratsiyalar
	store, err := postgres.NewPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Errorf("baza: %v", err)
		os.Exit(1)
	}
	defer store.CloseDB()

	if err := store.Sync().MarkInterrupted(ctx); err != nil {
		logger.Errorf("sync_runs: %v", err)
		os.Exit(1)
	}

	client := dona.New(dona.Options{
		BaseURL:       cfg.DonaBaseURL,
		Email:         cfg.DonaEmail,
		Password:      cfg.DonaPassword,
		WarehouseCode: cfg.DonaWarehouseCode,
		WarehouseID:   cfg.DonaWarehouseID,
		Timeout:       cfg.HTTPTimeout,
		MaxRetries:    cfg.MaxRetries,
	})
	sy := syncer.New(client, store.Sync(), syncer.Options{
		PageSize:     cfg.PageSize,
		RequestDelay: cfg.RequestDelay,
		Warehouse:    cfg.DonaWarehouseCode,
	})

	if cfg.SyncOnStart {
		go func() {
			// Xato Run ichida logga yoziladi va sync_runs'da saqlanadi.
			_, _ = sy.Run(ctx)
		}()
	}

	router := api.New(handler.NewHandler(store, sy))

	logger.Infof("server %s da ishga tushmoqda", cfg.HTTPAddr)
	if err := router.Run(cfg.HTTPAddr); err != nil {
		logger.Errorf("server to'xtadi: %v", err)
		os.Exit(1)
	}
}
