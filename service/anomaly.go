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
// Yangi qoida qo'shilganda: repo'dan natijasini olib, pastdagi ro'yxatga qo'shiladi.
func (a *anomalyService) GetList(ctx context.Context, rule, severity string) (*models.RuleResponse, error) {
	resp := &models.RuleResponse{
		Items: []models.RuleItem{},
	}

	issuedOrderPackageReturned, err := a.repo.IssuedOrderPackageReturnedRule(ctx)
	if err != nil {
		return nil, err
	}

	conflictingPackageStatuses, err := a.repo.ConflictingPackageStatusesRule(ctx)
	if err != nil {
		return nil, err
	}

	returnedToSellerInWarehouse, err := a.repo.ReturnedToSellerInWarehouseRule(ctx)
	if err != nil {
		return nil, err
	}

	packageOnUnknownShelf, err := a.repo.PackageOnUnknownShelfRule(ctx)
	if err != nil {
		return nil, err
	}

	dispatchedPackageOnShelf, err := a.repo.DispatchedPackageOnShelfRule(ctx)
	if err != nil {
		return nil, err
	}

	handoverTimestampsOutOfOrder, err := a.repo.HandoverTimestampsOutOfOrderRule(ctx)
	if err != nil {
		return nil, err
	}

	activityBeforeLifecycleStart, err := a.repo.ActivityBeforeLifecycleStartRule(ctx)
	if err != nil {
		return nil, err
	}

	handoverReceivedIncomplete, err := a.repo.HandoverReceivedIncompleteRule(ctx)
	if err != nil {
		return nil, err
	}

	orderItemCountMismatch, err := a.repo.OrderItemCountMismatchRule(ctx)
	if err != nil {
		return nil, err
	}

	for _, item := range []*models.RuleItem{
		issuedOrderPackageReturned,
		conflictingPackageStatuses,
		returnedToSellerInWarehouse,
		packageOnUnknownShelf,
		dispatchedPackageOnShelf,
		handoverTimestampsOutOfOrder,
		activityBeforeLifecycleStart,
		handoverReceivedIncomplete,
		orderItemCountMismatch,
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
