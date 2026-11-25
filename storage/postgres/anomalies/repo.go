package anomalies

import "github.com/jackc/pgx/v4/pgxpool"

// Repo — barcha qoidalar bitta joyda. Har qoida o'z faylida alohida struct;
// bu yerga qo'shilgach uning metodi Repo orqali chaqiriladi.
type Repo struct {
	*IssuedOrderInWarehouse
	*IssuedOrderPackageReturned
	*ConflictingPackageStatuses
	*ReturnedToSellerInWarehouse
	*PackageOnUnknownShelf
	*DispatchedPackageOnShelf
	*HandoverReceivedBeforeCreated
	*HandoverReceivedIncomplete
	*OrderItemCountMismatch
}

func NewRepo(db *pgxpool.Pool) *Repo {
	return &Repo{
		IssuedOrderInWarehouse:        NewIssuedOrderInWarehouseRepo(db),
		IssuedOrderPackageReturned:    NewIssuedOrderPackageReturnedRepo(db),
		ConflictingPackageStatuses:    NewConflictingPackageStatusesRepo(db),
		ReturnedToSellerInWarehouse:   NewReturnedToSellerInWarehouseRepo(db),
		PackageOnUnknownShelf:         NewPackageOnUnknownShelfRepo(db),
		DispatchedPackageOnShelf:      NewDispatchedPackageOnShelfRepo(db),
		HandoverReceivedBeforeCreated: NewHandoverReceivedBeforeCreatedRepo(db),
		HandoverReceivedIncomplete:    NewHandoverReceivedIncompleteRepo(db),
		OrderItemCountMismatch:        NewOrderItemCountMismatchRepo(db),
	}
}
