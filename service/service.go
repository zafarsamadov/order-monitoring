// Package service — biznes mantiq: handler service'ni, service esa storage (repo)'ni chaqiradi.
package service

import (
	"context"

	"order-monitoring/models"
	"order-monitoring/storage"
)

type ServiceI interface {
	Sync() SyncServiceI
	Order() OrderServiceI
	Anomaly() AnomalyServiceI
}

type SyncServiceI interface {
	Run(ctx context.Context) (*models.SyncRun, error)
	Running() bool
	LastRun(ctx context.Context) (*models.SyncRun, error)
}

type OrderServiceI interface {
	Timeline(ctx context.Context, code string) (*models.Timeline, error)
}

type AnomalyServiceI interface {
	GetList(ctx context.Context, rule, severity string) (*models.RuleResponse, error)
}

type Service struct {
	sync    SyncServiceI
	order   OrderServiceI
	anomaly AnomalyServiceI
}

func NewService(store storage.StorageI, client Fetcher, syncOpt models.SyncOptions) *Service {
	anomaly := NewAnomalyService(store.Anomalies())

	return &Service{
		sync:    NewSyncService(client, store.Sync(), syncOpt),
		order:   NewOrderService(store.Order(), anomaly),
		anomaly: anomaly,
	}
}

func (s *Service) Sync() SyncServiceI {
	return s.sync
}

func (s *Service) Order() OrderServiceI {
	return s.order
}

func (s *Service) Anomaly() AnomalyServiceI {
	return s.anomaly
}
