package service

import (
	"context"

	"order-monitoring/models"
	"order-monitoring/storage"
)

type anomalyService struct {
	repo storage.AnomaliesRepoI
}

func NewAnomalyService(repo storage.AnomaliesRepoI) *anomalyService {
	return &anomalyService{repo: repo}
}

// GetList rule va severity bo'yicha (ikkalasi ixtiyoriy) qoidalar natijasini yig'adi.
// Yangi qoida qo'shilganda: repo'dan natijasini olib, xuddi shunday add(...) qilinadi.
func (a *anomalyService) GetList(ctx context.Context, rule, severity string) (*models.RuleResponse, error) {
	resp := &models.RuleResponse{
		Items: []models.RuleItem{},
	}

	issuedOrderInWarehouse, err := a.repo.IssuedOrderInWarehouseRule(ctx)
	if err != nil {
		return nil, err
	}

	for _, item := range []*models.RuleItem{
		issuedOrderInWarehouse,
	} {
		if rule != "" && item.Rule != rule {
			continue
		}
		if severity != "" && item.Severity != severity {
			continue
		}

		resp.Items = append(resp.Items, *item)
		resp.TotalOrders += item.Total
	}

	return resp, nil
}
