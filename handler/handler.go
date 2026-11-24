package handler

import (
	"order-monitoring/storage"
	"order-monitoring/syncer"
)

type Handler struct {
	storage storage.StorageI
	syncer  *syncer.Syncer
}

func NewHandler(storage storage.StorageI, syncer *syncer.Syncer) *Handler {
	return &Handler{storage: storage, syncer: syncer}
}
