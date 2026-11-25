package anomalies

import "github.com/jackc/pgx/v4/pgxpool"

// Repo — barcha qoidalar bitta joyda. Har qoida o'z faylida alohida struct;
// bu yerga qo'shilgach uning metodi Repo orqali chaqiriladi.
type Repo struct {
	*IssuedOrderPackageReturned
	*ConflictingPackageStatuses
	*ReturnedToSellerInWarehouse
	*PackageOnUnknownShelf
	*DispatchedPackageOnShelf
	*HandoverTimestampsOutOfOrder
	*ActivityBeforeLifecycleStart
	*HandoverReceivedIncomplete
	*OrderItemCountMismatch
}

func NewRepo(db *pgxpool.Pool) *Repo {
	return &Repo{
		IssuedOrderPackageReturned:   NewIssuedOrderPackageReturnedRepo(db),
		ConflictingPackageStatuses:   NewConflictingPackageStatusesRepo(db),
		ReturnedToSellerInWarehouse:  NewReturnedToSellerInWarehouseRepo(db),
		PackageOnUnknownShelf:        NewPackageOnUnknownShelfRepo(db),
		DispatchedPackageOnShelf:     NewDispatchedPackageOnShelfRepo(db),
		HandoverTimestampsOutOfOrder: NewHandoverTimestampsOutOfOrderRepo(db),
		ActivityBeforeLifecycleStart: NewActivityBeforeLifecycleStartRepo(db),
		HandoverReceivedIncomplete:   NewHandoverReceivedIncompleteRepo(db),
		OrderItemCountMismatch:       NewOrderItemCountMismatchRepo(db),
	}
}
