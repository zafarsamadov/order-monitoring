package main

import (
	"context"
	"flag"
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
	// -migrate: faqat migratsiyalarni qo'llab chiqadi (server ham, sync ham ishga tushmaydi).
	migrateOnly := flag.Bool("migrate", false, "faqat migratsiyalarni qo'llash va chiqish")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		logger.Errorf("konfiguratsiya: %v", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// postgres connection + migratsiyalar
	store, err := postgres.NewPostgres(ctx, cfg.PostgresURL())
	if err != nil {
		logger.Errorf("baza: %v", err)
		os.Exit(1)
	}
	defer store.CloseDB()

	if *migrateOnly {
		logger.Success("migratsiyalar qo'llandi")
		return
	}

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
