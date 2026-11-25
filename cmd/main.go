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
	"order-monitoring/models"
	"order-monitoring/service"
	"order-monitoring/storage/postgres"
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

	client := dona.New(models.DonaOptions{
		BaseURL:       cfg.DonaBaseURL,
		Email:         cfg.DonaEmail,
		Password:      cfg.DonaPassword,
		WarehouseCode: cfg.DonaWarehouseCode,
		WarehouseID:   cfg.DonaWarehouseID,
		Timeout:       cfg.HTTPTimeout,
		MaxRetries:    cfg.MaxRetries,
	})
	svc := service.NewService(store, client, models.SyncOptions{
		PageSize:     cfg.PageSize,
		RequestDelay: cfg.RequestDelay,
		Warehouse:    cfg.DonaWarehouseCode,
	})

	if cfg.SyncOnStart {
		go func() {
			// Xato Run ichida logga yoziladi va sync_runs'da saqlanadi.
			_, _ = svc.Sync().Run(ctx)
		}()
	}

	router := api.New(handler.NewHandler(svc))

	logger.Infof("server %s da ishga tushmoqda", cfg.HTTPAddr)
	if err := router.Run(cfg.HTTPAddr); err != nil {
		logger.Errorf("server to'xtadi: %v", err)
		os.Exit(1)
	}
}
