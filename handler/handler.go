package handler

import (
	"order-monitoring/service"
)

type Handler struct {
	service service.ServiceI
}

func NewHandler(service service.ServiceI) *Handler {
	return &Handler{service: service}
}
