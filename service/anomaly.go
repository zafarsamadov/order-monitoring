package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"order-monitoring/models"
	"order-monitoring/storage"
)

// ErrUnknownRule — ?rule= qiymati hech bir qoida nomiga mos kelmadi.
var ErrUnknownRule = errors.New("noma'lum rule")

// examplesPerRule — hisobotda har qoida uchun nechta misol (har biri boshqa order).
const examplesPerRule = 3

type anomalyService struct {
	repo storage.AnomaliesRepoI
}

func NewAnomalyService(repo storage.AnomaliesRepoI) *anomalyService {
	return &anomalyService{repo: repo}
}

// GetList rule va severity bo'yicha (ikkalasi ixtiyoriy) qoidalar natijasini yig'adi.
// Total — noyob orderlar soni; Examples — 3 tagacha misol, har biri boshqa order.
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

	all := []*models.RuleItem{
		issuedOrderPackageReturned,
		conflictingPackageStatuses,
		returnedToSellerInWarehouse,
		packageOnUnknownShelf,
		dispatchedPackageOnShelf,
		handoverTimestampsOutOfOrder,
		activityBeforeLifecycleStart,
		handoverReceivedIncomplete,
		orderItemCountMismatch,
	}

	// Rule nomi severity filtridan oldin to'liq ro'yxat bilan solishtiriladi:
	// noma'lum nom — xato, mavjud nom + boshqa severity — bo'sh ro'yxat.
	if rule != "" && !slices.ContainsFunc(all, func(item *models.RuleItem) bool { return item.Rule == rule }) {
		names := make([]string, 0, len(all))
		for _, item := range all {
			names = append(names, item.Rule)
		}
		return nil, fmt.Errorf("%w %q; mavjudlari: %s", ErrUnknownRule, rule, strings.Join(names, ", "))
	}

	allOrders := map[string]bool{}
	for _, item := range all {
		if rule != "" && item.Rule != rule {
			continue
		}
		if severity != "" && item.Severity != severity {
			continue
		}

		orders := map[string]bool{}
		item.Examples = []models.Item{}
		for _, it := range item.Items {
			if !orders[it.OrderCode] && len(item.Examples) < examplesPerRule {
				item.Examples = append(item.Examples, it)
			}
			orders[it.OrderCode] = true
			allOrders[it.OrderCode] = true
		}
		item.Total = len(orders)

		resp.Items = append(resp.Items, *item)
	}
	resp.TotalOrders = len(allOrders)

	return resp, nil
}
